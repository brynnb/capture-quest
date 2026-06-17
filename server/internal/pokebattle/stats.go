package pokebattle

import "math"

// BaseStats represents a Pokémon's base stat values from the database.
type BaseStats struct {
	HP      int
	Attack  int
	Defense int
	Special int // Gen 1 has a single "Special" stat (used for both Sp.Atk and Sp.Def)
	Speed   int
}

// IVs represents a Pokémon's Individual Values (0–15 each in Gen 1).
// In Gen 1, the HP IV is derived from the other four IVs.
type IVs struct {
	Attack  int
	Defense int
	Special int
	Speed   int
}

// HPiv returns the HP IV, which in Gen 1 is derived from the lowest bits
// of the other four IVs: bit3=Atk, bit2=Def, bit1=Spd, bit0=Spc
func (iv IVs) HPiv() int {
	return ((iv.Attack & 1) << 3) | ((iv.Defense & 1) << 2) | ((iv.Speed & 1) << 1) | (iv.Special & 1)
}

// EVs represents a Pokémon's Effort Values (0–65535 each in Gen 1).
// In Gen 1, EVs (called "Stat Experience") are the base stats of defeated Pokémon,
// accumulated without limit up to 65535.
type EVs struct {
	HP      int
	Attack  int
	Defense int
	Special int
	Speed   int
}

// CalculateHP computes the HP stat using the Gen 1 formula:
// HP = ((Base + IV) * 2 + floor(sqrt(EV)) / 4) * Level / 100 + Level + 10
func CalculateHP(base, iv, ev, level int) int {
	evBonus := int(math.Floor(math.Sqrt(float64(ev)))) / 4
	return ((base+iv)*2+evBonus)*level/100 + level + 10
}

// CalculateStat computes a non-HP stat using the Gen 1 formula:
// Stat = ((Base + IV) * 2 + floor(sqrt(EV)) / 4) * Level / 100 + 5
func CalculateStat(base, iv, ev, level int) int {
	evBonus := int(math.Floor(math.Sqrt(float64(ev)))) / 4
	return ((base+iv)*2+evBonus)*level/100 + 5
}

// CalculateAllStats computes all five stats for a Pokémon.
func CalculateAllStats(base BaseStats, ivs IVs, evs EVs, level int) (hp, atk, def, spc, spd int) {
	hp = CalculateHP(base.HP, ivs.HPiv(), evs.HP, level)
	atk = CalculateStat(base.Attack, ivs.Attack, evs.Attack, level)
	def = CalculateStat(base.Defense, ivs.Defense, evs.Defense, level)
	spc = CalculateStat(base.Special, ivs.Special, evs.Special, level)
	spd = CalculateStat(base.Speed, ivs.Speed, evs.Speed, level)
	return
}

// GenerateWildIVs generates random IVs for a wild Pokémon encounter.
// In Gen 1, each IV (Attack, Defense, Speed, Special) is 0–15.
// HP IV is derived from the others.
func GenerateWildIVs(randFn func(n int) int) IVs {
	return IVs{
		Attack:  randFn(16),
		Defense: randFn(16),
		Speed:   randFn(16),
		Special: randFn(16),
	}
}

// GetMovesForLevel returns the moves a wild Pokémon would know at a given level.
// In Gen 1, a wild Pokémon knows its 4 most recent moves from its learnset
// (default moves + level-up moves up to its current level).
// defaultMoveIDs: the Pokémon's 4 default moves (from pokemon table, 0 = empty)
// learnset: list of (level, moveID) pairs sorted by level ascending
// Returns up to 4 move IDs (most recent moves, older ones pushed out).
func GetMovesForLevel(level int, defaultMoveIDs [4]int, learnset [][2]int) [4]int {
	// Start with default moves (push into a sliding window)
	moves := make([]int, 0, 4)
	for _, mid := range defaultMoveIDs {
		if mid > 0 {
			moves = append(moves, mid)
		}
	}

	// Apply level-up moves
	for _, entry := range learnset {
		learnLevel, moveID := entry[0], entry[1]
		if learnLevel > level {
			break
		}
		// Push new move, drop oldest if already have 4
		moves = append(moves, moveID)
		if len(moves) > 4 {
			moves = moves[len(moves)-4:]
		}
	}

	var result [4]int
	copy(result[:], moves)
	return result
}
