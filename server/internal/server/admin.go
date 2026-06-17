package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"capturequest/internal/config"
	"capturequest/internal/db"
	"capturequest/internal/session"
)

// adminAuthMiddleware checks for the X-Admin-Token header
func adminAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, _ := config.Get()
		adminKey := cfg.AdminKey

		// If no admin key is set in production, we block everything
		if adminKey == "" {
			log.Printf("⚠️ ADMIN ACCESS ATTEMPT REJECTED: No ADMIN_KEY set in environment")
			http.Error(w, "Unauthorized (System Misconfigured)", http.StatusUnauthorized)
			return
		}

		token := r.Header.Get("X-Admin-Token")
		if token != adminKey {
			log.Printf("⚠️ UNAUTHORIZED ADMIN ACCESS ATTEMPT FROM %s", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// getHostStats returns host-level memory and CPU load from /proc (Linux only)
func getHostStats() (totalMem, usedMem string, loadAvg string) {
	totalMem, usedMem, loadAvg = "N/A", "N/A", "N/A"

	// Memory
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(data), "\n")
		var total, free, available int64
		for _, line := range lines {
			if strings.HasPrefix(line, "MemTotal:") {
				fmt.Sscanf(line, "MemTotal: %d", &total)
			} else if strings.HasPrefix(line, "MemFree:") {
				fmt.Sscanf(line, "MemFree: %d", &free)
			} else if strings.HasPrefix(line, "MemAvailable:") {
				fmt.Sscanf(line, "MemAvailable: %d", &available)
			}
		}
		if total > 0 {
			totalMem = fmt.Sprintf("%d MB", total/1024)
			// 'Used' in Linux sense often includes cache, but 'Available' is more useful
			used := total - available
			usedMem = fmt.Sprintf("%d MB", used/1024)
		}
	}

	// CPU Load
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) > 0 {
			loadAvg = parts[0] // 1 min load
		}
	}

	return
}

// handleAdminLogs returns the last N lines of captured server logs
func handleAdminLogs(w http.ResponseWriter, r *http.Request) {
	if GlobalLogBuffer == nil {
		http.Error(w, "Log buffer not initialized", http.StatusInternalServerError)
		return
	}

	lines := GlobalLogBuffer.GetLines()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"lines": lines,
	})
}

// handleAdminStats returns basic server health and statistics
func handleAdminStats(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	liveSessions := []map[string]interface{}{}
	session.GetSessionManager().ForEachSession(func(s *session.Session) {
		zoneName := "Unknown"
		if s.MapID > 0 {
			zoneName = fmt.Sprintf("Zone %d", s.MapID)
		}

		charID := 0
		if s.Client != nil {
			charID = s.Client.ID()
		}

		sessInfo := map[string]interface{}{
			"id":        s.SessionID,
			"ip":        s.IP,
			"zone_name": zoneName,
			"char_name": s.CharacterName,
			"char_id":   charID,
		}
		liveSessions = append(liveSessions, sessInfo)
	})

	hostTotalMem, hostUsedMem, hostLoad := getHostStats()

	stats := map[string]interface{}{
		"online_users":   session.GetActiveSessionCount(),
		"goroutines":     runtime.NumGoroutine(),
		"cpu_cores":      runtime.NumCPU(),
		"memory_alloc":   fmt.Sprintf("%d MB", m.Alloc/1024/1024),
		"memory_sys":     fmt.Sprintf("%d MB", m.Sys/1024/1024),
		"uptime":         time.Since(startTime).String(),
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
		"live_sessions":  liveSessions,
		"host_mem_total": hostTotalMem,
		"host_mem_used":  hostUsedMem,
		"host_load":      hostLoad,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleAdminUsers returns a list of all accounts
func handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	type adminAccount struct {
		ID           int32
		Name         string
		Status       int32
		TimeCreation uint32
		DiscordID    string
	}

	rows, err := db.GlobalWorldDB.DB.QueryContext(r.Context(), `
		SELECT id, name, status, time_creation, discord_id
		FROM account
		ORDER BY id DESC
		LIMIT 100`)
	if err != nil {
		log.Printf("Error fetching admin users: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	accounts := []adminAccount{}
	for rows.Next() {
		var account adminAccount
		if err := rows.Scan(
			&account.ID,
			&account.Name,
			&account.Status,
			&account.TimeCreation,
			&account.DiscordID,
		); err != nil {
			log.Printf("Error scanning admin user: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Error reading admin users: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(accounts)
}

// handleAdminCharacters returns a list of all characters
func handleAdminCharacters(w http.ResponseWriter, r *http.Request) {
	type adminCharacter struct {
		ID        uint32
		Name      string
		Class     uint8
		FactionID uint16
		MapID     uint32
		Birthday  uint32
		LastLogin uint32
	}

	rows, err := db.GlobalWorldDB.DB.QueryContext(r.Context(), `
		SELECT id, name, class, faction_id, map_id, birthday, last_login
		FROM character_data
		ORDER BY id DESC
		LIMIT 100`)
	if err != nil {
		log.Printf("Error fetching admin characters: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	characters := []adminCharacter{}
	for rows.Next() {
		var character adminCharacter
		if err := rows.Scan(
			&character.ID,
			&character.Name,
			&character.Class,
			&character.FactionID,
			&character.MapID,
			&character.Birthday,
			&character.LastLogin,
		); err != nil {
			log.Printf("Error scanning admin character: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		characters = append(characters, character)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Error reading admin characters: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	var results []map[string]interface{}
	for _, c := range characters {
		zoneName := fmt.Sprintf("Zone %d", c.MapID)

		res := map[string]interface{}{
			"id":         c.ID,
			"name":       c.Name,
			"class":      c.Class, // Still send ID just in case
			"zone":       zoneName,
			"factionId":  c.FactionID,
			"birthday":   c.Birthday,
			"last_login": c.LastLogin,
		}
		results = append(results, res)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handleAdminSetGM toggles GM status for an account
func handleAdminSetGM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type request struct {
		AccountID int32 `json:"account_id"`
		Status    int32 `json:"status"` // 0 = Player, 255 = Lead GM
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	_, err := db.GlobalWorldDB.DB.ExecContext(r.Context(), `
		UPDATE account
		SET status = $1
		WHERE id = $2`,
		req.Status, req.AccountID)
	if err != nil {
		log.Printf("Error updating GM status: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	log.Printf("ADMIN: Set Account %d status to %d", req.AccountID, req.Status)
	w.WriteHeader(http.StatusOK)
}

// handleAdminDBTables returns a list of all tables in the database
func handleAdminDBTables(w http.ResponseWriter, r *http.Request) {
	rows, err := db.GlobalWorldDB.DB.QueryContext(r.Context(), `
		SELECT tablename
		FROM pg_catalog.pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename`)
	if err != nil {
		log.Printf("Error listing tables: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tables := []string{}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			log.Printf("Error scanning table name: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tables = append(tables, table)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tables)
}

// handleAdminDBQuery executes a raw SQL query (READ ONLY)
func handleAdminDBQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	query := strings.TrimSpace(req.Query)
	upperQuery := strings.ToUpper(query)

	// Safety check: Only allow safe, read-only operations
	allowed := false
	prefixes := []string{"SELECT", "SHOW", "DESCRIBE", "EXPLAIN"}
	for _, p := range prefixes {
		if strings.HasPrefix(upperQuery, p) {
			allowed = true
			break
		}
	}

	if !allowed {
		http.Error(w, "Forbidden: Only SELECT, SHOW, DESCRIBE, and EXPLAIN queries are permitted.", http.StatusForbidden)
		return
	}

	rows, err := db.GlobalWorldDB.DB.QueryContext(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		row := make(map[string]interface{})
		for i, colName := range cols {
			val := values[i]
			// Handle byte slices returned by database drivers for textual values.
			if b, ok := val.([]byte); ok {
				row[colName] = string(b)
			} else {
				row[colName] = val
			}
		}
		results = append(results, row)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handleAdminGetCharacterInventory returns the CQ inventory of a specific character.
func handleAdminGetCharacterInventory(w http.ResponseWriter, r *http.Request) {
	charIDStr := r.URL.Query().Get("id")
	if charIDStr == "" {
		http.Error(w, "Missing character id", http.StatusBadRequest)
		return
	}

	charID, err := strconv.Atoi(charIDStr)
	if err != nil {
		http.Error(w, "Invalid character id", http.StatusBadRequest)
		return
	}

	dest := []struct {
		RowID   int32
		ItemID  int32
		Name    string
		Charges int32
		Icon    int32
	}{}

	rows, err := db.GlobalWorldDB.DB.QueryContext(r.Context(), `
		SELECT ci.id AS row_id,
		       ii.item_id,
		       i.name,
		       ii.quantity AS charges,
		       i.icon
		FROM cq_character_inventory ci
		JOIN cq_item_instances ii ON ii.id = ci.item_instance_id
		JOIN cq_items i ON i.id = ii.item_id
		WHERE ci.character_id = $1
		ORDER BY i.name, ii.item_id, ci.id`,
		charID)
	if err != nil {
		log.Printf("Error fetching character inventory: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var item struct {
			RowID   int32
			ItemID  int32
			Name    string
			Charges int32
			Icon    int32
		}
		if err := rows.Scan(
			&item.RowID,
			&item.ItemID,
			&item.Name,
			&item.Charges,
			&item.Icon,
		); err != nil {
			log.Printf("Error scanning character inventory: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		dest = append(dest, item)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Error reading character inventory: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dest)
}

var startTime = time.Now()
