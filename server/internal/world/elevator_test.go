package world

import (
	"database/sql"
	"path/filepath"
	"testing"

	"capturequest/internal/db"

	_ "modernc.org/sqlite"
)

func TestAvailableElevatorFloorsReturnsOrderedFloors(t *testing.T) {
	setupElevatorRuntimeDB(t)

	access, err := AvailableElevatorFloors(1, 236, nil)
	if err != nil {
		t.Fatalf("AvailableElevatorFloors: %v", err)
	}
	if access.Message != "" {
		t.Fatalf("message = %q, want empty", access.Message)
	}
	if len(access.Floors) != 3 {
		t.Fatalf("floors = %#v, want three Silph floors", access.Floors)
	}

	want := []struct {
		label string
		mapID int
		x     int
		y     int
	}{
		{"1F", 181, 20, 0},
		{"2F", 207, 20, 0},
		{"11F", 235, 13, 0},
	}
	for i, expected := range want {
		floor := access.Floors[i]
		if floor.FloorLabel != expected.label || floor.FloorMapID != expected.mapID || floor.DestX != expected.x || floor.DestY != expected.y {
			t.Fatalf("floor[%d] = %#v, want %s map %d (%d,%d)",
				i, floor, expected.label, expected.mapID, expected.x, expected.y)
		}
	}
}

func TestElevatorDestinationRequiresCurrentElevatorFloorPair(t *testing.T) {
	setupElevatorRuntimeDB(t)

	floor, err := ElevatorDestination(1, 236, 235, nil)
	if err != nil {
		t.Fatalf("ElevatorDestination: %v", err)
	}
	if floor.FloorLabel != "11F" || floor.DestX != 13 || floor.DestY != 0 {
		t.Fatalf("Silph 11F destination = %#v, want 11F at (13,0)", floor)
	}

	if _, err := ElevatorDestination(1, 127, 235, nil); err == nil {
		t.Fatal("ElevatorDestination accepted a Silph floor from the Celadon elevator")
	}
}

func TestElevatorRuntimeSupportsCeladonAndRocketHideout(t *testing.T) {
	setupElevatorRuntimeDB(t)

	celadon, err := AvailableElevatorFloors(1, 127, nil)
	if err != nil {
		t.Fatalf("AvailableElevatorFloors Celadon: %v", err)
	}
	if len(celadon.Floors) != 1 || celadon.Floors[0].FloorLabel != "5F" {
		t.Fatalf("Celadon floors = %#v, want 5F in fixture", celadon.Floors)
	}

	rocketLocked, err := AvailableElevatorFloors(1, 203, nil)
	if err != nil {
		t.Fatalf("AvailableElevatorFloors Rocket locked: %v", err)
	}
	if len(rocketLocked.Floors) != 0 || rocketLocked.Message != elevatorNeedsKeyMessage {
		t.Fatalf("locked Rocket elevator = %#v, want no floors and key message", rocketLocked)
	}

	rocketUnlocked, err := AvailableElevatorFloors(42, 203, nil)
	if err != nil {
		t.Fatalf("AvailableElevatorFloors Rocket unlocked: %v", err)
	}
	if len(rocketUnlocked.Floors) != 3 {
		t.Fatalf("unlocked Rocket floors = %#v, want three floors", rocketUnlocked.Floors)
	}

	floor, err := ElevatorDestination(42, 203, 202, nil)
	if err != nil {
		t.Fatalf("ElevatorDestination Rocket B4F: %v", err)
	}
	if floor.FloorLabel != "B4F" || floor.DestX != 24 || floor.DestY != 15 {
		t.Fatalf("Rocket B4F destination = %#v, want B4F at (24,15)", floor)
	}

	if _, err := ElevatorDestination(1, 203, 202, nil); err == nil {
		t.Fatal("Rocket elevator B4F was accessible without Lift Key")
	}
}

func setupElevatorRuntimeDB(t *testing.T) {
	t.Helper()

	raw, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "elevator-runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		CREATE TABLE phaser_elevator_floors (
			elevator_map_id INTEGER NOT NULL,
			floor_map_id INTEGER NOT NULL,
			floor_label TEXT NOT NULL,
			dest_x INTEGER NOT NULL,
			dest_y INTEGER NOT NULL,
			sort_order INTEGER NOT NULL DEFAULT 0,
			requires_flag TEXT DEFAULT NULL,
			requires_item_id INTEGER DEFAULT NULL
		);
		CREATE TABLE cq_item_instances (
			id INTEGER PRIMARY KEY,
			item_id INTEGER NOT NULL,
			quantity INTEGER NOT NULL DEFAULT 1
		);
		CREATE TABLE cq_character_inventory (
			character_id INTEGER NOT NULL,
			item_instance_id INTEGER NOT NULL
		);
		INSERT INTO phaser_elevator_floors (
			elevator_map_id, floor_map_id, floor_label, dest_x, dest_y, sort_order, requires_item_id
		) VALUES
			(236, 235, '11F', 13, 0, 11, NULL),
			(236, 181, '1F', 20, 0, 1, NULL),
			(236, 207, '2F', 20, 0, 2, NULL),
			(127, 136, '5F', 1, 1, 5, NULL),
			(203, 199, 'B1F', 24, 19, 1, 74),
			(203, 200, 'B2F', 24, 19, 2, 74),
			(203, 202, 'B4F', 24, 15, 3, 74);
		INSERT INTO cq_item_instances (id, item_id, quantity)
		VALUES (7001, 74, 1);
		INSERT INTO cq_character_inventory (character_id, item_instance_id)
		VALUES (42, 7001);
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
}
