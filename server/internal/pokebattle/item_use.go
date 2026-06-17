package pokebattle

import "fmt"

// ItemEffect describes what an item does when used on a Pokémon.
type ItemEffect struct {
	HealAmount    int    // HP to restore (999 = full)
	StatusCure    string // "poison", "burn", "freeze", "sleep", "paralyze", "all", or ""
	PPRestore     int    // PP to restore per move (999 = full); 0 = none
	PPRestoreAll  bool   // If true, restore PP to all moves; if false, only one move (requires MoveSlot)
	RevivePercent int    // 0 = not a revive; 50 = Revive (half HP); 100 = Max Revive (full HP)
}

// ItemEffectFromData builds an ItemEffect from the cq_items DB fields.
func ItemEffectFromData(healAmount int, statusCure string, ppRestore int, revivePercent int) ItemEffect {
	ppAll := false
	// Elixir/Max Elixir restore PP to all moves
	if ppRestore > 0 {
		// ppRestore > 0 means it restores PP. We'll determine all-vs-one by caller context.
		// For Ether/Max Ether: single move. For Elixir/Max Elixir: all moves.
		// We'll let the caller set PPRestoreAll based on item ID.
		ppAll = false
	}
	return ItemEffect{
		HealAmount:    healAmount,
		StatusCure:    statusCure,
		PPRestore:     ppRestore,
		PPRestoreAll:  ppAll,
		RevivePercent: revivePercent,
	}
}

// ApplyItemEffect applies an item's effect to a Pokémon.
// moveSlot is used for single-move PP restore items (Ether/Max Ether); -1 if not applicable.
// Returns a message describing what happened, or an error if the item can't be used.
func ApplyItemEffect(p *Pokemon, eff ItemEffect, moveSlot int) (string, error) {
	if p == nil {
		return "", fmt.Errorf("no Pokémon selected")
	}

	// Revive: only works on fainted Pokémon
	if eff.RevivePercent > 0 {
		if !p.IsFainted() {
			return "", fmt.Errorf("%s isn't fainted", p.Name)
		}
		if eff.RevivePercent >= 100 {
			p.CurHP = p.MaxHP
		} else {
			p.CurHP = p.MaxHP * eff.RevivePercent / 100
			if p.CurHP < 1 {
				p.CurHP = 1
			}
		}
		p.ClearMajorStatus()
		return fmt.Sprintf("%s was revived!", p.Name), nil
	}

	// Non-revive items can't be used on fainted Pokémon
	if p.IsFainted() {
		return "", fmt.Errorf("%s has fainted", p.Name)
	}

	var messages []string

	// HP healing
	if eff.HealAmount > 0 {
		if p.CurHP >= p.MaxHP {
			// If this is a pure heal item (no status cure, no PP), reject
			if eff.StatusCure == "" && eff.PPRestore == 0 {
				return "", fmt.Errorf("%s's HP is already full", p.Name)
			}
		} else {
			oldHP := p.CurHP
			if eff.HealAmount >= 999 {
				p.CurHP = p.MaxHP
			} else {
				p.CurHP += eff.HealAmount
				if p.CurHP > p.MaxHP {
					p.CurHP = p.MaxHP
				}
			}
			healed := p.CurHP - oldHP
			messages = append(messages, fmt.Sprintf("%s recovered %d HP!", p.Name, healed))
		}
	}

	// Status cure
	if eff.StatusCure != "" && p.Status != StatusNone {
		cured := false
		switch eff.StatusCure {
		case "all":
			cured = true
		case "poison":
			cured = p.Status == StatusPoison || p.Status == StatusBadPoison
		case "burn":
			cured = p.Status == StatusBurn
		case "freeze":
			cured = p.Status == StatusFreeze
		case "sleep":
			cured = p.Status == StatusSleep
		case "paralyze":
			cured = p.Status == StatusParalyze
		}
		if cured {
			p.ClearMajorStatus()
			messages = append(messages, fmt.Sprintf("%s was cured!", p.Name))
		}
	}

	// PP restore
	if eff.PPRestore > 0 {
		if eff.PPRestoreAll {
			restored := false
			for i := range p.Moves {
				if p.Moves[i].ID > 0 && p.Moves[i].PP < p.Moves[i].MaxPP {
					if eff.PPRestore >= 999 {
						p.Moves[i].PP = p.Moves[i].MaxPP
					} else {
						p.Moves[i].PP += eff.PPRestore
						if p.Moves[i].PP > p.Moves[i].MaxPP {
							p.Moves[i].PP = p.Moves[i].MaxPP
						}
					}
					restored = true
				}
			}
			if !restored {
				return "", fmt.Errorf("PP is already full")
			}
			messages = append(messages, fmt.Sprintf("%s's PP was restored!", p.Name))
		} else {
			// Single move PP restore
			if moveSlot < 0 || moveSlot > 3 || p.Moves[moveSlot].ID == 0 {
				return "", fmt.Errorf("invalid move slot")
			}
			if p.Moves[moveSlot].PP >= p.Moves[moveSlot].MaxPP {
				return "", fmt.Errorf("%s's PP is already full", p.Moves[moveSlot].Name)
			}
			if eff.PPRestore >= 999 {
				p.Moves[moveSlot].PP = p.Moves[moveSlot].MaxPP
			} else {
				p.Moves[moveSlot].PP += eff.PPRestore
				if p.Moves[moveSlot].PP > p.Moves[moveSlot].MaxPP {
					p.Moves[moveSlot].PP = p.Moves[moveSlot].MaxPP
				}
			}
			messages = append(messages, fmt.Sprintf("%s's PP was restored!", p.Moves[moveSlot].Name))
		}
	}

	if len(messages) == 0 {
		return "", fmt.Errorf("it won't have any effect")
	}

	result := messages[0]
	for i := 1; i < len(messages); i++ {
		result += "\n" + messages[i]
	}
	return result, nil
}
