package world

import (
	"encoding/json"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/content"
	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func TestContentDetailsUseTypedFlatWireResponses(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.Content = content.New(database)
	db.GlobalWorldDB = nil
	testdb.Exec(t, database, `UPDATE phaser_pokemon SET default_move_1_id='SPLASH' WHERE id=25;
  INSERT INTO phaser_items(id,name,short_name) VALUES(1,'POTION','POTION')`)
	cases := []struct {
		request, response opcodes.OpCode
		payload           string
		values            map[string]any
		absent            []string
	}{
		{opcodes.PhaserPokemonDataRequest, opcodes.PhaserPokemonDataResponse, `{"pokemonId":25}`, map[string]any{"success": true, "id": float64(25), "hp": float64(35), "defaultMove1Id": "SPLASH", "defaultMove2Id": nil, "type2": nil}, []string{"hP", "defaultMove1", "PhaserPokemonFull"}},
		{opcodes.PhaserMoveDataRequest, opcodes.PhaserMoveDataResponse, `{"moveId":150}`, map[string]any{"success": true, "id": float64(150), "pp": float64(40), "isHm": float64(0), "battleAnimation": nil}, []string{"pP", "isHM", "PhaserMoveFull"}},
		{opcodes.PhaserItemDataRequest, opcodes.PhaserItemDataResponse, `{"itemId":1}`, map[string]any{"success": true, "id": float64(1), "price": nil, "moveId": nil}, []string{"PhaserItemFull"}},
	}
	for _, tc := range cases {
		battleDispatch(t, wh, ses, tc.request, tc.payload)
		message := messages.streams[len(messages.streams)-1]
		if message.opcode != tc.response {
			t.Fatalf("opcode=%d want %d", message.opcode, tc.response)
		}
		var value map[string]any
		if err := json.Unmarshal(message.payload, &value); err != nil {
			t.Fatal(err)
		}
		for key, want := range tc.values {
			got, present := value[key]
			if !present || got != want {
				t.Errorf("field %s=%v present=%t want=%v", key, got, present, want)
			}
		}
		for _, key := range tc.absent {
			if _, present := value[key]; present {
				t.Errorf("legacy/wrapped key %s present", key)
			}
		}
	}
	// Missing records, invalid field types, and database failures have the same
	// explicit failure shape, but read failures must not be called missing records.
	battleDispatch(t, wh, ses, opcodes.PhaserPokemonDataRequest, `{"pokemonId":999}`)
	assertQueryWireFailure(t, messages.streams[len(messages.streams)-1], opcodes.PhaserPokemonDataResponse)
	battleDispatch(t, wh, ses, opcodes.PhaserMoveDataRequest, `{"moveId":"invalid"}`)
	assertQueryWireFailure(t, messages.streams[len(messages.streams)-1], opcodes.PhaserMoveDataResponse)
	testdb.Exec(t, database, `ALTER TABLE phaser_items RENAME TO unavailable_items`)
	battleDispatch(t, wh, ses, opcodes.PhaserItemDataRequest, `{"itemId":1}`)
	message := messages.streams[len(messages.streams)-1]
	assertQueryWireFailure(t, message, opcodes.PhaserItemDataResponse)
	var failure map[string]any
	if err := json.Unmarshal(message.payload, &failure); err != nil {
		t.Fatal(err)
	}
	if failure["error"] != "failed to load item data" {
		t.Fatalf("database failure=%v", failure)
	}
}
