package scriptcandidateimport

import (
	"encoding/json"
	"errors"

	"capturequest/internal/scriptedevents"
)

var ErrChangesRequired = errors.New("script candidate outputs are not current")

const (
	extractorSource                      = "extractor"
	capturequestSource                   = "capturequest"
	generatedObjectVisibilityFileName    = "object_visibility.generated.json"
	generatedEventTilesFileName          = "event_tile_overrides.generated.json"
	generatedConditionalDialogueFileName = "conditional_dialogue.generated.json"
)

type Options struct {
	SQLitePath      string
	OutputDir       string
	DiagnosticsPath string
	Release         string
	DryRun          bool
	Check           bool
	Report          func(ReportEvent)
}

type ReportEvent struct {
	Kind string
	Path string
}

type Stats struct {
	Read                         int `json:"read"`
	Written                      int `json:"written"`
	Unchanged                    int `json:"unchanged"`
	SkippedOverrides             int `json:"skippedOverrides"`
	SkippedUnsupported           int `json:"skippedUnsupported"`
	TileOverrideRead             int `json:"tileOverrideRead"`
	TileOverrideRules            int `json:"tileOverrideRules"`
	TileOverrideWritten          int `json:"tileOverrideWritten"`
	TileOverrideUnchanged        int `json:"tileOverrideUnchanged"`
	TileOverrideSkippedOverrides int `json:"tileOverrideSkippedOverrides"`
	TileOverrideUnsupported      int `json:"tileOverrideUnsupported"`
	ObjectVisibilityRead         int `json:"objectVisibilityRead"`
	ObjectVisibilityRules        int `json:"objectVisibilityRules"`
	ObjectVisibilityWritten      int `json:"objectVisibilityWritten"`
	ObjectVisibilityUnchanged    int `json:"objectVisibilityUnchanged"`
	ObjectVisibilityUnsupported  int `json:"objectVisibilityUnsupported"`
	ConditionalDialogueRead      int `json:"conditionalDialogueRead"`
	ConditionalDialogueRules     int `json:"conditionalDialogueRules"`
	ConditionalDialogueWritten   int `json:"conditionalDialogueWritten"`
	ConditionalDialogueUnchanged int `json:"conditionalDialogueUnchanged"`
	ExtractorUnsupported         int `json:"extractorUnsupported"`
	ExtractorAmbiguous           int `json:"extractorAmbiguous"`
	ExtractorGenerated           int `json:"extractorGenerated"`
	ExtractorDiagnostics         int `json:"extractorDiagnostics"`
}

type scriptCandidate struct {
	Version     int                `json:"version"`
	Kind        string             `json:"kind"`
	MapName     string             `json:"mapName"`
	ScriptLabel string             `json:"scriptLabel"`
	Trigger     candidateTrigger   `json:"trigger"`
	Conditions  candidateCondition `json:"conditions"`
	Actions     []candidateAction  `json:"actions"`
	Confidence  string             `json:"confidence"`
}

type tileOverrideCandidate struct {
	Version      int                       `json:"version"`
	Kind         string                    `json:"kind"`
	MapName      string                    `json:"mapName"`
	ScriptLabel  string                    `json:"scriptLabel"`
	Replacements []tileOverrideReplacement `json:"replacements"`
	Confidence   string                    `json:"confidence"`
}

type conditionalDialogueCandidate struct {
	Version           int                `json:"version"`
	Kind              string             `json:"kind"`
	MapName           string             `json:"mapName"`
	ScriptLabel       string             `json:"scriptLabel"`
	SourceScriptLabel string             `json:"sourceScriptLabel"`
	TextConstant      string             `json:"textConstant"`
	Priority          int                `json:"priority"`
	Conditions        candidateCondition `json:"conditions"`
	DialogueLabels    []string           `json:"dialogueLabels"`
	Source            map[string]any     `json:"source"`
	Confidence        string             `json:"confidence"`
}

type objectVisibilityCandidate struct {
	Version       int            `json:"version"`
	Kind          string         `json:"kind"`
	MapName       string         `json:"mapName"`
	MapID         int            `json:"mapId"`
	ObjectName    string         `json:"objectName"`
	ObjectKey     string         `json:"objectKey"`
	Visible       bool           `json:"visible"`
	RequiresEvent string         `json:"requiresEvent"`
	Label         string         `json:"label"`
	SourceMapName string         `json:"sourceMapName"`
	ScriptLabel   string         `json:"scriptLabel"`
	Source        map[string]any `json:"source"`
	Confidence    string         `json:"confidence"`
}

type tileOverrideReplacement struct {
	BlockX              int    `json:"blockX"`
	BlockY              int    `json:"blockY"`
	BlockID             int    `json:"blockId"`
	RequiresEvent       string `json:"requiresEvent"`
	RequiresEventAbsent string `json:"requiresEventAbsent"`
	LabelPrefix         string `json:"labelPrefix"`
}

type candidateTrigger struct {
	Type        string                           `json:"type"`
	Label       string                           `json:"label"`
	Coordinates []scriptedevents.EventCoordinate `json:"coordinates"`
}

type candidateCondition struct {
	RequiresEvent         string   `json:"requiresEvent"`
	RequiresEventAbsent   string   `json:"requiresEventAbsent"`
	RequiresEvents        []string `json:"requiresEvents"`
	RequiresEventsAbsent  []string `json:"requiresEventsAbsent"`
	RequiresBadge         string   `json:"requiresBadge"`
	RequiresBadgeAbsent   string   `json:"requiresBadgeAbsent"`
	RequiresBadges        []string `json:"requiresBadges"`
	RequiresBadgesAbsent  []string `json:"requiresBadgesAbsent"`
	RequiresItem          string   `json:"requiresItem"`
	RequiresItemAbsent    string   `json:"requiresItemAbsent"`
	RequiresPokedexCaught int      `json:"requiresPokedexCaught"`
	RequiresMoney         int      `json:"requiresMoney"`
	RequiresMoneyBelow    int      `json:"requiresMoneyBelow"`
	RequiresCoins         int      `json:"requiresCoins"`
	RequiresCoinsBelow    int      `json:"requiresCoinsBelow"`
	RequiresPlayerFacing  string   `json:"requiresPlayerFacing"`
}

type candidateAction struct {
	Type              string                `json:"type"`
	Speaker           string                `json:"speaker"`
	Lines             []string              `json:"lines"`
	Prompt            string                `json:"prompt"`
	PromptLines       []string              `json:"promptLines"`
	YesLines          []string              `json:"yesLines"`
	NoLines           []string              `json:"noLines"`
	ContinueOnNo      bool                  `json:"continueOnNo"`
	StopOnYes         bool                  `json:"stopOnYes"`
	Destination       *candidateDestination `json:"destination"`
	Event             string                `json:"event"`
	Flag              string                `json:"flag"`
	ItemID            int                   `json:"itemId"`
	ItemName          string                `json:"itemName"`
	ItemConstant      string                `json:"itemConstant"`
	Quantity          int                   `json:"quantity"`
	PokemonID         int                   `json:"pokemonId"`
	SpeciesID         int                   `json:"speciesId"`
	PokemonName       string                `json:"pokemonName"`
	PokemonConstant   string                `json:"pokemonConstant"`
	SFXConstant       string                `json:"sfxConstant"`
	Volume            float64               `json:"volume"`
	Level             int                   `json:"level"`
	Message           string                `json:"message"`
	Money             int                   `json:"money"`
	Coins             int                   `json:"coins"`
	Actor             string                `json:"actor"`
	Movements         []string              `json:"movements"`
	MS                int                   `json:"ms"`
	MapID             int                   `json:"mapId"`
	X                 int                   `json:"x"`
	Y                 int                   `json:"y"`
	Direction         string                `json:"direction"`
	ObjectID          int                   `json:"objectId"`
	ObjectKey         string                `json:"objectKey"`
	ObjectMapName     string                `json:"objectMapName"`
	TriggerLabel      string                `json:"triggerLabel"`
	TextConstant      string                `json:"textConstant"`
	TrainerClass      string                `json:"trainerClass"`
	TrainerPartyIndex int                   `json:"partyIndex"`
	PartyByFlag       map[string]int        `json:"partyByFlag"`
	TrainerName       string                `json:"trainerName"`
	TrainerObjectID   int                   `json:"trainerObjectId"`
	WinFlag           string                `json:"winFlag"`
	LoseFlag          string                `json:"loseFlag"`
	LossMessage       string                `json:"lossMessage"`
	NoBlackoutOnLoss  bool                  `json:"noBlackoutOnLoss"`
	PostWinActions    []candidateAction     `json:"postWinActions"`
	PostLoseActions   []candidateAction     `json:"postLoseActions"`
	AllowedActions    []string              `json:"allowedActions"`
	GuaranteedCatch   bool                  `json:"guaranteedCatch"`
	PrizeWindow       int                   `json:"prizeWindow"`
}

type candidateDestination struct {
	MapName   string `json:"mapName"`
	MapID     int    `json:"mapId"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Direction string `json:"direction"`
}

type existingScript struct {
	Path        string
	Source      string
	ScriptLabel string
}

type existingScripts struct {
	ByLabel      map[string]existingScript
	ByTrigger    map[string]existingScript
	ByMapSetFlag map[string]existingScript
}

type importDecision struct {
	MapName     string          `json:"mapName"`
	ScriptLabel string          `json:"scriptLabel"`
	Status      string          `json:"status"`
	Reason      string          `json:"reason,omitempty"`
	Path        string          `json:"path,omitempty"`
	Details     json.RawMessage `json:"details,omitempty"`
}

type extractorDiagnostic struct {
	MapName     string          `json:"mapName"`
	ScriptLabel string          `json:"scriptLabel"`
	Status      string          `json:"status"`
	Reason      string          `json:"reason"`
	Details     json.RawMessage `json:"details"`
}

type importReport struct {
	DryRun               bool                  `json:"dryRun"`
	Stats                Stats                 `json:"stats"`
	Summary              importReportSummary   `json:"summary"`
	Decisions            []importDecision      `json:"decisions"`
	ExtractorDiagnostics []extractorDiagnostic `json:"extractorDiagnostics"`
}

type importReportSummary struct {
	DecisionsByStatus        map[string]int `json:"decisionsByStatus"`
	DecisionsByReason        map[string]int `json:"decisionsByReason"`
	ExtractorByStatus        map[string]int `json:"extractorByStatus"`
	ExtractorByReason        map[string]int `json:"extractorByReason"`
	GeneratedByAdapter       map[string]int `json:"generatedByAdapter"`
	SkippedOverridesByReason map[string]int `json:"skippedOverridesByReason"`
	UnsupportedByReason      map[string]int `json:"unsupportedByReason"`
}
