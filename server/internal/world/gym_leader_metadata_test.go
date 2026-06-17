package world

import (
	"database/sql"
	"testing"

	"capturequest/internal/db"
	"capturequest/internal/pokebattle"

	_ "modernc.org/sqlite"
)

func TestTrainerDataForObjectIDAppliesGymLeaderMetadata(t *testing.T) {
	setupGymLeaderMetadataDB(t)

	trainer, err := trainerDataForObjectID(976)
	if err != nil {
		t.Fatal(err)
	}
	if !trainer.IsGymLeader {
		t.Fatal("expected Brock to be marked as a gym leader")
	}
	if trainer.EventFlag != "EVENT_BEAT_BROCK" {
		t.Fatalf("EventFlag = %q, want EVENT_BEAT_BROCK", trainer.EventFlag)
	}
	if trainer.BattleTextLabel != "_PewterGymBrockPreBattleText" {
		t.Fatalf("BattleTextLabel = %q", trainer.BattleTextLabel)
	}
	if trainer.AfterBattleTextLabel != "_PewterGymBrockPostBattleAdviceText" {
		t.Fatalf("AfterBattleTextLabel = %q", trainer.AfterBattleTextLabel)
	}
}

func TestTrainerDataForObjectIDMatchesHeaderByNormalizedMapName(t *testing.T) {
	setupMtMoonTrainerAliasDB(t)

	trainer, err := trainerDataForObjectID(366)
	if err != nil {
		t.Fatal(err)
	}
	if trainer.Name != "MtMoon1F_NPC_2" {
		t.Fatalf("Name = %q, want MtMoon1F_NPC_2", trainer.Name)
	}
	if trainer.EventFlag != "EVENT_BEAT_MT_MOON_1_TRAINER_1" {
		t.Fatalf("EventFlag = %q, want EVENT_BEAT_MT_MOON_1_TRAINER_1", trainer.EventFlag)
	}
	if trainer.SightRange != 3 {
		t.Fatalf("SightRange = %d, want 3", trainer.SightRange)
	}
	if trainer.BattleTextLabel != "_MtMoon1FYoungster1BattleText" {
		t.Fatalf("BattleTextLabel = %q", trainer.BattleTextLabel)
	}
	if trainer.AfterBattleTextLabel != "_MtMoon1FYoungster1AfterBattleText" {
		t.Fatalf("AfterBattleTextLabel = %q", trainer.AfterBattleTextLabel)
	}
}

func TestTrainerDataForObjectIDLoadsPewterGymJrTrainerDialogueLabels(t *testing.T) {
	setupPewterGymJrTrainerDB(t)

	trainer, err := trainerDataForObjectID(448)
	if err != nil {
		t.Fatal(err)
	}
	if trainer.Name != "PewterGym_NPC_2" {
		t.Fatalf("Name = %q, want PewterGym_NPC_2", trainer.Name)
	}
	if trainer.EventFlag != "EVENT_BEAT_PEWTER_GYM_TRAINER_0" {
		t.Fatalf("EventFlag = %q, want EVENT_BEAT_PEWTER_GYM_TRAINER_0", trainer.EventFlag)
	}
	if trainer.BattleTextLabel != "_PewterGymCooltrainerMBattleText" {
		t.Fatalf("BattleTextLabel = %q", trainer.BattleTextLabel)
	}
	if trainer.AfterBattleTextLabel != "_PewterGymCooltrainerMAfterBattleText" {
		t.Fatalf("AfterBattleTextLabel = %q", trainer.AfterBattleTextLabel)
	}

	dialogue, err := trainerDialogueByLabel(trainer.AfterBattleTextLabel)
	if err != nil {
		t.Fatal(err)
	}
	if dialogue != "You're pretty hot,\nbut not as hot as BROCK!" {
		t.Fatalf("after-battle dialogue = %q", dialogue)
	}
}

func TestTrainerBattleSuppressedByGymLeaderDefeat(t *testing.T) {
	charID := int64(42)
	efm := NewEventFlagManager(nil)
	efm.flags[charID] = map[string]bool{
		"EVENT_BEAT_BROCK":                 true,
		"EVENT_BEAT_VIRIDIAN_GYM_GIOVANNI": true,
	}
	wh := &WorldHandler{EventFlags: efm}

	if !trainerBattleSuppressedByGymLeaderDefeat(charID, &trainerSightData{
		MapID:        54,
		TrainerClass: "JR_TRAINER_M",
	}, wh) {
		t.Fatal("Pewter gym trainer should be suppressed after Brock is defeated")
	}

	if trainerBattleSuppressedByGymLeaderDefeat(charID, &trainerSightData{
		MapID:        54,
		TrainerClass: "BROCK",
		IsGymLeader:  true,
	}, wh) {
		t.Fatal("gym leader should not be suppressed by the non-leader helper")
	}

	if trainerBattleSuppressedByGymLeaderDefeat(charID, &trainerSightData{
		MapID:        59,
		TrainerClass: "YOUNGSTER",
	}, wh) {
		t.Fatal("non-gym trainer should not be suppressed by Brock's flag")
	}

	if !trainerBattleSuppressedByGymLeaderDefeat(charID, &trainerSightData{
		MapID:        45,
		TrainerClass: "COOLTRAINER_M",
	}, wh) {
		t.Fatal("Viridian gym trainer should be suppressed after Giovanni is defeated")
	}
}

func TestTrainerBattleSuppressedByLegacyGiovanniFlag(t *testing.T) {
	charID := int64(42)
	efm := NewEventFlagManager(nil)
	efm.flags[charID] = map[string]bool{"EVENT_BEAT_GIOVANNI_GYM": true}
	wh := &WorldHandler{EventFlags: efm}

	if !trainerBattleSuppressedByGymLeaderDefeat(charID, &trainerSightData{
		MapID:        45,
		TrainerClass: "COOLTRAINER_M",
	}, wh) {
		t.Fatal("Viridian gym trainer should be suppressed by the legacy Giovanni gym flag")
	}
}

func TestTrainerDataForObjectIDRejectsScriptedStoryTrainerWithoutHeader(t *testing.T) {
	setupScriptedStoryTrainerWithoutHeaderDB(t)

	if trainer, err := trainerDataForObjectID(20); err == nil {
		t.Fatalf("trainerDataForObjectID returned %#v, want no generic trainer for scripted story object", trainer)
	}
}

func TestTrainerEncounterLoadMatchesHeaderByNormalizedMapName(t *testing.T) {
	setupMtMoonTrainerAliasDB(t)

	wh := &WorldHandler{
		ActorRegistry: NewActorRegistry(),
		ActorManager:  &PhaserActorManager{overworldMapIds: map[int]bool{}},
	}
	mgr := NewTrainerEncounterManager(wh)
	mgr.Load()

	trainers := mgr.byMap[59]
	if len(trainers) != 1 {
		t.Fatalf("loaded %d trainers for MT_MOON_1F, want 1", len(trainers))
	}
	trainer := trainers[0]
	if trainer.ObjectID != 366 {
		t.Fatalf("ObjectID = %d, want 366", trainer.ObjectID)
	}
	if trainer.SightRange != 3 {
		t.Fatalf("SightRange = %d, want 3", trainer.SightRange)
	}
	if trainer.EventFlag != "EVENT_BEAT_MT_MOON_1_TRAINER_1" {
		t.Fatalf("EventFlag = %q, want EVENT_BEAT_MT_MOON_1_TRAINER_1", trainer.EventFlag)
	}
	if trainer.RuntimeActorID != wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 366) {
		t.Fatalf("RuntimeActorID = %d, want registry mapping", trainer.RuntimeActorID)
	}
}

func setupPewterGymJrTrainerDB(t *testing.T) {
	t.Helper()

	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		CREATE TABLE phaser_maps (
			id INTEGER PRIMARY KEY,
			name TEXT
		);
		CREATE TABLE phaser_objects (
			id INTEGER PRIMARY KEY,
			map_id INTEGER NOT NULL,
			x INTEGER,
			y INTEGER,
			local_x INTEGER,
			local_y INTEGER,
			action_direction TEXT,
			trainer_class TEXT,
			trainer_party_index INTEGER,
			name TEXT,
			text TEXT
		);
		CREATE TABLE phaser_text_pointers (
			map_name TEXT,
			text_constant TEXT,
			pointer_index INTEGER,
			is_trainer INTEGER
		);
		CREATE TABLE phaser_trainer_headers (
			map_name TEXT,
			map_id INTEGER,
			header_index INTEGER,
			event_flag TEXT,
			sight_range INTEGER,
			battle_text_label TEXT,
			end_battle_text_label TEXT,
			after_battle_text_label TEXT
		);
		CREATE TABLE phaser_trainer_classes (
			constant_name TEXT PRIMARY KEY,
			is_gym_leader INTEGER
		);
		CREATE TABLE phaser_dialogue_text (
			label TEXT PRIMARY KEY,
			dialogue TEXT
		);
		INSERT INTO phaser_maps (id, name)
		VALUES (54, 'PEWTER_GYM');
		INSERT INTO phaser_objects (
			id, map_id, local_x, local_y, action_direction, trainer_class,
			trainer_party_index, name, text
		) VALUES (
			448, 54, 3, 6, 'RIGHT', 'JR_TRAINER_M', 1,
			'PewterGym_NPC_2', 'TEXT_PEWTERGYM_COOLTRAINER_M'
		);
		INSERT INTO phaser_text_pointers (map_name, text_constant, pointer_index, is_trainer)
		VALUES ('PewterGym', 'TEXT_PEWTERGYM_COOLTRAINER_M', 2, 1);
		INSERT INTO phaser_trainer_headers (
			map_name, map_id, header_index, event_flag, sight_range,
			battle_text_label, end_battle_text_label, after_battle_text_label
		) VALUES (
			'PewterGym', 54, 0, 'EVENT_BEAT_PEWTER_GYM_TRAINER_0', 5,
			'_PewterGymCooltrainerMBattleText',
			'_PewterGymCooltrainerMEndBattleText',
			'_PewterGymCooltrainerMAfterBattleText'
		);
		INSERT INTO phaser_trainer_classes (constant_name, is_gym_leader)
		VALUES ('JR_TRAINER_M', 0);
		INSERT INTO phaser_dialogue_text (label, dialogue)
		VALUES (
			'_PewterGymCooltrainerMAfterBattleText',
			'You''re pretty hot,
but not as hot as BROCK!'
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
}

func setupScriptedStoryTrainerWithoutHeaderDB(t *testing.T) {
	t.Helper()

	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		CREATE TABLE phaser_maps (
			id INTEGER PRIMARY KEY,
			name TEXT
		);
		CREATE TABLE phaser_objects (
			id INTEGER PRIMARY KEY,
			map_id INTEGER NOT NULL,
			x INTEGER,
			y INTEGER,
			local_x INTEGER,
			local_y INTEGER,
			action_direction TEXT,
			trainer_class TEXT,
			trainer_party_index INTEGER,
			name TEXT,
			text TEXT
		);
		CREATE TABLE phaser_text_pointers (
			map_name TEXT,
			text_constant TEXT,
			pointer_index INTEGER,
			is_trainer INTEGER
		);
		CREATE TABLE phaser_trainer_headers (
			map_name TEXT,
			map_id INTEGER,
			header_index INTEGER,
			event_flag TEXT,
			sight_range INTEGER,
			battle_text_label TEXT,
			end_battle_text_label TEXT,
			after_battle_text_label TEXT
		);
		CREATE TABLE phaser_trainer_classes (
			constant_name TEXT PRIMARY KEY,
			is_gym_leader INTEGER
		);
		INSERT INTO phaser_maps (id, name)
		VALUES (40, 'OAKS_LAB');
		INSERT INTO phaser_objects (
			id, map_id, local_x, local_y, action_direction, trainer_class,
			trainer_party_index, name, text
		) VALUES (
			20, 40, 4, 3, 'DOWN', 'RIVAL1', 1,
			'OaksLab_NPC_1', 'TEXT_OAKSLAB_RIVAL'
		);
		INSERT INTO phaser_text_pointers (map_name, text_constant, pointer_index, is_trainer)
		VALUES ('OaksLab', 'TEXT_OAKSLAB_RIVAL', 1, 0);
		INSERT INTO phaser_trainer_classes (constant_name, is_gym_leader)
		VALUES ('RIVAL1', 0);
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

func TestBattleShouldSendPostBattleMapScript(t *testing.T) {
	wonTrainer := &pokebattle.BattleState{
		Phase:      pokebattle.PhaseBattleEnd,
		BattleType: pokebattle.BattleTrainer,
		EnemyParty: []*pokebattle.Pokemon{{CurHP: 0}},
		Trainer:    &pokebattle.TrainerMeta{ClassName: "BROCK", WinFlag: "EVENT_BEAT_BROCK"},
	}
	if !battleShouldSendPostBattleMapScript(wonTrainer) {
		t.Fatal("won trainer battle should trigger post-battle map-script lookup")
	}

	wonTrainerWithoutFlag := &pokebattle.BattleState{
		Phase:      pokebattle.PhaseBattleEnd,
		BattleType: pokebattle.BattleTrainer,
		EnemyParty: []*pokebattle.Pokemon{{CurHP: 0}},
		Trainer:    &pokebattle.TrainerMeta{ClassName: "BUG_CATCHER"},
	}
	if battleShouldSendPostBattleMapScript(wonTrainerWithoutFlag) {
		t.Fatal("won trainer battle without a win flag should not search for flag-gated map scripts")
	}

	wonWild := &pokebattle.BattleState{
		Phase:      pokebattle.PhaseBattleEnd,
		BattleType: pokebattle.BattleWild,
		EnemyParty: []*pokebattle.Pokemon{{CurHP: 0}},
	}
	if battleShouldSendPostBattleMapScript(wonWild) {
		t.Fatal("won wild battle should not trigger trainer post-battle map scripts")
	}

	lostTrainer := &pokebattle.BattleState{
		Phase:      pokebattle.PhaseBattleEnd,
		BattleType: pokebattle.BattleTrainer,
		EnemyParty: []*pokebattle.Pokemon{{CurHP: 1}},
		Trainer:    &pokebattle.TrainerMeta{ClassName: "BROCK"},
	}
	if battleShouldSendPostBattleMapScript(lostTrainer) {
		t.Fatal("lost trainer battle should not trigger post-battle reward map scripts")
	}

	lostNoBlackoutScriptedTrainer := &pokebattle.BattleState{
		Phase:       pokebattle.PhaseBattleEnd,
		BattleType:  pokebattle.BattleTrainer,
		PlayerParty: []*pokebattle.Pokemon{{CurHP: 0}},
		EnemyParty:  []*pokebattle.Pokemon{{CurHP: 1}},
		Trainer: &pokebattle.TrainerMeta{
			ClassName:        "RIVAL1",
			LoseFlag:         "EVENT_BATTLED_RIVAL_IN_OAKS_LAB",
			NoBlackoutOnLoss: true,
		},
	}
	if !battleShouldSendPostBattleMapScript(lostNoBlackoutScriptedTrainer) {
		t.Fatal("scripted no-blackout trainer loss should trigger post-battle map-script lookup")
	}
}

func setupGymLeaderMetadataDB(t *testing.T) {
	t.Helper()

	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		CREATE TABLE phaser_maps (
			id INTEGER PRIMARY KEY,
			name TEXT
		);
		CREATE TABLE phaser_objects (
			id INTEGER PRIMARY KEY,
			map_id INTEGER NOT NULL,
			x INTEGER,
			y INTEGER,
			local_x INTEGER,
			local_y INTEGER,
			action_direction TEXT,
			trainer_class TEXT,
			trainer_party_index INTEGER,
			name TEXT,
			text TEXT
		);
		CREATE TABLE phaser_text_pointers (
			map_name TEXT,
			text_constant TEXT,
			pointer_index INTEGER,
			is_trainer INTEGER
		);
		CREATE TABLE phaser_trainer_headers (
			map_name TEXT,
			map_id INTEGER,
			header_index INTEGER,
			event_flag TEXT,
			sight_range INTEGER,
			battle_text_label TEXT,
			end_battle_text_label TEXT,
			after_battle_text_label TEXT
		);
		CREATE TABLE phaser_trainer_classes (
			constant_name TEXT PRIMARY KEY,
			is_gym_leader INTEGER
		);
		INSERT INTO phaser_maps (id, name)
		VALUES (54, 'PEWTER_GYM');
		INSERT INTO phaser_objects (
			id, map_id, local_x, local_y, action_direction, trainer_class,
			trainer_party_index, name, text
		) VALUES (
			976, 54, 4, 1, 'DOWN', 'BROCK', 1, 'PewterGym_NPC_1', 'TEXT_PEWTERGYM_BROCK'
		);
		INSERT INTO phaser_text_pointers (map_name, text_constant, pointer_index, is_trainer)
		VALUES ('PewterGym', 'TEXT_PEWTERGYM_BROCK', 1, 0);
		INSERT INTO phaser_trainer_classes (constant_name, is_gym_leader)
		VALUES ('BROCK', 1);
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

func setupMtMoonTrainerAliasDB(t *testing.T) {
	t.Helper()

	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		CREATE TABLE phaser_maps (
			id INTEGER PRIMARY KEY,
			name TEXT
		);
		CREATE TABLE phaser_objects (
			id INTEGER PRIMARY KEY,
			map_id INTEGER NOT NULL,
			x INTEGER,
			y INTEGER,
			local_x INTEGER,
			local_y INTEGER,
			action_direction TEXT,
			trainer_class TEXT,
			trainer_party_index INTEGER,
			name TEXT,
			text TEXT
		);
		CREATE TABLE phaser_text_pointers (
			map_name TEXT,
			text_constant TEXT,
			pointer_index INTEGER,
			is_trainer INTEGER
		);
		CREATE TABLE phaser_trainer_headers (
			map_name TEXT,
			map_id INTEGER,
			header_index INTEGER,
			event_flag TEXT,
			sight_range INTEGER,
			battle_text_label TEXT,
			end_battle_text_label TEXT,
			after_battle_text_label TEXT
		);
		CREATE TABLE phaser_trainer_classes (
			constant_name TEXT PRIMARY KEY,
			is_gym_leader INTEGER
		);
		INSERT INTO phaser_maps (id, name)
		VALUES (59, 'MT_MOON_1F');
		INSERT INTO phaser_objects (
			id, map_id, local_x, local_y, action_direction, trainer_class,
			trainer_party_index, name, text
		) VALUES (
			366, 59, 12, 16, 'RIGHT', 'YOUNGSTER', 3,
			'MtMoon1F_NPC_2', 'TEXT_MTMOON1F_YOUNGSTER1'
		);
		INSERT INTO phaser_text_pointers (map_name, text_constant, pointer_index, is_trainer)
		VALUES
			('MtMoon1F', 'TEXT_MTMOON1F_HIKER', 1, 1),
			('MtMoon1F', 'TEXT_MTMOON1F_YOUNGSTER1', 2, 1);
		INSERT INTO phaser_trainer_headers (
			map_name, map_id, header_index, event_flag, sight_range,
			battle_text_label, end_battle_text_label, after_battle_text_label
		) VALUES (
			'MtMoon1F', NULL, 1, 'EVENT_BEAT_MT_MOON_1_TRAINER_1', 3,
			'_MtMoon1FYoungster1BattleText',
			'_MtMoon1FYoungster1EndBattleText',
			'_MtMoon1FYoungster1AfterBattleText'
		);
		INSERT INTO phaser_trainer_classes (constant_name, is_gym_leader)
		VALUES ('YOUNGSTER', 0);
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
