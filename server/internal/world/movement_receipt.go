package world

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/protocol"
)

const movementReceiptVersion = 1

type storedMovementReceipt struct {
	Version int                          `json:"version"`
	Result  protocol.CommittedPlayerStep `json:"result"`
}

func validMovementToken(token string) bool {
	if len(token) != 32 {
		return false
	}
	decoded, err := hex.DecodeString(token)
	return err == nil && hex.EncodeToString(decoded) == token
}

// This is bounded by the one-outstanding-step protocol. Unconfirmed acceptance,
// failed commits, forced points and teleports cannot replace a completed receipt.
func saveMovementReceiptIn(tx db.DBTX, charID int64, token string, result movementStepResult) error {
	if err := db.RequireTransaction(tx); err != nil {
		return err
	}
	if !validMovementToken(token) {
		return fmt.Errorf("invalid movement receipt token")
	}
	receipt := storedMovementReceipt{Version: movementReceiptVersion, Result: protocol.CommittedPlayerStep{StepToken: token, MapID: result.MapID, X: result.X, Y: result.Y, Direction: result.Direction}}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO character_movement_receipts(character_id,step_token,result_json) VALUES($1,$2,$3) ON CONFLICT(character_id) DO UPDATE SET step_token=EXCLUDED.step_token,result_json=EXCLUDED.result_json,committed_at=CURRENT_TIMESTAMP`, charID, token, string(encoded))
	return err
}

func loadMovementReceipt(ctx context.Context, database *sql.DB, charID int64, token string) (*protocol.CommittedPlayerStep, error) {
	var result *protocol.CommittedPlayerStep
	err := db.Transaction(ctx, database, func(q db.DBTX) error {
		var encoded string
		err := q.QueryRow(`SELECT result_json FROM character_movement_receipts WHERE character_id=$1 AND step_token=$2`, charID, token).Scan(&encoded)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		var receipt storedMovementReceipt
		if err := decodePlayerMovement([]byte(encoded), &receipt); err != nil {
			return fmt.Errorf("decode movement receipt for character %d: %w", charID, err)
		}
		// Integer zero is a valid coordinate, so typed decoding alone cannot
		// distinguish it from a missing/null field in a damaged stored record.
		var coordinates struct {
			Result struct {
				X *int `json:"x"`
				Y *int `json:"y"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(encoded), &coordinates); err != nil || coordinates.Result.X == nil || coordinates.Result.Y == nil {
			return fmt.Errorf("movement receipt for character %d lacks integer coordinates", charID)
		}
		if receipt.Version != movementReceiptVersion || receipt.Result.StepToken != token || !validMovementToken(token) || receipt.Result.MapID == 0 || normalizeWarpDirection(receipt.Result.Direction) != receipt.Result.Direction {
			return fmt.Errorf("invalid movement receipt for character %d: version=%d token=%q map=%d direction=%q", charID, receipt.Version, receipt.Result.StepToken, receipt.Result.MapID, receipt.Result.Direction)
		}
		result = &receipt.Result
		return nil
	})
	return result, err
}

func (m *PlayerMovementManager) Load(ctx context.Context) error {
	rows, err := m.wh.database.QueryContext(ctx, `SELECT character_id,step_token,result_json,committed_at FROM character_movement_receipts LIMIT 0`)
	if err != nil {
		return fmt.Errorf("movement receipt schema: %w", err)
	}
	return rows.Close()
}
