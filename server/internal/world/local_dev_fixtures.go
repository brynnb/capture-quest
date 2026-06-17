package world

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"capturequest/internal/config"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
)

const (
	localDevStackableItemCount    = 5
	localDevNonStackableItemCount = 1
)

type localDevPokemonSpec struct {
	speciesID int
	level     int
	setup     func(*pokebattle.Pokemon)
}

func ensureLocalDevFixtures(charID int64) {
	cfg, err := config.Get()
	if err != nil {
		log.Printf("[LocalDevFixtures] Failed to read config: %v", err)
		return
	}
	if !cfg.Local || !localDevFixturesEnabled() {
		return
	}

	if err := ensureLocalDevPokemonParty(db.GlobalWorldDB.DB, charID); err != nil {
		log.Printf("[LocalDevFixtures] Failed to seed test party for char %d: %v", charID, err)
	}
	if err := ensureLocalDevInventory(db.GlobalWorldDB.DB, charID); err != nil {
		log.Printf("[LocalDevFixtures] Failed to seed test inventory for char %d: %v", charID, err)
	}
}

func localDevFixturesEnabled() bool {
	return captureQuestTestModeEnabled() ||
		os.Getenv("CAPTUREQUEST_LOCAL_FIXTURES") == "true"
}

func captureQuestTestModeEnabled() bool {
	return os.Getenv("CAPTUREQUEST_TEST_MODE") == "true"
}

func ensureLocalDevPokemonParty(myDB *sql.DB, charID int64) error {
	specs := []localDevPokemonSpec{
		{speciesID: 4, level: 5}, // Charmander starter, healthy.
		{speciesID: 25, level: 12, setup: func(p *pokebattle.Pokemon) {
			p.CurHP = clampDevHP(p.MaxHP / 3)
			p.Status = pokebattle.StatusParalyze
		}},
		{speciesID: 7, level: 12, setup: func(p *pokebattle.Pokemon) {
			p.CurHP = 0
			p.Status = pokebattle.StatusNone
		}},
		{speciesID: 1, level: 12, setup: func(p *pokebattle.Pokemon) {
			p.Status = pokebattle.StatusPoison
		}},
		{speciesID: 16, level: 18, setup: func(p *pokebattle.Pokemon) {
			for i := range p.Moves {
				if p.Moves[i].ID > 0 {
					p.Moves[i].PP = 0
				}
			}
		}},
		{speciesID: 39, level: 12, setup: func(p *pokebattle.Pokemon) {
			p.CurHP = clampDevHP(p.MaxHP / 2)
			p.Status = pokebattle.StatusSleep
			p.SleepTurns = 3
		}},
	}

	party := make([]*pokebattle.Pokemon, 0, len(specs))
	for _, spec := range specs {
		p, err := pokebattle.BuildWildPokemon(myDB, spec.speciesID, spec.level)
		if err != nil {
			return fmt.Errorf("build species %d L%d: %w", spec.speciesID, spec.level, err)
		}
		p.IsWild = false
		if spec.setup != nil {
			spec.setup(p)
		}
		party = append(party, p)
	}

	if err := pokebattle.SaveParty(myDB, charID, party); err != nil {
		return fmt.Errorf("save local dev party: %w", err)
	}
	return nil
}

func ensureLocalDevInventory(myDB *sql.DB, charID int64) error {
	tx, err := myDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	templates, err := loadLocalDevItemTemplates(tx)
	if err != nil {
		return err
	}
	counts, err := loadLocalDevItemCounts(tx, charID)
	if err != nil {
		return err
	}

	for _, item := range templates {
		count := counts[item.id]
		if item.stackable {
			if count.quantity >= localDevStackableItemCount {
				continue
			}
			if err := createLocalDevInventoryItem(tx, charID, item.id, localDevStackableItemCount-count.quantity); err != nil {
				return err
			}
			continue
		}

		for count.instances < localDevNonStackableItemCount {
			if err := createLocalDevInventoryItem(tx, charID, item.id, 1); err != nil {
				return err
			}
			count.instances++
		}
	}

	return tx.Commit()
}

type localDevItemTemplate struct {
	id        int32
	stackable bool
}

type localDevItemTotals struct {
	instances int
	quantity  int
}

func loadLocalDevItemTemplates(tx *sql.Tx) ([]localDevItemTemplate, error) {
	rows, err := tx.Query(`SELECT id, stackable FROM cq_items ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query cq_items: %w", err)
	}
	defer rows.Close()

	var items []localDevItemTemplate
	for rows.Next() {
		var item localDevItemTemplate
		if err := rows.Scan(&item.id, &item.stackable); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadLocalDevItemCounts(tx *sql.Tx, charID int64) (map[int32]localDevItemTotals, error) {
	rows, err := tx.Query(`
		SELECT ii.item_id, COUNT(*), COALESCE(SUM(ii.quantity), 0)
		FROM cq_character_inventory ci
		JOIN cq_item_instances ii ON ii.id = ci.item_instance_id
		WHERE ci.character_id = $1
		GROUP BY ii.item_id
	`, charID)
	if err != nil {
		return nil, fmt.Errorf("query local dev inventory counts: %w", err)
	}
	defer rows.Close()

	counts := map[int32]localDevItemTotals{}
	for rows.Next() {
		var itemID int32
		var count localDevItemTotals
		if err := rows.Scan(&itemID, &count.instances, &count.quantity); err != nil {
			return nil, err
		}
		counts[itemID] = count
	}
	return counts, rows.Err()
}

func createLocalDevInventoryItem(tx *sql.Tx, charID int64, itemID int32, quantity int) error {
	if quantity < 1 {
		quantity = 1
	}

	var instanceID int64
	err := tx.QueryRow(`
		INSERT INTO cq_item_instances (item_id, quantity, owner_id, owner_type)
		VALUES ($1, $2, $3, 0)
		RETURNING id
	`, itemID, quantity, charID).Scan(&instanceID)
	if err != nil {
		return fmt.Errorf("create local dev item %d: %w", itemID, err)
	}

	if _, err := tx.Exec(`
		INSERT INTO cq_character_inventory (character_id, item_instance_id)
		VALUES ($1, $2)
	`, charID, instanceID); err != nil {
		return fmt.Errorf("place local dev item %d in inventory: %w", itemID, err)
	}

	return nil
}

func clampDevHP(hp int) int {
	if hp < 1 {
		return 1
	}
	return hp
}
