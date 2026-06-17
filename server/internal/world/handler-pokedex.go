package world

import (
	"database/sql"
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/session"
)

// --- Response types ---

type PokedexSpeciesEntry struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Type1       string  `json:"type1"`
	Type2       *string `json:"type2"`
	PokedexType *string `json:"pokedexType"`
	Height      *string `json:"height"`
	Weight      *int    `json:"weight"`
	PokedexText *string `json:"pokedexText"`
	IconImage   *string `json:"iconImage"`
	CrySFX      *string `json:"crySfx,omitempty"`
	CryPitch    *int    `json:"cryPitch,omitempty"`
	CryLength   *int    `json:"cryLength,omitempty"`
}

type PokedexStatusEntry struct {
	PokemonID int  `json:"pokemonId"`
	Seen      bool `json:"seen"`
	Caught    bool `json:"caught"`
}

type TrainerCardResponse struct {
	Name          string   `json:"name"`
	Money         int      `json:"money"`
	TimePlayed    int      `json:"timePlayed"`
	Badges        []string `json:"badges"`
	BadgeCount    int      `json:"badgeCount"`
	PokedexSeen   int      `json:"pokedexSeen"`
	PokedexCaught int      `json:"pokedexCaught"`
}

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
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT id, name, type_1, type_2, pokedex_type, height, weight, pokedex_text, icon_image,
		       base_cry, cry_pitch, cry_length
		FROM phaser_pokemon WHERE id BETWEEN 1 AND 151 ORDER BY id`)
	if err != nil {
		log.Printf("[Pokedex] Error querying species: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PokedexListResponse)
		return false
	}
	defer rows.Close()

	var species []PokedexSpeciesEntry
	for rows.Next() {
		var s PokedexSpeciesEntry
		var baseCry, cryPitch, cryLength sql.NullInt64
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Type1, &s.Type2, &s.PokedexType,
			&s.Height, &s.Weight, &s.PokedexText, &s.IconImage,
			&baseCry, &cryPitch, &cryLength,
		); err != nil {
			log.Printf("[Pokedex] Error scanning species: %v", err)
			continue
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

	// Fetch seen/caught status for this character
	var status []PokedexStatusEntry
	if charID > 0 {
		statusRows, err := db.GlobalWorldDB.DB.Query(`
			SELECT pokemon_id, seen, caught
			FROM character_pokedex WHERE character_id = $1 ORDER BY pokemon_id`, charID)
		if err != nil {
			log.Printf("[Pokedex] Error querying status for char %d: %v", charID, err)
		} else {
			defer statusRows.Close()
			for statusRows.Next() {
				var e PokedexStatusEntry
				if err := statusRows.Scan(&e.PokemonID, &e.Seen, &e.Caught); err != nil {
					continue
				}
				status = append(status, e)
			}
		}
	}

	res := map[string]interface{}{
		"success": true,
		"species": StructToMap(species),
		"status":  StructToMap(status),
	}
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
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "not logged in"}, opcodes.PokedexStatusResponse)
		return false
	}

	statusRows, err := db.GlobalWorldDB.DB.Query(`
		SELECT pokemon_id, seen, caught
		FROM character_pokedex WHERE character_id = $1 ORDER BY pokemon_id`, charID)
	if err != nil {
		log.Printf("[Pokedex] Error querying status for char %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PokedexStatusResponse)
		return false
	}
	defer statusRows.Close()

	var status []PokedexStatusEntry
	for statusRows.Next() {
		var e PokedexStatusEntry
		if err := statusRows.Scan(&e.PokemonID, &e.Seen, &e.Caught); err != nil {
			continue
		}
		status = append(status, e)
	}

	res := map[string]interface{}{
		"success": true,
		"status":  StructToMap(status),
	}
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
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "not logged in"}, opcodes.TrainerCardResponse)
		return
	}

	charData := ses.Client.CharData()
	charID := int64(charData.ID)

	// Build trainer card
	card := TrainerCardResponse{
		Name:       charData.Name,
		TimePlayed: int(charData.TimePlayed),
	}

	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT COALESCE(pokedollars, 0) FROM character_wallet WHERE character_id = $1`, charData.ID).Scan(&card.Money)
	if err != nil {
		log.Printf("[TrainerCard] Error querying money for char %d: %v", charID, err)
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
	err = db.GlobalWorldDB.DB.QueryRow(`
		SELECT COALESCE(SUM(seen), 0), COALESCE(SUM(caught), 0)
		FROM character_pokedex WHERE character_id = $1`, charID).Scan(&card.PokedexSeen, &card.PokedexCaught)
	if err != nil {
		log.Printf("[TrainerCard] Error querying pokedex counts for char %d: %v", charID, err)
	}

	res := StructToMap(card).(map[string]interface{})
	res["success"] = true
	ses.SendStreamJSON(res, opcodes.TrainerCardResponse)
	log.Printf("[TrainerCard] Sent card for %s: %d badges, %d seen, %d caught",
		card.Name, card.BadgeCount, card.PokedexSeen, card.PokedexCaught)
}

// MarkPokemonSeen marks a Pokémon as seen in the character's Pokédex.
// Called when a wild/trainer battle starts.
func MarkPokemonSeen(charID int64, pokemonID int) {
	_, err := db.GlobalWorldDB.DB.Exec(`
		INSERT INTO character_pokedex (character_id, pokemon_id, seen, first_seen_at)
		VALUES ($1, $2, 1, NOW())
		ON CONFLICT (character_id, pokemon_id) DO UPDATE SET
			seen = 1,
			first_seen_at = COALESCE(character_pokedex.first_seen_at, CURRENT_TIMESTAMP)`,
		charID, pokemonID)
	if err != nil {
		log.Printf("[Pokedex] Error marking pokemon %d seen for char %d: %v", pokemonID, charID, err)
	} else {
		log.Printf("[Pokedex] Marked pokemon %d as seen for char %d", pokemonID, charID)
	}
}

// MarkPokemonCaught marks a Pokémon as caught (and seen) in the character's Pokédex.
// Called when a Pokémon is successfully caught.
func MarkPokemonCaught(charID int64, pokemonID int) {
	_, err := db.GlobalWorldDB.DB.Exec(`
		INSERT INTO character_pokedex (character_id, pokemon_id, seen, caught, first_seen_at, first_caught_at)
		VALUES ($1, $2, 1, 1, NOW(), NOW())
		ON CONFLICT (character_id, pokemon_id) DO UPDATE SET
			seen = 1,
			caught = 1,
			first_seen_at = COALESCE(character_pokedex.first_seen_at, EXCLUDED.first_seen_at),
			first_caught_at = COALESCE(character_pokedex.first_caught_at, EXCLUDED.first_caught_at)`,
		charID, pokemonID)
	if err != nil {
		log.Printf("[Pokedex] Error marking pokemon %d caught for char %d: %v", pokemonID, charID, err)
	}
}

// StructToMap helper is already defined in handler-phaser-data.go, reuse it.
// We reference it here but it's defined elsewhere in the package.
// (Go allows this since both files are in the same package.)
