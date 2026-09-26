package world

import (
	"encoding/binary"
	"encoding/json"
	"testing"

	"capturequest/internal/api"
	"capturequest/internal/api/opcodes"
	model "capturequest/internal/db/models"
	"capturequest/internal/session"
)

func clientPacket(op opcodes.OpCode, payload string) []byte {
	data := make([]byte, 2+len(payload))
	binary.LittleEndian.PutUint16(data, uint16(op))
	copy(data[2:], payload)
	return data
}

func TestDispatcherRejectsGameplayBeforeCharacterSelection(t *testing.T) {
	registry := NewWorldOpCodeRegistry()
	ses := &session.Session{Authenticated: true}
	// Exercise the actual handlers' boundary: these handlers require a character
	// and previously could dereference nil before reaching gameplay validation.
	for _, op := range []opcodes.OpCode{opcodes.PokeBattleActionRequest, opcodes.PokeBattleStartRequest, opcodes.PokemonPartyRequest} {
		registry.HandleWorldPacket(ses, clientPacket(op, "{}"))
	}
}

func TestDispatcherSessionStagesAndMalformedCommands(t *testing.T) {
	registry := NewWorldOpCodeRegistry()
	called := 0
	for op := range registry.handlers {
		registry.handlers[op] = func(*session.Session, []byte, *WorldHandler) bool { called++; return false }
	}
	guest := &session.Session{}
	account := &session.Session{Authenticated: true}
	playing := &session.Session{Authenticated: true, Client: &testSessionClient{char: &model.CharacterData{ID: 1}}}
	for _, tc := range []struct {
		ses   *session.Session
		op    opcodes.OpCode
		allow bool
	}{
		{guest, opcodes.JWTLogin, true}, {guest, opcodes.Heartbeat, true},
		{guest, opcodes.StaticDataRequest, false}, {guest, opcodes.EnterWorld, false},
		{account, opcodes.EnterWorld, true}, {account, opcodes.CQMerchantBuyRequest, false},
		{playing, opcodes.EnterWorld, false}, {playing, opcodes.JWTLogin, false},
		{playing, opcodes.CQMerchantBuyRequest, true},
	} {
		before := called
		registry.HandleWorldPacket(tc.ses, clientPacket(tc.op, "{}"))
		if (called > before) != tc.allow {
			t.Fatalf("opcode %d: called=%v, want %v", tc.op, called > before, tc.allow)
		}
	}
	before := called
	for _, payload := range []string{"", "null", "[]", "{", "{} {}"} {
		registry.HandleWorldPacket(playing, clientPacket(opcodes.CQMerchantBuyRequest, payload))
	}
	registry.HandleWorldPacket(playing, make([]byte, api.MaxClientPacketSize+1))
	playing.Close()
	registry.HandleWorldPacket(playing, clientPacket(opcodes.CQMerchantBuyRequest, "{}"))
	if called != before {
		t.Fatal("invalid or closed-session commands reached handlers")
	}
}

func TestPacketLimitAccommodatesExistingTileBatch(t *testing.T) {
	req := TileEditorPlaceReq{Tiles: make([]TileEdit, maxTilesPerRequest), MapID: UnifiedOverworldMapID}
	footID := 2147483647
	for i := range req.Tiles {
		req.Tiles[i] = TileEdit{X: -2147483648, Y: 2147483647, TileImageID: 2147483647, CollisionType: 2147483647, RawFootTileID: &footID, TalkOverTile: true, Erased: true}
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(data)+2 > api.MaxClientPacketSize {
		t.Fatalf("500-tile request exceeds packet limit: %d", len(data)+2)
	}
}
