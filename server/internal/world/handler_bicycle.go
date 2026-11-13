package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/itemuse"
	"capturequest/internal/session"
	"fmt"
)

type BicycleStateRequest struct {
	RequestID   string `json:"requestId"`
	CharacterID int64  `json:"characterId"`
	InstanceID  int32  `json:"instanceId,omitempty"`
	WantsRiding *bool  `json:"wantsRiding,omitempty"`
	Revision    *int64 `json:"revision,omitempty"`
}
type BicycleStateResponse struct {
	Success     bool               `json:"success" tstype:"true"`
	RequestID   string             `json:"requestId"`
	CharacterID int64              `json:"characterId"`
	Bicycle     BicycleToggleState `json:"bicycle"`
}

func HandleBicycleState(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req BicycleStateRequest
	fail := func() {
		sendInventoryCommandError(ses, req.RequestID, opcodes.BicycleStateResponse, "Could not change Bicycle preference. Check its current state.")
	}
	if err := decodePlayerMovement(payload, &req); err != nil || req.RequestID == "" || len(req.RequestID) > 64 || req.CharacterID != int64(ses.Client.CharData().ID) || wh.PlayerMovement == nil {
		fail()
		return false
	}
	charID := int(req.CharacterID)
	state, ok := wh.PlayerMovement.bicycleStateForSession(ses.SessionID, charID)
	if !ok {
		fail()
		return false
	}
	if req.WantsRiding != nil {
		if req.Revision == nil || *req.Revision < 0 || req.InstanceID <= 0 {
			fail()
			return false
		}
		// Reuse the durable ownership policy without making this session-local
		// preference an inventory revision or publishing before read commit.
		err := db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
			if err := db.LockCharacter(tx, int64(charID)); err != nil {
				return err
			}
			if err := requireNoOwnedBattleIn(tx, int64(charID)); err != nil {
				return err
			}
			found, err := cqitems.NewStore(tx).FindInventoryItemByInstanceIDContext(ses.CommandContext(), int32(charID), req.InstanceID)
			if err != nil {
				return err
			}
			if itemuse.ShortName(found.Item) != "BICYCLE" {
				return fmt.Errorf("not an owned Bicycle")
			}
			return nil
		})
		if err != nil || ses.CommandContext().Err() != nil {
			fail()
			return false
		}
		state, ok = wh.PlayerMovement.setBicycleForSession(ses.SessionID, charID, *req.WantsRiding, *req.Revision)
		if !ok {
			fail()
			return false
		}
	} else if req.Revision != nil || req.InstanceID != 0 {
		fail()
		return false
	}
	ses.SendStreamJSON(BicycleStateResponse{Success: true, RequestID: req.RequestID, CharacterID: req.CharacterID, Bicycle: state}, opcodes.BicycleStateResponse)
	return false
}
