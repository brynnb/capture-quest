package phaserdata

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
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
			got, ok := RawFootTileIDFromBlockData(blockData, tt.position)
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
	if got, ok := RawFootTileIDFromBlockData([]byte{0x01, 0x02, 0x03, 0x04}, 2); ok {
		t.Fatalf("RawFootTileIDFromBlockData short block = (%#x, true), want ok=false", got)
	}
	if got, ok := RawFootTileIDFromBlockData(make([]byte, 16), 4); ok {
		t.Fatalf("RawFootTileIDFromBlockData invalid position = (%#x, true), want ok=false", got)
	}
}

func TestViridianSouthLedgeRawFootTilePair(t *testing.T) {
	db := openPokemonSQLiteForTest(t)
	defer db.Close()

	standing := rawFootTileAt(t, db, 7, -46)
	front := rawFootTileAt(t, db, 7, -45)

	if standing != 0x39 || front != 0x37 {
		t.Fatalf("Viridian ledge raw foot pair = (%#x, %#x), want (0x39, 0x37)", standing, front)
	}
}

func TestFuchsiaDecorativeRockEdgesDoNotUseLedgeRawFootPairs(t *testing.T) {
	db := openPokemonSQLiteForTest(t)
	defer db.Close()

	rightStanding := rawFootTileAt(t, db, 132, 40)
	rightFront := rawFootTileAt(t, db, 133, 40)
	downStanding := rawFootTileAt(t, db, 131, 40)
	downFront := rawFootTileAt(t, db, 131, 41)

	if rightStanding != 0x2c || rightFront != 0x50 {
		t.Fatalf("Fuchsia right edge raw foot pair = (%#x, %#x), want (0x2c, 0x50)", rightStanding, rightFront)
	}
	if downStanding != 0x39 || downFront != 0x55 {
		t.Fatalf("Fuchsia down edge raw foot pair = (%#x, %#x), want (0x39, 0x55)", downStanding, downFront)
	}
}

func openPokemonSQLiteForTest(t *testing.T) *sql.DB {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "..", "public", "phaser", "pokemon.db"),
		filepath.Join("..", "public", "phaser", "pokemon.db"),
		filepath.Join("public", "phaser", "pokemon.db"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		db, err := sql.Open("sqlite", candidate)
		if err != nil {
			continue
		}
		if err := db.Ping(); err == nil {
			return db
		}
		db.Close()
	}
	t.Fatal("could not open public/phaser/pokemon.db")
	return nil
}

func rawFootTileAt(t *testing.T, db *sql.DB, x, y int) int {
	t.Helper()
	var (
		position  int
		blockData []byte
	)
	if err := db.QueryRow(`
		SELECT ti.position, bs.block_data
		FROM tiles t
		JOIN tile_images ti ON ti.id = t.tile_image_id
		JOIN blocksets bs
		  ON bs.tileset_id = ti.tileset_id AND bs.block_index = ti.block_index
		WHERE t.x = ? AND t.y = ?`,
		x, y,
	).Scan(&position, &blockData); err != nil {
		t.Fatalf("query raw foot tile at (%d,%d): %v", x, y, err)
	}
	raw, ok := RawFootTileIDFromBlockData(blockData, position)
	if !ok {
		t.Fatalf("RawFootTileIDFromBlockData at (%d,%d) returned ok=false", x, y)
	}
	return raw
}
