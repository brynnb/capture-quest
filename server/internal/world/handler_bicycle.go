package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/itemuse"
	"capturequest/internal/session"
	"context"
	"time"
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
		if battle := getBattle(int64(charID)); battle != nil && !battle.IsOver() {
			fail()
			return false
		}
		if req.Revision == nil || *req.Revision < 0 || req.InstanceID <= 0 {
			fail()
			return false
		}
		ctx, cancel := context.WithTimeout(ses.CommandContext(), 5*time.Second)
		defer cancel()
		found, err := cqitems.NewStore(wh.database).FindInventoryItemByInstanceIDContext(ctx, int32(charID), req.InstanceID)
		if err != nil || itemuse.ShortName(found.Item) != "BICYCLE" {
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
