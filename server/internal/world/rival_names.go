package world

import (
	"context"
	"strings"

	db_character "capturequest/internal/db/character"
)

func trainerNameForCharacter(charID int64, trainerClass, trainerName string) string {
	name := strings.TrimSpace(trainerName)
	if name == "" {
		name = trainerDisplayName(trainerClass)
	}
	if isRivalTrainerName(trainerClass, name) {
		return rivalNameForCharacter(charID)
	}
	return name
}

func rivalNameForCharacter(charID int64) string {
	opts, err := db_character.LoadOptions(context.Background(), int32(charID))
	if err != nil {
		return db_character.DefaultRivalName
	}
	return db_character.NormalizeRivalName(opts.RivalName)
}

func isRivalTrainerName(trainerClass, trainerName string) bool {
	class := strings.ToUpper(strings.TrimSpace(trainerClass))
	name := strings.ToUpper(strings.TrimSpace(trainerName))
	return strings.HasPrefix(class, "RIVAL") || name == "RIVAL"
}
