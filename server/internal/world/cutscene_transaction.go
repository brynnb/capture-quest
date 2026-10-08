package world

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
)

// cutsceneMutation is private transaction state, shared by the interpreter's
// nested lists and its caller (including a battle turn). Nothing in this plan is
// applied to session/world caches until the owning transaction commits.
type cutsceneMutation struct {
	boundBattle      bool
	position         *cutscenePosition
	database         db.DBTX
	characterID      int64
	party            *[]*pokebattle.Pokemon
	partyDirty       bool
	flagsChanged     bool
	inventoryChanged bool
	messages         []string
	publishActions   []func(CutsceneActionContext) error
	battle           *pokebattle.BattleState
}

func (m *cutsceneMutation) loadParty() ([]*pokebattle.Pokemon, error) {
	if m.party == nil {
		party, err := pokebattle.LoadParty(m.database, m.characterID)
		if err != nil {
			return nil, err
		}
		m.party = &party
	}
	return *m.party, nil
}
func (m *cutsceneMutation) saveParty() error {
	if !m.partyDirty {
		return nil
	}
	party, err := pokebattle.SavePartyInTransaction(m.database, m.characterID, *m.party)
	if err != nil {
		return err
	}
	*m.party = party
	if m.battle != nil {
		m.battle.PlayerParty = party
		return pokebattle.SaveBattleState(m.database, m.characterID, m.battle)
	}
	return nil
}
func (m *cutsceneMutation) publish(ctx CutsceneActionContext) {
	if m.flagsChanged && ctx.EventFlags != nil {
		if err := ctx.EventFlags.LoadFlags(m.characterID); err != nil {
			log.Printf("[Cutscene] Refresh flags for character %d: %v", m.characterID, err)
		}
	}
	for _, publish := range m.publishActions {
		if err := publish(ctx); err != nil {
			log.Printf("[Cutscene] Publish committed effect for character %d: %v", m.characterID, err)
		}
	}
	for _, message := range m.messages {
		sendCutsceneSystemMessage(ctx.Session, message)
	}
	if m.inventoryChanged {
		sendCutsceneInventorySnapshot(ctx.Session, int32(m.characterID))
	}
	if m.partyDirty && ctx.Session != nil {
		notifyResourceChange(ctx.Session)
	}
}

func ApplyCutsceneActionList(executionCtx context.Context, ctx CutsceneActionContext, mapName string, rawActions json.RawMessage, charID int64) ([]CutsceneActionEffect, bool, error) {
	return runCutsceneMutation(executionCtx, ctx, mapName, rawActions, charID, nil)
}

// ApplyCutsceneScript includes completion guards, rewards, completion flags and
// the final warp in one commit. Live handlers and the simulator use this boundary.
func ApplyCutsceneScript(executionCtx context.Context, ctx CutsceneActionContext, script *CutsceneScript, charID int64) ([]CutsceneActionEffect, bool, error) {
	if script == nil {
		return nil, false, fmt.Errorf("cutscene script is required")
	}
	return runCutsceneMutation(executionCtx, ctx, script.MapName, script.Actions, charID, script)
}

func runCutsceneMutation(executionCtx context.Context, ctx CutsceneActionContext, mapName string, raw json.RawMessage, charID int64, script *CutsceneScript) ([]CutsceneActionEffect, bool, error) {
	if ctx.mutation != nil {
		return applyCutsceneActionList(ctx, mapName, raw, charID)
	}
	// The dialogue-only simulator can evaluate choices without a character/DB.
	if charID == 0 {
		actions, err := DecodeCutsceneActions(raw)
		if err != nil {
			return nil, false, err
		}
		if !cutsceneIsPresentationOnly(actions) {
			return nil, false, fmt.Errorf("cutscene mutation requires a character")
		}
		return applyCutsceneActionList(ctx, mapName, raw, charID)
	}
	database := ctx.Database
	if database == nil && ctx.WorldHandler != nil {
		database = ctx.WorldHandler.database
	}
	if database == nil && db.GlobalWorldDB != nil {
		database = db.GlobalWorldDB.DB
	}
	var effects []CutsceneActionEffect
	var completed bool
	mutation := &cutsceneMutation{characterID: charID}
	ctx.mutation = mutation
	err := db.Transaction(executionCtx, database, func(tx db.DBTX) (err error) {
		if ctx.issuedCompletion == nil {
			if err := db.LockCharacter(tx, charID); err != nil {
				return err
			}
		}
		mutation.database = tx
		if completion := ctx.issuedCompletion; completion != nil {
			plan, err := loadCutscenePlanIn(tx, charID, completion.Token)
			if err != nil {
				return err
			}
			if plan.Resolution == "pending" {
				if err := db.LockCharacter(tx, charID); err != nil {
					return err
				}
				// A competing owner/command may have completed while the lock waited.
				plan, err = loadCutscenePlanIn(tx, charID, completion.Token)
				if err != nil {
					return err
				}
			}
			if plan.Event.Script.ScriptLabel != completion.Label || (plan.Resolution == "cancelled" && !completion.Cancel) {
				return fmt.Errorf("cutscene completion authorization unavailable")
			}
			completion.Event = plan.Event
			if plan.Resolution == "resolved" || plan.Resolution == "cancelled" {
				completed, completion.Replayed = plan.Completed, true
				return nil
			}
			if completion.Cancel {
				return resolveCutscenePlanIn(tx, charID, completion, false)
			}
			script = &completion.Event.Script
			mapName, raw = script.MapName, script.Actions
			ctx.issuedSource = &cutscenePosition{mapID: plan.Event.MapID, x: plan.Event.X, y: plan.Event.Y}
		}
		if source := ctx.issuedSource; source != nil {
			x, y, mapID, err := mutation.currentPosition(ctx)
			if err != nil {
				return err
			}
			if x != source.x || y != source.y || mapID != source.mapID {
				return fmt.Errorf("issued cutscene source changed: owned=(%d,%d,%d) issued=(%d,%d,%d)", mapID, x, y, source.mapID, source.x, source.y)
			}
			var savedX, savedY, savedMapID int
			if err := tx.QueryRow(`SELECT CAST(x AS INTEGER),CAST(y AS INTEGER),map_id FROM character_data WHERE id=$1`, charID).Scan(&savedX, &savedY, &savedMapID); err != nil {
				return err
			}
			if savedX != source.x || savedY != source.y || savedMapID != source.mapID {
				return fmt.Errorf("issued cutscene durable source changed: saved=(%d,%d,%d) issued=(%d,%d,%d)", savedMapID, savedX, savedY, source.mapID, source.x, source.y)
			}
			// Seed all nested relative actions from the captured source. Animation
			// coordinates never become the input to durable gameplay decisions.
			mutation.position = &cutscenePosition{mapID: source.mapID, x: source.x, y: source.y}
		}
		if script != nil {
			allowed, err := cutsceneCompletionAllowed(tx, charID, script)
			if err != nil {
				return err
			}
			if !allowed {
				return resolveCutscenePlanIn(tx, charID, ctx.issuedCompletion, false)
			}
		}
		effects, completed, err = applyCutsceneActionList(ctx, mapName, raw, charID)
		if err != nil {
			return err
		}
		if completed && script != nil {
			for _, flag := range script.SetsFlags {
				if flag == "" {
					continue
				}
				if err := mutation.setFlag(flag, true); err != nil {
					return err
				}
				effects = append(effects, CutsceneActionEffect{Type: "setsFlags", Detail: flag, Changed: true})
			}
			if script.WarpToMapID != nil && script.WarpToX != nil && script.WarpToY != nil {
				if err := mutation.movePlayer(ctx, *script.WarpToMapID, *script.WarpToX, *script.WarpToY, "DOWN", true); err != nil {
					return err
				}
				effects = append(effects, CutsceneActionEffect{Type: "warp", Detail: fmt.Sprintf("map=%d x=%d y=%d", *script.WarpToMapID, *script.WarpToX, *script.WarpToY), Changed: true})
			}
		}
		if err := mutation.saveParty(); err != nil {
			return err
		}
		return resolveCutscenePlanIn(tx, charID, ctx.issuedCompletion, completed)
	})
	if err != nil {
		return nil, false, err
	}
	mutation.publish(ctx)
	return effects, completed, nil
}

func cutsceneIsPresentationOnly(actions []CutsceneAction) bool {
	for _, a := range actions {
		switch a.Type {
		case "dialogue", "dialogueText", "choice", "move", "showActor", "hideActor", "delay", "playSFX", "playMusic", "playCry":
		case "parallel":
			if !cutsceneIsPresentationOnly(a.Actions) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// Recheck durable eligibility under the same character lock as the reward.
// Facing and trigger proximity authorize issuance; playback can intentionally
// move the player, so completion must not reinterpret that original trigger.
func cutsceneCompletionAllowed(database db.DBTX, charID int64, script *CutsceneScript) (bool, error) {
	required := append([]string(nil), script.RequiresFlags...)
	if script.RequiresFlag != nil {
		required = append(required, *script.RequiresFlag)
	}
	absent := append([]string(nil), script.RequiresFlagsAbst...)
	if script.RequiresFlagAbst != nil {
		absent = append(absent, *script.RequiresFlagAbst)
	}
	for _, group := range []struct {
		flags []string
		want  bool
	}{{required, true}, {absent, false}} {
		for _, flag := range group.flags {
			if flag == "" {
				continue
			}
			on, err := queryEventFlag(database, charID, flag)
			if err != nil {
				return false, err
			}
			if on != group.want {
				return false, nil
			}
		}
	}
	for _, check := range []struct {
		id   *int
		want bool
	}{{script.RequiresItemID, true}, {script.RequiresItemAbst, false}} {
		if check.id == nil || *check.id <= 0 {
			continue
		}
		var owns bool
		err := database.QueryRow(`SELECT EXISTS(SELECT 1 FROM cq_character_inventory ci JOIN cq_item_instances ii ON ii.id=ci.item_instance_id WHERE ci.character_id=$1 AND ii.owner_id=$1 AND ii.owner_type=0 AND ii.item_id=$2 AND ii.quantity>0)`, charID, *check.id).Scan(&owns)
		if err != nil {
			return false, err
		}
		if owns != check.want {
			return false, nil
		}
	}
	if script.RequiresCaught != nil && *script.RequiresCaught > 0 {
		var caught int
		if err := database.QueryRow(`SELECT COALESCE(SUM(caught),0) FROM character_pokedex WHERE character_id=$1`, charID).Scan(&caught); err != nil {
			return false, err
		}
		if caught < *script.RequiresCaught {
			return false, nil
		}
	}
	for _, balance := range []struct {
		query          string
		minimum, below *int
	}{
		{`SELECT COALESCE((SELECT pokedollars FROM character_wallet WHERE character_id=$1),0)`, script.RequiresMoney, script.RequiresMoneyBelow},
		{`SELECT COALESCE((SELECT coins FROM character_coins WHERE character_id=$1),0)`, script.RequiresCoins, script.RequiresCoinsBelow},
	} {
		if (balance.minimum == nil || *balance.minimum <= 0) && (balance.below == nil || *balance.below <= 0) {
			continue
		}
		var value int
		if err := database.QueryRow(balance.query, charID).Scan(&value); err != nil {
			return false, err
		}
		if balance.minimum != nil && *balance.minimum > 0 && value < *balance.minimum {
			return false, nil
		}
		if balance.below != nil && *balance.below > 0 && value >= *balance.below {
			return false, nil
		}
	}
	return true, nil
}

func (m *cutsceneMutation) setFlag(flag string, on bool) error {
	if err := writeEventFlag(m.database, m.characterID, flag, on); err != nil {
		return err
	}
	m.flagsChanged = true
	return nil
}

type cutscenePosition struct{ mapID, x, y int }

func (m *cutsceneMutation) currentPosition(ctx CutsceneActionContext) (int, int, int, error) {
	if p := m.position; p != nil {
		return p.x, p.y, p.mapID, nil
	}
	if ctx.Session != nil && ctx.WorldHandler != nil {
		x, y, mapID := ctx.WorldHandler.ownedPlayerPosition(ctx.Session)
		return x, y, mapID, nil
	}
	if ctx.Session != nil {
		return currentCutscenePlayerPosition(ctx.Session, m.characterID)
	}
	var x, y, mapID int
	err := m.database.QueryRow(`SELECT CAST(x AS INTEGER),CAST(y AS INTEGER),map_id FROM character_data WHERE id=$1`, m.characterID).Scan(&x, &y, &mapID)
	return x, y, mapID, err
}
func (m *cutsceneMutation) movePlayer(ctx CutsceneActionContext, mapID, x, y int, direction string, warp bool) error {
	storedMapID := normalizedVisiblePlayerMapID(ctx.WorldHandler, mapID)
	ended, err := endSafariForDestinationIn(m.database, m.characterID, storedMapID)
	if err != nil {
		return err
	}
	if ended {
		m.flagsChanged = true
	}
	exceptToken := ""
	if ctx.issuedCompletion != nil {
		exceptToken = ctx.issuedCompletion.Token
	}
	if err := cancelCutsceneSourcesIn(m.database, m.characterID, storedMapID, x, y, exceptToken); err != nil {
		return err
	}
	if _, err := m.database.Exec(`UPDATE character_data SET map_id=$1,x=$2,y=$3 WHERE id=$4`, storedMapID, x, y, m.characterID); err != nil {
		return err
	}
	m.position = &cutscenePosition{mapID: mapID, x: x, y: y}
	m.publishActions = append(m.publishActions, func(p CutsceneActionContext) error {
		// This updates the existing movement/world owner only after its durable
		// position has committed. Publication never issues another database write.
		setCutscenePlayerPosition(p.Session, p.WorldHandler, m.characterID, mapID, x, y, direction)
		if warp {
			sendCommittedWarpNotification(p.Session, mapID, x, y, direction)
		}
		return nil
	})
	return nil
}
