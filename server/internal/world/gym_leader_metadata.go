package world

type gymLeaderBattleMetadata struct {
	MapID             int
	TrainerClass      string
	WinFlag           string
	AlternateWinFlags []string
	PreBattleLabel    string
	AfterBattleLabel  string
}

var gymLeaderBattleMetadataByClass = map[string]gymLeaderBattleMetadata{
	"BROCK": {
		MapID:            54,
		TrainerClass:     "BROCK",
		WinFlag:          "EVENT_BEAT_BROCK",
		PreBattleLabel:   "_PewterGymBrockPreBattleText",
		AfterBattleLabel: "_PewterGymBrockPostBattleAdviceText",
	},
	"MISTY": {
		MapID:            65,
		TrainerClass:     "MISTY",
		WinFlag:          "EVENT_BEAT_MISTY",
		PreBattleLabel:   "_CeruleanGymMistyPreBattleText",
		AfterBattleLabel: "_CeruleanGymMistyTM11ExplanationText",
	},
	"LT_SURGE": {
		MapID:            92,
		TrainerClass:     "LT_SURGE",
		WinFlag:          "EVENT_BEAT_LT_SURGE",
		PreBattleLabel:   "_VermilionGymLTSurgePreBattleText",
		AfterBattleLabel: "_VermilionGymLTSurgePostBattleAdviceText",
	},
	"ERIKA": {
		MapID:            134,
		TrainerClass:     "ERIKA",
		WinFlag:          "EVENT_BEAT_ERIKA",
		PreBattleLabel:   "_CeladonGymErikaPreBattleText",
		AfterBattleLabel: "_CeladonGymErikaPostBattleAdviceText",
	},
	"KOGA": {
		MapID:            157,
		TrainerClass:     "KOGA",
		WinFlag:          "EVENT_BEAT_KOGA",
		PreBattleLabel:   "_FuchsiaGymKogaBeforeBattleText",
		AfterBattleLabel: "_FuchsiaGymKogaPostBattleAdviceText",
	},
	"BLAINE": {
		MapID:            166,
		TrainerClass:     "BLAINE",
		WinFlag:          "EVENT_BEAT_BLAINE",
		PreBattleLabel:   "_CinnabarGymBlainePreBattleText",
		AfterBattleLabel: "_CinnabarGymBlainePostBattleAdviceText",
	},
	"SABRINA": {
		MapID:            178,
		TrainerClass:     "SABRINA",
		WinFlag:          "EVENT_BEAT_SABRINA",
		PreBattleLabel:   "_SaffronGymSabrinaText",
		AfterBattleLabel: "_SaffronGymSabrinaPostBattleAdviceText",
	},
	"GIOVANNI": {
		MapID:             45,
		TrainerClass:      "GIOVANNI",
		WinFlag:           "EVENT_BEAT_VIRIDIAN_GYM_GIOVANNI",
		AlternateWinFlags: []string{"EVENT_BEAT_GIOVANNI_GYM"},
		PreBattleLabel:    "_ViridianGymGiovanniPreBattleText",
		AfterBattleLabel:  "_ViridianGymGiovanniPostBattleAdviceText",
	},
}

func gymLeaderMetadataFor(mapID int, trainerClass string) (gymLeaderBattleMetadata, bool) {
	meta, ok := gymLeaderBattleMetadataByClass[trainerClass]
	if !ok || meta.MapID != mapID {
		return gymLeaderBattleMetadata{}, false
	}
	return meta, true
}

func gymLeaderMetadataForMap(mapID int) (gymLeaderBattleMetadata, bool) {
	for _, meta := range gymLeaderBattleMetadataByClass {
		if meta.MapID == mapID {
			return meta, true
		}
	}
	return gymLeaderBattleMetadata{}, false
}

func trainerBattleSuppressedByGymLeaderDefeat(charID int64, t *trainerSightData, wh *WorldHandler) bool {
	if t == nil || t.IsGymLeader || wh == nil || wh.EventFlags == nil {
		return false
	}
	meta, ok := gymLeaderMetadataForMap(t.MapID)
	return ok && gymLeaderDefeatedForCharacter(charID, meta, wh.EventFlags)
}

func gymLeaderDefeatedForCharacter(charID int64, meta gymLeaderBattleMetadata, efm *EventFlagManager) bool {
	if efm == nil {
		return false
	}
	if meta.WinFlag != "" && efm.CheckFlag(charID, meta.WinFlag) {
		return true
	}
	for _, flag := range meta.AlternateWinFlags {
		if flag != "" && efm.CheckFlag(charID, flag) {
			return true
		}
	}
	return false
}

func applyGymLeaderBattleMetadata(t *trainerSightData) {
	if t == nil {
		return
	}
	meta, ok := gymLeaderMetadataFor(t.MapID, t.TrainerClass)
	if !ok {
		return
	}
	t.IsGymLeader = true
	if t.EventFlag == "" {
		t.EventFlag = meta.WinFlag
	}
	if t.BattleTextLabel == "" {
		t.BattleTextLabel = meta.PreBattleLabel
	}
	if t.AfterBattleTextLabel == "" {
		t.AfterBattleTextLabel = meta.AfterBattleLabel
	}
}
