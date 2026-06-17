package pokebattle

import "math/rand"

// handleCatch implements the Gen 1 Poké Ball catch formula.
//
// Gen 1 catch algorithm (from pret/pokered disassembly):
//
//  1. If Master Ball (ballModifier >= 255), always catch.
//
//  2. Generate N in [0, ballFactor). If N < statusThreshold, caught directly.
//     statusThreshold = catchRate + statusBonus (sleep/freeze=12, para/burn/psn=0 in Gen1,
//     but we add a small bonus for status).
//
//  3. Otherwise, compute M = max(1, (3*maxHP - 2*curHP) * 255 / (3*maxHP)).
//     Generate N2 in [0, 255]. If N2 <= M, caught directly.
//
//  4. Wobble check: f = min(255, catchRate * ballModifier * hpFactor).
//     hpFactor increases as HP decreases. For each of 3 shakes, random [0,255]
//     must be < f to continue shaking. 3 shakes = caught.
//
// ballModifier: 1.0 = Poké Ball, 1.5 = Great Ball, 2.0 = Ultra Ball, 255 = Master Ball
func (b *BattleState) handleCatch(ballModifier float64) []BattleEvent {
	// Can't catch trainer Pokémon
	if b.BattleType == BattleTrainer {
		b.addEvent(BattleEvent{Type: EventMessage, Message: "The trainer blocked the ball! Don't be a thief!"})
		return b.Events
	}

	enemy := b.GetEnemyPokemon()

	// Master Ball: always catch
	if ballModifier >= 255.0 {
		b.catchSuccess(enemy, "You threw a MASTER BALL!")
		return b.Events
	}

	catchRate := enemy.CatchRate
	if catchRate <= 0 {
		catchRate = 45
	}

	maxHP := enemy.MaxHP
	curHP := enemy.CurHP
	if curHP < 1 {
		curHP = 1
	}
	if maxHP < 1 {
		maxHP = 1
	}

	// Status bonus (Gen 1 gives +12 for sleep/freeze on the first check)
	statusBonus := 0
	if enemy.Status == StatusSleep || enemy.Status == StatusFreeze {
		statusBonus = 12
	} else if enemy.Status == StatusParalyze || enemy.Status == StatusBurn || enemy.Status == StatusPoison || enemy.Status == StatusBadPoison {
		statusBonus = 6
	}

	// --- Check 1: Random [0, 255] vs catchRate ---
	// In Gen 1, this uses the ball type to determine the random range.
	// Poké Ball: rand [0,255], Great Ball: rand [0,200], Ultra Ball: rand [0,150]
	ballRange := 256
	if ballModifier >= 2.0 {
		ballRange = 151 // Ultra Ball
	} else if ballModifier >= 1.5 {
		ballRange = 201 // Great Ball
	}

	n1 := rand.Intn(ballRange)
	threshold := catchRate + statusBonus
	if threshold > 255 {
		threshold = 255
	}

	if n1 < threshold {
		// Passed first check — now HP check
		// M = (3*maxHP - 2*curHP) * 255 / (3*maxHP)
		m := ((3*maxHP - 2*curHP) * 255) / (3 * maxHP)
		if m < 1 {
			m = 1
		}

		n2 := rand.Intn(256)
		if n2 <= m {
			b.catchSuccess(enemy, "You threw a POKé BALL!")
			return b.Events
		}
	}

	// --- Check 2: Wobble check ---
	// Wobble threshold f: higher = more likely to shake
	// f = catchRate * ballModifier * (maxHP / curHP) / 4
	// Clamped to [1, 255]
	hpRatio := float64(maxHP) / float64(curHP)
	f := int(float64(catchRate) * ballModifier * hpRatio / 4.0)
	if f < 1 {
		f = 1
	}
	if f > 255 {
		f = 255
	}

	// Add status bonus to wobble threshold too
	f += statusBonus
	if f > 255 {
		f = 255
	}

	// Each shake: random [0,255] must be < f to continue
	shakes := 0
	for i := 0; i < 3; i++ {
		if rand.Intn(256) < f {
			shakes++
		} else {
			break
		}
	}

	if shakes >= 3 {
		b.catchSuccess(enemy, "You threw a POKé BALL!")
		return b.Events
	}

	// Failed to catch
	b.addEvent(BattleEvent{
		Type:    EventCatchAttempt,
		Message: "You threw a POKé BALL!",
		Shakes:  shakes,
	})

	escapeMsg := "Oh no! The POKéMON broke free!"
	if shakes == 1 {
		escapeMsg = "Aww! It appeared to be caught!"
	} else if shakes == 2 {
		escapeMsg = "Aargh! Almost had it!"
	}

	b.addEvent(BattleEvent{
		Type:    EventCatchFail,
		Message: escapeMsg,
		Shakes:  shakes,
	})

	// Enemy still gets to attack after a failed catch
	b.executeMove(b.GetEnemyPokemon(), b.GetPlayerPokemon(), b.chooseEnemyMove())
	b.checkFaints()
	b.TurnNumber++

	return b.Events
}

func (b *BattleState) handleGuaranteedCatch(ballModifier float64) []BattleEvent {
	if b.BattleType == BattleTrainer {
		b.addEvent(BattleEvent{Type: EventMessage, Message: "The trainer blocked the ball! Don't be a thief!"})
		return b.Events
	}
	b.catchSuccess(b.GetEnemyPokemon(), catchThrowMessage(ballModifier))
	return b.Events
}

func catchThrowMessage(ballModifier float64) string {
	switch {
	case ballModifier >= 255.0:
		return "You threw a MASTER BALL!"
	case ballModifier >= 2.0:
		return "You threw an ULTRA BALL!"
	case ballModifier >= 1.5:
		return "You threw a GREAT BALL!"
	default:
		return "You threw a POKé BALL!"
	}
}

// catchSuccess emits catch events and sets battle state for a successful catch.
func (b *BattleState) catchSuccess(enemy *Pokemon, throwMsg string) {
	b.addEvent(BattleEvent{
		Type:    EventCatchAttempt,
		Message: throwMsg,
		Shakes:  3,
	})
	b.addEvent(BattleEvent{
		Type:    EventCatchSuccess,
		Message: "Gotcha! " + enemy.Name + " was caught!",
	})
	b.PlayerCaught = true
	b.Phase = PhaseBattleEnd
}
