package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"encoding/json"
	"testing"
)

func TestResourceChangeNoticesCarryOnlyCurrentOwnerIdentity(t *testing.T) {
	_, wh, ses, messages := battleTestWorld(t)
	wh.database = nil
	db.GlobalWorldDB = nil
	messages.streams = nil
	notifyResourceChange(ses)
	var payload map[string]interface{}
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.ResourcesChangedNotify || json.Unmarshal(messages.streams[0].payload, &payload) != nil {
		t.Fatal("missing resource notice")
	}
	if len(payload) != 3 || payload["success"] != true || payload["resourcesChanged"] != true || payload["characterId"] != float64(42) {
		t.Fatalf("historical data: %v", payload)
	}

	ses.Close()
	messages.streams = nil
	notifyResourceChange(ses)
	notifyResourceChange(ses)
	if len(messages.streams) != 0 {
		t.Fatal("closed owner received resource notice")
	}
}
