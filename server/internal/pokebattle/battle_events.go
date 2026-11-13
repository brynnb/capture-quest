package pokebattle

// BattleEvent represents something that happened during a turn, sent to the client for display.
type BattleEvent struct {
	Type    BattleEventType `json:"type"`
	Message string          `json:"message,omitempty"`

	// Move-related
	AttackerName  string `json:"attackerName,omitempty"`
	AttackerSide  string `json:"attackerSide,omitempty"` // "player" or "enemy"
	MoveName      string `json:"moveName,omitempty"`
	MoveSFX       string `json:"moveSfx,omitempty"`
	MoveSFXPitch  int    `json:"moveSfxPitch,omitempty"`
	MoveSFXTempo  int    `json:"moveSfxTempo,omitempty"`
	Damage        int    `json:"damage,omitempty"`
	IsCritical    bool   `json:"isCritical,omitempty"`
	Effectiveness int    `json:"effectiveness,omitempty"` // 0=immune, 50=NVE, 100=neutral, 200=SE, 400=4x

	// HP changes
	TargetName  string `json:"targetName,omitempty"`
	TargetSide  string `json:"targetSide,omitempty"` // "player" or "enemy"
	TargetHP    int    `json:"targetHp"`
	TargetMaxHP int    `json:"targetMaxHp"`

	// Status
	StatusApplied string `json:"statusApplied,omitempty"`

	// Faint
	FaintedName string `json:"faintedName,omitempty"`

	// Experience
	ExpGained int `json:"expGained,omitempty"`

	// Catch
	Shakes int `json:"shakes,omitempty"` // Number of ball shakes (0-3) before catch/escape

	// Move learning
	NewMoveID   int    `json:"newMoveId,omitempty"`   // Move ID being learned
	NewMoveName string `json:"newMoveName,omitempty"` // Move name being learned
	LearnedSlot int    `json:"learnedSlot,omitempty"` // Slot index where move was auto-learned (-1 if prompt needed)

	// Evolution
	EvolvedSpeciesID int    `json:"evolvedSpeciesId,omitempty"` // New species ID after evolution
	EvolvedName      string `json:"evolvedName,omitempty"`      // New species name after evolution
}

// BattleEventType categorizes battle events for the client.
type BattleEventType string

const (
	EventMoveUsed        BattleEventType = "move_used"
	EventDamageDealt     BattleEventType = "damage_dealt"
	EventMissed          BattleEventType = "missed"
	EventCriticalHit     BattleEventType = "critical_hit"
	EventSuperEffective  BattleEventType = "super_effective"
	EventNotEffective    BattleEventType = "not_effective"
	EventImmune          BattleEventType = "immune"
	EventFainted         BattleEventType = "fainted"
	EventStatusApplied   BattleEventType = "status_applied"
	EventStatChanged     BattleEventType = "stat_changed"
	EventRunSuccess      BattleEventType = "run_success"
	EventRunFail         BattleEventType = "run_fail"
	EventBattleWin       BattleEventType = "battle_win"
	EventBattleLose      BattleEventType = "battle_lose"
	EventMessage         BattleEventType = "message"
	EventExpGained       BattleEventType = "exp_gained"
	EventCatchAttempt    BattleEventType = "catch_attempt"
	EventCatchSuccess    BattleEventType = "catch_success"
	EventCatchFail       BattleEventType = "catch_fail"
	EventMoveLearned     BattleEventType = "move_learned"      // Auto-learned into empty slot
	EventMoveLearnPrompt BattleEventType = "move_learn_prompt" // All slots full, player must choose
	EventEvolution       BattleEventType = "evolution"         // Pokémon evolved into a new form
)

// SafariBattleEvent represents something that happened during a safari turn.
type SafariBattleEvent struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Shakes  int    `json:"shakes,omitempty"`
}
