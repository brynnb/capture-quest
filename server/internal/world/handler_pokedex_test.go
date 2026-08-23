package world

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupPokedexTestDB(t *testing.T) *sql.DB {
	t.Helper()
	raw, err := sql.Open("sqlite", "file:pokedex-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { raw.Close() })
	if _, err := raw.Exec(`
		CREATE TABLE character_pokedex (
			character_id INTEGER NOT NULL,
			pokemon_id INTEGER NOT NULL,
			seen INTEGER NOT NULL DEFAULT 0,
			caught INTEGER NOT NULL DEFAULT 0,
			first_seen_at TEXT,
			first_caught_at TEXT,
			PRIMARY KEY (character_id, pokemon_id)
		);
		CREATE TABLE character_pokemon (
			character_id INTEGER NOT NULL,
			pokemon_id INTEGER NOT NULL
		);`); err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPokedexSeenAndCaughtTransitions(t *testing.T) {
	raw := setupPokedexTestDB(t)
	if err := markPokemonSeen(raw, 42, 25); err != nil {
		t.Fatal(err)
	}
	var seen, caught int
	if err := raw.QueryRow(`SELECT seen, caught FROM character_pokedex WHERE character_id = 42 AND pokemon_id = 25`).Scan(&seen, &caught); err != nil {
		t.Fatal(err)
	}
	if seen != 1 || caught != 0 {
		t.Fatalf("after encounter seen/caught = %d/%d, want 1/0", seen, caught)
	}

	if _, err := raw.Exec(`UPDATE character_pokedex SET first_seen_at = '1998-09-28 12:00:00' WHERE character_id = 42 AND pokemon_id = 25`); err != nil {
		t.Fatal(err)
	}
	if err := markPokemonCaught(raw, 42, 25); err != nil {
		t.Fatal(err)
	}
	var firstSeen string
	if err := raw.QueryRow(`SELECT seen, caught, first_seen_at FROM character_pokedex WHERE character_id = 42 AND pokemon_id = 25`).Scan(&seen, &caught, &firstSeen); err != nil {
		t.Fatal(err)
	}
	if seen != 1 || caught != 1 || firstSeen != "1998-09-28 12:00:00" {
		t.Fatalf("after capture seen/caught/first_seen = %d/%d/%q", seen, caught, firstSeen)
	}

	if err := markPokemonSeen(raw, 42, 25); err != nil {
		t.Fatal(err)
	}
	if err := raw.QueryRow(`SELECT seen, caught FROM character_pokedex WHERE character_id = 42 AND pokemon_id = 25`).Scan(&seen, &caught); err != nil {
		t.Fatal(err)
	}
	if seen != 1 || caught != 1 {
		t.Fatalf("a later encounter lost ownership: %d/%d", seen, caught)
	}
}

func TestPokedexReconcilesEveryOwnedSpecies(t *testing.T) {
	raw := setupPokedexTestDB(t)
	if _, err := raw.Exec(`
		INSERT INTO character_pokemon (character_id, pokemon_id) VALUES
			(42, 1), (42, 1), (42, 4), (99, 7), (42, 152);
		INSERT INTO character_pokedex (character_id, pokemon_id, seen, caught, first_seen_at)
		VALUES (42, 4, 1, 0, '1998-09-28 12:00:00')`); err != nil {
		t.Fatal(err)
	}
	if err := reconcileOwnedPokemonPokedex(raw, 42); err != nil {
		t.Fatal(err)
	}

	rows, err := raw.Query(`SELECT pokemon_id, seen, caught FROM character_pokedex WHERE character_id = 42 ORDER BY pokemon_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type status struct{ id, seen, caught int }
	var got []status
	for rows.Next() {
		var value status
		if err := rows.Scan(&value.id, &value.seen, &value.caught); err != nil {
			t.Fatal(err)
		}
		got = append(got, value)
	}
	want := []status{{1, 1, 1}, {4, 1, 1}}
	if len(got) != len(want) {
		t.Fatalf("statuses = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("statuses = %#v, want %#v", got, want)
		}
	}
}

func TestPokedexRejectsInvalidIdentities(t *testing.T) {
	raw := setupPokedexTestDB(t)
	for _, test := range []struct {
		characterID int64
		pokemonID   int
	}{{0, 25}, {42, 0}, {42, 152}} {
		if err := markPokemonCaught(raw, test.characterID, test.pokemonID); err == nil {
			t.Fatalf("markPokemonCaught(%d, %d) unexpectedly succeeded", test.characterID, test.pokemonID)
		}
	}
}

func TestTrainerCardBadgeFlagsUseBadgeObtainedFlags(t *testing.T) {
	want := []string{
		"EVENT_GOT_BOULDERBADGE",
		"EVENT_GOT_CASCADEBADGE",
		"EVENT_GOT_THUNDERBADGE",
		"EVENT_GOT_RAINBOWBADGE",
		"EVENT_GOT_SOULBADGE",
		"EVENT_GOT_MARSHBADGE",
		"EVENT_GOT_VOLCANOBADGE",
		"EVENT_GOT_EARTHBADGE",
	}

	if len(badgeFlags) != len(want) {
		t.Fatalf("badgeFlags length = %d, want %d", len(badgeFlags), len(want))
	}
	for i := range want {
		if badgeFlags[i] != want[i] {
			t.Fatalf("badgeFlags[%d] = %q, want %q", i, badgeFlags[i], want[i])
		}
	}
}
