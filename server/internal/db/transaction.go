package db

import (
	"context"
	"database/sql"
	"errors"
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

// ContextDBTX is the cancellable query surface shared by database and transaction handles.
// Bootstrap operations use the caller's lifecycle context on every query.
type ContextDBTX interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// RequireTransaction guards operations whose caller owns the atomic boundary.
// A plain database would let partially completed writes escape on failure.
func RequireTransaction(database DBTX) error {
	switch handle := database.(type) {
	case transactionQueries:
		if handle.tx == nil {
			return fmt.Errorf("transaction handle is nil")
		}
		return nil
	case *sql.Tx:
		if handle == nil {
			return fmt.Errorf("transaction handle is nil")
		}
		return nil
	default:
		return fmt.Errorf("operation requires a transaction, got %T", database)
	}
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
		if database == nil {
			return fmt.Errorf("transaction database is required")
		}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}
		defer tx.Rollback()
		if err := operation(transactionQueries{tx, ctx}); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit transaction: %w", err)
		}
		return nil
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

// LockCharacter serializes one character inside an owned PostgreSQL transaction
// without manufacturing a write. A plain pool cannot retain this lock, so it is
// rejected before querying. A no-op UPDATE would fire triggers and create rows.
func LockCharacter(database DBTX, characterID int64) error {
	if err := RequireTransaction(database); err != nil {
		return fmt.Errorf("character ownership: %w", err)
	}
	var id int64
	err := database.QueryRow(`SELECT id FROM character_data WHERE id=$1 FOR UPDATE`, characterID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("character %d does not exist", characterID)
	}
	if err != nil {
		return fmt.Errorf("lock character %d: %w", characterID, err)
	}
	return nil
}

// Context queries let an owned transaction reuse cancellable read services
// without escaping to the pool. Both the outer deadline and caller cancellation
// apply. Query contexts retire when the outer transaction ends, including rows
// whose consumption outlives QueryContext's return.
func (q transactionQueries) queryContext(caller context.Context) context.Context {
	ctx, cancel := context.WithCancel(q.ctx)
	stopCaller := context.AfterFunc(caller, cancel)
	context.AfterFunc(ctx, func() { stopCaller() })
	if caller.Err() != nil {
		cancel()
	}
	return ctx
}
func (q transactionQueries) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return q.tx.QueryContext(q.queryContext(ctx), query, args...)
}
func (q transactionQueries) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return q.tx.QueryRowContext(q.queryContext(ctx), query, args...)
}
func (q transactionQueries) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return q.tx.ExecContext(q.queryContext(ctx), query, args...)
}
