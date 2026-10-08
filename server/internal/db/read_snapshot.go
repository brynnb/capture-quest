package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ReadDBTX supports bounded repository calls with or without an explicit context.
type ReadDBTX interface {
	DBTX
	ContextDBTX
}

// ReadSnapshot prevents an aggregate from mixing publications across its queries.
// It owns one operation budget and returns no partial result on load/commit failure.
func ReadSnapshot[T any](ctx context.Context, database *sql.DB, read func(context.Context, ReadDBTX) (T, error)) (T, error) {
	var zero T
	if database == nil {
		return zero, fmt.Errorf("content database is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return zero, err
	}
	defer tx.Rollback()
	result, err := read(ctx, transactionQueries{tx, ctx})
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, err
	}
	return result, nil
}
