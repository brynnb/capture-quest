package api

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

func TestReadClientFrameRejectsLengthBeforeReadingPayload(t *testing.T) {
	for _, n := range []uint32{0, 1, MaxClientPacketSize + 1, ^uint32(0)} {
		var header [4]byte
		binary.LittleEndian.PutUint32(header[:], n)
		reader := io.MultiReader(bytes.NewReader(header[:]), failReader{t})
		if _, err := ReadClientFrame(reader); err == nil {
			t.Fatalf("accepted length %d", n)
		}
	}
}

type failReader struct{ t *testing.T }

func (r failReader) Read([]byte) (int, error) {
	r.t.Fatal("read payload for rejected length")
	return 0, io.EOF
}

func TestReadClientFramePreservesBoundariesAndRejectsTruncation(t *testing.T) {
	var wire bytes.Buffer
	for _, payload := range [][]byte{{1, 0}, bytes.Repeat([]byte{'x'}, MaxClientPacketSize)} {
		binary.Write(&wire, binary.LittleEndian, uint32(len(payload)))
		wire.Write(payload)
	}
	for _, want := range []int{2, MaxClientPacketSize} {
		packet, err := ReadClientFrame(&wire)
		if err != nil || len(packet) != want {
			t.Fatalf("frame = %d bytes, %v", len(packet), err)
		}
	}
	for _, data := range [][]byte{{1}, {2, 0, 0, 0, 1}} {
		if _, err := ReadClientFrame(bytes.NewReader(data)); err == nil {
			t.Fatal("accepted truncated frame")
		}
	}
}
