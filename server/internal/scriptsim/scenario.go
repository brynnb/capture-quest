package scriptsim

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"capturequest/internal/world"
)

type Scenario struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Fixture       Fixture         `json:"fixture"`
	Trigger       Trigger         `json:"trigger"`
	ResolveBattle *ResolveBattle  `json:"resolveBattle"`
	Expect        ExpectedOutcome `json:"expect"`
}

type ResolveBattle struct {
	Result string `json:"result"`
}

type Fixture struct {
	CharacterName     string                    `json:"characterName"`
	MapName           string                    `json:"mapName"`
	X                 int                       `json:"x"`
	Y                 int                       `json:"y"`
	Direction         string                    `json:"direction"`
	Flags             []string                  `json:"flags"`
	Party             []FixturePokemon          `json:"party"`
	PC                []FixturePCPokemon        `json:"pc,omitempty"`
	Inventory         []FixtureItem             `json:"inventory"`
	Money             int                       `json:"money"`
	Coins             int                       `json:"coins"`
	PokedexSeen       []int                     `json:"pokedexSeen"`
	PokedexCaught     []int                     `json:"pokedexCaught"`
	HiddenObjects     []string                  `json:"hiddenObjects"`
	ObjectPositions   []FixtureObjectPosition   `json:"objectPositions,omitempty"`
	ActiveBattle      *FixtureActiveBattle      `json:"activeBattle,omitempty"`
	Safari            *SafariFixture            `json:"safari,omitempty"`
	DayCare           *DayCareFixture           `json:"dayCare,omitempty"`
	Repel             *RepelFixture             `json:"repel,omitempty"`
	VermilionGymTrash *VermilionGymTrashFixture `json:"vermilionGymTrash,omitempty"`
}

type FixturePokemon struct {
	SpeciesID int      `json:"speciesId"`
	Level     int      `json:"level"`
	MoveIDs   []int    `json:"moveIds,omitempty"`
	Moves     []string `json:"moves,omitempty"`
	MovePP    []int    `json:"movePp,omitempty"`
	CurHP     *int     `json:"curHp,omitempty"`
	Status    string   `json:"status,omitempty"`
	Exp       *int     `json:"exp,omitempty"`
	IVs       *IVsSpec `json:"ivs,omitempty"`
	EVs       *EVsSpec `json:"evs,omitempty"`
}

type IVsSpec struct {
	Attack  int `json:"attack"`
	Defense int `json:"defense"`
	Speed   int `json:"speed"`
	Special int `json:"special"`
}

type EVsSpec struct {
	HP      int `json:"hp"`
	Attack  int `json:"attack"`
	Defense int `json:"defense"`
	Speed   int `json:"speed"`
	Special int `json:"special"`
}

type FixturePCPokemon struct {
	SpeciesID int `json:"speciesId"`
	Level     int `json:"level"`
	Box       int `json:"box"`
	Slot      int `json:"slot"`
}

type FixtureItem struct {
	ItemID   int    `json:"itemId"`
	ItemName string `json:"itemName"`
	Quantity int    `json:"quantity"`
}

type FixtureObjectPosition struct {
	MapName    string `json:"mapName,omitempty"`
	ObjectKey  string `json:"objectKey,omitempty"`
	ObjectName string `json:"objectName,omitempty"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
}

type RepelFixture struct {
	StepsLeft int `json:"stepsLeft"`
}

type SafariFixture struct {
	Active    bool                 `json:"active"`
	BallsLeft int                  `json:"ballsLeft"`
	StepsLeft int                  `json:"stepsLeft"`
	Battle    *SafariBattleFixture `json:"battle,omitempty"`
}

type SafariBattleFixture struct {
	PokemonID int `json:"pokemonId"`
	Level     int `json:"level"`
}

type DayCareFixture struct {
	Pokemon    FixturePokemon `json:"pokemon"`
	StartLevel int            `json:"startLevel,omitempty"`
}

type FixtureActiveBattle struct {
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

type VermilionGymTrashFixture struct {
	FirstLockCanIndex  *int `json:"firstLockCanIndex,omitempty"`
	SecondLockCanIndex *int `json:"secondLockCanIndex,omitempty"`
}

type Trigger struct {
	Type                     string `json:"type"`
	ScriptLabel              string `json:"scriptLabel"`
	MapName                  string `json:"mapName"`
	ObjectID                 int    `json:"objectId"`
	ObjectKey                string `json:"objectKey"`
	TextConstant             string `json:"textConstant"`
	Choice                   *bool  `json:"choice"`
	TrashCanIndex            int    `json:"trashCanIndex"`
	RandomFirstLockCanIndex  *int   `json:"randomFirstLockCanIndex,omitempty"`
	RandomSecondLockCanIndex *int   `json:"randomSecondLockCanIndex,omitempty"`
	HoleIndex                int    `json:"holeIndex,omitempty"`
	FloorMapName             string `json:"floorMapName,omitempty"`
	PrizeName                string `json:"prizeName,omitempty"`
	Repeat                   int    `json:"repeat,omitempty"`
	Bet                      int    `json:"bet,omitempty"`
	Action                   string `json:"action,omitempty"`
	ItemID                   int    `json:"itemId,omitempty"`
	PartySlot                int    `json:"partySlot,omitempty"`
	IsLucky                  bool   `json:"isLucky,omitempty"`
	RandomValues             []int  `json:"randomValues,omitempty"`
	MoveName                 string `json:"moveName,omitempty"`
	MoveID                   int    `json:"moveId,omitempty"`
	Direction                string `json:"direction,omitempty"`
	ActivateStrength         bool   `json:"activateStrength,omitempty"`
	X                        int    `json:"x"`
	Y                        int    `json:"y"`
	DestX                    int    `json:"destX,omitempty"`
	DestY                    int    `json:"destY,omitempty"`
}

type ExpectedOutcome struct {
	ScriptLabel                 string                     `json:"scriptLabel"`
	FinalMapName                string                     `json:"finalMapName"`
	FinalX                      *int                       `json:"finalX"`
	FinalY                      *int                       `json:"finalY"`
	FinalDirection              string                     `json:"finalDirection,omitempty"`
	FlagsPresent                []string                   `json:"flagsPresent"`
	FlagsAbsent                 []string                   `json:"flagsAbsent"`
	PartySpecies                []int                      `json:"partySpecies"`
	PartySpeciesAbsent          []int                      `json:"partySpeciesAbsent"`
	PartyState                  []ExpectedPartyPokemon     `json:"partyState,omitempty"`
	PCContains                  []FixturePCPokemon         `json:"pcContains,omitempty"`
	PCAbsent                    []FixturePCPokemon         `json:"pcAbsent,omitempty"`
	HiddenObjectKeys            []string                   `json:"hiddenObjectKeys"`
	InventoryContains           []FixtureItem              `json:"inventoryContains"`
	InventoryAbsent             []FixtureItem              `json:"inventoryAbsent"`
	Money                       *int                       `json:"money"`
	Coins                       *int                       `json:"coins"`
	PokedexSeenCount            *int                       `json:"pokedexSeenCount"`
	PokedexCaughtCount          *int                       `json:"pokedexCaughtCount"`
	ElevatorFloors              []ElevatorFloorExpected    `json:"elevatorFloors,omitempty"`
	ElevatorMessage             string                     `json:"elevatorMessage,omitempty"`
	GameCornerPrizes            []GameCornerPrizeExpected  `json:"gameCornerPrizes,omitempty"`
	GameCornerPrizeMessage      string                     `json:"gameCornerPrizeMessage,omitempty"`
	GameCornerSlot              *GameCornerSlotExpected    `json:"gameCornerSlot,omitempty"`
	FieldMove                   *FieldMoveExpected         `json:"fieldMove,omitempty"`
	BoulderPush                 *BoulderPushExpected       `json:"boulderPush,omitempty"`
	Pathfind                    *PathfindExpected          `json:"pathfind,omitempty"`
	SafariActive                *bool                      `json:"safariActive"`
	SafariBallsLeft             *int                       `json:"safariBallsLeft"`
	SafariStepsLeft             *int                       `json:"safariStepsLeft"`
	SafariBattle                *SafariBattleExpected      `json:"safariBattle,omitempty"`
	DayCare                     *DayCareExpected           `json:"dayCare,omitempty"`
	Repel                       *RepelExpected             `json:"repel,omitempty"`
	Messages                    []ExpectedMessage          `json:"messages,omitempty"`
	ActionTypesContains         []string                   `json:"actionTypesContains"`
	ActionTypesAbsent           []string                   `json:"actionTypesAbsent,omitempty"`
	ActiveBattleType            string                     `json:"activeBattleType"`
	ActiveBattleTrainerClass    string                     `json:"activeBattleTrainerClass"`
	ActiveBattleEnemySpecies    []int                      `json:"activeBattleEnemySpecies"`
	ActiveBattleAllowedActions  []string                   `json:"activeBattleAllowedActions,omitempty"`
	ActiveBattleGuaranteedCatch *bool                      `json:"activeBattleGuaranteedCatch,omitempty"`
	ActiveBattleAbsent          bool                       `json:"activeBattleAbsent"`
	SeafoamSurfBlocked          *bool                      `json:"seafoamSurfBlocked,omitempty"`
	TileStates                  []TileState                `json:"tileStates"`
	ObjectStates                []ObjectState              `json:"objectStates"`
	VermilionGymTrash           *VermilionGymTrashExpected `json:"vermilionGymTrash,omitempty"`
}

type RepelExpected struct {
	Success   *bool  `json:"success,omitempty"`
	Active    *bool  `json:"active,omitempty"`
	StepsLeft *int   `json:"stepsLeft,omitempty"`
	Message   string `json:"message,omitempty"`
	WoreOff   *bool  `json:"woreOff,omitempty"`
}

type SafariBattleExpected struct {
	Active    *bool `json:"active,omitempty"`
	PokemonID int   `json:"pokemonId,omitempty"`
	Level     int   `json:"level,omitempty"`
	Caught    *bool `json:"caught,omitempty"`
	Fled      *bool `json:"fled,omitempty"`
}

type DayCareExpected struct {
	Active      *bool `json:"active,omitempty"`
	PokemonID   int   `json:"pokemonId,omitempty"`
	Level       int   `json:"level,omitempty"`
	StartLevel  int   `json:"startLevel,omitempty"`
	LevelsGrown *int  `json:"levelsGrown,omitempty"`
	Cost        *int  `json:"cost,omitempty"`
	Exp         *int  `json:"exp,omitempty"`
}

type ExpectedPartyPokemon struct {
	Slot      int      `json:"slot"`
	SpeciesID int      `json:"speciesId,omitempty"`
	Level     int      `json:"level,omitempty"`
	CurHP     *int     `json:"curHp,omitempty"`
	MaxHP     *int     `json:"maxHp,omitempty"`
	Status    string   `json:"status,omitempty"`
	Exp       *int     `json:"exp,omitempty"`
	MovePP    []int    `json:"movePp,omitempty"`
	IVs       *IVsSpec `json:"ivs,omitempty"`
	EVs       *EVsSpec `json:"evs,omitempty"`
}

type ExpectedMessage struct {
	Channel         string   `json:"channel,omitempty"`
	Opcode          int      `json:"opcode,omitempty"`
	PayloadContains []string `json:"payloadContains,omitempty"`
}

type VermilionGymTrashExpected struct {
	FirstLockCanIndex  *int `json:"firstLockCanIndex,omitempty"`
	SecondLockCanIndex *int `json:"secondLockCanIndex,omitempty"`
	SecondLockAbsent   bool `json:"secondLockAbsent,omitempty"`
}

type TileState struct {
	X             int    `json:"x"`
	Y             int    `json:"y"`
	TileImageID   int    `json:"tileImageId"`
	CollisionType int    `json:"collisionType"`
	Label         string `json:"label,omitempty"`
}

type ObjectState struct {
	MapName string `json:"mapName,omitempty"`
	Name    string `json:"name"`
	Text    string `json:"text,omitempty"`
	X       int    `json:"x,omitempty"`
	Y       int    `json:"y,omitempty"`
	Visible bool   `json:"visible"`
	Label   string `json:"label,omitempty"`
}

type ElevatorFloorExpected struct {
	MapName      string `json:"mapName,omitempty"`
	FloorMapID   int    `json:"floorMapId,omitempty"`
	Label        string `json:"label"`
	DestX        int    `json:"destX,omitempty"`
	DestY        int    `json:"destY,omitempty"`
	RequiresItem int    `json:"requiresItem,omitempty"`
}

type GameCornerPrizeExpected struct {
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	PokemonID int    `json:"pokemonId,omitempty"`
	ItemID    int    `json:"itemId,omitempty"`
	CoinCost  int    `json:"coinCost,omitempty"`
}

type GameCornerSlotExpected struct {
	Success       *bool  `json:"success,omitempty"`
	Bet           int    `json:"bet,omitempty"`
	Payout        *int   `json:"payout,omitempty"`
	MatchLine     string `json:"matchLine,omitempty"`
	MatchSymbol   string `json:"matchSymbol,omitempty"`
	ReelPositions []int  `json:"reelPositions,omitempty"`
}

type FieldMoveExpected struct {
	Allowed           *bool  `json:"allowed,omitempty"`
	Message           string `json:"message,omitempty"`
	MoveName          string `json:"moveName,omitempty"`
	RequiredBadgeFlag string `json:"requiredBadgeFlag,omitempty"`
	KnownBySpeciesID  int    `json:"knownBySpeciesId,omitempty"`
}

type BoulderPushExpected struct {
	Success    *bool  `json:"success,omitempty"`
	Message    string `json:"message,omitempty"`
	ObjectName string `json:"objectName,omitempty"`
	FromX      int    `json:"fromX,omitempty"`
	FromY      int    `json:"fromY,omitempty"`
	ToX        int    `json:"toX,omitempty"`
	ToY        int    `json:"toY,omitempty"`
	Dropped    *bool  `json:"dropped,omitempty"`
	FlagSet    string `json:"flagSet,omitempty"`
}

type PathfindExpected struct {
	Found    *bool            `json:"found,omitempty"`
	Length   int              `json:"length,omitempty"`
	Avoids   []world.PathNode `json:"avoids,omitempty"`
	Contains []world.PathNode `json:"contains,omitempty"`
}

func LoadScenario(path string) (*Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var scenario Scenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		return nil, fmt.Errorf("parse scenario %s: %w", path, err)
	}
	if scenario.Name == "" {
		scenario.Name = trimScenarioExt(filepath.Base(path))
	}
	if scenario.Fixture.Direction == "" {
		scenario.Fixture.Direction = "DOWN"
	}
	if scenario.Trigger.MapName == "" {
		scenario.Trigger.MapName = scenario.Fixture.MapName
	}
	return &scenario, nil
}

func ScenarioPath(nameOrPath string) string {
	if filepath.Ext(nameOrPath) != "" || filepath.Dir(nameOrPath) != "." {
		return nameOrPath
	}
	return filepath.Join("script_tests", "scenarios", nameOrPath+".json")
}

func GoldenPath(name string) string {
	return filepath.Join("script_tests", "golden", name+".golden")
}

func trimScenarioExt(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return name
	}
	return name[:len(name)-len(ext)]
}
