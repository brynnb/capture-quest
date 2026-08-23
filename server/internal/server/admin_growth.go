package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"capturequest/internal/db"
)

type adminGrowthPoint struct {
	Date            string `json:"date"`
	TotalUsers      int    `json:"total_users"`
	TotalCharacters int    `json:"total_characters"`
}

type adminActivityCounts struct {
	Daily   int `json:"daily"`
	Weekly  int `json:"weekly"`
	Monthly int `json:"monthly"`
}

func activityFromLastLogins(lastLogins []int64, now time.Time) adminActivityCounts {
	counts := adminActivityCounts{}
	for _, lastLogin := range lastLogins {
		login := time.Unix(lastLogin, 0)
		if !login.Before(now.Add(-24 * time.Hour)) {
			counts.Daily++
		}
		if !login.Before(now.Add(-7 * 24 * time.Hour)) {
			counts.Weekly++
		}
		if !login.Before(now.Add(-30 * 24 * time.Hour)) {
			counts.Monthly++
		}
	}
	return counts
}

func growthSeriesFromEpochs(userEpochs, characterEpochs []int64, days int, now time.Time) []adminGrowthPoint {
	today := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC)
	start := today.AddDate(0, 0, -(days - 1))
	normalize := func(epoch int64) time.Time {
		if epoch <= 0 {
			return start.Add(-time.Second)
		}
		value := time.Unix(epoch, 0).UTC()
		if value.After(now.UTC()) {
			return now.UTC()
		}
		return value
	}
	points := make([]adminGrowthPoint, 0, days)
	for offset := 0; offset < days; offset++ {
		day := start.AddDate(0, 0, offset)
		cutoff := day.AddDate(0, 0, 1)
		point := adminGrowthPoint{Date: day.Format("2006-01-02")}
		for _, epoch := range userEpochs {
			if normalize(epoch).Before(cutoff) {
				point.TotalUsers++
			}
		}
		for _, epoch := range characterEpochs {
			if normalize(epoch).Before(cutoff) {
				point.TotalCharacters++
			}
		}
		points = append(points, point)
	}
	return points
}

func handleAdminGrowth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	days := 365
	if raw := r.URL.Query().Get("days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || (parsed != 30 && parsed != 90 && parsed != 365) {
			http.Error(w, "days must be 30, 90, or 365", http.StatusBadRequest)
			return
		}
		days = parsed
	}
	if db.GlobalWorldDB == nil || db.GlobalWorldDB.DB == nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}

	query := func(sqlText string) ([]int64, error) {
		rows, err := db.GlobalWorldDB.DB.QueryContext(r.Context(), sqlText)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		values := []int64{}
		for rows.Next() {
			var value int64
			if err := rows.Scan(&value); err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, rows.Err()
	}
	users, err := query("SELECT time_creation FROM account")
	if err != nil {
		log.Printf("admin growth accounts query failed: %v", err)
		http.Error(w, "Could not load account growth", http.StatusInternalServerError)
		return
	}
	characters, err := query("SELECT birthday FROM character_data WHERE deleted_at IS NULL")
	if err != nil {
		log.Printf("admin growth characters query failed: %v", err)
		http.Error(w, "Could not load character growth", http.StatusInternalServerError)
		return
	}
	points := growthSeriesFromEpochs(users, characters, days, time.Now())
	lastLogins, err := query("SELECT MAX(last_login) FROM character_data WHERE deleted_at IS NULL GROUP BY account_id")
	if err != nil {
		log.Printf("admin activity query failed: %v", err)
		http.Error(w, "Could not load player activity", http.StatusInternalServerError)
		return
	}
	now := time.Now()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"total_users": len(users), "total_characters": len(characters), "points": points,
		"activity": activityFromLastLogins(lastLogins, now),
	})
}
