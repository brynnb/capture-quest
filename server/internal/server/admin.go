package server

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net"
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

// adminAuthMiddleware accepts only direct loopback requests from the dashboard
// backend. Reverse-proxied public requests carry forwarding headers and are
// rejected even when Caddy is accidentally misconfigured.
func adminAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isDirectLoopbackRequest(r) {
			http.NotFound(w, r)
			return
		}

		cfg, err := config.Get()
		if err != nil {
			log.Printf("ADMIN ACCESS REJECTED: failed to read configuration: %v", err)
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		adminKey := cfg.AdminKey

		// If no admin key is set in production, we block everything
		if adminKey == "" {
			log.Printf("⚠️ ADMIN ACCESS ATTEMPT REJECTED: No ADMIN_KEY set in environment")
			http.Error(w, "Unauthorized (System Misconfigured)", http.StatusUnauthorized)
			return
		}

		token := r.Header.Get("X-Admin-Token")
		if len(token) != len(adminKey) || subtle.ConstantTimeCompare([]byte(token), []byte(adminKey)) != 1 {
			log.Printf("⚠️ UNAUTHORIZED ADMIN ACCESS ATTEMPT FROM %s", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func isDirectLoopbackRequest(r *http.Request) bool {
	if r.Header.Get("Forwarded") != "" || r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Real-IP") != "" {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func requireAdminMethod(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
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

// handleAdminChats returns the most recent player chat messages in display
// order (oldest first) for the cross-game dashboard.
func handleAdminChats(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > 100 {
			http.Error(w, "limit must be between 1 and 100", http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	type adminChatMessage struct {
		Sender    string `json:"sender"`
		Text      string `json:"text"`
		Channel   string `json:"channel"`
		Location  string `json:"location,omitempty"`
		Timestamp int64  `json:"timestamp"`
	}

	messages := make([]adminChatMessage, 0, limit)
	if db.GlobalWorldDB == nil || db.GlobalWorldDB.DB == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
		return
	}

	rows, err := db.GlobalWorldDB.DB.QueryContext(r.Context(), `
		SELECT character_name, text, message_type, COALESCE(map_id, 0), created_at
		FROM chat_messages
		ORDER BY created_at DESC, id DESC
		LIMIT $1`, limit)
	if err != nil {
		log.Printf("Error fetching admin chat history: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var message adminChatMessage
		var mapID int
		var createdAt time.Time
		if err := rows.Scan(
			&message.Sender,
			&message.Text,
			&message.Channel,
			&mapID,
			&createdAt,
		); err != nil {
			log.Printf("Error scanning admin chat history: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if mapID > 0 {
			message.Location = fmt.Sprintf("Zone %d", mapID)
		}
		message.Timestamp = createdAt.UnixMilli()
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Error reading admin chat history: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
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
