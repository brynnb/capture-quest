package world

import (
	"context"
	"strings"
	"testing"

	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func TestCutscenePreloadRejectsMalformedConditionsAndWeakerSchema(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(40,'original',10,10);
 INSERT INTO phaser_cutscene_scripts(script_label,map_name,requires_flags,actions) VALUES('Reward','original','["FLAG"]','[{"type":"give_money","money":1}]');`)
	m := NewCutsceneManager(database)
	if err := m.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	previous := m.GetByLabel("Reward")
	if previous == nil || len(previous.RequiresFlags) != 1 {
		t.Fatal("cutscene prerequisites not loaded")
	}
	testdb.Exec(t, database, `UPDATE phaser_maps SET name='unpublished' WHERE id=40`)
	cases := []struct{ field, raw string }{
		{"requires_flags", `42`},
		{"requires_flags", `[null]`},
		{"requires_flags_absent", `{"FLAG":true}`},
		{"sets_flags", `[""]`},
		{"actions", `null`},
		{"actions", `[{"type":"give_money","money":"bad"}]`},
		{"actions", `[{"type":"trainer_battle","postWinActions":[null]}]`},
	}
	for _, tc := range cases {
		t.Run(tc.field+tc.raw, func(t *testing.T) {
			testdb.Exec(t, database, `UPDATE phaser_cutscene_scripts SET requires_flags='["FLAG"]',requires_flags_absent=NULL,sets_flags=NULL,actions='[]'`)
			testdb.Exec(t, database, `UPDATE phaser_cutscene_scripts SET `+tc.field+`=$1`, tc.raw)
			if err := m.Load(context.Background()); err == nil || !strings.Contains(err.Error(), "Reward") {
				t.Fatalf("missing malformed script error: %v", err)
			}
			if m.GetByLabel("Reward") != previous || m.MapNameForID(40) != "original" {
				t.Fatal("failed preload published partial family")
			}
		})
	}
	if wh, err := NewWorldHandler(context.Background(), session.NewSessionManager()); wh != nil || err == nil || !strings.Contains(err.Error(), "preload cutscenes") {
		t.Fatalf("malformed cutscene admitted world: %v", err)
	}
	testdb.Exec(t, database, `UPDATE phaser_cutscene_scripts SET actions='[]'; ALTER TABLE phaser_cutscene_scripts DROP COLUMN requires_flags`)
	if err := m.Load(context.Background()); err == nil {
		t.Fatal("missing prerequisite column accepted through weaker query")
	}
	if m.GetByLabel("Reward") != previous {
		t.Fatal("schema failure replaced cache")
	}
}
