package scriptcandidateimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

const supportedCandidateSchemaVersion = 1

func requireCandidateVersion(table string, id int64, version int) error {
	if version != supportedCandidateSchemaVersion {
		return fmt.Errorf("%s row %d uses unsupported candidate schema version %d; expected %d", table, id, version, supportedCandidateSchemaVersion)
	}
	return nil
}

func loadCandidates(ctx context.Context, db *sql.DB) ([]scriptCandidate, error) {
	exists, err := sqliteTableExists(ctx, db, "script_event_candidates")
	if err != nil {
		return nil, fmt.Errorf("check script_event_candidates table: %w", err)
	}
	if !exists {
		return nil, nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, map_name, script_label, trigger_type, trigger_label, confidence, candidate_json
		FROM script_event_candidates
		ORDER BY map_name, script_label, id`)
	if err != nil {
		return nil, fmt.Errorf("query script_event_candidates: %w", err)
	}
	defer rows.Close()

	candidates := []scriptCandidate{}
	for rows.Next() {
		var id int64
		var mapName, scriptLabel, triggerType, triggerLabel, confidence, raw string
		if err := rows.Scan(&id, &mapName, &scriptLabel, &triggerType, &triggerLabel, &confidence, &raw); err != nil {
			return nil, err
		}
		var candidate scriptCandidate
		if err := json.Unmarshal([]byte(raw), &candidate); err != nil {
			return nil, fmt.Errorf("decode script_event_candidates row %d JSON: %w", id, err)
		}
		if err := requireCandidateVersion("script_event_candidates", id, candidate.Version); err != nil {
			return nil, err
		}
		if candidate.MapName != mapName || candidate.ScriptLabel != scriptLabel || candidate.Trigger.Type != triggerType || candidate.Trigger.Label != triggerLabel || candidate.Confidence != confidence {
			return nil, fmt.Errorf("script_event_candidates row %d JSON disagrees with relational columns", id)
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
		return nil, false, nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, map_name, script_label, candidate_json
		FROM script_event_tile_overrides
		ORDER BY map_name, script_label, id`)
	if err != nil {
		return nil, true, fmt.Errorf("query script_event_tile_overrides: %w", err)
	}
	defer rows.Close()

	candidates := []tileOverrideCandidate{}
	for rows.Next() {
		var id int64
		var mapName, scriptLabel, raw string
		if err := rows.Scan(&id, &mapName, &scriptLabel, &raw); err != nil {
			return nil, true, err
		}
		var candidate tileOverrideCandidate
		if err := json.Unmarshal([]byte(raw), &candidate); err != nil {
			return nil, true, fmt.Errorf("decode script_event_tile_overrides row %d JSON: %w", id, err)
		}
		if err := requireCandidateVersion("script_event_tile_overrides", id, candidate.Version); err != nil {
			return nil, true, err
		}
		if candidate.MapName != mapName || candidate.ScriptLabel != scriptLabel {
			return nil, true, fmt.Errorf("script_event_tile_overrides row %d JSON disagrees with relational columns", id)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, true, err
	}
	return candidates, true, nil
}
