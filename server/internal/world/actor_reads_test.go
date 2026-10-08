package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/testdb"
	"encoding/json"
	"testing"
)

func TestActorReadIsCorrelatedInjectedAndIncludesOwnedOverrides(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	testdb.Exec(t, wh.database, `INSERT INTO phaser_objects(id,map_id,name,object_type,sprite_name,x,y) VALUES(777,50,'Boulder','npc','SPRITE_BOULDER',8,8); INSERT INTO character_object_positions(character_id,object_id,map_id,x,y) VALUES(42,777,50,9,8)`)
	db.GlobalWorldDB = nil
	battleDispatch(t, wh, ses, opcodes.PhaserActorsRequest, `{"requestId":"actors","characterId":42,"mapId":50}`)
	var reply PhaserActorsResponse
	if len(messages.streams) < 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || !reply.Success || reply.RequestID != "actors" || reply.CharacterID != 42 || reply.MapID != 50 || len(reply.Actors) != 1 || reply.Actors[0].ID != wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 777) || *reply.Actors[0].X != 9 {
		t.Fatalf("owned actor reply: %+v", reply)
	}
	for _, payload := range []string{`{"mapId":50}`, `{"requestId":"foreign","characterId":43,"mapId":50}`, `{"requestId":"bad","characterId":42,"mapId":50,"extra":true}`} {
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.PhaserActorsRequest, payload)
		var failure struct{ Success bool }
		if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &failure) != nil || failure.Success {
			t.Fatal("unowned actor read accepted")
		}
	}
}
