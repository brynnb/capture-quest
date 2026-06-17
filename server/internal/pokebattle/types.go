package pokebattle

// Gen 1 Pokémon types
type PokemonType int

const (
	TypeNormal PokemonType = iota
	TypeFire
	TypeWater
	TypeElectric
	TypeGrass
	TypeIce
	TypeFighting
	TypePoison
	TypeGround
	TypeFlying
	TypePsychic
	TypeBug
	TypeRock
	TypeGhost
	TypeDragon
	NumTypes // sentinel: total number of types
)

var typeNames = map[PokemonType]string{
	TypeNormal:   "NORMAL",
	TypeFire:     "FIRE",
	TypeWater:    "WATER",
	TypeElectric: "ELECTRIC",
	TypeGrass:    "GRASS",
	TypeIce:      "ICE",
	TypeFighting: "FIGHTING",
	TypePoison:   "POISON",
	TypeGround:   "GROUND",
	TypeFlying:   "FLYING",
	TypePsychic:  "PSYCHIC",
	TypeBug:      "BUG",
	TypeRock:     "ROCK",
	TypeGhost:    "GHOST",
	TypeDragon:   "DRAGON",
}

var typeFromName = map[string]PokemonType{
	"NORMAL":   TypeNormal,
	"FIRE":     TypeFire,
	"WATER":    TypeWater,
	"ELECTRIC": TypeElectric,
	"GRASS":    TypeGrass,
	"ICE":      TypeIce,
	"FIGHTING": TypeFighting,
	"POISON":   TypePoison,
	"GROUND":   TypeGround,
	"FLYING":   TypeFlying,
	"PSYCHIC":  TypePsychic,
	"BUG":      TypeBug,
	"ROCK":     TypeRock,
	"GHOST":    TypeGhost,
	"DRAGON":   TypeDragon,
}

func (t PokemonType) String() string {
	if name, ok := typeNames[t]; ok {
		return name
	}
	return "UNKNOWN"
}

// TypeFromString converts a type name string (e.g. "FIRE") to a PokemonType.
// Returns TypeNormal and false if the name is not recognized.
func TypeFromString(name string) (PokemonType, bool) {
	t, ok := typeFromName[name]
	return t, ok
}

// Effectiveness multipliers (stored as fixed-point: 10 = 1.0x)
// 0  = immune (0x)
// 5  = not very effective (0.5x)
// 10 = neutral (1.0x)
// 20 = super effective (2.0x)
//
// Gen 1 type chart. Notable quirks:
// - Ghost has NO EFFECT on Psychic (this was a bug in the original game)
// - Poison is super effective against Bug (removed in Gen 2)
// - Bug is super effective against Poison (removed in Gen 2)
// - Ice is neutral against Fire (changed to not very effective in Gen 2)
//
// Row = attacking type, Column = defending type
// Order: Normal, Fire, Water, Electric, Grass, Ice, Fighting, Poison, Ground, Flying, Psychic, Bug, Rock, Ghost, Dragon
var typeChart [NumTypes][NumTypes]int

func init() {
	// Initialize all to neutral (10 = 1.0x)
	for i := 0; i < int(NumTypes); i++ {
		for j := 0; j < int(NumTypes); j++ {
			typeChart[i][j] = 10
		}
	}

	// Super effective (20 = 2.0x)
	se := [][2]PokemonType{
		// Fire
		{TypeFire, TypeGrass}, {TypeFire, TypeIce}, {TypeFire, TypeBug},
		// Water
		{TypeWater, TypeFire}, {TypeWater, TypeGround}, {TypeWater, TypeRock},
		// Electric
		{TypeElectric, TypeWater}, {TypeElectric, TypeFlying},
		// Grass
		{TypeGrass, TypeWater}, {TypeGrass, TypeGround}, {TypeGrass, TypeRock},
		// Ice
		{TypeIce, TypeGrass}, {TypeIce, TypeGround}, {TypeIce, TypeFlying}, {TypeIce, TypeDragon},
		// Fighting
		{TypeFighting, TypeNormal}, {TypeFighting, TypeIce}, {TypeFighting, TypeRock},
		// Poison
		{TypePoison, TypeGrass}, {TypePoison, TypeBug},
		// Ground
		{TypeGround, TypeFire}, {TypeGround, TypeElectric}, {TypeGround, TypePoison}, {TypeGround, TypeRock},
		// Flying
		{TypeFlying, TypeGrass}, {TypeFlying, TypeFighting}, {TypeFlying, TypeBug},
		// Psychic
		{TypePsychic, TypeFighting}, {TypePsychic, TypePoison},
		// Bug
		{TypeBug, TypeGrass}, {TypeBug, TypePoison}, {TypeBug, TypePsychic},
		// Rock
		{TypeRock, TypeFire}, {TypeRock, TypeIce}, {TypeRock, TypeFlying}, {TypeRock, TypeBug},
		// Ghost
		{TypeGhost, TypeGhost},
		// Dragon
		{TypeDragon, TypeDragon},
	}
	for _, pair := range se {
		typeChart[pair[0]][pair[1]] = 20
	}

	// Not very effective (5 = 0.5x)
	nve := [][2]PokemonType{
		// Normal
		{TypeNormal, TypeRock},
		// Fire
		{TypeFire, TypeFire}, {TypeFire, TypeWater}, {TypeFire, TypeRock}, {TypeFire, TypeDragon},
		// Water
		{TypeWater, TypeWater}, {TypeWater, TypeGrass}, {TypeWater, TypeDragon},
		// Electric
		{TypeElectric, TypeElectric}, {TypeElectric, TypeGrass}, {TypeElectric, TypeDragon},
		// Grass
		{TypeGrass, TypeFire}, {TypeGrass, TypeGrass}, {TypeGrass, TypePoison}, {TypeGrass, TypeFlying}, {TypeGrass, TypeBug}, {TypeGrass, TypeDragon},
		// Ice
		{TypeIce, TypeWater}, {TypeIce, TypeIce},
		// Fighting
		{TypeFighting, TypePoison}, {TypeFighting, TypeFlying}, {TypeFighting, TypePsychic}, {TypeFighting, TypeBug},
		// Poison
		{TypePoison, TypePoison}, {TypePoison, TypeGround}, {TypePoison, TypeRock}, {TypePoison, TypeGhost},
		// Ground
		{TypeGround, TypeGrass}, {TypeGround, TypeBug},
		// Flying
		{TypeFlying, TypeElectric}, {TypeFlying, TypeRock},
		// Psychic
		{TypePsychic, TypePsychic},
		// Bug
		{TypeBug, TypeFire}, {TypeBug, TypeFighting}, {TypeBug, TypeFlying}, {TypeBug, TypeGhost},
		// Rock
		{TypeRock, TypeFighting}, {TypeRock, TypeGround},
		// Ghost — no NVE entries in Gen 1 (Normal is immune, not NVE)
		// Dragon — no NVE entries
	}
	for _, pair := range nve {
		typeChart[pair[0]][pair[1]] = 5
	}

	// Immunities (0 = 0x)
	immune := [][2]PokemonType{
		{TypeNormal, TypeGhost},
		{TypeElectric, TypeGround},
		{TypeFighting, TypeGhost},
		{TypeGround, TypeFlying},
		{TypeGhost, TypeNormal},
		{TypeGhost, TypePsychic}, // Gen 1 bug: Ghost doesn't affect Psychic
	}
	for _, pair := range immune {
		typeChart[pair[0]][pair[1]] = 0
	}
}

// GetTypeEffectiveness returns the effectiveness multiplier (fixed-point: 10 = 1.0x)
// for an attacking type against a defending type.
// Returns: 0 (immune), 5 (not very effective), 10 (neutral), 20 (super effective)
func GetTypeEffectiveness(atkType, defType PokemonType) int {
	if atkType < 0 || atkType >= NumTypes || defType < 0 || defType >= NumTypes {
		return 10 // neutral for invalid types
	}
	return typeChart[atkType][defType]
}

// GetMoveEffectiveness calculates the combined type effectiveness multiplier
// for a move type against a Pokémon with one or two types.
// Returns the multiplier as fixed-point (100 = 1.0x).
// Examples: 0 (immune), 25 (0.25x), 50 (0.5x), 100 (1.0x), 200 (2.0x), 400 (4.0x)
func GetMoveEffectiveness(moveType PokemonType, defType1, defType2 PokemonType) int {
	eff1 := GetTypeEffectiveness(moveType, defType1)

	if defType2 < 0 || defType2 >= NumTypes || defType2 == defType1 {
		// Single-type Pokémon: just scale to 100-base
		return eff1 * 10 // 0→0, 5→50, 10→100, 20→200
	}

	eff2 := GetTypeEffectiveness(moveType, defType2)
	// Multiply: (eff1/10) * (eff2/10) scaled to 100-base
	// eff1 * eff2 / 10 gives us the right scale
	// e.g. 20*20/10 = 40 → but we want 400, so: eff1 * eff2
	return eff1 * eff2 // 0→0, 5*10=50, 10*10=100, 20*10=200, 20*20=400, 5*5=25
}
