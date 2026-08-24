package world

import (
	"testing"

	model "capturequest/internal/db/models"
)

func TestApplyCharacterPrivilegesGrantsLocalGM(t *testing.T) {
	character := &model.CharacterData{}
	applyCharacterPrivileges(character, true)
	if character.Gm != 1 {
		t.Fatalf("expected local character GM=1, got %d", character.Gm)
	}
}

func TestApplyCharacterPrivilegesDoesNotGrantProductionGM(t *testing.T) {
	character := &model.CharacterData{}
	applyCharacterPrivileges(character, false)
	if character.Gm != 0 {
		t.Fatalf("expected production character GM to remain 0, got %d", character.Gm)
	}
}
