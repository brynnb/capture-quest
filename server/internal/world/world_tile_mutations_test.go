package world

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	model "capturequest/internal/db/models"
	"capturequest/internal/overworldoverview"
	"capturequest/internal/session"

	_ "modernc.org/sqlite"
)

func setupWorldTileMutationDB(t *testing.T) *sql.DB {
	t.Helper()

	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	schema := `
		CREATE TABLE account (
			id INTEGER PRIMARY KEY,
			status INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE phaser_maps (
			id INTEGER PRIMARY KEY,
			name TEXT,
			width INTEGER NOT NULL,
			height INTEGER NOT NULL,
			is_overworld INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE phaser_tile_images (
			id INTEGER PRIMARY KEY,
			image_path TEXT NOT NULL
		);
		CREATE TABLE phaser_tile_properties (
			tile_image_id INTEGER PRIMARY KEY,
			name TEXT,
			collision_type INTEGER,
			is_user_editable INTEGER
		);
		CREATE TABLE phaser_tiles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			x INTEGER NOT NULL,
			y INTEGER NOT NULL,
			tile_image_id INTEGER NOT NULL,
			local_x INTEGER,
			local_y INTEGER,
			map_id INTEGER,
			source_map_id INTEGER,
			collision_type INTEGER DEFAULT 0,
			raw_foot_tile_id INTEGER,
			talk_over_tile BOOLEAN NOT NULL DEFAULT false,
			encounter_area_id INTEGER,
			is_native_game_data BOOLEAN NOT NULL DEFAULT false,
			coordinate_origin TEXT NOT NULL DEFAULT 'user',
			content_origin TEXT NOT NULL DEFAULT 'user',
			is_original_tile_location INTEGER NOT NULL DEFAULT 0,
			has_tile_edit INTEGER NOT NULL DEFAULT 0,
			is_tile_erased INTEGER NOT NULL DEFAULT 0,
			original_tile_image_id INTEGER,
			original_collision_type INTEGER,
			original_raw_foot_tile_id INTEGER,
			original_talk_over_tile BOOLEAN,
			original_encounter_area_id INTEGER,
			original_local_x INTEGER,
			original_local_y INTEGER,
			original_source_map_id INTEGER,
			last_edited_by_char_id INTEGER,
			last_edited_at TEXT,
			last_edit_source TEXT,
			is_user_placed INTEGER NOT NULL DEFAULT 0,
			placed_by_char_id INTEGER,
			placed_at TEXT
		);
		CREATE UNIQUE INDEX phaser_tiles_coord_unique_idx ON phaser_tiles (x, y, COALESCE(map_id, -1));

		INSERT INTO account (id, status) VALUES (1, 0), (2, 1);
		INSERT INTO phaser_maps (id, name, width, height, is_overworld) VALUES (40, 'TEST_MAP', 10, 10, 0);
		INSERT INTO phaser_tile_images (id, image_path) VALUES
			(1, 'tile_0.png'), (2, 'tile_1.png'), (3, 'tile_2.png');
		INSERT INTO phaser_tile_properties (tile_image_id, name, collision_type, is_user_editable)
		VALUES (1, 'grass', 1, 1), (2, 'wall', 0, 1), (3, 'floor', 1, 1);
		INSERT INTO phaser_tiles (
			x, y, tile_image_id, map_id, collision_type, is_original_tile_location,
			is_native_game_data, coordinate_origin, content_origin,
			has_tile_edit, is_tile_erased, original_tile_image_id, original_collision_type,
			original_talk_over_tile
		) VALUES (5, 5, 1, NULL, 1, 1, true, 'native', 'native', 0, 0, 1, 1, false);
	`
	if _, err := raw.Exec(schema); err != nil {
		raw.Close()
		t.Fatalf("seed tile mutation db: %v", err)
	}

	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: raw}
	t.Cleanup(func() {
		db.GlobalWorldDB = previous
		raw.Close()
	})
	return raw
}

func TestWorldTilePlacementInvalidatesOverworldOverviewChunk(t *testing.T) {
	raw := setupWorldTileMutationDB(t)
	tileDir := t.TempDir()
	writeOverviewTestTile(t, filepath.Join(tileDir, "tile_0.png"), color.RGBA{R: 255, A: 255})
	writeOverviewTestTile(t, filepath.Join(tileDir, "tile_1.png"), color.RGBA{B: 255, A: 255})
	writeOverviewTestTile(t, filepath.Join(tileDir, "tile_2.png"), color.RGBA{G: 255, A: 255})
	service, err := overworldoverview.NewService(raw, tileDir, strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("new overview service: %v", err)
	}
	overworldoverview.SetDefault(service)
	t.Cleanup(func() { overworldoverview.SetDefault(nil) })

	before, err := service.RenderChunk(0, 0)
	if err != nil {
		t.Fatalf("render overview before placement: %v", err)
	}

	if _, err := applyWorldTilePlacements(nil, nil, UnifiedOverworldMapID, []TileEdit{
		{X: 5, Y: 5, TileImageID: 2},
	}, "test"); err != nil {
		t.Fatalf("apply placement: %v", err)
	}
	after, err := service.RenderChunk(0, 0)
	if err != nil {
		t.Fatalf("render overview after placement: %v", err)
	}
	if after.ETag == before.ETag {
		t.Fatal("overview chunk retained its ETag after committed placement")
	}
	decoded, err := png.Decode(bytes.NewReader(after.PNG))
	if err != nil {
		t.Fatalf("decode updated overview: %v", err)
	}
	got := color.RGBAModel.Convert(decoded.At(5*overworldoverview.PixelsPerTile, 5*overworldoverview.PixelsPerTile)).(color.RGBA)
	if got != (color.RGBA{B: 255, A: 255}) {
		t.Fatalf("updated overview pixel = %#v, want blue", got)
	}
}

func writeOverviewTestTile(t *testing.T, filename string, tileColor color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetRGBA(x, y, tileColor)
		}
	}
	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("create overview test tile: %v", err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatalf("encode overview test tile: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close overview test tile: %v", err)
	}
}

func TestWorldTilePlacementPreservesOriginalMetadata(t *testing.T) {
	raw := setupWorldTileMutationDB(t)

	result, err := applyWorldTilePlacements(nil, nil, UnifiedOverworldMapID, []TileEdit{
		{X: 5, Y: 5, TileImageID: 2},
	}, "test")
	if err != nil {
		t.Fatalf("apply placement: %v", err)
	}
	if result.Changed != 1 || len(result.Edits) != 1 {
		t.Fatalf("placement result = %+v, want one edit", result)
	}

	var currentTile, originalTile, isOriginal, hasEdit, isErased int
	var source string
	if err := raw.QueryRow(`
		SELECT tile_image_id, original_tile_image_id, is_original_tile_location,
			has_tile_edit, is_tile_erased, last_edit_source
		FROM phaser_tiles
		WHERE map_id IS NULL AND x = 5 AND y = 5`,
	).Scan(&currentTile, &originalTile, &isOriginal, &hasEdit, &isErased, &source); err != nil {
		t.Fatalf("query edited tile: %v", err)
	}
	if currentTile != 2 || originalTile != 1 || isOriginal != 1 || hasEdit != 1 || isErased != 0 || source != "test" {
		t.Fatalf("edited original row = current:%d original:%d isOriginal:%d hasEdit:%d isErased:%d source:%q",
			currentTile, originalTile, isOriginal, hasEdit, isErased, source)
	}
}

func TestWorldTileEraseOriginalKeepsTombstone(t *testing.T) {
	raw := setupWorldTileMutationDB(t)

	result, err := applyWorldTileErasures(nil, nil, UnifiedOverworldMapID, []tileCoord{{X: 5, Y: 5}}, "test")
	if err != nil {
		t.Fatalf("apply erase: %v", err)
	}
	if result.Changed != 1 || !result.Edits[0].Erased || result.Edits[0].TileImageID != 0 {
		t.Fatalf("erase result = %+v, want erased broadcast", result)
	}

	var rows, hasEdit, isErased, originalTile int
	if err := raw.QueryRow(`
		SELECT COUNT(*), MAX(has_tile_edit), MAX(is_tile_erased), MAX(original_tile_image_id)
		FROM phaser_tiles
		WHERE map_id IS NULL AND x = 5 AND y = 5`,
	).Scan(&rows, &hasEdit, &isErased, &originalTile); err != nil {
		t.Fatalf("query tombstone: %v", err)
	}
	if rows != 1 || hasEdit != 1 || isErased != 1 || originalTile != 1 {
		t.Fatalf("tombstone = rows:%d hasEdit:%d isErased:%d original:%d", rows, hasEdit, isErased, originalTile)
	}
	if _, ok, err := currentWorldTileImageID(UnifiedOverworldMapID, 5, 5); err != nil || ok {
		t.Fatalf("current tile after tombstone = ok:%v err:%v, want hidden", ok, err)
	}
}

func TestWorldTileErasePlacedTileDeletesRow(t *testing.T) {
	raw := setupWorldTileMutationDB(t)

	if _, err := applyWorldTilePlacements(nil, nil, UnifiedOverworldMapID, []TileEdit{
		{X: 8, Y: 8, TileImageID: 3},
	}, "test"); err != nil {
		t.Fatalf("place new tile: %v", err)
	}
	if _, err := applyWorldTileErasures(nil, nil, UnifiedOverworldMapID, []tileCoord{{X: 8, Y: 8}}, "test"); err != nil {
		t.Fatalf("erase new tile: %v", err)
	}

	var rows int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM phaser_tiles WHERE map_id IS NULL AND x = 8 AND y = 8`).Scan(&rows); err != nil {
		t.Fatalf("count erased placed tile: %v", err)
	}
	if rows != 0 {
		t.Fatalf("placed tile rows after erase = %d, want 0", rows)
	}
}

func TestPhaserTilesRequestReturnsEditedTilesAndHidesErasedOriginals(t *testing.T) {
	raw := setupWorldTileMutationDB(t)
	if _, err := raw.Exec(`
		INSERT INTO phaser_tiles (
			x, y, tile_image_id, map_id, collision_type, is_original_tile_location,
			has_tile_edit, is_tile_erased, original_tile_image_id, original_collision_type
		) VALUES (6, 6, 1, NULL, 1, 1, 0, 0, 1, 1)`,
	); err != nil {
		t.Fatalf("seed second original tile: %v", err)
	}

	if _, err := applyWorldTilePlacements(nil, nil, UnifiedOverworldMapID, []TileEdit{
		{X: 5, Y: 5, TileImageID: 2},
	}, "test"); err != nil {
		t.Fatalf("edit original tile: %v", err)
	}
	if _, err := applyWorldTileErasures(nil, nil, UnifiedOverworldMapID, []tileCoord{{X: 6, Y: 6}}, "test"); err != nil {
		t.Fatalf("erase original tile: %v", err)
	}

	messenger := &recordingMessenger{}
	ses := session.NewSessionManager().CreateSession(messenger, 1, "test", nil)
	ses.Authenticated = true
	payload, err := json.Marshal(PhaserTilesRequest{MapID: UnifiedOverworldMapID})
	if err != nil {
		t.Fatalf("marshal tiles request: %v", err)
	}

	HandlePhaserTilesRequest(ses, payload, nil)

	if len(messenger.streams) != 1 {
		t.Fatalf("messages = %d, want one tile response", len(messenger.streams))
	}
	if messenger.streams[0].opcode != opcodes.PhaserTilesResponse {
		t.Fatalf("opcode = %d, want PhaserTilesResponse", messenger.streams[0].opcode)
	}
	responseJSON := string(messenger.streams[0].payload)
	if strings.Contains(responseJSON, `"sourceMapName":null`) || strings.Contains(responseJSON, `"rawFootTileId":null`) {
		t.Fatalf("tile response retained omitted nullable fields: %s", responseJSON)
	}
	if !strings.Contains(responseJSON, `"tileImageId":`) || !strings.Contains(responseJSON, `"isNativeGameData":`) {
		t.Fatalf("tile response does not use the camelCase wire contract: %s", responseJSON)
	}
	var tiles []PhaserTile
	if err := json.Unmarshal(messenger.streams[0].payload, &tiles); err != nil {
		t.Fatalf("decode tiles response: %v", err)
	}
	byCoord := make(map[tileCoord]PhaserTile, len(tiles))
	for _, tile := range tiles {
		byCoord[tileCoord{X: tile.X, Y: tile.Y}] = tile
	}
	if got, ok := byCoord[tileCoord{X: 5, Y: 5}]; !ok || got.TileImageID != 2 {
		t.Fatalf("edited tile = %+v, present=%v; want tile image 2", got, ok)
	}
	if got, ok := byCoord[tileCoord{X: 6, Y: 6}]; ok {
		t.Fatalf("erased original was returned: %+v", got)
	}
}

func TestPhaserTilesRequestSupportsBoundedCorrelatedPages(t *testing.T) {
	raw := setupWorldTileMutationDB(t)
	if _, err := raw.Exec(`
		INSERT INTO phaser_tiles (
			x, y, tile_image_id, map_id, collision_type, is_original_tile_location,
			is_native_game_data, coordinate_origin, content_origin
		) VALUES
			(20, 20, 1, NULL, 1, 1, 1, 'native', 'native'),
			(21, 20, 2, NULL, 1, 1, 1, 'native', 'native'),
			(40, 40, 2, NULL, 1, 1, 1, 'native', 'native')`,
	); err != nil {
		t.Fatalf("seed paged tiles: %v", err)
	}

	messenger := &recordingMessenger{}
	ses := session.NewSessionManager().CreateSession(messenger, 1, "test", nil)
	requestID := "near-player"
	minX, minY, maxX, maxY := 20, 20, 21, 20
	limit := 1
	payload, err := json.Marshal(PhaserTilesRequest{
		MapID: UnifiedOverworldMapID, RequestID: requestID,
		MinX: &minX, MinY: &minY, MaxX: &maxX, MaxY: &maxY, Limit: &limit,
	})
	if err != nil {
		t.Fatalf("marshal bounded tiles request: %v", err)
	}
	HandlePhaserTilesRequest(ses, payload, nil)

	if len(messenger.streams) != 1 {
		t.Fatalf("messages = %d, want one", len(messenger.streams))
	}
	var first PhaserTilesResponse
	if err := json.Unmarshal(messenger.streams[0].payload, &first); err != nil {
		t.Fatalf("decode bounded response: %v", err)
	}
	if first.RequestID != requestID || first.MapID != UnifiedOverworldMapID {
		t.Fatalf("response correlation = (%q, %d), want (%q, %d)", first.RequestID, first.MapID, requestID, UnifiedOverworldMapID)
	}
	if len(first.Tiles) != 1 || first.Tiles[0].X != 20 || first.Tiles[0].Y != 20 || !first.HasMore {
		t.Fatalf("first page = %+v, want one bounded tile and hasMore", first)
	}

	messenger.streams = nil
	afterID := first.NextAfterID
	requestID = "near-player-2"
	payload, err = json.Marshal(PhaserTilesRequest{
		MapID: UnifiedOverworldMapID, RequestID: requestID,
		MinX: &minX, MinY: &minY, MaxX: &maxX, MaxY: &maxY,
		AfterID: &afterID, Limit: &limit,
	})
	if err != nil {
		t.Fatalf("marshal second tiles request: %v", err)
	}
	HandlePhaserTilesRequest(ses, payload, nil)
	var second PhaserTilesResponse
	if err := json.Unmarshal(messenger.streams[0].payload, &second); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if len(second.Tiles) != 1 || second.Tiles[0].X != 21 || second.Tiles[0].Y != 20 {
		t.Fatalf("second page = %+v, want tile (21,20)", second)
	}
}

func TestTileEditorBroadcastsLiveChangesAndLaterJoinLoadsPersistedMap(t *testing.T) {
	setupWorldTileMutationDB(t)

	sessionManager := session.NewSessionManager()
	originMessenger := &recordingMessenger{}
	viewerMessenger := &recordingMessenger{}
	laterMessenger := &recordingMessenger{}

	origin := sessionManager.CreateSession(originMessenger, 1, "origin", nil)
	origin.Authenticated = true
	origin.AccountID = 2
	origin.MapID = UnifiedOverworldMapID
	origin.Client = &testSessionClient{char: &model.CharacterData{ID: 101, Name: "Admin", Gm: 1, MapID: UnifiedOverworldMapID}}

	viewer := sessionManager.CreateSession(viewerMessenger, 2, "viewer", nil)
	viewer.Authenticated = true
	viewer.MapID = UnifiedOverworldMapID

	wh := &WorldHandler{sessionManager: sessionManager}

	placePayload, err := json.Marshal(TileEditorPlaceReq{
		MapID: UnifiedOverworldMapID,
		Tiles: []TileEdit{
			{X: 9, Y: 9, TileImageID: 3},
		},
	})
	if err != nil {
		t.Fatalf("marshal place request: %v", err)
	}
	HandleTileEditorPlace(origin, placePayload, wh)

	broadcasts := tileEditorBroadcastsFromMessenger(t, viewerMessenger)
	if len(broadcasts) != 1 {
		t.Fatalf("viewer broadcasts after place = %d, want 1", len(broadcasts))
	}
	if broadcasts[0].MapID != UnifiedOverworldMapID || len(broadcasts[0].Tiles) != 1 {
		t.Fatalf("place broadcast = %+v, want one overworld tile", broadcasts[0])
	}
	placed := broadcasts[0].Tiles[0]
	if placed.X != 9 || placed.Y != 9 || placed.TileImageID != 3 || placed.Erased {
		t.Fatalf("place broadcast tile = %+v, want placed tile 3 at (9,9)", placed)
	}

	erasePayload, err := json.Marshal(TileEditorEraseReq{
		MapID: UnifiedOverworldMapID,
		Tiles: []struct {
			X int `json:"x"`
			Y int `json:"y"`
		}{
			{X: 5, Y: 5},
		},
	})
	if err != nil {
		t.Fatalf("marshal erase request: %v", err)
	}
	HandleTileEditorErase(origin, erasePayload, wh)

	broadcasts = tileEditorBroadcastsFromMessenger(t, viewerMessenger)
	if len(broadcasts) != 2 {
		t.Fatalf("viewer broadcasts after erase = %d, want 2 total", len(broadcasts))
	}
	erased := broadcasts[1].Tiles[0]
	if erased.X != 5 || erased.Y != 5 || erased.TileImageID != 0 || !erased.Erased {
		t.Fatalf("erase broadcast tile = %+v, want erased tile at (5,5)", erased)
	}

	later := sessionManager.CreateSession(laterMessenger, 3, "later", nil)
	later.Authenticated = true
	later.MapID = UnifiedOverworldMapID
	requestPayload, err := json.Marshal(PhaserTilesRequest{MapID: UnifiedOverworldMapID})
	if err != nil {
		t.Fatalf("marshal tiles request: %v", err)
	}
	HandlePhaserTilesRequest(later, requestPayload, wh)

	tiles := phaserTilesResponseFromMessenger(t, laterMessenger)
	byCoord := make(map[tileCoord]PhaserTile, len(tiles))
	for _, tile := range tiles {
		byCoord[tileCoord{X: tile.X, Y: tile.Y}] = tile
	}
	if got, ok := byCoord[tileCoord{X: 9, Y: 9}]; !ok || got.TileImageID != 3 {
		t.Fatalf("later loaded placed tile = %+v, present=%v; want tile image 3", got, ok)
	}
	if got, ok := byCoord[tileCoord{X: 5, Y: 5}]; ok {
		t.Fatalf("later loaded erased original tile: %+v", got)
	}
}

func TestTileEditorRawEditRequiresAdmin(t *testing.T) {
	raw := setupWorldTileMutationDB(t)
	messenger := &recordingMessenger{}
	ses := session.NewSessionManager().CreateSession(messenger, 1, "test", nil)
	ses.Authenticated = true
	ses.AccountID = 1
	ses.Client = &testSessionClient{char: &model.CharacterData{ID: 99, Name: "Player", Gm: 0}}

	payload, err := json.Marshal(TileEditorPlaceReq{
		MapID: UnifiedOverworldMapID,
		Tiles: []TileEdit{{X: 7, Y: 7, TileImageID: 3}},
	})
	if err != nil {
		t.Fatalf("marshal place request: %v", err)
	}

	HandleTileEditorPlace(ses, payload, &WorldHandler{})

	if len(messenger.streams) != 1 {
		t.Fatalf("messages = %d, want one rejection", len(messenger.streams))
	}
	if messenger.streams[0].opcode != opcodes.TileEditorPlaceResponse {
		t.Fatalf("opcode = %d, want TileEditorPlaceResponse", messenger.streams[0].opcode)
	}
	var response map[string]any
	if err := json.Unmarshal(messenger.streams[0].payload, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["success"] != false {
		t.Fatalf("response = %#v, want success false", response)
	}
	var rows int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM phaser_tiles WHERE map_id IS NULL AND x = 7 AND y = 7`).Scan(&rows); err != nil {
		t.Fatalf("count unauthorized edit: %v", err)
	}
	if rows != 0 {
		t.Fatalf("unauthorized edit inserted %d rows", rows)
	}
}

func tileEditorBroadcastsFromMessenger(t *testing.T, messenger *recordingMessenger) []TileEditorBroadcastPayload {
	t.Helper()
	var broadcasts []TileEditorBroadcastPayload
	for _, stream := range messenger.streams {
		if stream.opcode != opcodes.TileEditorBroadcast {
			continue
		}
		var payload TileEditorBroadcastPayload
		if err := json.Unmarshal(stream.payload, &payload); err != nil {
			t.Fatalf("decode tile editor broadcast: %v", err)
		}
		broadcasts = append(broadcasts, payload)
	}
	return broadcasts
}

func phaserTilesResponseFromMessenger(t *testing.T, messenger *recordingMessenger) []PhaserTile {
	t.Helper()
	for _, stream := range messenger.streams {
		if stream.opcode != opcodes.PhaserTilesResponse {
			continue
		}
		var tiles []PhaserTile
		if err := json.Unmarshal(stream.payload, &tiles); err != nil {
			t.Fatalf("decode phaser tiles response: %v", err)
		}
		return tiles
	}
	t.Fatalf("missing PhaserTilesResponse in %d stream messages", len(messenger.streams))
	return nil
}
