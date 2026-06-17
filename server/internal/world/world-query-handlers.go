package world

import (
	"context"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/session"
	"capturequest/internal/staticdata"
)

func HandleStaticDataRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	ctx := context.Background()
	data, err := staticdata.GetStaticData(ctx)
	if err != nil {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.StaticDataResponse)
		return false
	}

	ses.SendStreamJSON(map[string]interface{}{
		"success":     true,
		"classes":     StructToMap(data.Classes),
		"factions":    StructToMap(data.Factions),
		"maps":        StructToMap(data.Maps),
		"startCities": StructToMap(data.StartCities),
	}, opcodes.StaticDataResponse)
	return false
}

func HandleCharCreateDataRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	ctx := context.Background()
	data, err := staticdata.GetStaticData(ctx)
	if err != nil {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.CharCreateDataResponse)
		return false
	}
	ses.SendStreamJSON(map[string]interface{}{
		"success":     true,
		"factions":    StructToMap(data.Factions),
		"classes":     StructToMap(data.Classes),
		"startCities": StructToMap(data.StartCities),
	}, opcodes.CharCreateDataResponse)
	return false
}
