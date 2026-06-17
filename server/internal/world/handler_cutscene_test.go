package world

import (
	"encoding/json"
	"testing"
)

func TestNormalizePokemonLookupName(t *testing.T) {
	tests := map[string]string{
		"Lapras":         "LAPRAS",
		"MR. MIME":       "MR_MIME",
		"Mr Mime":        "MR_MIME",
		"Farfetch'd":     "FARFETCHD",
		"Nidoran♂":       "NIDORAN_M",
		"NIDORAN_FEMALE": "NIDORAN_F",
	}

	for input, want := range tests {
		if got := normalizePokemonLookupName(input); got != want {
			t.Fatalf("normalizePokemonLookupName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDecodeCutsceneActionsPreservesShowActorSprite(t *testing.T) {
	raw := json.RawMessage(`[
		{"type":"hideObject","textConstant":"TEXT_OAKSLAB_OAK"},
		{"type":"showActor","actor":"OAK","x":5,"y":2,"sprite":"SPRITE_OAK"}
	]`)

	actions, err := DecodeCutsceneActions(raw)
	if err != nil {
		t.Fatalf("DecodeCutsceneActions returned error: %v", err)
	}
	if got, want := actions[1].Sprite, "SPRITE_OAK"; got != want {
		t.Fatalf("decoded sprite = %q, want %q", got, want)
	}

	encoded, err := json.Marshal(actions)
	if err != nil {
		t.Fatalf("Marshal actions returned error: %v", err)
	}
	var roundTrip []map[string]interface{}
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatalf("Unmarshal encoded actions returned error: %v", err)
	}
	if got, want := roundTrip[1]["sprite"], "SPRITE_OAK"; got != want {
		t.Fatalf("round-tripped sprite = %v, want %q", got, want)
	}
}

func TestDecodeCutsceneActionsPreservesAudioActions(t *testing.T) {
	raw := json.RawMessage(`[
		{"type":"playSFX","sfxConstant":"SFX_GET_ITEM_1","volume":0.6},
		{"type":"playCry","pokemonName":"PIKACHU","sfxConstant":"SFX_CRY_0F","volume":0.8}
	]`)

	actions, err := DecodeCutsceneActions(raw)
	if err != nil {
		t.Fatalf("DecodeCutsceneActions returned error: %v", err)
	}
	if got, want := actions[0].SFXConstant, "SFX_GET_ITEM_1"; got != want {
		t.Fatalf("decoded sfxConstant = %q, want %q", got, want)
	}
	if got, want := actions[0].Volume, 0.6; got != want {
		t.Fatalf("decoded volume = %v, want %v", got, want)
	}
	if got, want := actions[1].PokemonName, "PIKACHU"; got != want {
		t.Fatalf("decoded pokemonName = %q, want %q", got, want)
	}
	if got, want := CutsceneActionSummary(actions[1]), "PIKACHU"; got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func TestDecodeCutsceneActionsPreservesGameCornerPrizeVendor(t *testing.T) {
	raw := json.RawMessage(`[
		{"type":"gameCornerPrizeVendor","textConstant":"TEXT_GAMECORNERPRIZEROOM_PRIZE_VENDOR_3","prizeWindow":3}
	]`)

	actions, err := DecodeCutsceneActions(raw)
	if err != nil {
		t.Fatalf("DecodeCutsceneActions returned error: %v", err)
	}
	if got, want := actions[0].PrizeWindow, 3; got != want {
		t.Fatalf("prizeWindow = %d, want %d", got, want)
	}
	if got, want := CutsceneActionSummary(actions[0]), "TEXT_GAMECORNERPRIZEROOM_PRIZE_VENDOR_3 window=3"; got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func TestFillBufferedItemDialogueLines(t *testing.T) {
	lines := []string{
		"(PLAYER) received\n!",
		"contains\nICE BEAM!",
		"(PLAYER) received\na \n!",
		"(PLAYER) received\nan \n!",
	}
	got := fillBufferedItemDialogueLines(lines, "TM13")
	want := []string{
		"(PLAYER) received\nTM13!",
		"TM13 contains\nICE BEAM!",
		"(PLAYER) received\na TM13!",
		"(PLAYER) received\nan TM13!",
	}
	if len(got) != len(want) {
		t.Fatalf("filled lines length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
	if lines[0] == got[0] {
		t.Fatalf("first buffered line was not filled")
	}
}

func TestApplyCutsceneActionListEmitsChoiceBranchDialogue(t *testing.T) {
	choice := false
	raw := json.RawMessage(`[
		{
			"type": "choice",
			"prompt": "Do you believe in GHOSTs?",
			"noLines": ["Hahaha, I guess not."]
		},
		{
			"type": "dialogue",
			"lines": ["This should not run."]
		}
	]`)

	effects, completed, err := ApplyCutsceneActionList(CutsceneActionContext{
		Choice:       &choice,
		StopAtChoice: true,
	}, "LAVENDER_TOWN", raw, 0)
	if err != nil {
		t.Fatalf("ApplyCutsceneActionList returned error: %v", err)
	}
	if completed {
		t.Fatalf("choice without continueOnNo completed, want stopped")
	}
	if len(effects) != 2 {
		t.Fatalf("effects length = %d, want choice plus branch dialogue", len(effects))
	}
	if effects[0].Type != "choice" {
		t.Fatalf("first effect type = %s, want choice", effects[0].Type)
	}
	if effects[1].Type != "dialogue" || effects[1].Detail != " [Hahaha, I guess not.]" {
		t.Fatalf("branch dialogue effect = %#v", effects[1])
	}
}

func TestCutsceneAffectsTrainerCardForBadgeSetsFlags(t *testing.T) {
	cs := &CutsceneScript{
		ScriptLabel: "ViridianGymGiovanniPostBattle",
		SetsFlags:   []string{"EVENT_GOT_EARTHBADGE"},
	}

	if !cutsceneAffectsTrainerCard(cs) {
		t.Fatalf("expected badge SetsFlags to affect trainer card")
	}
}

func TestCutsceneAffectsTrainerCardForNestedBadgeAction(t *testing.T) {
	cs := &CutsceneScript{
		ScriptLabel: "NestedBadgeGrant",
		Actions: json.RawMessage(`[
			{
				"type": "parallel",
				"actions": [
					{"type": "setFlag", "flag": "EVENT_GOT_BOULDERBADGE"}
				]
			}
		]`),
	}

	if !cutsceneAffectsTrainerCard(cs) {
		t.Fatalf("expected nested badge action to affect trainer card")
	}
}

func TestCutsceneAffectsTrainerCardIgnoresNonBadgeFlags(t *testing.T) {
	cs := &CutsceneScript{
		ScriptLabel: "NonBadgeFlags",
		SetsFlags:   []string{"EVENT_GOT_TM27"},
		Actions: json.RawMessage(`[
			{"type": "setFlag", "flag": "EVENT_BEAT_BROCK"}
		]`),
	}

	if cutsceneAffectsTrainerCard(cs) {
		t.Fatalf("expected non-badge flags not to affect trainer card")
	}
}
