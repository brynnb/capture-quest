package world

import (
	"encoding/binary"
	"encoding/json"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/session"
)

type discordChatMessenger struct {
	datagrams []recordedStreamMessage
}

func (m *discordChatMessenger) SendDatagram(sessionID int, data []byte) error {
	m.datagrams = append(m.datagrams, recordedStreamMessage{
		sessionID: sessionID,
		opcode:    opcodes.OpCode(binary.LittleEndian.Uint16(data[:2])),
		payload:   append([]byte(nil), data[2:]...),
	})
	return nil
}

func (m *discordChatMessenger) SendStream(int, []byte) error { return nil }

func TestDiscordChatUsesGlobalBroadcastAndOutboundSink(t *testing.T) {
	messenger := &discordChatMessenger{}
	manager := session.NewSessionManager()
	session.InitSessionManager(manager)
	observer := manager.CreateSession(messenger, 1, "127.0.0.1", nil)
	observer.Authenticated = true
	wh := &WorldHandler{}
	var bridged ChatMessageBroadcast
	wh.SetPublicChatSink(func(message ChatMessageBroadcast) { bridged = message })

	if err := wh.BroadcastExternalChat("Tester[Discord]", " hello   CaptureQuest "); err != nil {
		t.Fatal(err)
	}
	if len(messenger.datagrams) != 1 || messenger.datagrams[0].opcode != opcodes.ChatMessageBroadcast {
		t.Fatalf("unexpected game broadcast: %#v", messenger.datagrams)
	}
	var message ChatMessageBroadcast
	if err := json.Unmarshal(messenger.datagrams[0].payload, &message); err != nil {
		t.Fatal(err)
	}
	if message.SenderName != "Tester[Discord]" || message.Text != "hello CaptureQuest" || message.MessageType != "general" {
		t.Fatalf("unexpected external chat message: %#v", message)
	}
	if message != bridged {
		t.Fatalf("game=%#v outbound=%#v", message, bridged)
	}
}
