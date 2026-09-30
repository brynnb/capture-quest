package world

import (
	"encoding/json"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/content"
	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func TestContentAggregateWireShapesAndLateFailures(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.Content = content.New(database)
	db.GlobalWorldDB = nil
	testdb.Exec(t, database, `INSERT INTO phaser_map_scripts(map_name,script_index,script_label,script_constant) VALUES('TEST',0,'TEST_SCRIPT','CONST');
  INSERT INTO phaser_event_flags(map_name,flag_name,operation) VALUES('TEST','FLAG','set');
  INSERT INTO phaser_coordinate_triggers(map_name,label,x,y) VALUES('TEST','COORD',3,4);
  INSERT INTO phaser_npc_movement_data(map_name,label,movements) VALUES('TEST','NPC','["UP"]');
  INSERT INTO phaser_pokemon_learnset(pokemon_id,pokemon_name,level,move_name) VALUES(25,'PIKACHU',5,'GROWL');
  INSERT INTO phaser_pokemon_tmhm(pokemon_id,pokemon_name,tm_hm_name,move_name,is_hm) VALUES(25,'PIKACHU','HM01','CUT',1)`)
	battleDispatch(t, wh, ses, opcodes.PhaserMapScriptsRequest, `{"mapName":"TEST"}`)
	var result map[string]any
	if err := json.Unmarshal(messages.streams[0].payload, &result); err != nil {
		t.Fatal(err)
	}
	if result["success"] != true || result["mapName"] != "TEST" {
		t.Fatalf("map=%v", result)
	}
	script := result["scripts"].([]any)[0].(map[string]any)
	if raw, present := script["rawAsm"]; !present || raw != nil {
		t.Fatalf("rawAsm=%v", script)
	}
	if _, present := script["rawASM"]; present {
		t.Fatal("legacy ASM key leaked")
	}
	if result["npcMovements"].([]any)[0].(map[string]any)["movements"] != `["UP"]` {
		t.Fatalf("movement source value=%v", result)
	}
	battleDispatch(t, wh, ses, opcodes.PhaserLearnsetRequest, `{"pokemonId":25}`)
	if err := json.Unmarshal(messages.streams[1].payload, &result); err != nil {
		t.Fatal(err)
	}
	tmhm := result["tmhm"].([]any)[0].(map[string]any)
	if tmhm["tmHmName"] != "HM01" || tmhm["isHm"] != float64(1) {
		t.Fatalf("TM/HM=%v", tmhm)
	}
	if move, present := tmhm["moveId"]; !present || move != nil {
		t.Fatalf("nullable move=%v", tmhm)
	}
	for _, key := range []string{"tMHMName", "isHM"} {
		if _, present := tmhm[key]; present {
			t.Fatalf("legacy key %s leaked", key)
		}
	}
	battleDispatch(t, wh, ses, opcodes.PhaserMapScriptsRequest, `{"mapName":"EMPTY"}`)
	if err := json.Unmarshal(messages.streams[2].payload, &result); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"scripts", "eventFlags", "coordinateTriggers", "npcMovements"} {
		if values, ok := result[key].([]any); !ok || len(values) != 0 {
			t.Fatalf("empty %s=%v", key, result[key])
		}
	}
	testdb.Exec(t, database, `ALTER TABLE phaser_npc_movement_data RENAME TO missing_npc_movements;
  ALTER TABLE phaser_pokemon_tmhm RENAME TO missing_tmhm`)
	battleDispatch(t, wh, ses, opcodes.PhaserMapScriptsRequest, `{"mapName":"TEST"}`)
	assertQueryWireFailure(t, messages.streams[3], opcodes.PhaserMapScriptsResponse)
	battleDispatch(t, wh, ses, opcodes.PhaserLearnsetRequest, `{"pokemonId":25}`)
	assertQueryWireFailure(t, messages.streams[4], opcodes.PhaserLearnsetResponse)
}
