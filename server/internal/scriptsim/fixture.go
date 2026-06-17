package scriptsim

import (
	"database/sql"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
	"capturequest/internal/world"
)

type AppliedFixture struct {
	CharacterID int64
	MapID       int
}

func ApplyFixture(f Fixture) (*AppliedFixture, error) {
	if f.CharacterName == "" {
		return nil, fmt.Errorf("fixture characterName is required")
	}
	mapID, err := mapIDForName(f.MapName)
	if err != nil {
		return nil, err
	}
	charID, err := ensureCharacter(f.CharacterName, mapID, f.X, f.Y)
	if err != nil {
		return nil, err
	}

	if err := resetCharacterState(charID); err != nil {
		return nil, err
	}
	heading, err := headingForFixtureDirection(f.Direction)
	if err != nil {
		return nil, err
	}
	if _, err := db.GlobalWorldDB.DB.Exec(
		`UPDATE character_data SET map_id = $1, x = $2, y = $3, heading = $4 WHERE id = $5`,
		mapID, f.X, f.Y, heading, charID); err != nil {
		return nil, fmt.Errorf("set fixture position: %w", err)
	}
	for _, flag := range f.Flags {
		if _, err := db.GlobalWorldDB.DB.Exec(
			`INSERT INTO character_event_flags (character_id, flag_name)
			VALUES ($1, $2)
			ON CONFLICT (character_id, flag_name) DO NOTHING`,
			charID, flag); err != nil {
			return nil, fmt.Errorf("seed flag %s: %w", flag, err)
		}
	}
	for _, p := range f.Party {
		level := p.Level
		if level <= 0 {
			level = 5
		}
		if _, _, _, err := pokebattle.AddPokemonToPartyOrPC(db.GlobalWorldDB.DB, charID, p.SpeciesID, level); err != nil {
			return nil, fmt.Errorf("seed pokemon %d L%d: %w", p.SpeciesID, level, err)
		}
		if err := seedFixturePokemonMoves(charID, p); err != nil {
			return nil, fmt.Errorf("seed pokemon %d moves: %w", p.SpeciesID, err)
		}
	}
	if err := seedFixturePartyPokemonDetails(charID, f.Party); err != nil {
		return nil, err
	}
	for _, p := range f.PC {
		if err := seedFixturePCPokemon(charID, p); err != nil {
			return nil, err
		}
	}
	if f.DayCare != nil {
		if err := seedFixtureDayCare(charID, *f.DayCare); err != nil {
			return nil, err
		}
	}
	for _, item := range f.Inventory {
		itemID, err := resolveFixtureItemID(item)
		if err != nil {
			return nil, err
		}
		quantity := item.Quantity
		if quantity <= 0 {
			quantity = 1
		}
		if _, err := cqitems.AddItemToInventory(int32(charID), int32(itemID), uint16(quantity)); err != nil {
			return nil, fmt.Errorf("seed item %d x%d: %w", itemID, quantity, err)
		}
	}
	if f.Money > 0 {
		if _, err := db.GlobalWorldDB.DB.Exec(`
			INSERT INTO character_wallet (character_id, pokedollars)
			VALUES ($1, $2)
			ON CONFLICT (character_id) DO UPDATE SET
				pokedollars = EXCLUDED.pokedollars`,
			charID, f.Money); err != nil {
			return nil, fmt.Errorf("seed money %d: %w", f.Money, err)
		}
	}
	if f.Coins > 0 {
		if _, err := db.GlobalWorldDB.DB.Exec(
			`INSERT INTO character_coins (character_id, coins) VALUES ($1, $2)
			ON CONFLICT (character_id) DO UPDATE SET coins = EXCLUDED.coins`,
			charID, f.Coins); err != nil {
			return nil, fmt.Errorf("seed coins %d: %w", f.Coins, err)
		}
	}
	for _, pokemonID := range f.PokedexSeen {
		if err := seedPokedexEntry(charID, pokemonID, false); err != nil {
			return nil, err
		}
	}
	for _, pokemonID := range f.PokedexCaught {
		if err := seedPokedexEntry(charID, pokemonID, true); err != nil {
			return nil, err
		}
	}
	for _, key := range f.HiddenObjects {
		ids, err := world.ResolveCutsceneObjectKey(f.MapName, key)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if _, err := db.GlobalWorldDB.DB.Exec(
				`INSERT INTO character_collected_items (character_id, object_id)
				VALUES ($1, $2)
				ON CONFLICT (character_id, object_id) DO NOTHING`,
				charID, id); err != nil {
				return nil, fmt.Errorf("seed hidden object %s/%d: %w", key, id, err)
			}
		}
	}
	for _, pos := range f.ObjectPositions {
		if err := seedObjectPosition(charID, f.MapName, pos); err != nil {
			return nil, err
		}
	}
	if f.ActiveBattle != nil {
		if err := seedFixtureActiveBattle(charID, *f.ActiveBattle); err != nil {
			return nil, err
		}
	}
	if f.VermilionGymTrash != nil {
		if f.VermilionGymTrash.FirstLockCanIndex == nil {
			return nil, fmt.Errorf("vermilionGymTrash fixture requires firstLockCanIndex")
		}
		if err := world.SetVermilionGymTrashState(charID, world.VermilionGymTrashState{
			FirstLockCanIndex:  *f.VermilionGymTrash.FirstLockCanIndex,
			SecondLockCanIndex: f.VermilionGymTrash.SecondLockCanIndex,
		}); err != nil {
			return nil, fmt.Errorf("seed Vermilion Gym trash state: %w", err)
		}
	}
	return &AppliedFixture{CharacterID: charID, MapID: mapID}, nil
}

func seedPokedexEntry(charID int64, pokemonID int, caught bool) error {
	if pokemonID <= 0 {
		return fmt.Errorf("seed pokedex pokemon id must be positive")
	}
	if caught {
		if _, err := db.GlobalWorldDB.DB.Exec(`
			INSERT INTO character_pokedex (character_id, pokemon_id, seen, caught, first_seen_at, first_caught_at)
			VALUES ($1, $2, 1, 1, NOW(), NOW())
			ON CONFLICT (character_id, pokemon_id) DO UPDATE SET
				seen = 1,
				caught = 1,
				first_seen_at = COALESCE(character_pokedex.first_seen_at, EXCLUDED.first_seen_at),
				first_caught_at = COALESCE(character_pokedex.first_caught_at, EXCLUDED.first_caught_at)`,
			charID, pokemonID); err != nil {
			return fmt.Errorf("seed caught pokedex %d: %w", pokemonID, err)
		}
		return nil
	}
	if _, err := db.GlobalWorldDB.DB.Exec(`
		INSERT INTO character_pokedex (character_id, pokemon_id, seen, caught, first_seen_at)
		VALUES ($1, $2, 1, 0, NOW())
		ON CONFLICT (character_id, pokemon_id) DO UPDATE SET
			seen = 1,
			first_seen_at = COALESCE(character_pokedex.first_seen_at, EXCLUDED.first_seen_at)`,
		charID, pokemonID); err != nil {
		return fmt.Errorf("seed seen pokedex %d: %w", pokemonID, err)
	}
	return nil
}

func resolveFixtureItemID(item FixtureItem) (int, error) {
	if item.ItemID > 0 {
		return item.ItemID, nil
	}
	if item.ItemName == "" {
		return 0, fmt.Errorf("fixture item requires itemId or itemName")
	}
	var id int
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT id FROM cq_items WHERE name = $1 OR short_name = $2 LIMIT 1`,
		item.ItemName, item.ItemName).Scan(&id); err != nil {
		return 0, fmt.Errorf("lookup item %s: %w", item.ItemName, err)
	}
	return id, nil
}

func ensureCharacter(name string, mapID, x, y int) (int64, error) {
	var id int64
	err := db.GlobalWorldDB.DB.QueryRow(`SELECT id FROM character_data WHERE name = $1`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("lookup fixture character: %w", err)
	}

	err = db.GlobalWorldDB.DB.QueryRow(`
		INSERT INTO character_data
			(account_id, name, map_id, x, y, z, heading, gender, faction_id, class, level)
		VALUES
			(0, $1, $2, $3, $4, 0, 0, 0, 1, 1, 1)
		RETURNING id`,
		name, mapID, x, y).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create fixture character %s: %w", name, err)
	}
	return id, nil
}

func resetCharacterState(charID int64) error {
	world.ClearBattleForCharacter(charID)
	statements := []string{
		`DELETE FROM character_daycare WHERE character_id = $1`,
		`DELETE FROM character_event_flags WHERE character_id = $1`,
		`DELETE FROM character_in_game_trades WHERE character_id = $1`,
		`DELETE FROM character_pokemon WHERE character_id = $1`,
		`DELETE FROM character_field_move_state WHERE character_id = $1`,
		`DELETE FROM character_object_positions WHERE character_id = $1`,
		`DELETE FROM character_object_visibility_overrides WHERE character_id = $1`,
		`DELETE FROM character_collected_items WHERE character_id = $1`,
		`DELETE FROM character_collected_hidden_coins WHERE character_id = $1`,
		`DELETE FROM character_pokedex WHERE character_id = $1`,
		`DELETE FROM character_coins WHERE character_id = $1`,
		`DELETE FROM character_pc_state WHERE character_id = $1`,
		`DELETE FROM character_vermilion_gym_trash_state WHERE character_id = $1`,
		`DELETE FROM character_wallet WHERE character_id = $1`,
		`DELETE FROM cq_item_instances WHERE id IN (SELECT item_instance_id FROM cq_character_inventory WHERE character_id = $1)`,
		`DELETE FROM cq_character_inventory WHERE character_id = $1`,
		`DELETE FROM character_battle_state WHERE character_id = $1`,
	}
	for _, stmt := range statements {
		if _, err := db.GlobalWorldDB.DB.Exec(stmt, charID); err != nil {
			return fmt.Errorf("reset fixture state: %w", err)
		}
	}
	return nil
}

func mapIDForName(mapName string) (int, error) {
	if mapName == "" {
		return 0, fmt.Errorf("mapName is required")
	}
	var id int
	var isOverworld bool
	if err := db.GlobalWorldDB.DB.QueryRow(`SELECT id, is_overworld FROM phaser_maps WHERE name = $1`, mapName).Scan(&id, &isOverworld); err != nil {
		return 0, fmt.Errorf("lookup map %s: %w", mapName, err)
	}
	if isOverworld && id != 9999 {
		return 9999, nil
	}
	return id, nil
}

func mapNameForID(mapID int) (string, error) {
	var name string
	if err := db.GlobalWorldDB.DB.QueryRow(`SELECT name FROM phaser_maps WHERE id = $1`, mapID).Scan(&name); err != nil {
		return "", fmt.Errorf("lookup map id %d: %w", mapID, err)
	}
	return name, nil
}

func isOverworldMapName(mapName string) bool {
	if mapName == "" {
		return false
	}
	var isOverworld bool
	if err := db.GlobalWorldDB.DB.QueryRow(`SELECT is_overworld FROM phaser_maps WHERE name = $1`, mapName).Scan(&isOverworld); err != nil {
		return false
	}
	return isOverworld
}
