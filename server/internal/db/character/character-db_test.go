package db_character

import (
	"context"
	"database/sql"
	"testing"

	"capturequest/internal/db"
	model "capturequest/internal/db/models"

	_ "modernc.org/sqlite"
)

func TestCumulativePlaytimeSurvivesGeneralCharacterSaveAndRepeatedTotals(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if _, err := database.Exec(`
		CREATE TABLE character_data (
			id INTEGER PRIMARY KEY,
			map_id INTEGER NOT NULL,
			x REAL NOT NULL,
			y REAL NOT NULL,
			z REAL NOT NULL,
			heading REAL NOT NULL,
			last_login INTEGER NOT NULL,
			time_played INTEGER NOT NULL
		);
		INSERT INTO character_data VALUES (9, 1, 2, 3, 0, 0, 100, 10);
	`); err != nil {
		t.Fatal(err)
	}

	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: database}
	t.Cleanup(func() { db.GlobalWorldDB = previous })

	for _, total := range []uint32{15, 15, 12} {
		if err := SaveCharacterPlaytime(context.Background(), database, 9, 7, total); err != nil {
			t.Fatal(err)
		}
	}
	if err := UpdateCharacter(context.Background(), database, &model.CharacterData{
		ID:         9,
		MapID:      2,
		X:          4,
		Y:          5,
		LastLogin:  200,
		TimePlayed: 0,
	}, 7); err != nil {
		t.Fatal(err)
	}

	var seconds int
	if err := database.QueryRow(`SELECT time_played FROM character_data WHERE id = 9`).Scan(&seconds); err != nil {
		t.Fatal(err)
	}
	if seconds != 15 {
		t.Fatalf("time_played = %d, want 15", seconds)
	}
	if err := SaveCharacterPlaytime(context.Background(), database, 404, 7, 5); err == nil {
		t.Fatal("missing character accepted a playtime save")
	}
	if err := UpdateCharacter(context.Background(), database, &model.CharacterData{ID: 404}, 7); err == nil {
		t.Fatal("missing character accepted a general save")
	}
}
