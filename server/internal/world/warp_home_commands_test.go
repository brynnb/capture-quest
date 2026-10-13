package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/testdb"
	"encoding/json"
	"testing"
)

func TestWarpHomeRejectsIdentityFreeOrForeignCommandsBeforeMutation(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	for _, input := range []string{`{}`, `{"requestId":"home","characterId":99,"commandRevision":0}`, `{"requestId":"home","characterId":42}`} {
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.WarpHomeRequest, input)
		var reply InventoryCommandError
		if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.WarpHomeResponse || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success {
			t.Fatalf("invalid home accepted: %+v", messages.streams)
		}
	}
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM character_field_command_state WHERE character_id=42 AND domain='travel'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid home committed revision: %d %v", count, err)
	}
}

func TestWarpHomeOwnershipReadAndResetDoNotRequireHealthySourceOrBattle(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	battleTestStart(t, database, false, nil)
	forgetBattle(42, getBattle(42))
	testdb.Exec(t, database, `UPDATE character_data SET map_id=0,x=0,y=0 WHERE id=42;UPDATE character_battle_state SET battle_json='malformed battle payload' WHERE character_id=42`)
	ses.Client.CharData().MapID, ses.Client.CharData().X, ses.Client.CharData().Y = 50, 7, 8
	battleDispatch(t, wh, ses, opcodes.WarpHomeRequest, `{"requestId":"ownership","characterId":42,"current":true}`)
	var read WarpHomeResponse
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &read) != nil || !read.Success || !read.Current || read.Revision != 0 {
		t.Fatalf("source-independent ownership read %+v", messages.streams)
	}
	var count, mapID int
	var payload string
	if err := database.QueryRow(`SELECT count(*) FROM character_field_command_state WHERE character_id=42 AND domain='travel'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("ownership read mutated revision")
	}
	if err := database.QueryRow(`SELECT battle_json FROM character_battle_state WHERE character_id=42`).Scan(&payload); err != nil || payload != "malformed battle payload" {
		t.Fatal("ownership read changed battle")
	}
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.WarpHomeRequest, `{"requestId":"rescue","characterId":42,"commandRevision":0}`)
	var reset WarpHomeResponse
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reset) != nil || !reset.Success || reset.Current || reset.Revision != 1 || reset.MapID != UnifiedOverworldMapID {
		t.Fatalf("broken source not rescued %+v", messages.streams)
	}
	if err := database.QueryRow(`SELECT map_id FROM character_data WHERE id=42`).Scan(&mapID); err != nil || mapID != UnifiedOverworldMapID || int(ses.Client.CharData().MapID) != mapID {
		t.Fatal("rescue saved/owned identity differs")
	}
	if err := database.QueryRow(`SELECT count(*) FROM character_battle_state WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatal("broken battle retained")
	}
}
