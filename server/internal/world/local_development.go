package world

import (
	"os"

	"capturequest/internal/config"
	model "capturequest/internal/db/models"
)

func localDevelopmentEnabled() bool {
	serverConfig, err := config.Get()
	return err == nil && serverConfig.Local
}

func debugToolsEnabled() bool {
	return localDevelopmentEnabled() || os.Getenv("CAPTUREQUEST_TEST_MODE") == "true"
}

// applyLocalCharacterPrivileges grants local-development capabilities only to
// the in-memory character. It deliberately does not update character_data.gm,
// so pointing a production server at the same database cannot inherit them.
func applyLocalCharacterPrivileges(character *model.CharacterData) {
	applyCharacterPrivileges(character, localDevelopmentEnabled())
}

func applyCharacterPrivileges(character *model.CharacterData, local bool) {
	if character != nil && local {
		character.Gm = 1
	}
}
