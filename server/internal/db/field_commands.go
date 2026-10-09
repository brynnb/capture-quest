package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// ExecuteFieldCommand retains one committed result per character/domain. A
// revision fences every older request even after that latest receipt is replaced.
// The caller supplies gameplay policy; this boundary owns only lock/commit/replay.
func ExecuteFieldCommand(ctx context.Context, database *sql.DB, charID int64, domain, requestID string, expected *int64, input []byte, apply func(DBTX) ([]byte, error)) ([]byte, bool, error) {
	var result []byte
	var replay bool
	digest := sha256.Sum256(input)
	hash := hex.EncodeToString(digest[:])
	err := Transaction(ctx, database, func(tx DBTX) error {
		if err := LockCharacter(tx, charID); err != nil {
			return err
		}
		if expected == nil {
			var err error
			result, err = apply(tx)
			return err
		}
		if domain == "" || len(domain) > 64 || requestID == "" || len(requestID) > 64 || *expected < 0 || *expected >= 9007199254740991 {
			return fmt.Errorf("invalid field command identity")
		}
		if _, err := tx.Exec(`INSERT INTO character_field_command_state(character_id,domain,revision) VALUES($1,$2,0) ON CONFLICT DO NOTHING`, charID, domain); err != nil {
			return err
		}
		var revision int64
		var oldID, oldHash, oldResult sql.NullString
		if err := tx.QueryRow(`SELECT revision,request_id,input_hash,result_json FROM character_field_command_state WHERE character_id=$1 AND domain=$2`, charID, domain).Scan(&revision, &oldID, &oldHash, &oldResult); err != nil {
			return err
		}
		if revision == *expected+1 && oldID.String == requestID {
			if !oldHash.Valid || oldHash.String != hash || !oldResult.Valid {
				return fmt.Errorf("field command replay input differs or receipt is incomplete")
			}
			result = []byte(oldResult.String)
			replay = true
			return nil
		}
		if revision != *expected {
			return fmt.Errorf("field command is stale; recover current state")
		}
		var err error
		result, err = apply(tx)
		if err != nil {
			return err
		}
		if !json.Valid(result) {
			return fmt.Errorf("field command returned invalid JSON receipt")
		}
		changed, err := tx.Exec(`UPDATE character_field_command_state SET revision=revision+1,request_id=$3,input_hash=$4,result_json=$5 WHERE character_id=$1 AND domain=$2 AND revision=$6`, charID, domain, requestID, hash, string(result), *expected)
		if err != nil {
			return err
		}
		rows, err := changed.RowsAffected()
		if err != nil {
			return err
		}
		if rows != 1 {
			return errors.New("field command revision changed")
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return result, replay, nil
}

func FieldCommandRevisions(q DBTX, charID int64) (map[string]int64, error) {
	result := map[string]int64{}
	rows, err := q.Query(`SELECT domain,revision FROM character_field_command_state WHERE character_id=$1`, charID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var domain string
		var revision int64
		if err := rows.Scan(&domain, &revision); err != nil {
			return nil, err
		}
		result[domain] = revision
	}
	return result, rows.Err()
}
