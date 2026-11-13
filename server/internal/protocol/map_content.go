package protocol

import "encoding/json"

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

// PhaserMapLoadRequest loads the owned location. Teleport intent belongs to
// the explicit warp commands; destination coordinates are never load inputs.
type PhaserMapLoadRequest struct {
	MapID     int    `json:"mapId"`
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
	SafariVisitState       `tstype:",extends"`
	WarpTileTeleportNotify `tstype:",extends"`
	Message                string `json:"message"`
}

// PlayerStepRequest proposes one direction from an expected owned source.
// The server resolves the target and issues a session-bound animation token.
type PlayerStepRequest struct {
	MapID     int    `json:"mapId"`
	FromX     *int   `json:"fromX" tstype:"number,required"`
	FromY     *int   `json:"fromY" tstype:"number,required"`
	Direction string `json:"direction"`
	RequestID string `json:"requestId"`
}
type PlayerStepResponse struct {
	Success   bool   `json:"success" tstype:"true"`
	RequestID string `json:"requestId"`
	StepToken string `json:"stepToken"`
	MapID     int    `json:"mapId"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Direction string `json:"direction"`
	LedgeJump bool   `json:"ledgeJump"`
}

// PlayerStepCompleteRequest acknowledges issued movement, never coordinates.
type PlayerStepCompleteRequest struct {
	StepToken string `json:"stepToken"`
	RequestID string `json:"requestId"`
}
type PlayerStepCompleteResponse struct {
	Replayed  bool   `json:"replayed,omitempty"`
	Success   bool   `json:"success" tstype:"true"`
	RequestID string `json:"requestId"`
	MapID     int    `json:"mapId"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Direction string `json:"direction"`
}

// PlayerStepError returns the owned location so rejected animations can reconcile.
type PlayerStepError struct {
	Success               bool   `json:"success" tstype:"false"`
	RequestID             string `json:"requestId"`
	Error                 string `json:"error"`
	MapID                 int    `json:"mapId"`
	X                     int    `json:"x"`
	Y                     int    `json:"y"`
	Direction             string `json:"direction"`
	ServerMovementPending bool   `json:"serverMovementPending"`
}

// Facing validates an expected owned source and cannot supply a destination.
type PlayerFacingRequest struct {
	MapID     int    `json:"mapId"`
	FromX     *int   `json:"fromX" tstype:"number,required"`
	FromY     *int   `json:"fromY" tstype:"number,required"`
	Direction string `json:"direction"`
	RequestID string `json:"requestId"`
}
type PlayerFacingResponse struct {
	Success               bool   `json:"success" tstype:"true"`
	RequestID             string `json:"requestId"`
	MapID                 int    `json:"mapId"`
	X                     int    `json:"x"`
	Y                     int    `json:"y"`
	Direction             string `json:"direction"`
	ServerMovementPending bool   `json:"serverMovementPending"`
}

// Server-controlled path positions are committed before projection to the owner.
type ServerPlayerMovementNotify struct {
	SpriteName   string `json:"spriteName"`
	ActorID      int    `json:"actorId"`
	MapID        int    `json:"mapId"`
	X            int    `json:"x"`
	Y            int    `json:"y"`
	Direction    string `json:"direction"`
	MoveSpeed    int    `json:"moveSpeed"`
	PathFinished bool   `json:"pathFinished"`
}

// Cutscene completion carries authorization/correlation only, never coordinates.
type CutsceneEndRequest struct {
	Cancel          bool   `json:"cancel,omitempty"`
	CompletionToken string `json:"completionToken"`
	ScriptLabel     string `json:"scriptLabel"`
	RequestID       string `json:"requestId"`
}

// CommittedPlayerStep is a historical receipt, never a new position command.
type CommittedPlayerStep struct {
	StepToken string `json:"stepToken"`
	MapID     int    `json:"mapId"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Direction string `json:"direction"`
}

type OwnedPlayerPositionRequest struct {
	RequestID string `json:"requestId"`
	StepToken string `json:"stepToken,omitempty"`
}
type OwnedPlayerPositionResponse struct {
	CommittedStep         *CommittedPlayerStep `json:"committedStep,omitempty"`
	Success               bool                 `json:"success" tstype:"true"`
	RequestID             string               `json:"requestId"`
	MapID                 int                  `json:"mapId"`
	X                     int                  `json:"x"`
	Y                     int                  `json:"y"`
	Direction             string               `json:"direction"`
	ServerMovementPending bool                 `json:"serverMovementPending"`
}
type CutsceneEndResponse struct {
	OwnedPlayerPositionResponse `tstype:",extends"`
	Completed                   bool `json:"completed"`
	Replayed                    bool `json:"replayed"`
}

// Actions retain the shared compiler/runtime/simulator contract.
type CutsceneStartNotify struct {
	ScriptLabel     string          `json:"scriptLabel"`
	CompletionToken string          `json:"completionToken"`
	MapName         string          `json:"mapName"`
	Actions         json.RawMessage `json:"actions" tstype:"import(\"./scriptedactions\").Action[]"`
}

// TrainerEncounterNotifyPayload is sent to the client when a trainer spots the player.
// The client should show "!" and animate the trainer locally to ApproachToX/Y.
// The server owns the committed player position; trainer approach is presentation.
type TrainerEncounterNotifyPayload struct {
	EncounterToken string `json:"encounterToken"`
	TrainerActorID int    `json:"trainerActorId"` // Runtime actor ID (from ActorRegistry)
	TrainerX       int    `json:"trainerX"`       // Trainer's current position
	TrainerY       int    `json:"trainerY"`
	PlayerX        int    `json:"playerX"`
	PlayerY        int    `json:"playerY"`
	ApproachToX    int    `json:"approachToX"` // Client-only trainer destination adjacent to the player
	ApproachToY    int    `json:"approachToY"`
	WalkToX        int    `json:"walkToX"` // Legacy: current player tile so old clients do not force-walk
	WalkToY        int    `json:"walkToY"`
	TrainerClass   string `json:"trainerClass"`
	TrainerName    string `json:"trainerName"`
}

// TrainerEncounterReadyRequest is sent by the client when the local trainer
// approach animation finishes and the battle can start.
type TrainerEncounterReadyRequest struct {
	EncounterToken string `json:"encounterToken"`
	TrainerActorID int    `json:"trainerActorId"`
}

// SafariVisitState scopes frequent HUD updates to one durable visit publication.
type SafariVisitState struct {
	CharacterID   int64  `json:"characterId"`
	VisitID       string `json:"visitId"`
	VisitRevision int64  `json:"visitRevision"`
	Active        bool   `json:"active"`
	BallsLeft     int    `json:"ballsLeft"`
	StepsLeft     int    `json:"stepsLeft"`
}
