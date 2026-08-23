package db_character

import (
	"database/sql"
	"testing"

	"capturequest/internal/db"
	model "capturequest/internal/db/models"

	_ "modernc.org/sqlite"
)

func TestPlaytimeIncrementSurvivesGeneralCharacterSave(t *testing.T) {
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

	if err := AddCharacterPlaytime(9, 7, 5); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCharacter(&model.CharacterData{
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
}
