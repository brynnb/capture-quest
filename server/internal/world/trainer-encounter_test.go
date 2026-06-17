package world

import (
	"database/sql"
	"testing"

	"capturequest/internal/db"

	_ "modernc.org/sqlite"
)

func TestTrainerApproachTargetForPlayer(t *testing.T) {
	mgr := &TrainerEncounterManager{}
	tests := []struct {
		name      string
		direction string
		wantX     int
		wantY     int
	}{
		{name: "trainer facing up stops below player", direction: "UP", wantX: 10, wantY: 11},
		{name: "trainer facing down stops above player", direction: "DOWN", wantX: 10, wantY: 9},
		{name: "trainer facing left stops right of player", direction: "LEFT", wantX: 11, wantY: 10},
		{name: "trainer facing right stops left of player", direction: "RIGHT", wantX: 9, wantY: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotX, gotY := mgr.approachTargetForPlayer(&trainerSightData{Direction: tt.direction}, 10, 10)
			if gotX != tt.wantX || gotY != tt.wantY {
				t.Fatalf("approachTargetForPlayer() = (%d,%d), want (%d,%d)", gotX, gotY, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestTrainerCanAutoTriggerBySightSkipsGymLeaders(t *testing.T) {
	mgr := &TrainerEncounterManager{}

	if !mgr.canAutoTriggerBySight(&trainerSightData{TrainerClass: "BUG_CATCHER"}) {
		t.Fatal("regular trainer should be allowed to auto-trigger by sight")
	}
	if mgr.canAutoTriggerBySight(&trainerSightData{TrainerClass: "BROCK", IsGymLeader: true}) {
		t.Fatal("gym leader should require direct interaction instead of sight auto-trigger")
	}
	if mgr.canAutoTriggerBySight(nil) {
		t.Fatal("nil trainer should not auto-trigger")
	}
}

func TestTrainerActiveSightTilesUsesTrainerRangeAndFacing(t *testing.T) {
	mgr := &TrainerEncounterManager{}
	trainer := &trainerSightData{
		MapID:      59,
		X:          12,
		Y:          16,
		Direction:  "RIGHT",
		SightRange: 3,
	}

	got := mgr.activeSightTiles(0, trainer)
	want := []PathNode{{X: 13, Y: 16}, {X: 14, Y: 16}, {X: 15, Y: 16}}
	if len(got) != len(want) {
		t.Fatalf("activeSightTiles length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("activeSightTiles[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestTrainerSightLineBlockedByCollision(t *testing.T) {
	previousDB := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() {
		db.GlobalWorldDB = previousDB
	})

	mgr := &TrainerEncounterManager{
		wh: &WorldHandler{
			ActorManager: &PhaserActorManager{
				collisionMap: map[int]map[string]int{
					59: {
						tileKey(13, 16): 1,
						tileKey(14, 16): 0,
						tileKey(15, 16): 1,
					},
				},
			},
		},
	}
	trainer := &trainerSightData{
		MapID:      59,
		X:          12,
		Y:          16,
		Direction:  "RIGHT",
		SightRange: 3,
	}

	if !mgr.hasClearSightLine(0, trainer, 13, 16) {
		t.Fatal("first visible tile should be clear")
	}
	if mgr.hasClearSightLine(0, trainer, 15, 16) {
		t.Fatal("wall tile between trainer and player should block sight")
	}
	got := mgr.activeSightTiles(0, trainer)
	want := []PathNode{{X: 13, Y: 16}}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("activeSightTiles with blocker = %v, want %v", got, want)
	}
}

func TestTrainerSightLineBlockedByVisibleObject(t *testing.T) {
	setupTrainerSightObjectBlockerDB(t)

	mgr := &TrainerEncounterManager{
		wh: &WorldHandler{
			ActorManager: &PhaserActorManager{
				collisionMap: map[int]map[string]int{
					59: {
						tileKey(13, 16): 1,
						tileKey(14, 16): 1,
						tileKey(15, 16): 1,
					},
				},
			},
		},
	}
	trainer := &trainerSightData{
		ObjectID:   366,
		MapID:      59,
		X:          12,
		Y:          16,
		Direction:  "RIGHT",
		SightRange: 3,
	}

	if !mgr.hasClearSightLine(7, trainer, 13, 16) {
		t.Fatal("tile before object blocker should be clear")
	}
	if mgr.hasClearSightLine(7, trainer, 15, 16) {
		t.Fatal("visible object between trainer and player should block sight")
	}
}

func setupTrainerSightObjectBlockerDB(t *testing.T) {
	t.Helper()

	raw, err := sql.Open("sqlite", ":memory:")
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
			object_type TEXT
		);
		CREATE TABLE character_object_positions (
			character_id INTEGER,
			object_id INTEGER,
			x INTEGER,
			y INTEGER
		);
		CREATE TABLE character_collected_items (
			character_id INTEGER,
			object_id INTEGER
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
		INSERT INTO phaser_objects (id, map_id, x, y, name, object_type)
		VALUES (999, 59, 14, 16, 'BlockingNPC', 'npc');
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
