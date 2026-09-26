package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DBTX is the common query surface for owned runtime repositories. A transaction
// passed to a repository must remain the only query handle for that operation.
type DBTX interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
	Exec(string, ...any) (sql.Result, error)
}

// Transaction joins a supplied transaction, or owns a bounded transaction on a
// database. Callers joining an existing transaction must propagate errors and
// must not publish success or mutate caches until their outer commit succeeds.
func Transaction(ctx context.Context, database DBTX, operation func(DBTX) error) error {
	switch database := database.(type) {
	case transactionQueries:
		return operation(database)
	case *sql.Tx:
		return operation(transactionQueries{database, ctx})
	case *sql.DB:
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if err := operation(transactionQueries{tx, ctx}); err != nil {
			return err
		}
		return tx.Commit()
	default:
		return fmt.Errorf("transaction requires a database or transaction, got %T", database)
	}
}

type transactionQueries struct {
	tx  *sql.Tx
	ctx context.Context
}

func (q transactionQueries) Query(query string, args ...any) (*sql.Rows, error) {
	return q.tx.QueryContext(q.ctx, query, args...)
}
func (q transactionQueries) QueryRow(query string, args ...any) *sql.Row {
	return q.tx.QueryRowContext(q.ctx, query, args...)
}
func (q transactionQueries) Exec(query string, args ...any) (sql.Result, error) {
	return q.tx.ExecContext(q.ctx, query, args...)
}

// LockCharacter serializes durable operations for one character, including when
// the wallet or inventory is empty. A no-op update takes a row lock on PostgreSQL
// and also works in the offline SQLite simulator. Call only inside Transaction.
func LockCharacter(database DBTX, characterID int64) error {
	result, err := database.Exec(`UPDATE character_data SET id = id WHERE id = $1`, characterID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("character %d does not exist", characterID)
	}
	return nil
}
