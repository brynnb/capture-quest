package pokebattle

import (
	"encoding/json"
	"math/rand"
)

// BattlePhase represents the current phase of the battle.
type BattlePhase int

const (
	PhaseNotStarted   BattlePhase = iota
	PhaseActionSelect             // Waiting for player to choose: Fight/Items/Pokemon/Run
	PhaseMoveSelect               // Waiting for player to choose a move
	PhaseExecuteTurn              // Both sides have chosen, executing the turn
	PhaseFaintSwitch              // Player's Pokémon fainted, must switch
	PhaseBattleEnd                // Battle is over
)

// BattleType distinguishes wild encounters from trainer battles.
type BattleType int

const (
	BattleWild BattleType = iota
	BattleTrainer
)

// PlayerAction represents what the player chose to do this turn.
type PlayerAction int

const (
	ActionNone   PlayerAction = iota
	ActionFight               // Use a move
	ActionItem                // Use an item
	ActionSwitch              // Switch Pokémon
	ActionRun                 // Attempt to flee
)

// TurnAction represents a single combatant's chosen action for a turn.
type TurnAction struct {
	Action       PlayerAction
	MoveSlot     int     // 0-3 index into Moves array (for ActionFight)
	BallModifier float64 // Ball catch rate multiplier (for ActionItem: 1.0=Poké, 1.5=Great, 2.0=Ultra, 255=Master)
	ItemID       int32   // Item template ID being used (for ActionItem)
}

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

// TrainerMeta holds metadata about the trainer for end-of-battle messages.
// Only populated for trainer battles.
type TrainerMeta struct {
	ClassName        string // e.g. "BUG_CATCHER"
	Name             string // e.g. "Bug Catcher Rick"
	PrizeMoney       int    // Base prize money (calculated from class base * highest level)
	TrainerObjectID  int    // DB object ID for tracking defeated trainers
	WinFlag          string // Optional event flag set when the player wins this trainer battle
	LoseFlag         string // Optional event flag set when the player loses this trainer battle
	LossMessage      string // Optional battle-end message shown when the player loses
	NoBlackoutOnLoss bool   // If true, loss heals the party but does not warp to a Pokémon Center
	PostWinMapName   string // Optional map context for post-win scripted actions
	PostWinActions   json.RawMessage
	PostLoseMapName  string // Optional map context for post-loss scripted actions
	PostLoseActions  json.RawMessage
}

// PendingMove tracks a move waiting for the player to choose which move to forget.
type PendingMove struct {
	PokemonIndex int    // Index in PlayerParty
	MoveID       int    // Move ID to learn
	MoveName     string // Move name (for display)
}

// BattleState holds the full state of a Pokémon battle.
type BattleState struct {
	Phase      BattlePhase
	BattleType BattleType
	TurnNumber int

	// Player side
	PlayerParty  []*Pokemon // Full party (up to 6)
	PlayerActive int        // Index into PlayerParty of the active Pokémon

	// Player ownership/progression metadata used for outsider Pokémon obedience.
	PlayerTrainerID      int64
	PlayerObedienceLevel int

	// Enemy side
	EnemyParty  []*Pokemon // For trainers: full party. For wild: single Pokémon.
	EnemyActive int        // Index into EnemyParty of the active Pokémon

	// Turn state
	PlayerAction TurnAction
	RunAttempts  int // Number of times the player has tried to run (affects formula)

	// Events generated during the current turn
	Events []BattleEvent

	// Catch state
	PlayerCaught bool // True if the player caught the wild Pokémon

	// Scripted battle rules.
	AllowedActions  []string // Optional allowed action names: fight, item, switch, run
	GuaranteedCatch bool     // If true, the first ball throw succeeds.

	// Scripted wild-battle metadata.
	WildWinFlag        string // Optional event flag set when the player wins or catches this wild Pokémon
	WildPostWinMapName string // Optional map context for post-win scripted actions
	WildPostWinActions json.RawMessage

	// Trainer metadata (nil for wild battles)
	Trainer *TrainerMeta

	// Pending move learn (set when player's Pokémon wants to learn a move but has 4 already)
	PendingMoveLearn *PendingMove

	// PostMoveLearnEvents holds events that should be shown after the move learn
	// prompt is resolved (e.g. other pokemon XP, trainer dialogue, prize money).
	PostMoveLearnEvents []BattleEvent
}

// NewWildBattle creates a new battle state for a wild encounter.
func NewWildBattle(playerParty []*Pokemon, wildPokemon *Pokemon) *BattleState {
	return &BattleState{
		Phase:                PhaseActionSelect,
		BattleType:           BattleWild,
		TurnNumber:           0,
		PlayerParty:          playerParty,
		PlayerActive:         FirstAlivePartyIndex(playerParty),
		PlayerObedienceLevel: MaxObedienceLevel,
		EnemyParty:           []*Pokemon{wildPokemon},
		EnemyActive:          0,
	}
}

// NewTrainerBattle creates a new battle state for a trainer battle.
func NewTrainerBattle(playerParty []*Pokemon, trainerParty []*Pokemon) *BattleState {
	return &BattleState{
		Phase:                PhaseActionSelect,
		BattleType:           BattleTrainer,
		TurnNumber:           0,
		PlayerParty:          playerParty,
		PlayerActive:         FirstAlivePartyIndex(playerParty),
		PlayerObedienceLevel: MaxObedienceLevel,
		EnemyParty:           trainerParty,
		EnemyActive:          0,
	}
}

// FirstAlivePartyIndex returns the first non-fainted party slot, or 0 when no
// usable slot exists. Battle-start callers already reject all-fainted parties;
// the fallback keeps empty/test battle states from carrying an invalid index.
func FirstAlivePartyIndex(party []*Pokemon) int {
	for i, pokemon := range party {
		if pokemon != nil && !pokemon.IsFainted() {
			return i
		}
	}
	return 0
}

// GetPlayerPokemon returns the player's currently active Pokémon.
func (b *BattleState) GetPlayerPokemon() *Pokemon {
	return b.PlayerParty[b.PlayerActive]
}

// GetEnemyPokemon returns the enemy's currently active Pokémon.
func (b *BattleState) GetEnemyPokemon() *Pokemon {
	return b.EnemyParty[b.EnemyActive]
}

func (b *BattleState) IsActionAllowed(action PlayerAction) bool {
	if b == nil || len(b.AllowedActions) == 0 {
		return true
	}
	actionName := playerActionName(action)
	for _, allowed := range b.AllowedActions {
		if allowed == actionName {
			return true
		}
	}
	return false
}

func playerActionName(action PlayerAction) string {
	switch action {
	case ActionFight:
		return "fight"
	case ActionItem:
		return "item"
	case ActionSwitch:
		return "switch"
	case ActionRun:
		return "run"
	default:
		return ""
	}
}

// SubmitAction processes the player's chosen action and executes the turn.
// Returns the list of events that occurred during the turn.
func (b *BattleState) SubmitAction(action TurnAction) []BattleEvent {
	b.Events = nil
	if !b.IsActionAllowed(action.Action) {
		b.addEvent(BattleEvent{Type: EventMessage, Message: "Use an item."})
		return b.Events
	}

	switch action.Action {
	case ActionRun:
		return b.handleRun()
	case ActionFight:
		b.PlayerAction = action
		return b.executeTurn()
	case ActionSwitch:
		// Switching uses the player's turn; enemy still attacks
		b.PlayerAction = action
		return b.executeSwitchTurn(action.MoveSlot) // MoveSlot repurposed as party index
	case ActionItem:
		b.PlayerAction = action
		if b.GuaranteedCatch {
			return b.handleGuaranteedCatch(action.BallModifier)
		}
		return b.handleCatch(action.BallModifier)
	default:
		return b.Events
	}
}

// handleRun attempts to flee from a wild battle.
// Gen 1 flee formula: (playerSpeed * 32 / enemySpeed) + 30 * runAttempts > rand(0,255)
func (b *BattleState) handleRun() []BattleEvent {
	if b.BattleType == BattleTrainer {
		b.addEvent(BattleEvent{Type: EventRunFail, Message: "Can't escape from a trainer battle!"})
		return b.Events
	}

	b.RunAttempts++
	playerSpd := b.GetPlayerPokemon().EffectiveSpeed()
	enemySpd := b.GetEnemyPokemon().EffectiveSpeed()

	// If player is faster, always escape
	if playerSpd >= enemySpd {
		b.addEvent(BattleEvent{Type: EventRunSuccess, Message: "Got away safely!"})
		b.Phase = PhaseBattleEnd
		return b.Events
	}

	// Gen 1 formula
	threshold := (playerSpd * 32 / enemySpd) + 30*b.RunAttempts
	if threshold > 255 || rand.Intn(256) < threshold {
		b.addEvent(BattleEvent{Type: EventRunSuccess, Message: "Got away safely!"})
		b.Phase = PhaseBattleEnd
		return b.Events
	}

	b.addEvent(BattleEvent{Type: EventRunFail, Message: "Can't escape!"})

	// Enemy still gets to attack
	b.executeMove(b.GetEnemyPokemon(), b.GetPlayerPokemon(), b.chooseEnemyMove())
	b.applyEndTurnStatusDamage()
	b.checkFaints()
	b.TurnNumber++

	return b.Events
}

// executeTurn runs a full turn: both sides attack in speed order.
func (b *BattleState) executeTurn() []BattleEvent {
	player := b.GetPlayerPokemon()
	enemy := b.GetEnemyPokemon()

	playerMove := b.PlayerAction.MoveSlot
	enemyMove := b.chooseEnemyMove()

	// Determine turn order by speed (player wins ties)
	playerFirst := player.EffectiveSpeed() >= enemy.EffectiveSpeed()

	if playerFirst {
		b.executeMove(player, enemy, playerMove)
		if !enemy.IsFainted() {
			b.executeMove(enemy, player, enemyMove)
		}
	} else {
		b.executeMove(enemy, player, enemyMove)
		if !player.IsFainted() {
			b.executeMove(player, enemy, playerMove)
		}
	}

	b.applyEndTurnStatusDamage()
	b.checkFaints()
	b.TurnNumber++

	return b.Events
}

// executeSwitchTurn handles a turn where the player switches Pokémon.
func (b *BattleState) executeSwitchTurn(partyIndex int) []BattleEvent {
	if partyIndex < 0 || partyIndex >= len(b.PlayerParty) {
		return b.Events
	}
	target := b.PlayerParty[partyIndex]
	if target.IsFainted() || partyIndex == b.PlayerActive {
		return b.Events
	}

	old := b.GetPlayerPokemon()
	old.ResetVolatileStatus()

	b.PlayerActive = partyIndex
	newPoke := b.GetPlayerPokemon()
	b.addEvent(BattleEvent{
		Type:    EventMessage,
		Message: old.Name + ", come back! Go, " + newPoke.Name + "!",
	})

	// Enemy attacks the new Pokémon
	b.executeMove(b.GetEnemyPokemon(), b.GetPlayerPokemon(), b.chooseEnemyMove())
	b.applyEndTurnStatusDamage()
	b.checkFaints()
	b.TurnNumber++

	return b.Events
}

// ExecuteEnemyTurn runs just the enemy's attack (used when the player uses an item).
// Returns the events generated by the enemy's turn.
func (b *BattleState) ExecuteEnemyTurn() []BattleEvent {
	b.Events = nil
	enemy := b.GetEnemyPokemon()
	player := b.GetPlayerPokemon()
	if enemy.IsFainted() || player.IsFainted() {
		return b.Events
	}
	b.executeMove(enemy, player, b.chooseEnemyMove())
	b.applyEndTurnStatusDamage()
	b.checkFaints()
	b.TurnNumber++
	return b.Events
}

// executeMove has one Pokémon use a move against another.
func (b *BattleState) executeMove(attacker, defender *Pokemon, moveSlot int) {
	if attacker.IsFainted() || defender.IsFainted() {
		return
	}

	// Check status prevents acting
	if !b.canAct(attacker) {
		return
	}

	if b.handleDisobedience(attacker, defender, moveSlot) {
		return
	}

	b.executeMoveDirect(attacker, defender, moveSlot, true, true)
}

// executeMoveDirect executes a move after turn-prevention checks have already
// run. It can be used for disobedient random moves, which spend PP from the
// commanded move instead of the move that actually comes out.
func (b *BattleState) executeMoveDirect(attacker, defender *Pokemon, moveSlot int, deductPP, requirePP bool) {
	// Validate move slot bounds
	if moveSlot < 0 || moveSlot >= len(attacker.Moves) {
		b.executeStruggle(attacker, defender)
		return
	}

	move := &attacker.Moves[moveSlot]
	if move.ID == 0 || (requirePP && move.PP <= 0) {
		// Struggle if no PP (simplified: 50 power Normal-type, 1/2 recoil)
		b.executeStruggle(attacker, defender)
		return
	}

	if deductPP && move.PP > 0 {
		move.PP--
	}

	b.addEvent(BattleEvent{
		Type:         EventMoveUsed,
		AttackerName: attacker.Name,
		AttackerSide: b.pokemonSide(attacker),
		MoveName:     move.Name,
		MoveSFX:      move.BattleSFX,
		MoveSFXPitch: move.SFXPitch,
		MoveSFXTempo: move.SFXTempo,
	})

	// Accuracy check
	if move.Accuracy > 0 && !b.accuracyCheck(attacker, defender, move.Accuracy) {
		b.addEvent(BattleEvent{
			Type:         EventMissed,
			AttackerName: attacker.Name,
			AttackerSide: b.pokemonSide(attacker),
			Message:      attacker.Name + "'s attack missed!",
		})
		return
	}

	hit := true
	if move.Power > 0 {
		hit = b.executeDamageMove(attacker, defender, move)
	}
	if hit {
		b.applyMoveEffect(attacker, defender, move)
	}
}

// executeDamageMove calculates and applies damage from a damaging move.
func (b *BattleState) executeDamageMove(attacker, defender *Pokemon, move *MoveSlot) bool {
	// Determine if physical or special (Gen 1: types determine this)
	isSpecial := isSpecialType(move.Type)

	var atkStat, defStat int
	if isSpecial {
		atkStat = attacker.EffectiveSpecial()
		defStat = defender.EffectiveSpecial()
	} else {
		atkStat = attacker.EffectiveAttack()
		defStat = defender.EffectiveDefense()
	}

	// Critical hit check
	isCrit := IsCriticalHitBoosted(attacker.BaseSpeed, move.IsHighCrit, attacker.DireHit, rand.Intn(256))

	// On critical hit in Gen 1, ignore stat stages (use raw stats)
	if isCrit {
		if isSpecial {
			atkStat = attacker.Special
			defStat = defender.Special
		} else {
			atkStat = attacker.Attack
			defStat = defender.Defense
		}
		// Burn still halves attack on crits in Gen 1
		if !isSpecial && attacker.Status == StatusBurn {
			atkStat /= 2
		}
	}

	// Random factor: 217–255
	randomVal := 217 + rand.Intn(39) // 217 to 255 inclusive

	result := CalculateDamage(DamageInput{
		Level:         attacker.Level,
		AttackStat:    atkStat,
		MoveType:      move.Type,
		MovePower:     move.Power,
		AttackerType1: attacker.Type1,
		AttackerType2: attacker.Type2,
		DefenseStat:   defStat,
		DefenderType1: defender.Type1,
		DefenderType2: defender.Type2,
		IsCritical:    isCrit,
		RandomValue:   randomVal,
	})

	if result.Effectiveness == 0 {
		b.addEvent(BattleEvent{
			Type:       EventImmune,
			TargetName: defender.Name,
			TargetSide: b.pokemonSide(defender),
			Message:    "It doesn't affect " + defender.Name + "...",
		})
		return false
	}

	// Apply damage
	defender.CurHP -= result.Damage
	if defender.CurHP < 0 {
		defender.CurHP = 0
	}

	b.addEvent(BattleEvent{
		Type:          EventDamageDealt,
		AttackerName:  attacker.Name,
		AttackerSide:  b.pokemonSide(attacker),
		TargetName:    defender.Name,
		TargetSide:    b.pokemonSide(defender),
		MoveName:      move.Name,
		Damage:        result.Damage,
		IsCritical:    result.IsCritical,
		Effectiveness: result.Effectiveness,
		TargetHP:      defender.CurHP,
		TargetMaxHP:   defender.MaxHP,
	})
	b.thawFrozenTargetIfFireMove(defender, move)
	return true
}

func (b *BattleState) thawFrozenTargetIfFireMove(target *Pokemon, move *MoveSlot) {
	if target == nil || move == nil || target.IsFainted() {
		return
	}
	if target.Status != StatusFreeze || move.Type != TypeFire || move.Power <= 0 {
		return
	}
	target.ClearMajorStatus()
	b.addEvent(BattleEvent{
		Type:        EventMessage,
		TargetName:  target.Name,
		TargetSide:  b.pokemonSide(target),
		TargetHP:    target.CurHP,
		TargetMaxHP: target.MaxHP,
		Message:     target.Name + " thawed out!",
	})
}

func (b *BattleState) applyMoveEffect(attacker, defender *Pokemon, move *MoveSlot) {
	if attacker.IsFainted() || defender.IsFainted() {
		return
	}

	switch move.Effect {
	case "SLEEP_EFFECT":
		b.tryApplyMajorStatus(defender, StatusSleep, 100)
	case "POISON_EFFECT":
		if move.ID == 92 || move.Name == "TOXIC" {
			b.tryApplyMajorStatus(defender, StatusBadPoison, 100)
		} else {
			b.tryApplyMajorStatus(defender, StatusPoison, 100)
		}
	case "PARALYZE_EFFECT":
		b.tryApplyMajorStatus(defender, StatusParalyze, 100)
	case "CONFUSION_EFFECT":
		b.tryApplyConfusion(defender, 100)
	case "LEECH_SEED_EFFECT":
		b.tryApplyLeechSeed(defender)
	case "BURN_SIDE_EFFECT1":
		b.tryApplyMajorStatus(defender, StatusBurn, 10)
	case "FREEZE_SIDE_EFFECT1":
		b.tryApplyMajorStatus(defender, StatusFreeze, 10)
	case "PARALYZE_SIDE_EFFECT1":
		b.tryApplyMajorStatus(defender, StatusParalyze, 10)
	case "PARALYZE_SIDE_EFFECT2":
		b.tryApplyMajorStatus(defender, StatusParalyze, 30)
	case "POISON_SIDE_EFFECT1":
		b.tryApplyMajorStatus(defender, StatusPoison, 20)
	case "POISON_SIDE_EFFECT2":
		b.tryApplyMajorStatus(defender, StatusPoison, 30)
	case "CONFUSION_SIDE_EFFECT":
		b.tryApplyConfusion(defender, 10)
	}
}

func (b *BattleState) tryApplyMajorStatus(target *Pokemon, status StatusCondition, chancePercent int) bool {
	if target == nil || target.IsFainted() || target.Status != StatusNone {
		return false
	}
	if chancePercent < 100 && rand.Intn(100) >= chancePercent {
		return false
	}
	if targetImmuneToStatus(target, status) {
		return false
	}

	target.Status = status
	target.BadPoisonTurns = 0
	if status == StatusSleep {
		target.SleepTurns = 1 + rand.Intn(7)
	} else {
		target.SleepTurns = 0
	}
	if status == StatusBadPoison {
		target.BadPoisonTurns = 1
	}

	b.addEvent(BattleEvent{
		Type:          EventStatusApplied,
		TargetName:    target.Name,
		TargetSide:    b.pokemonSide(target),
		TargetHP:      target.CurHP,
		TargetMaxHP:   target.MaxHP,
		StatusApplied: status.String(),
		Message:       statusAppliedMessage(target.Name, status),
	})
	return true
}

func (b *BattleState) tryApplyConfusion(target *Pokemon, chancePercent int) bool {
	if target == nil || target.IsFainted() || target.ConfusionTurns > 0 {
		return false
	}
	if chancePercent < 100 && rand.Intn(100) >= chancePercent {
		return false
	}
	target.ConfusionTurns = 2 + rand.Intn(4)
	b.addEvent(BattleEvent{
		Type:        EventStatusApplied,
		TargetName:  target.Name,
		TargetSide:  b.pokemonSide(target),
		TargetHP:    target.CurHP,
		TargetMaxHP: target.MaxHP,
		Message:     target.Name + " became confused!",
	})
	return true
}

func (b *BattleState) tryApplyLeechSeed(target *Pokemon) bool {
	if target == nil || target.IsFainted() || target.IsSeeded {
		return false
	}
	if target.Type1 == TypeGrass || target.Type2 == TypeGrass {
		return false
	}
	target.IsSeeded = true
	b.addEvent(BattleEvent{
		Type:        EventStatusApplied,
		TargetName:  target.Name,
		TargetSide:  b.pokemonSide(target),
		TargetHP:    target.CurHP,
		TargetMaxHP: target.MaxHP,
		Message:     target.Name + " was seeded!",
	})
	return true
}

func targetImmuneToStatus(target *Pokemon, status StatusCondition) bool {
	switch status {
	case StatusBurn:
		return target.Type1 == TypeFire || target.Type2 == TypeFire
	case StatusFreeze:
		return target.Type1 == TypeIce || target.Type2 == TypeIce
	case StatusPoison, StatusBadPoison:
		return target.Type1 == TypePoison || target.Type2 == TypePoison
	default:
		return false
	}
}

func statusAppliedMessage(name string, status StatusCondition) string {
	switch status {
	case StatusBurn:
		return name + " was burned!"
	case StatusFreeze:
		return name + " was frozen solid!"
	case StatusParalyze:
		return name + " was paralyzed! It may not attack!"
	case StatusPoison, StatusBadPoison:
		return name + " was poisoned!"
	case StatusSleep:
		return name + " fell asleep!"
	default:
		return name + " was affected!"
	}
}

func (b *BattleState) applyEndTurnStatusDamage() {
	b.applyResidualStatusDamage(b.GetPlayerPokemon())
	b.applyResidualStatusDamage(b.GetEnemyPokemon())
	b.applyLeechSeedDrain(b.GetPlayerPokemon(), b.GetEnemyPokemon())
	b.applyLeechSeedDrain(b.GetEnemyPokemon(), b.GetPlayerPokemon())
}

func (b *BattleState) applyResidualStatusDamage(p *Pokemon) {
	if p == nil || p.IsFainted() {
		return
	}
	var message string
	var damage int
	switch p.Status {
	case StatusBurn:
		message = p.Name + " is hurt by its burn!"
		damage = p.MaxHP / 16
	case StatusPoison:
		message = p.Name + " is hurt by poison!"
		damage = p.MaxHP / 16
	case StatusBadPoison:
		message = p.Name + " is hurt by poison!"
		if p.BadPoisonTurns <= 0 {
			p.BadPoisonTurns = 1
		}
		damage = p.MaxHP * p.BadPoisonTurns / 16
		p.BadPoisonTurns++
	default:
		return
	}

	if damage < 1 {
		damage = 1
	}
	p.CurHP -= damage
	if p.CurHP < 0 {
		p.CurHP = 0
	}
	b.addEvent(BattleEvent{
		Type:        EventDamageDealt,
		TargetName:  p.Name,
		TargetSide:  b.pokemonSide(p),
		Damage:      damage,
		TargetHP:    p.CurHP,
		TargetMaxHP: p.MaxHP,
		Message:     message,
	})
}

func (b *BattleState) applyLeechSeedDrain(seeded, recipient *Pokemon) {
	if seeded == nil || recipient == nil || seeded.IsFainted() || recipient.IsFainted() || !seeded.IsSeeded {
		return
	}

	damage := seeded.MaxHP / 16
	if damage < 1 {
		damage = 1
	}
	if damage > seeded.CurHP {
		damage = seeded.CurHP
	}
	seeded.CurHP -= damage
	recipient.CurHP += damage
	if recipient.CurHP > recipient.MaxHP {
		recipient.CurHP = recipient.MaxHP
	}

	b.addEvent(BattleEvent{
		Type:        EventDamageDealt,
		TargetName:  seeded.Name,
		TargetSide:  b.pokemonSide(seeded),
		Damage:      damage,
		TargetHP:    seeded.CurHP,
		TargetMaxHP: seeded.MaxHP,
		Message:     seeded.Name + "'s health was sapped by LEECH SEED!",
	})
	b.addEvent(BattleEvent{
		Type:        EventMessage,
		TargetName:  recipient.Name,
		TargetSide:  b.pokemonSide(recipient),
		TargetHP:    recipient.CurHP,
		TargetMaxHP: recipient.MaxHP,
		Message:     recipient.Name + " absorbed health!",
	})
}

// executeStruggle handles the Struggle move (used when all PP is depleted).
func (b *BattleState) executeStruggle(attacker, defender *Pokemon) {
	b.addEvent(BattleEvent{
		Type:         EventMoveUsed,
		AttackerName: attacker.Name,
		AttackerSide: b.pokemonSide(attacker),
		MoveName:     "Struggle",
	})

	randomVal := 217 + rand.Intn(39)
	result := CalculateDamage(DamageInput{
		Level:         attacker.Level,
		AttackStat:    attacker.EffectiveAttack(),
		MoveType:      TypeNormal,
		MovePower:     50,
		AttackerType1: attacker.Type1,
		AttackerType2: attacker.Type2,
		DefenseStat:   defender.EffectiveDefense(),
		DefenderType1: defender.Type1,
		DefenderType2: defender.Type2,
		IsCritical:    false,
		RandomValue:   randomVal,
	})

	defender.CurHP -= result.Damage
	if defender.CurHP < 0 {
		defender.CurHP = 0
	}

	// Recoil: 1/2 of damage dealt
	recoil := result.Damage / 2
	if recoil < 1 {
		recoil = 1
	}
	attacker.CurHP -= recoil
	if attacker.CurHP < 0 {
		attacker.CurHP = 0
	}

	b.addEvent(BattleEvent{
		Type:         EventDamageDealt,
		AttackerName: attacker.Name,
		AttackerSide: b.pokemonSide(attacker),
		TargetName:   defender.Name,
		TargetSide:   b.pokemonSide(defender),
		Damage:       result.Damage,
		TargetHP:     defender.CurHP,
		TargetMaxHP:  defender.MaxHP,
	})
	b.addEvent(BattleEvent{
		Type:    EventMessage,
		Message: attacker.Name + " is hit with recoil!",
	})
}

// canAct checks if a Pokémon can act this turn (status checks).
func (b *BattleState) canAct(p *Pokemon) bool {
	switch p.Status {
	case StatusSleep:
		p.SleepTurns--
		if p.SleepTurns <= 0 {
			p.ClearMajorStatus()
			b.addEvent(BattleEvent{Type: EventMessage, Message: p.Name + " woke up!"})
			return true
		}
		b.addEvent(BattleEvent{Type: EventMessage, Message: p.Name + " is fast asleep."})
		return false

	case StatusFreeze:
		// In Gen 1, frozen Pokémon can never thaw naturally (only via opponent's Fire move)
		b.addEvent(BattleEvent{Type: EventMessage, Message: p.Name + " is frozen solid!"})
		return false

	case StatusParalyze:
		// 25% chance of being fully paralyzed
		if rand.Intn(4) == 0 {
			b.addEvent(BattleEvent{Type: EventMessage, Message: p.Name + " is fully paralyzed!"})
			return false
		}
	}

	// Confusion check
	if p.ConfusionTurns > 0 {
		p.ConfusionTurns--
		if p.ConfusionTurns == 0 {
			b.addEvent(BattleEvent{Type: EventMessage, Message: p.Name + " snapped out of confusion!"})
		} else {
			b.addEvent(BattleEvent{Type: EventMessage, Message: p.Name + " is confused!"})
			// 50% chance of hitting self
			if rand.Intn(2) == 0 {
				// Self-hit: 40 power typeless physical attack
				selfDmg := ((2*p.Level/5+2)*40*p.EffectiveAttack()/p.EffectiveDefense())/50 + 2
				p.CurHP -= selfDmg
				if p.CurHP < 0 {
					p.CurHP = 0
				}
				b.addEvent(BattleEvent{Type: EventMessage, Message: "It hurt itself in its confusion!"})
				return false
			}
		}
	}

	return true
}

// accuracyCheck determines if a move hits.
// moveAccuracy is a percentage (0-100) from the database.
// Modified by accuracy/evasion stat stages.
func (b *BattleState) accuracyCheck(attacker, defender *Pokemon, moveAccuracy int) bool {
	// Apply accuracy/evasion stages
	accMod := applyStatStage(100, attacker.AccStage)
	evaMod := applyStatStage(100, defender.EvaStage)
	if evaMod < 1 {
		evaMod = 1
	}

	// Effective accuracy as a percentage
	effectiveAcc := moveAccuracy * accMod / evaMod
	if effectiveAcc > 100 {
		effectiveAcc = 100
	}

	return rand.Intn(100) < effectiveAcc
}

// chooseEnemyMove picks a random move for the AI opponent.
// In Gen 1, wild Pokémon and most trainers choose moves randomly.
func (b *BattleState) chooseEnemyMove() int {
	enemy := b.GetEnemyPokemon()
	validMoves := make([]int, 0, 4)
	for i, m := range enemy.Moves {
		if m.ID > 0 && m.PP > 0 {
			validMoves = append(validMoves, i)
		}
	}
	if len(validMoves) == 0 {
		return 0 // Will trigger Struggle
	}
	return validMoves[rand.Intn(len(validMoves))]
}

// checkFaints checks if either active Pokémon has fainted and updates battle phase.
func (b *BattleState) checkFaints() {
	enemy := b.GetEnemyPokemon()
	player := b.GetPlayerPokemon()

	if enemy.IsFainted() {
		b.addEvent(BattleEvent{Type: EventFainted, FaintedName: enemy.Name, Message: enemy.Name + " fainted!"})

		// Check if enemy has more Pokémon (trainer battle)
		nextEnemy := b.findNextAlive(b.EnemyParty, b.EnemyActive)
		if nextEnemy == -1 {
			// Player wins — no explicit "You won" message here;
			// trainer battles append parting words + prize money,
			// wild battles show result in the battle_end phase.
			b.Phase = PhaseBattleEnd
			return
		}
		// Trainer sends out next Pokémon
		b.EnemyActive = nextEnemy
		next := b.GetEnemyPokemon()
		b.addEvent(BattleEvent{Type: EventMessage, Message: "The enemy sent out " + next.Name + "!"})
		b.Phase = PhaseActionSelect
		return
	}

	if player.IsFainted() {
		b.addEvent(BattleEvent{Type: EventFainted, FaintedName: player.Name, Message: player.Name + " fainted!"})

		nextPlayer := b.findNextAlive(b.PlayerParty, b.PlayerActive)
		if nextPlayer == -1 {
			// Player loses
			b.addEvent(BattleEvent{Type: EventBattleLose, Message: "You blacked out!"})
			b.Phase = PhaseBattleEnd
			return
		}
		// Player must switch
		b.Phase = PhaseFaintSwitch
		return
	}

	// Neither fainted, continue
	b.Phase = PhaseActionSelect
}

// ForceSwitchIn is called when the player selects a replacement after a faint.
func (b *BattleState) ForceSwitchIn(partyIndex int) []BattleEvent {
	b.Events = nil

	if partyIndex < 0 || partyIndex >= len(b.PlayerParty) {
		return b.Events
	}
	target := b.PlayerParty[partyIndex]
	if target.IsFainted() {
		return b.Events
	}

	b.PlayerActive = partyIndex
	newPoke := b.GetPlayerPokemon()
	b.addEvent(BattleEvent{
		Type:    EventMessage,
		Message: "Go, " + newPoke.Name + "!",
	})
	b.Phase = PhaseActionSelect
	return b.Events
}

// RunFromFaintSwitch attempts to flee during the faint switch phase (wild battles only).
// Uses the same Gen 1 run formula. If the run fails, the player must still switch.
func (b *BattleState) RunFromFaintSwitch() []BattleEvent {
	b.Events = nil

	if b.BattleType == BattleTrainer {
		b.addEvent(BattleEvent{Type: EventRunFail, Message: "Can't escape from a trainer battle!"})
		return b.Events
	}

	b.RunAttempts++

	// Use the speed of the next alive Pokémon for the run calc
	nextIdx := b.findNextAlive(b.PlayerParty, b.PlayerActive)
	if nextIdx == -1 {
		// No alive Pokémon — shouldn't happen, but treat as fail
		b.addEvent(BattleEvent{Type: EventRunFail, Message: "Can't escape!"})
		return b.Events
	}

	playerSpd := b.PlayerParty[nextIdx].EffectiveSpeed()
	enemySpd := b.GetEnemyPokemon().EffectiveSpeed()

	// If player is faster, always escape
	if playerSpd >= enemySpd {
		b.addEvent(BattleEvent{Type: EventRunSuccess, Message: "Got away safely!"})
		b.Phase = PhaseBattleEnd
		return b.Events
	}

	// Gen 1 formula
	threshold := (playerSpd * 32 / enemySpd) + 30*b.RunAttempts
	if threshold > 255 || rand.Intn(256) < threshold {
		b.addEvent(BattleEvent{Type: EventRunSuccess, Message: "Got away safely!"})
		b.Phase = PhaseBattleEnd
		return b.Events
	}

	b.addEvent(BattleEvent{Type: EventRunFail, Message: "Can't escape!"})
	// Phase stays PhaseFaintSwitch — player must still pick a Pokémon
	return b.Events
}

// findNextAlive returns the index of the next non-fainted Pokémon, or -1 if none.
func (b *BattleState) findNextAlive(party []*Pokemon, skipIndex int) int {
	for i, p := range party {
		if i != skipIndex && !p.IsFainted() {
			return i
		}
	}
	return -1
}

// IsOver returns true if the battle has ended.
func (b *BattleState) IsOver() bool {
	return b.Phase == PhaseBattleEnd
}

// PlayerWon returns true if the player won (all enemy Pokémon fainted).
func (b *BattleState) PlayerWon() bool {
	for _, p := range b.EnemyParty {
		if !p.IsFainted() {
			return false
		}
	}
	return true
}

func (b *BattleState) addEvent(e BattleEvent) {
	b.Events = append(b.Events, e)
}

func (b *BattleState) pokemonSide(p *Pokemon) string {
	if p == nil {
		return ""
	}
	for _, candidate := range b.PlayerParty {
		if p == candidate {
			return "player"
		}
	}
	for _, candidate := range b.EnemyParty {
		if p == candidate {
			return "enemy"
		}
	}
	return ""
}

// isSpecialType returns true if the given type uses the Special stat in Gen 1.
// Physical types: Normal, Fighting, Flying, Poison, Ground, Rock, Bug, Ghost
// Special types: Fire, Water, Electric, Grass, Ice, Psychic, Dragon
func isSpecialType(t PokemonType) bool {
	switch t {
	case TypeFire, TypeWater, TypeElectric, TypeGrass, TypeIce, TypePsychic, TypeDragon:
		return true
	default:
		return false
	}
}
