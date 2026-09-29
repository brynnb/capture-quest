package world

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
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

// HandlePokedexListRequest returns all 151 Pokémon species data + the player's seen/caught status.
func HandlePokedexListRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var charID int64
	if ses.HasValidClient() {
		charID = int64(ses.Client.CharData().ID)
	}

	// Fetch all Pokémon species (1-151)
	rows, err := wh.database.Query(`
		SELECT id, name, type_1, type_2, pokedex_type, height, weight, pokedex_text, icon_image,
		       base_cry, cry_pitch, cry_length
		FROM phaser_pokemon WHERE id BETWEEN 1 AND 151 ORDER BY id`)
	if err != nil {
		log.Printf("[Pokedex] Error querying species: %v", err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.PokedexListResponse)
		return false
	}
	defer rows.Close()

	species := make([]protocol.PokedexSpeciesEntry, 0)
	for rows.Next() {
		var s protocol.PokedexSpeciesEntry
		var baseCry, cryPitch, cryLength sql.NullInt64
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Type1, &s.Type2, &s.PokedexType,
			&s.Height, &s.Weight, &s.PokedexText, &s.IconImage,
			&baseCry, &cryPitch, &cryLength,
		); err != nil {
			log.Printf("[Pokedex] Error scanning species: %v", err)
			ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.PokedexListResponse)
			return false
		}
		if baseCry.Valid {
			crySFX := fmt.Sprintf("SFX_CRY_%02X", baseCry.Int64)
			s.CrySFX = &crySFX
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
		ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.PokedexListResponse)
		return false
	}

	// Fetch seen/caught status for this character
	status := make([]protocol.PokedexStatusEntry, 0)
	if charID > 0 {
		if err := reconcileOwnedPokemonPokedex(wh.database, charID); err != nil {
			log.Printf("[Pokedex] Error reconciling owned pokemon for char %d: %v", charID, err)
			ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.PokedexListResponse)
			return false
		}
		statusRows, err := wh.database.Query(`
			SELECT pokemon_id, seen, caught
			FROM character_pokedex WHERE character_id = $1 ORDER BY pokemon_id`, charID)
		if err != nil {
			log.Printf("[Pokedex] Error querying status for char %d: %v", charID, err)
			ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.PokedexListResponse)
			return false
		} else {
			defer statusRows.Close()
			for statusRows.Next() {
				var e protocol.PokedexStatusEntry
				if err := statusRows.Scan(&e.PokemonID, &e.Seen, &e.Caught); err != nil {
					ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.PokedexListResponse)
					return false
				}
				status = append(status, e)
			}
			if err := statusRows.Err(); err != nil {
				ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.PokedexListResponse)
				return false
			}
		}
	}

	res := protocol.PokedexListResponse{Success: true, Species: species, Status: status}
	ses.SendStreamJSON(res, opcodes.PokedexListResponse)
	log.Printf("[Pokedex] Sent %d species + %d status entries for char %d", len(species), len(status), charID)
	return false
}

// HandlePokedexStatusRequest returns just the seen/caught status (lightweight refresh).
func HandlePokedexStatusRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var charID int64
	if ses.HasValidClient() {
		charID = int64(ses.Client.CharData().ID)
	}
	if charID == 0 {
		ses.SendStreamJSON(protocol.ErrorResponse{Error: "not logged in"}, opcodes.PokedexStatusResponse)
		return false
	}
	if err := reconcileOwnedPokemonPokedex(wh.database, charID); err != nil {
		log.Printf("[Pokedex] Error reconciling owned pokemon for char %d: %v", charID, err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.PokedexStatusResponse)
		return false
	}

	statusRows, err := wh.database.Query(`
		SELECT pokemon_id, seen, caught
		FROM character_pokedex WHERE character_id = $1 ORDER BY pokemon_id`, charID)
	if err != nil {
		log.Printf("[Pokedex] Error querying status for char %d: %v", charID, err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.PokedexStatusResponse)
		return false
	}
	defer statusRows.Close()

	status := make([]protocol.PokedexStatusEntry, 0)
	for statusRows.Next() {
		var e protocol.PokedexStatusEntry
		if err := statusRows.Scan(&e.PokemonID, &e.Seen, &e.Caught); err != nil {
			ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.PokedexStatusResponse)
			return false
		}
		status = append(status, e)
	}

	if err := statusRows.Err(); err != nil {
		ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.PokedexStatusResponse)
		return false
	}
	res := protocol.PokedexStatusResponse{Success: true, Status: status}
	ses.SendStreamJSON(res, opcodes.PokedexStatusResponse)
	return false
}

// HandleTrainerCardRequest returns trainer card data: name, badges, play time, money, pokédex counts.
func HandleTrainerCardRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	sendTrainerCardResponse(ses, wh)
	return false
}

func sendTrainerCardResponse(ses *session.Session, wh *WorldHandler) {
	if !ses.HasValidClient() {
		ses.SendStreamJSON(protocol.ErrorResponse{Error: "not logged in"}, opcodes.TrainerCardResponse)
		return
	}

	charData := ses.Client.CharData()
	charID := int64(charData.ID)
	if err := reconcileOwnedPokemonPokedex(wh.database, charID); err != nil {
		log.Printf("[TrainerCard] Error reconciling owned pokemon for char %d: %v", charID, err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.TrainerCardResponse)
		return
	}

	// Build trainer card
	card := protocol.TrainerCardResponse{
		Success:    true,
		Badges:     make([]string, 0),
		Name:       charData.Name,
		TimePlayed: int(ses.CurrentPlaytime(time.Now())),
	}

	err := wh.database.QueryRow(`
		SELECT COALESCE(pokedollars, 0) FROM character_wallet WHERE character_id = $1`, charData.ID).Scan(&card.Money)
	if err != nil {
		log.Printf("[TrainerCard] Error querying money for char %d: %v", charID, err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.TrainerCardResponse)
		return
	}

	// Get badges from event flags
	if wh.EventFlags != nil {
		for _, flag := range badgeFlags {
			if wh.EventFlags.CheckFlag(charID, flag) {
				card.Badges = append(card.Badges, flag)
			}
		}
		card.BadgeCount = len(card.Badges)
	}

	// Get pokédex counts
	err = wh.database.QueryRow(`
		SELECT COALESCE(SUM(seen), 0), COALESCE(SUM(caught), 0)
		FROM character_pokedex WHERE character_id = $1`, charID).Scan(&card.PokedexSeen, &card.PokedexCaught)
	if err != nil {
		log.Printf("[TrainerCard] Error querying pokedex counts for char %d: %v", charID, err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: err.Error()}, opcodes.TrainerCardResponse)
		return
	}

	ses.SendStreamJSON(card, opcodes.TrainerCardResponse)
	log.Printf("[TrainerCard] Sent card for %s: %d badges, %d seen, %d caught",
		card.Name, card.BadgeCount, card.PokedexSeen, card.PokedexCaught)
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
			first_caught_at = COALESCE(character_pokedex.first_caught_at, EXCLUDED.first_caught_at)`, charID)
	return err
}

// We reference it here but it's defined elsewhere in the package.
// (Go allows this since both files are in the same package.)
