package world

import (
	"testing"

	model "capturequest/internal/db/models"
)

func TestCharacterSelectEntryFromDataIncludesMapID(t *testing.T) {
	entry := characterSelectEntryFromData(model.CharacterData{
		ID:        77,
		Name:      "Palette",
		MapID:     0,
		Gender:    1,
		FactionID: 4,
		Class:     7,
		LastLogin: 12345,
	})

	assertEqual(t, entry, "id", uint32(77))
	assertEqual(t, entry, "name", "Palette")
	assertEqual(t, entry, "factionId", uint16(4))
	assertEqual(t, entry, "class", uint8(7))
	assertEqual(t, entry, "mapId", uint32(0))
	assertEqual(t, entry, "gender", uint8(1))
	assertEqual(t, entry, "lastLogin", uint32(12345))
}

func assertEqual[T comparable](t *testing.T, values map[string]interface{}, key string, want T) {
	t.Helper()
	got, ok := values[key]
	if !ok {
		t.Fatalf("missing key %q", key)
	}
	if got != want {
		t.Fatalf("%s = %#v, want %#v", key, got, want)
	}
}
