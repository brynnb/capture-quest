package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/options"
	"capturequest/internal/session"
	"log"
)

type PreferenceRequest struct {
	RequestID   string           `json:"requestId"`
	CharacterID int64            `json:"characterId"`
	Current     bool             `json:"current,omitempty"`
	Revision    int64            `json:"revision"`
	OptionID    options.OptionId `json:"optionId" tstype:"number"`
	Value       int              `json:"value"`
}
type PreferenceResponse struct {
	Success               bool   `json:"success" tstype:"true"`
	RequestID             string `json:"requestId"`
	CharacterID           int64  `json:"characterId"`
	Revision              int64  `json:"revision"`
	ShowNetworkStats      bool   `json:"showNetworkStats"`
	AllowTrainerRebattles bool   `json:"allowTrainerRebattles"`
}

func HandleSetOption(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PreferenceRequest
	fail := func(message string) bool {
		ses.SendStreamJSON(BattleCommandError{RequestID: req.RequestID, Error: message}, opcodes.SetOption)
		return false
	}
	if decodePlayerMovement(payload, &req) != nil || !validBattleRequestID(req.RequestID) || !ses.HasValidClient() {
		return fail("Invalid preference request")
	}
	charID := int64(ses.Client.CharData().ID)
	if req.CharacterID != charID {
		return fail("Preference character changed")
	}
	if !req.Current {
		var key string
		switch req.OptionID {
		case options.OptionShowNetworkStats:
			key = "showNetworkStats"
		case options.OptionAllowTrainerRebattles:
			key = "allowTrainerRebattles"
		default:
			return fail("Unsupported preference")
		}
		if req.Value != 0 && req.Value != 1 {
			return fail("Invalid preference value")
		}
		if err := db_character.SetBooleanOption(ses.CommandContext(), wh.database, int32(charID), key, req.Value == 1, req.Revision); err != nil {
			log.Printf("[Options] Character %d: %v", charID, err)
			return fail("Preference could not be saved; read current preferences")
		}
	}
	var opts *db_character.CharacterOptions
	err := db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
		// This read shares the character lock without firing UPDATE triggers.
		var id int64
		if err := tx.QueryRow(`SELECT id FROM character_data WHERE id=$1 FOR UPDATE`, charID).Scan(&id); err != nil {
			return err
		}
		var err error
		opts, err = db_character.LoadOptionsFrom(ses.CommandContext(), tx.(db.ContextDBTX), int32(charID))
		return err
	})
	if err != nil {
		return fail("Current preferences unavailable; recover their current state")
	}
	if cached := ses.Client.Options(); cached != nil {
		cached.PreferenceRevision = opts.PreferenceRevision
	}
	ses.Client.SetShowNetworkStatsEnabled(opts.ShowNetworkStats)
	ses.Client.SetAllowTrainerRebattlesEnabled(opts.AllowTrainerRebattles)
	ses.SendStreamJSON(PreferenceResponse{Success: true, RequestID: req.RequestID, CharacterID: charID, Revision: opts.PreferenceRevision, ShowNetworkStats: opts.ShowNetworkStats, AllowTrainerRebattles: opts.AllowTrainerRebattles}, opcodes.SetOption)
	return false
}
