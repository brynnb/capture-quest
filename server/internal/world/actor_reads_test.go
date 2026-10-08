package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/testdb"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
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

func TestEntryPreparesSurfingBeforeQueryFreeActorPresentation(t *testing.T) {
	for _, water := range []bool{false, true} {
		t.Run(fmt.Sprint(water), func(t *testing.T) {
			wh, ses, _ := setupIssuedStep(t)
			collision := 1
			if water {
				collision = 2
			}
			testdb.Exec(t, wh.database, `UPDATE phaser_tiles SET collision_type=$1 WHERE map_id=50 AND x=7 AND y=8`, collision)
			wh.ActorManager.InvalidateCollisionMap(50)
			old := db.GlobalWorldDB
			db.GlobalWorldDB = nil
			t.Cleanup(func() { db.GlobalWorldDB = old })
			wh.database.SetMaxOpenConns(1)
			if err := ses.ExecuteCommand(context.Background(), func() {
				if err := wh.PlayerMovement.restoreMovementRoute(ses); err != nil {
					t.Fatal(err)
				}
			}); err != nil {
				t.Fatal(err)
			}
			if wh.PlayerMovement.IsSurfing(42) != water {
				t.Fatal("entry did not prepare authoritative surfing state")
			}
			wh.ActorManager.InvalidateCollisionMap(50)
			lease, err := wh.database.Conn(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			before := wh.database.Stats().WaitCount
			done := make(chan *PhaserActor, 1)
			go func() { done <- createPlayerActor(ses, wh) }()
			var actor *PhaserActor
			select {
			case actor = <-done:
			case <-time.After(200 * time.Millisecond):
				lease.Close()
				<-done
				t.Fatal("actor presentation performed collision SQL")
			}
			presence := ses.Presence()
			wanted := playerSpriteName(presence.Gender, false, water)
			if actor == nil || actor.SpriteName == nil || *actor.SpriteName != wanted {
				t.Fatalf("prepared actor sprite: %+v want %s", actor, wanted)
			}
			if wh.database.Stats().WaitCount != before {
				t.Fatal("actor presentation borrowed a connection")
			}
		})
	}
}

func TestEntrySurfPreparationFailurePreservesMovementState(t *testing.T) {
	for _, failure := range []string{"pool-cancel", "source-error"} {
		t.Run(failure, func(t *testing.T) {
			wh, ses, _ := setupIssuedStep(t)
			state := wh.PlayerMovement.players[42]
			state.IsSurfing = true
			wh.ActorManager.InvalidateCollisionMap(50)
			ctx := context.Background()
			if failure == "pool-cancel" {
				wh.database.SetMaxOpenConns(1)
				lease, err := wh.database.Conn(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer lease.Close()
				owned, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
				ctx = owned
			} else {
				testdb.Exec(t, wh.database, `ALTER TABLE phaser_tiles RENAME TO unavailable_tiles`)
			}
			var preparation error
			err := ses.ExecuteCommand(ctx, func() { preparation = wh.PlayerMovement.restoreMovementRoute(ses) })
			if failure == "pool-cancel" {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal(err)
				}
			} else if preparation == nil {
				t.Fatal("missing entry tile source accepted")
			}
			if !state.IsSurfing || state.CurrentX != 7 || state.CurrentY != 8 {
				t.Fatal("failed entry preparation projected guessed state")
			}
		})
	}
}
