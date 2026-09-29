package world

import (
	"encoding/json"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func TestPokedexAndTrainerCardWireContracts(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	ses.Client.CharData().Name = "Red"
	testdb.Exec(t, database, `UPDATE phaser_pokemon SET base_cry=18, cry_pitch=0, cry_length=255 WHERE id=25`)
	// These query handlers must use their world's injected database.
	db.GlobalWorldDB = nil
	battleDispatch(t, wh, ses, opcodes.PokedexListRequest, `{}`)
	if len(messages.streams) != 1 {
		t.Fatalf("list messages=%d", len(messages.streams))
	}
	var list struct {
		Success bool             `json:"success"`
		Species []map[string]any `json:"species"`
		Status  []map[string]any `json:"status"`
	}
	if err := json.Unmarshal(messages.streams[0].payload, &list); err != nil {
		t.Fatal(err)
	}
	if messages.streams[0].opcode != opcodes.PokedexListResponse || !list.Success || len(list.Species) != 2 || len(list.Status) != 1 {
		t.Fatalf("list=%+v", list)
	}
	pikachu := list.Species[0]
	if pikachu["crySfx"] != "SFX_CRY_12" || pikachu["cryPitch"] != float64(0) || pikachu["cryLength"] != float64(255) {
		t.Fatalf("cry metadata=%v", pikachu)
	}
	if _, exists := pikachu["crySFX"]; exists {
		t.Fatal("legacy acronym field leaked")
	}
	if value, exists := pikachu["type2"]; !exists || value != nil {
		t.Fatalf("nullable type2=%v exists=%t", value, exists)
	}
	if _, exists := list.Species[1]["crySfx"]; exists {
		t.Fatal("missing cry was not omitted")
	}
	if list.Status[0]["pokemonId"] != float64(25) || list.Status[0]["caught"] != true {
		t.Fatalf("status=%v", list.Status)
	}

	battleDispatch(t, wh, ses, opcodes.TrainerCardRequest, `{}`)
	if len(messages.streams) != 2 || messages.streams[1].opcode != opcodes.TrainerCardResponse {
		t.Fatalf("card messages=%v", messages.streams)
	}
	var card map[string]any
	if err := json.Unmarshal(messages.streams[1].payload, &card); err != nil {
		t.Fatal(err)
	}
	if card["success"] != true || card["name"] != "Red" || card["money"] != float64(100) || card["pokedexCaught"] != float64(1) {
		t.Fatalf("card=%v", card)
	}
	if badges, ok := card["badges"].([]any); !ok || len(badges) != 0 {
		t.Fatalf("empty badges=%#v", card["badges"])
	}

	testdb.Exec(t, database, `DELETE FROM character_pokemon; DELETE FROM character_pokedex`)
	battleDispatch(t, wh, ses, opcodes.PokedexStatusRequest, `{}`)
	var status map[string]any
	if err := json.Unmarshal(messages.streams[2].payload, &status); err != nil {
		t.Fatal(err)
	}
	if entries, ok := status["status"].([]any); !ok || len(entries) != 0 {
		t.Fatalf("empty status=%#v", status["status"])
	}

	testdb.Exec(t, database, `ALTER TABLE phaser_pokemon RENAME TO unavailable_pokemon`)
	battleDispatch(t, wh, ses, opcodes.PokedexListRequest, `{}`)
	assertPokedexWireFailure(t, messages.streams[3], opcodes.PokedexListResponse)
	// A scan error after a valid species must not publish a shortened success list.
	testdb.Exec(t, database, `ALTER TABLE unavailable_pokemon RENAME TO phaser_pokemon;
  ALTER TABLE phaser_pokemon ALTER COLUMN name DROP NOT NULL;
  UPDATE phaser_pokemon SET name=NULL WHERE id=129`)
	battleDispatch(t, wh, ses, opcodes.PokedexListRequest, `{}`)
	assertPokedexWireFailure(t, messages.streams[4], opcodes.PokedexListResponse)
	testdb.Exec(t, database, `UPDATE phaser_pokemon SET name='MAGIKARP' WHERE id=129;
  ALTER TABLE character_wallet RENAME TO unavailable_wallet`)
	battleDispatch(t, wh, ses, opcodes.TrainerCardRequest, `{}`)
	assertPokedexWireFailure(t, messages.streams[5], opcodes.TrainerCardResponse)
	// Reconciliation failure must not masquerade as an empty status snapshot.
	testdb.Exec(t, database, `DROP TABLE character_pokedex`)
	battleDispatch(t, wh, ses, opcodes.PokedexStatusRequest, `{}`)
	assertPokedexWireFailure(t, messages.streams[6], opcodes.PokedexStatusResponse)
}

func assertPokedexWireFailure(t *testing.T, message recordedStreamMessage, opcode opcodes.OpCode) {
	t.Helper()
	var failure map[string]any
	if err := json.Unmarshal(message.payload, &failure); err != nil {
		t.Fatal(err)
	}
	explanation, ok := failure["error"].(string)
	if message.opcode != opcode || failure["success"] != false || !ok || explanation == "" || len(failure) != 2 {
		t.Fatalf("query failure=%v opcode=%d", failure, message.opcode)
	}
}
