package world

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

const (
	cqItemTypeMisc           = 0
	cqItemTypePokeBall       = 1
	cqItemTypeMedicine       = 2
	cqItemTypeBattleItem     = 3
	cqItemTypeFieldItem      = 4
	cqItemTypeTM             = 5
	cqItemTypeHM             = 6
	cqItemTypeEvolutionStone = 9

	vitaminEVIncrease = 2560
	vitaminEVCap      = 25600
)

func itemShortName(item cqitems.CQItem) string {
	return strings.ToUpper(item.ShortName)
}

func itemStatusCure(item cqitems.CQItem) string {
	if item.StatusCure == nil {
		return ""
	}
	return *item.StatusCure
}

func isMedicineItem(item cqitems.CQItem) bool {
	return item.HealAmount > 0 || itemStatusCure(item) != "" || item.RevivePercent > 0 || item.PPRestore > 0
}

func isPPRestoreAll(item cqitems.CQItem) bool {
	switch itemShortName(item) {
	case "ELIXIR", "MAX_ELIXIR":
		return true
	default:
		return false
	}
}

func medicineEffectFromItem(item cqitems.CQItem) pokebattle.ItemEffect {
	eff := pokebattle.ItemEffectFromData(
		int(item.HealAmount),
		itemStatusCure(item),
		int(item.PPRestore),
		int(item.RevivePercent),
	)
	eff.PPRestoreAll = isPPRestoreAll(item)
	return eff
}

func isRareCandy(item cqitems.CQItem) bool {
	return itemShortName(item) == "RARE_CANDY" || item.ID == 40
}

func isTMHM(item cqitems.CQItem) bool {
	return item.ItemType == cqItemTypeTM || item.ItemType == cqItemTypeHM
}

func isEvolutionStone(item cqitems.CQItem) bool {
	return item.ItemType == cqItemTypeEvolutionStone || strings.HasSuffix(itemShortName(item), "_STONE")
}

func isPPUp(item cqitems.CQItem) bool {
	short := itemShortName(item)
	return short == "PP_UP" || short == "PP_UP_2"
}

func vitaminTarget(item cqitems.CQItem) (statName string, current func(*pokebattle.Pokemon) *int, ok bool) {
	switch itemShortName(item) {
	case "HP_UP":
		return "HP", func(p *pokebattle.Pokemon) *int { return &p.EVs.HP }, true
	case "PROTEIN":
		return "ATTACK", func(p *pokebattle.Pokemon) *int { return &p.EVs.Attack }, true
	case "IRON":
		return "DEFENSE", func(p *pokebattle.Pokemon) *int { return &p.EVs.Defense }, true
	case "CARBOS":
		return "SPEED", func(p *pokebattle.Pokemon) *int { return &p.EVs.Speed }, true
	case "CALCIUM":
		return "SPECIAL", func(p *pokebattle.Pokemon) *int { return &p.EVs.Special }, true
	default:
		return "", nil, false
	}
}

func itemUsableOnPartyOutsideBattle(item cqitems.CQItem) bool {
	if isMedicineItem(item) || isRareCandy(item) || isTMHM(item) || isEvolutionStone(item) || isPPUp(item) {
		return true
	}
	_, _, isVitamin := vitaminTarget(item)
	return isVitamin
}

func applyVitamin(item cqitems.CQItem, p *pokebattle.Pokemon) (string, error) {
	if p == nil {
		return "", fmt.Errorf("no Pokémon selected")
	}
	if p.IsFainted() {
		return "", fmt.Errorf("%s has fainted", p.Name)
	}
	statName, getEV, ok := vitaminTarget(item)
	if !ok {
		return "", fmt.Errorf("not a vitamin")
	}
	ev := getEV(p)
	if *ev >= vitaminEVCap {
		return "", fmt.Errorf("it won't have any effect")
	}

	oldMaxHP := p.MaxHP
	*ev += vitaminEVIncrease
	if *ev > vitaminEVCap {
		*ev = vitaminEVCap
	}
	p.RecalculateStats()
	if p.MaxHP > oldMaxHP {
		p.CurHP += p.MaxHP - oldMaxHP
	}
	if p.CurHP > p.MaxHP {
		p.CurHP = p.MaxHP
	}
	return fmt.Sprintf("%s's %s rose!", p.Name, statName), nil
}

func applyPPUp(p *pokebattle.Pokemon, moveSlot int) (string, error) {
	if p == nil {
		return "", fmt.Errorf("no Pokémon selected")
	}
	if p.IsFainted() {
		return "", fmt.Errorf("%s has fainted", p.Name)
	}
	if moveSlot < 0 || moveSlot > 3 || p.Moves[moveSlot].ID == 0 {
		return "", fmt.Errorf("invalid move slot")
	}
	move := &p.Moves[moveSlot]
	if move.PPUps >= 3 {
		return "", fmt.Errorf("%s's PP won't go any higher", move.Name)
	}

	basePP := move.BasePP
	if basePP <= 0 {
		basePP = move.MaxPP * 5 / (5 + move.PPUps)
		if basePP <= 0 {
			basePP = move.MaxPP
		}
	}
	oldMaxPP := move.MaxPP
	move.PPUps++
	move.BasePP = basePP
	move.MaxPP = pokebattle.MaxPPWithUps(basePP, move.PPUps)
	move.PP += move.MaxPP - oldMaxPP
	if move.PP > move.MaxPP {
		move.PP = move.MaxPP
	}
	return fmt.Sprintf("%s's PP rose!", move.Name), nil
}

func applyStoneEvolution(myDB pokebattle.DBTX, item cqitems.CQItem, p *pokebattle.Pokemon) (string, error) {
	if p == nil {
		return "", fmt.Errorf("no Pokémon selected")
	}
	if p.IsFainted() {
		return "", fmt.Errorf("%s has fainted", p.Name)
	}

	evolvedID, ok := stoneEvolutionTarget(itemShortName(item), p.ID)
	if !ok {
		return "", fmt.Errorf("it won't have any effect")
	}
	oldName := p.Name
	if err := pokebattle.EvolvePokemon(myDB, p, evolvedID); err != nil {
		return "", fmt.Errorf("failed to evolve %s: %w", oldName, err)
	}
	return fmt.Sprintf("What? %s is evolving!\n%s evolved into %s!", oldName, oldName, p.Name), nil
}

func stoneEvolutionTarget(stone string, pokemonID int) (int, bool) {
	evolutions := map[string]map[int]int{
		"MOON_STONE": {
			30: 31, 33: 34, 35: 36, 39: 40,
		},
		"FIRE_STONE": {
			37: 38, 58: 59, 133: 136,
		},
		"THUNDER_STONE": {
			25: 26, 133: 135,
		},
		"WATER_STONE": {
			61: 62, 90: 91, 120: 121, 133: 134,
		},
		"LEAF_STONE": {
			44: 45, 70: 71, 102: 103,
		},
	}
	targets, ok := evolutions[stone]
	if !ok {
		return 0, false
	}
	evolvedID, ok := targets[pokemonID]
	return evolvedID, ok
}

func battleItemHasEffect(item cqitems.CQItem) bool {
	return item.ItemType == cqItemTypeBattleItem ||
		item.IsGuardDrink ||
		item.BonusAttack != 0 ||
		item.BonusDefense != 0 ||
		item.BonusSpeed != 0 ||
		item.BonusSpecial != 0 ||
		item.BonusAccuracy != 0 ||
		item.BonusEvasion != 0 ||
		item.BonusCrit != 0 ||
		item.BonusFlee != 0 ||
		itemShortName(item) == "POKE_FLUTE"
}

func applyBattleBoostItem(item cqitems.CQItem, p *pokebattle.Pokemon) (string, error) {
	if p == nil {
		return "", fmt.Errorf("no Pokémon selected")
	}
	if p.IsFainted() {
		return "", fmt.Errorf("%s has fainted", p.Name)
	}

	switch itemShortName(item) {
	case "GUARD_SPEC":
		if p.GuardSpec {
			return "", fmt.Errorf("it won't have any effect")
		}
		p.GuardSpec = true
		return fmt.Sprintf("%s became guarded against stat drops!", p.Name), nil
	case "DIRE_HIT":
		if p.DireHit {
			return "", fmt.Errorf("it won't have any effect")
		}
		p.DireHit = true
		return fmt.Sprintf("%s is getting pumped!", p.Name), nil
	}

	boosts := battleStageBoosts(item)
	if len(boosts) == 0 {
		return "", fmt.Errorf("that item can't be used here")
	}

	var messages []string
	for _, boost := range boosts {
		msg, err := raiseBattleStage(p, boost.name, boost.delta)
		if err != nil {
			continue
		}
		messages = append(messages, msg)
	}
	if len(messages) == 0 {
		return "", fmt.Errorf("it won't have any effect")
	}
	return strings.Join(messages, "\n"), nil
}

type battleStageBoost struct {
	name  string
	delta int
}

func battleStageBoosts(item cqitems.CQItem) []battleStageBoost {
	short := itemShortName(item)
	candidates := []battleStageBoost{
		{name: "ATTACK", delta: int(defaultBattleBoost(short, "X_ATTACK", item.BonusAttack))},
		{name: "DEFENSE", delta: int(defaultBattleBoost(short, "X_DEFEND", item.BonusDefense))},
		{name: "SPEED", delta: int(defaultBattleBoost(short, "X_SPEED", item.BonusSpeed))},
		{name: "SPECIAL", delta: int(defaultBattleBoost(short, "X_SPECIAL", item.BonusSpecial))},
		{name: "ACCURACY", delta: int(defaultBattleBoost(short, "X_ACCURACY", item.BonusAccuracy))},
		{name: "EVASION", delta: int(item.BonusEvasion)},
	}
	boosts := make([]battleStageBoost, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.delta != 0 {
			boosts = append(boosts, candidate)
		}
	}
	return boosts
}

func defaultBattleBoost(short string, matchingShort string, dbValue int32) int32 {
	if dbValue != 0 {
		return dbValue
	}
	if short == matchingShort {
		return 1
	}
	return 0
}

func raiseBattleStage(p *pokebattle.Pokemon, statName string, delta int) (string, error) {
	stage := battleStagePointer(p, statName)
	if stage == nil {
		return "", fmt.Errorf("unknown stat")
	}
	if *stage >= 6 && delta > 0 {
		return "", fmt.Errorf("it won't have any effect")
	}
	if *stage <= -6 && delta < 0 {
		return "", fmt.Errorf("it won't have any effect")
	}
	*stage += delta
	if *stage > 6 {
		*stage = 6
	}
	if *stage < -6 {
		*stage = -6
	}
	return fmt.Sprintf("%s's %s rose!", p.Name, statName), nil
}

func battleStagePointer(p *pokebattle.Pokemon, statName string) *int {
	switch statName {
	case "ATTACK":
		return &p.AtkStage
	case "DEFENSE":
		return &p.DefStage
	case "SPEED":
		return &p.SpdStage
	case "SPECIAL":
		return &p.SpcStage
	case "ACCURACY":
		return &p.AccStage
	case "EVASION":
		return &p.EvaStage
	default:
		return nil
	}
}

func applyPokeFluteBattle(battle *pokebattle.BattleState) (string, error) {
	var woke []string
	for _, p := range []*pokebattle.Pokemon{battle.GetPlayerPokemon(), battle.GetEnemyPokemon()} {
		if p != nil && p.Status == pokebattle.StatusSleep {
			p.ClearMajorStatus()
			woke = append(woke, p.Name)
		}
	}
	if len(woke) == 0 {
		return "", fmt.Errorf("it won't have any effect")
	}
	return "The Poké Flute played!\n" + strings.Join(woke, " woke up!\n") + " woke up!", nil
}

func tryHandleFieldItemUse(ses *session.Session, wh *WorldHandler, found *cqitems.CQInventoryItem, charID int32) bool {
	item := found.Item
	switch itemShortName(item) {
	case "REPEL", "SUPER_REPEL", "MAX_REPEL":
		handleCQRepelUse(ses, wh, found, charID)
		return true
	case "ESCAPE_ROPE":
		handleCQEscapeRopeUse(ses, wh, found, charID)
		return true
	case "OLD_ROD", "GOOD_ROD", "SUPER_ROD":
		payload, _ := json.Marshal(map[string]interface{}{"itemId": item.ID, "rodType": itemShortName(item)})
		HandlePokeFishing(ses, payload, wh)
		return true
	case "BICYCLE":
		result := BicycleToggleState{}
		ok := false
		if wh.PlayerMovement != nil {
			result, ok = wh.PlayerMovement.ToggleBicycle(int(charID))
		}
		message := "You got off the Bicycle."
		if ok && result.ForcedRiding {
			message = "You can't get off here."
		} else if ok && result.WantsRiding {
			if result.ActiveRiding {
				message = "You got on the Bicycle!"
			} else {
				message = "You'll get on the Bicycle when you go outside."
			}
		}
		sendCQItemUseSuccess(ses, found, message, found.Instance.Quantity, map[string]interface{}{
			"bicycle": result,
		})
		return true
	case "TOWN_MAP":
		sendCQItemUseSuccess(ses, found, currentMapMessage(ses), found.Instance.Quantity)
		return true
	case "ITEMFINDER":
		sendCQItemUseSuccess(ses, found, itemfinderMessage(ses, wh), found.Instance.Quantity)
		return true
	case "POKE_FLUTE":
		handleCQPokeFluteUse(ses, found, charID)
		return true
	case "POKEDEX":
		sendCQItemUseSuccess(ses, found, "You checked your Pokédex.", found.Instance.Quantity)
		return true
	case "EXP_ALL":
		sendCQItemUseSuccess(ses, found, "EXP.ALL is ready.", found.Instance.Quantity)
		return true
	default:
		return false
	}
}

func handleCQRepelUse(ses *session.Session, wh *WorldHandler, found *cqitems.CQInventoryItem, charID int32) {
	result, err := UseRepelInventoryItem(wh, charID, found.Item.ID, found)
	if err != nil {
		sendCQItemUseError(ses, err.Error())
		return
	}
	sendCQItemUseSuccess(ses, found, result.Message, result.NewQuantity)
}

func handleCQEscapeRopeUse(ses *session.Session, wh *WorldHandler, found *cqitems.CQInventoryItem, charID int32) {
	destMapID, destX, destY, err := escapeRopeDestination(ses, wh)
	if err != nil {
		sendCQItemUseError(ses, err.Error())
		return
	}
	newQty, _ := cqitems.DecrementItemQuantity(charID, found.Instance.ID)
	teleportPlayerTo(ses, wh, destMapID, destX, destY)
	sendCQItemUseSuccess(ses, found, "You escaped from the dungeon.", newQty)
}

func handleCQPokeFluteUse(ses *session.Session, found *cqitems.CQInventoryItem, charID int32) {
	myDB := db.GlobalWorldDB.DB
	party, err := pokebattle.LoadParty(myDB, int64(charID))
	if err != nil || len(party) == 0 {
		sendCQItemUseError(ses, "Failed to load party")
		return
	}
	woke := false
	for _, p := range party {
		if p != nil && p.Status == pokebattle.StatusSleep {
			p.ClearMajorStatus()
			woke = true
		}
	}
	if !woke {
		sendCQItemUseError(ses, "it won't have any effect")
		return
	}
	_ = pokebattle.SaveParty(myDB, int64(charID), party)
	sendCQItemUseSuccess(ses, found, "The Poké Flute woke sleeping Pokémon!", found.Instance.Quantity)
	sendPartyUpdate(ses)
}

func currentMapMessage(ses *session.Session) string {
	mapID := ses.MapID
	if ses.HasValidClient() && ses.Client.CharData() != nil {
		mapID = int(ses.Client.CharData().MapID)
	}
	var name string
	if err := db.GlobalWorldDB.DB.QueryRow(`SELECT name FROM phaser_maps WHERE id = $1`, mapID).Scan(&name); err == nil && name != "" {
		return "You're currently at " + name + "."
	}
	return "You checked the Town Map."
}

func itemfinderMessage(ses *session.Session, wh *WorldHandler) string {
	x, y, mapID := currentTilePosition(ses, wh)
	var count int
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT (
			SELECT COUNT(*) FROM phaser_hidden_items WHERE map_id = $1 AND ABS(x - $2) <= 7 AND ABS(y - $3) <= 7
		) + (
			SELECT COUNT(*) FROM phaser_hidden_coins WHERE map_id = $4 AND ABS(x - $5) <= 7 AND ABS(y - $6) <= 7
		) + (
			SELECT COUNT(*) FROM phaser_hidden_objects WHERE map_id = $7 AND ABS(x - $8) <= 7 AND ABS(y - $9) <= 7
		)
	`, mapID, x, y, mapID, x, y, mapID, x, y).Scan(&count)
	if err == nil && count > 0 {
		return "The ITEMFINDER's responding!"
	}
	return "Nope! There's no response."
}

func escapeRopeDestination(ses *session.Session, wh *WorldHandler) (int, int, int, error) {
	_, _, mapID := currentTilePosition(ses, wh)
	if isEscapeRopeBlockedOnMap(mapID) {
		return 0, 0, 0, fmt.Errorf("Can't use that here")
	}

	var destMapID, destX, destY int
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT pw.destination_map_id, pw.destination_x, pw.destination_y
		FROM phaser_warps pw
		LEFT JOIN phaser_maps pm ON pm.id = pw.destination_map_id
		WHERE pw.source_map_id = $1
			AND pw.destination_map_id IS NOT NULL
			AND pw.destination_x IS NOT NULL
			AND pw.destination_y IS NOT NULL
		ORDER BY CASE WHEN COALESCE(pm.is_overworld, 0) = 1 OR pw.destination_map_id = $2 THEN 0 ELSE 1 END, pw.id
		LIMIT 1
	`, mapID, UnifiedOverworldMapID).Scan(&destMapID, &destX, &destY)
	if err == sql.ErrNoRows {
		return 0, 0, 0, fmt.Errorf("Can't use that here")
	}
	if err != nil {
		return 0, 0, 0, fmt.Errorf("Failed to find an exit")
	}
	return destMapID, destX, destY, nil
}

func isEscapeRopeBlockedOnMap(mapID int) bool {
	if mapID == UnifiedOverworldMapID {
		return true
	}
	if db.GlobalWorldDB == nil || db.GlobalWorldDB.DB == nil {
		return false
	}

	var isOverworld int
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT COALESCE(is_overworld, 0)
		FROM phaser_maps
		WHERE id = $1`,
		mapID,
	).Scan(&isOverworld)
	return err == nil && isOverworld != 0
}

func currentTilePosition(ses *session.Session, wh *WorldHandler) (int, int, int) {
	charID := 0
	if ses.HasValidClient() && ses.Client.CharData() != nil {
		char := ses.Client.CharData()
		charID = int(char.ID)
	}
	if charID > 0 && wh != nil && wh.PlayerMovement != nil {
		if x, y, mapID, ok := wh.PlayerMovement.GetPosition(charID); ok {
			return x, y, mapID
		}
	}
	if ses.HasValidClient() && ses.Client.CharData() != nil {
		char := ses.Client.CharData()
		return int(math.Round(char.X)), int(math.Round(char.Y)), int(char.MapID)
	}
	return int(math.Round(float64(ses.X))), int(math.Round(float64(ses.Y))), ses.MapID
}

func teleportPlayerTo(ses *session.Session, wh *WorldHandler, mapID int, x int, y int) {
	if ses != nil && ses.HasValidClient() {
		setServerTeleportedPlayerPosition(ses, wh, mapID, x, y, "DOWN")
	}
	ses.SendStreamJSON(map[string]interface{}{
		"mapId": mapID,
		"x":     x,
		"y":     y,
	}, opcodes.WarpTileTeleportNotify)
}

func sendCQItemUseSuccess(ses *session.Session, found *cqitems.CQInventoryItem, message string, newQty uint16, extra ...map[string]interface{}) {
	payload := map[string]interface{}{
		"success":    true,
		"message":    message,
		"instanceId": found.Instance.ID,
		"newQty":     newQty,
	}
	for _, fields := range extra {
		for key, value := range fields {
			payload[key] = value
		}
	}
	ses.SendStreamJSON(payload, opcodes.CQItemUseResponse)
}

func sendCQItemUseError(ses *session.Session, err string) {
	ses.SendStreamJSON(map[string]interface{}{
		"success": false,
		"error":   err,
	}, opcodes.CQItemUseResponse)
}
