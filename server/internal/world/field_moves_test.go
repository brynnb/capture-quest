package world

import (
	"database/sql"
	"testing"

	"capturequest/internal/db"
	"capturequest/internal/session"

	_ "modernc.org/sqlite"
)

func TestFieldMoveBadgeGateRules(t *testing.T) {
	tests := []struct {
		move      string
		moveID    int
		badgeFlag string
	}{
		{move: "CUT", moveID: 15, badgeFlag: "EVENT_GOT_CASCADEBADGE"},
		{move: "FLY", moveID: 19, badgeFlag: "EVENT_GOT_THUNDERBADGE"},
		{move: "SURF", moveID: 57, badgeFlag: "EVENT_GOT_SOULBADGE"},
		{move: "STRENGTH", moveID: 70, badgeFlag: "EVENT_GOT_RAINBOWBADGE"},
		{move: "FLASH", moveID: 148, badgeFlag: "EVENT_GOT_BOULDERBADGE"},
	}

	for _, tt := range tests {
		t.Run(tt.move, func(t *testing.T) {
			rule, ok := fieldMoveRuleForName(tt.move)
			if !ok {
				t.Fatalf("fieldMoveRuleForName(%q) not found", tt.move)
			}
			if rule.MoveID != tt.moveID {
				t.Fatalf("%s move ID = %d, want %d", tt.move, rule.MoveID, tt.moveID)
			}
			if rule.RequiredBadgeFlag != tt.badgeFlag {
				t.Fatalf("%s badge flag = %q, want %q", tt.move, rule.RequiredBadgeFlag, tt.badgeFlag)
			}
		})
	}
}

func TestFieldMoveNoBadgeRequiredRules(t *testing.T) {
	for _, move := range []string{"DIG", "TELEPORT", "SOFTBOILED"} {
		t.Run(move, func(t *testing.T) {
			rule, ok := fieldMoveRuleForName(move)
			if !ok {
				t.Fatalf("fieldMoveRuleForName(%q) not found", move)
			}
			if rule.RequiredBadgeFlag != "" {
				t.Fatalf("%s badge flag = %q, want no badge gate", move, rule.RequiredBadgeFlag)
			}
		})
	}
}

func TestRecordFieldMoveStateUpsertsMapState(t *testing.T) {
	raw := setupFieldMoveStateTestDB(t)

	if err := recordFieldMoveState(7, 40, "flash"); err != nil {
		t.Fatalf("record FLASH state: %v", err)
	}
	if err := recordFieldMoveState(7, 41, "FLASH"); err != nil {
		t.Fatalf("update FLASH state: %v", err)
	}

	var (
		moveName string
		mapID    int
		active   int
	)
	if err := raw.QueryRow(`
		SELECT move_name, map_id, active
		FROM character_field_move_state
		WHERE character_id = 7`,
	).Scan(&moveName, &mapID, &active); err != nil {
		t.Fatalf("query field move state: %v", err)
	}
	if moveName != "FLASH" || mapID != 41 || active != 1 {
		t.Fatalf("field move state = (%s,%d,%d), want (FLASH,41,1)", moveName, mapID, active)
	}
}

func TestCharacterBindDestinationUsesPrimaryBind(t *testing.T) {
	setupFieldMoveStateTestDB(t)
	if _, err := db.GlobalWorldDB.DB.Exec(`
		INSERT INTO character_bind (id, slot, map_id, x, y, z, heading)
		VALUES (9, 0, 67, 3, 7, 0, 180)`); err != nil {
		t.Fatalf("insert bind: %v", err)
	}

	mapID, x, y, err := characterBindDestination(9)
	if err != nil {
		t.Fatalf("characterBindDestination: %v", err)
	}
	if mapID != 67 || x != 3 || y != 7 {
		t.Fatalf("bind destination = (%d,%d,%d), want (67,3,7)", mapID, x, y)
	}
}

func TestCharacterBindDestinationFallsBackToRecoverySpawn(t *testing.T) {
	setupFieldMoveStateTestDB(t)

	mapID, x, y, err := characterBindDestination(999)
	if err != nil {
		t.Fatalf("characterBindDestination fallback: %v", err)
	}
	if mapID != RecoverySpawnMap || x != int(RecoverySpawnX) || y != int(RecoverySpawnY) {
		t.Fatalf("fallback destination = (%d,%d,%d), want (%d,%d,%d)", mapID, x, y, RecoverySpawnMap, int(RecoverySpawnX), int(RecoverySpawnY))
	}
}

func TestEscapeRopeDestinationPrefersOverworldExit(t *testing.T) {
	raw := setupFieldMoveStateTestDB(t)
	insertFieldMoveMap(t, raw, 50, "ROCK_TUNNEL_B1F", 0)
	insertFieldMoveMap(t, raw, 60, "ROCK_TUNNEL_1F", 0)
	insertFieldMoveMap(t, raw, 16, "ROUTE5", 1)
	insertFieldMoveWarp(t, raw, 1, 50, 60, 11, 12)
	insertFieldMoveWarp(t, raw, 2, 50, 16, 197, -135)

	mapID, x, y, err := escapeRopeDestination(&session.Session{MapID: 50, X: 4, Y: 7}, nil)
	if err != nil {
		t.Fatalf("escapeRopeDestination: %v", err)
	}
	if mapID != 16 || x != 197 || y != -135 {
		t.Fatalf("escape destination = (%d,%d,%d), want (16,197,-135)", mapID, x, y)
	}
}

func TestEscapeRopeDestinationRejectsUnifiedOverworld(t *testing.T) {
	setupFieldMoveStateTestDB(t)

	if _, _, _, err := escapeRopeDestination(&session.Session{MapID: UnifiedOverworldMapID}, nil); err == nil {
		t.Fatal("escapeRopeDestination on unified overworld succeeded, want error")
	}
}

func TestEscapeRopeDestinationRejectsLocalOverworldMap(t *testing.T) {
	raw := setupFieldMoveStateTestDB(t)
	insertFieldMoveMap(t, raw, 16, "ROUTE5", 1)
	insertFieldMoveMap(t, raw, 67, "CERULEAN_MART", 0)
	insertFieldMoveWarp(t, raw, 1, 16, 67, 3, 7)

	if _, _, _, err := escapeRopeDestination(&session.Session{MapID: 16}, nil); err == nil {
		t.Fatal("escapeRopeDestination on local overworld map succeeded, want error")
	}
}

func TestEscapeRopeDestinationRequiresExitWarp(t *testing.T) {
	raw := setupFieldMoveStateTestDB(t)
	insertFieldMoveMap(t, raw, 50, "ROCK_TUNNEL_B1F", 0)

	if _, _, _, err := escapeRopeDestination(&session.Session{MapID: 50}, nil); err == nil {
		t.Fatal("escapeRopeDestination with no exit warp succeeded, want error")
	}
}

func setupFieldMoveStateTestDB(t *testing.T) *sql.DB {
	t.Helper()
	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { raw.Close() })

	stmts := []string{
		`CREATE TABLE character_field_move_state (
			character_id bigint NOT NULL,
			move_name varchar(50) NOT NULL,
			map_id integer NOT NULL,
			active smallint NOT NULL DEFAULT 1,
			updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (character_id, move_name)
		)`,
		`CREATE TABLE character_bind (
			id integer NOT NULL,
			slot integer NOT NULL,
			map_id integer NOT NULL,
			x real NOT NULL,
			y real NOT NULL,
			z real NOT NULL,
			heading real NOT NULL,
			PRIMARY KEY (id, slot)
		)`,
		`CREATE TABLE phaser_maps (
			id integer PRIMARY KEY,
			name text NOT NULL,
			is_overworld smallint NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE phaser_warps (
			id integer PRIMARY KEY,
			source_map_id integer NOT NULL,
			destination_map_id integer,
			destination_x integer,
			destination_y integer
		)`,
	}
	for _, stmt := range stmts {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("setup test db: %v", err)
		}
	}

	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: raw}
	t.Cleanup(func() { db.GlobalWorldDB = previous })
	return raw
}

func insertFieldMoveMap(t *testing.T, raw *sql.DB, id int, name string, isOverworld int) {
	t.Helper()
	if _, err := raw.Exec(`INSERT INTO phaser_maps (id, name, is_overworld) VALUES (?, ?, ?)`, id, name, isOverworld); err != nil {
		t.Fatalf("insert phaser map %d: %v", id, err)
	}
}

func insertFieldMoveWarp(t *testing.T, raw *sql.DB, id, sourceMapID, destinationMapID, destinationX, destinationY int) {
	t.Helper()
	if _, err := raw.Exec(`
		INSERT INTO phaser_warps (id, source_map_id, destination_map_id, destination_x, destination_y)
		VALUES (?, ?, ?, ?, ?)`,
		id,
		sourceMapID,
		destinationMapID,
		destinationX,
		destinationY,
	); err != nil {
		t.Fatalf("insert phaser warp %d: %v", id, err)
	}
}
