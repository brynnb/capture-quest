package world

import (
	"database/sql"
	"testing"

	"capturequest/internal/db"

	_ "modernc.org/sqlite"
)

func TestPerformInGameTradeRoute18Gate2FYoungster(t *testing.T) {
	raw := setupInGameTradeTestDB(t, 42, true)

	trade, err := queryInGameTradeDefinitionByText(raw, route18Gate2FYoungsterTextConstant)
	if err != nil {
		t.Fatal(err)
	}

	outcome, err := performInGameTrade(raw, 42, trade)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Traded || outcome.WrongPokemon || outcome.AlreadyCompleted {
		t.Fatalf("outcome = %#v", outcome)
	}
	if outcome.Dialogue != "Okay, connect the cable like so!\n\nYou traded SLOWBRO for LICKITUNG!\n\nHey thanks!" {
		t.Fatalf("dialogue = %q", outcome.Dialogue)
	}

	var slowbroCount int
	if err := raw.QueryRow(`
		SELECT COUNT(*)
		FROM character_pokemon
		WHERE character_id = 42 AND box = -1 AND pokemon_id = 80`).Scan(&slowbroCount); err != nil {
		t.Fatal(err)
	}
	if slowbroCount != 0 {
		t.Fatalf("slowbro count = %d, want 0", slowbroCount)
	}

	var pokemonID int
	var nickname string
	var originalTrainerID int
	if err := raw.QueryRow(`
		SELECT pokemon_id, nickname, original_trainer_id
		FROM character_pokemon
		WHERE character_id = 42 AND box = -1 AND party_slot = 0`).Scan(&pokemonID, &nickname, &originalTrainerID); err != nil {
		t.Fatal(err)
	}
	if pokemonID != 108 || nickname != "MARC" || originalTrainerID != 900005 {
		t.Fatalf("received pokemon = id %d nickname %q ot %d", pokemonID, nickname, originalTrainerID)
	}

	var completedCount int
	if err := raw.QueryRow(`
		SELECT COUNT(*)
		FROM character_in_game_trades
		WHERE character_id = 42 AND trade_key = 'TRADE_FOR_MARC'`).Scan(&completedCount); err != nil {
		t.Fatal(err)
	}
	if completedCount != 1 {
		t.Fatalf("completed count = %d, want 1", completedCount)
	}

	var seen, caught int
	if err := raw.QueryRow(`
		SELECT seen, caught
		FROM character_pokedex
		WHERE character_id = 42 AND pokemon_id = 108`).Scan(&seen, &caught); err != nil {
		t.Fatal(err)
	}
	if seen != 1 || caught != 1 {
		t.Fatalf("pokedex seen/caught = %d/%d, want 1/1", seen, caught)
	}
}

func TestPerformInGameTradeRequiresRequestedPartyPokemon(t *testing.T) {
	raw := setupInGameTradeTestDB(t, 42, false)

	trade, err := queryInGameTradeDefinitionByText(raw, route18Gate2FYoungsterTextConstant)
	if err != nil {
		t.Fatal(err)
	}

	outcome, err := performInGameTrade(raw, 42, trade)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.WrongPokemon || outcome.Traded {
		t.Fatalf("outcome = %#v", outcome)
	}

	var completedCount int
	if err := raw.QueryRow(`
		SELECT COUNT(*)
		FROM character_in_game_trades
		WHERE character_id = 42 AND trade_key = 'TRADE_FOR_MARC'`).Scan(&completedCount); err != nil {
		t.Fatal(err)
	}
	if completedCount != 0 {
		t.Fatalf("completed count = %d, want 0", completedCount)
	}
}

func TestPerformInGameTradeSelectsRequestedSpeciesFromMixedParty(t *testing.T) {
	raw := setupInGameTradeTestDB(t, 42, true)
	if _, err := raw.Exec(`
		INSERT INTO phaser_pokemon (
			id, name, hp, atk, def, spd, spc, type_1, type_2, catch_rate, base_exp, growth_rate
		) VALUES
			(25, 'PIKACHU', 35, 55, 30, 90, 50, 'ELECTRIC', NULL, 190, 82, 'MEDIUM_FAST');
		UPDATE character_pokemon
		SET party_slot = 1, box_slot = 1
		WHERE character_id = 42 AND pokemon_id = 80;
		INSERT INTO character_pokemon (
			character_id, party_slot, box, box_slot, pokemon_id, nickname,
			level, exp, growth_rate, cur_hp, max_hp,
			iv_atk, iv_def, iv_spd, iv_spc,
			ev_hp, ev_atk, ev_def, ev_spd, ev_spc,
			move1_id, move1_pp, move2_id, move2_pp,
			move3_id, move3_pp, move4_id, move4_pp,
			move1_pp_up, move2_pp_up, move3_pp_up, move4_pp_up,
			status, original_trainer_id
		)
			VALUES (
				42, 0, -1, 0, 25, '',
				12, 0, 'MEDIUM_FAST', 35, 35,
				10, 10, 10, 10,
				0, 0, 0, 0, 0,
				0, 0, 0, 0,
				0, 0, 0, 0,
				0, 0, 0, 0,
				0, 42
			)`); err != nil {
		t.Fatal(err)
	}

	trade, err := queryInGameTradeDefinitionByText(raw, route18Gate2FYoungsterTextConstant)
	if err != nil {
		t.Fatal(err)
	}

	outcome, err := performInGameTrade(raw, 42, trade)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Traded || outcome.WrongPokemon {
		t.Fatalf("outcome = %#v", outcome)
	}

	var pikachuCount, slowbroCount, lickitungCount int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM character_pokemon WHERE character_id = 42 AND box = -1 AND pokemon_id = 25`).Scan(&pikachuCount); err != nil {
		t.Fatal(err)
	}
	if err := raw.QueryRow(`SELECT COUNT(*) FROM character_pokemon WHERE character_id = 42 AND box = -1 AND pokemon_id = 80`).Scan(&slowbroCount); err != nil {
		t.Fatal(err)
	}
	if err := raw.QueryRow(`SELECT COUNT(*) FROM character_pokemon WHERE character_id = 42 AND box = -1 AND pokemon_id = 108`).Scan(&lickitungCount); err != nil {
		t.Fatal(err)
	}
	if pikachuCount != 1 || slowbroCount != 0 || lickitungCount != 1 {
		t.Fatalf("party counts pikachu=%d slowbro=%d lickitung=%d, want 1/0/1", pikachuCount, slowbroCount, lickitungCount)
	}
}

func TestPerformInGameTradeAlreadyCompleted(t *testing.T) {
	raw := setupInGameTradeTestDB(t, 42, true)
	if _, err := raw.Exec(`
		INSERT INTO character_in_game_trades (
			character_id, trade_key, given_pokemon_id, received_pokemon_id, received_nickname
		)
		VALUES (42, 'TRADE_FOR_MARC', 80, 108, 'MARC')`); err != nil {
		t.Fatal(err)
	}

	trade, err := queryInGameTradeDefinitionByText(raw, route18Gate2FYoungsterTextConstant)
	if err != nil {
		t.Fatal(err)
	}

	outcome, err := performInGameTrade(raw, 42, trade)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.AlreadyCompleted || outcome.Traded {
		t.Fatalf("outcome = %#v", outcome)
	}
	if outcome.Dialogue != "Isn't my old\nLICKITUNG great?" {
		t.Fatalf("dialogue = %q", outcome.Dialogue)
	}
}

func TestInGameTradeDialogueSets(t *testing.T) {
	tests := []struct {
		name     string
		trade    inGameTradeDefinition
		no       string
		wrong    string
		complete string
		after    string
	}{
		{
			name: "casual",
			trade: inGameTradeDefinition{
				RequestedPokemonName: "SLOWBRO",
				OfferedPokemonName:   "LICKITUNG",
				DialogueSet:          "CASUAL",
			},
			no:       "Awww!\nOh well...",
			wrong:    "What? That's not\nSLOWBRO!\n\nIf you get one,\ncome back here!",
			complete: "Okay, connect the cable like so!\n\nYou traded SLOWBRO for LICKITUNG!\n\nHey thanks!",
			after:    "Isn't my old\nLICKITUNG great?",
		},
		{
			name: "evolution",
			trade: inGameTradeDefinition{
				RequestedPokemonName: "POLIWHIRL",
				OfferedPokemonName:   "JYNX",
				DialogueSet:          "EVOLUTION",
			},
			no:       "Well, if you\ndon't want to...",
			wrong:    "Hmmm? This isn't\nPOLIWHIRL.\n\nThink of me when\nyou get one.",
			complete: "Okay, connect the cable like so!\n\nYou traded POLIWHIRL for JYNX!\n\nThanks!",
			after:    "The POLIWHIRL you\ntraded to me\n\nwent and evolved!",
		},
		{
			name: "happy",
			trade: inGameTradeDefinition{
				RequestedPokemonName: "SPEAROW",
				OfferedPokemonName:   "FARFETCHD",
				DialogueSet:          "HAPPY",
			},
			no:       "That's too bad.",
			wrong:    "...This is no\nSPEAROW.\n\nIf you get one,\ntrade it with me!",
			complete: "Okay, connect the cable like so!\n\nYou traded SPEAROW for FARFETCHD!\n\nThanks pal!",
			after:    "How is my old\nFARFETCHD?\n\nMy SPEAROW is\ndoing great!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.trade.noTradeDialogue(); got != tt.no {
				t.Fatalf("noTradeDialogue = %q, want %q", got, tt.no)
			}
			if got := tt.trade.wrongPokemonDialogue(); got != tt.wrong {
				t.Fatalf("wrongPokemonDialogue = %q, want %q", got, tt.wrong)
			}
			if got := tt.trade.completedTradeDialogue(); got != tt.complete {
				t.Fatalf("completedTradeDialogue = %q, want %q", got, tt.complete)
			}
			if got := tt.trade.afterTradeDialogue(); got != tt.after {
				t.Fatalf("afterTradeDialogue = %q, want %q", got, tt.after)
			}
		})
	}
}

func setupInGameTradeTestDB(t *testing.T, charID int64, includeSlowbro bool) *sql.DB {
	t.Helper()

	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		CREATE TABLE phaser_in_game_trades (
			trade_key TEXT PRIMARY KEY,
			text_constant TEXT NOT NULL UNIQUE,
			map_name TEXT NOT NULL,
			source_file TEXT NOT NULL,
			script_label TEXT NOT NULL,
			requested_pokemon_id INTEGER NOT NULL,
			requested_pokemon_name TEXT NOT NULL,
			offered_pokemon_id INTEGER NOT NULL,
			offered_pokemon_name TEXT NOT NULL,
			offered_nickname TEXT NOT NULL,
			dialogue_set TEXT NOT NULL,
			original_trade_index INTEGER
		);
		CREATE TABLE character_in_game_trades (
			character_id INTEGER NOT NULL,
			trade_key TEXT NOT NULL,
			given_pokemon_id INTEGER NOT NULL,
			received_pokemon_id INTEGER NOT NULL,
			received_nickname TEXT NOT NULL,
			completed_at TEXT DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (character_id, trade_key)
		);
		CREATE TABLE phaser_pokemon (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			hp INTEGER NOT NULL,
			atk INTEGER NOT NULL,
			def INTEGER NOT NULL,
			spd INTEGER NOT NULL,
			spc INTEGER NOT NULL,
			type_1 TEXT NOT NULL,
			type_2 TEXT,
			catch_rate INTEGER NOT NULL,
			base_exp INTEGER NOT NULL,
			growth_rate TEXT NOT NULL DEFAULT 'MEDIUM_FAST',
			default_move_1_id TEXT,
			default_move_2_id TEXT,
			default_move_3_id TEXT,
			default_move_4_id TEXT,
			base_cry INTEGER,
			cry_pitch INTEGER,
			cry_length INTEGER,
			evolve_level INTEGER,
			evolve_pokemon TEXT
		);
		CREATE TABLE phaser_pokemon_learnset (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pokemon_id INTEGER NOT NULL,
			pokemon_name TEXT NOT NULL,
			level INTEGER NOT NULL,
			move_name TEXT NOT NULL,
			move_id INTEGER NOT NULL
		);
		CREATE TABLE character_pokemon (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			character_id INTEGER NOT NULL,
			party_slot INTEGER,
			box INTEGER NOT NULL DEFAULT -1,
			box_slot INTEGER NOT NULL DEFAULT -1,
			pokemon_id INTEGER NOT NULL,
			nickname TEXT DEFAULT '',
			level INTEGER NOT NULL DEFAULT 5,
			exp INTEGER NOT NULL DEFAULT 0,
			growth_rate TEXT NOT NULL DEFAULT 'MEDIUM_FAST',
			cur_hp INTEGER NOT NULL,
			max_hp INTEGER NOT NULL,
			iv_atk INTEGER NOT NULL DEFAULT 0,
			iv_def INTEGER NOT NULL DEFAULT 0,
			iv_spd INTEGER NOT NULL DEFAULT 0,
			iv_spc INTEGER NOT NULL DEFAULT 0,
			ev_hp INTEGER NOT NULL DEFAULT 0,
			ev_atk INTEGER NOT NULL DEFAULT 0,
			ev_def INTEGER NOT NULL DEFAULT 0,
			ev_spd INTEGER NOT NULL DEFAULT 0,
			ev_spc INTEGER NOT NULL DEFAULT 0,
			move1_id INTEGER NOT NULL DEFAULT 0,
			move1_pp INTEGER NOT NULL DEFAULT 0,
			move1_pp_up INTEGER NOT NULL DEFAULT 0,
			move2_id INTEGER NOT NULL DEFAULT 0,
			move2_pp INTEGER NOT NULL DEFAULT 0,
			move2_pp_up INTEGER NOT NULL DEFAULT 0,
			move3_id INTEGER NOT NULL DEFAULT 0,
			move3_pp INTEGER NOT NULL DEFAULT 0,
			move3_pp_up INTEGER NOT NULL DEFAULT 0,
			move4_id INTEGER NOT NULL DEFAULT 0,
			move4_pp INTEGER NOT NULL DEFAULT 0,
			move4_pp_up INTEGER NOT NULL DEFAULT 0,
			status INTEGER NOT NULL DEFAULT 0,
			original_trainer_id INTEGER
		);
		CREATE TABLE character_pokedex (
			character_id INTEGER NOT NULL,
			pokemon_id INTEGER NOT NULL,
			seen INTEGER NOT NULL DEFAULT 0,
			caught INTEGER NOT NULL DEFAULT 0,
			first_seen_at TEXT,
			first_caught_at TEXT,
			PRIMARY KEY (character_id, pokemon_id)
		);
		INSERT INTO phaser_pokemon (
			id, name, hp, atk, def, spd, spc, type_1, type_2, catch_rate, base_exp, growth_rate
		) VALUES
			(80, 'SLOWBRO', 95, 75, 110, 30, 80, 'WATER', 'PSYCHIC', 75, 164, 'MEDIUM_FAST'),
			(108, 'LICKITUNG', 90, 55, 75, 30, 60, 'NORMAL', NULL, 45, 127, 'MEDIUM_FAST');
		INSERT INTO phaser_in_game_trades (
			trade_key, text_constant, map_name, source_file, script_label,
			requested_pokemon_id, requested_pokemon_name,
			offered_pokemon_id, offered_pokemon_name, offered_nickname,
			dialogue_set, original_trade_index
		) VALUES (
			'TRADE_FOR_MARC', 'TEXT_ROUTE18GATE2F_YOUNGSTER', 'ROUTE_18_GATE_2F',
			'scripts/Route18Gate2F.asm', 'Route18Gate2FYoungsterText',
			80, 'SLOWBRO', 108, 'LICKITUNG', 'MARC', 'CASUAL', 5
		);
	`); err != nil {
		raw.Close()
		t.Fatal(err)
	}

	if includeSlowbro {
		if _, err := raw.Exec(`
			INSERT INTO character_pokemon (
				character_id, party_slot, box, box_slot, pokemon_id, nickname,
				level, exp, growth_rate, cur_hp, max_hp,
				iv_atk, iv_def, iv_spd, iv_spc,
				ev_hp, ev_atk, ev_def, ev_spd, ev_spc,
				move1_id, move1_pp, move2_id, move2_pp,
				move3_id, move3_pp, move4_id, move4_pp,
				move1_pp_up, move2_pp_up, move3_pp_up, move4_pp_up,
				status, original_trainer_id
			)
			VALUES (
				$1, 0, -1, 0, 80, '',
				25, 0, 'MEDIUM_FAST', 80, 80,
				10, 10, 10, 10,
				0, 0, 0, 0, 0,
				0, 0, 0, 0,
				0, 0, 0, 0,
				0, 0, 0, 0,
				0, $1
			)`, charID); err != nil {
			raw.Close()
			t.Fatal(err)
		}
	}

	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: raw}
	t.Cleanup(func() {
		db.GlobalWorldDB = previous
		raw.Close()
	})
	return raw
}
