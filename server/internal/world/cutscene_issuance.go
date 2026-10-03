package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"github.com/google/uuid"
)

// Eight pending plans preserve the previous admission bound. Terminal receipts
// retain the latest eight outcomes independently; pending work is never pruned.
const cutscenePlanLimit = 8

type durableCutscene struct {
	Token      string
	Event      issuedCutscene
	Resolution string
	Completed  bool
}

type cutsceneCompletion struct {
	Token, Label string
	Event        issuedCutscene
	Cancel       bool
	Replayed     bool
}

func requireCutsceneIssuanceSchema(ctx context.Context, database *sql.DB) error {
	rows, err := database.QueryContext(ctx, `SELECT character_id,completion_token,version,script_label,script_json,map_id,x,y,resolution,completed,sequence FROM character_cutscene_plans LIMIT 0`)
	if err != nil {
		return fmt.Errorf("cutscene issuance schema: %w", err)
	}
	return rows.Close()
}

// Caller holds the character lock and owns the transaction. Movement joins this
// write to its step; other authorized triggers commit issuance before delivery.
func issueCutsceneIn(tx db.DBTX, charID int64, event issuedCutscene) (*durableCutscene, error) {
	if err := db.RequireTransaction(tx); err != nil {
		return nil, err
	}
	if event.Script.ScriptLabel == "" {
		return nil, fmt.Errorf("cutscene label required")
	}
	if _, err := DecodeCutsceneActions(event.Script.Actions); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(event.Script)
	if err != nil {
		return nil, err
	}
	var existing string
	err = tx.QueryRow(`SELECT completion_token FROM character_cutscene_plans WHERE character_id=$1 AND resolution='pending' AND script_label=$2 AND script_json=$3 AND map_id=$4 AND x=$5 AND y=$6 ORDER BY sequence LIMIT 1`, charID, event.Script.ScriptLabel, string(encoded), event.MapID, event.X, event.Y).Scan(&existing)
	if err == nil {
		return &durableCutscene{Token: existing, Event: event, Resolution: "pending"}, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM character_cutscene_plans WHERE character_id=$1 AND resolution='pending'`, charID).Scan(&count); err != nil {
		return nil, err
	}
	if count >= cutscenePlanLimit {
		return nil, fmt.Errorf("too many pending cutscenes")
	}
	if _, err := tx.Exec(`DELETE FROM character_cutscene_plans WHERE character_id=$1 AND resolution<>'pending' AND completion_token IN (SELECT completion_token FROM character_cutscene_plans WHERE character_id=$1 AND resolution<>'pending' ORDER BY sequence DESC OFFSET $2)`, charID, cutscenePlanLimit); err != nil {
		return nil, err
	}
	plan := &durableCutscene{Token: uuid.NewString(), Event: event, Resolution: "pending"}
	_, err = tx.Exec(`INSERT INTO character_cutscene_plans(character_id,completion_token,version,script_label,script_json,map_id,x,y,resolution,completed) VALUES($1,$2,1,$3,$4,$5,$6,$7,'pending',false)`, charID, plan.Token, event.Script.ScriptLabel, string(encoded), event.MapID, event.X, event.Y)
	return plan, err
}

func loadCutscenePlanIn(tx db.DBTX, charID int64, token string) (*durableCutscene, error) {
	p := &durableCutscene{Token: token}
	var version int
	var label, raw string
	err := tx.QueryRow(`SELECT version,script_label,script_json,map_id,x,y,resolution,completed FROM character_cutscene_plans WHERE character_id=$1 AND completion_token=$2`, charID, token).Scan(&version, &label, &raw, &p.Event.MapID, &p.Event.X, &p.Event.Y, &p.Resolution, &p.Completed)
	if err != nil {
		return nil, err
	}
	var script *CutsceneScript
	if err := decodePlayerMovement([]byte(raw), &script); err != nil {
		return nil, fmt.Errorf("decode cutscene plan: %w", err)
	}
	if version != 1 || script == nil || script.ScriptLabel != label || label == "" {
		return nil, fmt.Errorf("invalid cutscene plan version/identity")
	}
	if _, err := DecodeCutsceneActions(script.Actions); err != nil {
		return nil, err
	}
	p.Event.Script = *script
	return p, nil
}

func publishCutscenePlan(ses *session.Session, plan *durableCutscene, wh *WorldHandler) {
	cs := &plan.Event.Script
	actions := cs.Actions
	if wh != nil {
		annotated, err := annotateCutsceneActionsForClient(ses.CommandContext(), cs, wh)
		if err != nil {
			log.Printf("[Cutscene] Annotate %s: %v", cs.ScriptLabel, err)
			return
		} // Durable plan remains available; never discard authority on delivery failure.
		if len(annotated) > 0 {
			actions = annotated
		}
	}
	if err := ses.SendStreamJSON(protocol.CutsceneStartNotify{ScriptLabel: cs.ScriptLabel, CompletionToken: plan.Token, MapName: cs.MapName, Actions: actions}, opcodes.CutsceneStartNotify); err != nil {
		log.Printf("[Cutscene] Deliver %s: %v", cs.ScriptLabel, err)
	}
}

func resumePendingCutscene(ses *session.Session, wh *WorldHandler) (bool, error) {
	charID := int64(ses.Client.CharData().ID)
	x, y, mapID := wh.ownedPlayerPosition(ses)
	var plan *durableCutscene
	err := db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
		var token string
		err := tx.QueryRow(`SELECT completion_token FROM character_cutscene_plans WHERE character_id=$1 AND resolution='pending' AND map_id=$2 AND x=$3 AND y=$4 ORDER BY sequence LIMIT 1`, charID, mapID, x, y).Scan(&token)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		plan, err = loadCutscenePlanIn(tx, charID, token)
		return err
	})
	if err != nil || plan == nil {
		return false, err
	}
	publishCutscenePlan(ses, plan, wh)
	return true, nil
}

func resolveCutscenePlanIn(tx db.DBTX, charID int64, completion *cutsceneCompletion, completed bool) error {
	if completion == nil {
		return nil
	}
	// Refresh order on resolution, so an old pending plan retains its new outcome
	// even when newer issuances have already completed.
	resolution := "resolved"
	if completion.Cancel {
		resolution = "cancelled"
	}
	if _, err := tx.Exec(`UPDATE character_cutscene_plans SET sequence=DEFAULT,resolution=$4,completed=$1 WHERE character_id=$2 AND completion_token=$3`, completed, charID, completion.Token, resolution); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM character_cutscene_plans WHERE character_id=$1 AND resolution<>'pending' AND completion_token IN (SELECT completion_token FROM character_cutscene_plans WHERE character_id=$1 AND resolution<>'pending' ORDER BY sequence DESC OFFSET $2)`, charID, cutscenePlanLimit)
	return err
}

// Destination changes invalidate pending sources. Script execution excludes its
// own token until the owning transaction records the completion result.
func cancelCutsceneSourcesIn(tx db.DBTX, charID int64, mapID, x, y int, exceptToken string) error {
	_, err := tx.Exec(`UPDATE character_cutscene_plans SET resolution='cancelled' WHERE character_id=$1 AND resolution='pending' AND completion_token<>$5 AND (map_id<>$2 OR x<>$3 OR y<>$4)`, charID, mapID, x, y, exceptToken)
	return err
}
