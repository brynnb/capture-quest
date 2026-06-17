package server

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"capturequest/internal/db"
)

const tileImageDir = "../public/phaser/tile_images"
const tileSize = 16

// handleTileReplace handles POST /api/tiles/replace
// Replaces a single tile image file on disk.
func handleTileReplace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 2MB)
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid form data"})
		return
	}

	tileImageIdStr := r.FormValue("tileImageId")
	if tileImageIdStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "tileImageId required"})
		return
	}

	tileImageId, err := strconv.Atoi(tileImageIdStr)
	if err != nil || tileImageId < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid tileImageId"})
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "image file required"})
		return
	}
	defer file.Close()

	// Decode the image to validate dimensions
	img, _, err := image.Decode(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid image file"})
		return
	}

	bounds := img.Bounds()
	if bounds.Dx() != tileSize || bounds.Dy() != tileSize {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("image must be %dx%d pixels (got %dx%d)", tileSize, tileSize, bounds.Dx(), bounds.Dy()),
		})
		return
	}

	// Write the PNG to disk (0-indexed filename)
	fileIndex := tileImageId - 1
	destPath := filepath.Join(tileImageDir, fmt.Sprintf("tile_%d.png", fileIndex))

	// Ensure directory exists
	if err := os.MkdirAll(tileImageDir, 0755); err != nil {
		log.Printf("[TileArt] Failed to create tile image dir: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "server error"})
		return
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		log.Printf("[TileArt] Failed to create file %s: %v", destPath, err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "failed to write file"})
		return
	}
	defer outFile.Close()

	// Encode as PNG
	if err := png.Encode(outFile, img); err != nil {
		log.Printf("[TileArt] Failed to encode PNG: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "failed to encode image"})
		return
	}

	log.Printf("[TileArt] Replaced tile image %d at %s", tileImageId, destPath)
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "tileImageId": tileImageId})
}

// handleStampCreate handles POST /api/tiles/stamp
// Slices a multi-tile image into individual 16x16 tiles and saves them.
func handleStampCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid form data"})
		return
	}

	name := r.FormValue("name")
	if name == "" {
		name = "Unnamed Stamp"
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "image file required"})
		return
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid image file"})
		return
	}

	bounds := img.Bounds()
	if bounds.Dx()%tileSize != 0 || bounds.Dy()%tileSize != 0 || bounds.Dx() == 0 || bounds.Dy() == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("image dimensions must be multiples of %d (got %dx%d)", tileSize, bounds.Dx(), bounds.Dy()),
		})
		return
	}

	widthTiles := bounds.Dx() / tileSize
	heightTiles := bounds.Dy() / tileSize

	// Get the next available tile_image_id
	var maxTileImageId int
	err = db.GlobalWorldDB.DB.QueryRow(`SELECT COALESCE(MAX(tile_image_id), 0) FROM phaser_tile_properties`).Scan(&maxTileImageId)
	if err != nil {
		log.Printf("[TileArt] Failed to get max tile_image_id: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "database error"})
		return
	}

	// Also check max id in phaser_tile_images
	var maxTileImagesId int
	err = db.GlobalWorldDB.DB.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM phaser_tile_images`).Scan(&maxTileImagesId)
	if err != nil {
		// Table might not have data, that's ok
		maxTileImagesId = 0
	}
	if maxTileImagesId > maxTileImageId {
		maxTileImageId = maxTileImagesId
	}

	nextId := maxTileImageId + 1

	// Create stamp record
	var stampId int64
	err = db.GlobalWorldDB.DB.QueryRow(
		`INSERT INTO phaser_stamps (name, width_tiles, height_tiles) VALUES ($1, $2, $3) RETURNING id`,
		name, widthTiles, heightTiles,
	).Scan(&stampId)
	if err != nil {
		log.Printf("[TileArt] Failed to create stamp: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "database error"})
		return
	}

	// Slice the image into tiles
	tileImageIds := make([][]int, heightTiles)
	if err := os.MkdirAll(tileImageDir, 0755); err != nil {
		log.Printf("[TileArt] Failed to create tile image dir: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "server error"})
		return
	}

	tx, err := db.GlobalWorldDB.DB.Begin()
	if err != nil {
		log.Printf("[TileArt] Failed to begin transaction: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "database error"})
		return
	}

	currentId := nextId
	for row := 0; row < heightTiles; row++ {
		tileImageIds[row] = make([]int, widthTiles)
		for col := 0; col < widthTiles; col++ {
			// Extract sub-image
			subImg := image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
			for y := 0; y < tileSize; y++ {
				for x := 0; x < tileSize; x++ {
					subImg.Set(x, y, img.At(bounds.Min.X+col*tileSize+x, bounds.Min.Y+row*tileSize+y))
				}
			}

			// Save tile image file (0-indexed filename)
			fileIndex := currentId - 1
			destPath := filepath.Join(tileImageDir, fmt.Sprintf("tile_%d.png", fileIndex))
			outFile, err := os.Create(destPath)
			if err != nil {
				log.Printf("[TileArt] Failed to create tile file %s: %v", destPath, err)
				tx.Rollback()
				writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "failed to write tile file"})
				return
			}
			if err := png.Encode(outFile, subImg); err != nil {
				outFile.Close()
				log.Printf("[TileArt] Failed to encode tile PNG: %v", err)
				tx.Rollback()
				writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "failed to encode tile"})
				return
			}
			outFile.Close()

			// Insert into phaser_tile_images
			_, err = tx.Exec(
				`INSERT INTO phaser_tile_images (id, image_path) VALUES ($1, $2)`,
				currentId, fmt.Sprintf("tile_%d.png", fileIndex),
			)
			if err != nil {
				log.Printf("[TileArt] Failed to insert tile_image %d: %v", currentId, err)
			}

			// Insert into phaser_tile_properties
			tileName := fmt.Sprintf("%s [%d,%d]", name, col, row)
			_, err = tx.Exec(
				`INSERT INTO phaser_tile_properties (tile_image_id, name, collision_type, is_user_editable) VALUES ($1, $2, 0, TRUE)`,
				currentId, tileName,
			)
			if err != nil {
				log.Printf("[TileArt] Failed to insert tile_property %d: %v", currentId, err)
			}

			// Insert into phaser_stamp_tiles
			_, err = tx.Exec(
				`INSERT INTO phaser_stamp_tiles (stamp_id, offset_x, offset_y, tile_image_id) VALUES ($1, $2, $3, $4)`,
				stampId, col, row, currentId,
			)
			if err != nil {
				log.Printf("[TileArt] Failed to insert stamp_tile: %v", err)
			}

			tileImageIds[row][col] = currentId
			currentId++
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[TileArt] Failed to commit stamp transaction: %v", err)
		tx.Rollback()
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "database error"})
		return
	}

	log.Printf("[TileArt] Created stamp '%s' (id=%d) with %d tiles (%dx%d)", name, stampId, widthTiles*heightTiles, widthTiles, heightTiles)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"stampId":      stampId,
		"width":        widthTiles,
		"height":       heightTiles,
		"tileImageIds": tileImageIds,
	})
}

// handleStampList handles GET /api/tiles/stamps
func handleStampList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT s.id, s.name, s.width_tiles, s.height_tiles, s.created_at
		FROM phaser_stamps s
		ORDER BY s.created_at DESC
	`)
	if err != nil {
		log.Printf("[TileArt] Failed to query stamps: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "database error"})
		return
	}
	defer rows.Close()

	type StampEntry struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		WidthTiles  int    `json:"widthTiles"`
		HeightTiles int    `json:"heightTiles"`
		CreatedAt   string `json:"createdAt"`
	}

	var stamps []StampEntry
	for rows.Next() {
		var s StampEntry
		if err := rows.Scan(&s.ID, &s.Name, &s.WidthTiles, &s.HeightTiles, &s.CreatedAt); err != nil {
			log.Printf("[TileArt] Error scanning stamp: %v", err)
			continue
		}
		stamps = append(stamps, s)
	}

	// For each stamp, get its tile image IDs
	type StampWithTiles struct {
		StampEntry
		TileImageIds [][]int `json:"tileImageIds"`
	}

	var result []StampWithTiles
	for _, stamp := range stamps {
		tileRows, err := db.GlobalWorldDB.DB.Query(
			`SELECT offset_x, offset_y, tile_image_id FROM phaser_stamp_tiles WHERE stamp_id = $1 ORDER BY offset_y, offset_x`,
			stamp.ID,
		)
		if err != nil {
			log.Printf("[TileArt] Error querying stamp tiles: %v", err)
			continue
		}

		tileGrid := make([][]int, stamp.HeightTiles)
		for i := range tileGrid {
			tileGrid[i] = make([]int, stamp.WidthTiles)
		}

		for tileRows.Next() {
			var ox, oy, tid int
			tileRows.Scan(&ox, &oy, &tid)
			if oy < stamp.HeightTiles && ox < stamp.WidthTiles {
				tileGrid[oy][ox] = tid
			}
		}
		tileRows.Close()

		result = append(result, StampWithTiles{
			StampEntry:   stamp,
			TileImageIds: tileGrid,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "stamps": result})
}

// handleTileAnimationCreate handles POST /api/tiles/animation
func handleTileAnimationCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid form data"})
		return
	}

	tileImageIdStr := r.FormValue("tileImageId")
	if tileImageIdStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "tileImageId required"})
		return
	}
	tileImageId, err := strconv.Atoi(tileImageIdStr)
	if err != nil || tileImageId < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid tileImageId"})
		return
	}

	frameDurationStr := r.FormValue("frameDurationMs")
	frameDuration := 500
	if frameDurationStr != "" {
		if fd, err := strconv.Atoi(frameDurationStr); err == nil && fd > 0 {
			frameDuration = fd
		}
	}

	// Get all frame files
	files := r.MultipartForm.File["frames"]
	if len(files) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "at least one frame file required"})
		return
	}

	// Create animation frames directory
	animDir := filepath.Join(tileImageDir, "animations")
	if err := os.MkdirAll(animDir, 0755); err != nil {
		log.Printf("[TileArt] Failed to create animation dir: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "server error"})
		return
	}

	// Clear existing animation frames for this tile
	db.GlobalWorldDB.DB.Exec(`DELETE FROM phaser_tile_animations WHERE tile_image_id = $1`, tileImageId)

	for i, fh := range files {
		f, err := fh.Open()
		if err != nil {
			log.Printf("[TileArt] Failed to open frame file: %v", err)
			continue
		}

		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			log.Printf("[TileArt] Failed to decode frame image: %v", err)
			continue
		}

		bounds := img.Bounds()
		if bounds.Dx() != tileSize || bounds.Dy() != tileSize {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("frame %d must be %dx%d pixels (got %dx%d)", i, tileSize, tileSize, bounds.Dx(), bounds.Dy()),
			})
			return
		}

		framePath := fmt.Sprintf("animations/tile_%d_frame_%d.png", tileImageId, i)
		destPath := filepath.Join(tileImageDir, framePath)

		outFile, err := os.Create(destPath)
		if err != nil {
			log.Printf("[TileArt] Failed to create frame file: %v", err)
			continue
		}
		png.Encode(outFile, img)
		outFile.Close()

		db.GlobalWorldDB.DB.Exec(
			`INSERT INTO phaser_tile_animations (tile_image_id, frame_index, frame_image_path, frame_duration_ms) VALUES ($1, $2, $3, $4)`,
			tileImageId, i, framePath, frameDuration,
		)
	}

	log.Printf("[TileArt] Created %d animation frames for tile %d", len(files), tileImageId)
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "frames": len(files)})
}

// handleTileAnimationList handles GET /api/tiles/animations
func handleTileAnimationList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"success":false,"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT tile_image_id, frame_index, frame_image_path, frame_duration_ms
		FROM phaser_tile_animations
		ORDER BY tile_image_id, frame_index
	`)
	if err != nil {
		log.Printf("[TileArt] Failed to query animations: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "database error"})
		return
	}
	defer rows.Close()

	type AnimFrame struct {
		TileImageId     int    `json:"tileImageId"`
		FrameIndex      int    `json:"frameIndex"`
		FrameImagePath  string `json:"frameImagePath"`
		FrameDurationMs int    `json:"frameDurationMs"`
	}

	var frames []AnimFrame
	for rows.Next() {
		var f AnimFrame
		if err := rows.Scan(&f.TileImageId, &f.FrameIndex, &f.FrameImagePath, &f.FrameDurationMs); err != nil {
			continue
		}
		frames = append(frames, f)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "animations": frames})
}

// writeJSON is a helper to write JSON responses.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
