package scriptsim

import (
	"database/sql"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
	"capturequest/internal/world"
)

type Snapshot struct {
	CharacterID       int64
	CharacterName     string
	MapID             int
	MapName           string
	X                 int
	Y                 int
	Direction         string
	Flags             []string
	Money             int
	Coins             int
	PokedexSeen       int
	PokedexCaught     int
	Party             []PokemonSummary
	PC                []PCPokemonSummary
	Inventory         []ItemSummary
	HiddenObjects     []ObjectSummary
	ActiveBattle      *world.ActiveBattleSummary
	DayCare           world.DayCareStatus
	VermilionGymTrash *world.VermilionGymTrashState
}

type PokemonSummary struct {
	Slot        int
	SpeciesID   int
	Name        string
	Level       int
	CurHP       int
	MaxHP       int
	Status      string
	Exp         int
	ExpectedExp int
	IVs         IVsSpec
	EVs         EVsSpec
	Moves       []string
	MovePP      []int
	MoveMaxPP   []int
}

type PCPokemonSummary struct {
	Box       int
	Slot      int
	SpeciesID int
	Name      string
	Level     int
}

type ObjectSummary struct {
	ObjectID int
	Key      string
	Name     string
	Text     string
}

type ItemSummary struct {
	ItemID   int
	Name     string
	Quantity int
}

func CaptureSnapshot(charID int64, mapName string) (*Snapshot, error) {
	s := &Snapshot{CharacterID: charID}
	var heading float64
	var currentMapName sql.NullString
	if err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT cd.name, cd.map_id, pm.name, CAST(cd.x AS INTEGER), CAST(cd.y AS INTEGER), COALESCE(cd.heading, 0)
		FROM character_data cd
		LEFT JOIN phaser_maps pm ON pm.id = cd.map_id
		WHERE cd.id = $1`, charID).Scan(&s.CharacterName, &s.MapID, &currentMapName, &s.X, &s.Y, &heading); err != nil {
		return nil, fmt.Errorf("snapshot character: %w", err)
	}
	s.MapName = currentMapName.String
	if s.MapName == "" {
		s.MapName = mapName
	}
	s.Direction = fixtureDirectionFromHeading(heading)
	if mapName == "" {
		mapName = s.MapName
	}
	flags, err := loadFlags(charID)
	if err != nil {
		return nil, err
	}
	s.Flags = flags

	money, err := loadMoney(charID)
	if err != nil {
		return nil, err
	}
	s.Money = money

	coins, err := loadCoins(charID)
	if err != nil {
		return nil, err
	}
	s.Coins = coins

	seen, caught, err := loadPokedexCounts(charID)
	if err != nil {
		return nil, err
	}
	s.PokedexSeen = seen
	s.PokedexCaught = caught

	party, err := loadPartySummary(charID)
	if err != nil {
		return nil, err
	}
	s.Party = party

	pc, err := loadPCSummary(charID)
	if err != nil {
		return nil, err
	}
	s.PC = pc

	inventory, err := loadInventorySummary(charID)
	if err != nil {
		return nil, err
	}
	s.Inventory = inventory

	hidden, err := loadHiddenObjects(charID, mapName)
	if err != nil {
		return nil, err
	}
	s.HiddenObjects = hidden
	s.ActiveBattle = world.ActiveBattleSummaryForCharacter(charID)
	dayCare, err := world.LoadDayCareStatus(charID)
	if err != nil {
		return nil, fmt.Errorf("snapshot Day Care state: %w", err)
	}
	s.DayCare = dayCare
	trashState, err := world.LoadVermilionGymTrashState(charID)
	if err != nil {
		return nil, fmt.Errorf("snapshot Vermilion Gym trash state: %w", err)
	}
	s.VermilionGymTrash = trashState
	return s, nil
}

func loadFlags(charID int64) ([]string, error) {
	rows, err := db.GlobalWorldDB.DB.Query(
		`SELECT flag_name FROM character_event_flags WHERE character_id = $1 ORDER BY flag_name`, charID)
	if err != nil {
		return nil, fmt.Errorf("snapshot flags: %w", err)
	}
	defer rows.Close()

	var flags []string
	for rows.Next() {
		var flag string
		if err := rows.Scan(&flag); err != nil {
			return nil, err
		}
		flags = append(flags, flag)
	}
	return flags, rows.Err()
}

func loadMoney(charID int64) (int, error) {
	var money int
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT COALESCE(pokedollars, 0) FROM character_wallet WHERE character_id = $1`, charID).Scan(&money); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("snapshot money: %w", err)
	}
	return money, nil
}

func loadCoins(charID int64) (int, error) {
	var coins int
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT coins FROM character_coins WHERE character_id = $1`, charID).Scan(&coins); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("snapshot coins: %w", err)
	}
	return coins, nil
}

func loadPokedexCounts(charID int64) (int, int, error) {
	var seen int
	var caught int
	if err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT COALESCE(SUM(seen), 0), COALESCE(SUM(caught), 0)
		FROM character_pokedex
		WHERE character_id = $1`, charID).Scan(&seen, &caught); err != nil {
		return 0, 0, fmt.Errorf("snapshot pokedex counts: %w", err)
	}
	return seen, caught, nil
}

func loadPartySummary(charID int64) ([]PokemonSummary, error) {
	party, err := pokebattle.LoadParty(db.GlobalWorldDB.DB, charID)
	if err != nil {
		return nil, err
	}
	summaries := make([]PokemonSummary, 0, len(party))
	for slot, p := range party {
		if p == nil {
			continue
		}
		moves := make([]string, 0, len(p.Moves))
		movePP := make([]int, 0, len(p.Moves))
		moveMaxPP := make([]int, 0, len(p.Moves))
		for _, move := range p.Moves {
			if move.ID != 0 {
				name := move.Name
				if move.PP != move.MaxPP {
					name = fmt.Sprintf("%s(%d/%d)", move.Name, move.PP, move.MaxPP)
				}
				moves = append(moves, name)
				movePP = append(movePP, move.PP)
				moveMaxPP = append(moveMaxPP, move.MaxPP)
			}
		}
		summaries = append(summaries, PokemonSummary{
			Slot:        slot,
			SpeciesID:   p.ID,
			Name:        p.Name,
			Level:       p.Level,
			CurHP:       p.CurHP,
			MaxHP:       p.MaxHP,
			Status:      p.Status.String(),
			Exp:         p.Exp,
			ExpectedExp: pokebattle.ExpForLevel(p.GrowthRt, p.Level),
			IVs: IVsSpec{
				Attack:  p.IVs.Attack,
				Defense: p.IVs.Defense,
				Speed:   p.IVs.Speed,
				Special: p.IVs.Special,
			},
			EVs: EVsSpec{
				HP:      p.EVs.HP,
				Attack:  p.EVs.Attack,
				Defense: p.EVs.Defense,
				Speed:   p.EVs.Speed,
				Special: p.EVs.Special,
			},
			Moves:     moves,
			MovePP:    movePP,
			MoveMaxPP: moveMaxPP,
		})
	}
	return summaries, nil
}

func loadPCSummary(charID int64) ([]PCPokemonSummary, error) {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT cp.box, cp.box_slot, cp.pokemon_id, pp.name, cp.level
		FROM character_pokemon cp
		JOIN phaser_pokemon pp ON pp.id = cp.pokemon_id
		WHERE cp.character_id = $1 AND cp.box >= 0
		ORDER BY cp.box, cp.box_slot`, charID)
	if err != nil {
		return nil, fmt.Errorf("snapshot PC pokemon: %w", err)
	}
	defer rows.Close()

	pc := []PCPokemonSummary{}
	for rows.Next() {
		var pokemon PCPokemonSummary
		if err := rows.Scan(&pokemon.Box, &pokemon.Slot, &pokemon.SpeciesID, &pokemon.Name, &pokemon.Level); err != nil {
			return nil, err
		}
		pc = append(pc, pokemon)
	}
	return pc, rows.Err()
}

func loadInventorySummary(charID int64) ([]ItemSummary, error) {
	items, err := cqitems.GetCharacterInventory(int32(charID))
	if err != nil {
		return nil, err
	}
	summaries := make([]ItemSummary, 0, len(items))
	for _, item := range items {
		summaries = append(summaries, ItemSummary{
			ItemID:   int(item.Item.ID),
			Name:     item.Item.Name,
			Quantity: int(item.Instance.Quantity),
		})
	}
	return summaries, nil
}

func loadHiddenObjects(charID int64, mapName string) ([]ObjectSummary, error) {
	objects, err := queryHiddenObjectsByMapName(charID, mapName)
	if err != nil {
		return nil, err
	}
	if len(objects) > 0 || !isOverworldMapName(mapName) {
		return objects, nil
	}

	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT po.id, COALESCE(po.name, ''), COALESCE(po.text, '')
		FROM character_collected_items cci
		JOIN phaser_objects po ON po.id = cci.object_id
		WHERE cci.character_id = $1 AND po.map_id = $2
		ORDER BY po.id`, charID, world.UnifiedOverworldMapID)
	if err != nil {
		return nil, fmt.Errorf("snapshot hidden objects: %w", err)
	}
	defer rows.Close()

	objects = []ObjectSummary{}
	for rows.Next() {
		var obj ObjectSummary
		if err := rows.Scan(&obj.ObjectID, &obj.Name, &obj.Text); err != nil {
			return nil, err
		}
		obj.Key = fmt.Sprintf("object:%d", obj.ObjectID)
		objects = append(objects, obj)
	}
	return objects, rows.Err()
}

func queryHiddenObjectsByMapName(charID int64, mapName string) ([]ObjectSummary, error) {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT po.id, COALESCE(po.name, ''), COALESCE(po.text, '')
		FROM character_collected_items cci
		JOIN phaser_objects po ON po.id = cci.object_id
		JOIN phaser_maps pm ON pm.id = po.map_id
		WHERE cci.character_id = $1 AND pm.name = $2
		ORDER BY po.id`, charID, mapName)
	if err != nil {
		return nil, fmt.Errorf("snapshot hidden objects: %w", err)
	}
	defer rows.Close()

	var objects []ObjectSummary
	for rows.Next() {
		var obj ObjectSummary
		if err := rows.Scan(&obj.ObjectID, &obj.Name, &obj.Text); err != nil {
			return nil, err
		}
		obj.Key = fmt.Sprintf("object:%d", obj.ObjectID)
		objects = append(objects, obj)
	}
	return objects, rows.Err()
}

func hasFlag(flags []string, flag string) bool {
	for _, existing := range flags {
		if existing == flag {
			return true
		}
	}
	return false
}
