package world

import (
	"capturequest/internal/testdb"
	"context"
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

	if fishable, err := isFacingFishableWater(context.Background(), wh, 1, 5, 5, "UP"); err != nil || !fishable {
		t.Fatalf("expected fishing to be allowed when facing water")
	}
	if fishable, err := isFacingFishableWater(context.Background(), wh, 1, 5, 5, "DOWN"); err != nil || fishable {
		t.Fatalf("expected fishing to be blocked when facing land")
	}
	if fishable, err := isFacingFishableWater(context.Background(), wh, 1, 5, 5, "LEFT"); err != nil || fishable {
		t.Fatalf("expected fishing to be blocked when facing blocked tile")
	}
	if fishable, err := isFacingFishableWater(context.Background(), wh, 1, 5, 5, "RIGHT"); err != nil || fishable {
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
			name: "numeric IDs are catalog selectors, not rod names",
			req:  PokeFishingRequestPayload{ItemID: 76},
			want: "",
		},
		{
			name: "unknown",
			req:  PokeFishingRequestPayload{ItemID: 999, RodType: "BICYCLE"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeFishingRodName(tt.req.RodType); got != tt.want {
				t.Fatalf("normalizeFishingRodName(%#v) = %q, want %q", tt.req, got, tt.want)
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
	db.GlobalWorldDB = nil
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
		database: testDB,
		ActorManager: &PhaserActorManager{
			collisionMap: map[int]map[string]int{
				1: {
					"5,5": collisionLand,
					"6,5": collisionWater,
				},
			},
		},
	}

	revision := int64(0)
	payload, err := json.Marshal(PokeFishingRequestPayload{RequestID: "fishing:normal", CharacterID: 42, InstanceID: 1, CommandRevision: &revision, RodType: "OLD_ROD", Direction: "RIGHT"})
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
	testDB := testdb.Postgres(t)
	t.Cleanup(func() { testDB.Close() })

	if _, err := testDB.Exec(`
        INSERT INTO character_data(id,name,map_id,x,y,heading) VALUES(42,'fishing',1,5,5,90);
 INSERT INTO cq_items(id,name,short_name) VALUES(76,'Old Rod','OLD_ROD');
 INSERT INTO cq_item_instances(id,item_id,quantity,owner_type,owner_id) VALUES(1,76,1,0,42);
 INSERT INTO cq_character_inventory(character_id,item_instance_id) VALUES(42,1);
 INSERT INTO phaser_maps(id,name,width,height) VALUES(1,'ROOM',10,10);
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type) VALUES(1,5,5,0,1),(1,6,5,0,2);
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

func TestFishingRejectsForgedSourceUnownedRodAndFailedCommit(t *testing.T) {
	for _, kind := range []string{"remote", "facing", "unowned", "rod-disagreement", "catalog-error", "commit-error", "moving", "owned-source", "catalog-id", "catalog-alias", "success"} {
		t.Run(kind, func(t *testing.T) {
			database := openFishingTestDB(t)
			previous := db.GlobalWorldDB
			db.GlobalWorldDB = nil
			t.Cleanup(func() { db.GlobalWorldDB = previous; forgetBattle(42, getBattle(42)) })
			messages := &recordingMessenger{}
			ses := &session.Session{SessionID: 1, Authenticated: true, Messenger: messages, Client: &testSessionClient{char: &model.CharacterData{ID: 42, MapID: 1, X: 5, Y: 5, Heading: 90}}}
			wh := &WorldHandler{database: database, ActorManager: &PhaserActorManager{collisionMap: map[int]map[string]int{}}}
			revision := int64(0)
			request := PokeFishingRequestPayload{CommandRevision: &revision, RequestID: "fish:owned", CharacterID: 42, InstanceID: 1, ItemID: 76, RodType: "OLD_ROD", Direction: "RIGHT"}
			switch kind {
			case "catalog-id":
				request.RodType = ""
			case "catalog-alias":
				testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name) VALUES(500,'Catalog Old Rod','OLD_ROD'); UPDATE cq_item_instances SET item_id=500 WHERE id=1`)
				request.ItemID = 500
				request.RodType = ""
			case "moving":
				wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
				wh.PlayerMovement.RegisterPlayer(ses, 42, 5, 5, 1, "RIGHT")
				wh.PlayerMovement.players[42].Path = []PathNode{{X: 6, Y: 5}}
			case "owned-source":
				wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
				wh.PlayerMovement.RegisterPlayer(ses, 42, 5, 5, 1, "RIGHT")
				ses.Client.CharData().X = 99
				ses.Client.CharData().Y = 99
			case "remote":
				x := 99
				request.X = &x
			case "facing":
				request.Direction = "LEFT"
			case "unowned":
				testdb.Exec(t, database, `UPDATE cq_item_instances SET owner_id=99 WHERE id=1`)
			case "rod-disagreement":
				request.RodType = "SUPER_ROD"
			case "catalog-error":
				testdb.Exec(t, database, `ALTER TABLE phaser_pokemon RENAME TO unavailable_species`)
			case "commit-error":
				testdb.Exec(t, database, `CREATE FUNCTION reject_fishing() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late fishing failure'; END $$; CREATE CONSTRAINT TRIGGER reject_fishing AFTER INSERT ON character_battle_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_fishing()`)
			}
			payload, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			HandlePokeFishing(ses, payload, wh)
			var response struct {
				Success bool
				Hooked  bool
			}
			if len(messages.streams) == 0 || json.Unmarshal(messages.streams[0].payload, &response) != nil {
				t.Fatal("missing terminal fishing response")
			}
			var envelope map[string]any
			if json.Unmarshal(messages.streams[0].payload, &envelope) != nil || envelope["requestId"] != "fish:owned" || envelope["characterId"] != float64(42) || envelope["instanceId"] != float64(1) {
				t.Fatalf("fishing identity=%v", envelope)
			}
			var battles, seen int
			database.QueryRow(`SELECT COUNT(*) FROM character_battle_state WHERE character_id=42`).Scan(&battles)
			database.QueryRow(`SELECT COUNT(*) FROM character_pokedex WHERE character_id=42`).Scan(&seen)
			if kind == "success" || kind == "owned-source" || kind == "catalog-id" || kind == "catalog-alias" {
				if !response.Success || !response.Hooked || battles != 1 || seen != 1 || getBattle(42) == nil {
					t.Fatalf("successful fishing=%+v battles=%d seen=%d", response, battles, seen)
				}
				messages.streams = nil
				HandlePokeFishing(ses, payload, wh)
				json.Unmarshal(messages.streams[0].payload, &response)
				if !response.Success || len(messages.streams) != 1 {
					t.Fatal("duplicate did not replay its receipt without another battle publication")
				}
				testdb.Exec(t, database, `DELETE FROM character_battle_state WHERE character_id=42`)
				forgetBattle(42, getBattle(42))
				messages.streams = nil
				HandlePokeFishing(ses, payload, wh)
				database.QueryRow(`SELECT COUNT(*) FROM character_battle_state WHERE character_id=42`).Scan(&battles)
				if len(messages.streams) != 1 || battles != 0 || getBattle(42) != nil {
					t.Fatal("receipt replay resurrected a dismissed encounter")
				}
			} else if response.Success || battles != 0 || seen != 0 || getBattle(42) != nil {
				t.Fatalf("rejected fishing=%+v battles=%d seen=%d", response, battles, seen)
			}
		})
	}
}

func TestFishingNoBiteReceiptSurvivesRodRemovalAndOwnerReplacement(t *testing.T) {
	database := openFishingTestDB(t)
	testdb.Exec(t, database, `UPDATE cq_items SET short_name='SUPER_ROD',name='Super Rod' WHERE id=76`)
	messages := &recordingMessenger{}
	ses := &session.Session{SessionID: 1, Authenticated: true, Messenger: messages, Client: &testSessionClient{char: &model.CharacterData{ID: 42, MapID: 1, X: 5, Y: 5, Heading: 90}}}
	wh := &WorldHandler{database: database, ActorManager: &PhaserActorManager{collisionMap: map[int]map[string]int{}}}
	revision := int64(0)
	request := PokeFishingRequestPayload{CommandRevision: &revision, RequestID: "no-bite", CharacterID: 42, InstanceID: 1, Direction: "RIGHT"}
	payload, _ := json.Marshal(request)
	HandlePokeFishing(ses, payload, wh)
	if len(messages.streams) != 1 {
		t.Fatalf("no bite packets=%d", len(messages.streams))
	}
	first := string(messages.streams[0].payload)
	var result struct {
		Success bool
		Hooked  bool
	}
	json.Unmarshal(messages.streams[0].payload, &result)
	if !result.Success || result.Hooked {
		t.Fatalf("no-bite=%s", first)
	}
	testdb.Exec(t, database, `DELETE FROM cq_character_inventory WHERE character_id=42; ALTER TABLE phaser_pokemon RENAME TO unavailable_species`)
	messages.streams = nil
	ses.SessionID = 2
	ses.Client.CharData().Heading = 180
	HandlePokeFishing(ses, payload, wh)
	if len(messages.streams) != 1 || string(messages.streams[0].payload) != first || getBattle(42) != nil {
		t.Fatalf("replayed no-bite changed or rerolled=%+v", messages.streams)
	}
}

func TestFishingRetiredIdentityFreeRequestsCannotMutate(t *testing.T) {
	database := openFishingTestDB(t)
	messages := &recordingMessenger{}
	ses := &session.Session{SessionID: 1, Authenticated: true, Messenger: messages, Client: &testSessionClient{char: &model.CharacterData{ID: 42, MapID: 1, X: 5, Y: 5, Heading: 90}}}
	wh := &WorldHandler{database: database, ActorManager: &PhaserActorManager{collisionMap: map[int]map[string]int{}}}
	for _, payload := range []string{
		`{"rodType":"OLD_ROD","direction":"RIGHT"}`,
		`{"requestId":"current","characterId":42,"instanceId":1,"direction":"RIGHT"}`,
		`{"requestId":"current","characterId":42,"commandRevision":0,"itemId":76,"direction":"RIGHT"}`,
		`{"requestId":"current","characterId":99,"instanceId":1,"commandRevision":0,"direction":"RIGHT"}`,
		`{"requestId":"current","characterId":42,"instanceId":1,"commandRevision":0,"direction":"RIGHT","unexpected":true}`,
	} {
		messages.streams = nil
		HandlePokeFishing(ses, []byte(payload), wh)
		var response struct {
			Success bool
			Error   string
		}
		if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &response) != nil || response.Success || response.Error == "" {
			t.Fatalf("retired request accepted: %s response=%+v", payload, response)
		}
	}
	for _, table := range []string{"character_battle_state", "character_pokedex", "character_field_command_state"} {
		var count int
		if err := database.QueryRow(`SELECT count(*) FROM ` + table + ` WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("retired request wrote %s: count=%d error=%v", table, count, err)
		}
	}
}
