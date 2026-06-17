package world

import (
	"database/sql"
	"testing"

	"capturequest/internal/db"

	_ "modernc.org/sqlite"
)

func TestRuntimeBoulderFacingAttemptIgnoresNonBoulderBlocker(t *testing.T) {
	setupBoulderPushDB(t, false)

	result, attempted, err := TryPushBoulderFromFacingAttempt(1, 59, 10, -44, "UP", true, nil)
	if err != nil {
		t.Fatalf("TryPushBoulderFromFacingAttempt error: %v", err)
	}
	if attempted {
		t.Fatalf("attempted = true, want false for non-boulder blocker")
	}
	if result.Message != boulderNoBoulderMessage {
		t.Fatalf("message = %q, want %q", result.Message, boulderNoBoulderMessage)
	}
	if result.StrengthUsed != nil {
		t.Fatalf("StrengthUsed = %#v, want nil before a boulder is found", result.StrengthUsed)
	}
}

func TestTryPushBoulderChecksStrengthAfterFindingBoulder(t *testing.T) {
	setupBoulderPushDB(t, true)

	result, attempted, err := TryPushBoulderFromFacingAttempt(1, 59, 10, -44, "UP", true, nil)
	if err != nil {
		t.Fatalf("TryPushBoulderFromFacingAttempt error: %v", err)
	}
	if !attempted {
		t.Fatalf("attempted = false, want true when a visible boulder is in front")
	}
	if result.StrengthUsed == nil {
		t.Fatal("StrengthUsed = nil, want Strength check for a real boulder")
	}
	if result.Message != "No POKEMON knows that move." {
		t.Fatalf("message = %q, want Strength failure", result.Message)
	}
}

func TestVictoryRoadBoulderTargetLookupPrefersDatabaseRows(t *testing.T) {
	raw, err := sql.Open("sqlite", "file:victory_road_boulder_targets?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		CREATE TABLE phaser_boulder_targets (
			target_family TEXT,
			map_name TEXT,
			x INTEGER,
			y INTEGER,
			flag TEXT,
			drops_through_hole INTEGER,
			source_object_name TEXT,
			destination_map_name TEXT,
			destination_object_name TEXT
		);
		INSERT INTO phaser_boulder_targets (
			target_family, map_name, x, y, flag, drops_through_hole,
			source_object_name, destination_map_name, destination_object_name
		)
		VALUES (
			'victory_road', 'VICTORY_ROAD_9F', 4, 5, 'EVENT_TEST_BOULDER_SWITCH', 1,
			'SourceBoulder', 'VICTORY_ROAD_8F', 'DestinationBoulder'
		);
	`); err != nil {
		raw.Close()
		t.Fatal(err)
	}
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: raw}
	t.Cleanup(func() {
		db.GlobalWorldDB = previous
		raw.Close()
	})

	target, ok := VictoryRoadBoulderTargetAt("VICTORY_ROAD_9F", 4, 5)
	if !ok {
		t.Fatal("VictoryRoadBoulderTargetAt did not find DB target")
	}
	if target.Flag != "EVENT_TEST_BOULDER_SWITCH" || !target.DropsThroughHole {
		t.Fatalf("target = %#v, want DB-seeded flag and hole behavior", target)
	}
	if target.SourceObjectName != "SourceBoulder" || target.DestinationMapName != "VICTORY_ROAD_8F" || target.DestinationObjectName != "DestinationBoulder" {
		t.Fatalf("target object mapping = %#v, want DB-seeded names", target)
	}
}

func setupBoulderPushDB(t *testing.T, includeBoulder bool) {
	t.Helper()

	raw, err := sql.Open("sqlite", "file:boulder_push_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		CREATE TABLE phaser_objects (
			id INTEGER PRIMARY KEY,
			map_id INTEGER,
			x INTEGER,
			y INTEGER,
			local_x INTEGER,
			local_y INTEGER,
			name TEXT,
			text TEXT,
			sprite_name TEXT
		);
		CREATE TABLE character_object_positions (
			character_id INTEGER,
			object_id INTEGER,
			x INTEGER,
			y INTEGER
		);
		CREATE TABLE phaser_event_object_visibility (
			id INTEGER PRIMARY KEY,
			map_id INTEGER,
			object_name TEXT,
			visible INTEGER,
			requires_flag TEXT,
			requires_flag_absent TEXT,
			label TEXT
		);
		CREATE TABLE character_object_visibility_overrides (
			character_id INTEGER,
			object_id INTEGER,
			visible INTEGER,
			source TEXT
		);
		CREATE TABLE character_pokemon (
			id INTEGER,
			character_id INTEGER,
			party_slot INTEGER,
			box INTEGER,
			pokemon_id INTEGER,
			nickname TEXT,
			level INTEGER,
			exp INTEGER,
			growth_rate TEXT,
			cur_hp INTEGER,
			max_hp INTEGER,
			iv_atk INTEGER,
			iv_def INTEGER,
			iv_spd INTEGER,
			iv_spc INTEGER,
			ev_hp INTEGER,
			ev_atk INTEGER,
			ev_def INTEGER,
			ev_spd INTEGER,
			ev_spc INTEGER,
			move1_id INTEGER,
			move1_pp INTEGER,
			move2_id INTEGER,
			move2_pp INTEGER,
			move3_id INTEGER,
			move3_pp INTEGER,
			move4_id INTEGER,
			move4_pp INTEGER,
			move1_pp_up INTEGER,
			move2_pp_up INTEGER,
			move3_pp_up INTEGER,
			move4_pp_up INTEGER,
			status INTEGER,
			original_trainer_id INTEGER
		);
	`); err != nil {
		raw.Close()
		t.Fatal(err)
	}
	if includeBoulder {
		if _, err := raw.Exec(`
			INSERT INTO phaser_objects (id, map_id, x, y, local_x, local_y, name, text, sprite_name)
			VALUES (1001, 59, 10, -45, 10, -45, 'TestBoulder', '', 'SPRITE_BOULDER')
		`); err != nil {
			raw.Close()
			t.Fatal(err)
		}
	}

	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: raw}
	t.Cleanup(func() {
		db.GlobalWorldDB = previous
		raw.Close()
	})
}
