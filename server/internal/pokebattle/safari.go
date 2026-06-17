package pokebattle

import "math/rand"

// SafariBattleState holds the state of a Safari Zone encounter.
// Safari battles have no player Pokémon — the player throws Safari Balls,
// bait, or rocks at wild Pokémon. The Pokémon can flee each turn.
//
// Gen 1 mechanics (from dragonflycave.com/mechanics/gen-i-safari-zone):
// - Rocks double catch rate C (capped at 255), add 1-5 to angry counter, reset eating counter
// - Bait halves catch rate C (rounded down), add 1-5 to eating counter, reset angry counter
// - When angry counter decrements to 0, catch rate resets to species' original catch rate
// - Flee check uses actual Speed stat (not base speed): X = Speed*2, angry doubles, eating /4
// - Safari Ball = Ultra Ball (same catch algorithm as regular capture)
type SafariBattleState struct {
	WildPokemon *Pokemon
	BallsLeft   int
	StepsLeft   int

	// Modified catch rate (starts at species catch rate, modified by rocks/bait)
	CurrentCatchRate  int
	OriginalCatchRate int

	// Angry/eating counters (only one can be nonzero at a time)
	AngryCounter  int // Increased by rock (adds 1-5), decreases each turn
	EatingCounter int // Increased by bait (adds 1-5), decreases each turn

	// Battle state
	Phase   SafariPhase
	Events  []SafariBattleEvent
	Caught  bool
	Fled    bool
	TurnNum int
}

// SafariPhase represents the current phase.
type SafariPhase int

const (
	SafariPhaseAction SafariPhase = iota // Waiting for player action
	SafariPhaseOver                      // Battle ended (caught, fled, or out of balls)
)

// SafariBattleEvent represents something that happened during a safari turn.
type SafariBattleEvent struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Shakes  int    `json:"shakes,omitempty"`
}

// NewSafariBattle creates a new Safari Zone battle.
func NewSafariBattle(wild *Pokemon, ballsLeft, stepsLeft int) *SafariBattleState {
	catchRate := wild.CatchRate
	if catchRate <= 0 {
		catchRate = 45
	}
	return &SafariBattleState{
		WildPokemon:       wild,
		BallsLeft:         ballsLeft,
		StepsLeft:         stepsLeft,
		Phase:             SafariPhaseAction,
		CurrentCatchRate:  catchRate,
		OriginalCatchRate: catchRate,
	}
}

// ThrowBall attempts to catch the wild Pokémon with a Safari Ball.
// Uses the actual Gen 1 R/B/Y capture algorithm:
//  1. N = rand(0..150)  (Safari Ball = Ultra Ball range)
//  2. If N > catchRate → break free (no status in Safari)
//  3. f = HPmax * 255 * 4 / (HPcurrent * 12) — at full HP this is 85
//  4. M = rand(0..255). If f >= M → caught!
//
// Wobble count (cosmetic, shown on break-free):
//
//	d = catchRate * 100 / 150
//	x = d * f / 255
//	x < 10 → 0 shakes, x < 30 → 1, x < 70 → 2, else 3
func (s *SafariBattleState) ThrowBall() {
	s.Events = nil
	s.BallsLeft--
	s.TurnNum++

	s.Events = append(s.Events, SafariBattleEvent{
		Type:    "safari_ball",
		Message: "You threw a SAFARI BALL!",
	})

	catchRate := s.CurrentCatchRate
	if catchRate < 1 {
		catchRate = 1
	}
	if catchRate > 255 {
		catchRate = 255
	}

	// Step 1: Generate N in [0, 150] (Safari/Ultra Ball range)
	n := rand.Intn(151)

	// Step 2: Status check — no status in Safari, so just compare N vs catchRate
	if n > catchRate {
		// Break free — calculate wobble count for display
		// f = 85 at full HP (255 * 4 / 12)
		f := 85
		d := catchRate * 100 / 150
		x := d * f / 255
		shakes := 3
		if x < 10 {
			shakes = 0
		} else if x < 30 {
			shakes = 1
		} else if x < 70 {
			shakes = 2
		}

		escapeMsg := "Oh no! The POKéMON broke free!"
		if shakes == 0 {
			escapeMsg = "The POKéMON broke free!"
		} else if shakes == 1 {
			escapeMsg = "Aww! It appeared to be caught!"
		} else if shakes == 2 {
			escapeMsg = "Aargh! Almost had it!"
		}

		s.Events = append(s.Events, SafariBattleEvent{
			Type:    "catch_fail",
			Message: escapeMsg,
			Shakes:  shakes,
		})

		// Pokémon's turn: tick counters then check flee
		s.pokemonTurn()

		// Check if out of balls
		if !s.Fled && s.BallsLeft <= 0 {
			s.Phase = SafariPhaseOver
			s.Events = append(s.Events, SafariBattleEvent{
				Type:    "out_of_balls",
				Message: "You have no SAFARI BALLs left!",
			})
		}
		return
	}

	// Step 3: f = HPmax * 255 * 4 / (HPcurrent * 12)
	// In Safari Zone, HP is always full, so f = 255 * 4 / 12 = 85
	f := 85

	// Step 4: M = rand(0..255). If f >= M → caught
	m := rand.Intn(256)
	if f >= m {
		s.Caught = true
		s.Phase = SafariPhaseOver
		s.Events = append(s.Events, SafariBattleEvent{
			Type:    "catch_success",
			Message: "Gotcha! " + s.WildPokemon.Name + " was caught!",
			Shakes:  3,
		})
		return
	}

	// Passed step 2 but failed step 4 — ball shakes 3 times then breaks free
	s.Events = append(s.Events, SafariBattleEvent{
		Type:    "catch_fail",
		Message: "Aargh! Almost had it!",
		Shakes:  3,
	})

	// Pokémon's turn: tick counters then check flee
	s.pokemonTurn()

	// Check if out of balls
	if !s.Fled && s.BallsLeft <= 0 {
		s.Phase = SafariPhaseOver
		s.Events = append(s.Events, SafariBattleEvent{
			Type:    "out_of_balls",
			Message: "You have no SAFARI BALLs left!",
		})
	}
}

// ThrowBait makes the Pokémon less likely to flee but harder to catch.
// Gen 1: halves catch rate C, adds 1-5 to eating counter, resets angry counter.
func (s *SafariBattleState) ThrowBait() {
	s.Events = nil
	s.TurnNum++

	s.Events = append(s.Events, SafariBattleEvent{
		Type:    "bait",
		Message: "You threw some bait.",
	})

	// Halve catch rate (Gen 1: rounded down)
	s.CurrentCatchRate /= 2
	if s.CurrentCatchRate < 1 {
		s.CurrentCatchRate = 1
	}

	// Add 1-5 to eating counter, reset angry counter
	s.EatingCounter += rand.Intn(5) + 1 // 1-5 added
	if s.EatingCounter > 255 {
		s.EatingCounter = 255
	}
	s.AngryCounter = 0

	// Pokémon's turn: tick counters then check flee
	s.pokemonTurn()
}

// ThrowRock makes the Pokémon easier to catch but more likely to flee.
// Gen 1: doubles catch rate C (capped at 255), adds 1-5 to angry counter, resets eating counter.
func (s *SafariBattleState) ThrowRock() {
	s.Events = nil
	s.TurnNum++

	s.Events = append(s.Events, SafariBattleEvent{
		Type:    "rock",
		Message: "You threw a rock.",
	})

	// Double catch rate (Gen 1: capped at 255)
	s.CurrentCatchRate *= 2
	if s.CurrentCatchRate > 255 {
		s.CurrentCatchRate = 255
	}

	// Add 1-5 to angry counter, reset eating counter
	s.AngryCounter += rand.Intn(5) + 1 // 1-5 added
	if s.AngryCounter > 255 {
		s.AngryCounter = 255
	}
	s.EatingCounter = 0

	// Pokémon's turn: tick counters then check flee
	s.pokemonTurn()
}

// Run ends the safari battle (player chose to run).
func (s *SafariBattleState) Run() {
	s.Events = nil
	s.Phase = SafariPhaseOver
	s.Events = append(s.Events, SafariBattleEvent{
		Type:    "run",
		Message: "Got away safely!",
	})
}

// pokemonTurn handles the Pokémon's turn after the player acts.
// Gen 1 order: 1) show angry/eating message & decrement counter, 2) check flee.
// When angry counter decrements to 0, catch rate resets to original.
func (s *SafariBattleState) pokemonTurn() {
	if s.Phase == SafariPhaseOver {
		return
	}

	// Step 1: Tick counters and show status message
	if s.AngryCounter > 0 {
		s.Events = append(s.Events, SafariBattleEvent{
			Type:    "wild_reaction",
			Message: s.WildPokemon.Name + " is angry!",
		})
		s.AngryCounter--
		// Gen 1: when angry counter hits 0, catch rate resets to original
		if s.AngryCounter == 0 {
			s.CurrentCatchRate = s.OriginalCatchRate
		}
	} else if s.EatingCounter > 0 {
		s.Events = append(s.Events, SafariBattleEvent{
			Type:    "wild_reaction",
			Message: s.WildPokemon.Name + " is eating!",
		})
		s.EatingCounter--
	}

	// Step 2: Flee check
	// Gen 1 formula: X = (Speed & 0xFF) * 2
	// If X > 255, auto-flee. Then angry doubles X, eating divides by 4.
	// rand(256) < X → flee.
	spd := s.WildPokemon.Speed & 0xFF // low byte of actual Speed stat
	x := spd * 2

	if x > 255 {
		// Speed >= 128: auto-flee
		s.Fled = true
		s.Phase = SafariPhaseOver
		s.Events = append(s.Events, SafariBattleEvent{
			Type:    "fled",
			Message: s.WildPokemon.Name + " fled!",
		})
		return
	}

	// Apply angry/eating modifiers (check CURRENT state after decrement)
	if s.AngryCounter > 0 {
		x *= 2
		if x > 255 {
			x = 255
		}
	} else if s.EatingCounter > 0 {
		x /= 4
	}

	if rand.Intn(256) < x {
		s.Fled = true
		s.Phase = SafariPhaseOver
		s.Events = append(s.Events, SafariBattleEvent{
			Type:    "fled",
			Message: s.WildPokemon.Name + " fled!",
		})
	}
}

// IsOver returns true if the safari battle is finished.
func (s *SafariBattleState) IsOver() bool {
	return s.Phase == SafariPhaseOver
}
