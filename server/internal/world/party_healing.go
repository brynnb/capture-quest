package world

import (
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
)

func HealPokemonParty(party []*pokebattle.Pokemon) {
	for _, p := range party {
		if p == nil {
			continue
		}
		p.CurHP = p.MaxHP
		p.ClearMajorStatus()
		for i := range p.Moves {
			if p.Moves[i].ID > 0 {
				p.Moves[i].PP = p.Moves[i].MaxPP
			}
		}
	}
}

func HealCharacterParty(charID int64) ([]*pokebattle.Pokemon, error) {
	myDB := db.GlobalWorldDB.DB
	party, err := pokebattle.LoadParty(myDB, charID)
	if err != nil {
		return nil, fmt.Errorf("load party: %w", err)
	}
	if len(party) == 0 {
		return nil, fmt.Errorf("no party to heal")
	}
	HealPokemonParty(party)
	if err := pokebattle.SaveParty(myDB, charID, party); err != nil {
		return nil, fmt.Errorf("save healed party: %w", err)
	}
	return party, nil
}
