package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/content"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"capturequest/internal/staticdata"
)

type StaticDataRequest struct {
	RequestID string `json:"requestId"`
}

type StaticDataResponse struct {
	RequestID   string                     `json:"requestId"`
	Success     bool                       `json:"success" tstype:"true"`
	Classes     []staticdata.ClassInfo     `json:"classes" tstype:"import(\"./staticdata\").ClassInfo[]"`
	Factions    []staticdata.FactionInfo   `json:"factions" tstype:"import(\"./staticdata\").FactionInfo[]"`
	Maps        []staticdata.MapInfo       `json:"maps" tstype:"import(\"./staticdata\").MapInfo[]"`
	StartCities []staticdata.StartCityInfo `json:"startCities" tstype:"import(\"./staticdata\").StartCityInfo[]"`
}

func sendStaticData(ses *session.Session, payload []byte, wh *WorldHandler, opcode opcodes.OpCode) bool {
	var req StaticDataRequest
	if decodePlayerMovement(payload, &req) != nil || !validBattleRequestID(req.RequestID) {
		ses.SendStreamJSON(protocol.PlayerStepError{RequestID: req.RequestID, Error: "Invalid static read request"}, opcode)
		return false
	}
	service := wh.Content
	if service == nil {
		service = content.New(wh.database)
	}
	data, err := service.StaticData(ses.CommandContext())
	if err != nil {
		ses.SendStreamJSON(protocol.PlayerStepError{RequestID: req.RequestID, Error: "Static content unavailable; retry the read"}, opcode)
		return false
	}
	ses.SendStreamJSON(StaticDataResponse{RequestID: req.RequestID, Success: true, Classes: data.Classes, Factions: data.Factions, Maps: data.Maps, StartCities: data.StartCities}, opcode)
	return false
}
func HandleStaticDataRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	return sendStaticData(ses, payload, wh, opcodes.StaticDataResponse)
}
func HandleCharCreateDataRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	return sendStaticData(ses, payload, wh, opcodes.CharCreateDataResponse)
}
