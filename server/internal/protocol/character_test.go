package protocol_test

import (
	"encoding/binary"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/db/models"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
)

type frameRecorder struct{ frames [][]byte }

func (r *frameRecorder) SendDatagram(int, []byte) error { return nil }
func (r *frameRecorder) SendStream(_ int, frame []byte) error {
	r.frames = append(r.frames, append([]byte(nil), frame...))
	return nil
}

func TestCharacterStreamsUseExplicitJSONContract(t *testing.T) {
	recorder := &frameRecorder{}
	ses := session.NewSessionManager().CreateSession(recorder, 1, "wire-test", nil)
	stored := `{"rivalName":"stored value must not leak"}`
	deleted := time.Date(2026, 9, 29, 1, 2, 3, 0, time.UTC)
	char := protocol.CharacterData{
		CharacterData: models.CharacterData{ID: 7, AccountID: 9, Name: "Red", MapID: 38, FactionID: 2, DeletedAt: &deleted, Options: &stored},
		Options:       db_character.DefaultOptions(),
	}
	cases := []struct {
		opcode   opcodes.OpCode
		value    any
		required map[string]any
	}{
		{opcodes.CharacterData, char, map[string]any{"id": float64(7), "accountId": float64(9), "name": "Red", "mapId": float64(38), "factionId": float64(2), "deletedAt": "2026-09-29T01:02:03Z"}},
		{opcodes.CharacterWallet, models.CharacterWallet{CharacterID: 7, Pokedollars: 500}, map[string]any{"characterId": float64(7), "pokedollars": float64(500)}},
		{opcodes.CharacterBind, models.CharacterBind{ID: 7, MapID: 41, X: 3, Y: 4}, map[string]any{"id": float64(7), "mapId": float64(41), "x": float64(3), "y": float64(4)}},
	}
	for _, tc := range cases {
		if err := ses.SendStreamJSON(tc.value, tc.opcode); err != nil {
			t.Fatal(err)
		}
		frame := recorder.frames[len(recorder.frames)-1]
		if int(binary.LittleEndian.Uint32(frame[:4])) != len(frame)-4 || opcodes.OpCode(binary.LittleEndian.Uint16(frame[4:6])) != tc.opcode {
			t.Fatal("invalid frame header")
		}
		var payload map[string]any
		if err := json.Unmarshal(frame[6:], &payload); err != nil {
			t.Fatal(err)
		}
		for key, value := range tc.required {
			if !reflect.DeepEqual(payload[key], value) {
				t.Errorf("opcode %d field %s = %#v, want %#v", tc.opcode, key, payload[key], value)
			}
		}
		for _, forbidden := range []string{"ID", "MapID", "CharacterData", "characterData"} {
			if _, ok := payload[forbidden]; ok {
				t.Errorf("unexpected field %s", forbidden)
			}
		}
		if tc.opcode == opcodes.CharacterData {
			preferences, ok := payload["options"].(map[string]any)
			if !ok || preferences["rivalName"] != db_character.DefaultRivalName || preferences["showNetworkStats"] != true {
				t.Fatalf("options are not the parsed preferences: %#v", payload["options"])
			}
		}
	}
	// Optional values have one representation in JSON and generated TypeScript.
	char.Options, char.DeletedAt = nil, nil
	if err := ses.SendStreamJSON(char, opcodes.CharacterData); err != nil {
		t.Fatal(err)
	}
	var optionalPayload map[string]any
	if err := json.Unmarshal(recorder.frames[len(recorder.frames)-1][6:], &optionalPayload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"options", "deletedAt"} {
		if _, present := optionalPayload[key]; present {
			t.Errorf("nil %s was not omitted", key)
		}
	}

}

func TestBaseModelsDeclareEveryJSONField(t *testing.T) {
	for _, model := range []any{models.Variables{}, models.CharacterData{}, models.CharacterWallet{}, models.CharacterBind{}, db_character.CharacterOptions{}} {
		typ := reflect.TypeOf(model)
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath == "" && field.Tag.Get("json") == "" {
				t.Errorf("%s.%s has no explicit JSON name", typ.Name(), field.Name)
			}
		}
	}
}
