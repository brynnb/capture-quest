package models

import "time"

type Variables struct {
	ID          int32     `sql:"primary_key" json:"id"`
	Varname     string    `json:"varname"`
	Value       string    `json:"value"`
	Information string    `json:"information"`
	Ts          time.Time `json:"ts"`
}

type CharacterBind struct {
	ID      uint32  `sql:"primary_key" json:"id"`
	Slot    int32   `sql:"primary_key" json:"slot"`
	MapID   uint32  `json:"mapId"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Z       float64 `json:"z"`
	Heading float64 `json:"heading"`
}

type CharacterWallet struct {
	CharacterID uint32 `sql:"primary_key" json:"characterId"`
	Pokedollars uint32 `json:"pokedollars"`
}

type CharacterData struct {
	ID         uint32     `sql:"primary_key" json:"id"`
	AccountID  int32      `json:"accountId"`
	Name       string     `json:"name"`
	LastName   string     `json:"lastName"`
	Title      string     `json:"title"`
	Suffix     string     `json:"suffix"`
	MapID      uint32     `json:"mapId"`
	Y          float64    `json:"y"`
	X          float64    `json:"x"`
	Z          float64    `json:"z"`
	Heading    float64    `json:"heading"`
	Gender     uint8      `json:"gender"`
	FactionID  uint16     `json:"factionId"`
	Class      uint8      `json:"class"`
	Birthday   uint32     `json:"birthday"`
	LastLogin  uint32     `json:"lastLogin"`
	TimePlayed uint32     `json:"timePlayed"`
	Gm         uint8      `json:"gm"`
	DeletedAt  *time.Time `json:"deletedAt,omitempty"`
	// Options is the stored JSON string; the wire response exposes parsed preferences.
	Options *string `json:"-"`
}
