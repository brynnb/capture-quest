// Package overworldoverview renders low-resolution, cacheable overworld image
// chunks from the authoritative runtime tile rows.
package overworldoverview

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"
)

const (
	// ChunkTileSpan is deliberately aligned with the exact-tile streaming grid.
	ChunkTileSpan = 64
	// PixelsPerTile keeps the overview compact while retaining recognizable map
	// structure. Exact 16x16 tiles replace this image near the camera.
	PixelsPerTile   = 4
	chunkPixelSize  = ChunkTileSpan * PixelsPerTile
	maxChunkIndex   = 1 << 20
	maxCachedChunks = 256
)

var (
	ErrChunkIndexOutOfRange = errors.New("overworld overview chunk index out of range")
	ErrChunkOutsideWorld    = errors.New("overworld overview chunk is outside world bounds")
)

type Coordinate struct {
	X int
	Y int
}

type Chunk struct {
	PNG  []byte
	ETag string
}

type chunkKey struct {
	X int
	Y int
}

type cachedChunk struct {
	chunk    Chunk
	lastUsed uint64
}

type chunkRevision struct {
	all   uint64
	chunk uint64
}

type tileRecord struct {
	x         int
	y         int
	tileImage int
	imagePath string
}

// Service owns the bounded in-process PNG and decoded-tile caches. The source
// rows remain in Postgres; this cache is disposable and is invalidated only
// after a tile mutation commits successfully.
type Service struct {
	db             *sql.DB
	tileImageDir   string
	catalogVersion string

	chunkMu     sync.Mutex
	chunks      map[chunkKey]cachedChunk
	clock       uint64
	allRevision uint64
	revisions   map[chunkKey]uint64

	tileMu        sync.RWMutex
	tileImages    map[int]image.Image
	tileRevisions map[int]uint64

	minChunkX int
	minChunkY int
	maxChunkX int
	maxChunkY int
	renders   singleflight.Group
}

func NewService(database *sql.DB, tileImageDir, catalogVersion string) (*Service, error) {
	if database == nil {
		return nil, errors.New("overworld overview database is required")
	}
	tileImageDir = filepath.Clean(strings.TrimSpace(tileImageDir))
	if tileImageDir == "." || tileImageDir == "" {
		return nil, errors.New("overworld overview tile image directory is required")
	}
	if info, err := os.Stat(tileImageDir); err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("not a directory")
		}
		return nil, fmt.Errorf("open overworld overview tile catalog %q: %w", tileImageDir, err)
	}
	catalogVersion = strings.TrimSpace(catalogVersion)
	if catalogVersion == "" {
		return nil, errors.New("overworld overview tile catalog version is required")
	}
	var minX, minY, maxX, maxY sql.NullInt64
	if err := database.QueryRow(`
		SELECT MIN(x), MIN(y), MAX(x), MAX(y)
		FROM phaser_tiles
		WHERE map_id IS NULL AND is_tile_erased = 0`).Scan(
		&minX, &minY, &maxX, &maxY,
	); err != nil {
		return nil, fmt.Errorf("read overworld overview bounds: %w", err)
	}
	if !minX.Valid || !minY.Valid || !maxX.Valid || !maxY.Valid {
		return nil, errors.New("overworld overview has no durable tile bounds")
	}
	return &Service{
		db:             database,
		tileImageDir:   tileImageDir,
		catalogVersion: catalogVersion,
		chunks:         make(map[chunkKey]cachedChunk),
		revisions:      make(map[chunkKey]uint64),
		tileImages:     make(map[int]image.Image),
		tileRevisions:  make(map[int]uint64),
		minChunkX:      floorDiv(int(minX.Int64), ChunkTileSpan),
		minChunkY:      floorDiv(int(minY.Int64), ChunkTileSpan),
		maxChunkX:      floorDiv(int(maxX.Int64), ChunkTileSpan),
		maxChunkY:      floorDiv(int(maxY.Int64), ChunkTileSpan),
	}, nil
}

type runtimeAssetContract struct {
	TileCatalogSHA256 string `json:"tileCatalogSha256"`
}

// NewRuntimeService resolves the atomic tile-image/contract family in both the
// deployed layout (dist) and the local development layout (public).
func NewRuntimeService(database *sql.DB, deployedLayout bool) (*Service, error) {
	configuredTileDir := strings.TrimSpace(os.Getenv("CAPTUREQUEST_TILE_IMAGE_DIR"))
	configuredContract := strings.TrimSpace(os.Getenv("CAPTUREQUEST_RUNTIME_ASSET_CONTRACT"))
	if (configuredTileDir == "") != (configuredContract == "") {
		return nil, errors.New("CAPTUREQUEST_TILE_IMAGE_DIR and CAPTUREQUEST_RUNTIME_ASSET_CONTRACT must be set together")
	}
	candidates := runtimeTileImageCandidates(deployedLayout)
	var failures []string
	for _, tileDir := range candidates {
		contractPath := configuredContract
		if contractPath == "" {
			contractPath = filepath.Join(filepath.Dir(tileDir), "runtime_asset_contract.json")
		}
		version, err := readCatalogVersion(contractPath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", tileDir, err))
			continue
		}
		actualVersion, err := tileCatalogSHA256(tileDir)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", tileDir, err))
			continue
		}
		if !strings.EqualFold(actualVersion, version) {
			failures = append(failures, fmt.Sprintf("%s: tile catalog hash %s does not match contract %s", tileDir, actualVersion, version))
			continue
		}
		service, err := NewService(database, tileDir, version)
		if err == nil {
			return service, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", tileDir, err))
	}
	return nil, fmt.Errorf("resolve overworld overview runtime catalog: %s", strings.Join(failures, "; "))
}

func tileCatalogSHA256(tileDir string) (string, error) {
	files, err := filepath.Glob(filepath.Join(tileDir, "tile_*.png"))
	if err != nil {
		return "", fmt.Errorf("enumerate tile catalog: %w", err)
	}
	if len(files) == 0 {
		return "", errors.New("tile catalog contains no tile_*.png files")
	}
	sort.Strings(files)
	digest := sha256.New()
	for _, filename := range files {
		data, err := os.ReadFile(filename)
		if err != nil {
			return "", fmt.Errorf("read tile catalog file %q: %w", filename, err)
		}
		fileDigest := sha256.Sum256(data)
		digest.Write([]byte(filepath.Base(filename)))
		digest.Write([]byte{0})
		digest.Write(fileDigest[:])
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func runtimeTileImageCandidates(deployedLayout bool) []string {
	if configured := strings.TrimSpace(os.Getenv("CAPTUREQUEST_TILE_IMAGE_DIR")); configured != "" {
		return []string{configured}
	}
	if !deployedLayout {
		return []string{
			"../public/phaser/tile_images",
			"public/phaser/tile_images",
			"../dist/phaser/tile_images",
			"dist/phaser/tile_images",
		}
	}
	// Production publishes one atomic frontend artifact under dist. Falling
	// back to a leftover public catalog can pair current database IDs with stale
	// pixels and recreate the exact live-site corruption this contract prevents.
	return []string{
		"../dist/phaser/tile_images",
		"dist/phaser/tile_images",
	}
}

func readCatalogVersion(contractPath string) (string, error) {
	data, err := os.ReadFile(contractPath)
	if err != nil {
		return "", fmt.Errorf("read runtime asset contract %q: %w", contractPath, err)
	}
	var contract runtimeAssetContract
	if err := json.Unmarshal(data, &contract); err != nil {
		return "", fmt.Errorf("decode runtime asset contract %q: %w", contractPath, err)
	}
	if len(contract.TileCatalogSHA256) != sha256.Size*2 {
		return "", fmt.Errorf("runtime asset contract %q has invalid tileCatalogSha256", contractPath)
	}
	if _, err := hex.DecodeString(contract.TileCatalogSHA256); err != nil {
		return "", fmt.Errorf("runtime asset contract %q has invalid tileCatalogSha256: %w", contractPath, err)
	}
	return strings.ToLower(contract.TileCatalogSHA256), nil
}

func (s *Service) RenderChunk(chunkX, chunkY int) (Chunk, error) {
	if chunkX < -maxChunkIndex || chunkX > maxChunkIndex || chunkY < -maxChunkIndex || chunkY > maxChunkIndex {
		return Chunk{}, ErrChunkIndexOutOfRange
	}
	if !s.containsChunk(chunkX, chunkY) {
		return Chunk{}, ErrChunkOutsideWorld
	}
	key := chunkKey{X: chunkX, Y: chunkY}
	if chunk, ok := s.cached(key); ok {
		return chunk, nil
	}
	value, err, _ := s.renders.Do(fmt.Sprintf("%d:%d", chunkX, chunkY), func() (interface{}, error) {
		if chunk, ok := s.cached(key); ok {
			return chunk, nil
		}
		return s.renderChunk(key)
	})
	if err != nil {
		return Chunk{}, err
	}
	return value.(Chunk), nil
}

func (s *Service) renderChunk(key chunkKey) (Chunk, error) {
	chunkX := key.X
	chunkY := key.Y
	revision := s.revision(key)

	minX := chunkX * ChunkTileSpan
	minY := chunkY * ChunkTileSpan
	maxX := minX + ChunkTileSpan - 1
	maxY := minY + ChunkTileSpan - 1
	rows, err := s.db.Query(`
		SELECT pt.x, pt.y, pt.tile_image_id, ti.image_path
		FROM phaser_tiles pt
		LEFT JOIN phaser_tile_images ti ON ti.id = pt.tile_image_id
		WHERE pt.map_id IS NULL
		  AND pt.is_tile_erased = 0
		  AND pt.x BETWEEN $1 AND $2
		  AND pt.y BETWEEN $3 AND $4
		ORDER BY pt.y, pt.x`, minX, maxX, minY, maxY)
	if err != nil {
		return Chunk{}, fmt.Errorf("query overworld overview chunk (%d,%d): %w", chunkX, chunkY, err)
	}
	defer rows.Close()

	records := make([]tileRecord, 0, ChunkTileSpan*ChunkTileSpan)
	for rows.Next() {
		var record tileRecord
		var imagePath sql.NullString
		if err := rows.Scan(&record.x, &record.y, &record.tileImage, &imagePath); err != nil {
			return Chunk{}, fmt.Errorf("scan overworld overview chunk (%d,%d): %w", chunkX, chunkY, err)
		}
		if !imagePath.Valid || strings.TrimSpace(imagePath.String) == "" {
			return Chunk{}, fmt.Errorf("overworld tile (%d,%d) references missing tile image %d", record.x, record.y, record.tileImage)
		}
		record.imagePath = imagePath.String
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return Chunk{}, fmt.Errorf("read overworld overview chunk (%d,%d): %w", chunkX, chunkY, err)
	}
	// Empty aligned chunks inside the overworld's rectangular bounds are valid:
	// the source maps do not occupy every square. Keep their pixels transparent
	// so one sparse chunk cannot make the complete overview load fail.
	canvas := image.NewRGBA(image.Rect(0, 0, chunkPixelSize, chunkPixelSize))
	for _, record := range records {
		tileImage, err := s.loadTileImage(record.tileImage, record.imagePath)
		if err != nil {
			return Chunk{}, fmt.Errorf("render overworld overview tile (%d,%d): %w", record.x, record.y, err)
		}
		destX := (record.x - minX) * PixelsPerTile
		destY := (record.y - minY) * PixelsPerTile
		drawNearest(canvas, destX, destY, tileImage)
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		return Chunk{}, fmt.Errorf("encode overworld overview chunk (%d,%d): %w", chunkX, chunkY, err)
	}
	pngBytes := encoded.Bytes()
	digest := sha256.New()
	digest.Write([]byte(s.catalogVersion))
	digest.Write([]byte{0})
	digest.Write(pngBytes)
	chunk := Chunk{
		PNG:  append([]byte(nil), pngBytes...),
		ETag: `"cq-overworld-v1-` + hex.EncodeToString(digest.Sum(nil)) + `"`,
	}
	if !s.storeIfCurrent(key, revision, chunk) {
		return s.renderChunk(key)
	}
	return chunk, nil
}

func (s *Service) cached(key chunkKey) (Chunk, bool) {
	s.chunkMu.Lock()
	defer s.chunkMu.Unlock()
	entry, ok := s.chunks[key]
	if !ok {
		return Chunk{}, false
	}
	s.clock++
	entry.lastUsed = s.clock
	s.chunks[key] = entry
	return entry.chunk, true
}

func (s *Service) revision(key chunkKey) chunkRevision {
	s.chunkMu.Lock()
	defer s.chunkMu.Unlock()
	return chunkRevision{all: s.allRevision, chunk: s.revisions[key]}
}

func (s *Service) storeIfCurrent(key chunkKey, revision chunkRevision, chunk Chunk) bool {
	s.chunkMu.Lock()
	defer s.chunkMu.Unlock()
	if revision.all != s.allRevision || revision.chunk != s.revisions[key] {
		return false
	}
	s.clock++
	if len(s.chunks) >= maxCachedChunks {
		var oldestKey chunkKey
		oldestUse := ^uint64(0)
		for candidate, entry := range s.chunks {
			if entry.lastUsed < oldestUse {
				oldestKey = candidate
				oldestUse = entry.lastUsed
			}
		}
		delete(s.chunks, oldestKey)
	}
	s.chunks[key] = cachedChunk{chunk: chunk, lastUsed: s.clock}
	return true
}

func (s *Service) loadTileImage(tileImageID int, imagePath string) (image.Image, error) {
	s.tileMu.RLock()
	img, ok := s.tileImages[tileImageID]
	revision := s.tileRevisions[tileImageID]
	s.tileMu.RUnlock()
	if ok {
		return img, nil
	}

	// image_path comes from the validated runtime catalog. Taking only its base
	// name ensures even malformed database content cannot escape tileImageDir.
	filename := filepath.Base(filepath.Clean(imagePath))
	if filename == "." || filename == string(filepath.Separator) || !strings.HasSuffix(strings.ToLower(filename), ".png") {
		return nil, fmt.Errorf("tile image %d has unsafe path %q", tileImageID, imagePath)
	}
	file, err := os.Open(filepath.Join(s.tileImageDir, filename))
	if err != nil {
		return nil, fmt.Errorf("open tile image %d (%s): %w", tileImageID, filename, err)
	}
	defer file.Close()
	decoded, err := png.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode tile image %d (%s): %w", tileImageID, filename, err)
	}
	if decoded.Bounds().Dx() != 16 || decoded.Bounds().Dy() != 16 {
		return nil, fmt.Errorf("tile image %d (%s) is %dx%d, want 16x16", tileImageID, filename, decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}

	s.tileMu.Lock()
	if s.tileRevisions[tileImageID] != revision {
		// A local tile-art replacement completed while this file was being
		// decoded. Return this image to the in-flight renderer, but never put
		// the superseded pixels back in the shared cache.
	} else if existing, exists := s.tileImages[tileImageID]; exists {
		decoded = existing
	} else {
		s.tileImages[tileImageID] = decoded
	}
	s.tileMu.Unlock()
	return decoded, nil
}

func drawNearest(dest *image.RGBA, destX, destY int, source image.Image) {
	bounds := source.Bounds()
	for y := 0; y < PixelsPerTile; y++ {
		sourceY := bounds.Min.Y + y*bounds.Dy()/PixelsPerTile
		for x := 0; x < PixelsPerTile; x++ {
			sourceX := bounds.Min.X + x*bounds.Dx()/PixelsPerTile
			dest.Set(destX+x, destY+y, source.At(sourceX, sourceY))
		}
	}
}

func (s *Service) InvalidateCoordinates(coords []Coordinate) {
	s.chunkMu.Lock()
	defer s.chunkMu.Unlock()
	for _, coord := range coords {
		key := chunkKey{X: floorDiv(coord.X, ChunkTileSpan), Y: floorDiv(coord.Y, ChunkTileSpan)}
		if key.X < s.minChunkX {
			s.minChunkX = key.X
		}
		if key.X > s.maxChunkX {
			s.maxChunkX = key.X
		}
		if key.Y < s.minChunkY {
			s.minChunkY = key.Y
		}
		if key.Y > s.maxChunkY {
			s.maxChunkY = key.Y
		}
		delete(s.chunks, key)
		s.revisions[key]++
	}
}

func (s *Service) containsChunk(chunkX, chunkY int) bool {
	s.chunkMu.Lock()
	defer s.chunkMu.Unlock()
	return chunkX >= s.minChunkX && chunkX <= s.maxChunkX && chunkY >= s.minChunkY && chunkY <= s.maxChunkY
}

func (s *Service) InvalidateTileImage(tileImageID int) {
	s.tileMu.Lock()
	delete(s.tileImages, tileImageID)
	s.tileRevisions[tileImageID]++
	s.tileMu.Unlock()
	s.InvalidateAll()
}

func (s *Service) InvalidateAll() {
	s.chunkMu.Lock()
	clear(s.chunks)
	clear(s.revisions)
	s.allRevision++
	s.chunkMu.Unlock()
}

func floorDiv(value, divisor int) int {
	quotient := value / divisor
	if value < 0 && value%divisor != 0 {
		quotient--
	}
	return quotient
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	chunkX, err := strconv.Atoi(r.URL.Query().Get("chunkX"))
	if err != nil {
		http.Error(w, "invalid chunkX", http.StatusBadRequest)
		return
	}
	chunkY, err := strconv.Atoi(r.URL.Query().Get("chunkY"))
	if err != nil {
		http.Error(w, "invalid chunkY", http.StatusBadRequest)
		return
	}
	w.Header().Set("X-Overworld-Tile-Catalog", s.catalogVersion)
	clientCatalog := strings.TrimSpace(r.URL.Query().Get("catalog"))
	if clientCatalog != "" && !strings.EqualFold(clientCatalog, s.catalogVersion) {
		http.Error(w, "client tile catalog does not match server", http.StatusConflict)
		return
	}
	chunk, err := s.RenderChunk(chunkX, chunkY)
	if err != nil {
		if errors.Is(err, ErrChunkIndexOutOfRange) {
			http.Error(w, "overview chunk index out of range", http.StatusBadRequest)
			return
		}
		if errors.Is(err, ErrChunkOutsideWorld) {
			http.Error(w, "overview chunk outside world bounds", http.StatusNotFound)
			return
		}
		log.Printf("[OverworldOverview] Failed rendering chunk (%d,%d): %v", chunkX, chunkY, err)
		http.Error(w, "failed to render overview chunk", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	// Stored responses are useful, but every reuse must revalidate so a manual
	// edit cannot remain stale behind a browser or reverse-proxy cache.
	w.Header().Set("Cache-Control", "public, no-cache")
	w.Header().Set("ETag", chunk.ETag)
	w.Header().Set("Content-Length", strconv.Itoa(len(chunk.PNG)))
	w.Header().Set("X-Overworld-Chunk-Tile-Span", strconv.Itoa(ChunkTileSpan))
	w.Header().Set("X-Overworld-Pixels-Per-Tile", strconv.Itoa(PixelsPerTile))
	if r.Header.Get("If-None-Match") == chunk.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(chunk.PNG)
}

var defaultService struct {
	sync.RWMutex
	service *Service
}

func SetDefault(service *Service) {
	defaultService.Lock()
	defaultService.service = service
	defaultService.Unlock()
}

func InvalidateCoordinates(coords []Coordinate) {
	defaultService.RLock()
	service := defaultService.service
	defaultService.RUnlock()
	if service != nil {
		service.InvalidateCoordinates(coords)
	}
}

func InvalidateTileImage(tileImageID int) {
	defaultService.RLock()
	service := defaultService.service
	defaultService.RUnlock()
	if service != nil {
		service.InvalidateTileImage(tileImageID)
	}
}

func InvalidateAll() {
	defaultService.RLock()
	service := defaultService.service
	defaultService.RUnlock()
	if service != nil {
		service.InvalidateAll()
	}
}
