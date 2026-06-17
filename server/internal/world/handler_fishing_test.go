package world

import (
	"database/sql"
	"encoding/json"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	model "capturequest/internal/db/models"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"

	_ "modernc.org/sqlite"
)

func TestFishingTargetTileUsesFacingDirection(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		wantX     int
		wantY     int
		wantOK    bool
	}{
		{name: "up", direction: "UP", wantX: 5, wantY: 4, wantOK: true},
		{name: "down", direction: "DOWN", wantX: 5, wantY: 6, wantOK: true},
		{name: "left", direction: "left", wantX: 4, wantY: 5, wantOK: true},
		{name: "right", direction: "RIGHT", wantX: 6, wantY: 5, wantOK: true},
		{name: "invalid", direction: "NORTH", wantX: 5, wantY: 5, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotX, gotY, gotOK := fishingTargetTile(5, 5, tt.direction)
			if gotX != tt.wantX || gotY != tt.wantY || gotOK != tt.wantOK {
				t.Fatalf("fishingTargetTile = (%d,%d,%v), want (%d,%d,%v)", gotX, gotY, gotOK, tt.wantX, tt.wantY, tt.wantOK)
			}
		})
	}
}

func TestFishingRequiresWaterInFacingTile(t *testing.T) {
	actorManager := &PhaserActorManager{
		collisionMap: map[int]map[string]int{
			1: {
				"5,5": collisionLand,
				"5,4": collisionWater,
				"5,6": collisionLand,
				"4,5": collisionBlocked,
			},
		},
	}
	wh := &WorldHandler{ActorManager: actorManager}

	if !isFacingFishableWater(wh, 1, 5, 5, "UP") {
		t.Fatalf("expected fishing to be allowed when facing water")
	}
	if isFacingFishableWater(wh, 1, 5, 5, "DOWN") {
		t.Fatalf("expected fishing to be blocked when facing land")
	}
	if isFacingFishableWater(wh, 1, 5, 5, "LEFT") {
		t.Fatalf("expected fishing to be blocked when facing blocked tile")
	}
	if isFacingFishableWater(wh, 1, 5, 5, "RIGHT") {
		t.Fatalf("expected fishing to be blocked when facing missing tile")
	}
}

func TestFishingRodTypePrefersStableRodName(t *testing.T) {
	tests := []struct {
		name string
		req  PokeFishingRequestPayload
		want string
	}{
		{
			name: "old rod short name",
			req:  PokeFishingRequestPayload{RodType: "OLD_ROD"},
			want: "old_rod",
		},
		{
			name: "good rod display name",
			req:  PokeFishingRequestPayload{RodType: "Good Rod"},
			want: "good_rod",
		},
		{
			name: "super rod ignores mismatched fallback id",
			req:  PokeFishingRequestPayload{ItemID: 76, RodType: "SUPER_ROD"},
			want: "super_rod",
		},
		{
			name: "legacy old rod id",
			req:  PokeFishingRequestPayload{ItemID: 76},
			want: "old_rod",
		},
		{
			name: "unknown",
			req:  PokeFishingRequestPayload{ItemID: 999, RodType: "BICYCLE"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fishingRodType(tt.req); got != tt.want {
				t.Fatalf("fishingRodType(%#v) = %q, want %q", tt.req, got, tt.want)
			}
		})
	}
}

func TestDirectionFromCharacterHeading(t *testing.T) {
	tests := []struct {
		heading float64
		want    string
	}{
		{heading: 0, want: "UP"},
		{heading: 90, want: "RIGHT"},
		{heading: 180, want: "DOWN"},
		{heading: 270, want: "LEFT"},
		{heading: -90, want: "LEFT"},
		{heading: 450, want: "RIGHT"},
	}

	for _, tt := range tests {
		if got := directionFromCharacterHeading(tt.heading); got != tt.want {
			t.Fatalf("directionFromCharacterHeading(%v) = %q, want %q", tt.heading, got, tt.want)
		}
	}
}

func TestFishingOldRodFacingWaterStartsBattle(t *testing.T) {
	testDB := openFishingTestDB(t)
	previousWorldDB := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: testDB}
	t.Cleanup(func() {
		db.GlobalWorldDB = previousWorldDB
		activeBattlesMu.Lock()
		delete(activeBattles, 42)
		activeBattlesMu.Unlock()
	})

	messenger := &recordingMessenger{}
	ses := &session.Session{
		SessionID:     1,
		Authenticated: true,
		Messenger:     messenger,
		Client: &testSessionClient{char: &model.CharacterData{
			ID:      42,
			Name:    "FishingTester",
			MapID:   1,
			X:       5,
			Y:       5,
			Heading: 90,
		}},
	}
	wh := &WorldHandler{
		ActorManager: &PhaserActorManager{
			collisionMap: map[int]map[string]int{
				1: {
					"5,5": collisionLand,
					"6,5": collisionWater,
				},
			},
		},
	}

	payload, err := json.Marshal(PokeFishingRequestPayload{RodType: "OLD_ROD", Direction: "RIGHT"})
	if err != nil {
		t.Fatalf("marshal fishing request: %v", err)
	}

	HandlePokeFishing(ses, payload, wh)

	if got := len(messenger.streams); got != 2 {
		t.Fatalf("stream messages = %d, want fishing response and battle start", got)
	}
	if got := messenger.streams[0].opcode; got != opcodes.PokeFishingResponse {
		t.Fatalf("first opcode = %d, want %d", got, opcodes.PokeFishingResponse)
	}
	var fishResp struct {
		Success bool   `json:"success"`
		Hooked  bool   `json:"hooked"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(messenger.streams[0].payload, &fishResp); err != nil {
		t.Fatalf("unmarshal fishing response: %v", err)
	}
	if !fishResp.Success || !fishResp.Hooked || fishResp.Message != "Oh! A bite!" {
		t.Fatalf("fishing response = %+v, want hooked bite", fishResp)
	}
	if got := messenger.streams[1].opcode; got != opcodes.PokeBattleStartResponse {
		t.Fatalf("second opcode = %d, want %d", got, opcodes.PokeBattleStartResponse)
	}
	var battleResp struct {
		Success      bool `json:"success"`
		EnemyPokemon struct {
			Name  string `json:"name"`
			Level int    `json:"level"`
		} `json:"enemyPokemon"`
	}
	if err := json.Unmarshal(messenger.streams[1].payload, &battleResp); err != nil {
		t.Fatalf("unmarshal battle response: %v", err)
	}
	if !battleResp.Success || battleResp.EnemyPokemon.Name != "MAGIKARP" || battleResp.EnemyPokemon.Level != 5 {
		t.Fatalf("battle response enemy = %+v, want L5 MAGIKARP", battleResp.EnemyPokemon)
	}

	battle := getBattle(42)
	if battle == nil {
		t.Fatal("expected active battle after fishing")
	}
	if battle.BattleType != pokebattle.BattleWild {
		t.Fatalf("battle type = %v, want wild", battle.BattleType)
	}
	if enemy := battle.GetEnemyPokemon(); enemy == nil || enemy.Name != "MAGIKARP" || enemy.Level != 5 {
		t.Fatalf("active battle enemy = %+v, want L5 MAGIKARP", enemy)
	}
}

func openFishingTestDB(t *testing.T) *sql.DB {
	t.Helper()
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { testDB.Close() })

	if _, err := testDB.Exec(`
		CREATE TABLE phaser_pokemon (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			type_1 TEXT NOT NULL,
			type_2 TEXT,
			hp INTEGER NOT NULL,
			atk INTEGER NOT NULL,
			def INTEGER NOT NULL,
			spd INTEGER NOT NULL,
			spc INTEGER NOT NULL,
			catch_rate INTEGER NOT NULL,
			base_exp INTEGER NOT NULL,
			growth_rate TEXT NOT NULL,
			default_move_1_id TEXT,
			default_move_2_id TEXT,
			default_move_3_id TEXT,
			default_move_4_id TEXT,
			base_cry INTEGER,
			cry_pitch INTEGER,
			cry_length INTEGER,
			evolve_level INTEGER,
			evolve_pokemon TEXT
		);
		CREATE TABLE phaser_pokemon_learnset (
			pokemon_id INTEGER NOT NULL,
			level INTEGER NOT NULL,
			move_id INTEGER NOT NULL,
			move_name TEXT NOT NULL
		);
		CREATE TABLE character_pokemon (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			character_id INTEGER NOT NULL,
			party_slot INTEGER,
			box INTEGER NOT NULL DEFAULT -1,
			box_slot INTEGER NOT NULL DEFAULT -1,
			pokemon_id INTEGER NOT NULL,
			nickname TEXT DEFAULT '',
			level INTEGER NOT NULL DEFAULT 5,
			exp INTEGER NOT NULL DEFAULT 0,
			growth_rate TEXT NOT NULL DEFAULT 'MEDIUM_FAST',
			cur_hp INTEGER NOT NULL,
			max_hp INTEGER NOT NULL,
			iv_atk INTEGER NOT NULL DEFAULT 0,
			iv_def INTEGER NOT NULL DEFAULT 0,
			iv_spd INTEGER NOT NULL DEFAULT 0,
			iv_spc INTEGER NOT NULL DEFAULT 0,
			ev_hp INTEGER NOT NULL DEFAULT 0,
			ev_atk INTEGER NOT NULL DEFAULT 0,
			ev_def INTEGER NOT NULL DEFAULT 0,
			ev_spd INTEGER NOT NULL DEFAULT 0,
			ev_spc INTEGER NOT NULL DEFAULT 0,
			move1_id INTEGER NOT NULL DEFAULT 0,
			move1_pp INTEGER NOT NULL DEFAULT 0,
			move1_pp_up INTEGER NOT NULL DEFAULT 0,
			move2_id INTEGER NOT NULL DEFAULT 0,
			move2_pp INTEGER NOT NULL DEFAULT 0,
			move2_pp_up INTEGER NOT NULL DEFAULT 0,
			move3_id INTEGER NOT NULL DEFAULT 0,
			move3_pp INTEGER NOT NULL DEFAULT 0,
			move3_pp_up INTEGER NOT NULL DEFAULT 0,
			move4_id INTEGER NOT NULL DEFAULT 0,
			move4_pp INTEGER NOT NULL DEFAULT 0,
			move4_pp_up INTEGER NOT NULL DEFAULT 0,
			status INTEGER NOT NULL DEFAULT 0,
			original_trainer_id INTEGER
		);
		INSERT INTO phaser_pokemon (
			id, name, type_1, type_2, hp, atk, def, spd, spc, catch_rate, base_exp, growth_rate
		) VALUES
			(25, 'PIKACHU', 'ELECTRIC', 'ELECTRIC', 35, 55, 40, 90, 50, 190, 82, 'MEDIUM_FAST'),
			(129, 'MAGIKARP', 'WATER', 'WATER', 20, 10, 55, 80, 20, 255, 20, 'SLOW');
		INSERT INTO character_pokemon (
			character_id, party_slot, box, box_slot, pokemon_id, nickname,
			level, exp, growth_rate, cur_hp, max_hp,
			iv_atk, iv_def, iv_spd, iv_spc,
			ev_hp, ev_atk, ev_def, ev_spd, ev_spc,
			status, original_trainer_id
		)
		VALUES (42, 0, -1, 0, 25, '', 20, 0, 'MEDIUM_FAST', 35, 35, 10, 10, 10, 10, 0, 0, 0, 0, 0, 0, 42);
	`); err != nil {
		t.Fatalf("seed fishing db: %v", err)
	}
	return testDB
}
