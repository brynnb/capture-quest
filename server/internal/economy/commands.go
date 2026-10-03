package economy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"capturequest/internal/db"
)

// Advance only inside the mutation's character-locked transaction. A stale
// request remains stale after reconnect or process replacement; failures roll
// back this advancement with payment, inventory and merchant stock.
func advanceShopRevision(tx db.DBTX, charID int32, expected int64) error {
	if expected < 0 || expected >= 9007199254740991 {
		return fmt.Errorf("invalid shop revision")
	}
	if _, err := tx.Exec(`INSERT INTO character_shop_state(character_id,revision) VALUES($1,0) ON CONFLICT DO NOTHING`, charID); err != nil {
		return err
	}
	var revision int64
	err := tx.QueryRow(`UPDATE character_shop_state SET revision=revision+1 WHERE character_id=$1 AND revision=$2 RETURNING revision`, charID, expected).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("shop changed; read current inventory before another command")
	}
	return err
}

// Readiness must fail before serving mutations against a missing schema.
func (s *Service) ValidateSchema(ctx context.Context) error {
	rows, err := s.database.QueryContext(ctx, `SELECT character_id,revision FROM character_shop_state LIMIT 0`)
	if err != nil {
		return fmt.Errorf("shop command schema: %w", err)
	}
	return rows.Close()
}
