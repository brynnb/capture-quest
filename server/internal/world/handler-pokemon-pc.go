package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
	"fmt"
	"log"
)

const (
	pcBoxCount = 12
	pcBoxSize  = 20
)

type PokemonPCOpenRequest struct {
	RequestID   string `json:"requestId"`
	CharacterID int64  `json:"characterId"`
	SourceID    int    `json:"sourceId"`
}
type PokemonPCCommandRequest struct {
	RequestID    string                    `json:"requestId"`
	Command      *InventoryCommandIdentity `json:"command" tstype:"InventoryCommandIdentity"`
	SourceID     int                       `json:"sourceId"`
	PokemonRowID int64                     `json:"pokemonRowId"`
	Box          *int                      `json:"box"`
}
type PokemonPCResponse struct {
	Success     bool                        `json:"success" tstype:"true"`
	RequestID   string                      `json:"requestId"`
	CharacterID int64                       `json:"characterId"`
	SourceID    int                         `json:"sourceId"`
	PC          PCStorageSnapshot           `json:"pc"`
	Party       []PokemonDTO                `json:"party"`
	Inventory   cqitems.CQInventorySnapshot `json:"inventory" tstype:"import(\"./cqitems\").CQInventorySnapshot"`
}

func authorizePCIn(tx db.DBTX, ses *session.Session, wh *WorldHandler, sourceID int) (int, error) {
	if sourceID <= 0 {
		return 0, fmt.Errorf("PC source identity required")
	}
	charID := int64(ses.Client.CharData().ID)
	var mapID, x, y int
	var facing, routine, kind string
	if err := tx.QueryRow(`SELECT map_id,x,y,item_or_direction,routine,object_type FROM phaser_hidden_objects WHERE id=$1`, sourceID).Scan(&mapID, &x, &y, &facing, &routine, &kind); err != nil {
		return 0, err
	}
	position := wh.ownedPlayerSnapshot(ses, "")
	if routine != "OpenPokemonCenterPC" || kind != "pc" || facing != "SPRITE_FACING_UP" || position.MapID != mapID || position.X != x || position.Y != y+1 || position.Direction != "UP" || position.ServerMovementPending {
		return 0, fmt.Errorf("PC source %d unavailable: owned=(%d,%d,%d,%s,pending=%t) source=(%d,%d,%d,%s,%s,%s)", sourceID, position.MapID, position.X, position.Y, position.Direction, position.ServerMovementPending, mapID, x, y, facing, routine, kind)
	}
	var savedMap, savedX, savedY int
	if err := tx.QueryRow(`SELECT map_id,CAST(x AS INTEGER),CAST(y AS INTEGER) FROM character_data WHERE id=$1`, charID).Scan(&savedMap, &savedX, &savedY); err != nil {
		return 0, err
	}
	if savedMap != mapID || savedX != position.X || savedY != position.Y {
		return 0, fmt.Errorf("PC source ownership changed")
	}
	battle, err := pokebattle.LoadBattleState(tx, charID)
	if err != nil {
		return 0, err
	}
	if battle != nil {
		return 0, fmt.Errorf("finish the battle before using PC storage")
	}
	visit, err := safariSessionIn(tx, charID)
	if err != nil {
		return 0, err
	}
	if visit != nil && visit.Battle != nil {
		return 0, fmt.Errorf("finish the safari encounter before using PC storage")
	}
	return mapID, nil
}
func readPCResponseIn(tx db.DBTX, charID int64, mapID int) (PokemonPCResponse, error) {
	pc, err := readPCStorageIn(tx, charID, mapID)
	if err != nil {
		return PokemonPCResponse{}, err
	}
	party, err := pokebattle.LoadParty(tx, charID)
	if err != nil {
		return PokemonPCResponse{}, err
	}
	result := PokemonPCResponse{Success: true, CharacterID: charID, PC: pc, Party: make([]PokemonDTO, 0, len(party))}
	for _, p := range party {
		result.Party = append(result.Party, pokemonToDTO(p))
	}
	return result, nil
}
func HandlePokemonPCOpen(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req PokemonPCOpenRequest
	charID := int64(ses.Client.CharData().ID)
	if err := decodePlayerMovement(payload, &req); err != nil || req.CharacterID != charID || req.RequestID == "" || len(req.RequestID) > 64 || req.SourceID <= 0 {
		sendInventoryCommandError(ses, req.RequestID, opcodes.PokemonPCOpenResponse, "Invalid PC read.")
		return false
	}
	var result PokemonPCResponse
	err := db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
		var lockedID int64
		if err := tx.QueryRow(`SELECT id FROM character_data WHERE id=$1 FOR UPDATE`, charID).Scan(&lockedID); err != nil {
			return err
		}
		mapID, err := authorizePCIn(tx, ses, wh, req.SourceID)
		if err != nil {
			return err
		}
		result, err = readPCResponseIn(tx, charID, mapID)
		if err != nil {
			return err
		}
		result.Inventory, err = cqitems.NewStore(tx).GetCharacterSnapshot(ses.CommandContext(), int32(charID))
		return err
	})
	if err != nil {
		log.Printf("[PC] Open source %d for character %d: %v", req.SourceID, charID, err)
		sendInventoryCommandError(ses, req.RequestID, opcodes.PokemonPCOpenResponse, "Interact with an available PC terminal.")
		return false
	}
	result.RequestID, result.SourceID = req.RequestID, req.SourceID
	ses.SendStreamJSON(result, opcodes.PokemonPCOpenResponse)
	return false
}
func HandlePokemonPCDeposit(s *session.Session, p []byte, w *WorldHandler) bool {
	return handlePCCommand(s, p, w, opcodes.PokemonPCDepositResponse, "deposit")
}
func HandlePokemonPCWithdraw(s *session.Session, p []byte, w *WorldHandler) bool {
	return handlePCCommand(s, p, w, opcodes.PokemonPCWithdrawResponse, "withdraw")
}
func HandlePokemonPCRelease(s *session.Session, p []byte, w *WorldHandler) bool {
	return handlePCCommand(s, p, w, opcodes.PokemonPCReleaseResponse, "release")
}
func HandlePokemonPCSwitchBox(s *session.Session, p []byte, w *WorldHandler) bool {
	return handlePCCommand(s, p, w, opcodes.PokemonPCSwitchBoxResponse, "switch")
}

// All PC commands join the existing revision/transaction owner. Source permission,
// domain mutation, box preference and full projection must succeed before commit.
func handlePCCommand(ses *session.Session, payload []byte, wh *WorldHandler, opcode opcodes.OpCode, kind string) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req PokemonPCCommandRequest
	if err := decodePlayerMovement(payload, &req); err != nil || !validInventoryCommand(ses, req.RequestID, req.Command) || req.SourceID <= 0 || req.Box == nil || *req.Box < 0 || *req.Box >= pcBoxCount || (kind != "switch" && req.PokemonRowID <= 0) || (kind == "switch" && req.PokemonRowID != 0) {
		sendInventoryCommandError(ses, req.RequestID, opcode, "Invalid PC command.")
		return false
	}
	charID := int64(ses.Client.CharData().ID)
	var result PokemonPCResponse
	inventory, err := cqitems.NewStore(wh.database).ExecuteCommand(ses.CommandContext(), int32(charID), *req.Command.Revision, func(tx db.DBTX) error {
		mapID, err := authorizePCIn(tx, ses, wh, req.SourceID)
		if err != nil {
			return err
		}
		switch kind {
		case "deposit":
			_, err = pokebattle.DepositPokemonToPCInTransaction(tx, charID, req.PokemonRowID, *req.Box)
		case "withdraw":
			_, err = pokebattle.WithdrawPokemonFromPCInTransaction(tx, charID, req.PokemonRowID, *req.Box)
		case "release":
			err = pokebattle.ReleasePokemonFromPCInTransaction(tx, charID, req.PokemonRowID, *req.Box)
		case "switch":
		default:
			return fmt.Errorf("unknown PC command %q", kind)
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO character_pc_state(character_id,current_box) VALUES($1,$2) ON CONFLICT(character_id) DO UPDATE SET current_box=EXCLUDED.current_box`, charID, *req.Box); err != nil {
			return err
		}
		result, err = readPCResponseIn(tx, charID, mapID)
		return err
	})
	if err != nil {
		log.Printf("[PC] %s for character %d: %v", kind, charID, err)
		sendInventoryCommandError(ses, req.RequestID, opcode, "Could not change PC storage. Check its current state before trying again.")
		return false
	}
	result.RequestID, result.SourceID, result.Inventory = req.RequestID, req.SourceID, inventory
	ses.SendStreamJSON(result, opcode)
	return false
}
