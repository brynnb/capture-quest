// Package scriptedactions defines the transport-neutral action contract shared
// by candidate compilation, runtime execution, and offline simulation.
package scriptedactions

type Action struct {
	Type              string         `json:"type"`
	Speaker           string         `json:"speaker,omitempty"`
	Lines             []string       `json:"lines,omitempty"`
	Actor             string         `json:"actor,omitempty"`
	Sprite            string         `json:"sprite,omitempty"`
	Movements         []string       `json:"movements,omitempty"`
	PokemonID         int            `json:"pokemonId,omitempty"`
	SpeciesID         int            `json:"speciesId,omitempty"`
	PokemonName       string         `json:"pokemonName,omitempty"`
	PokemonConstant   string         `json:"pokemonConstant,omitempty"`
	SFXConstant       string         `json:"sfxConstant,omitempty"`
	MusicConstant     string         `json:"musicConstant,omitempty"`
	MusicPath         string         `json:"musicPath,omitempty"`
	Loop              *bool          `json:"loop,omitempty"`
	Volume            float64        `json:"volume,omitempty"`
	Level             int            `json:"level,omitempty"`
	Flag              string         `json:"flag,omitempty"`
	Message           string         `json:"message,omitempty"`
	Money             int            `json:"money,omitempty"`
	Coins             int            `json:"coins,omitempty"`
	ItemID            int            `json:"itemId,omitempty"`
	ItemName          string         `json:"itemName,omitempty"`
	Quantity          int            `json:"quantity,omitempty"`
	MapID             int            `json:"mapId,omitempty"`
	X                 int            `json:"x,omitempty"`
	Y                 int            `json:"y,omitempty"`
	Direction         string         `json:"direction,omitempty"`
	ObjectID          int            `json:"objectId,omitempty"`
	ObjectKey         string         `json:"objectKey,omitempty"`
	ObjectMapName     string         `json:"objectMapName,omitempty"`
	TriggerLabel      string         `json:"triggerLabel,omitempty"`
	TextConstant      string         `json:"textConstant,omitempty"`
	Prompt            string         `json:"prompt,omitempty"`
	YesLines          []string       `json:"yesLines,omitempty"`
	NoLines           []string       `json:"noLines,omitempty"`
	ContinueOnNo      bool           `json:"continueOnNo,omitempty"`
	StopOnYes         bool           `json:"stopOnYes,omitempty"`
	ActorID           int            `json:"actorId,omitempty"`
	TrainerClass      string         `json:"trainerClass,omitempty"`
	TrainerPartyIndex int            `json:"partyIndex,omitempty"`
	PartyByFlag       map[string]int `json:"partyByFlag,omitempty"`
	TrainerName       string         `json:"trainerName,omitempty"`
	TrainerObjectID   int            `json:"trainerObjectId,omitempty"`
	WinFlag           string         `json:"winFlag,omitempty"`
	LoseFlag          string         `json:"loseFlag,omitempty"`
	LossMessage       string         `json:"lossMessage,omitempty"`
	NoBlackoutOnLoss  bool           `json:"noBlackoutOnLoss,omitempty"`
	PostWinActions    []Action       `json:"postWinActions,omitempty"`
	PostLoseActions   []Action       `json:"postLoseActions,omitempty"`
	AllowedActions    []string       `json:"allowedActions,omitempty"`
	GuaranteedCatch   bool           `json:"guaranteedCatch,omitempty"`
	PrizeWindow       int            `json:"prizeWindow,omitempty"`
	Actions           []Action       `json:"actions,omitempty"`
	MS                int            `json:"ms,omitempty"`
}
