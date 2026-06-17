package world

import (
	"fmt"
	"regexp"
	"strings"

	"capturequest/internal/db"
)

var (
	camelLocationBoundaryPattern = regexp.MustCompile(`([a-z])([A-Z0-9])`)
	capsLocationBoundaryPattern  = regexp.MustCompile(`([A-Z])([A-Z][a-z])`)
	locationFloorPattern         = regexp.MustCompile(`(?i)([0-9])\s+([FB])\b`)
	locationFloorTokenPattern    = regexp.MustCompile(`^(B?[0-9]+F|[0-9]+B)$`)
	locationNumberTokenPattern   = regexp.MustCompile(`^[0-9]+$`)
)

func mapEntryDisplayName(wh *WorldHandler, mapID int, x, y float64) string {
	if mapID == UnifiedOverworldMapID {
		if name := OverworldMapLoadNameForPosition(int(x), int(y)); name != "" {
			return formatMapDisplayName(name)
		}
		return "Kanto"
	}

	if raw, ok := rawMapNameByID(mapID); ok {
		return formatMapDisplayName(raw)
	}

	if wh != nil && wh.ActorManager != nil && wh.ActorManager.IsOverworld(mapID) {
		return "Kanto"
	}

	return fmt.Sprintf("Map %d", mapID)
}

func rawMapNameByID(mapID int) (string, bool) {
	var name string
	if err := dbQueryRow(`SELECT name FROM phaser_maps WHERE id = $1`, mapID).Scan(&name); err != nil {
		return "", false
	}
	return name, true
}

func formatMapDisplayName(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	normalized := normalizeLocationName(trimmed)
	if normalized == "UNIFIED_OVERWORLD" || normalized == "OVERWORLD" || normalized == "UNIFIED_OVERWORLD_MAP" {
		return "Kanto"
	}

	if normalized == "SS_ANNE" || strings.HasPrefix(normalized, "SS_ANNE_") || normalized == "S_S_ANNE" || strings.HasPrefix(normalized, "S_S_ANNE_") {
		suffix := strings.TrimPrefix(normalized, "SS_ANNE")
		suffix = strings.TrimPrefix(suffix, "S_S_ANNE")
		suffix = strings.TrimPrefix(suffix, "_")
		if suffix == "" {
			return "S.S. Anne"
		}
		return "S.S. Anne " + formatLocationWords(strings.Split(suffix, "_"))
	}

	spaced := strings.ReplaceAll(trimmed, "_", " ")
	spaced = strings.ReplaceAll(spaced, "-", " ")
	spaced = camelLocationBoundaryPattern.ReplaceAllString(spaced, "$1 $2")
	spaced = capsLocationBoundaryPattern.ReplaceAllString(spaced, "$1 $2")
	spaced = locationFloorPattern.ReplaceAllString(spaced, "$1$2")
	return formatLocationWords(strings.Fields(spaced))
}

func normalizeLocationName(name string) string {
	var b strings.Builder
	lastWasSeparator := false
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			if lastWasSeparator && b.Len() > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r)
			lastWasSeparator = false
			continue
		}
		if r >= 'a' && r <= 'z' {
			if lastWasSeparator && b.Len() > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'a' + 'A')
			lastWasSeparator = false
			continue
		}
		lastWasSeparator = true
	}
	return strings.Trim(b.String(), "_")
}

func formatLocationWords(words []string) string {
	formatted := make([]string, 0, len(words))
	for _, word := range words {
		if word == "" {
			continue
		}
		formatted = append(formatted, formatLocationWord(word))
	}
	return strings.Join(formatted, " ")
}

func formatLocationWord(word string) string {
	upper := strings.ToUpper(word)
	if locationFloorTokenPattern.MatchString(upper) {
		return upper
	}
	if locationNumberTokenPattern.MatchString(upper) {
		return upper
	}

	switch upper {
	case "MT":
		return "Mt."
	case "MR":
		return "Mr."
	case "MRS":
		return "Mrs."
	case "TM", "HM", "PC":
		return upper
	case "POKEMON":
		return "Pokemon"
	default:
		return strings.ToUpper(upper[:1]) + strings.ToLower(upper[1:])
	}
}

type scanner interface {
	Scan(dest ...any) error
}

var dbQueryRow = func(query string, args ...any) scanner {
	return db.GlobalWorldDB.DB.QueryRow(query, args...)
}
