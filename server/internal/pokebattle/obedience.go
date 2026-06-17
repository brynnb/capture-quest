package pokebattle

import "math/rand"

const (
	// Gen 1 outsider obedience caps.
	BaseObedienceLevel = 10
	MaxObedienceLevel  = 100
)

type obedienceOutcome int

const (
	obedienceOutcomeObey obedienceOutcome = iota
	obedienceOutcomeRandomMove
	obedienceOutcomeSleep
	obedienceOutcomeSelfHit
	obedienceOutcomeNoAction
)

func (b *BattleState) shouldCheckObedience(p *Pokemon) bool {
	if b == nil || p == nil {
		return false
	}
	if b.pokemonSide(p) != "player" {
		return false
	}
	if b.PlayerTrainerID <= 0 || p.OriginalTrainerID <= 0 || p.OriginalTrainerID == b.PlayerTrainerID {
		return false
	}
	levelCap := b.PlayerObedienceLevel
	if levelCap <= 0 || levelCap >= MaxObedienceLevel {
		return false
	}
	return p.Level > levelCap
}

func (b *BattleState) handleDisobedience(attacker, defender *Pokemon, selectedSlot int) bool {
	if !b.shouldCheckObedience(attacker) {
		return false
	}

	outcome := rollObedienceOutcome(attacker.Level, b.PlayerObedienceLevel)
	if outcome == obedienceOutcomeObey {
		return false
	}
	return b.applyDisobedienceOutcome(attacker, defender, selectedSlot, outcome)
}

func rollObedienceOutcome(level, badgeLevel int) obedienceOutcome {
	if badgeLevel <= 0 {
		badgeLevel = BaseObedienceLevel
	}
	if level <= badgeLevel || badgeLevel >= MaxObedienceLevel {
		return obedienceOutcomeObey
	}

	randRange := level + badgeLevel
	if rand.Intn(randRange) < badgeLevel {
		return obedienceOutcomeObey
	}
	if rand.Intn(randRange) < badgeLevel {
		return obedienceOutcomeRandomMove
	}
	return obedienceOutcomeForFinalRoll(level, badgeLevel, rand.Intn(256))
}

func obedienceOutcomeForFinalRoll(level, badgeLevel, roll int) obedienceOutcome {
	levelGap := level - badgeLevel
	if levelGap <= 0 {
		return obedienceOutcomeObey
	}
	if roll < levelGap {
		return obedienceOutcomeSleep
	}
	if roll < 2*levelGap {
		return obedienceOutcomeSelfHit
	}
	return obedienceOutcomeNoAction
}

func (b *BattleState) applyDisobedienceOutcome(attacker, defender *Pokemon, selectedSlot int, outcome obedienceOutcome) bool {
	switch outcome {
	case obedienceOutcomeRandomMove:
		if b.tryDisobedientRandomMove(attacker, defender, selectedSlot) {
			return true
		}
		b.addEvent(BattleEvent{Type: EventMessage, Message: disobedienceNoActionMessage(attacker.Name)})
		return true
	case obedienceOutcomeSleep:
		if attacker.Status == StatusNone {
			attacker.Status = StatusSleep
			attacker.SleepTurns = 1 + rand.Intn(7)
			b.addEvent(BattleEvent{Type: EventMessage, Message: attacker.Name + " began to nap!"})
			return true
		}
		b.addEvent(BattleEvent{Type: EventMessage, Message: disobedienceNoActionMessage(attacker.Name)})
		return true
	case obedienceOutcomeSelfHit:
		b.addEvent(BattleEvent{Type: EventMessage, Message: attacker.Name + " won't obey!"})
		b.applyDisobedienceSelfDamage(attacker)
		return true
	case obedienceOutcomeNoAction:
		b.addEvent(BattleEvent{Type: EventMessage, Message: disobedienceNoActionMessage(attacker.Name)})
		return true
	default:
		return false
	}
}

func (b *BattleState) tryDisobedientRandomMove(attacker, defender *Pokemon, selectedSlot int) bool {
	if selectedSlot < 0 || selectedSlot >= len(attacker.Moves) {
		return false
	}
	selectedMove := &attacker.Moves[selectedSlot]
	if selectedMove.ID == 0 || selectedMove.PP <= 0 {
		return false
	}

	choices := make([]int, 0, len(attacker.Moves)-1)
	for i, move := range attacker.Moves {
		if i == selectedSlot || move.ID == 0 {
			continue
		}
		choices = append(choices, i)
	}
	if len(choices) == 0 {
		return false
	}

	selectedMove.PP--
	randomSlot := choices[rand.Intn(len(choices))]
	b.addEvent(BattleEvent{
		Type:    EventMessage,
		Message: attacker.Name + " used " + attacker.Moves[randomSlot].Name + " instead!",
	})
	b.executeMoveDirect(attacker, defender, randomSlot, false, false)
	return true
}

func (b *BattleState) applyDisobedienceSelfDamage(p *Pokemon) {
	damage := ((2*p.Level/5+2)*40*p.EffectiveAttack()/p.EffectiveDefense())/50 + 2
	if damage < 1 {
		damage = 1
	}
	p.CurHP -= damage
	if p.CurHP < 0 {
		p.CurHP = 0
	}
	b.addEvent(BattleEvent{Type: EventMessage, Message: "It hurt itself in its confusion!"})
	b.addEvent(BattleEvent{
		Type:        EventDamageDealt,
		TargetName:  p.Name,
		TargetSide:  b.pokemonSide(p),
		Damage:      damage,
		TargetHP:    p.CurHP,
		TargetMaxHP: p.MaxHP,
	})
}

func disobedienceNoActionMessage(pokemonName string) string {
	switch rand.Intn(4) {
	case 0:
		return pokemonName + " ignored orders!"
	case 1:
		return pokemonName + " is loafing around!"
	case 2:
		return pokemonName + " turned away!"
	default:
		return pokemonName + " won't obey!"
	}
}
