package world

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	model "capturequest/internal/db/models"
	"capturequest/internal/session"

	"github.com/google/uuid"
)

type CharacterSelectEntry struct {
	ID        uint32 `json:"id"`
	Name      string `json:"name"`
	FactionID uint16 `json:"factionId"`
	Class     uint8  `json:"class"`
	MapID     uint32 `json:"mapId"`
	Gender    uint8  `json:"gender"`
	LastLogin uint32 `json:"lastLogin"`
}

type CharacterListRequest struct {
	RequestID string `json:"requestId"`
}

type CharacterListResponse struct {
	Success    bool                   `json:"success" tstype:"true"`
	RequestID  string                 `json:"requestId"`
	AccountID  int64                  `json:"accountId"`
	Characters []CharacterSelectEntry `json:"characters"`
}

type CharacterDeleteRequest struct {
	RequestID   string `json:"requestId"`
	CharacterID int64  `json:"characterId"`
}

type CharacterDeleteResponse struct {
	Success     bool                   `json:"success" tstype:"true"`
	RequestID   string                 `json:"requestId"`
	AccountID   int64                  `json:"accountId"`
	CharacterID int64                  `json:"characterId"`
	Characters  []CharacterSelectEntry `json:"characters"`
}

type CharacterCommandError struct {
	Success   bool   `json:"success" tstype:"false"`
	RequestID string `json:"requestId"`
	Error     string `json:"error"`
}

var errCharacterDeleteDenied = errors.New("Character is unavailable for this account.")

func characterSelectEntryFromData(c model.CharacterData) CharacterSelectEntry {
	return CharacterSelectEntry{
		ID: c.ID, Name: c.Name, FactionID: c.FactionID, Class: c.Class,
		MapID: c.MapID, Gender: c.Gender, LastLogin: c.LastLogin,
	}
}

// A single statement supplies one complete current roster. The same reader joins
// the deletion transaction; callers never publish a partial list or an SQL error
// as an empty account. Character identity is uncached.
func readCharacterRoster(ctx context.Context, database db.ContextDBTX, accountID int64) ([]CharacterSelectEntry, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT id,name,map_id,gender,faction_id,class,last_login
		FROM character_data
		WHERE account_id=$1 AND deleted_at IS NULL
		ORDER BY last_login DESC,id`, accountID)
	if err != nil {
		return nil, fmt.Errorf("read character roster: %w", err)
	}
	defer rows.Close()
	characters := []CharacterSelectEntry{}
	for rows.Next() {
		var entry CharacterSelectEntry
		if err := rows.Scan(&entry.ID, &entry.Name, &entry.MapID, &entry.Gender, &entry.FactionID, &entry.Class, &entry.LastLogin); err != nil {
			return nil, fmt.Errorf("scan character roster: %w", err)
		}
		characters = append(characters, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("finish character roster: %w", err)
	}
	return characters, nil
}

func characterSelectSession(ses *session.Session) bool {
	return ses != nil && !ses.IsClosed() && ses.Authenticated && ses.AccountID > 0 && !ses.HasValidClient()
}

func sendCharacterCommandError(ses *session.Session, opcode opcodes.OpCode, requestID, message string) {
	if ses != nil && !ses.IsClosed() {
		ses.SendStreamJSON(CharacterCommandError{RequestID: requestID, Error: message}, opcode)
	}
}

func HandleCharacterList(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req CharacterListRequest
	if decodePlayerMovement(payload, &req) != nil || !validBattleRequestID(req.RequestID) || !characterSelectSession(ses) {
		sendCharacterCommandError(ses, opcodes.CharacterListResponse, req.RequestID, "Invalid character list request.")
		return false
	}
	ctx, cancel := context.WithTimeout(ses.CommandContext(), 5*time.Second)
	defer cancel()
	characters, err := readCharacterRoster(ctx, wh.database, ses.AccountID)
	if err != nil {
		log.Printf("[WORLD] Account %d roster: %v", ses.AccountID, err)
		sendCharacterCommandError(ses, opcodes.CharacterListResponse, req.RequestID, "Could not load your characters. Please retry.")
		return false
	}
	ses.SendStreamJSON(CharacterListResponse{Success: true, RequestID: req.RequestID, AccountID: ses.AccountID, Characters: characters}, opcodes.CharacterListResponse)
	return false
}

func HandleCharacterDelete(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req CharacterDeleteRequest
	if decodePlayerMovement(payload, &req) != nil || !validBattleRequestID(req.RequestID) || req.CharacterID <= 0 || req.CharacterID > 9007199254740991 || !characterSelectSession(ses) {
		sendCharacterCommandError(ses, opcodes.CharacterDeleteResponse, req.RequestID, "Invalid character deletion request.")
		return false
	}
	ctx, cancel := context.WithTimeout(ses.CommandContext(), 5*time.Second)
	defer cancel()
	characters, err := deleteOwnedCharacter(ctx, wh, ses, req.CharacterID)
	if err != nil {
		log.Printf("[WORLD] Account %d delete character %d: %v", ses.AccountID, req.CharacterID, err)
		message := "Could not delete character. Refresh your characters before trying again."
		if errors.Is(err, errCharacterDeleteDenied) {
			message = errCharacterDeleteDenied.Error()
		}
		sendCharacterCommandError(ses, opcodes.CharacterDeleteResponse, req.RequestID, message)
		return false
	}
	// The desired state and roster have committed. A duplicate stable ID is inert;
	// it can never delete another character that later reuses the original name.
	ses.SendStreamJSON(CharacterDeleteResponse{Success: true, RequestID: req.RequestID, AccountID: ses.AccountID, CharacterID: req.CharacterID, Characters: characters}, opcodes.CharacterDeleteResponse)
	return false
}

// Deletion shares entry's exclusive owner and drain. Verify the account before
// disturbing any live session, then recheck after the prior owner has finished.
// Never hold a database row lock while waiting for that owner's command drain.
func deleteOwnedCharacter(ctx context.Context, wh *WorldHandler, ses *session.Session, characterID int64) ([]CharacterSelectEntry, error) {
	accountID := ses.AccountID
	var ownerAccount int64
	err := wh.database.QueryRowContext(ctx, `SELECT account_id FROM character_data WHERE id=$1`, characterID).Scan(&ownerAccount)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && ownerAccount != accountID) {
		return nil, errCharacterDeleteDenied
	}
	if err != nil {
		return nil, err
	}
	if err := wh.characterOwners.acquire(ctx, characterID, ses, wh.cleanupCharacterSession); err != nil {
		return nil, err
	}
	defer wh.characterOwners.release(characterID, ses)

	var characters []CharacterSelectEntry
	err = db.Transaction(ctx, wh.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, characterID); err != nil {
			return err
		}
		var name string
		var deletedAt sql.NullTime
		if err := tx.QueryRow(`SELECT account_id,name,deleted_at FROM character_data WHERE id=$1`, characterID).Scan(&ownerAccount, &name, &deletedAt); err != nil {
			return err
		}
		if ownerAccount != accountID {
			return errCharacterDeleteDenied
		}
		if !deletedAt.Valid {
			result, err := tx.Exec(`UPDATE character_data SET name=$1,deleted_at=CURRENT_TIMESTAMP WHERE id=$2 AND account_id=$3 AND deleted_at IS NULL`, name+"-DELETED-"+uuid.NewString(), characterID, accountID)
			if err != nil {
				return err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if affected != 1 {
				return fmt.Errorf("character deletion updated %d rows", affected)
			}
		}
		var err error
		characters, err = readCharacterRoster(ctx, tx.(db.ContextDBTX), accountID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return characters, nil
}
