package models

import "time"

type Variables struct {
	ID          int32 `sql:"primary_key"`
	Varname     string
	Value       string
	Information string
	Ts          time.Time
}

type CharacterBind struct {
	ID      uint32 `sql:"primary_key"`
	Slot    int32  `sql:"primary_key"`
	MapID   uint32
	X       float64
	Y       float64
	Z       float64
	Heading float64
}

type CharacterWallet struct {
	CharacterID uint32 `sql:"primary_key"`
	Pokedollars uint32
}

type CharacterData struct {
	ID         uint32 `sql:"primary_key"`
	AccountID  int32
	Name       string
	LastName   string
	Title      string
	Suffix     string
	MapID      uint32
	Y          float64
	X          float64
	Z          float64
	Heading    float64
	Gender     uint8
	FactionID  uint16
	Class      uint8
	Birthday   uint32
	LastLogin  uint32
	TimePlayed uint32
	Gm         uint8
	DeletedAt  *time.Time
	Options    *string
}
