package main

import (
	"testing"

	"capturequest/internal/phaserdata"
)

func TestRawFootTileIDFromBlockData(t *testing.T) {
	blockData := []byte{
		0x00, 0x01, 0x02, 0x03,
		0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b,
		0x0c, 0x0d, 0x0e, 0x0f,
	}

	tests := []struct {
		position int
		want     int
	}{
		{position: 0, want: 0x04},
		{position: 1, want: 0x06},
		{position: 2, want: 0x0c},
		{position: 3, want: 0x0e},
	}

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.position)), func(t *testing.T) {
			got, ok := phaserdata.RawFootTileIDFromBlockData(blockData, tt.position)
			if !ok {
				t.Fatalf("RawFootTileIDFromBlockData(%d) returned ok=false", tt.position)
			}
			if got != tt.want {
				t.Fatalf("RawFootTileIDFromBlockData(%d) = %#x, want %#x", tt.position, got, tt.want)
			}
		})
	}
}

func TestRawFootTileIDFromBlockDataRejectsInvalidPosition(t *testing.T) {
	if got, ok := phaserdata.RawFootTileIDFromBlockData([]byte{0x01, 0x02, 0x03, 0x04}, 2); ok {
		t.Fatalf("RawFootTileIDFromBlockData short block = (%#x, true), want ok=false", got)
	}
	if got, ok := phaserdata.RawFootTileIDFromBlockData(make([]byte, 16), 4); ok {
		t.Fatalf("RawFootTileIDFromBlockData invalid position = (%#x, true), want ok=false", got)
	}
}
