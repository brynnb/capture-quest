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
