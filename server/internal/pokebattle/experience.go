package pokebattle

// GrowthRate represents a Pokémon's experience growth rate.
type GrowthRate int

const (
	GrowthMediumFast   GrowthRate = iota // n^3
	GrowthSlightlyFast                   // unused in Gen 1
	GrowthSlightlySlow                   // unused in Gen 1
	GrowthMediumSlow                     // 6/5*n^3 - 15*n^2 + 100*n - 140
	GrowthFast                           // 4/5*n^3
	GrowthSlow                           // 5/4*n^3
)

// GrowthRateFromString parses a growth rate string from the database.
func GrowthRateFromString(s string) GrowthRate {
	switch s {
	case "MEDIUM_FAST":
		return GrowthMediumFast
	case "SLIGHTLY_FAST":
		return GrowthSlightlyFast
	case "SLIGHTLY_SLOW":
		return GrowthSlightlySlow
	case "MEDIUM_SLOW":
		return GrowthMediumSlow
	case "FAST":
		return GrowthFast
	case "SLOW":
		return GrowthSlow
	default:
		return GrowthMediumFast
	}
}

func (g GrowthRate) String() string {
	switch g {
	case GrowthMediumFast:
		return "MEDIUM_FAST"
	case GrowthSlightlyFast:
		return "SLIGHTLY_FAST"
	case GrowthSlightlySlow:
		return "SLIGHTLY_SLOW"
	case GrowthMediumSlow:
		return "MEDIUM_SLOW"
	case GrowthFast:
		return "FAST"
	case GrowthSlow:
		return "SLOW"
	default:
		return "MEDIUM_FAST"
	}
}

// ExpForLevel returns the total experience needed to reach a given level
// for a given growth rate, using the Gen 1 formulas.
func ExpForLevel(rate GrowthRate, level int) int {
	if level <= 1 {
		return 0
	}
	n := level
	n3 := n * n * n

	switch rate {
	case GrowthFast:
		// 4/5 * n^3
		return (4 * n3) / 5
	case GrowthMediumFast:
		// n^3
		return n3
	case GrowthMediumSlow:
		// 6/5*n^3 - 15*n^2 + 100*n - 140
		result := (6*n3)/5 - 15*n*n + 100*n - 140
		if result < 0 {
			return 0
		}
		return result
	case GrowthSlow:
		// 5/4 * n^3
		return (5 * n3) / 4
	default:
		return n3
	}
}

// LevelForExp returns the level a Pokémon should be at given its total experience
// and growth rate. Max level is 100.
func LevelForExp(rate GrowthRate, exp int) int {
	for level := 100; level >= 1; level-- {
		if exp >= ExpForLevel(rate, level) {
			return level
		}
	}
	return 1
}

// CalculateBattleExp calculates the experience gained from defeating a Pokémon.
// Uses the Gen 1 formula: (a * b * L) / 7
// where:
//
//	a = 1.0 for wild, 1.5 for trainer
//	b = base experience yield of the defeated Pokémon
//	L = level of the defeated Pokémon
func CalculateBattleExp(baseExp, defeatedLevel int, isTrainer bool) int {
	a := 1.0
	if isTrainer {
		a = 1.5
	}
	return int(a * float64(baseExp) * float64(defeatedLevel) / 7.0)
}
