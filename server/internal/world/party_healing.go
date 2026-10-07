package world

import "capturequest/internal/pokebattle"

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
