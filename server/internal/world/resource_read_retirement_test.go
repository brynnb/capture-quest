package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"encoding/json"
	"testing"
)

func TestRetiredResourceReadOpcodesCannotPublishUnownedSnapshots(t *testing.T) {
	_, wh, ses, messages := battleTestWorld(t)
	wh.database = nil
	db.GlobalWorldDB = nil
	for _, opcode := range []opcodes.OpCode{opcodes.CQInventoryRequest, opcodes.PokemonPartyRequest} {
		messages.streams = nil
		battleDispatch(t, wh, ses, opcode, `{}`)
		var reply struct {
			Success bool
			Error   string
		}
		if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success || reply.Error != "Use current gameplay recovery." {
			t.Fatalf("legacy read %d published %+v", opcode, reply)
		}
	}
}
