package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/session"
)

// Post-commit notices carry no historical resource projection. The current
// scene reconciles through its owned, locked gameplay read.
type ResourceChangeNotify struct {
	Success          bool  `json:"success" tstype:"true"`
	ResourcesChanged bool  `json:"resourcesChanged" tstype:"true"`
	CharacterID      int64 `json:"characterId"`
}

func notifyResourceChange(ses *session.Session) {
	if ses == nil || !ses.HasValidClient() || ses.IsClosed() {
		return
	}
	ses.SendStreamJSON(ResourceChangeNotify{Success: true, ResourcesChanged: true, CharacterID: int64(ses.Client.CharData().ID)}, opcodes.ResourcesChangedNotify)
}
