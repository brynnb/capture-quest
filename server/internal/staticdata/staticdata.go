// Package staticdata defines the shared static-content projection.
package staticdata

// ClassInfo represents a playable class
type ClassInfo struct {
	ID        int32  `json:"id"`
	Name      string `json:"name"`
	ClassType string `json:"classType"`
	Lore      string `json:"lore"`
}

// FactionInfo represents a faction in CaptureQuest
type FactionInfo struct {
	ID         int32  `json:"id"`
	Name       string `json:"name"`
	ShortName  string `json:"shortName"`
	Lore       string `json:"lore"`
	IsPlayable bool   `json:"isPlayable"`
	IsStarting bool   `json:"isStarting"`
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
	TilesetID       *int32 `json:"tilesetId" tstype:"number | null"`
	IsOverworld     bool   `json:"isOverworld"`
	NorthConnection *int32 `json:"northConnection" tstype:"number | null"`
	SouthConnection *int32 `json:"southConnection" tstype:"number | null"`
	WestConnection  *int32 `json:"westConnection" tstype:"number | null"`
	EastConnection  *int32 `json:"eastConnection" tstype:"number | null"`
}

// StaticData holds all static game data
type StaticData struct {
	Classes     []ClassInfo     `json:"classes"`
	Factions    []FactionInfo   `json:"factions"`
	Maps        []MapInfo       `json:"maps"`
	StartCities []StartCityInfo `json:"startCities"`
}
