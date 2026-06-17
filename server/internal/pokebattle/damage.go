package pokebattle

// DamageInput contains all parameters needed to calculate damage for a single attack.
type DamageInput struct {
	// Attacker
	Level         int         // Attacker's level
	AttackStat    int         // Attacker's effective Attack or Special stat
	MoveType      PokemonType // The type of the move being used
	MovePower     int         // The move's base power (0 for status moves)
	AttackerType1 PokemonType // Attacker's primary type (for STAB)
	AttackerType2 PokemonType // Attacker's secondary type (use same as Type1 if single-type)

	// Defender
	DefenseStat   int         // Defender's effective Defense or Special stat
	DefenderType1 PokemonType // Defender's primary type
	DefenderType2 PokemonType // Defender's secondary type (use same as Type1 if single-type)

	// Critical hit
	IsCritical bool // Whether this is a critical hit

	// Random factor (217–255 in Gen 1). Pass 0 to use a default of 255 (max damage).
	RandomValue int
}

// DamageResult contains the output of a damage calculation.
type DamageResult struct {
	Damage        int  // Final damage dealt
	Effectiveness int  // Combined type effectiveness (100 = neutral, 200 = SE, etc.)
	IsSTAB        bool // Whether STAB was applied
	IsCritical    bool // Whether this was a critical hit
}

// CalculateDamage computes damage using the Gen 1 formula:
//
//	damage = ((2*Level*Crit/5 + 2) * Power * A/D) / 50 + 2
//	damage = damage * STAB (1.5x if applicable)
//	damage = damage * TypeEffectiveness (per defender type)
//	damage = damage * Random/255
//
// Gen 1 critical hits double the attacker's level in the formula (Crit = 2 if critical, 1 otherwise).
// Gen 1 also ignores stat modifications on critical hits (the caller should pass base stats).
//
// Returns 0 for status moves (MovePower == 0) or immune matchups.
func CalculateDamage(input DamageInput) DamageResult {
	result := DamageResult{
		IsCritical: input.IsCritical,
	}

	// Status moves do no damage
	if input.MovePower == 0 {
		return result
	}

	// Defense stat floor of 1 to prevent division by zero
	defStat := input.DefenseStat
	if defStat < 1 {
		defStat = 1
	}

	// Calculate type effectiveness (100-base: 100 = 1.0x)
	result.Effectiveness = GetMoveEffectiveness(input.MoveType, input.DefenderType1, input.DefenderType2)

	// Immune = 0 damage
	if result.Effectiveness == 0 {
		return result
	}

	// Critical hit: level is doubled in the formula
	level := input.Level
	if input.IsCritical {
		level *= 2
	}

	// Base damage: ((2*Level/5 + 2) * Power * A/D) / 50 + 2
	damage := ((2*level/5+2)*input.MovePower*input.AttackStat/defStat)/50 + 2

	// STAB (Same Type Attack Bonus): 1.5x if move type matches attacker type
	if input.MoveType == input.AttackerType1 || input.MoveType == input.AttackerType2 {
		result.IsSTAB = true
		damage = damage * 3 / 2
	}

	// Type effectiveness: apply per defender type
	// GetMoveEffectiveness returns 100-base, so we divide by 100
	// But Gen 1 applies each type multiplier separately to avoid rounding issues.
	// We apply them individually: eff1 then eff2
	eff1 := GetTypeEffectiveness(input.MoveType, input.DefenderType1)
	damage = damage * eff1 / 10

	if input.DefenderType2 != input.DefenderType1 &&
		input.DefenderType2 >= 0 && input.DefenderType2 < NumTypes {
		eff2 := GetTypeEffectiveness(input.MoveType, input.DefenderType2)
		damage = damage * eff2 / 10
	}

	// Random factor: 217–255 in Gen 1 (divide by 255)
	randVal := input.RandomValue
	if randVal <= 0 {
		randVal = 255
	}
	damage = damage * randVal / 255

	// Minimum 1 damage (if not immune, which we already handled)
	if damage < 1 {
		damage = 1
	}

	result.Damage = damage
	return result
}

// CriticalHitChance returns the probability (0.0–1.0) of a critical hit in Gen 1.
// In Gen 1: base_speed / 512 for normal moves.
// High-crit moves (like Slash, Razor Leaf) use base_speed * 8 / 512, capped at 255/256.
func CriticalHitChance(baseSpeed int, isHighCritMove bool) float64 {
	return float64(criticalHitThreshold(baseSpeed, isHighCritMove, false)) / 256.0
}

// IsCriticalHit determines if an attack is a critical hit.
// randVal should be a random number 0–255.
func IsCriticalHit(baseSpeed int, isHighCritMove bool, randVal int) bool {
	return IsCriticalHitBoosted(baseSpeed, isHighCritMove, false, randVal)
}

// IsCriticalHitBoosted determines if an attack is a critical hit with an item
// critical boost such as Dire Hit active.
func IsCriticalHitBoosted(baseSpeed int, isHighCritMove bool, boosted bool, randVal int) bool {
	return randVal < criticalHitThreshold(baseSpeed, isHighCritMove, boosted)
}

func criticalHitThreshold(baseSpeed int, isHighCritMove bool, boosted bool) int {
	multiplier := 1
	if isHighCritMove {
		multiplier *= 8
	}
	if boosted {
		multiplier *= 4
	}
	threshold := baseSpeed * multiplier / 2
	if isHighCritMove || boosted {
		threshold = baseSpeed * multiplier
	}
	if threshold > 255 {
		threshold = 255
	}
	return threshold
}
