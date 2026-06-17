package world

import (
	"database/sql"
	"log"

	"capturequest/internal/db"
)

type EventObjectState struct {
	ObjectID int
	MapID    int
	X        int
	Y        int
	Name     string
	Text     string
	Visible  bool
	Label    string
}

type eventObjectVisibility struct {
	ObjectName         string
	Visible            bool
	RequiresFlag       sql.NullString
	RequiresFlagAbsent sql.NullString
	Label              sql.NullString
}

func ApplyEventObjectVisibilityToActors(charID int64, mapID int, efm *EventFlagManager, actors []PhaserActor) []PhaserActor {
	if len(actors) == 0 {
		return actors
	}

	mapIDs := map[int]bool{mapID: true}
	if mapID == UnifiedOverworldMapID {
		for _, actor := range actors {
			if actor.MapID >= 0 {
				mapIDs[actor.MapID] = true
			}
		}
	}

	rulesByMap := make(map[int][]eventObjectVisibility, len(mapIDs))
	hasRules := false
	for id := range mapIDs {
		rules, err := eventObjectVisibilityForMap(id)
		if err != nil {
			log.Printf("[EventObjects] Failed to load visibility rules for map %d: %v", id, err)
			continue
		}
		if len(rules) > 0 {
			hasRules = true
			rulesByMap[id] = rules
		}
	}
	if !hasRules {
		return actors
	}

	overrides, err := objectVisibilityOverridesForCharacter(charID)
	if err != nil {
		log.Printf("[EventObjects] Failed to load visibility overrides for character %d: %v", charID, err)
		overrides = nil
	}

	filtered := actors[:0]
	for _, actor := range actors {
		name := ""
		if actor.Name != nil {
			name = *actor.Name
		}
		ruleMapID := mapID
		if mapID == UnifiedOverworldMapID && actor.MapID >= 0 {
			ruleMapID = actor.MapID
		}
		rules := rulesByMap[ruleMapID]
		visible, label := currentEventObjectVisibility(charID, efm, name, rules)
		visible, _ = applyObjectVisibilityOverride(actor.DbID, visible, label, overrides)
		if visible {
			filtered = append(filtered, actor)
		}
	}
	return filtered
}

func EventObjectStatesForCharacter(charID int64, mapID int, efm *EventFlagManager) ([]EventObjectState, error) {
	rules, err := eventObjectVisibilityForMap(mapID)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}
	overrides, err := objectVisibilityOverridesForCharacter(charID)
	if err != nil {
		return nil, err
	}

	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT po.id, po.map_id,
			COALESCE(cop.x, po.x, po.local_x) AS x,
			COALESCE(cop.y, po.y, po.local_y) AS y,
			po.name, COALESCE(po.text, '')
		FROM phaser_objects po
		LEFT JOIN character_object_positions cop
			ON cop.character_id = $1 AND cop.object_id = po.id
		JOIN (
			SELECT DISTINCT map_id, object_name
			FROM phaser_event_object_visibility
		) ev ON ev.map_id = po.map_id AND ev.object_name = po.name
		WHERE po.map_id = $2
		ORDER BY po.name, po.id`, charID, mapID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := []EventObjectState{}
	for rows.Next() {
		var state EventObjectState
		if err := rows.Scan(&state.ObjectID, &state.MapID, &state.X, &state.Y, &state.Name, &state.Text); err != nil {
			return nil, err
		}
		state.Visible, state.Label = currentEventObjectVisibility(charID, efm, state.Name, rules)
		state.Visible, state.Label = applyObjectVisibilityOverride(state.ObjectID, state.Visible, state.Label, overrides)
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return states, nil
}

func currentEventObjectVisibility(charID int64, efm *EventFlagManager, objectName string, rules []eventObjectVisibility) (bool, string) {
	visible := true
	label := ""
	for _, rule := range rules {
		if rule.ObjectName != objectName {
			continue
		}
		if !rule.eventObjectEligible(charID, efm) {
			continue
		}
		visible = rule.Visible
		if rule.Label.Valid {
			label = rule.Label.String
		}
	}
	return visible, label
}

func eventObjectVisibilityForMap(mapID int) ([]eventObjectVisibility, error) {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT object_name, visible, requires_flag, requires_flag_absent, label
		FROM phaser_event_object_visibility
		WHERE map_id = $1
		ORDER BY id`, mapID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := []eventObjectVisibility{}
	for rows.Next() {
		var rule eventObjectVisibility
		if err := rows.Scan(
			&rule.ObjectName,
			&rule.Visible,
			&rule.RequiresFlag,
			&rule.RequiresFlagAbsent,
			&rule.Label,
		); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

func (rule eventObjectVisibility) eventObjectEligible(charID int64, efm *EventFlagManager) bool {
	if rule.RequiresFlag.Valid && rule.RequiresFlag.String != "" {
		if efm == nil || charID == 0 || !efm.CheckFlag(charID, rule.RequiresFlag.String) {
			return false
		}
	}
	if rule.RequiresFlagAbsent.Valid && rule.RequiresFlagAbsent.String != "" {
		if efm != nil && charID > 0 && efm.CheckFlag(charID, rule.RequiresFlagAbsent.String) {
			return false
		}
	}
	return true
}
