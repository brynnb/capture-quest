package db

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"capturequest/internal/testdb"
	"github.com/jackc/pgconn"
)

// Pool exhaustion and a row-lock wait used to surface as the same unqualified
// deadline error. Keep error identity while identifying the failed boundary.
func TestTransactionFailureStageAndRecovery(t *testing.T) {
	for _, stage := range []string{"begin transaction", "lock character 1", "commit transaction"} {
		t.Run(stage, func(t *testing.T) {
			database := testdb.Postgres(t)
			testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(1,'transaction-test')`)
			ctx := context.Background()
			release := func() {}
			switch stage {
			case "begin transaction":
				database.SetMaxOpenConns(1)
				conn, err := database.Conn(ctx)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { conn.Close() })
				release = func() {
					if err := conn.Close(); err != nil {
						t.Fatal(err)
					}
				}
			case "lock character 1":
				lock, err := database.Begin()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { lock.Rollback() })
				if err := LockCharacter(lock, 1); err != nil {
					t.Fatal(err)
				}
				release = func() {
					if err := lock.Rollback(); err != nil {
						t.Fatal(err)
					}
				}
			case "commit transaction":
				testdb.Exec(t, database, `CREATE FUNCTION reject_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject commit'; END $$;
 CREATE CONSTRAINT TRIGGER reject_commit AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_commit()`)
				release = func() { testdb.Exec(t, database, `DROP TRIGGER reject_commit ON character_data`) }
			}
			if stage != "commit transaction" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
			}
			called := false
			err := Transaction(ctx, database, func(tx DBTX) error {
				called = true
				if err := LockCharacter(tx, 1); err != nil {
					return err
				}
				_, err := tx.Exec(`UPDATE character_data SET x=99 WHERE id=1`)
				return err
			})
			if err == nil || !strings.Contains(err.Error(), stage+":") {
				t.Fatalf("missing stage %q: %v", stage, err)
			}
			if stage == "commit transaction" {
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
					t.Fatalf("lost database error: %v", err)
				}
			} else if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("lost deadline identity: %v", err)
			}
			if stage == "begin transaction" && (called || database.Stats().WaitCount == 0) {
				t.Fatal("pool-wait fixture did not prevent admission")
			}
			release()
			// Prove no partial write escaped and a fresh transaction can acquire the
			// same character after cancellation or commit rejection.
			if err := Transaction(context.Background(), database, func(tx DBTX) error {
				if err := LockCharacter(tx, 1); err != nil {
					return err
				}
				var x int
				if err := tx.QueryRow(`SELECT x FROM character_data WHERE id=1`).Scan(&x); err != nil {
					return err
				}
				if x == 99 {
					t.Fatal("failed transaction committed position")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
