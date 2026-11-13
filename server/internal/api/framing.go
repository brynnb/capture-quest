package api

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MaxClientPacketSize includes the opcode but excludes the four-byte frame
// header. 256 KiB accommodates the existing 500-tile edit/undo batch, including
// full metadata. This is an inbound limit; catalog responses can be larger.
const MaxClientPacketSize = 256 * 1024

// ReadClientFrame checks the untrusted length before allocating its payload.
// The transport owns deadlines and closes the connection on framing errors.
func ReadClientFrame(r io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(header[:])
	if n < 2 || n > MaxClientPacketSize {
		return nil, fmt.Errorf("invalid client frame length %d", n)
	}
	packet := make([]byte, n)
	if _, err := io.ReadFull(r, packet); err != nil {
		return nil, err
	}
	return packet, nil
}
