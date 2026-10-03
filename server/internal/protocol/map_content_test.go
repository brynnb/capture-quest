package protocol_test

import (
	"encoding/json"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
)

func TestMapStreamsUseExplicitJSONContract(t *testing.T) {
	recorder := &frameRecorder{}
	ses := session.NewSessionManager().CreateSession(recorder, 1, "map-wire", nil)
	zero, negative := 0, -20
	info := protocol.PhaserMapInfo{ID: 9999, Name: "Unified Overworld", Width: 16, Height: 28, IsOverworld: 1, TileMinX: &zero, TileMinY: &negative}
	if err := ses.SendStreamJSON(protocol.PhaserMapInfoResponse{PhaserMapInfo: info, Success: true, RequestID: "query"}, opcodes.PhaserMapInfoResponse); err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(recorder.frames[0][6:], &fields); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"id": "9999", "success": "true", "requestId": `"query"`, "isOverworld": "1", "tileMinX": "0", "tileMinY": "-20"} {
		if string(fields[key]) != value {
			t.Errorf("%s=%s, want %s", key, fields[key], value)
		}
	}
	for _, key := range []string{"ID", "is_overworld", "tilesetId", "tileMaxX", "tileMaxY"} {
		if _, exists := fields[key]; exists {
			t.Errorf("unexpected key %s", key)
		}
	}
	request, err := json.Marshal(protocol.PhaserMapLoadRequest{MapID: 9999, RequestID: "arrival"})
	if err != nil || string(request) != `{"mapId":9999,"requestId":"arrival"}` {
		t.Fatalf("destination request=%s %v", request, err)
	}
}

func TestCommittedWarpNotificationUsesExplicitJSONContract(t *testing.T) {
	recorder := &frameRecorder{}
	ses := session.NewSessionManager().CreateSession(recorder, 1, "warp-wire", nil)
	if err := ses.SendStreamJSON(protocol.WarpTileTeleportNotify{MapID: 9999, X: -2, Y: 0, Direction: "UP"}, opcodes.WarpTileTeleportNotify); err != nil {
		t.Fatal(err)
	}
	if len(recorder.frames) != 1 || string(recorder.frames[0][6:]) != `{"mapId":9999,"x":-2,"y":0,"direction":"UP"}` {
		t.Fatalf("unexpected committed warp wire: %q", recorder.frames)
	}
}
