package pokebattle

import "math/rand"

// Pokemon represents a single Pokémon in battle with all its runtime state.
type Pokemon struct {
	// Identity
	ID     int    // Database ID (species)
	Name   string // Species name (e.g. "PIKACHU")
	Level  int
	IsWild bool // True for wild encounters, false for trainer/player Pokémon

	// Types
	Type1 PokemonType
	Type2 PokemonType // Same as Type1 if single-type

	// Base stats (from DB)
	BaseStats BaseStats

	// IVs and EVs
	IVs IVs
	EVs EVs

	// Computed stats (calculated from base + IV + EV + level)
	MaxHP   int
	CurHP   int
	Attack  int
	Defense int
	Special int
	Speed   int

	// Moves (up to 4)
	Moves [4]MoveSlot

	// Status condition
	Status         StatusCondition
	SleepTurns     int // Remaining sleep turns (1-7 in Gen 1)
	BadPoisonTurns int // Toxic counter for ramping battle damage

	// Volatile status (reset on switch-out)
	ConfusionTurns int  // 0 = not confused
	IsSeeded       bool // Leech Seed
	SubstituteHP   int  // 0 = no substitute
	DireHit        bool // Dire Hit critical boost active
	GuardSpec      bool // Guard Spec stat-drop protection active

	// Stat stages (-6 to +6)
	AtkStage int
	DefStage int
	SpcStage int
	SpdStage int
	AccStage int
	EvaStage int

	// Catch rate (for wild Pokémon)
	CatchRate int

	// Cry metadata from the original species data.
	CrySFX    string
	CryPitch  int
	CryLength int

	// Base speed (for critical hit calculation)
	BaseSpeed int

	// Experience
	BaseExp  int        // Base experience yield (from species data)
	GrowthRt GrowthRate // Experience growth rate
	Exp      int        // Current total experience points

	// Evolution
	EvolveLevel       int    // Level at which this Pokémon evolves (0 = no level evolution)
	EvolvePokemonName string // Name of the evolved form (e.g. "CHARMELEON")

	// PC storage metadata (only set when loaded from a PC box)
	BoxSlot int // box_slot in the DB (-1 if not from PC)

	// Ownership metadata. Outsider Pokémon with a different original trainer may
	// disobey if they exceed the player's badge obedience level.
	OriginalTrainerID int64
}

// MoveSlot represents a single move slot on a Pokémon.
type MoveSlot struct {
	ID         int         // Move database ID (0 = empty slot)
	Name       string      // Move name
	Type       PokemonType // Move type
	Power      int         // Base power (0 for status moves)
	Accuracy   int         // Accuracy (0-100, 0 = always hits)
	PP         int         // Current PP
	MaxPP      int         // Maximum PP
	BasePP     int         // Original move PP before PP Ups
	PPUps      int         // Number of PP Ups used, 0-3
	Effect     string      // Effect constant (e.g. "NO_ADDITIONAL_EFFECT", "POISON_SIDE_EFFECT1")
	BattleSFX  string      // Source SFX constant used by the original battle animation
	SFXPitch   int         // Original battle sound pitch parameter
	SFXTempo   int         // Original battle sound tempo parameter
	IsHighCrit bool        // Whether this is a high-crit-ratio move (Slash, Razor Leaf, etc.)
}

// StatusCondition represents a non-volatile status condition.
type StatusCondition int

const (
	StatusNone StatusCondition = iota
	StatusBurn
	StatusFreeze
	StatusParalyze
	StatusPoison
	StatusBadPoison // Toxic
	StatusSleep
)

func (s StatusCondition) String() string {
	switch s {
	case StatusBurn:
		return "BRN"
	case StatusFreeze:
		return "FRZ"
	case StatusParalyze:
		return "PAR"
	case StatusPoison:
		return "PSN"
	case StatusBadPoison:
		return "TOX"
	case StatusSleep:
		return "SLP"
	default:
		return ""
	}
}

// IsFainted returns true if the Pokémon has 0 HP.
func (p *Pokemon) IsFainted() bool {
	return p.CurHP <= 0
}

// RecalculateStats recomputes all stats from base stats, IVs, EVs, and level.
func (p *Pokemon) RecalculateStats() {
	p.MaxHP, p.Attack, p.Defense, p.Special, p.Speed = CalculateAllStats(
		p.BaseStats, p.IVs, p.EVs, p.Level,
	)
}

// ResetVolatileStatus clears all volatile status effects (called on switch-out).
func (p *Pokemon) ResetVolatileStatus() {
	p.ConfusionTurns = 0
	p.IsSeeded = false
	p.SubstituteHP = 0
	p.DireHit = false
	p.GuardSpec = false
	if p.Status == StatusBadPoison {
		p.BadPoisonTurns = 1
	} else {
		p.BadPoisonTurns = 0
	}
	p.AtkStage = 0
	p.DefStage = 0
	p.SpcStage = 0
	p.SpdStage = 0
	p.AccStage = 0
	p.EvaStage = 0
}

// ClearMajorStatus removes non-volatile battle status and associated counters.
func (p *Pokemon) ClearMajorStatus() {
	p.Status = StatusNone
	p.SleepTurns = 0
	p.BadPoisonTurns = 0
}

// EffectiveAttack returns the Attack stat modified by stat stages and burn.
func (p *Pokemon) EffectiveAttack() int {
	atk := applyStatStage(p.Attack, p.AtkStage)
	if p.Status == StatusBurn {
		atk /= 2
	}
	if atk < 1 {
		atk = 1
	}
	return atk
}

// EffectiveDefense returns the Defense stat modified by stat stages.
func (p *Pokemon) EffectiveDefense() int {
	def := applyStatStage(p.Defense, p.DefStage)
	if def < 1 {
		def = 1
	}
	return def
}

// EffectiveSpecial returns the Special stat modified by stat stages.
func (p *Pokemon) EffectiveSpecial() int {
	spc := applyStatStage(p.Special, p.SpcStage)
	if spc < 1 {
		spc = 1
	}
	return spc
}

// EffectiveSpeed returns the Speed stat modified by stat stages and paralysis.
func (p *Pokemon) EffectiveSpeed() int {
	spd := applyStatStage(p.Speed, p.SpdStage)
	if p.Status == StatusParalyze {
		spd /= 4
	}
	if spd < 1 {
		spd = 1
	}
	return spd
}

// Gen 1 stat stage multipliers: stage -6 to +6
// Multiplier = numerator/denominator
var statStageMultipliers = [13][2]int{
	{25, 100},  // -6: 25/100
	{28, 100},  // -5: 28/100
	{33, 100},  // -4: 33/100
	{40, 100},  // -3: 40/100
	{50, 100},  // -2: 50/100
	{66, 100},  // -1: 66/100
	{1, 1},     //  0: 1/1
	{150, 100}, // +1: 150/100
	{2, 1},     // +2: 2/1
	{250, 100}, // +3: 250/100
	{3, 1},     // +4: 3/1
	{350, 100}, // +5: 350/100
	{4, 1},     // +6: 4/1
}

func applyStatStage(baseStat, stage int) int {
	if stage < -6 {
		stage = -6
	}
	if stage > 6 {
		stage = 6
	}
	idx := stage + 6
	return baseStat * statStageMultipliers[idx][0] / statStageMultipliers[idx][1]
}

// MaxPPWithUps returns a move's max PP after PP Up uses.
func MaxPPWithUps(basePP int, ppUps int) int {
	if ppUps < 0 {
		ppUps = 0
	}
	if ppUps > 3 {
		ppUps = 3
	}
	return basePP + basePP*ppUps/5
}

// NewWildPokemon creates a wild Pokémon with random IVs and computed stats.
func NewWildPokemon(id int, name string, level int, type1, type2 PokemonType, base BaseStats, catchRate int, moves [4]MoveSlot) *Pokemon {
	ivs := GenerateWildIVs(rand.Intn)
	p := &Pokemon{
		ID:        id,
		Name:      name,
		Level:     level,
		IsWild:    true,
		Type1:     type1,
		Type2:     type2,
		BaseStats: base,
		IVs:       ivs,
		EVs:       EVs{}, // Wild Pokémon have 0 EVs
		Moves:     moves,
		CatchRate: catchRate,
		BaseSpeed: base.Speed,
	}
	p.RecalculateStats()
	p.CurHP = p.MaxHP
	return p
}
