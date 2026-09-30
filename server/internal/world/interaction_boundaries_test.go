package world

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func TestDialogueChoiceBindsActorPromptMapAndDurablePrerequisite(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.Cutscenes = NewCutsceneManager(database)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(1,'ROOM',10,10),(2,'OTHER',10,10);
		INSERT INTO phaser_objects(id,map_id,x,y,object_type,text) VALUES(10,1,1,0,'npc','PROMPT');
		INSERT INTO phaser_branching_dialogue(prompt_text_constant,prompt_text,map_name,yes_dialogue,no_dialogue,requires_event_flag,yes_actions)
		VALUES('PROMPT','Buy?','ROOM','Yes','No','ALLOWED','[{"type":"takeMoney","money":5}]'),
		('UNRELATED','Other?','ROOM','Yes','No',NULL,'[{"type":"takeMoney","money":80}]')`)
	actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	db.GlobalWorldDB = nil
	ses.Client.CharData().MapID = 1
	request := func(id int, prompt string, choice, want bool) {
		t.Helper()
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.DialogueChoiceRequest, fmt.Sprintf(`{"actorId":%d,"textConstant":%q,"choice":%v}`, id, prompt, choice))
		var response map[string]any
		found := false
		for _, message := range messages.streams {
			if message.opcode == opcodes.DialogueChoiceResponse {
				found = true
				if err := json.Unmarshal(message.payload, &response); err != nil {
					t.Fatal(err)
				}
			}
		}
		if !found || response["success"] != want {
			t.Fatalf("choice response=%v want success=%v", response, want)
		}
	}
	request(10, "PROMPT", true, false) // Persistent ID is not a runtime actor ID.
	request(actorID, "UNRELATED", true, false)
	ses.Client.CharData().X = 8
	request(actorID, "PROMPT", true, false)
	ses.Client.CharData().X = 0
	ses.Client.CharData().MapID = 2
	request(actorID, "PROMPT", true, false)
	ses.Client.CharData().MapID = 1
	wh.EventFlags.flags[42] = map[string]bool{"ALLOWED": true} // Stale cache cannot grant a durable reward.
	request(actorID, "PROMPT", true, false)
	var money int
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 100 {
		t.Fatalf("unauthorized wallet=%d err=%v", money, err)
	}
	testdb.Exec(t, database, `INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'ALLOWED')`)
	request(actorID, "PROMPT", false, true)
	request(actorID, "PROMPT", true, true)
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 95 {
		t.Fatalf("authorized wallet=%d err=%v", money, err)
	}
	testdb.Exec(t, database, `UPDATE phaser_branching_dialogue SET map_name='OTHER' WHERE prompt_text_constant='PROMPT'`)
	request(actorID, "PROMPT", true, false)
	testdb.Exec(t, database, `UPDATE phaser_branching_dialogue SET map_name='ROOM' WHERE prompt_text_constant='PROMPT'`)
	wh.Cutscenes.byLabel["PROMPT"] = &CutsceneScript{ScriptLabel: "PROMPT", TriggerType: "npc_click"}
	request(actorID, "PROMPT", true, false) // Cannot bypass issued cutscene completion.
	delete(wh.Cutscenes.byLabel, "PROMPT")
	testdb.Exec(t, database, `UPDATE phaser_branching_dialogue SET yes_dialogue=NULL,yes_text_constant='MISSING' WHERE prompt_text_constant='PROMPT'`)
	request(actorID, "PROMPT", true, false) // Missing follow-up cannot publish an empty success.
}

func TestTradeChoiceAuthorizationRollbackRetryAndDuplicate(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(1,'ROOM',10,10),(2,'OTHER',10,10);
		INSERT INTO phaser_objects(id,map_id,x,y,object_type,text) VALUES(10,1,1,0,'npc','TRADE_PROMPT');
		INSERT INTO phaser_in_game_trades(trade_key,text_constant,map_name,source_file,script_label,requested_pokemon_id,requested_pokemon_name,offered_pokemon_id,offered_pokemon_name,offered_nickname,dialogue_set)
		VALUES('TRADE','TRADE_PROMPT','Room','test','TRADE_SCRIPT',25,'PIKACHU',129,'MAGIKARP','FISH','CASUAL')`)
	db.GlobalWorldDB = nil
	ses.Client.CharData().MapID = 2
	request := func(want bool) {
		t.Helper()
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.DialogueChoiceRequest, fmt.Sprintf(`{"actorId":%d,"textConstant":"TRADE_PROMPT","choice":true}`, actorID))
		var response map[string]any
		if len(messages.streams) == 0 || messages.streams[0].opcode != opcodes.DialogueChoiceResponse {
			t.Fatal("trade choice response missing")
		}
		if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil {
			t.Fatal(err)
		}
		if response["success"] != want {
			t.Fatalf("trade response=%v want=%v", response, want)
		}
		if !want {
			for _, message := range messages.streams {
				if message.opcode == opcodes.PokemonPartyResponse {
					t.Fatal("failed trade published party")
				}
			}
		}
	}
	var originalRow int64
	if err := database.QueryRow(`SELECT id FROM character_pokemon WHERE character_id=42`).Scan(&originalRow); err != nil {
		t.Fatal(err)
	}
	unchanged := func() {
		t.Helper()
		var row int64
		var species, trades int
		if err := database.QueryRow(`SELECT id,pokemon_id FROM character_pokemon WHERE character_id=42`).Scan(&row, &species); err != nil || row != originalRow || species != 25 {
			t.Fatalf("party row=%d species=%d err=%v", row, species, err)
		}
		if err := database.QueryRow(`SELECT COUNT(*) FROM character_in_game_trades`).Scan(&trades); err != nil || trades != 0 {
			t.Fatalf("trade count=%d err=%v", trades, err)
		}
	}
	request(false)
	unchanged()
	ses.Client.CharData().MapID = 1
	testdb.Exec(t, database, `ALTER TABLE character_in_game_trades ADD CONSTRAINT reject_trade CHECK(trade_key <> 'TRADE')`)
	request(false)
	unchanged() // Late failure restores consumed Pokémon and row identity.
	testdb.Exec(t, database, `ALTER TABLE character_in_game_trades DROP CONSTRAINT reject_trade`)
	trade, err := queryInGameTradeDefinitionByTextContext(context.Background(), database, "TRADE_PROMPT")
	if err != nil {
		t.Fatal(err)
	}
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`UPDATE character_data SET name=name WHERE id=42`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := performInGameTradeContext(ctx, database, 42, trade); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancelled trade=%v", err)
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	unchanged()
	request(true)
	var receivedRow int64
	var species int
	if err := database.QueryRow(`SELECT id,pokemon_id FROM character_pokemon WHERE character_id=42`).Scan(&receivedRow, &species); err != nil || species != 129 {
		t.Fatalf("received species=%d err=%v", species, err)
	}
	if len(messages.streams) != 2 || messages.streams[1].opcode != opcodes.PokemonPartyResponse {
		t.Fatal("committed party snapshot missing")
	}
	request(true)
	var duplicateRow int64
	if err := database.QueryRow(`SELECT id FROM character_pokemon WHERE character_id=42`).Scan(&duplicateRow); err != nil || duplicateRow != receivedRow || len(messages.streams) != 1 {
		t.Fatalf("duplicate trade row=%d wanted=%d err=%v messages=%d", duplicateRow, receivedRow, err, len(messages.streams))
	}
}

func TestTrainerClickAndBattleStartRequireReachableVisibleActor(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(1,'ROOM',10,10),(2,'OTHER',10,10);
		INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,text,trainer_class,trainer_party_index) VALUES(10,1,1,0,'npc','Trainer','TRAINER','YOUNGSTER',1);
		INSERT INTO phaser_text_pointers(map_name,text_constant,local_label,pointer_index,is_trainer) VALUES('ROOM','TRAINER','TRAINER',1,1);
		INSERT INTO phaser_trainer_headers(map_id,map_name,header_label,header_index,event_flag,battle_text_label) VALUES(1,'ROOM','HEADER',0,'WIN','PRE');
		INSERT INTO phaser_dialogue_text(label,source_file,dialogue) VALUES('PRE','test','Battle!');
		INSERT INTO phaser_trainer_classes(id,constant_name,display_name) VALUES(1,'YOUNGSTER','Youngster');
		INSERT INTO phaser_trainer_parties(id,trainer_class_id,party_index) VALUES(1,1,1);
		INSERT INTO phaser_trainer_party_pokemon(trainer_party_id,slot_index,pokemon_name,level) VALUES(1,0,'MAGIKARP',5)`)
	db.GlobalWorldDB = nil
	actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	ses.Client.CharData().MapID = 1
	request := func(opcode opcodes.OpCode, want bool) {
		t.Helper()
		messages.streams = nil
		battleDispatch(t, wh, ses, opcode, fmt.Sprintf(`{"actorId":%d,"trainerActorId":%d}`, actorID, actorID))
		if len(messages.streams) != 1 {
			t.Fatalf("trainer responses=%d", len(messages.streams))
		}
		var response map[string]any
		if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil {
			t.Fatal(err)
		}
		if response["success"] != want {
			t.Fatalf("trainer response=%v want=%v", response, want)
		}
	}
	ses.Client.CharData().MapID = 2
	request(opcodes.TrainerInteractRequest, false)
	request(opcodes.TrainerBattleStartRequest, false)
	ses.Client.CharData().MapID = 1
	ses.Client.CharData().X = 8
	request(opcodes.TrainerBattleStartRequest, false)
	ses.Client.CharData().X = 0
	testdb.Exec(t, database, `INSERT INTO character_object_visibility_overrides(character_id,object_id,visible,source) VALUES(42,10,false,'test')`)
	request(opcodes.TrainerBattleStartRequest, false)
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_battle_state`).Scan(&count); err != nil || count != 0 || getBattle(42) != nil {
		t.Fatalf("unauthorized battle count=%d err=%v", count, err)
	}
	testdb.Exec(t, database, `DELETE FROM character_object_visibility_overrides`)
	request(opcodes.TrainerInteractRequest, true)
	// The battle-start request must recheck reach after the dialogue was shown.
	ses.Client.CharData().X = 8
	request(opcodes.TrainerBattleStartRequest, false)
	ses.Client.CharData().X = 0
	request(opcodes.TrainerBattleStartRequest, true)
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_battle_state`).Scan(&count); err != nil || count != 1 || getBattle(42) == nil {
		t.Fatalf("authorized battle count=%d err=%v", count, err)
	}
	current := getBattle(42)
	request(opcodes.TrainerBattleStartRequest, false)
	if getBattle(42) != current {
		t.Fatal("duplicate request replaced the active battle")
	}
}
