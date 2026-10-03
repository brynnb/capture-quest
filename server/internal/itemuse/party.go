package itemuse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/db/pokedex"
	"capturequest/internal/pokebattle"
)

type Service struct{ database *sql.DB }

func New(database *sql.DB) *Service { return &Service{database: database} }

// Rejection is a gameplay rule failure suitable for presentation to the player.
// Database failures remain ordinary errors and must not be sent to the client.
type Rejection struct{ Message string }

func (e *Rejection) Error() string            { return e.Message }
func reject(format string, args ...any) error { return &Rejection{fmt.Sprintf(format, args...)} }

type PartyUse struct {
	Success       bool                  `json:"success"`
	InstanceID    int32                 `json:"instanceId"`
	PartySlot     int                   `json:"partySlot"`
	NewQuantity   uint16                `json:"newQty"`
	Message       string                `json:"message"`
	NeedsMoveSlot bool                  `json:"needsMoveSlot,omitempty"`
	MoveID        int                   `json:"moveId,omitempty"`
	MoveName      string                `json:"moveName,omitempty"`
	Party         []*pokebattle.Pokemon `json:"-"`
}

// UsePartyItem reads mutable state under the character lock, then commits the
// item, party effect, and any evolved species registration together. A move
// selection prompt is a read-only result; a later choice revalidates everything.
func (s *Service) UsePartyItem(ctx context.Context, charID int32, instanceID int32, partySlot, moveSlot int) (PartyUse, error) {
	var result PartyUse
	err := db.Transaction(ctx, s.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, int64(charID)); err != nil {
			return err
		}
		store := cqitems.NewStore(tx)
		owned, err := store.FindInventoryItemByInstanceID(charID, instanceID)
		if errors.Is(err, sql.ErrNoRows) {
			return reject("Item not found in inventory")
		}
		if err != nil {
			return err
		}
		if owned == nil || owned.Instance.Quantity == 0 {
			return reject("Item not found in inventory")
		}
		// The link alone is not authority if corrupt/stale inventory data points
		// at another owner's instance, including reusable items such as HMs.
		var owns bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM cq_item_instances WHERE id=$1 AND owner_id=$2 AND owner_type=0)`, instanceID, charID).Scan(&owns); err != nil {
			return err
		}
		if !owns {
			return reject("Item not found in inventory")
		}
		item := owned.Item
		flute := ShortName(item) == "POKE_FLUTE"
		if !flute && !item.IsUsable {
			return reject("That item can't be used like that")
		}
		if !flute && !itemUsableOnPartyOutsideBattle(item) {
			return reject("That item can't be used outside of battle")
		}
		party, err := pokebattle.LoadParty(tx, int64(charID))
		if err != nil {
			return err
		}
		if len(party) == 0 {
			return reject("No party to use this item on")
		}
		result = PartyUse{Success: true, InstanceID: instanceID, PartySlot: partySlot, NewQuantity: owned.Instance.Quantity}
		consume := !flute && item.ItemType != cqItemTypeHM
		if flute {
			woke := false
			for _, p := range party {
				if p.Status == pokebattle.StatusSleep {
					p.ClearMajorStatus()
					woke = true
				}
			}
			if !woke {
				return reject("It won't have any effect")
			}
			result.Message = "The Poké Flute woke sleeping Pokémon!"
		} else {
			if partySlot < 0 || partySlot >= len(party) {
				return reject("Invalid Pokémon")
			}
			p := party[partySlot]
			beforeSpecies := p.ID
			if isTMHM(item) {
				result.Message, result.MoveID, result.MoveName, result.NeedsMoveSlot, err = teachMove(tx, item, p, moveSlot)
				if err != nil {
					return err
				}
				if result.NeedsMoveSlot {
					return nil
				}
			} else {
				result.Message, err = applyPartyEffect(tx, item, p, moveSlot)
				if err != nil {
					return err
				}
			}
			if p.ID != beforeSpecies {
				if err := pokedex.MarkCaught(tx, int64(charID), p.ID); err != nil {
					return err
				}
			}
		}
		if consume {
			result.NewQuantity, err = store.DecrementItemQuantity(charID, instanceID)
			if err != nil {
				return err
			}
		}
		result.Party, err = pokebattle.SavePartyInTransaction(tx, int64(charID), party)
		return err
	})
	if err != nil {
		return PartyUse{}, err
	}
	return result, nil
}

func teachMove(database db.DBTX, item cqitems.CQItem, p *pokebattle.Pokemon, moveSlot int) (message string, moveID int, moveName string, prompt bool, err error) {
	if item.MoveID == nil {
		err = fmt.Errorf("TM/HM item %d has no move_id", item.ID)
		return
	}
	moveID = int(*item.MoveID)
	if !pokebattle.CanLearnTMHM(database, p.ID, moveID) {
		err = reject("%s can't learn that move", p.Name)
		return
	}
	move, loadErr := pokebattle.LoadMoveSlotFromDB(database, moveID)
	if loadErr != nil {
		err = loadErr
		return
	}
	moveName = move.Name
	empty := -1
	for i, m := range p.Moves {
		if m.ID == moveID {
			err = reject("%s already knows %s", p.Name, m.Name)
			return
		}
		if m.ID == 0 && empty < 0 {
			empty = i
		}
	}
	if empty >= 0 {
		p.Moves[empty] = move
		message = fmt.Sprintf("%s learned %s!", p.Name, move.Name)
		return
	}
	if moveSlot < 0 {
		message = fmt.Sprintf("%s wants to learn %s, but already knows 4 moves. Choose a move to forget.", p.Name, moveName)
		prompt = true
		return
	}
	if moveSlot >= 4 {
		err = reject("Invalid move slot")
		return
	}
	forgotten := p.Moves[moveSlot]
	if pokebattle.IsHMMove(database, forgotten.ID) {
		err = reject("HM moves can't be forgotten! (%s)", forgotten.Name)
		return
	}
	p.Moves[moveSlot] = move
	message = fmt.Sprintf("1, 2, and… Poof!\n%s forgot %s.\nAnd…\n%s learned %s!", p.Name, forgotten.Name, p.Name, move.Name)
	return
}

func applyPartyEffect(database db.DBTX, item cqitems.CQItem, p *pokebattle.Pokemon, moveSlot int) (string, error) {
	switch {
	case isRareCandy(item):
		if p.IsFainted() {
			return "", reject("%s has fainted", p.Name)
		}
		if p.Level >= 100 {
			return "", reject("%s is already at max level", p.Name)
		}
		oldMaxHP := p.MaxHP
		p.Level++
		p.Exp = pokebattle.ExpForLevel(p.GrowthRt, p.Level)
		p.RecalculateStats()
		p.CurHP += p.MaxHP - oldMaxHP
		message := fmt.Sprintf("%s grew to level %d!", p.Name, p.Level)
		evolvedID, evolvedName := pokebattle.CheckEvolution(database, p)
		if p.EvolveLevel > 0 && p.Level >= p.EvolveLevel && p.EvolvePokemonName != "" && evolvedID == 0 {
			return "", fmt.Errorf("could not resolve evolution %q for species %d", p.EvolvePokemonName, p.ID)
		}
		if evolvedID > 0 {
			oldName := p.Name
			if err := pokebattle.EvolvePokemon(database, p, evolvedID); err != nil {
				return "", err
			}
			message += fmt.Sprintf("\nWhat? %s is evolving!\n%s evolved into %s!", oldName, oldName, evolvedName)
		}
		moves, err := pokebattle.GetMovesLearnedInRange(database, p.ID, p.Level-1, p.Level)
		if err != nil {
			return "", err
		}
		for _, learned := range moves {
			known := false
			empty := -1
			for i, m := range p.Moves {
				if m.ID == learned.MoveID {
					known = true
				}
				if m.ID == 0 && empty < 0 {
					empty = i
				}
			}
			// Preserve the existing Rare Candy rule: skip a new move when all
			// slots are full; interactive move selection belongs to battle.
			if known || empty < 0 {
				continue
			}
			move, err := pokebattle.LoadMoveSlotFromDB(database, learned.MoveID)
			if err != nil {
				return "", err
			}
			p.Moves[empty] = move
			message += fmt.Sprintf("\n%s learned %s!", p.Name, move.Name)
		}
		return message, nil
	case isEvolutionStone(item):
		// Storage/source errors from evolution stay internal.
		if p.IsFainted() {
			return "", reject("%s has fainted", p.Name)
		}
		if _, ok := stoneEvolutionTarget(ShortName(item), p.ID); !ok {
			return "", reject("It won't have any effect")
		}
		return applyStoneEvolution(database, item, p)
	}
	var message string
	var err error
	if _, _, vitamin := vitaminTarget(item); vitamin {
		message, err = applyVitamin(item, p)
	} else if isPPUp(item) {
		message, err = applyPPUp(p, moveSlot)
	} else {
		message, err = pokebattle.ApplyItemEffect(p, MedicineEffect(item), moveSlot)
	}
	if err != nil {
		return "", reject("%s", err)
	}
	return message, nil
}
