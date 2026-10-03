package protocol

type PhaserMapScriptsRequest struct {
	MapName string `json:"mapName"` // Runtime map name
}

type PhaserLearnsetRequest struct {
	PokemonID int `json:"pokemonId"`
}

type PhaserLearnsetEntry struct {
	Level    int    `json:"level"`
	MoveName string `json:"moveName"`
	MoveID   *int   `json:"moveId" tstype:"number | null,required"`
}

type PhaserTMHMEntry struct {
	TMHMName string `json:"tmHmName"`
	MoveName string `json:"moveName"`
	MoveID   *int   `json:"moveId" tstype:"number | null,required"`
	IsHM     int    `json:"isHm"`
}

type PhaserMapScript struct {
	ScriptIndex    int     `json:"scriptIndex"`
	ScriptLabel    string  `json:"scriptLabel"`
	ScriptConstant string  `json:"scriptConstant"`
	RawASM         *string `json:"rawAsm" tstype:"string | null,required"`
}

type PhaserEventFlag struct {
	FlagName     string  `json:"flagName"`
	Operation    string  `json:"operation"`
	ContextLabel *string `json:"contextLabel" tstype:"string | null,required"`
}

type PhaserCoordinateTrigger struct {
	Label string `json:"label"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
}

type PhaserNPCMovement struct {
	Label     string `json:"label"`
	Movements string `json:"movements"` // JSON array string
}

type PhaserMapScriptsResponse struct {
	Success            bool                      `json:"success" tstype:"true"`
	MapName            string                    `json:"mapName"`
	Scripts            []PhaserMapScript         `json:"scripts"`
	EventFlags         []PhaserEventFlag         `json:"eventFlags"`
	CoordinateTriggers []PhaserCoordinateTrigger `json:"coordinateTriggers"`
	NPCMovements       []PhaserNPCMovement       `json:"npcMovements"`
}

type PhaserLearnsetResponse struct {
	Success   bool                  `json:"success" tstype:"true"`
	PokemonID int                   `json:"pokemonId"`
	Learnset  []PhaserLearnsetEntry `json:"learnset"`
	TMHM      []PhaserTMHMEntry     `json:"tmhm"`
}

// PhaserMapInfo represents a map in the 2D Phaser game
type PhaserMapInfo struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	TilesetID   *int   `json:"tilesetId,omitempty"`
	IsOverworld int    `json:"isOverworld"`
	TileMinX    *int   `json:"tileMinX,omitempty"`
	TileMinY    *int   `json:"tileMinY,omitempty"`
	TileMaxX    *int   `json:"tileMaxX,omitempty"`
	TileMaxY    *int   `json:"tileMaxY,omitempty"`
}

// PhaserMapInfoRequest only reads metadata; gameplay destinations use MapLoad.
type PhaserMapInfoRequest struct {
	MapID     int    `json:"mapId"`
	RequestID string `json:"requestId"`
}

type PhaserMapInfoResponse struct {
	PhaserMapInfo `tstype:",extends"`
	Success       bool   `json:"success" tstype:"true"`
	RequestID     string `json:"requestId"`
}

type PhaserMapLoadRequest struct {
	MapID     int    `json:"mapId"`
	DestX     *int   `json:"destX,omitempty"`
	DestY     *int   `json:"destY,omitempty"`
	RequestID string `json:"requestId"`
}

type PhaserMapLoadResponse struct {
	Success   bool   `json:"success" tstype:"true"`
	RequestID string `json:"requestId"`
	MapID     int    `json:"mapId"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
}

type PhaserMapRequestError struct {
	Success   bool   `json:"success" tstype:"false"`
	RequestID string `json:"requestId"`
	Error     string `json:"error"`
}

// Ordinary map warps are server-resolved; this request cannot name a destination.
type PhaserWarpActivateRequest struct {
	WarpID      int    `json:"warpId"`
	Direction   string `json:"direction"`
	InputSource string `json:"inputSource" tstype:"\"click\" | \"keyboard\""`
	RequestID   string `json:"requestId"`
}

type PhaserWarpActivateResponse struct {
	Success         bool   `json:"success" tstype:"true"`
	RequestID       string `json:"requestId"`
	MapID           int    `json:"mapId"`
	PlayerMapID     int    `json:"playerMapId"`
	X               int    `json:"x"`
	Y               int    `json:"y"`
	Direction       string `json:"direction"`
	AnimateExitStep bool   `json:"animateExitStep,omitempty"`
	AnimationStartX *int   `json:"animationStartX,omitempty"`
	AnimationStartY *int   `json:"animationStartY,omitempty"`
}

// Instant Warp is an explicit ordinary-player catalog destination command.
type PhaserInstantWarpRequest struct {
	MapID     int    `json:"mapId"`
	X         *int   `json:"x" tstype:"number,required"`
	Y         *int   `json:"y" tstype:"number,required"`
	Direction string `json:"direction"`
	RequestID string `json:"requestId"`
}

type PhaserInstantWarpResponse struct {
	Success   bool   `json:"success" tstype:"true"`
	RequestID string `json:"requestId"`
	MapID     int    `json:"mapId"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Direction string `json:"direction"`
}

// WarpTileTeleportNotify describes a destination already committed by the server.
// Arrival effects for legacy producers still run during the following MapLoad.
type WarpTileTeleportNotify struct {
	MapID     int    `json:"mapId"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Direction string `json:"direction"`
}

type SafariZoneExitNotify struct {
	WarpTileTeleportNotify `tstype:",extends"`
	StepsLeft              int    `json:"stepsLeft"`
	BallsLeft              int    `json:"ballsLeft"`
	Message                string `json:"message"`
}
