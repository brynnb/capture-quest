package session

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"capturequest/internal/api/opcodes"
)

// SendJSON sends a JSON payload prefixed with an opcode via datagram
func (s *Session) SendJSON(data interface{}, opcode opcodes.OpCode) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("SendJSON marshal error: %w", err)
	}

	totalLen := 2 + len(payload)
	buf := make([]byte, totalLen)
	binary.LittleEndian.PutUint16(buf[:2], uint16(opcode))
	copy(buf[2:], payload)

	return s.Messenger.SendDatagram(s.SessionID, buf)
}

// SendStreamJSON sends a JSON payload prefixed with an opcode via reliable stream
func (s *Session) SendStreamJSON(data interface{}, opcode opcodes.OpCode) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("SendStreamJSON marshal error: %w", err)
	}

	const headerSize = 6
	totalLen := headerSize + len(payload)
	buf := make([]byte, totalLen)

	// [length:uint32_LE][opcode:uint16_LE][payload]
	binary.LittleEndian.PutUint32(buf[0:4], uint32(2+len(payload)))
	binary.LittleEndian.PutUint16(buf[4:6], uint16(opcode))
	copy(buf[6:], payload)

	return s.Messenger.SendStream(s.SessionID, buf)
}
