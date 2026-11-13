package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/testdb"
	"testing"
)

func TestRetiredMapChangeRequestCannotMutateOrPublishCharacterState(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	char := ses.Client.CharData()
	char.MapID, char.X, char.Y, char.Z, char.Heading = 50, 7, 8, 1, 2
	ses.MapID, ses.InstanceID, ses.X, ses.Y = 50, 2, 7, 8
	wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "UP")
	testdb.Exec(t, database, `UPDATE character_data SET map_id=50,x=7,y=8,z=1,heading=2 WHERE id=42`)
	for _, payload := range []string{
		`{"mapId":60,"instanceId":99,"x":3,"y":4,"z":5,"heading":6}`,
		`{"zoneId":60,"instanceId":99,"x":3,"y":4,"z":5,"heading":6}`,
		`{"mapId":50,"instanceId":99,"x":3,"y":4,"z":5,"heading":6}`,
	} {
		battleDispatch(t, wh, ses, opcodes.MapChangeRequest, payload)
	}
	if ses.MapID != 50 || ses.InstanceID != 2 || ses.X != 7 || ses.Y != 8 || char.MapID != 50 || char.X != 7 || char.Y != 8 || char.Z != 1 || char.Heading != 2 {
		t.Fatal("retired opcode changed live character state")
	}
	x, y, mapID, ok := wh.PlayerMovement.GetPosition(42)
	if !ok || x != 7 || y != 8 || mapID != 50 {
		t.Fatal("retired opcode changed movement state")
	}
	var savedMap int
	var savedX, savedY, savedZ, savedHeading float64
	if err := database.QueryRow(`SELECT map_id,x,y,z,heading FROM character_data WHERE id=42`).Scan(&savedMap, &savedX, &savedY, &savedZ, &savedHeading); err != nil {
		t.Fatal(err)
	}
	if savedMap != 50 || savedX != 7 || savedY != 8 || savedZ != 1 || savedHeading != 2 {
		t.Fatal("retired opcode changed durable position")
	}
	if len(messages.streams) != 0 {
		t.Fatalf("retired opcode published %+v", messages.streams)
	}
	if _, registered := NewWorldOpCodeRegistry().handlers[opcodes.MapChangeRequest]; registered {
		t.Fatal("legacy setter remains registered")
	}
}
