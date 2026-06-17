package world

import (
	"database/sql"
	"testing"

	"capturequest/internal/db"

	_ "modernc.org/sqlite"
)

func TestActorForCharacterHonorsOakLabSourceVisibility(t *testing.T) {
	setupActorVisibilityDB(t)

	wh := &WorldHandler{
		ActorRegistry: NewActorRegistry(),
	}
	mgr := NewPhaserActorManager(wh)
	wh.ActorManager = mgr

	oakTopName := "OaksLab_NPC_5"
	oakTop := PhaserActor{
		ID:         wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 105),
		DbID:       105,
		MapID:      40,
		ObjectType: "npc",
		Name:       &oakTopName,
	}
	if _, visible := mgr.actorForCharacter(oakTop, 1); visible {
		t.Fatal("top Oak visible before source event state, want hidden")
	}

	oakEntryName := "OaksLab_NPC_8"
	oakEntry := PhaserActor{
		ID:         wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 108),
		DbID:       108,
		MapID:      40,
		ObjectType: "npc",
		Name:       &oakEntryName,
	}
	if _, visible := mgr.actorForCharacter(oakEntry, 1); visible {
		t.Fatal("entry Oak visible before source event state, want hidden")
	}

	rivalName := "OaksLab_NPC_1"
	rival := PhaserActor{
		ID:         wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 101),
		DbID:       101,
		MapID:      40,
		ObjectType: "npc",
		Name:       &rivalName,
	}
	if _, visible := mgr.actorForCharacter(rival, 1); !visible {
		t.Fatal("rival hidden by Oak Lab visibility rules, want visible from source initial state")
	}

	if err := SetCharacterObjectVisibilityOverride(1, 105, true, "test"); err != nil {
		t.Fatalf("SetCharacterObjectVisibilityOverride: %v", err)
	}
	if _, visible := mgr.actorForCharacter(oakTop, 1); !visible {
		t.Fatal("top Oak hidden after character show override, want visible")
	}
}

func TestLoadWalkingActorsPreservesOriginalObjectID(t *testing.T) {
	setupActorVisibilityDB(t)

	wh := &WorldHandler{
		ActorRegistry: NewActorRegistry(),
	}
	mgr := NewPhaserActorManager(wh)
	wh.ActorManager = mgr

	mgr.loadWalkingActors()

	runtimeID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 101)
	actor, ok := mgr.walkingActors[runtimeID]
	if !ok {
		t.Fatalf("walking actor %d not loaded", runtimeID)
	}
	if actor.DbID != 101 {
		t.Fatalf("walking actor DbID = %d, want original object id 101", actor.DbID)
	}
	if actor.ID != runtimeID {
		t.Fatalf("walking actor ID = %d, want runtime id %d", actor.ID, runtimeID)
	}
}

func setupActorVisibilityDB(t *testing.T) {
	t.Helper()

	raw, err := sql.Open("sqlite", "file:actor_visibility_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		CREATE TABLE phaser_maps (
			id INTEGER PRIMARY KEY,
			name TEXT,
			is_overworld INTEGER DEFAULT 0
		);
		CREATE TABLE phaser_objects (
			id INTEGER PRIMARY KEY,
			map_id INTEGER,
			x INTEGER,
			y INTEGER,
			local_x INTEGER,
			local_y INTEGER,
			object_type TEXT,
			sprite_name TEXT,
			name TEXT,
			action_type TEXT,
			action_direction TEXT,
			movement_type TEXT,
			text TEXT,
			trainer_class TEXT,
			trainer_party_index INTEGER,
			item_id INTEGER
		);
		CREATE TABLE phaser_tiles (
			map_id INTEGER,
			x INTEGER,
			y INTEGER,
			collision_type INTEGER,
			raw_foot_tile_id INTEGER
		);
		CREATE TABLE character_object_positions (
			character_id INTEGER,
			object_id INTEGER,
			map_id INTEGER,
			x INTEGER,
			y INTEGER,
			PRIMARY KEY (character_id, object_id)
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
			source TEXT,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (character_id, object_id)
		);

		INSERT INTO phaser_maps (id, name, is_overworld)
		VALUES (40, 'OAKS_LAB', 0);
		INSERT INTO phaser_objects (
			id, map_id, x, y, local_x, local_y, object_type, sprite_name, name,
			action_type, action_direction, movement_type, text
		) VALUES
			(101, 40, 4, 3, 4, 3, 'npc', 'SPRITE_BLUE', 'OaksLab_NPC_1', 'STAY', 'DOWN', 'LAND', 'TEXT_OAKSLAB_RIVAL'),
			(105, 40, 5, 2, 5, 2, 'npc', 'SPRITE_OAK', 'OaksLab_NPC_5', 'STAY', 'DOWN', 'LAND', 'TEXT_OAKSLAB_OAK1'),
			(108, 40, 5, 10, 5, 10, 'npc', 'SPRITE_OAK', 'OaksLab_NPC_8', 'STAY', 'UP', 'LAND', 'TEXT_OAKSLAB_OAK2');
		INSERT INTO phaser_tiles (map_id, x, y, collision_type, raw_foot_tile_id)
		VALUES (40, 4, 3, 1, NULL), (40, 5, 2, 1, NULL), (40, 5, 10, 1, NULL);
		INSERT INTO phaser_event_object_visibility (map_id, object_name, visible, label)
		VALUES
			(40, 'OaksLab_NPC_1', 1, 'HS_OAKS_LAB_RIVAL'),
			(40, 'OaksLab_NPC_5', 0, 'HS_OAKS_LAB_OAK_1'),
			(40, 'OaksLab_NPC_8', 0, 'HS_OAKS_LAB_OAK_2');
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
