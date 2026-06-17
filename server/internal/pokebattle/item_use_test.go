package pokebattle

import "testing"

func TestApplyItemEffect_HealHP(t *testing.T) {
	p := &Pokemon{Name: "PIKACHU", CurHP: 20, MaxHP: 50, Status: StatusNone}
	eff := ItemEffect{HealAmount: 20}
	msg, err := ApplyItemEffect(p, eff, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.CurHP != 40 {
		t.Errorf("expected CurHP=40, got %d", p.CurHP)
	}
	if msg == "" {
		t.Error("expected a message")
	}
}

func TestApplyItemEffect_HealHP_Full(t *testing.T) {
	p := &Pokemon{Name: "PIKACHU", CurHP: 50, MaxHP: 50, Status: StatusNone}
	eff := ItemEffect{HealAmount: 20}
	_, err := ApplyItemEffect(p, eff, -1)
	if err == nil {
		t.Error("expected error when HP is already full")
	}
}

func TestApplyItemEffect_HealHP_Cap(t *testing.T) {
	p := &Pokemon{Name: "PIKACHU", CurHP: 45, MaxHP: 50, Status: StatusNone}
	eff := ItemEffect{HealAmount: 999} // Max Potion
	msg, err := ApplyItemEffect(p, eff, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.CurHP != 50 {
		t.Errorf("expected CurHP=50, got %d", p.CurHP)
	}
	if msg == "" {
		t.Error("expected a message")
	}
}

func TestApplyItemEffect_StatusCure(t *testing.T) {
	p := &Pokemon{Name: "PIKACHU", CurHP: 50, MaxHP: 50, Status: StatusPoison}
	eff := ItemEffect{StatusCure: "poison"}
	msg, err := ApplyItemEffect(p, eff, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status != StatusNone {
		t.Errorf("expected StatusNone, got %v", p.Status)
	}
	if msg == "" {
		t.Error("expected a message")
	}
}

func TestApplyItemEffect_StatusCureClearsBadPoisonCounter(t *testing.T) {
	p := &Pokemon{Name: "PIKACHU", CurHP: 50, MaxHP: 50, Status: StatusBadPoison, BadPoisonTurns: 4}
	eff := ItemEffect{StatusCure: "poison"}
	msg, err := ApplyItemEffect(p, eff, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status != StatusNone {
		t.Errorf("expected StatusNone, got %v", p.Status)
	}
	if p.BadPoisonTurns != 0 {
		t.Errorf("expected BadPoisonTurns=0, got %d", p.BadPoisonTurns)
	}
	if msg == "" {
		t.Error("expected a message")
	}
}

func TestApplyItemEffect_StatusCure_Wrong(t *testing.T) {
	p := &Pokemon{Name: "PIKACHU", CurHP: 50, MaxHP: 50, Status: StatusBurn}
	eff := ItemEffect{StatusCure: "poison"} // Antidote won't cure burn
	_, err := ApplyItemEffect(p, eff, -1)
	if err == nil {
		t.Error("expected error when status doesn't match")
	}
}

func TestApplyItemEffect_FullRestore(t *testing.T) {
	p := &Pokemon{Name: "PIKACHU", CurHP: 10, MaxHP: 50, Status: StatusParalyze}
	eff := ItemEffect{HealAmount: 999, StatusCure: "all"}
	msg, err := ApplyItemEffect(p, eff, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.CurHP != 50 {
		t.Errorf("expected CurHP=50, got %d", p.CurHP)
	}
	if p.Status != StatusNone {
		t.Errorf("expected StatusNone, got %v", p.Status)
	}
	if msg == "" {
		t.Error("expected a message")
	}
}

func TestApplyItemEffect_Revive(t *testing.T) {
	p := &Pokemon{Name: "PIKACHU", CurHP: 0, MaxHP: 50, Status: StatusNone}
	eff := ItemEffect{RevivePercent: 50}
	msg, err := ApplyItemEffect(p, eff, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.CurHP != 25 {
		t.Errorf("expected CurHP=25, got %d", p.CurHP)
	}
	if msg == "" {
		t.Error("expected a message")
	}
}

func TestApplyItemEffect_Revive_NotFainted(t *testing.T) {
	p := &Pokemon{Name: "PIKACHU", CurHP: 30, MaxHP: 50, Status: StatusNone}
	eff := ItemEffect{RevivePercent: 50}
	_, err := ApplyItemEffect(p, eff, -1)
	if err == nil {
		t.Error("expected error when Pokémon isn't fainted")
	}
}

func TestApplyItemEffect_Fainted_NonRevive(t *testing.T) {
	p := &Pokemon{Name: "PIKACHU", CurHP: 0, MaxHP: 50, Status: StatusNone}
	eff := ItemEffect{HealAmount: 20}
	_, err := ApplyItemEffect(p, eff, -1)
	if err == nil {
		t.Error("expected error when using potion on fainted Pokémon")
	}
}

func TestApplyItemEffect_PPRestore_Single(t *testing.T) {
	p := &Pokemon{
		Name:  "PIKACHU",
		CurHP: 50, MaxHP: 50,
		Moves: [4]MoveSlot{
			{ID: 1, Name: "THUNDERBOLT", PP: 5, MaxPP: 15},
			{ID: 2, Name: "QUICK ATTACK", PP: 30, MaxPP: 30},
			{},
			{},
		},
	}
	eff := ItemEffect{PPRestore: 10}
	msg, err := ApplyItemEffect(p, eff, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Moves[0].PP != 15 {
		t.Errorf("expected PP=15, got %d", p.Moves[0].PP)
	}
	if msg == "" {
		t.Error("expected a message")
	}
}

func TestApplyItemEffect_PPRestore_All(t *testing.T) {
	p := &Pokemon{
		Name:  "PIKACHU",
		CurHP: 50, MaxHP: 50,
		Moves: [4]MoveSlot{
			{ID: 1, Name: "THUNDERBOLT", PP: 5, MaxPP: 15},
			{ID: 2, Name: "QUICK ATTACK", PP: 20, MaxPP: 30},
			{},
			{},
		},
	}
	eff := ItemEffect{PPRestore: 999, PPRestoreAll: true}
	msg, err := ApplyItemEffect(p, eff, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Moves[0].PP != 15 {
		t.Errorf("expected move 0 PP=15, got %d", p.Moves[0].PP)
	}
	if p.Moves[1].PP != 30 {
		t.Errorf("expected move 1 PP=30, got %d", p.Moves[1].PP)
	}
	if msg == "" {
		t.Error("expected a message")
	}
}
