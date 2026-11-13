package db_character

import (
	"context"
	"errors"
	"testing"
	"time"

	"capturequest/internal/testdb"
)

func TestLastLoginPreservesGameplayAndRepeatedPlaytimeTotals(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,account_id,map_id,x,y,z,heading,last_login,time_played) VALUES(9,7,1,2,3,8,9,100,10)`)
	for _, total := range []uint32{15, 15, 12} {
		if err := SaveCharacterPlaytime(context.Background(), database, 9, 7, total); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := SetCharacterLastLogin(context.Background(), database, 9, 7, 200); err != nil {
			t.Fatal(err)
		}
	}
	var mapID, x, y, z, heading, lastLogin, seconds int
	if err := database.QueryRow(`SELECT map_id,x,y,z,heading,last_login,time_played FROM character_data WHERE id=9`).Scan(&mapID, &x, &y, &z, &heading, &lastLogin, &seconds); err != nil {
		t.Fatal(err)
	}
	if mapID != 1 || x != 2 || y != 3 || z != 8 || heading != 9 || lastLogin != 200 || seconds != 15 {
		t.Fatalf("login changed gameplay: map=%d pose=(%d,%d,%d,%d) login=%d playtime=%d", mapID, x, y, z, heading, lastLogin, seconds)
	}
	for _, identity := range [][2]int64{{404, 7}, {9, 8}} {
		if err := SetCharacterLastLogin(context.Background(), database, uint32(identity[0]), identity[1], 300); err == nil {
			t.Fatalf("invalid identity accepted: %v", identity)
		}
	}
	testdb.Exec(t, database, `UPDATE character_data SET deleted_at=CURRENT_TIMESTAMP WHERE id=9`)
	if err := SetCharacterLastLogin(context.Background(), database, 9, 7, 300); err == nil {
		t.Fatal("deleted character accepted login")
	}
	if err := SetCharacterLastLogin(context.Background(), nil, 9, 7, 300); err == nil {
		t.Fatal("missing database accepted login")
	}
	if err := SaveCharacterPlaytime(context.Background(), database, 404, 7, 5); err == nil {
		t.Fatal("missing character accepted playtime")
	}
	if err := database.QueryRow(`SELECT last_login FROM character_data WHERE id=9`).Scan(&lastLogin); err != nil || lastLogin != 200 {
		t.Fatalf("rejected login changed metadata=%d error=%v", lastLogin, err)
	}
}

func TestLastLoginCommitFailureAndPoolCancellationLeaveMetadataUnchanged(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,account_id,last_login) VALUES(9,7,100);
 CREATE FUNCTION reject_login_save() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject login'; END $$;
 CREATE CONSTRAINT TRIGGER reject_login_save AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_login_save();`)
	if err := SetCharacterLastLogin(context.Background(), database, 9, 7, 200); err == nil {
		t.Fatal("late login failure accepted")
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_login_save ON character_data`)
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := SetCharacterLastLogin(ctx, database, 9, 7, 200); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancelled login=%v", err)
	}
	lease.Close()
	var lastLogin int
	if err := database.QueryRow(`SELECT last_login FROM character_data WHERE id=9`).Scan(&lastLogin); err != nil || lastLogin != 100 {
		t.Fatalf("failed login=%d error=%v", lastLogin, err)
	}
	if err := SetCharacterLastLogin(context.Background(), database, 9, 7, 200); err != nil {
		t.Fatal(err)
	}
}
