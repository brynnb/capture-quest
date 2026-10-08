package world

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	db_currency "capturequest/internal/db/currency"
	"capturequest/internal/db/pokedex"
	"capturequest/internal/pokebattle"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
)

// Badge event flag names in gym order.
var badgeFlags = []string{
	"EVENT_GOT_BOULDERBADGE",
	"EVENT_GOT_CASCADEBADGE",
	"EVENT_GOT_THUNDERBADGE",
	"EVENT_GOT_RAINBOWBADGE",
	"EVENT_GOT_SOULBADGE",
	"EVENT_GOT_MARSHBADGE",
	"EVENT_GOT_VOLCANOBADGE",
	"EVENT_GOT_EARTHBADGE",
}

// readPokedexSnapshot shares one budget across the historical owned-species
// repair and the response. Repair is a monotonic write under character ownership;
// the following aggregate is read-only and rejects partial or mixed snapshots.
func readPokedexSnapshot[T any](ctx context.Context, database *sql.DB, charID int64, read func(context.Context, db.ReadDBTX) (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var zero T
	if charID > 0 {
		if err := db.Transaction(ctx, database, func(tx db.DBTX) error {
			if err := db.LockCharacter(tx, charID); err != nil {
				return err
			}
			return reconcileOwnedPokemonPokedex(tx, charID)
		}); err != nil {
			return zero, fmt.Errorf("reconcile pokedex: %w", err)
		}
	}
	return db.ReadSnapshot(ctx, database, read)
}

func readPokedexSpecies(q db.DBTX) ([]protocol.PokedexSpeciesEntry, error) {
	rows, err := q.Query(`SELECT id,name,type_1,type_2,pokedex_type,height,weight,pokedex_text,icon_image,base_cry,cry_pitch,cry_length FROM phaser_pokemon WHERE id BETWEEN 1 AND 151 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	species := make([]protocol.PokedexSpeciesEntry, 0)
	for rows.Next() {
		var s protocol.PokedexSpeciesEntry
		var baseCry, cryPitch, cryLength sql.NullInt64
		if err := rows.Scan(&s.ID, &s.Name, &s.Type1, &s.Type2, &s.PokedexType, &s.Height, &s.Weight, &s.PokedexText, &s.IconImage, &baseCry, &cryPitch, &cryLength); err != nil {
			return nil, err
		}
		if baseCry.Valid {
			value := fmt.Sprintf("SFX_CRY_%02X", baseCry.Int64)
			s.CrySFX = &value
		}
		if cryPitch.Valid {
			value := int(cryPitch.Int64)
			s.CryPitch = &value
		}
		if cryLength.Valid {
			value := int(cryLength.Int64)
			s.CryLength = &value
		}
		species = append(species, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return species, nil
}

func readPokedexStatus(q db.DBTX, charID int64) ([]protocol.PokedexStatusEntry, error) {
	status := make([]protocol.PokedexStatusEntry, 0)
	if charID == 0 {
		return status, nil
	}
	rows, err := q.Query(`SELECT pokemon_id,seen,caught FROM character_pokedex WHERE character_id=$1 ORDER BY pokemon_id`, charID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var entry protocol.PokedexStatusEntry
		if err := rows.Scan(&entry.PokemonID, &entry.Seen, &entry.Caught); err != nil {
			return nil, err
		}
		status = append(status, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return status, nil
}

// HandlePokedexListRequest retains character-select catalog access with no status.
func HandlePokedexListRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var charID int64
	if ses.HasValidClient() {
		charID = int64(ses.Client.CharData().ID)
	}
	result, err := readPokedexSnapshot(ses.CommandContext(), wh.database, charID, func(_ context.Context, q db.ReadDBTX) (protocol.PokedexListResponse, error) {
		species, err := readPokedexSpecies(q)
		if err != nil {
			return protocol.PokedexListResponse{}, fmt.Errorf("pokedex species: %w", err)
		}
		status, err := readPokedexStatus(q, charID)
		if err != nil {
			return protocol.PokedexListResponse{}, fmt.Errorf("pokedex status: %w", err)
		}
		return protocol.PokedexListResponse{Success: true, Species: species, Status: status}, nil
	})
	if err != nil {
		ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.PokedexListResponse)
		return false
	}
	ses.SendStreamJSON(result, opcodes.PokedexListResponse)
	return false
}

func HandlePokedexStatusRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() || ses.Client.CharData().ID == 0 {
		ses.SendStreamJSON(protocol.ErrorResponse{Error: "not logged in"}, opcodes.PokedexStatusResponse)
		return false
	}
	charID := int64(ses.Client.CharData().ID)
	result, err := readPokedexSnapshot(ses.CommandContext(), wh.database, charID, func(_ context.Context, q db.ReadDBTX) (protocol.PokedexStatusResponse, error) {
		status, err := readPokedexStatus(q, charID)
		return protocol.PokedexStatusResponse{Success: true, Status: status}, err
	})
	if err != nil {
		ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.PokedexStatusResponse)
		return false
	}
	ses.SendStreamJSON(result, opcodes.PokedexStatusResponse)
	return false
}

func HandleTrainerCardRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	sendTrainerCardResponse(ses, wh)
	return false
}

func sendTrainerCardResponse(ses *session.Session, wh *WorldHandler) {
	if !ses.HasValidClient() || ses.Client.CharData().ID == 0 {
		ses.SendStreamJSON(protocol.ErrorResponse{Error: "not logged in"}, opcodes.TrainerCardResponse)
		return
	}
	char := ses.Client.CharData()
	name, charID, playtime := char.Name, int64(char.ID), int(ses.CurrentPlaytime(time.Now()))
	result, err := readPokedexSnapshot(ses.CommandContext(), wh.database, charID, func(ctx context.Context, q db.ReadDBTX) (protocol.TrainerCardResponse, error) {
		card := protocol.TrainerCardResponse{Success: true, Name: name, TimePlayed: playtime, Badges: make([]string, 0)}
		wallet, err := db_currency.GetCharacterWalletContext(ctx, q, uint32(charID))
		if err != nil {
			return protocol.TrainerCardResponse{}, fmt.Errorf("trainer wallet: %w", err)
		}
		card.Money = int(wallet.Pokedollars)
		flags, err := eventFlagSnapshotIn(q, charID)
		if err != nil {
			return protocol.TrainerCardResponse{}, fmt.Errorf("trainer flags: %w", err)
		}
		for _, flag := range badgeFlags {
			if flags.CheckFlag(charID, flag) {
				card.Badges = append(card.Badges, flag)
			}
		}
		card.BadgeCount = len(card.Badges)
		if err := q.QueryRow(`SELECT COALESCE(SUM(seen),0),COALESCE(SUM(caught),0) FROM character_pokedex WHERE character_id=$1`, charID).Scan(&card.PokedexSeen, &card.PokedexCaught); err != nil {
			return protocol.TrainerCardResponse{}, fmt.Errorf("trainer counts: %w", err)
		}
		return card, nil
	})
	if err != nil {
		ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.TrainerCardResponse)
		return
	}
	ses.SendStreamJSON(result, opcodes.TrainerCardResponse)
}

// MarkPokemonSeen marks a Pokémon as seen in the character's Pokédex.
// Called when a wild/trainer battle starts.
func MarkPokemonSeen(charID int64, pokemonID int) {
	err := markPokemonSeen(db.GlobalWorldDB.DB, charID, pokemonID)
	if err != nil {
		log.Printf("[Pokedex] Error marking pokemon %d seen for char %d: %v", pokemonID, charID, err)
	} else {
		log.Printf("[Pokedex] Marked pokemon %d as seen for char %d", pokemonID, charID)
	}
}

func markPokemonSeen(database pokebattle.DBTX, charID int64, pokemonID int) error {
	if charID <= 0 || pokemonID < 1 || pokemonID > 151 {
		return fmt.Errorf("invalid pokedex identity character=%d pokemon=%d", charID, pokemonID)
	}
	_, err := database.Exec(`
		INSERT INTO character_pokedex (character_id, pokemon_id, seen, first_seen_at)
		VALUES ($1, $2, 1, CURRENT_TIMESTAMP)
		ON CONFLICT (character_id, pokemon_id) DO UPDATE SET
			seen = 1,
			first_seen_at = COALESCE(character_pokedex.first_seen_at, CURRENT_TIMESTAMP)`,
		charID, pokemonID)
	return err
}

// MarkPokemonCaught marks a Pokémon as caught (and seen) in the character's Pokédex.
// Called when a Pokémon is successfully caught.
func MarkPokemonCaught(charID int64, pokemonID int) {
	if err := pokedex.MarkCaught(db.GlobalWorldDB.DB, charID, pokemonID); err != nil {
		log.Printf("[Pokedex] Error marking pokemon %d caught for char %d: %v", pokemonID, charID, err)
	}
}

// reconcileOwnedPokemonPokedex enforces the Gen I invariant that every species
// currently owned by the trainer is both seen and caught. It also repairs saves
// created before all acquisition paths updated the Pokédex directly.
func reconcileOwnedPokemonPokedex(database pokebattle.DBTX, charID int64) error {
	if charID <= 0 {
		return fmt.Errorf("invalid pokedex character=%d", charID)
	}
	_, err := database.Exec(`
		INSERT INTO character_pokedex (
			character_id, pokemon_id, seen, caught, first_seen_at, first_caught_at
		)
		SELECT $1, pokemon_id, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		FROM character_pokemon
		WHERE character_id = $1 AND pokemon_id BETWEEN 1 AND 151
		GROUP BY pokemon_id
		ON CONFLICT (character_id, pokemon_id) DO UPDATE SET
			seen = 1,
			caught = 1,
			first_seen_at = COALESCE(character_pokedex.first_seen_at, EXCLUDED.first_seen_at),
			first_caught_at = COALESCE(character_pokedex.first_caught_at, EXCLUDED.first_caught_at)
		WHERE character_pokedex.seen <> 1 OR character_pokedex.caught <> 1
		   OR character_pokedex.first_seen_at IS NULL OR character_pokedex.first_caught_at IS NULL`, charID)
	return err
}
