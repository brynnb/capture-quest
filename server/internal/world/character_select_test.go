package world

import (
	"encoding/json"
	"testing"

	model "capturequest/internal/db/models"
)

func TestCharacterSelectEntryFromDataIncludesMapID(t *testing.T) {
	entry := characterSelectEntryFromData(model.CharacterData{
		ID: 77, Name: "Palette", MapID: 0, Gender: 1, FactionID: 4, Class: 7, LastLogin: 12345,
	})
	want := CharacterSelectEntry{ID: 77, Name: "Palette", MapID: 0, Gender: 1, FactionID: 4, Class: 7, LastLogin: 12345}
	if entry != want {
		t.Fatalf("entry=%+v want=%+v", entry, want)
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]interface{}{"id": float64(77), "name": "Palette", "mapId": float64(0), "factionId": float64(4), "class": float64(7), "gender": float64(1), "lastLogin": float64(12345)} {
		if got, present := fields[key]; !present || got != want {
			t.Fatalf("wire %s=%v present=%t want=%v", key, got, present, want)
		}
	}
}
