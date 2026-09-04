package scriptcandidateimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func loadCandidates(ctx context.Context, db *sql.DB) ([]scriptCandidate, error) {
	exists, err := sqliteTableExists(ctx, db, "script_event_candidates")
	if err != nil {
		return nil, fmt.Errorf("check script_event_candidates table: %w", err)
	}
	if !exists {
		log.Printf("[ScriptCandidates] SQLite has no script_event_candidates table; skipping")
		return nil, nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT candidate_json
		FROM script_event_candidates
		ORDER BY map_name, script_label, id`)
	if err != nil {
		return nil, fmt.Errorf("query script_event_candidates: %w", err)
	}
	defer rows.Close()

	candidates := []scriptCandidate{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var candidate scriptCandidate
		if err := json.Unmarshal([]byte(raw), &candidate); err != nil {
			return nil, fmt.Errorf("decode candidate JSON: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candidates, nil
}

func loadTileOverrideCandidates(ctx context.Context, db *sql.DB) ([]tileOverrideCandidate, bool, error) {
	exists, err := sqliteTableExists(ctx, db, "script_event_tile_overrides")
	if err != nil {
		return nil, false, fmt.Errorf("check script_event_tile_overrides table: %w", err)
	}
	if !exists {
		log.Printf("[ScriptCandidates] SQLite has no script_event_tile_overrides table; skipping generated event tiles")
		return nil, false, nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT candidate_json
		FROM script_event_tile_overrides
		ORDER BY map_name, script_label, id`)
	if err != nil {
		return nil, true, fmt.Errorf("query script_event_tile_overrides: %w", err)
	}
	defer rows.Close()

	candidates := []tileOverrideCandidate{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, true, err
		}
		var candidate tileOverrideCandidate
		if err := json.Unmarshal([]byte(raw), &candidate); err != nil {
			return nil, true, fmt.Errorf("decode tile override candidate JSON: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, true, err
	}
	return candidates, true, nil
}
