package cqitems

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"capturequest/internal/db"
)

const inventoryReceiptVersion = 1

type inventoryCommandReceipt struct {
	Version     int             `json:"version"`
	Kind        string          `json:"kind"`
	RequestID   string          `json:"requestId"`
	CharacterID int32           `json:"characterId"`
	Revision    int64           `json:"revision"`
	InputHash   string          `json:"inputHash"`
	OutcomeHash string          `json:"outcomeHash"`
	Result      json.RawMessage `json:"result"`
}

type inventoryReceiptIdentity struct {
	kind, requestID, hash string
}

// ExecuteCommand owns admission, revision and the committed bag projection for
// shop, party, Repel and consumptive field commands. A stale request is rejected;
// clients recover current state without automatically resending the mutation.
// The same character_shop_state row retains an optional latest command receipt.
func (s *Store) ExecuteCommand(ctx context.Context, charID int32, expected int64, apply func(db.DBTX) error) (CQInventorySnapshot, error) {
	snapshot, _, _, err := s.executeCommand(ctx, charID, expected, nil, func(tx db.DBTX) ([]byte, error) { return nil, apply(tx) })
	return snapshot, err
}

// ExecuteCommandWithReceipt shares the inventory executor, revision and commit.
// An exact latest replay returns its frozen outcome without rerunning gameplay.
// Outcomes must not contain resource projections: the client owns a current read.
func (s *Store) ExecuteCommandWithReceipt(ctx context.Context, charID int32, expected int64, kind, requestID string, input []byte, apply func(db.DBTX) ([]byte, error)) ([]byte, bool, error) {
	if kind == "" || len(kind) > 64 || requestID == "" || len(requestID) > 64 {
		return nil, false, fmt.Errorf("invalid inventory receipt identity")
	}
	digest := sha256.New()
	digest.Write([]byte(fmt.Sprintf("inventory-receipt:%d:%s\n", inventoryReceiptVersion, kind)))
	digest.Write(input)
	identity := &inventoryReceiptIdentity{kind: kind, requestID: requestID, hash: hex.EncodeToString(digest.Sum(nil))}
	_, receipt, replay, err := s.executeCommand(ctx, charID, expected, identity, apply)
	return receipt, replay, err
}

// This boundary cannot join a parent's provisional work: only its own final
// commit can authorize publication. All repository effects share its DBTX owner.
func (s *Store) executeCommand(ctx context.Context, charID int32, expected int64, identity *inventoryReceiptIdentity, apply func(db.DBTX) ([]byte, error)) (CQInventorySnapshot, []byte, bool, error) {
	database, ok := s.database.(*sql.DB)
	if !ok || database == nil {
		return CQInventorySnapshot{}, nil, false, fmt.Errorf("inventory command requires a database pool to own its commit, got %T", s.database)
	}
	if charID <= 0 || expected < 0 || expected >= 9007199254740991 {
		return CQInventorySnapshot{}, nil, false, fmt.Errorf("invalid inventory command revision")
	}
	var snapshot CQInventorySnapshot
	var result []byte
	var replay bool
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, int64(charID)); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO character_shop_state(character_id,revision) VALUES($1,0) ON CONFLICT DO NOTHING`, charID); err != nil {
			return err
		}
		var revision int64
		var oldID, oldHash, oldResult sql.NullString
		if err := tx.QueryRow(`SELECT revision,request_id,input_hash,result_json FROM character_shop_state WHERE character_id=$1`, charID).Scan(&revision, &oldID, &oldHash, &oldResult); err != nil {
			return err
		}
		if identity != nil && oldID.Valid && oldID.String == identity.requestID {
			if revision != expected+1 || !oldHash.Valid || oldHash.String != identity.hash || !oldResult.Valid {
				return fmt.Errorf("inventory replay identity, revision or fingerprint differs")
			}
			var receipt inventoryCommandReceipt
			encoded := []byte(oldResult.String)
			decoder := json.NewDecoder(bytes.NewReader(encoded))
			decoder.DisallowUnknownFields()
			if !json.Valid(encoded) || decoder.Decode(&receipt) != nil || receipt.Version != inventoryReceiptVersion || receipt.Kind != identity.kind || receipt.RequestID != identity.requestID || receipt.CharacterID != charID || receipt.Revision != revision || receipt.InputHash != identity.hash || !validInventoryOutcome(receipt.Result) || receipt.OutcomeHash != inventoryOutcomeHash(receipt.Result) {
				return fmt.Errorf("inventory command receipt is invalid or unsupported")
			}
			result, replay = receipt.Result, true
			return nil
		}
		if revision != expected {
			return fmt.Errorf("inventory command is stale; recover current state")
		}
		var next int64
		if err := tx.QueryRow(`UPDATE character_shop_state SET revision=revision+1,request_id=NULL,input_hash=NULL,result_json=NULL WHERE character_id=$1 AND revision=$2 RETURNING revision`, charID, expected).Scan(&next); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("inventory command is stale; recover current state")
			}
			return err
		}
		var err error
		result, err = apply(tx)
		if err != nil {
			return err
		}
		snapshot, err = NewStore(tx).GetCharacterSnapshot(ctx, charID)
		if err != nil {
			return err
		}
		if snapshot.CommandRevision != next {
			return fmt.Errorf("inventory revision changed inside command")
		}
		if identity != nil {
			if !validInventoryOutcome(result) {
				return fmt.Errorf("inventory command returned invalid outcome JSON")
			}
			receipt, err := json.Marshal(inventoryCommandReceipt{Version: inventoryReceiptVersion, Kind: identity.kind, RequestID: identity.requestID, CharacterID: charID, Revision: next, InputHash: identity.hash, OutcomeHash: inventoryOutcomeHash(result), Result: result})
			if err != nil {
				return err
			}
			updated, err := tx.Exec(`UPDATE character_shop_state SET request_id=$2,input_hash=$3,result_json=$4 WHERE character_id=$1 AND revision=$5`, charID, identity.requestID, identity.hash, string(receipt), next)
			if err != nil {
				return err
			}
			affected, err := updated.RowsAffected()
			if err != nil {
				return err
			}
			if affected != 1 {
				return fmt.Errorf("inventory receipt owner changed")
			}
		}
		return nil
	})
	if err != nil {
		return CQInventorySnapshot{}, nil, false, err
	}
	return snapshot, result, replay, nil
}

func validInventoryOutcome(result []byte) bool {
	value := bytes.TrimSpace(result)
	return len(value) > 0 && value[0] == '{' && json.Valid(value)
}

func inventoryOutcomeHash(result []byte) string {
	var compact bytes.Buffer
	if err := json.Compact(&compact, result); err != nil {
		return ""
	}
	digest := sha256.Sum256(compact.Bytes())
	return hex.EncodeToString(digest[:])
}

func (s *Store) ValidateCommandSchema(ctx context.Context) error {
	return db.Transaction(ctx, s.database, func(tx db.DBTX) error {
		rows, err := tx.Query(`SELECT character_id,revision,request_id,input_hash,result_json FROM character_shop_state LIMIT 0`)
		if err != nil {
			return fmt.Errorf("inventory command schema: %w", err)
		}
		return rows.Close()
	})
}
