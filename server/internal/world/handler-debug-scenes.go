package world

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

// DebugScene is the UI-facing summary of a script simulator scenario.
type DebugScene struct {
	SeqNum       int    `json:"seqNum"`
	Label        string `json:"label"`
	Description  string `json:"description"`
	ScenarioName string `json:"scenarioName"`
	ScenarioJSON string `json:"scenarioJson,omitempty"`
	TriggerType  string `json:"triggerType"`
	MapName      string `json:"mapName"`
	ScriptLabel  string `json:"scriptLabel,omitempty"`
	Category     string `json:"category,omitempty"`
}

type debugScenarioFile struct {
	SeqNum   int
	Path     string
	RawJSON  string
	Scenario debugScenario
}

type debugScenario struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Fixture     debugFixture         `json:"fixture"`
	Trigger     debugTrigger         `json:"trigger"`
	Expect      debugExpectedOutcome `json:"expect"`
}

type debugFixture struct {
	CharacterName     string                       `json:"characterName"`
	MapName           string                       `json:"mapName"`
	X                 int                          `json:"x"`
	Y                 int                          `json:"y"`
	Direction         string                       `json:"direction"`
	Flags             []string                     `json:"flags"`
	Party             []debugFixturePokemon        `json:"party"`
	PC                []debugFixturePCPokemon      `json:"pc,omitempty"`
	Inventory         []debugFixtureItem           `json:"inventory"`
	Money             int                          `json:"money"`
	Coins             int                          `json:"coins"`
	PokedexSeen       []int                        `json:"pokedexSeen"`
	PokedexCaught     []int                        `json:"pokedexCaught"`
	HiddenObjects     []string                     `json:"hiddenObjects"`
	ObjectPositions   []debugFixtureObjectPosition `json:"objectPositions,omitempty"`
	ActiveBattle      *debugFixtureActiveBattle    `json:"activeBattle,omitempty"`
	Safari            *debugSafariFixture          `json:"safari,omitempty"`
	VermilionGymTrash *debugVermilionGymTrash      `json:"vermilionGymTrash,omitempty"`
}

type debugFixturePokemon struct {
	SpeciesID int      `json:"speciesId"`
	Level     int      `json:"level"`
	MoveIDs   []int    `json:"moveIds,omitempty"`
	Moves     []string `json:"moves,omitempty"`
}

type debugFixturePCPokemon struct {
	SpeciesID int `json:"speciesId"`
	Level     int `json:"level"`
	Box       int `json:"box"`
	Slot      int `json:"slot"`
}

type debugFixtureItem struct {
	ItemID   int    `json:"itemId"`
	ItemName string `json:"itemName"`
	Quantity int    `json:"quantity"`
}

type debugFixtureObjectPosition struct {
	MapName    string `json:"mapName,omitempty"`
	ObjectKey  string `json:"objectKey,omitempty"`
	ObjectName string `json:"objectName,omitempty"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
}

type debugFixtureActiveBattle struct {
	Type             string          `json:"type"`
	PokemonID        int             `json:"pokemonId,omitempty"`
	Level            int             `json:"level,omitempty"`
	TrainerClass     string          `json:"trainerClass,omitempty"`
	PartyIndex       int             `json:"partyIndex,omitempty"`
	TrainerName      string          `json:"trainerName,omitempty"`
	WinFlag          string          `json:"winFlag,omitempty"`
	LoseFlag         string          `json:"loseFlag,omitempty"`
	LossMessage      string          `json:"lossMessage,omitempty"`
	NoBlackoutOnLoss bool            `json:"noBlackoutOnLoss,omitempty"`
	PostWinMapName   string          `json:"postWinMapName,omitempty"`
	PostWinActions   json.RawMessage `json:"postWinActions,omitempty"`
	PostLoseMapName  string          `json:"postLoseMapName,omitempty"`
	PostLoseActions  json.RawMessage `json:"postLoseActions,omitempty"`
	AllowedActions   []string        `json:"allowedActions,omitempty"`
	GuaranteedCatch  bool            `json:"guaranteedCatch,omitempty"`
}

type debugSafariFixture struct {
	Active    bool                      `json:"active"`
	BallsLeft int                       `json:"ballsLeft"`
	StepsLeft int                       `json:"stepsLeft"`
	Battle    *debugSafariBattleFixture `json:"battle,omitempty"`
}

type debugSafariBattleFixture struct {
	PokemonID int `json:"pokemonId"`
	Level     int `json:"level"`
}

type debugVermilionGymTrash struct {
	FirstLockCanIndex  *int `json:"firstLockCanIndex,omitempty"`
	SecondLockCanIndex *int `json:"secondLockCanIndex,omitempty"`
}

type debugTrigger struct {
	Type         string `json:"type"`
	ScriptLabel  string `json:"scriptLabel"`
	MapName      string `json:"mapName"`
	ObjectID     int    `json:"objectId"`
	ObjectKey    string `json:"objectKey"`
	TextConstant string `json:"textConstant"`
	X            int    `json:"x"`
	Y            int    `json:"y"`
	DestX        int    `json:"destX,omitempty"`
	DestY        int    `json:"destY,omitempty"`
}

type debugExpectedOutcome struct {
	ScriptLabel string `json:"scriptLabel"`
}

// HandleDebugSceneListRequest sends the ordered script scenario list to the client.
func HandleDebugSceneListRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	scenarios, err := loadDebugScenarioFiles()
	if err != nil {
		log.Printf("[DebugScene] Failed to load scenarios: %v", err)
		ses.SendStreamJSON(map[string]interface{}{
			"scenes": []DebugScene{},
			"error":  err.Error(),
		}, opcodes.DebugSceneListResponse)
		return false
	}

	scenes := make([]DebugScene, 0, len(scenarios))
	for _, scenario := range scenarios {
		scenes = append(scenes, debugSceneSummary(scenario))
	}
	ses.SendStreamJSON(map[string]interface{}{"scenes": scenes}, opcodes.DebugSceneListResponse)
	return false
}

// HandleDebugSceneJumpRequest applies one script simulator fixture to the current character and warps there.
func HandleDebugSceneJumpRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req struct {
		SeqNum       int    `json:"seqNum"`
		ScenarioName string `json:"scenarioName"`
		ResetAll     bool   `json:"resetAll"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[DebugScene] Invalid jump request: %v", err)
		return false
	}
	if !ses.HasValidClient() {
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	if req.ResetAll {
		log.Printf("[DebugScene] Resetting player %d to fresh-world baseline", charID)
		mapID, x, y, err := resetDebugCharacterToFreshStart(charID, wh)
		if err != nil {
			log.Printf("[DebugScene] Failed to reset player %d: %v", charID, err)
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			}, opcodes.DebugSceneJumpResponse)
			return false
		}

		sendDebugSceneTeleport(ses, wh, charID, mapID, x, y, DefaultSpawnDirection)
		sendDebugFreshStartHiddenActorDespawns(ses, wh, "REDS_HOUSE_2F")
		sendDebugFixtureSnapshots(ses, charID)
		ses.SendStreamJSON(map[string]interface{}{
			"success":      true,
			"label":        "Reset all",
			"scenarioName": "fresh_world_reset",
			"triggerType":  "reset",
			"mapName":      "REDS_HOUSE_2F",
		}, opcodes.DebugSceneJumpResponse)
		return false
	}

	scenario, scene, err := findDebugScenario(req.SeqNum, req.ScenarioName)
	if err != nil {
		log.Printf("[DebugScene] %v", err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, opcodes.DebugSceneJumpResponse)
		return false
	}

	log.Printf("[DebugScene] Applying scenario %s to player %d", scenario.Scenario.Name, charID)
	mapID, x, y, err := applyDebugScenarioFixture(charID, scenario.Scenario.Fixture, wh)
	if err != nil {
		log.Printf("[DebugScene] Failed to apply %s: %v", scenario.Scenario.Name, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, opcodes.DebugSceneJumpResponse)
		return false
	}

	sendDebugSceneTeleport(ses, wh, charID, mapID, x, y, scenario.Scenario.Fixture.Direction)
	sendDebugFixtureSnapshots(ses, charID)
	sendDebugSafariState(ses, charID, wh, mapID)

	ses.SendStreamJSON(map[string]interface{}{
		"success":      true,
		"label":        scene.Label,
		"scenarioName": scene.ScenarioName,
		"triggerType":  scene.TriggerType,
		"mapName":      scene.MapName,
		"scriptLabel":  scene.ScriptLabel,
	}, opcodes.DebugSceneJumpResponse)

	if err := sendDebugActiveBattle(ses, charID, scenario.Scenario.Fixture.ActiveBattle); err != nil {
		log.Printf("[DebugScene] Failed to start active battle for %s: %v", scenario.Scenario.Name, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, opcodes.PokeBattleStartResponse)
	}

	return false
}

func sendDebugFixtureSnapshots(ses *session.Session, charID int64) {
	sendPCUpdate(ses, charID, debugCurrentPCBox(charID))
	sendCQInventorySnapshot(ses, int32(charID))
}

// HandleDebugGivePowerPokemonRequest gives the current debug player a
// deliberately overpowered test Pokémon for quickly clearing battle blockers.
func HandleDebugGivePowerPokemonRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	const (
		debugPowerPokemonSpeciesID = 150 // Mewtwo
		debugPowerPokemonLevel     = 100
		maxGen1StatExp             = 65535
	)

	charID := int64(ses.Client.CharData().ID)
	pokemon, err := pokebattle.BuildWildPokemon(db.GlobalWorldDB.DB, debugPowerPokemonSpeciesID, debugPowerPokemonLevel)
	if err != nil {
		log.Printf("[DebugScene] Failed building power Pokémon for char %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, opcodes.DebugGivePowerPokemonResponse)
		return false
	}
	pokemon.IVs = pokebattle.IVs{Attack: 15, Defense: 15, Speed: 15, Special: 15}
	pokemon.EVs = pokebattle.EVs{
		HP:      maxGen1StatExp,
		Attack:  maxGen1StatExp,
		Defense: maxGen1StatExp,
		Speed:   maxGen1StatExp,
		Special: maxGen1StatExp,
	}
	pokemon.RecalculateStats()
	pokemon.CurHP = pokemon.MaxHP

	addedToParty, box, slot, err := pokebattle.SavePreparedPokemonToPartyOrPC(db.GlobalWorldDB.DB, charID, pokemon)
	if err != nil {
		log.Printf("[DebugScene] Failed saving power Pokémon for char %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, opcodes.DebugGivePowerPokemonResponse)
		return false
	}

	location := "party"
	if !addedToParty {
		location = "pc"
	}
	refreshBox := box
	if refreshBox < 0 {
		refreshBox = debugCurrentPCBox(charID)
	}
	sendPCUpdate(ses, charID, refreshBox)
	ses.SendStreamJSON(map[string]interface{}{
		"success":      true,
		"speciesId":    debugPowerPokemonSpeciesID,
		"name":         pokemon.Name,
		"level":        pokemon.Level,
		"addedToParty": addedToParty,
		"box":          box,
		"slot":         slot,
		"location":     location,
		"message":      debugPowerPokemonMessage(pokemon.Name, pokemon.Level, addedToParty, box, slot),
	}, opcodes.DebugGivePowerPokemonResponse)

	return false
}

func debugCurrentPCBox(charID int64) int {
	var currentBox int
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT current_box FROM character_pc_state WHERE character_id = $1`,
		charID,
	).Scan(&currentBox); err != nil {
		return 0
	}
	return currentBox
}

func debugPowerPokemonMessage(name string, level int, addedToParty bool, box, slot int) string {
	if addedToParty {
		return fmt.Sprintf("Added %s L%d to party.", name, level)
	}
	return fmt.Sprintf("Party full; sent %s L%d to PC box %d slot %d.", name, level, box+1, slot+1)
}

func sendDebugSceneTeleport(ses *session.Session, wh *WorldHandler, charID int64, mapID, x, y int, direction string) {
	if direction == "" {
		direction = "DOWN"
	}
	setServerTeleportedPlayerPosition(ses, wh, mapID, x, y, direction)

	ses.SendStreamJSON(map[string]interface{}{
		"mapId":     mapID,
		"x":         x,
		"y":         y,
		"direction": direction,
	}, opcodes.WarpTileTeleportNotify)
}

func sendDebugFreshStartHiddenActorDespawns(ses *session.Session, wh *WorldHandler, mapName string) {
	if wh == nil || wh.ActorRegistry == nil {
		return
	}
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT po.id
		FROM phaser_objects po
		JOIN phaser_maps pm ON pm.id = po.map_id
		JOIN phaser_event_object_visibility ev
			ON ev.map_id = po.map_id AND ev.object_name = po.name
		WHERE pm.name = $1
		  AND ev.visible = FALSE
		  AND COALESCE(ev.requires_flag, '') = ''
		  AND COALESCE(ev.requires_flag_absent, '') = ''`, mapName)
	if err != nil {
		log.Printf("[DebugScene] Failed to query fresh-start hidden actors for %s: %v", mapName, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var objectID int
		if err := rows.Scan(&objectID); err != nil {
			log.Printf("[DebugScene] Failed to scan fresh-start hidden actor for %s: %v", mapName, err)
			continue
		}
		ses.SendStreamJSON(map[string]interface{}{
			"id": wh.ActorRegistry.GetPhaserID(ActorTypeNPC, objectID),
		}, opcodes.PhaserActorDespawn)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[DebugScene] Failed reading fresh-start hidden actors for %s: %v", mapName, err)
	}
}

func findDebugScenario(seqNum int, scenarioName string) (*debugScenarioFile, DebugScene, error) {
	scenarios, err := loadDebugScenarioFiles()
	if err != nil {
		return nil, DebugScene{}, err
	}
	for _, scenario := range scenarios {
		if seqNum > 0 && scenario.SeqNum == seqNum {
			scene := debugSceneSummary(scenario)
			return &scenario, scene, nil
		}
		if scenarioName != "" && scenario.Scenario.Name == scenarioName {
			scene := debugSceneSummary(scenario)
			return &scenario, scene, nil
		}
	}
	if scenarioName != "" {
		return nil, DebugScene{}, fmt.Errorf("unknown scenario %s", scenarioName)
	}
	return nil, DebugScene{}, fmt.Errorf("unknown scene seqNum %d", seqNum)
}

func loadDebugScenarioFiles() ([]debugScenarioFile, error) {
	dir, err := debugScenarioDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read scenario dir %s: %w", dir, err)
	}

	files := make([]debugScenarioFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read scenario %s: %w", path, err)
		}
		var scenario debugScenario
		if err := json.Unmarshal(data, &scenario); err != nil {
			return nil, fmt.Errorf("parse scenario %s: %w", path, err)
		}
		if scenario.Name == "" {
			scenario.Name = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		}
		files = append(files, debugScenarioFile{Path: path, RawJSON: string(data), Scenario: scenario})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Scenario.Name < files[j].Scenario.Name
	})
	for i := range files {
		files[i].SeqNum = i + 1
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no script scenarios found in %s", dir)
	}
	return files, nil
}

func debugScenarioDir() (string, error) {
	candidates := []string{
		filepath.Join("script_tests", "scenarios"),
		filepath.Join("server", "script_tests", "scenarios"),
		filepath.Join("..", "server", "script_tests", "scenarios"),
		filepath.Join("..", "script_tests", "scenarios"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("script_tests/scenarios directory not found")
}

func debugSceneSummary(file debugScenarioFile) DebugScene {
	s := file.Scenario
	mapName := s.Trigger.MapName
	if mapName == "" {
		mapName = s.Fixture.MapName
	}
	scriptLabel := s.Expect.ScriptLabel
	if scriptLabel == "" {
		scriptLabel = s.Trigger.ScriptLabel
	}
	description := s.Description
	if description == "" {
		description = fmt.Sprintf("%s on %s", s.Trigger.Type, mapName)
	}
	return DebugScene{
		SeqNum:       file.SeqNum,
		Label:        s.Name,
		Description:  description,
		ScenarioName: s.Name,
		ScenarioJSON: file.RawJSON,
		TriggerType:  s.Trigger.Type,
		MapName:      mapName,
		ScriptLabel:  scriptLabel,
		Category:     debugSceneCategory(s, scriptLabel),
	}
}

func debugSceneCategory(s debugScenario, scriptLabel string) string {
	text := strings.ToLower(strings.Join([]string{
		s.Name,
		s.Description,
		s.Trigger.Type,
		s.Trigger.TextConstant,
		scriptLabel,
	}, " "))
	if strings.Contains(text, "trade") ||
		s.Trigger.TextConstant == route18Gate2FYoungsterTextConstant {
		return "trade"
	}
	if s.Trigger.Type == "field_move_permission" {
		return "field"
	}
	return ""
}

func applyDebugScenarioFixture(charID int64, f debugFixture, wh *WorldHandler) (int, int, int, error) {
	if f.MapName == "" {
		return 0, 0, 0, fmt.Errorf("fixture mapName is required")
	}
	mapID, x, y, err := debugMapIDAndCoordinatesForName(f.MapName, f.X, f.Y)
	if err != nil {
		return 0, 0, 0, err
	}
	if err := resetDebugCharacterState(charID, wh); err != nil {
		return 0, 0, 0, err
	}
	if _, err := db.GlobalWorldDB.DB.Exec(
		`UPDATE character_data SET map_id = $1, x = $2, y = $3 WHERE id = $4`,
		mapID, x, y, charID); err != nil {
		return 0, 0, 0, fmt.Errorf("set debug fixture position: %w", err)
	}
	if err := seedDebugFlags(charID, f.Flags, wh); err != nil {
		return 0, 0, 0, err
	}
	for _, pokemon := range f.Party {
		level := pokemon.Level
		if level <= 0 {
			level = 5
		}
		if _, _, _, err := pokebattle.AddPokemonToPartyOrPC(db.GlobalWorldDB.DB, charID, pokemon.SpeciesID, level); err != nil {
			return 0, 0, 0, fmt.Errorf("seed pokemon %d L%d: %w", pokemon.SpeciesID, level, err)
		}
		if err := seedDebugPokemonMoves(charID, pokemon); err != nil {
			return 0, 0, 0, err
		}
	}
	for _, pokemon := range f.PC {
		if err := seedDebugPCPokemon(charID, pokemon); err != nil {
			return 0, 0, 0, err
		}
	}
	for _, item := range f.Inventory {
		itemID, err := debugFixtureItemID(item)
		if err != nil {
			return 0, 0, 0, err
		}
		quantity := item.Quantity
		if quantity <= 0 {
			quantity = 1
		}
		if _, err := cqitems.AddItemToInventory(int32(charID), int32(itemID), uint16(quantity)); err != nil {
			return 0, 0, 0, fmt.Errorf("seed item %d x%d: %w", itemID, quantity, err)
		}
	}
	if f.Money > 0 {
		if _, err := db.GlobalWorldDB.DB.Exec(`
			INSERT INTO character_wallet (character_id, pokedollars)
			VALUES ($1, $2)
			ON CONFLICT (character_id) DO UPDATE SET
				pokedollars = EXCLUDED.pokedollars`,
			charID, f.Money); err != nil {
			return 0, 0, 0, fmt.Errorf("seed money %d: %w", f.Money, err)
		}
	}
	if f.Coins > 0 {
		if _, err := db.GlobalWorldDB.DB.Exec(`
			INSERT INTO character_coins (character_id, coins)
			VALUES ($1, $2)
			ON CONFLICT (character_id) DO UPDATE SET coins = EXCLUDED.coins`,
			charID, f.Coins); err != nil {
			return 0, 0, 0, fmt.Errorf("seed coins %d: %w", f.Coins, err)
		}
	}
	for _, pokemonID := range f.PokedexSeen {
		if err := seedDebugPokedexEntry(charID, pokemonID, false); err != nil {
			return 0, 0, 0, err
		}
	}
	for _, pokemonID := range f.PokedexCaught {
		if err := seedDebugPokedexEntry(charID, pokemonID, true); err != nil {
			return 0, 0, 0, err
		}
	}
	for _, key := range f.HiddenObjects {
		ids, err := debugObjectIDsByKey(f.MapName, key)
		if err != nil {
			return 0, 0, 0, err
		}
		for _, id := range ids {
			if _, err := db.GlobalWorldDB.DB.Exec(
				`INSERT INTO character_collected_items (character_id, object_id)
				VALUES ($1, $2)
				ON CONFLICT (character_id, object_id) DO NOTHING`,
				charID, id); err != nil {
				return 0, 0, 0, fmt.Errorf("seed hidden object %s/%d: %w", key, id, err)
			}
		}
	}
	for _, pos := range f.ObjectPositions {
		if err := seedDebugObjectPosition(charID, f.MapName, pos); err != nil {
			return 0, 0, 0, err
		}
	}
	if f.VermilionGymTrash != nil {
		if err := seedDebugVermilionGymTrash(charID, f.VermilionGymTrash); err != nil {
			return 0, 0, 0, err
		}
	}
	if err := seedDebugSafariSession(charID, f.Safari, wh); err != nil {
		return 0, 0, 0, err
	}
	return mapID, x, y, nil
}

func seedDebugPCPokemon(charID int64, fixture debugFixturePCPokemon) error {
	if fixture.SpeciesID <= 0 {
		return fmt.Errorf("pc pokemon fixture requires speciesId")
	}
	level := fixture.Level
	if level <= 0 {
		level = 5
	}
	pokemon, err := pokebattle.BuildWildPokemon(db.GlobalWorldDB.DB, fixture.SpeciesID, level)
	if err != nil {
		return fmt.Errorf("build PC pokemon %d L%d: %w", fixture.SpeciesID, level, err)
	}
	pokemon.IsWild = false
	if err := pokebattle.SavePokemonToPCSlot(db.GlobalWorldDB.DB, charID, fixture.Box, fixture.Slot, pokemon); err != nil {
		return fmt.Errorf("seed PC pokemon %d box=%d slot=%d: %w", fixture.SpeciesID, fixture.Box, fixture.Slot, err)
	}
	return nil
}

func seedDebugSafariSession(charID int64, fixture *debugSafariFixture, wh *WorldHandler) error {
	if fixture == nil {
		return nil
	}
	if wh == nil || wh.Safari == nil {
		return fmt.Errorf("safari fixture requires world safari manager")
	}
	session := SafariSession{
		Active:    fixture.Active,
		BallsLeft: fixture.BallsLeft,
		StepsLeft: fixture.StepsLeft,
	}
	if fixture.Battle != nil {
		if !fixture.Active {
			return fmt.Errorf("safari battle fixture requires active safari session")
		}
		wild, err := pokebattle.BuildWildPokemon(db.GlobalWorldDB.DB, fixture.Battle.PokemonID, fixture.Battle.Level)
		if err != nil {
			return fmt.Errorf("build safari battle pokemon #%d L%d: %w", fixture.Battle.PokemonID, fixture.Battle.Level, err)
		}
		session.Battle = pokebattle.NewSafariBattle(wild, fixture.BallsLeft, fixture.StepsLeft)
	}
	wh.Safari.SetSession(charID, session)
	return nil
}

func sendDebugSafariState(ses *session.Session, charID int64, wh *WorldHandler, mapID int) {
	if wh == nil || wh.Safari == nil {
		return
	}
	session := wh.Safari.GetSession(charID)
	if session == nil || !session.Active {
		if IsInSafariZone(mapID) {
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"message": "no active safari session",
			}, opcodes.SafariZoneEnterResponse)
		}
		return
	}

	ses.SendStreamJSON(map[string]interface{}{
		"success":   true,
		"ballsLeft": session.BallsLeft,
		"stepsLeft": session.StepsLeft,
	}, opcodes.SafariZoneEnterResponse)
	ses.SendStreamJSON(map[string]interface{}{
		"stepsLeft": session.StepsLeft,
		"ballsLeft": session.BallsLeft,
	}, opcodes.SafariZoneStepUpdate)

	if session.Battle == nil || session.Battle.IsOver() {
		return
	}
	wild := session.Battle.WildPokemon
	ses.SendStreamJSON(map[string]interface{}{
		"pokemon": map[string]interface{}{
			"id":        wild.ID,
			"name":      wild.Name,
			"level":     wild.Level,
			"hp":        wild.CurHP,
			"maxHp":     wild.MaxHP,
			"spriteId":  wild.ID,
			"catchRate": wild.CatchRate,
		},
		"ballsLeft": session.BallsLeft,
		"stepsLeft": session.StepsLeft,
	}, opcodes.SafariBattleStartNotify)
}

func sendDebugActiveBattle(ses *session.Session, charID int64, fixture *debugFixtureActiveBattle) error {
	if fixture == nil {
		return nil
	}

	var (
		battle *pokebattle.BattleState
		events []pokebattle.BattleEvent
		err    error
	)

	switch fixture.Type {
	case "wild":
		if fixture.PokemonID <= 0 || fixture.Level <= 0 {
			return fmt.Errorf("activeBattle wild fixture requires pokemonId and level")
		}
		battle, events, err = StartScriptedWildBattle(charID, ScriptedWildBattleSpec{
			PokemonID:       fixture.PokemonID,
			Level:           fixture.Level,
			WinFlag:         fixture.WinFlag,
			PostWinMapName:  fixture.PostWinMapName,
			PostWinActions:  fixture.PostWinActions,
			AllowedActions:  fixture.AllowedActions,
			GuaranteedCatch: fixture.GuaranteedCatch,
		})
	case "trainer":
		if fixture.TrainerClass == "" || fixture.PartyIndex <= 0 {
			return fmt.Errorf("activeBattle trainer fixture requires trainerClass and partyIndex")
		}
		battle, events, err = StartScriptedTrainerBattle(charID, ScriptedTrainerBattleSpec{
			TrainerClass:     fixture.TrainerClass,
			PartyIndex:       fixture.PartyIndex,
			TrainerName:      fixture.TrainerName,
			WinFlag:          fixture.WinFlag,
			LoseFlag:         fixture.LoseFlag,
			LossMessage:      fixture.LossMessage,
			NoBlackoutOnLoss: fixture.NoBlackoutOnLoss,
			PostWinMapName:   fixture.PostWinMapName,
			PostWinActions:   fixture.PostWinActions,
			PostLoseMapName:  fixture.PostLoseMapName,
			PostLoseActions:  fixture.PostLoseActions,
		})
	default:
		return fmt.Errorf("unsupported activeBattle fixture type %q", fixture.Type)
	}
	if err != nil {
		return fmt.Errorf("seed %s active battle: %w", fixture.Type, err)
	}

	resp := buildBattleStateResponse(battle)
	resp["events"] = events
	if battle.Trainer != nil {
		resp["trainerClass"] = battle.Trainer.ClassName
		resp["trainerName"] = battle.Trainer.Name
	}
	ses.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)
	return nil
}

func resetDebugCharacterToFreshStart(charID int64, wh *WorldHandler) (int, int, int, error) {
	if err := resetDebugCharacterState(charID, wh); err != nil {
		return 0, 0, 0, err
	}

	storedMapID := DefaultSpawnMap
	x := int(DefaultSpawnX)
	y := int(DefaultSpawnY)
	if _, err := db.GlobalWorldDB.DB.Exec(`
		UPDATE character_data
		SET map_id = $1,
		    x = $2,
		    y = $3,
		    z = $4,
		    heading = 0
		WHERE id = $5`,
		storedMapID,
		DefaultSpawnX,
		DefaultSpawnY,
		DefaultSpawnZ,
		charID,
	); err != nil {
		return 0, 0, 0, fmt.Errorf("reset character baseline: %w", err)
	}

	if _, err := db.GlobalWorldDB.DB.Exec(`
		DELETE FROM character_bind
		WHERE id = $1`, charID); err != nil {
		return 0, 0, 0, fmt.Errorf("reset character binds: %w", err)
	}
	for slot := 0; slot < 5; slot++ {
		if _, err := db.GlobalWorldDB.DB.Exec(`
			INSERT INTO character_bind (id, map_id, x, y, z, heading, slot)
			VALUES ($1, $2, $3, $4, $5, 0, $6)`,
			charID,
			storedMapID,
			DefaultSpawnX,
			DefaultSpawnY,
			DefaultSpawnZ,
			slot,
		); err != nil {
			return 0, 0, 0, fmt.Errorf("seed fresh character bind %d: %w", slot, err)
		}
	}

	return storedMapID, x, y, nil
}

func resetDebugCharacterState(charID int64, wh *WorldHandler) error {
	ClearBattleForCharacter(charID)
	if wh != nil && wh.Safari != nil {
		wh.Safari.EndSession(charID)
	}
	if wh != nil && wh.EventFlags != nil {
		for _, flag := range wh.EventFlags.GetAllFlags(charID) {
			if err := wh.EventFlags.ResetFlag(charID, flag); err != nil {
				return fmt.Errorf("reset debug flag %s: %w", flag, err)
			}
		}
	}
	statements := []string{
		`DELETE FROM character_defeated_trainers WHERE character_id = $1`,
		`DELETE FROM character_event_flags WHERE character_id = $1`,
		`DELETE FROM character_in_game_trades WHERE character_id = $1`,
		`DELETE FROM character_pokemon WHERE character_id = $1`,
		`DELETE FROM character_pc_state WHERE character_id = $1`,
		`DELETE FROM character_field_move_state WHERE character_id = $1`,
		`DELETE FROM character_object_positions WHERE character_id = $1`,
		`DELETE FROM character_object_visibility_overrides WHERE character_id = $1`,
		`DELETE FROM character_collected_items WHERE character_id = $1`,
		`DELETE FROM character_collected_hidden_coins WHERE character_id = $1`,
		`DELETE FROM character_pokedex WHERE character_id = $1`,
		`DELETE FROM character_coins WHERE character_id = $1`,
		`DELETE FROM character_vermilion_gym_trash_state WHERE character_id = $1`,
		`DELETE FROM character_wallet WHERE character_id = $1`,
		`DELETE FROM cq_item_instances WHERE id IN (SELECT item_instance_id FROM cq_character_inventory WHERE character_id = $1)`,
		`DELETE FROM cq_character_inventory WHERE character_id = $1`,
		`DELETE FROM character_battle_state WHERE character_id = $1`,
	}
	for _, stmt := range statements {
		if _, err := db.GlobalWorldDB.DB.Exec(stmt, charID); err != nil {
			return fmt.Errorf("reset debug fixture state: %w", err)
		}
	}
	return nil
}

func seedDebugFlags(charID int64, flags []string, wh *WorldHandler) error {
	if len(flags) == 0 {
		return nil
	}
	if wh != nil && wh.EventFlags != nil {
		return wh.EventFlags.SetFlagBatch(charID, flags)
	}
	for _, flag := range flags {
		if _, err := db.GlobalWorldDB.DB.Exec(
			`INSERT INTO character_event_flags (character_id, flag_name)
			VALUES ($1, $2)
			ON CONFLICT (character_id, flag_name) DO NOTHING`,
			charID, flag); err != nil {
			return fmt.Errorf("seed flag %s: %w", flag, err)
		}
	}
	return nil
}

func debugFixtureItemID(item debugFixtureItem) (int, error) {
	if item.ItemID > 0 {
		return item.ItemID, nil
	}
	if item.ItemName == "" {
		return 0, fmt.Errorf("fixture item requires itemId or itemName")
	}
	var id int
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT id FROM cq_items WHERE name = $1 OR short_name = $2 LIMIT 1`,
		item.ItemName, item.ItemName).Scan(&id); err != nil {
		return 0, fmt.Errorf("lookup item %s: %w", item.ItemName, err)
	}
	return id, nil
}

func seedDebugPokemonMoves(charID int64, pokemon debugFixturePokemon) error {
	if len(pokemon.MoveIDs) == 0 && len(pokemon.Moves) == 0 {
		return nil
	}
	moveIDs := append([]int{}, pokemon.MoveIDs...)
	for _, moveName := range pokemon.Moves {
		moveID, err := debugMoveID(moveName)
		if err != nil {
			return err
		}
		moveIDs = append(moveIDs, moveID)
	}
	if len(moveIDs) > 4 {
		return fmt.Errorf("pokemon fixture supports at most 4 moves, got %d", len(moveIDs))
	}

	var pokemonRowID int64
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT id FROM character_pokemon WHERE character_id = $1 ORDER BY id DESC LIMIT 1`,
		charID).Scan(&pokemonRowID); err != nil {
		return fmt.Errorf("lookup seeded pokemon row: %w", err)
	}

	var ids [4]int
	var pps [4]int
	for i, moveID := range moveIDs {
		pp, err := debugMovePP(moveID)
		if err != nil {
			return err
		}
		ids[i] = moveID
		pps[i] = pp
	}
	if _, err := db.GlobalWorldDB.DB.Exec(`
		UPDATE character_pokemon
		SET move1_id = $1, move1_pp = $2,
		    move2_id = $3, move2_pp = $4,
		    move3_id = $5, move3_pp = $6,
		    move4_id = $7, move4_pp = $8
		WHERE id = $9`,
		ids[0], pps[0],
		ids[1], pps[1],
		ids[2], pps[2],
		ids[3], pps[3],
		pokemonRowID); err != nil {
		return fmt.Errorf("update seeded pokemon moves: %w", err)
	}
	return nil
}

func debugMoveID(moveName string) (int, error) {
	normalized := debugNormalizeName(moveName)
	if normalized == "" {
		return 0, fmt.Errorf("move name is required")
	}
	var id int
	if err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT id
		FROM phaser_moves
		WHERE REPLACE(REPLACE(UPPER(name), '-', ''), ' ', '') = $1
		   OR REPLACE(REPLACE(UPPER(short_name), '-', ''), ' ', '') = $2
		LIMIT 1`, normalized, normalized).Scan(&id); err != nil {
		return 0, fmt.Errorf("lookup move %s: %w", moveName, err)
	}
	return id, nil
}

func debugMovePP(moveID int) (int, error) {
	var pp int
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT COALESCE(pp, 0) FROM phaser_moves WHERE id = $1`,
		moveID).Scan(&pp); err != nil {
		return 0, fmt.Errorf("lookup move %d pp: %w", moveID, err)
	}
	return pp, nil
}

func debugNormalizeName(name string) string {
	var b strings.Builder
	for _, ch := range name {
		switch {
		case ch >= 'a' && ch <= 'z':
			b.WriteRune(ch - 32)
		case (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9'):
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func seedDebugPokedexEntry(charID int64, pokemonID int, caught bool) error {
	if pokemonID <= 0 {
		return fmt.Errorf("seed pokedex pokemon id must be positive")
	}
	if caught {
		if _, err := db.GlobalWorldDB.DB.Exec(`
			INSERT INTO character_pokedex (character_id, pokemon_id, seen, caught, first_seen_at, first_caught_at)
			VALUES ($1, $2, 1, 1, NOW(), NOW())
			ON CONFLICT (character_id, pokemon_id) DO UPDATE SET
				seen = 1,
				caught = 1,
				first_seen_at = COALESCE(character_pokedex.first_seen_at, EXCLUDED.first_seen_at),
				first_caught_at = COALESCE(character_pokedex.first_caught_at, EXCLUDED.first_caught_at)`,
			charID, pokemonID); err != nil {
			return fmt.Errorf("seed caught pokedex %d: %w", pokemonID, err)
		}
		return nil
	}
	if _, err := db.GlobalWorldDB.DB.Exec(`
		INSERT INTO character_pokedex (character_id, pokemon_id, seen, caught, first_seen_at)
		VALUES ($1, $2, 1, 0, NOW())
		ON CONFLICT (character_id, pokemon_id) DO UPDATE SET
			seen = 1,
			first_seen_at = COALESCE(character_pokedex.first_seen_at, EXCLUDED.first_seen_at)`,
		charID, pokemonID); err != nil {
		return fmt.Errorf("seed seen pokedex %d: %w", pokemonID, err)
	}
	return nil
}

func seedDebugObjectPosition(charID int64, defaultMapName string, pos debugFixtureObjectPosition) error {
	mapName := pos.MapName
	if mapName == "" {
		mapName = defaultMapName
	}
	mapID, x, y, err := debugMapIDAndCoordinatesForName(mapName, pos.X, pos.Y)
	if err != nil {
		return err
	}
	objectID, err := debugObjectPositionID(mapName, pos)
	if err != nil {
		return err
	}
	if _, err := db.GlobalWorldDB.DB.Exec(`
		INSERT INTO character_object_positions (character_id, object_id, map_id, x, y)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (character_id, object_id) DO UPDATE SET
			map_id = EXCLUDED.map_id,
			x = EXCLUDED.x,
			y = EXCLUDED.y`,
		charID, objectID, mapID, x, y); err != nil {
		return fmt.Errorf("seed object position %d: %w", objectID, err)
	}
	return nil
}

func debugObjectPositionID(mapName string, pos debugFixtureObjectPosition) (int, error) {
	if pos.ObjectKey != "" {
		ids, err := debugObjectIDsByKey(mapName, pos.ObjectKey)
		if err != nil {
			return 0, err
		}
		if len(ids) == 0 {
			return 0, fmt.Errorf("object key %s did not resolve on %s", pos.ObjectKey, mapName)
		}
		return ids[0], nil
	}
	if pos.ObjectName == "" {
		return 0, fmt.Errorf("objectPositions requires objectKey or objectName")
	}
	var objectID int
	if err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT po.id
		FROM phaser_objects po
		JOIN phaser_maps pm ON pm.id = po.map_id
		WHERE pm.name = $1 AND po.name = $2
		LIMIT 1`, mapName, pos.ObjectName).Scan(&objectID); err != nil {
		if debugIsOverworldMapName(mapName) {
			if err2 := db.GlobalWorldDB.DB.QueryRow(`
				SELECT id FROM phaser_objects
				WHERE map_id = $1 AND name = $2
				LIMIT 1`, UnifiedOverworldMapID, pos.ObjectName).Scan(&objectID); err2 == nil {
				return objectID, nil
			}
		}
		return 0, fmt.Errorf("lookup object %s/%s: %w", mapName, pos.ObjectName, err)
	}
	return objectID, nil
}

func debugObjectIDsByKey(mapName, key string) ([]int, error) {
	var explicitID int
	if _, err := fmt.Sscanf(key, "object:%d", &explicitID); err == nil && explicitID > 0 {
		return []int{explicitID}, nil
	}
	if _, err := fmt.Sscanf(key, "phaser_object:%d", &explicitID); err == nil && explicitID > 0 {
		return []int{explicitID}, nil
	}

	ids, err := debugQueryObjectIDsByMapName(mapName, key)
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 || !debugIsOverworldMapName(mapName) {
		return ids, nil
	}
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT po.id
		FROM phaser_objects po
		WHERE po.map_id = $1 AND (po.text = $2 OR po.name = $3)
		ORDER BY po.id`, UnifiedOverworldMapID, key, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func debugQueryObjectIDsByMapName(mapName, key string) ([]int, error) {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT po.id
		FROM phaser_objects po
		JOIN phaser_maps pm ON pm.id = po.map_id
		WHERE pm.name = $1 AND (po.text = $2 OR po.name = $3)
		ORDER BY po.id`, mapName, key, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func seedDebugVermilionGymTrash(charID int64, fixture *debugVermilionGymTrash) error {
	if fixture.FirstLockCanIndex == nil {
		return fmt.Errorf("vermilionGymTrash fixture requires firstLockCanIndex")
	}
	var second interface{}
	if fixture.SecondLockCanIndex != nil {
		second = *fixture.SecondLockCanIndex
	}
	if _, err := db.GlobalWorldDB.DB.Exec(`
		INSERT INTO character_vermilion_gym_trash_state
			(character_id, first_lock_can_index, second_lock_can_index)
		VALUES ($1, $2, $3)
		ON CONFLICT (character_id) DO UPDATE SET
			first_lock_can_index = EXCLUDED.first_lock_can_index,
			second_lock_can_index = EXCLUDED.second_lock_can_index,
			updated_at = CURRENT_TIMESTAMP`,
		charID, *fixture.FirstLockCanIndex, second); err != nil {
		return fmt.Errorf("seed Vermilion Gym trash state: %w", err)
	}
	return nil
}

type debugMapInfo struct {
	ID          int
	IsOverworld bool
	Width       int
	Height      int
}

func debugMapIDAndCoordinatesForName(mapName string, x, y int) (int, int, int, error) {
	info, err := debugMapInfoForName(mapName)
	if err != nil {
		return 0, 0, 0, err
	}
	if !info.IsOverworld || info.ID == UnifiedOverworldMapID {
		return info.ID, x, y, nil
	}

	// Script simulator fixtures usually use original per-map coordinates. The
	// client renders the stitched Kanto map, so translate local overworld
	// coordinates into global coordinates. Hand-authored debug fixtures that
	// already use global coordinates usually sit outside the source map bounds
	// and are left untouched.
	if x < 0 || y < 0 {
		return UnifiedOverworldMapID, x, y, nil
	}

	offsetX, offsetY, err := debugOverworldMapOffset(info.ID)
	if err != nil {
		return 0, 0, 0, err
	}
	return UnifiedOverworldMapID, x + offsetX, y + offsetY, nil
}

func debugMapIDForName(mapName string) (int, error) {
	info, err := debugMapInfoForName(mapName)
	if err != nil {
		return 0, err
	}
	if info.IsOverworld && info.ID != UnifiedOverworldMapID {
		return UnifiedOverworldMapID, nil
	}
	return info.ID, nil
}

func debugMapInfoForName(mapName string) (debugMapInfo, error) {
	if mapName == "" {
		return debugMapInfo{}, fmt.Errorf("mapName is required")
	}
	var info debugMapInfo
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT id, is_overworld = 1, width, height FROM phaser_maps WHERE name = $1`, mapName).Scan(
		&info.ID,
		&info.IsOverworld,
		&info.Width,
		&info.Height,
	); err != nil {
		return debugMapInfo{}, fmt.Errorf("lookup map %s: %w", mapName, err)
	}
	return info, nil
}

func debugOverworldMapOffset(mapID int) (int, int, error) {
	var offsetX, offsetY int
	if err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT
			COALESCE(MIN(x) - MIN(local_x), 0),
			COALESCE(MIN(y) - MIN(local_y), 0)
		FROM phaser_tiles
		WHERE source_map_id = $1
		  AND local_x IS NOT NULL
		  AND local_y IS NOT NULL`,
		mapID,
	).Scan(&offsetX, &offsetY); err != nil {
		return 0, 0, fmt.Errorf("lookup overworld offset for map %d: %w", mapID, err)
	}
	return offsetX, offsetY, nil
}

func debugIsOverworldMapName(mapName string) bool {
	if mapName == "" {
		return false
	}
	var isOverworld bool
	err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT is_overworld = 1 FROM phaser_maps WHERE name = $1`, mapName).Scan(&isOverworld)
	return err == nil && isOverworld
}
