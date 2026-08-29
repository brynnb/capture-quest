package overworldoverview

import (
	"bytes"
	"database/sql"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOverviewChunkRendersSignedCoordinatesAndInvalidates(t *testing.T) {
	database, tileDir := setupOverviewFixture(t)
	service, err := NewService(database, tileDir, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	first, err := service.RenderChunk(-1, -1)
	if err != nil {
		t.Fatalf("render initial chunk: %v", err)
	}
	assertChunkPixel(t, first.PNG, chunkPixelSize-PixelsPerTile, chunkPixelSize-PixelsPerTile, color.RGBA{R: 255, A: 255})

	if _, err := database.Exec(`UPDATE phaser_tiles SET tile_image_id = 2 WHERE x = -1 AND y = -1`); err != nil {
		t.Fatalf("edit tile: %v", err)
	}
	// The cached image is intentionally stable until the post-commit mutation
	// hook invalidates its aligned chunk.
	cached, err := service.RenderChunk(-1, -1)
	if err != nil {
		t.Fatalf("render cached chunk: %v", err)
	}
	if cached.ETag != first.ETag {
		t.Fatalf("cached ETag changed before invalidation: %s != %s", cached.ETag, first.ETag)
	}

	service.InvalidateCoordinates([]Coordinate{{X: -1, Y: -1}})
	second, err := service.RenderChunk(-1, -1)
	if err != nil {
		t.Fatalf("render invalidated chunk: %v", err)
	}
	assertChunkPixel(t, second.PNG, chunkPixelSize-PixelsPerTile, chunkPixelSize-PixelsPerTile, color.RGBA{B: 255, A: 255})
	if second.ETag == first.ETag {
		t.Fatalf("ETag did not change after tile edit: %s", second.ETag)
	}
}

func TestOverviewHTTPRevalidatesStrongETag(t *testing.T) {
	database, tileDir := setupOverviewFixture(t)
	service, err := NewService(database, tileDir, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	firstRequest := httptest.NewRequest(http.MethodGet, "/api/overworld/overview?chunkX=-1&chunkY=-1", nil)
	firstResponse := httptest.NewRecorder()
	service.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("initial response status = %d, want 200: %s", firstResponse.Code, firstResponse.Body.String())
	}
	if got := firstResponse.Header().Get("Cache-Control"); got != "public, no-cache" {
		t.Fatalf("Cache-Control = %q, want public, no-cache", got)
	}
	if got := firstResponse.Header().Get("X-Overworld-Chunk-Tile-Span"); got != "64" {
		t.Fatalf("tile span header = %q, want 64", got)
	}
	etag := firstResponse.Header().Get("ETag")
	if etag == "" {
		t.Fatal("initial response has no ETag")
	}

	revalidateRequest := httptest.NewRequest(http.MethodGet, "/api/overworld/overview?chunkX=-1&chunkY=-1", nil)
	revalidateRequest.Header.Set("If-None-Match", etag)
	revalidateResponse := httptest.NewRecorder()
	service.ServeHTTP(revalidateResponse, revalidateRequest)
	if revalidateResponse.Code != http.StatusNotModified {
		t.Fatalf("revalidation status = %d, want 304", revalidateResponse.Code)
	}

	if _, err := database.Exec(`UPDATE phaser_tiles SET tile_image_id = 2 WHERE x = -1 AND y = -1`); err != nil {
		t.Fatalf("edit tile: %v", err)
	}
	service.InvalidateCoordinates([]Coordinate{{X: -1, Y: -1}})
	updatedRequest := httptest.NewRequest(http.MethodGet, "/api/overworld/overview?chunkX=-1&chunkY=-1", nil)
	updatedRequest.Header.Set("If-None-Match", etag)
	updatedResponse := httptest.NewRecorder()
	service.ServeHTTP(updatedResponse, updatedRequest)
	if updatedResponse.Code != http.StatusOK {
		t.Fatalf("post-edit response status = %d, want 200: %s", updatedResponse.Code, updatedResponse.Body.String())
	}
	if updatedResponse.Header().Get("ETag") == etag {
		t.Fatal("post-edit response reused stale ETag")
	}
}

func TestEmptyOverviewChunkIsTransparentAndCacheable(t *testing.T) {
	database, tileDir := setupOverviewFixture(t)
	service, err := NewService(database, tileDir, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	chunk, err := service.RenderChunk(0, 0)
	if err != nil {
		t.Fatalf("render empty chunk: %v", err)
	}
	assertChunkPixel(t, chunk.PNG, 0, 0, color.RGBA{})

	request := httptest.NewRequest(http.MethodGet, "/api/overworld/overview?chunkX=0&chunkY=0", nil)
	response := httptest.NewRecorder()
	service.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("empty chunk response status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("ETag") == "" {
		t.Fatal("empty chunk response has no ETag")
	}
}

func TestOverviewHTTPRejectsStaleCatalogAndOutOfWorldChunks(t *testing.T) {
	database, tileDir := setupOverviewFixture(t)
	catalog := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	service, err := NewService(database, tileDir, catalog)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	staleRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/overworld/overview?chunkX=-1&chunkY=-1&catalog="+strings.Repeat("e", 64),
		nil,
	)
	staleResponse := httptest.NewRecorder()
	service.ServeHTTP(staleResponse, staleRequest)
	if staleResponse.Code != http.StatusConflict {
		t.Fatalf("stale catalog status = %d, want 409", staleResponse.Code)
	}
	if got := staleResponse.Header().Get("X-Overworld-Tile-Catalog"); got != catalog {
		t.Fatalf("catalog response header = %q, want %q", got, catalog)
	}

	outsideRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/overworld/overview?chunkX=500&chunkY=500&catalog="+catalog,
		nil,
	)
	outsideResponse := httptest.NewRecorder()
	service.ServeHTTP(outsideResponse, outsideRequest)
	if outsideResponse.Code != http.StatusNotFound {
		t.Fatalf("outside-world status = %d, want 404", outsideResponse.Code)
	}
}

func TestCatalogVersionParticipatesInETag(t *testing.T) {
	database, tileDir := setupOverviewFixture(t)
	first, err := NewService(database, tileDir, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("new first service: %v", err)
	}
	second, err := NewService(database, tileDir, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatalf("new second service: %v", err)
	}
	firstChunk, err := first.RenderChunk(-1, -1)
	if err != nil {
		t.Fatalf("render first chunk: %v", err)
	}
	secondChunk, err := second.RenderChunk(-1, -1)
	if err != nil {
		t.Fatalf("render second chunk: %v", err)
	}
	if firstChunk.ETag == secondChunk.ETag {
		t.Fatal("different runtime tile catalogs produced the same ETag")
	}
}

func TestProductionCatalogCandidatesNeverFallBackToPublicAssets(t *testing.T) {
	t.Setenv("CAPTUREQUEST_TILE_IMAGE_DIR", "")
	production := runtimeTileImageCandidates(true)
	if len(production) == 0 {
		t.Fatal("production catalog candidates are empty")
	}
	for _, candidate := range production {
		if strings.Contains(candidate, "public") {
			t.Fatalf("production candidate %q can pair the database with stale public assets", candidate)
		}
	}
	local := runtimeTileImageCandidates(false)
	if len(local) == 0 || !strings.Contains(local[0], "public") {
		t.Fatalf("local catalog candidates = %v, want public assets first", local)
	}
}

func TestRuntimeOverviewServiceRequiresOneVerifiedCustomAssetFamily(t *testing.T) {
	database, tileDir := setupOverviewFixture(t)
	t.Setenv("CAPTUREQUEST_TILE_IMAGE_DIR", tileDir)
	t.Setenv("CAPTUREQUEST_RUNTIME_ASSET_CONTRACT", "")
	if _, err := NewRuntimeService(database, false); err == nil || !strings.Contains(err.Error(), "must be set together") {
		t.Fatalf("lone tile-directory override error = %v, want paired-override rejection", err)
	}

	contractPath := filepath.Join(t.TempDir(), "runtime_asset_contract.json")
	t.Setenv("CAPTUREQUEST_RUNTIME_ASSET_CONTRACT", contractPath)
	if err := os.WriteFile(contractPath, []byte(`{"tileCatalogSha256":"`+strings.Repeat("e", 64)+`"}`), 0o644); err != nil {
		t.Fatalf("write mismatched contract: %v", err)
	}
	if _, err := NewRuntimeService(database, false); err == nil || !strings.Contains(err.Error(), "does not match contract") {
		t.Fatalf("mismatched custom catalog error = %v, want hash rejection", err)
	}

	catalogHash, err := tileCatalogSHA256(tileDir)
	if err != nil {
		t.Fatalf("hash fixture catalog: %v", err)
	}
	if err := os.WriteFile(contractPath, []byte(`{"tileCatalogSha256":"`+catalogHash+`"}`), 0o644); err != nil {
		t.Fatalf("write matching contract: %v", err)
	}
	if _, err := NewRuntimeService(database, false); err != nil {
		t.Fatalf("matching custom asset family: %v", err)
	}
}

func setupOverviewFixture(t *testing.T) (*sql.DB, string) {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { database.Close() })
	if _, err := database.Exec(`
		CREATE TABLE phaser_tile_images (
			id INTEGER PRIMARY KEY,
			image_path TEXT NOT NULL
		);
		CREATE TABLE phaser_tiles (
			id INTEGER PRIMARY KEY,
			x INTEGER NOT NULL,
			y INTEGER NOT NULL,
			tile_image_id INTEGER NOT NULL,
			map_id INTEGER,
			is_tile_erased INTEGER NOT NULL DEFAULT 0
		);
		INSERT INTO phaser_tile_images (id, image_path) VALUES
			(1, 'tile_images/tile_0.png'),
			(2, 'tile_images/tile_1.png');
		INSERT INTO phaser_tiles (id, x, y, tile_image_id, map_id, is_tile_erased)
		VALUES
			(1, -1, -1, 1, NULL, 0),
			(2, 64, 64, 2, NULL, 0);
	`); err != nil {
		t.Fatalf("seed database: %v", err)
	}

	tileDir := t.TempDir()
	writeSolidTile(t, filepath.Join(tileDir, "tile_0.png"), color.RGBA{R: 255, A: 255})
	writeSolidTile(t, filepath.Join(tileDir, "tile_1.png"), color.RGBA{B: 255, A: 255})
	return database, tileDir
}

func writeSolidTile(t *testing.T, filename string, tileColor color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetRGBA(x, y, tileColor)
		}
	}
	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("create tile image: %v", err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatalf("encode tile image: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close tile image: %v", err)
	}
}

func assertChunkPixel(t *testing.T, pngBytes []byte, x, y int, want color.RGBA) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("decode overview PNG: %v", err)
	}
	got := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
	if got != want {
		t.Fatalf("overview pixel (%d,%d) = %#v, want %#v", x, y, got, want)
	}
}
