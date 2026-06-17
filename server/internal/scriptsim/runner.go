package scriptsim

import (
	"encoding/json"
	"fmt"
	"strings"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/world"
)

type Result struct {
	Scenario      *Scenario
	CharacterID   int64
	Script        *world.CutsceneScript
	Initial       *Snapshot
	Final         *Snapshot
	ActionEffects []ActionEffect
	Elevator      *ElevatorSummary
	GameCorner    *GameCornerSummary
	FieldMove     *FieldMoveSummary
	BoulderPush   *BoulderPushSummary
	Pathfind      *PathfindSummary
	Safari        *SafariSummary
	Repel         *RepelSummary
	Messages      []RecordedMessage
	TileStates    []TileState
	ObjectStates  []ObjectState
	SurfBlocked   *bool
}

type ElevatorSummary struct {
	Floors   []ElevatorFloorSummary
	Message  string
	Selected *ElevatorFloorSummary
}

type ElevatorFloorSummary struct {
	MapID          int
	MapName        string
	Label          string
	DestX          int
	DestY          int
	RequiresFlag   string
	RequiresItemID int
}

type GameCornerSummary struct {
	SlotPlay      *GameCornerSlotSummary
	PrizeList     *GameCornerPrizeListSummary
	PrizePurchase *GameCornerPrizePurchaseSummary
	HiddenCoin    *GameCornerHiddenCoinSummary
}

type GameCornerSlotSummary struct {
	Success       bool
	Message       string
	Bet           int
	Payout        int
	MatchLine     string
	MatchSymbol   string
	Coins         int
	ReelPositions []int
	Reels         [][]string
}

type GameCornerPrizeListSummary struct {
	Success bool
	Message string
	Coins   int
	Prizes  []GameCornerPrizeSummary
}

type GameCornerPrizePurchaseSummary struct {
	Success      bool
	Message      string
	Coins        int
	Prize        GameCornerPrizeSummary
	PrizeLevel   int
	AddedToParty bool
	PCBox        int
	PCSlot       int
}

type GameCornerPrizeSummary struct {
	Name      string
	Type      string
	PokemonID int
	ItemID    int
	CoinCost  int
}

type GameCornerHiddenCoinSummary struct {
	Success      bool
	Message      string
	CoinID       int
	Amount       int
	Coins        int
	AlreadyFound bool
	Attempts     int
}

type SafariSummary struct {
	Active    bool
	BallsLeft int
	StepsLeft int
	Battle    *SafariBattleSummary
}

type SafariBattleSummary struct {
	PokemonID int
	Name      string
	Level     int
	Caught    bool
	Fled      bool
	IsOver    bool
}

type RepelSummary struct {
	InitialActive    bool
	InitialStepsLeft int
	Success          bool
	Message          string
	Active           bool
	StepsLeft        int
	WoreOff          bool
	ItemID           int
}

func Run(scenario *Scenario) (*Result, error) {
	applied, err := ApplyFixture(scenario.Fixture)
	if err != nil {
		return nil, err
	}

	efm := world.NewEventFlagManager(db.GlobalWorldDB.DB)
	if err := efm.LoadFlags(applied.CharacterID); err != nil {
		return nil, err
	}
	cutscenes := world.NewCutsceneManager(db.GlobalWorldDB.DB)
	cutscenes.Load()

	initial, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	if scenario.Trigger.Type == "safari_enter" {
		return runSafariEnter(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "safari_step" {
		return runSafariStep(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "safari_battle_action" {
		return runSafariBattleAction(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "daycare_deposit" {
		return runDayCareDeposit(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "daycare_step" {
		return runDayCareStep(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "daycare_withdraw" {
		return runDayCareWithdraw(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "repel_use" {
		return runRepelUse(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "repel_step" {
		return runRepelStep(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "active_battle_state" {
		return runActiveBattleState(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "fixture_state" {
		return runFixtureState(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "resolve_active_battle" {
		return runResolveActiveBattle(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "gamecorner_buy_coins" {
		return runGameCornerBuyCoins(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "gamecorner_slot_play" {
		return runGameCornerSlotPlay(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "gamecorner_prize_list" {
		return runGameCornerPrizeList(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "gamecorner_prize_buy" {
		return runGameCornerPrizeBuy(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "gamecorner_hidden_coin" {
		return runGameCornerHiddenCoin(scenario, applied, initial)
	}
	if scenario.Trigger.Type == "field_move_permission" {
		return runFieldMovePermission(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "boulder_push" {
		return runBoulderPush(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "runtime_boulder_push" {
		return runRuntimeBoulderPush(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "pathfind" {
		return runPathfind(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "vermilion_gym_trash" {
		return runVermilionGymTrash(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "silph_card_key" {
		return runSilphCardKey(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "seafoam_boulder_hole" {
		return runSeafoamBoulderHole(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "seafoam_current" {
		return runSeafoamCurrent(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "seafoam_surf_check" {
		return runSeafoamSurfCheck(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "elevator_floors" {
		return runElevatorFloors(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "elevator_select" {
		return runElevatorSelect(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "dialogue_choice" {
		return runDialogueChoice(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "map_load" {
		return runMapLoad(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "tile_state" {
		return runTileState(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "event_object_state" {
		return runEventObjectState(scenario, applied, initial, efm)
	}
	if scenario.Trigger.Type == "click_no_script" {
		return runClickNoScript(scenario, applied, initial, cutscenes, efm)
	}

	cs, err := resolveScript(scenario, applied, cutscenes, efm)
	if err != nil {
		return nil, err
	}
	var scenarioWorld *world.WorldHandler
	if scenario.Fixture.Safari != nil {
		scenarioWorld, err = newSafariScenarioWorld(scenario, applied.CharacterID)
		if err != nil {
			return nil, err
		}
	}
	effects, err := ExecuteServerActionsWithChoiceAndWorld(applied.CharacterID, cs, efm, scenario.Trigger.Choice, scenarioWorld)
	if err != nil {
		return nil, err
	}
	if s := scenario.ResolveBattle; s != nil {
		battleEffects, err := ResolveActiveBattle(applied.CharacterID, *s, efm)
		if err != nil {
			return nil, err
		}
		effects = append(effects, battleEffects...)
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	objectStates, err := objectStatesForExpectedMaps(scenario, applied, efm)
	if err != nil {
		return nil, err
	}

	result := &Result{
		Scenario:      scenario,
		CharacterID:   applied.CharacterID,
		Script:        cs,
		Initial:       initial,
		Final:         final,
		ActionEffects: effects,
		ObjectStates:  objectStates,
	}
	if scenarioWorld != nil && scenarioWorld.Safari != nil {
		result.Safari = safariSummaryFromSession(scenarioWorld.Safari.GetSession(applied.CharacterID))
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runVermilionGymTrash(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	outcome, err := world.HandleVermilionGymTrashCanWithPicker(
		applied.CharacterID,
		scenario.Trigger.TrashCanIndex,
		efm,
		world.FixedVermilionGymTrashPicker{
			FirstLockCanIndex:  scenario.Trigger.RandomFirstLockCanIndex,
			SecondLockCanIndex: scenario.Trigger.RandomSecondLockCanIndex,
		},
	)
	if err != nil {
		return nil, err
	}
	detail := fmt.Sprintf("can=%d text=%s first=%d", outcome.CanIndex, outcome.TextConstant, outcome.State.FirstLockCanIndex)
	if outcome.State.SecondLockCanIndex != nil {
		detail = fmt.Sprintf("%s second=%d", detail, *outcome.State.SecondLockCanIndex)
	}

	tileStates, err := eventTileStates(applied, efm)
	if err != nil {
		return nil, err
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "VermilionGymTrash",
			MapName:     scenario.Trigger.MapName,
			TriggerType: "vermilion_gym_trash",
		},
		Initial:       initial,
		Final:         final,
		ActionEffects: []ActionEffect{{Type: "vermilionGymTrash", Detail: detail, Changed: outcome.Changed}},
		TileStates:    tileStates,
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runSilphCardKey(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	if scenario.Trigger.TextConstant == "" {
		return nil, fmt.Errorf("silph_card_key trigger requires textConstant")
	}
	outcome, err := world.HandleSilphCardKeyDoor(applied.CharacterID, scenario.Trigger.TextConstant, efm)
	if err != nil {
		return nil, err
	}
	detail := fmt.Sprintf("door=%s %s%d flag=%s hasKey=%t opened=%t",
		outcome.Door.MapName,
		outcome.Door.FloorLabel,
		outcome.Door.DoorIndex,
		outcome.Door.Flag,
		outcome.HasCardKey,
		outcome.Opened)
	if outcome.AlreadyOpen {
		detail = fmt.Sprintf("%s alreadyOpen=true", detail)
	}

	tileStates, err := eventTileStates(applied, efm)
	if err != nil {
		return nil, err
	}
	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: world.SilphCardKeyScriptLabel(),
			MapName:     scenario.Trigger.MapName,
			TriggerType: "silph_card_key",
		},
		Initial:       initial,
		Final:         final,
		ActionEffects: []ActionEffect{{Type: "silphCardKey", Detail: detail, Changed: outcome.Changed}},
		TileStates:    tileStates,
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runSeafoamBoulderHole(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	if scenario.Trigger.HoleIndex <= 0 {
		return nil, fmt.Errorf("seafoam_boulder_hole trigger requires holeIndex")
	}
	outcome, err := world.HandleSeafoamBoulderHole(applied.CharacterID, scenario.Trigger.MapName, scenario.Trigger.HoleIndex, efm)
	if err != nil {
		return nil, err
	}
	detail := fmt.Sprintf("map=%s hole=%d (%d,%d) flag=%s source=%s destination=%s/%s",
		outcome.Hole.MapName,
		outcome.Hole.HoleIndex,
		outcome.Hole.HoleX,
		outcome.Hole.HoleY,
		outcome.Hole.Flag,
		outcome.Hole.SourceObjectName,
		outcome.Hole.DestinationMapName,
		outcome.Hole.DestinationObjectName)
	if outcome.AlreadySet {
		detail = fmt.Sprintf("%s alreadySet=true", detail)
	}

	objectStates, err := objectStatesForMaps(applied, efm, outcome.Hole.MapName, outcome.Hole.DestinationMapName)
	if err != nil {
		return nil, err
	}
	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: world.SeafoamBoulderScriptLabel(),
			MapName:     scenario.Trigger.MapName,
			TriggerType: "seafoam_boulder_hole",
		},
		Initial:       initial,
		Final:         final,
		ActionEffects: []ActionEffect{{Type: "seafoamBoulderHole", Detail: detail, Changed: outcome.Changed}},
		ObjectStates:  objectStates,
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runSeafoamCurrent(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	x, y := triggerOrFixturePosition(scenario)
	current, ok := world.SeafoamCurrentAt(applied.CharacterID, scenario.Trigger.MapName, x, y, efm)
	if !ok {
		return nil, fmt.Errorf("no Seafoam current for %s (%d,%d) with fixture flags", scenario.Trigger.MapName, x, y)
	}
	finalX, finalY := world.SeafoamCurrentFinalPosition(x, y, current.Movements)
	if _, err := db.GlobalWorldDB.DB.Exec(
		`UPDATE character_data SET x = $1, y = $2 WHERE id = $3`,
		finalX, finalY, applied.CharacterID); err != nil {
		return nil, fmt.Errorf("apply Seafoam current final position: %w", err)
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	directions := world.ExpandMovements(current.Movements)
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "SeafoamCurrent:" + current.Label,
			MapName:     scenario.Trigger.MapName,
			TriggerType: "seafoam_current",
		},
		Initial: initial,
		Final:   final,
		ActionEffects: []ActionEffect{{
			Type:    "seafoamCurrent",
			Detail:  fmt.Sprintf("%s start=(%d,%d) final=(%d,%d) movements=%v", current.Label, x, y, finalX, finalY, directions),
			Changed: true,
		}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runSeafoamSurfCheck(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	x, y := triggerOrFixturePosition(scenario)
	blocked := world.SeafoamSurfBlocked(applied.CharacterID, scenario.Trigger.MapName, x, y, efm)
	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "SeafoamSurfCheck",
			MapName:     scenario.Trigger.MapName,
			TriggerType: "seafoam_surf_check",
		},
		Initial: initial,
		Final:   final,
		ActionEffects: []ActionEffect{{
			Type:   "seafoamSurfCheck",
			Detail: fmt.Sprintf("map=%s (%d,%d) blocked=%t", scenario.Trigger.MapName, x, y, blocked),
		}},
		SurfBlocked: &blocked,
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func triggerOrFixturePosition(scenario *Scenario) (int, int) {
	x, y := scenario.Trigger.X, scenario.Trigger.Y
	if x == 0 && y == 0 {
		x, y = scenario.Fixture.X, scenario.Fixture.Y
	}
	return x, y
}

func runElevatorFloors(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	access, err := world.AvailableElevatorFloors(applied.CharacterID, applied.MapID, efm)
	if err != nil {
		return nil, err
	}
	summary, err := elevatorSummary(access)
	if err != nil {
		return nil, err
	}

	detail := fmt.Sprintf("floors=%d", len(summary.Floors))
	if summary.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, summary.Message)
	}
	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "ElevatorFloors:" + scenario.Trigger.MapName,
			MapName:     scenario.Trigger.MapName,
			TriggerType: "elevator_floors",
		},
		Initial:  initial,
		Final:    final,
		Elevator: summary,
		ActionEffects: []ActionEffect{{
			Type:   "elevatorFloors",
			Detail: detail,
		}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runElevatorSelect(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	if scenario.Trigger.FloorMapName == "" {
		return nil, fmt.Errorf("elevator_select trigger requires floorMapName")
	}
	floorMapID, err := mapIDForName(scenario.Trigger.FloorMapName)
	if err != nil {
		return nil, err
	}
	floor, err := world.ElevatorDestination(applied.CharacterID, applied.MapID, floorMapID, efm)
	if err != nil {
		return nil, err
	}
	if _, err := db.GlobalWorldDB.DB.Exec(
		`UPDATE character_data SET map_id = $1, x = $2, y = $3 WHERE id = $4`,
		floor.FloorMapID, floor.DestX, floor.DestY, applied.CharacterID); err != nil {
		return nil, fmt.Errorf("apply elevator destination: %w", err)
	}

	selected, err := elevatorFloorSummary(*floor)
	if err != nil {
		return nil, err
	}
	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "ElevatorSelect:" + scenario.Trigger.FloorMapName,
			MapName:     scenario.Trigger.MapName,
			TriggerType: "elevator_select",
		},
		Initial:  initial,
		Final:    final,
		Elevator: &ElevatorSummary{Selected: &selected},
		ActionEffects: []ActionEffect{{
			Type:    "elevatorSelect",
			Detail:  fmt.Sprintf("%s -> %s[%d] (%d,%d)", scenario.Trigger.MapName, selected.MapName, selected.MapID, selected.DestX, selected.DestY),
			Changed: true,
		}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func elevatorSummary(access world.ElevatorAccess) (*ElevatorSummary, error) {
	summary := &ElevatorSummary{
		Floors:  make([]ElevatorFloorSummary, 0, len(access.Floors)),
		Message: access.Message,
	}
	for _, floor := range access.Floors {
		item, err := elevatorFloorSummary(floor)
		if err != nil {
			return nil, err
		}
		summary.Floors = append(summary.Floors, item)
	}
	return summary, nil
}

func elevatorFloorSummary(floor world.ElevatorFloor) (ElevatorFloorSummary, error) {
	mapName, err := mapNameForID(floor.FloorMapID)
	if err != nil {
		return ElevatorFloorSummary{}, err
	}
	return ElevatorFloorSummary{
		MapID:          floor.FloorMapID,
		MapName:        mapName,
		Label:          floor.FloorLabel,
		DestX:          floor.DestX,
		DestY:          floor.DestY,
		RequiresFlag:   floor.RequiresFlag,
		RequiresItemID: floor.RequiresItemID,
	}, nil
}

func runTileState(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	tileStates, err := eventTileStates(applied, efm)
	if err != nil {
		return nil, err
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "TileState:" + scenario.Trigger.MapName,
			MapName:     scenario.Trigger.MapName,
			TriggerType: "tile_state",
		},
		Initial:       initial,
		Final:         final,
		ActionEffects: []ActionEffect{{Type: "tileState", Detail: fmt.Sprintf("%d event tiles", len(tileStates))}},
		TileStates:    tileStates,
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runEventObjectState(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	objectStates, err := objectStatesForMaps(applied, efm, scenario.Trigger.MapName)
	if err != nil {
		return nil, err
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "EventObjectState:" + scenario.Trigger.MapName,
			MapName:     scenario.Trigger.MapName,
			TriggerType: "event_object_state",
		},
		Initial:       initial,
		Final:         final,
		ActionEffects: []ActionEffect{{Type: "eventObjectState", Detail: fmt.Sprintf("%d event objects", len(objectStates))}},
		ObjectStates:  objectStates,
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runMapLoad(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	mapName := scenario.Trigger.MapName
	if mapName == "" {
		mapName = scenario.Fixture.MapName
	}
	if _, err := mapIDForName(mapName); err != nil {
		return nil, err
	}
	effect, err := world.ApplyMapLoadScriptEffectsForMapName(applied.CharacterID, mapName, efm)
	if err != nil {
		return nil, err
	}

	objectStates, err := objectStatesForMaps(applied, efm, effect.AffectedMapNames...)
	if err != nil {
		return nil, err
	}
	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}

	detail := fmt.Sprintf("%s set=%v reset=%v affected=%v", mapName, effect.SetFlags, effect.ResetFlags, effect.AffectedMapNames)
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "MapLoad:" + mapName,
			MapName:     mapName,
			TriggerType: "map_load",
		},
		Initial:       initial,
		Final:         final,
		ObjectStates:  objectStates,
		ActionEffects: []ActionEffect{{Type: "mapLoad", Detail: detail, Changed: effect.Changed()}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func eventTileStates(applied *AppliedFixture, efm *world.EventFlagManager) ([]TileState, error) {
	states, err := world.EventTileStatesForCharacter(applied.CharacterID, applied.MapID, efm)
	if err != nil {
		return nil, err
	}
	tileStates := make([]TileState, 0, len(states))
	for _, state := range states {
		tileStates = append(tileStates, TileState{
			X:             state.X,
			Y:             state.Y,
			TileImageID:   state.TileImageID,
			CollisionType: state.CollisionType,
			Label:         state.Label,
		})
	}
	return tileStates, nil
}

func objectStatesForMaps(applied *AppliedFixture, efm *world.EventFlagManager, mapNames ...string) ([]ObjectState, error) {
	objectStates := []ObjectState{}
	seenMaps := make(map[string]bool)
	for _, mapName := range mapNames {
		if mapName == "" || seenMaps[mapName] {
			continue
		}
		seenMaps[mapName] = true
		mapID, err := eventObjectStateMapIDForName(mapName)
		if err != nil {
			return nil, err
		}
		states, err := world.EventObjectStatesForCharacter(applied.CharacterID, mapID, efm)
		if err != nil {
			return nil, err
		}
		for _, state := range states {
			objectStates = append(objectStates, ObjectState{
				MapName: mapName,
				Name:    state.Name,
				Text:    state.Text,
				X:       state.X,
				Y:       state.Y,
				Visible: state.Visible,
				Label:   state.Label,
			})
		}
	}
	return objectStates, nil
}

func eventObjectStateMapIDForName(mapName string) (int, error) {
	if mapName == "" {
		return 0, fmt.Errorf("mapName is required")
	}
	var id int
	if err := db.GlobalWorldDB.DB.QueryRow(`SELECT id FROM phaser_maps WHERE name = $1`, mapName).Scan(&id); err != nil {
		return 0, fmt.Errorf("lookup event object map %s: %w", mapName, err)
	}
	return id, nil
}

func objectStatesForExpectedMaps(scenario *Scenario, applied *AppliedFixture, efm *world.EventFlagManager) ([]ObjectState, error) {
	if len(scenario.Expect.ObjectStates) == 0 {
		return nil, nil
	}
	mapNames := make([]string, 0, len(scenario.Expect.ObjectStates))
	for _, expected := range scenario.Expect.ObjectStates {
		mapName := expected.MapName
		if mapName == "" {
			mapName = scenario.Trigger.MapName
		}
		if mapName == "" {
			mapName = scenario.Fixture.MapName
		}
		mapNames = append(mapNames, mapName)
	}
	return objectStatesForMaps(applied, efm, mapNames...)
}

func runDialogueChoice(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	if scenario.Trigger.TextConstant == "" {
		return nil, fmt.Errorf("dialogue_choice trigger requires textConstant")
	}
	if scenario.Trigger.Choice == nil {
		return nil, fmt.Errorf("dialogue_choice trigger requires choice")
	}

	outcome, err := world.ResolveDialogueChoice(scenario.Trigger.TextConstant, *scenario.Trigger.Choice)
	if err != nil {
		return nil, err
	}
	mapName := outcome.MapName
	if mapName == "" {
		mapName = scenario.Trigger.MapName
	}
	effects, err := ExecuteActionList(applied.CharacterID, mapName, outcome.Actions, efm)
	if err != nil {
		return nil, err
	}
	effects = append([]ActionEffect{{
		Type:    "dialogueChoice",
		Detail:  fmt.Sprintf("%s choice=%t", scenario.Trigger.TextConstant, *scenario.Trigger.Choice),
		Changed: len(outcome.Actions) > 0,
	}}, effects...)

	if s := scenario.ResolveBattle; s != nil {
		battleEffects, err := ResolveActiveBattle(applied.CharacterID, *s, efm)
		if err != nil {
			return nil, err
		}
		effects = append(effects, battleEffects...)
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "DialogueChoice:" + scenario.Trigger.TextConstant,
			MapName:     mapName,
			TriggerType: "dialogue_choice",
		},
		Initial:       initial,
		Final:         final,
		ActionEffects: effects,
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runGameCornerBuyCoins(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	purchase := world.TryBuyGameCornerCoins(applied.CharacterID)
	detail := fmt.Sprintf("success=%t money=%d coins=%d", purchase.Success, purchase.Money, purchase.Coins)
	if purchase.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, purchase.Message)
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "GameCornerBuyCoins",
			MapName:     scenario.Trigger.MapName,
			TriggerType: "gamecorner_buy_coins",
		},
		Initial: initial,
		Final:   final,
		ActionEffects: []ActionEffect{{
			Type:    "gameCornerBuyCoins",
			Detail:  detail,
			Changed: purchase.Success,
		}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runGameCornerSlotPlay(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	rng := &fixedGameCornerRandom{values: scenario.Trigger.RandomValues}
	slot := world.TryPlayGameCornerSlot(applied.CharacterID, scenario.Trigger.Bet, scenario.Trigger.IsLucky, rng)
	summary := gameCornerSlotSummary(slot)
	detail := fmt.Sprintf("success=%t bet=%d payout=%d coins=%d positions=%v",
		summary.Success,
		summary.Bet,
		summary.Payout,
		summary.Coins,
		summary.ReelPositions)
	if summary.MatchLine != "" {
		detail = fmt.Sprintf("%s match=%s:%s", detail, summary.MatchLine, summary.MatchSymbol)
	}
	if summary.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, summary.Message)
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:      scenario,
		CharacterID:   applied.CharacterID,
		Script:        basicScript("GameCornerSlotPlay", scenario.Trigger.MapName, "gamecorner_slot_play"),
		Initial:       initial,
		Final:         final,
		GameCorner:    &GameCornerSummary{SlotPlay: &summary},
		ActionEffects: []ActionEffect{{Type: "gameCornerSlotPlay", Detail: detail, Changed: summary.Success}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runGameCornerPrizeList(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	prizes, err := world.AvailableGameCornerPrizes(applied.CharacterID)
	if err != nil {
		return nil, err
	}
	summary := gameCornerPrizeListSummary(prizes)
	detail := fmt.Sprintf("success=%t prizes=%d coins=%d", summary.Success, len(summary.Prizes), summary.Coins)
	if summary.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, summary.Message)
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:      scenario,
		CharacterID:   applied.CharacterID,
		Script:        basicScript("GameCornerPrizeList", scenario.Trigger.MapName, "gamecorner_prize_list"),
		Initial:       initial,
		Final:         final,
		GameCorner:    &GameCornerSummary{PrizeList: &summary},
		ActionEffects: []ActionEffect{{Type: "gameCornerPrizeList", Detail: detail}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runGameCornerPrizeBuy(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	if scenario.Trigger.PrizeName == "" {
		return nil, fmt.Errorf("gamecorner_prize_buy trigger requires prizeName")
	}
	purchase := world.TryBuyGameCornerPrizeByName(applied.CharacterID, scenario.Trigger.PrizeName)
	summary := gameCornerPrizePurchaseSummary(purchase)
	detail := fmt.Sprintf("success=%t prize=%s coins=%d", summary.Success, summary.Prize.Name, summary.Coins)
	if summary.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, summary.Message)
	}
	if summary.PrizeLevel > 0 {
		detail = fmt.Sprintf("%s level=%d", detail, summary.PrizeLevel)
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:      scenario,
		CharacterID:   applied.CharacterID,
		Script:        basicScript("GameCornerPrizeBuy:"+scenario.Trigger.PrizeName, scenario.Trigger.MapName, "gamecorner_prize_buy"),
		Initial:       initial,
		Final:         final,
		GameCorner:    &GameCornerSummary{PrizePurchase: &summary},
		ActionEffects: []ActionEffect{{Type: "gameCornerPrizeBuy", Detail: detail, Changed: summary.Success}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runGameCornerHiddenCoin(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	x, y := triggerOrFixturePosition(scenario)
	attempts := scenario.Trigger.Repeat
	if attempts <= 0 {
		attempts = 1
	}

	var pickup world.GameCornerHiddenCoinPickupResult
	changed := false
	for i := 0; i < attempts; i++ {
		pickup = world.TryPickUpGameCornerHiddenCoin(applied.CharacterID, applied.MapID, x, y)
		if pickup.Success {
			changed = true
		}
	}
	summary := gameCornerHiddenCoinSummary(pickup, attempts)
	detail := fmt.Sprintf("success=%t coinId=%d amount=%d coins=%d attempts=%d",
		summary.Success,
		summary.CoinID,
		summary.Amount,
		summary.Coins,
		summary.Attempts)
	if summary.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, summary.Message)
	}
	if summary.AlreadyFound {
		detail = fmt.Sprintf("%s alreadyFound=true", detail)
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:      scenario,
		CharacterID:   applied.CharacterID,
		Script:        basicScript("GameCornerHiddenCoin", scenario.Trigger.MapName, "gamecorner_hidden_coin"),
		Initial:       initial,
		Final:         final,
		GameCorner:    &GameCornerSummary{HiddenCoin: &summary},
		ActionEffects: []ActionEffect{{Type: "gameCornerHiddenCoin", Detail: detail, Changed: changed}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

type fixedGameCornerRandom struct {
	values []int
	index  int
}

func (r *fixedGameCornerRandom) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	if r.index >= len(r.values) {
		return 0
	}
	value := r.values[r.index] % n
	r.index++
	if value < 0 {
		value += n
	}
	return value
}

func basicScript(label, mapName, triggerType string) *world.CutsceneScript {
	return &world.CutsceneScript{
		ScriptLabel: label,
		MapName:     mapName,
		TriggerType: triggerType,
	}
}

func gameCornerSlotSummary(result world.GameCornerSlotPlayResult) GameCornerSlotSummary {
	return GameCornerSlotSummary{
		Success:       result.Success,
		Message:       result.Message,
		Bet:           result.Bet,
		Payout:        result.Payout,
		MatchLine:     result.MatchLine,
		MatchSymbol:   result.MatchSymbol,
		Coins:         result.Coins,
		ReelPositions: result.ReelPositions,
		Reels:         result.Reels,
	}
}

func gameCornerPrizeListSummary(result world.GameCornerPrizeListResult) GameCornerPrizeListSummary {
	summary := GameCornerPrizeListSummary{
		Success: result.Success,
		Message: result.Message,
		Coins:   result.Coins,
		Prizes:  make([]GameCornerPrizeSummary, 0, len(result.Prizes)),
	}
	for _, prize := range result.Prizes {
		summary.Prizes = append(summary.Prizes, gameCornerPrizeSummary(prize))
	}
	return summary
}

func gameCornerPrizePurchaseSummary(result world.GameCornerPrizePurchaseResult) GameCornerPrizePurchaseSummary {
	summary := GameCornerPrizePurchaseSummary{
		Success:      result.Success,
		Message:      result.Message,
		Coins:        result.Coins,
		PrizeLevel:   result.PrizeLevel,
		AddedToParty: result.AddedToParty,
		PCBox:        result.PCBox,
		PCSlot:       result.PCSlot,
	}
	if result.Prize != nil {
		summary.Prize = gameCornerPrizeSummary(*result.Prize)
	}
	return summary
}

func gameCornerHiddenCoinSummary(result world.GameCornerHiddenCoinPickupResult, attempts int) GameCornerHiddenCoinSummary {
	return GameCornerHiddenCoinSummary{
		Success:      result.Success,
		Message:      result.Message,
		CoinID:       result.CoinID,
		Amount:       result.Amount,
		Coins:        result.Coins,
		AlreadyFound: result.AlreadyFound,
		Attempts:     attempts,
	}
}

func gameCornerPrizeSummary(prize world.GameCornerPrize) GameCornerPrizeSummary {
	summary := GameCornerPrizeSummary{
		Name:     prize.Name,
		Type:     prize.Type,
		CoinCost: prize.CoinCost,
	}
	if prize.PokemonID != nil {
		summary.PokemonID = *prize.PokemonID
	}
	if prize.ItemID != nil {
		summary.ItemID = *prize.ItemID
	}
	return summary
}

func runSafariEnter(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	safari := world.NewSafariZoneManager()
	entry := world.TryStartSafariZoneVisit(applied.CharacterID, safari)
	detail := fmt.Sprintf("success=%t money=%d", entry.Success, entry.Money)
	if entry.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, entry.Message)
	}
	if entry.Success {
		detail = fmt.Sprintf("%s balls=%d steps=%d", detail, entry.BallsLeft, entry.StepsLeft)
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}

	summary := &SafariSummary{}
	if session := safari.GetSession(applied.CharacterID); session != nil {
		summary.Active = session.Active
		summary.BallsLeft = session.BallsLeft
		summary.StepsLeft = session.StepsLeft
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "SafariZoneEnter",
			MapName:     scenario.Trigger.MapName,
			TriggerType: "safari_enter",
		},
		Initial: initial,
		Final:   final,
		ActionEffects: []ActionEffect{{
			Type:    "safariEnter",
			Detail:  detail,
			Changed: entry.Success,
		}},
		Safari: summary,
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runSafariStep(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	wh, err := newSafariScenarioWorld(scenario, applied.CharacterID)
	if err != nil {
		return nil, err
	}
	safari := wh.Safari
	repeat := scenario.Trigger.Repeat
	if repeat <= 0 {
		repeat = 1
	}
	stepsLeft := 0
	ballsLeft := 0
	expired := false
	for i := 0; i < repeat; i++ {
		stepsLeft, ballsLeft, expired = safari.DecrementStep(applied.CharacterID)
	}
	summary := safariSummaryFromSession(safari.GetSession(applied.CharacterID))

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	detail := fmt.Sprintf("steps=%d active=%t left=%d balls=%d expired=%t", repeat, summary.Active, stepsLeft, ballsLeft, expired)
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "SafariStep",
			MapName:     scenario.Trigger.MapName,
			TriggerType: "safari_step",
		},
		Initial: initial,
		Final:   final,
		ActionEffects: []ActionEffect{{
			Type:    "safariStep",
			Detail:  detail,
			Changed: repeat > 0,
		}},
		Safari: summary,
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runSafariBattleAction(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	if scenario.Trigger.Action == "" {
		return nil, fmt.Errorf("safari_battle_action trigger requires action")
	}
	wh, err := newSafariScenarioWorld(scenario, applied.CharacterID)
	if err != nil {
		return nil, err
	}
	before := safariSummaryFromSession(wh.Safari.GetSession(applied.CharacterID))
	ses, recorder := NewRecordedSession(
		applied.CharacterID,
		initial.CharacterName,
		initial.MapID,
		initial.X,
		initial.Y,
	)
	payload, err := json.Marshal(map[string]string{"action": scenario.Trigger.Action})
	if err != nil {
		return nil, err
	}
	world.HandleSafariBattleAction(ses, payload, wh)
	message := lastRecordedMessage(recorder.Messages, opcodes.SafariBattleActionResponse)
	if message == nil {
		return nil, fmt.Errorf("safari_battle_action did not emit SafariBattleActionResponse")
	}
	action, err := parseSafariBattleActionResponse(message.Payload)
	if err != nil {
		return nil, err
	}
	summary := safariSummaryFromSession(wh.Safari.GetSession(applied.CharacterID))

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	detail := fmt.Sprintf("action=%s success=%t active=%t balls=%d steps=%d isOver=%t caught=%t fled=%t",
		scenario.Trigger.Action, action.Success, summary.Active, summary.BallsLeft, summary.StepsLeft, action.IsOver, action.Caught, action.Fled)
	if before.Battle != nil {
		detail = fmt.Sprintf("%s battleBefore=#%d L%d", detail, before.Battle.PokemonID, before.Battle.Level)
	}
	if action.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, action.Message)
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "SafariBattleAction",
			MapName:     scenario.Trigger.MapName,
			TriggerType: "safari_battle_action",
		},
		Initial: initial,
		Final:   final,
		ActionEffects: []ActionEffect{{
			Type:    "safariBattleAction",
			Detail:  detail,
			Changed: action.Success,
		}},
		Safari:   summary,
		Messages: recorder.Messages,
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

type safariBattleActionResponse struct {
	Success bool
	Message string
	IsOver  bool
	Caught  bool
	Fled    bool
}

func parseSafariBattleActionResponse(payload []byte) (safariBattleActionResponse, error) {
	var response struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
		IsOver  bool   `json:"isOver"`
		Caught  bool   `json:"caught"`
		Fled    bool   `json:"fled"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return safariBattleActionResponse{}, fmt.Errorf("parse SafariBattleActionResponse: %w", err)
	}
	message := response.Error
	if message == "" && response.Success {
		message = "ok"
	}
	return safariBattleActionResponse{
		Success: response.Success,
		Message: message,
		IsOver:  response.IsOver,
		Caught:  response.Caught,
		Fled:    response.Fled,
	}, nil
}

func newSafariScenarioWorld(scenario *Scenario, charID int64) (*world.WorldHandler, error) {
	safari := world.NewSafariZoneManager()
	if fixture := scenario.Fixture.Safari; fixture != nil {
		session := world.SafariSession{
			Active:    fixture.Active,
			BallsLeft: fixture.BallsLeft,
			StepsLeft: fixture.StepsLeft,
		}
		if fixture.Battle != nil {
			if !fixture.Active {
				return nil, fmt.Errorf("safari battle fixture requires active safari session")
			}
			wild, err := pokebattle.BuildWildPokemon(db.GlobalWorldDB.DB, fixture.Battle.PokemonID, fixture.Battle.Level)
			if err != nil {
				return nil, fmt.Errorf("build safari battle pokemon #%d L%d: %w", fixture.Battle.PokemonID, fixture.Battle.Level, err)
			}
			session.Battle = pokebattle.NewSafariBattle(wild, fixture.BallsLeft, fixture.StepsLeft)
		}
		safari.SetSession(charID, session)
	}
	return &world.WorldHandler{Safari: safari}, nil
}

func safariSummaryFromSession(session *world.SafariSession) *SafariSummary {
	summary := &SafariSummary{}
	if session == nil {
		return summary
	}
	summary.Active = session.Active
	summary.BallsLeft = session.BallsLeft
	summary.StepsLeft = session.StepsLeft
	if session.Battle != nil {
		wild := session.Battle.WildPokemon
		summary.Battle = &SafariBattleSummary{
			PokemonID: wild.ID,
			Name:      wild.Name,
			Level:     wild.Level,
			Caught:    session.Battle.Caught,
			Fled:      session.Battle.Fled,
			IsOver:    session.Battle.IsOver(),
		}
	}
	return summary
}

func runRepelUse(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	if scenario.Trigger.ItemID <= 0 {
		return nil, fmt.Errorf("repel_use trigger requires itemId")
	}
	wh, summary := newRepelScenarioWorld(scenario, applied.CharacterID)
	summary.ItemID = scenario.Trigger.ItemID

	ses, recorder := NewRecordedSession(
		applied.CharacterID,
		initial.CharacterName,
		initial.MapID,
		initial.X,
		initial.Y,
	)
	payload, err := json.Marshal(map[string]int{"itemId": scenario.Trigger.ItemID})
	if err != nil {
		return nil, err
	}
	world.HandleRepelUse(ses, payload, wh)
	message := lastRecordedMessage(recorder.Messages, opcodes.RepelUseResponse)
	if message == nil {
		return nil, fmt.Errorf("repel_use did not emit RepelUseResponse")
	}
	success, responseText, err := parseRepelUseResponse(message.Payload)
	if err != nil {
		return nil, err
	}
	summary.Success = success
	summary.Message = responseText

	status := wh.WildEncounter.RepelStatus(applied.CharacterID)
	summary.Active = status.Active
	summary.StepsLeft = status.StepsLeft

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	detail := fmt.Sprintf("success=%t item=%d steps=%d", summary.Success, summary.ItemID, summary.StepsLeft)
	if summary.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, summary.Message)
	}
	resultState := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "RepelUse",
			MapName:     scenario.Trigger.MapName,
			TriggerType: "repel_use",
		},
		Initial: initial,
		Final:   final,
		ActionEffects: []ActionEffect{{
			Type:    "repelUse",
			Detail:  detail,
			Changed: summary.Success,
		}},
		Repel:    summary,
		Messages: recorder.Messages,
	}
	if err := resultState.ValidateExpectations(); err != nil {
		return resultState, err
	}
	return resultState, nil
}

func lastRecordedMessage(messages []RecordedMessage, opcode opcodes.OpCode) *RecordedMessage {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Opcode == opcode {
			return &messages[i]
		}
	}
	return nil
}

func parseRepelUseResponse(payload []byte) (bool, string, error) {
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return false, "", fmt.Errorf("parse RepelUseResponse: %w", err)
	}
	if response.Message != "" {
		return response.Success, response.Message, nil
	}
	return response.Success, response.Error, nil
}

func runRepelStep(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	wh, summary := newRepelScenarioWorld(scenario, applied.CharacterID)
	repeat := scenario.Trigger.Repeat
	if repeat <= 0 {
		repeat = 1
	}
	for i := 0; i < repeat; i++ {
		if wh.WildEncounter.AdvanceRepelStep(applied.CharacterID) {
			summary.WoreOff = true
			summary.Message = "REPEL's effect wore off!"
		}
	}
	status := wh.WildEncounter.RepelStatus(applied.CharacterID)
	summary.Active = status.Active
	summary.StepsLeft = status.StepsLeft

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	detail := fmt.Sprintf("steps=%d active=%t left=%d", repeat, summary.Active, summary.StepsLeft)
	if summary.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, summary.Message)
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "RepelStep",
			MapName:     scenario.Trigger.MapName,
			TriggerType: "repel_step",
		},
		Initial: initial,
		Final:   final,
		ActionEffects: []ActionEffect{{
			Type:    "repelStep",
			Detail:  detail,
			Changed: summary.InitialStepsLeft != summary.StepsLeft,
		}},
		Repel: summary,
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runActiveBattleState(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "ActiveBattleState",
			MapName:     scenario.Trigger.MapName,
			TriggerType: "active_battle_state",
		},
		Initial: initial,
		Final:   final,
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runFixtureState(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "FixtureState",
			MapName:     scenario.Trigger.MapName,
			TriggerType: "fixture_state",
		},
		Initial: initial,
		Final:   final,
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runResolveActiveBattle(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	if scenario.ResolveBattle == nil {
		return nil, fmt.Errorf("resolve_active_battle requires resolveBattle")
	}
	effects, err := ResolveActiveBattle(applied.CharacterID, *scenario.ResolveBattle, efm)
	if err != nil {
		return nil, err
	}
	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script: &world.CutsceneScript{
			ScriptLabel: "ResolveActiveBattle",
			MapName:     scenario.Trigger.MapName,
			TriggerType: "resolve_active_battle",
		},
		Initial:       initial,
		Final:         final,
		ActionEffects: effects,
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func newRepelScenarioWorld(scenario *Scenario, charID int64) (*world.WorldHandler, *RepelSummary) {
	wh := &world.WorldHandler{}
	wh.WildEncounter = world.NewWildEncounterManager(wh)
	if scenario.Fixture.Repel != nil {
		wh.WildEncounter.SetRepelSteps(charID, scenario.Fixture.Repel.StepsLeft)
	}
	status := wh.WildEncounter.RepelStatus(charID)
	return wh, &RepelSummary{
		InitialActive:    status.Active,
		InitialStepsLeft: status.StepsLeft,
		Active:           status.Active,
		StepsLeft:        status.StepsLeft,
	}
}

func resolveScript(s *Scenario, applied *AppliedFixture, cutscenes *world.CutsceneManager, efm *world.EventFlagManager) (*world.CutsceneScript, error) {
	switch s.Trigger.Type {
	case "direct":
		if s.Trigger.ScriptLabel == "" {
			return nil, fmt.Errorf("direct trigger requires scriptLabel")
		}
		cs := cutscenes.GetByLabel(s.Trigger.ScriptLabel)
		if cs == nil {
			return nil, fmt.Errorf("script %s not found", s.Trigger.ScriptLabel)
		}
		if !cutscenes.CheckEligible(cs, applied.CharacterID, efm, s.Fixture.Direction) {
			return nil, fmt.Errorf("script %s is not eligible for fixture state", s.Trigger.ScriptLabel)
		}
		return cs, nil
	case "map_script":
		cs := cutscenes.FindEligibleMapScriptCutscene(s.Trigger.MapName, applied.CharacterID, efm, s.Fixture.Direction)
		if cs == nil {
			return nil, fmt.Errorf("no eligible map_script cutscene for %s", s.Trigger.MapName)
		}
		return cs, nil
	case "npc_click", "object_click":
		keys := clickKeys(s.Trigger)
		if err := ensureClickTargetExists(s.Trigger.MapName, keys); err != nil {
			return nil, err
		}
		cs := cutscenes.FindEligibleClickCutscene(s.Trigger.MapName, keys, applied.CharacterID, efm, s.Fixture.Direction)
		if cs == nil {
			return nil, fmt.Errorf("no eligible click cutscene for %s keys=%v", s.Trigger.MapName, keys)
		}
		return cs, nil
	case "coord":
		triggers := world.NewCoordinateTriggerManager(db.GlobalWorldDB.DB)
		triggers.Load()
		x, y := s.Trigger.X, s.Trigger.Y
		if x == 0 && y == 0 {
			x, y = s.Fixture.X, s.Fixture.Y
		}
		for _, trigger := range triggers.CheckTileTriggers(applied.MapID, x, y) {
			if cs := cutscenes.FindEligibleCoordCutsceneForTrigger(trigger, applied.CharacterID, efm, s.Fixture.Direction); cs != nil {
				return cs, nil
			}
		}
		return nil, fmt.Errorf("no eligible coord cutscene for %s (%d,%d)", s.Trigger.MapName, x, y)
	default:
		return nil, fmt.Errorf("unknown trigger type %q", s.Trigger.Type)
	}
}

func clickKeys(trigger Trigger) []string {
	keys := []string{}
	if trigger.ObjectID > 0 {
		keys = append(keys,
			fmt.Sprintf("object:%d", trigger.ObjectID),
			fmt.Sprintf("phaser_object:%d", trigger.ObjectID),
		)
	}
	for _, key := range []string{trigger.ObjectKey, trigger.TextConstant} {
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func ensureClickTargetExists(mapName string, keys []string) error {
	for _, key := range keys {
		ids, err := world.ResolveCutsceneObjectKey(mapName, key)
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			return nil
		}
	}
	return fmt.Errorf("click target not found for %s keys=%v", mapName, keys)
}

func (r *Result) ValidateExpectations() error {
	exp := r.Scenario.Expect
	if exp.ScriptLabel != "" && r.Script.ScriptLabel != exp.ScriptLabel {
		return fmt.Errorf("expected script %s, got %s", exp.ScriptLabel, r.Script.ScriptLabel)
	}
	if exp.FinalMapName != "" && r.Final.MapName != exp.FinalMapName {
		return fmt.Errorf("expected final map %s, got %s", exp.FinalMapName, r.Final.MapName)
	}
	if exp.FinalX != nil && r.Final.X != *exp.FinalX {
		return fmt.Errorf("expected final x %d, got %d", *exp.FinalX, r.Final.X)
	}
	if exp.FinalY != nil && r.Final.Y != *exp.FinalY {
		return fmt.Errorf("expected final y %d, got %d", *exp.FinalY, r.Final.Y)
	}
	if exp.FinalDirection != "" && r.Final.Direction != normalizeFixtureMoveName(exp.FinalDirection) {
		return fmt.Errorf("expected final direction %s, got %s", normalizeFixtureMoveName(exp.FinalDirection), r.Final.Direction)
	}
	for _, flag := range exp.FlagsPresent {
		if !hasFlag(r.Final.Flags, flag) {
			return fmt.Errorf("expected final flag %s", flag)
		}
	}
	for _, flag := range exp.FlagsAbsent {
		if hasFlag(r.Final.Flags, flag) {
			return fmt.Errorf("expected final flag %s to be absent", flag)
		}
	}
	for _, speciesID := range exp.PartySpecies {
		if !finalPartyHasSpecies(r.Final.Party, speciesID) {
			return fmt.Errorf("expected final party species %d", speciesID)
		}
	}
	for _, speciesID := range exp.PartySpeciesAbsent {
		if finalPartyHasSpecies(r.Final.Party, speciesID) {
			return fmt.Errorf("expected final party species %d to be absent", speciesID)
		}
	}
	for _, expected := range exp.PartyState {
		if err := validatePartyState(r.Final.Party, expected); err != nil {
			return err
		}
	}
	for _, expected := range exp.PCContains {
		if !finalPCHasPokemon(r.Final.PC, expected) {
			return fmt.Errorf("expected PC pokemon species=%d box=%d slot=%d", expected.SpeciesID, expected.Box, expected.Slot)
		}
	}
	for _, expected := range exp.PCAbsent {
		if finalPCHasPokemon(r.Final.PC, expected) {
			return fmt.Errorf("expected PC pokemon species=%d absent from box=%d slot=%d", expected.SpeciesID, expected.Box, expected.Slot)
		}
	}
	for _, item := range exp.InventoryContains {
		itemID, err := resolveFixtureItemID(item)
		if err != nil {
			return err
		}
		quantity := item.Quantity
		if quantity <= 0 {
			quantity = 1
		}
		if inventoryQuantity(r.Final.Inventory, itemID) < quantity {
			return fmt.Errorf("expected inventory item %d x%d", itemID, quantity)
		}
	}
	for _, item := range exp.InventoryAbsent {
		itemID, err := resolveFixtureItemID(item)
		if err != nil {
			return err
		}
		if inventoryQuantity(r.Final.Inventory, itemID) > 0 {
			return fmt.Errorf("expected inventory item %d to be absent", itemID)
		}
	}
	if exp.PokedexSeenCount != nil && r.Final.PokedexSeen != *exp.PokedexSeenCount {
		return fmt.Errorf("expected pokedex seen count %d, got %d", *exp.PokedexSeenCount, r.Final.PokedexSeen)
	}
	if exp.PokedexCaughtCount != nil && r.Final.PokedexCaught != *exp.PokedexCaughtCount {
		return fmt.Errorf("expected pokedex caught count %d, got %d", *exp.PokedexCaughtCount, r.Final.PokedexCaught)
	}
	if len(exp.ElevatorFloors) > 0 {
		if r.Elevator == nil {
			return fmt.Errorf("expected elevator floor summary")
		}
		if !elevatorFloorsMatch(r.Elevator.Floors, exp.ElevatorFloors) {
			return fmt.Errorf("expected elevator floors %v, got %v", exp.ElevatorFloors, r.Elevator.Floors)
		}
	}
	if exp.ElevatorMessage != "" {
		if r.Elevator == nil {
			return fmt.Errorf("expected elevator message %q, got no elevator summary", exp.ElevatorMessage)
		}
		if r.Elevator.Message != exp.ElevatorMessage {
			return fmt.Errorf("expected elevator message %q, got %q", exp.ElevatorMessage, r.Elevator.Message)
		}
	}
	if len(exp.GameCornerPrizes) > 0 {
		if r.GameCorner == nil || r.GameCorner.PrizeList == nil {
			return fmt.Errorf("expected Game Corner prize list")
		}
		if !gameCornerPrizesMatch(r.GameCorner.PrizeList.Prizes, exp.GameCornerPrizes) {
			return fmt.Errorf("expected Game Corner prizes %v, got %v", exp.GameCornerPrizes, r.GameCorner.PrizeList.Prizes)
		}
	}
	if exp.GameCornerPrizeMessage != "" {
		if r.GameCorner == nil {
			return fmt.Errorf("expected Game Corner message %q, got no Game Corner summary", exp.GameCornerPrizeMessage)
		}
		message := ""
		if r.GameCorner.PrizeList != nil {
			message = r.GameCorner.PrizeList.Message
		}
		if r.GameCorner.PrizePurchase != nil {
			message = r.GameCorner.PrizePurchase.Message
		}
		if r.GameCorner.HiddenCoin != nil {
			message = r.GameCorner.HiddenCoin.Message
		}
		if r.GameCorner.SlotPlay != nil {
			message = r.GameCorner.SlotPlay.Message
		}
		if message != exp.GameCornerPrizeMessage {
			return fmt.Errorf("expected Game Corner message %q, got %q", exp.GameCornerPrizeMessage, message)
		}
	}
	if exp.GameCornerSlot != nil {
		if r.GameCorner == nil || r.GameCorner.SlotPlay == nil {
			return fmt.Errorf("expected Game Corner slot summary")
		}
		if !gameCornerSlotMatches(*r.GameCorner.SlotPlay, *exp.GameCornerSlot) {
			return fmt.Errorf("expected Game Corner slot %v, got %v", *exp.GameCornerSlot, *r.GameCorner.SlotPlay)
		}
	}
	if exp.FieldMove != nil {
		if r.FieldMove == nil {
			return fmt.Errorf("expected field move summary")
		}
		if !fieldMoveMatches(*r.FieldMove, *exp.FieldMove) {
			return fmt.Errorf("expected field move %v, got %v", *exp.FieldMove, *r.FieldMove)
		}
	}
	if exp.BoulderPush != nil {
		if r.BoulderPush == nil {
			return fmt.Errorf("expected boulder push summary")
		}
		if !boulderPushMatches(*r.BoulderPush, *exp.BoulderPush) {
			return fmt.Errorf("expected boulder push %v, got %v", *exp.BoulderPush, *r.BoulderPush)
		}
	}
	if exp.Pathfind != nil {
		if r.Pathfind == nil {
			return fmt.Errorf("expected pathfind summary")
		}
		if !pathfindMatches(*r.Pathfind, *exp.Pathfind) {
			return fmt.Errorf("expected pathfind %v, got %v", *exp.Pathfind, *r.Pathfind)
		}
	}
	if exp.Money != nil && r.Final.Money != *exp.Money {
		return fmt.Errorf("expected money %d, got %d", *exp.Money, r.Final.Money)
	}
	if exp.Coins != nil && r.Final.Coins != *exp.Coins {
		return fmt.Errorf("expected coins %d, got %d", *exp.Coins, r.Final.Coins)
	}
	if exp.SafariActive != nil {
		active := r.Safari != nil && r.Safari.Active
		if active != *exp.SafariActive {
			return fmt.Errorf("expected safari active %t, got %t", *exp.SafariActive, active)
		}
	}
	if exp.SafariBallsLeft != nil {
		if r.Safari == nil {
			return fmt.Errorf("expected safari balls left %d, got no safari summary", *exp.SafariBallsLeft)
		}
		if r.Safari.BallsLeft != *exp.SafariBallsLeft {
			return fmt.Errorf("expected safari balls left %d, got %d", *exp.SafariBallsLeft, r.Safari.BallsLeft)
		}
	}
	if exp.SafariStepsLeft != nil {
		if r.Safari == nil {
			return fmt.Errorf("expected safari steps left %d, got no safari summary", *exp.SafariStepsLeft)
		}
		if r.Safari.StepsLeft != *exp.SafariStepsLeft {
			return fmt.Errorf("expected safari steps left %d, got %d", *exp.SafariStepsLeft, r.Safari.StepsLeft)
		}
	}
	if exp.SafariBattle != nil {
		if err := validateSafariBattleExpectation(r.Safari, *exp.SafariBattle); err != nil {
			return err
		}
	}
	if exp.DayCare != nil {
		if err := validateDayCareExpectation(r.Final.DayCare, *exp.DayCare); err != nil {
			return err
		}
	}
	if exp.Repel != nil {
		if r.Repel == nil {
			return fmt.Errorf("expected repel summary")
		}
		if exp.Repel.Success != nil && r.Repel.Success != *exp.Repel.Success {
			return fmt.Errorf("expected repel success %t, got %t", *exp.Repel.Success, r.Repel.Success)
		}
		if exp.Repel.Active != nil && r.Repel.Active != *exp.Repel.Active {
			return fmt.Errorf("expected repel active %t, got %t", *exp.Repel.Active, r.Repel.Active)
		}
		if exp.Repel.StepsLeft != nil && r.Repel.StepsLeft != *exp.Repel.StepsLeft {
			return fmt.Errorf("expected repel steps left %d, got %d", *exp.Repel.StepsLeft, r.Repel.StepsLeft)
		}
		if exp.Repel.Message != "" && r.Repel.Message != exp.Repel.Message {
			return fmt.Errorf("expected repel message %q, got %q", exp.Repel.Message, r.Repel.Message)
		}
		if exp.Repel.WoreOff != nil && r.Repel.WoreOff != *exp.Repel.WoreOff {
			return fmt.Errorf("expected repel wore off %t, got %t", *exp.Repel.WoreOff, r.Repel.WoreOff)
		}
	}
	if len(exp.Messages) > 0 {
		if err := validateExpectedMessages(r.Messages, exp.Messages); err != nil {
			return err
		}
	}
	for _, key := range exp.HiddenObjectKeys {
		ids, err := world.ResolveCutsceneObjectKey(r.Script.MapName, key)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if !finalHiddenObjectID(r.Final.HiddenObjects, id) {
				return fmt.Errorf("expected hidden object %s/%d", key, id)
			}
		}
	}
	for _, actionType := range exp.ActionTypesContains {
		if !effectsContainType(r.ActionEffects, actionType) {
			return fmt.Errorf("expected action trace to contain %s", actionType)
		}
	}
	for _, actionType := range exp.ActionTypesAbsent {
		if effectsContainType(r.ActionEffects, actionType) {
			return fmt.Errorf("expected action trace not to contain %s", actionType)
		}
	}
	if exp.ActiveBattleType != "" {
		if r.Final.ActiveBattle == nil {
			return fmt.Errorf("expected active battle type %s, got none", exp.ActiveBattleType)
		}
		if r.Final.ActiveBattle.BattleType != exp.ActiveBattleType {
			return fmt.Errorf("expected active battle type %s, got %s", exp.ActiveBattleType, r.Final.ActiveBattle.BattleType)
		}
	}
	if exp.ActiveBattleTrainerClass != "" {
		if r.Final.ActiveBattle == nil {
			return fmt.Errorf("expected active battle trainer %s, got none", exp.ActiveBattleTrainerClass)
		}
		if r.Final.ActiveBattle.TrainerClass != exp.ActiveBattleTrainerClass {
			return fmt.Errorf("expected active battle trainer %s, got %s", exp.ActiveBattleTrainerClass, r.Final.ActiveBattle.TrainerClass)
		}
	}
	for _, speciesID := range exp.ActiveBattleEnemySpecies {
		if !activeBattleHasEnemySpecies(r.Final.ActiveBattle, speciesID) {
			return fmt.Errorf("expected active battle enemy species %d", speciesID)
		}
	}
	if len(exp.ActiveBattleAllowedActions) > 0 {
		if r.Final.ActiveBattle == nil {
			return fmt.Errorf("expected active battle allowed actions %v, got no active battle", exp.ActiveBattleAllowedActions)
		}
		if !sameStringList(r.Final.ActiveBattle.AllowedActions, exp.ActiveBattleAllowedActions) {
			return fmt.Errorf("expected active battle allowed actions %v, got %v",
				exp.ActiveBattleAllowedActions, r.Final.ActiveBattle.AllowedActions)
		}
	}
	if exp.ActiveBattleGuaranteedCatch != nil {
		if r.Final.ActiveBattle == nil {
			return fmt.Errorf("expected active battle guaranteed catch=%t, got no active battle", *exp.ActiveBattleGuaranteedCatch)
		}
		if r.Final.ActiveBattle.GuaranteedCatch != *exp.ActiveBattleGuaranteedCatch {
			return fmt.Errorf("expected active battle guaranteed catch=%t, got %t",
				*exp.ActiveBattleGuaranteedCatch, r.Final.ActiveBattle.GuaranteedCatch)
		}
	}
	if exp.ActiveBattleAbsent && r.Final.ActiveBattle != nil {
		return fmt.Errorf("expected no active battle, got %s", r.Final.ActiveBattle.TrainerClass)
	}
	if exp.SeafoamSurfBlocked != nil {
		if r.SurfBlocked == nil {
			return fmt.Errorf("expected Seafoam surf blocked=%t, got no surf summary", *exp.SeafoamSurfBlocked)
		}
		if *r.SurfBlocked != *exp.SeafoamSurfBlocked {
			return fmt.Errorf("expected Seafoam surf blocked=%t, got %t", *exp.SeafoamSurfBlocked, *r.SurfBlocked)
		}
	}
	for _, expected := range exp.TileStates {
		if !hasTileState(r.TileStates, expected) {
			return fmt.Errorf("expected tile state (%d,%d) tile=%d collision=%d", expected.X, expected.Y, expected.TileImageID, expected.CollisionType)
		}
	}
	for _, expected := range exp.ObjectStates {
		if !hasObjectState(r.ObjectStates, expected) {
			return fmt.Errorf("expected object state %s visible=%t", expected.Name, expected.Visible)
		}
	}
	if exp.VermilionGymTrash != nil {
		if r.Final.VermilionGymTrash == nil {
			return fmt.Errorf("expected Vermilion Gym trash state")
		}
		if exp.VermilionGymTrash.FirstLockCanIndex != nil && r.Final.VermilionGymTrash.FirstLockCanIndex != *exp.VermilionGymTrash.FirstLockCanIndex {
			return fmt.Errorf("expected Vermilion Gym first trash can %d, got %d",
				*exp.VermilionGymTrash.FirstLockCanIndex,
				r.Final.VermilionGymTrash.FirstLockCanIndex)
		}
		if exp.VermilionGymTrash.SecondLockAbsent && r.Final.VermilionGymTrash.SecondLockCanIndex != nil {
			return fmt.Errorf("expected no Vermilion Gym second trash can, got %d", *r.Final.VermilionGymTrash.SecondLockCanIndex)
		}
		if exp.VermilionGymTrash.SecondLockCanIndex != nil {
			if r.Final.VermilionGymTrash.SecondLockCanIndex == nil {
				return fmt.Errorf("expected Vermilion Gym second trash can %d, got none", *exp.VermilionGymTrash.SecondLockCanIndex)
			}
			if *r.Final.VermilionGymTrash.SecondLockCanIndex != *exp.VermilionGymTrash.SecondLockCanIndex {
				return fmt.Errorf("expected Vermilion Gym second trash can %d, got %d",
					*exp.VermilionGymTrash.SecondLockCanIndex,
					*r.Final.VermilionGymTrash.SecondLockCanIndex)
			}
		}
	}
	return nil
}

func finalPartyHasSpecies(party []PokemonSummary, speciesID int) bool {
	for _, p := range party {
		if p.SpeciesID == speciesID {
			return true
		}
	}
	return false
}

func validatePartyState(party []PokemonSummary, expected ExpectedPartyPokemon) error {
	for _, pokemon := range party {
		if pokemon.Slot != expected.Slot {
			continue
		}
		if expected.SpeciesID != 0 && pokemon.SpeciesID != expected.SpeciesID {
			return fmt.Errorf("expected party slot %d species #%d, got #%d", expected.Slot, expected.SpeciesID, pokemon.SpeciesID)
		}
		if expected.Level != 0 && pokemon.Level != expected.Level {
			return fmt.Errorf("expected party slot %d level %d, got %d", expected.Slot, expected.Level, pokemon.Level)
		}
		if expected.CurHP != nil && pokemon.CurHP != *expected.CurHP {
			return fmt.Errorf("expected party slot %d curHp %d, got %d", expected.Slot, *expected.CurHP, pokemon.CurHP)
		}
		if expected.MaxHP != nil && pokemon.MaxHP != *expected.MaxHP {
			return fmt.Errorf("expected party slot %d maxHp %d, got %d", expected.Slot, *expected.MaxHP, pokemon.MaxHP)
		}
		if expected.Status != "" && pokemon.Status != normalizeExpectedStatus(expected.Status) {
			return fmt.Errorf("expected party slot %d status %s, got %s", expected.Slot, normalizeExpectedStatus(expected.Status), pokemon.Status)
		}
		if expected.Exp != nil && pokemon.Exp != *expected.Exp {
			return fmt.Errorf("expected party slot %d exp %d, got %d", expected.Slot, *expected.Exp, pokemon.Exp)
		}
		for i, pp := range expected.MovePP {
			if i >= len(pokemon.MovePP) {
				return fmt.Errorf("expected party slot %d movePp index %d, got only %d moves", expected.Slot, i, len(pokemon.MovePP))
			}
			if pokemon.MovePP[i] != pp {
				return fmt.Errorf("expected party slot %d move %d PP %d, got %d", expected.Slot, i, pp, pokemon.MovePP[i])
			}
		}
		if expected.IVs != nil && pokemon.IVs != *expected.IVs {
			return fmt.Errorf("expected party slot %d IVs %+v, got %+v", expected.Slot, *expected.IVs, pokemon.IVs)
		}
		if expected.EVs != nil && pokemon.EVs != *expected.EVs {
			return fmt.Errorf("expected party slot %d EVs %+v, got %+v", expected.Slot, *expected.EVs, pokemon.EVs)
		}
		return nil
	}
	return fmt.Errorf("expected party slot %d, got no pokemon in that slot", expected.Slot)
}

func normalizeExpectedStatus(status string) string {
	parsed, err := parseFixturePokemonStatus(status)
	if err != nil {
		return status
	}
	return parsed.String()
}

func finalPCHasPokemon(pc []PCPokemonSummary, expected FixturePCPokemon) bool {
	for _, pokemon := range pc {
		if expected.SpeciesID > 0 && pokemon.SpeciesID != expected.SpeciesID {
			continue
		}
		if pokemon.Box != expected.Box || pokemon.Slot != expected.Slot {
			continue
		}
		if expected.Level > 0 && pokemon.Level != expected.Level {
			continue
		}
		return true
	}
	return false
}

func finalHiddenObjectID(objects []ObjectSummary, objectID int) bool {
	for _, obj := range objects {
		if obj.ObjectID == objectID {
			return true
		}
	}
	return false
}

func inventoryQuantity(items []ItemSummary, itemID int) int {
	total := 0
	for _, item := range items {
		if item.ItemID == itemID {
			total += item.Quantity
		}
	}
	return total
}

func effectsContainType(effects []ActionEffect, actionType string) bool {
	for _, effect := range effects {
		if effect.Type == actionType {
			return true
		}
	}
	return false
}

func activeBattleHasEnemySpecies(active *world.ActiveBattleSummary, speciesID int) bool {
	if active == nil {
		return false
	}
	for _, pokemon := range active.EnemyParty {
		if pokemon.SpeciesID == speciesID {
			return true
		}
	}
	return false
}

func sameStringList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func hasTileState(states []TileState, expected TileState) bool {
	for _, state := range states {
		if state.X != expected.X || state.Y != expected.Y {
			continue
		}
		if state.TileImageID != expected.TileImageID || state.CollisionType != expected.CollisionType {
			return false
		}
		if expected.Label != "" && state.Label != expected.Label {
			return false
		}
		return true
	}
	return false
}

func hasObjectState(states []ObjectState, expected ObjectState) bool {
	for _, state := range states {
		if state.Name != expected.Name {
			continue
		}
		if expected.MapName != "" && state.MapName != expected.MapName {
			continue
		}
		if expected.X != 0 && state.X != expected.X {
			continue
		}
		if expected.Y != 0 && state.Y != expected.Y {
			continue
		}
		if expected.Text != "" && state.Text != expected.Text {
			continue
		}
		if expected.Label != "" && state.Label != expected.Label {
			continue
		}
		return state.Visible == expected.Visible
	}
	return false
}

func elevatorFloorsMatch(actual []ElevatorFloorSummary, expected []ElevatorFloorExpected) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i, exp := range expected {
		floor := actual[i]
		if exp.Label != "" && floor.Label != exp.Label {
			return false
		}
		if exp.MapName != "" && floor.MapName != exp.MapName {
			return false
		}
		if exp.FloorMapID != 0 && floor.MapID != exp.FloorMapID {
			return false
		}
		if exp.DestX != 0 && floor.DestX != exp.DestX {
			return false
		}
		if exp.DestY != 0 && floor.DestY != exp.DestY {
			return false
		}
		if exp.RequiresItem != 0 && floor.RequiresItemID != exp.RequiresItem {
			return false
		}
	}
	return true
}

func gameCornerPrizesMatch(actual []GameCornerPrizeSummary, expected []GameCornerPrizeExpected) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i, exp := range expected {
		prize := actual[i]
		if exp.Name != "" && prize.Name != exp.Name {
			return false
		}
		if exp.Type != "" && prize.Type != exp.Type {
			return false
		}
		if exp.PokemonID != 0 && prize.PokemonID != exp.PokemonID {
			return false
		}
		if exp.ItemID != 0 && prize.ItemID != exp.ItemID {
			return false
		}
		if exp.CoinCost != 0 && prize.CoinCost != exp.CoinCost {
			return false
		}
	}
	return true
}

func gameCornerSlotMatches(actual GameCornerSlotSummary, expected GameCornerSlotExpected) bool {
	if expected.Success != nil && actual.Success != *expected.Success {
		return false
	}
	if expected.Bet != 0 && actual.Bet != expected.Bet {
		return false
	}
	if expected.Payout != nil && actual.Payout != *expected.Payout {
		return false
	}
	if expected.MatchLine != "" && actual.MatchLine != expected.MatchLine {
		return false
	}
	if expected.MatchSymbol != "" && actual.MatchSymbol != expected.MatchSymbol {
		return false
	}
	if len(expected.ReelPositions) > 0 && !intSlicesEqual(actual.ReelPositions, expected.ReelPositions) {
		return false
	}
	return true
}

func intSlicesEqual(actual, expected []int) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range expected {
		if actual[i] != expected[i] {
			return false
		}
	}
	return true
}

func validateSafariBattleExpectation(summary *SafariSummary, expected SafariBattleExpected) error {
	var battle *SafariBattleSummary
	if summary != nil {
		battle = summary.Battle
	}
	if expected.Active != nil {
		active := battle != nil
		if active != *expected.Active {
			return fmt.Errorf("expected safari battle active %t, got %t", *expected.Active, active)
		}
	}
	if battle == nil {
		if expected.PokemonID != 0 || expected.Level != 0 || expected.Caught != nil || expected.Fled != nil {
			return fmt.Errorf("expected safari battle details %+v, got none", expected)
		}
		return nil
	}
	if expected.PokemonID != 0 && battle.PokemonID != expected.PokemonID {
		return fmt.Errorf("expected safari battle pokemon #%d, got #%d", expected.PokemonID, battle.PokemonID)
	}
	if expected.Level != 0 && battle.Level != expected.Level {
		return fmt.Errorf("expected safari battle level %d, got %d", expected.Level, battle.Level)
	}
	if expected.Caught != nil && battle.Caught != *expected.Caught {
		return fmt.Errorf("expected safari battle caught %t, got %t", *expected.Caught, battle.Caught)
	}
	if expected.Fled != nil && battle.Fled != *expected.Fled {
		return fmt.Errorf("expected safari battle fled %t, got %t", *expected.Fled, battle.Fled)
	}
	return nil
}

func validateExpectedMessages(messages []RecordedMessage, expected []ExpectedMessage) error {
	for _, exp := range expected {
		found := false
		for _, message := range messages {
			if exp.Channel != "" && message.Channel != exp.Channel {
				continue
			}
			if exp.Opcode != 0 && int(message.Opcode) != exp.Opcode {
				continue
			}
			payload := string(message.Payload)
			missingPayload := false
			for _, part := range exp.PayloadContains {
				if !strings.Contains(payload, part) {
					missingPayload = true
					break
				}
			}
			if missingPayload {
				continue
			}
			found = true
			break
		}
		if !found {
			return fmt.Errorf(
				"expected recorded message channel=%q opcode=%d payloadContains=%v",
				exp.Channel,
				exp.Opcode,
				exp.PayloadContains,
			)
		}
	}
	return nil
}
