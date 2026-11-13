package world

import (
	"capturequest/internal/logutil"
	"encoding/json"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

type WarpHomeRequest struct {
	RequestID       string `json:"requestId"`
	CharacterID     int64  `json:"characterId"`
	CommandRevision *int64 `json:"commandRevision,omitempty"`
	Current         bool   `json:"current,omitempty"`
}

type WarpHomeResponse struct {
	Success     bool   `json:"success" tstype:"true"`
	RequestID   string `json:"requestId"`
	CharacterID int64  `json:"characterId"`
	MoveName    string `json:"moveName"`
	MapID       int    `json:"mapId"`
	Message     string `json:"message"`
	Current     bool   `json:"current"`
	Revision    int64  `json:"revision"`
}

// HandleWarpHomeRequest is an emergency recovery action.
func HandleWarpHomeRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	char := ses.Client.CharData()
	if char == nil {
		return false
	}

	var req WarpHomeRequest
	reject := func() {
		sendInventoryCommandError(ses, req.RequestID, opcodes.WarpHomeResponse, "Could not recover home. Refresh current state before trying again.")
	}
	if err := decodePlayerMovement(payload, &req); err != nil || req.RequestID == "" || len(req.RequestID) > 64 || req.CharacterID != int64(char.ID) || (req.Current && req.CommandRevision != nil) || (!req.Current && req.CommandRevision == nil) {
		reject()
		return false
	}
	charID := req.CharacterID
	destination := recoverySpawnDestination()
	mapID, x, y := destination.MapID, destination.X, destination.Y
	if req.Current {
		var revisions map[string]int64
		err := db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
			if err := db.LockCharacter(tx, charID); err != nil {
				return err
			}
			var err error
			revisions, err = db.FieldCommandRevisions(tx, charID)
			return err
		})
		if err != nil {
			logutil.Debugf("[WarpHome] revision character=%d: %v", charID, err)
			reject()
			return false
		}
		// This read grants no mutation permission and never requires a healthy pose,
		// party or battle payload. The following reset still checks the revision.
		ses.SendStreamJSON(WarpHomeResponse{Success: true, RequestID: req.RequestID, CharacterID: charID, MoveName: "WARP_HOME", MapID: mapID, Current: true, Revision: revisions["travel"], Message: "Current recovery ownership."}, opcodes.WarpHomeResponse)
		return false
	}
	previousBattle := getBattle(charID)
	input, _ := json.Marshal(req)
	encoded, replay, err := db.ExecuteFieldCommand(ses.CommandContext(), wh.database, charID, "travel", req.RequestID, req.CommandRevision, input, func(tx db.DBTX) ([]byte, error) {
		// Emergency travel intentionally permits active battle and stale/missing
		// source pose. Character/revision ownership still guards the atomic reset.
		if err := endSafariSessionIn(tx, charID); err != nil {
			return nil, err
		}
		if err := pokebattle.DeleteBattleState(tx, charID); err != nil {
			return nil, err
		}
		if err := saveFieldDestinationIn(tx, charID, mapID, x, y); err != nil {
			return nil, err
		}
		return json.Marshal(WarpHomeResponse{Success: true, RequestID: req.RequestID, CharacterID: charID, MoveName: "WARP_HOME", MapID: mapID, Message: "Warped home.", Revision: *req.CommandRevision + 1})
	})
	if err != nil {
		logutil.Debugf("[WarpHome] character=%d: %v", charID, err)
		reject()
		return false
	}
	var reply WarpHomeResponse
	if err := json.Unmarshal(encoded, &reply); err != nil || !reply.Success || reply.RequestID != req.RequestID || reply.CharacterID != charID || reply.MoveName != "WARP_HOME" || reply.MapID != mapID || reply.Message == "" || reply.Current || reply.Revision != *req.CommandRevision+1 {
		reject()
		return false
	}
	if !replay {
		forgetBattle(charID, previousBattle)
		refreshSafariFlags(ses, wh, charID)
		publishCommittedPlayerPosition(ses, wh, mapID, x, y, RecoverySpawnDirection)
	}
	ses.SendStreamJSON(reply, opcodes.WarpHomeResponse)
	return false
}
