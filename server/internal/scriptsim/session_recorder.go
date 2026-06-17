package scriptsim

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"capturequest/internal/api/opcodes"
	model "capturequest/internal/db/models"
	"capturequest/internal/session"
)

type RecordedMessage struct {
	Channel string
	Opcode  opcodes.OpCode
	Payload json.RawMessage
}

type SessionRecorder struct {
	Messages []RecordedMessage
}

func (r *SessionRecorder) SendDatagram(_ int, data []byte) error {
	if len(data) < 2 {
		return fmt.Errorf("record datagram: short frame %d", len(data))
	}
	r.Messages = append(r.Messages, RecordedMessage{
		Channel: "datagram",
		Opcode:  opcodes.OpCode(binary.LittleEndian.Uint16(data[:2])),
		Payload: append([]byte(nil), data[2:]...),
	})
	return nil
}

func (r *SessionRecorder) SendStream(_ int, data []byte) error {
	if len(data) < 6 {
		return fmt.Errorf("record stream: short frame %d", len(data))
	}
	payloadLen := int(binary.LittleEndian.Uint32(data[:4]))
	if payloadLen != len(data)-4 {
		return fmt.Errorf("record stream: length header %d does not match frame %d", payloadLen, len(data)-4)
	}
	r.Messages = append(r.Messages, RecordedMessage{
		Channel: "stream",
		Opcode:  opcodes.OpCode(binary.LittleEndian.Uint16(data[4:6])),
		Payload: append([]byte(nil), data[6:]...),
	})
	return nil
}

func NewRecordedSession(charID int64, name string, mapID int, x, y int) (*session.Session, *SessionRecorder) {
	recorder := &SessionRecorder{}
	char := &model.CharacterData{
		ID:    uint32(charID),
		Name:  name,
		MapID: uint32(mapID),
		X:     float64(x),
		Y:     float64(y),
	}
	return &session.Session{
		SessionID:     int(charID),
		Authenticated: true,
		MapID:         mapID,
		X:             float32(x),
		Y:             float32(y),
		CharacterName: name,
		Client:        &recordedClient{char: char},
		Messenger:     recorder,
	}, recorder
}

type recordedClient struct {
	char           *model.CharacterData
	systemMessages []string
}

func (c *recordedClient) CharData() *model.CharacterData       { return c.char }
func (c *recordedClient) ID() int                              { return int(c.char.ID) }
func (c *recordedClient) Name() string                         { return c.char.Name }
func (c *recordedClient) Say(string)                           {}
func (c *recordedClient) ShowNetworkStatsEnabled() bool        { return false }
func (c *recordedClient) SetShowNetworkStatsEnabled(bool)      {}
func (c *recordedClient) AllowTrainerRebattles() bool          { return false }
func (c *recordedClient) SetAllowTrainerRebattlesEnabled(bool) {}
func (c *recordedClient) Options() interface{}                 { return nil }
func (c *recordedClient) SaveOptions() error                   { return nil }
func (c *recordedClient) SendSystemMessage(text string) {
	c.systemMessages = append(c.systemMessages, text)
}
func (c *recordedClient) SendSpecialMessage(text, _ string) {
	c.systemMessages = append(c.systemMessages, text)
}
func (c *recordedClient) SendStateUpdate() {}
func (c *recordedClient) Shutdown()        {}
