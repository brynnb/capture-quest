package world

import (
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

type PokemonPartyReorderRequest struct {
	RequestID  string                    `json:"requestId"`
	Command    *InventoryCommandIdentity `json:"command" tstype:"InventoryCommandIdentity"`
	PokemonIDs []int64                   `json:"pokemonIds"`
}

type PokemonPartyReorderResponse struct {
	Success   bool                        `json:"success" tstype:"true"`
	RequestID string                      `json:"requestId"`
	Inventory cqitems.CQInventorySnapshot `json:"inventory" tstype:"import(\"./cqitems\").CQInventorySnapshot"`
	Party     []PokemonDTO                `json:"party"`
}

// Reordering shares admission, revision, commit and recovery with party-item
// commands. The IDs express the intended final party, never a permutation of
// whatever happens to occupy old slots when a delayed packet arrives.
func HandlePokemonPartyReorder(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req PokemonPartyReorderRequest
	if err := decodePlayerMovement(payload, &req); err != nil || !validInventoryCommand(ses, req.RequestID, req.Command) {
		sendInventoryCommandError(ses, req.RequestID, opcodes.PokemonPartyReorderResponse, "Invalid party reorder command.")
		return false
	}
	charID := int64(ses.Client.CharData().ID)
	var party []*pokebattle.Pokemon
	snapshot, err := cqitems.NewStore(wh.database).ExecuteCommand(ses.CommandContext(), int32(charID), *req.Command.Revision, func(tx db.DBTX) error {
		// Even a terminal battle owns its party until dismissal and pending move
		// choices commit. Check durable state under the same character lock.
		battle, err := pokebattle.LoadBattleState(tx, charID)
		if err != nil {
			return err
		}
		if battle != nil {
			return fmt.Errorf("finish the current battle before reordering")
		}
		visit, err := safariSessionIn(tx, charID)
		if err != nil {
			return err
		}
		if visit != nil && visit.Battle != nil {
			return fmt.Errorf("finish the safari encounter before reordering")
		}
		party, err = pokebattle.ReorderPartyInTransaction(tx, charID, req.PokemonIDs)
		return err
	})
	if err != nil {
		log.Printf("[Party] Reorder failed for character %d: %v", charID, err)
		sendInventoryCommandError(ses, req.RequestID, opcodes.PokemonPartyReorderResponse, "Could not reorder your party. Check its current state before trying again.")
		return false
	}
	dtos := make([]PokemonDTO, len(party))
	for i, pokemon := range party {
		dtos[i] = pokemonToDTO(pokemon)
	}
	ses.SendStreamJSON(PokemonPartyReorderResponse{Success: true, RequestID: req.RequestID, Inventory: snapshot, Party: dtos}, opcodes.PokemonPartyReorderResponse)
	return false
}
