package world

func resolveScriptDialogueFallbackEntries(textConstant string, charID int64, _ *EventFlagManager) []PhaserDialogueEntry {
	if entries, ok := resolveInGameTradeDialogueEntries(textConstant, charID); ok {
		return entries
	}
	return nil
}
