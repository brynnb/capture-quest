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

func TestEntryPresencePublishesOnceAndActorReadsDoNotRebroadcast(t *testing.T) {
	wh, fixture, _ := setupIssuedStep(t)
	originMessages, peerMessages := &recordingMessenger{}, &recordingMessenger{}
	origin := wh.sessionManager.CreateNextSession(originMessages, "", nil)
	origin.Client, origin.Authenticated = fixture.Client, true
	wh.PlayerMovement.RegisterPlayer(origin, 42, 7, 8, 50, "RIGHT")
	peer := wh.sessionManager.CreateNextSession(peerMessages, "", nil)
	peer.Authenticated, peer.MapID = true, 50
	peer.PublishPresence()
	SendPlayerSpawn(origin, wh)
	if len(peerMessages.streams) != 1 || peerMessages.streams[0].opcode != opcodes.PhaserActorPositionUpdate || len(originMessages.streams) != 0 {
		t.Fatal("entry did not publish single peer stream event")
	}
	peerMessages.streams = nil
	battleDispatch(t, wh, origin, opcodes.PhaserActorsRequest, `{"requestId":"refresh","characterId":42,"mapId":50}`)
	if len(peerMessages.streams) != 0 {
		t.Fatal("actor read rebroadcast player entry")
	}
	origin.Close()
	peer.Close()
}

func TestBoulderPublicationIsScopedToProducingCharacter(t *testing.T) {
	wh, fixture, _ := setupIssuedStep(t)
	ownerMessages, peerMessages := &recordingMessenger{}, &recordingMessenger{}
	owner := wh.sessionManager.CreateNextSession(ownerMessages, "", nil)
	owner.Client, owner.Authenticated = fixture.Client, true
	wh.PlayerMovement.RegisterPlayer(owner, 42, 7, 8, 50, "RIGHT")
	peer := wh.sessionManager.CreateNextSession(peerMessages, "", nil)
	peer.Authenticated, peer.MapID = true, 50
	peer.PublishPresence()
	db.GlobalWorldDB = nil
	wh.PlayerMovement.broadcastBoulderPushResult(42, BoulderPushResult{Success: true, ObjectID: 777, ObjectName: "Boulder", MapID: 50, ToX: 9, ToY: 8, Direction: "RIGHT"})
	if len(ownerMessages.streams) != 1 || ownerMessages.streams[0].opcode != opcodes.PhaserActorPositionUpdate || len(peerMessages.streams) != 0 {
		t.Fatal("character-private boulder update crossed viewer boundary")
	}
	owner.Close()
	peer.Close()
}
