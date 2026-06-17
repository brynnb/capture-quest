// Package staticdata handles loading of game data from the database.
package staticdata

import (
	"context"
	"fmt"
	"sync"

	"capturequest/internal/cache"
	"capturequest/internal/db"
)

// ClassInfo represents a playable class
type ClassInfo struct {
	ID        int32  `json:"id"`
	Name      string `json:"name"`
	ClassType string `json:"class_type"`
	Lore      string `json:"lore"`
}

// FactionInfo represents a faction in CaptureQuest
type FactionInfo struct {
	ID         int32  `json:"id"`
	Name       string `json:"name"`
	ShortName  string `json:"short_name"`
	Lore       string `json:"lore"`
	IsPlayable bool   `json:"is_playable"`
	IsStarting bool   `json:"is_starting"`
}

// StartCityInfo represents a starting city for character creation
type StartCityInfo struct {
	ID          int32  `json:"id"`
	MapID       int32  `json:"mapId"`
	Name        string `json:"name"`
	SpawnX      int32  `json:"spawnX"`
	SpawnY      int32  `json:"spawnY"`
	Description string `json:"description"`
	SortOrder   int32  `json:"sortOrder"`
}

// MapInfo represents a map in CaptureQuest.
type MapInfo struct {
	ID              int32  `json:"id"`
	Name            string `json:"name"`
	Width           int32  `json:"width"`
	Height          int32  `json:"height"`
	TilesetID       *int32 `json:"tileset_id"`
	IsOverworld     bool   `json:"is_overworld"`
	NorthConnection *int32 `json:"north_connection"`
	SouthConnection *int32 `json:"south_connection"`
	WestConnection  *int32 `json:"west_connection"`
	EastConnection  *int32 `json:"east_connection"`
}

// StaticData holds all static game data
type StaticData struct {
	Classes     []ClassInfo
	Factions    []FactionInfo
	Maps        []MapInfo
	StartCities []StartCityInfo
}

var (
	staticData     *StaticData
	staticDataOnce sync.Once
	staticDataErr  error
)

// GetStaticData returns the cached static data, loading it if necessary
func GetStaticData(ctx context.Context) (*StaticData, error) {
	staticDataOnce.Do(func() {
		staticData, staticDataErr = loadStaticData(ctx)
	})
	return staticData, staticDataErr
}

func loadStaticData(ctx context.Context) (*StaticData, error) {
	cacheKey := "staticdata:all"
	if val, found, err := cache.GetCache().Get(cacheKey); err == nil && found {
		if data, ok := val.(*StaticData); ok {
			return data, nil
		}
	}

	data := &StaticData{}

	// Load classes from DB
	classes, err := loadClassesFromDB(ctx)
	if err != nil {
		return nil, fmt.Errorf("load classes: %w", err)
	}
	data.Classes = classes

	// Load factions from DB
	factions, err := loadFactionsFromDB(ctx)
	if err != nil {
		fmt.Printf("Warning: failed to load factions: %v\n", err)
	} else {
		data.Factions = factions
	}

	// Load maps from DB
	maps, err := loadMapsFromDB(ctx)
	if err != nil {
		fmt.Printf("Warning: failed to load maps: %v\n", err)
	} else {
		data.Maps = maps
	}

	// Load start cities from DB
	startCities, err := loadStartCitiesFromDB(ctx)
	if err != nil {
		fmt.Printf("Warning: failed to load start cities: %v\n", err)
	} else {
		data.StartCities = startCities
	}

	cache.GetCache().Set(cacheKey, data)
	return data, nil
}

func loadClassesFromDB(ctx context.Context) ([]ClassInfo, error) {
	return loadPokeClassesFromDB(ctx)
}

func loadPokeClassesFromDB(ctx context.Context) ([]ClassInfo, error) {
	rows, err := db.GlobalWorldDB.DB.QueryContext(ctx, "SELECT id, name, class_type, lore FROM poke_classes ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var classes []ClassInfo
	for rows.Next() {
		var c ClassInfo
		var classType, lore *string

		err := rows.Scan(&c.ID, &c.Name, &classType, &lore)
		if err != nil {
			return nil, err
		}

		if classType != nil {
			c.ClassType = *classType
		}
		if lore != nil {
			c.Lore = *lore
		}

		classes = append(classes, c)
	}

	return classes, nil
}

func loadStartCitiesFromDB(ctx context.Context) ([]StartCityInfo, error) {
	rows, err := db.GlobalWorldDB.DB.QueryContext(ctx, "SELECT id, map_id, name, spawn_x, spawn_y, description, sort_order FROM poke_start_cities ORDER BY sort_order ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cities []StartCityInfo
	for rows.Next() {
		var c StartCityInfo
		var desc *string

		err := rows.Scan(&c.ID, &c.MapID, &c.Name, &c.SpawnX, &c.SpawnY, &desc, &c.SortOrder)
		if err != nil {
			return nil, err
		}

		if desc != nil {
			c.Description = *desc
		}

		cities = append(cities, c)
	}

	return cities, nil
}

func loadFactionsFromDB(ctx context.Context) ([]FactionInfo, error) {
	rows, err := db.GlobalWorldDB.DB.QueryContext(ctx, "SELECT id, name, short_name, lore, is_playable, is_starting FROM poke_factions ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var factions []FactionInfo
	for rows.Next() {
		var f FactionInfo
		var shortName, lore *string
		var isPlayable, isStarting *int8

		err := rows.Scan(&f.ID, &f.Name, &shortName, &lore, &isPlayable, &isStarting)
		if err != nil {
			return nil, err
		}

		if shortName != nil {
			f.ShortName = *shortName
		}
		if lore != nil {
			f.Lore = *lore
		}
		if isPlayable != nil {
			f.IsPlayable = *isPlayable == 1
		}
		if isStarting != nil {
			f.IsStarting = *isStarting == 1
		}
		factions = append(factions, f)
	}

	return factions, nil
}

func loadMapsFromDB(ctx context.Context) ([]MapInfo, error) {
	rows, err := db.GlobalWorldDB.DB.QueryContext(ctx, `
		SELECT id, name, width, height, tileset_id, is_overworld,
		       north_connection, south_connection, west_connection, east_connection
		FROM phaser_maps ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var maps []MapInfo
	for rows.Next() {
		var m MapInfo
		var isOverworld *int8

		err := rows.Scan(&m.ID, &m.Name, &m.Width, &m.Height, &m.TilesetID, &isOverworld,
			&m.NorthConnection, &m.SouthConnection, &m.WestConnection, &m.EastConnection)
		if err != nil {
			return nil, err
		}

		if isOverworld != nil {
			m.IsOverworld = *isOverworld == 1
		}

		maps = append(maps, m)
	}

	return maps, nil
}
