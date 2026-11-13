package world

import (
	"time"

	db_character "capturequest/internal/db/character"
)

const (
	DefaultSpawnX   = 3.0
	DefaultSpawnY   = 6.0
	DefaultSpawnZ   = 0.0
	DefaultSpawnMap = 38 // REDS_HOUSE_2F
	// Red/Blue starts the player facing the SNES in their upstairs room.
	DefaultSpawnDirection = "UP"

	PalletTownMapID = 0

	RecoverySpawnX         = 9.0
	RecoverySpawnY         = 4.0
	RecoverySpawnZ         = 0.0
	RecoverySpawnMap       = PalletTownMapID
	RecoverySpawnDirection = "DOWN"
)

type CharCreateRequest struct {
	Name          string `json:"name"`
	FactionID     uint32 `json:"factionId"`
	CharClass     uint32 `json:"charClass"`
	Gender        uint32 `json:"gender"`
	HomeTownMapID int32  `json:"homeTownMapId"`
	RivalName     string `json:"rivalName"`
}

type BindPoint struct {
	MapID   int32   `json:"mapId"`
	X       float32 `json:"x"`
	Y       float32 `json:"y"`
	Z       float32 `json:"z"`
	Heading float32 `json:"heading"`
}

type CharacterCreateProfile struct {
	Name      string                         `json:"name"`
	FactionID int32                          `json:"factionId"`
	CharClass int32                          `json:"charClass"`
	Gender    int32                          `json:"gender"`
	MapID     int32                          `json:"mapId"`
	X         float32                        `json:"x"`
	Y         float32                        `json:"y"`
	Z         float32                        `json:"z"`
	Heading   float32                        `json:"heading"`
	LastLogin int32                          `json:"lastLogin"`
	Birthday  int32                          `json:"birthday"`
	Binds     []BindPoint                    `json:"binds"`
	Options   *db_character.CharacterOptions `json:"-"`
}

func InitializeNewCharacterProfile(p *CharacterCreateProfile, cc CharCreateRequest) {
	p.Name = cc.Name
	p.FactionID = int32(cc.FactionID)
	p.CharClass = int32(cc.CharClass)
	p.Gender = int32(cc.Gender)

	p.MapID = DefaultSpawnMap
	p.LastLogin = int32(time.Now().Unix())
	p.Birthday = p.LastLogin
	p.Options = db_character.DefaultOptions()
	p.Options.RivalName = db_character.NormalizeRivalName(cc.RivalName)
}

func ApplyDefaultSpawn(p *CharacterCreateProfile) {
	p.MapID = DefaultSpawnMap
	p.X = DefaultSpawnX
	p.Y = DefaultSpawnY
	p.Z = DefaultSpawnZ
}

// Cardinal facing belongs to the live movement owner; PlayerFacingRequest does
// not persist it into character_data.heading. Entry intentionally starts a fresh
// facing owner, with the source upstairs spawn as the only initial-direction rule.
func initialPlayerDirection(mapID, x, y int) string {
	if mapID == DefaultSpawnMap && x == int(DefaultSpawnX) && y == int(DefaultSpawnY) {
		return DefaultSpawnDirection
	}
	return "DOWN"
}
