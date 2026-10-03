package cqitems

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"capturequest/internal/db"
)

// ExecuteCommand owns admission, the durable revision and the committed bag
// projection for shop and party-item commands. A stale request is rejected;
// clients recover current state without automatically resending the mutation.
// Unlike repository helpers, this boundary cannot join a parent transaction:
// returning a successful projection requires owning the final commit.
// Keep the existing character_shop_state row so deployed revisions are preserved
// without a second counter or a data migration. Its scope now includes party use.
func (s *Store) ExecuteCommand(ctx context.Context, charID int32, expected int64, apply func(db.DBTX) error) (CQInventorySnapshot, error) {
	database, ok := s.database.(*sql.DB)
	if !ok || database == nil {
		return CQInventorySnapshot{}, fmt.Errorf("inventory command requires a database pool to own its commit, got %T", s.database)
	}
	var result CQInventorySnapshot
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if expected < 0 || expected >= 9007199254740991 {
			return fmt.Errorf("invalid inventory command revision")
		}
		if err := db.LockCharacter(tx, int64(charID)); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO character_shop_state(character_id,revision) VALUES($1,0) ON CONFLICT DO NOTHING`, charID); err != nil {
			return err
		}
		var next int64
		if err := tx.QueryRow(`UPDATE character_shop_state SET revision=revision+1 WHERE character_id=$1 AND revision=$2 RETURNING revision`, charID, expected).Scan(&next); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("inventory command is stale; recover current state")
			}
			return err
		}
		if err := apply(tx); err != nil {
			return err
		}
		var err error
		result, err = NewStore(tx).GetCharacterSnapshot(ctx, charID)
		return err
	})
	if err != nil {
		return CQInventorySnapshot{}, err
	}
	return result, nil
}

func (s *Store) ValidateCommandSchema(ctx context.Context) error {
	return db.Transaction(ctx, s.database, func(tx db.DBTX) error {
		rows, err := tx.Query(`SELECT character_id,revision FROM character_shop_state LIMIT 0`)
		if err != nil {
			return fmt.Errorf("inventory command schema: %w", err)
		}
		return rows.Close()
	})
}
