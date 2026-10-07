package scriptcandidateimport

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestNurseMacroFamilyCompilesSourceIdentityAndRejectsMalformedRecords(t *testing.T) {
	if _, err := mapActions([]candidateAction{{Type: "healParty", HealingPolicy: "unknown"}}); err == nil {
		t.Fatal("accepted unknown healing policy")
	}
	path := createSQLite(t, false, nil)
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	_, err = database.Exec(`
 DROP TABLE text_pointers; DROP TABLE dialogue_text;
 CREATE TABLE script_event_ir_blocks(id INTEGER,map_name TEXT,map_id INTEGER,label TEXT,kind TEXT,raw_asm TEXT);
 CREATE TABLE text_pointers(map_name TEXT,map_id INTEGER,local_label TEXT,text_constant TEXT,is_trainer INTEGER);
 CREATE TABLE dialogue_text(label TEXT,dialogue TEXT);
 INSERT INTO maps(id,name) VALUES(41,'FIRST_CENTER'),(42,'SECOND_CENTER');
 INSERT INTO script_event_ir_blocks VALUES(1,'FirstCenter',41,'FirstNurse','text','FirstNurse:
 script_pokecenter_nurse'),(2,'SecondCenter',42,'SecondNurse','text','SecondNurse:
 script_pokecenter_nurse');
 INSERT INTO text_pointers VALUES('FirstCenter',41,'FirstNurse','TEXT_FIRST_NURSE',0),('SecondCenter',42,'SecondNurse','TEXT_SECOND_NURSE',0);
 INSERT INTO dialogue_text VALUES('_PokemonCenterWelcomeText','Welcome'),('_ShallWeHealYourPokemonText','Heal?'),('_NeedYourPokemonText','OK'),('_PokemonFightingFitText','Fit'),('_PokemonCenterFarewellText','Bye');`)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := loadTextMacroCandidates(context.Background(), database)
	if err != nil || len(candidates) != 2 {
		t.Fatalf("candidates=%v err=%v", candidates, err)
	}
	for _, candidate := range candidates {
		event, err := mapCandidate(candidate)
		if err != nil || event.Trigger.Label != candidate.Trigger.Label || !strings.Contains(string(event.Actions[4]), `"healingPolicy":"center"`) {
			t.Fatalf("event=%+v err=%v", event, err)
		}
	}
	for _, mutation := range []string{
		`UPDATE script_event_ir_blocks SET raw_asm=raw_asm || '\n extra_command' WHERE id=1`,
		`UPDATE text_pointers SET map_id=99 WHERE local_label='FirstNurse'`,
		`INSERT INTO script_event_ir_blocks SELECT * FROM script_event_ir_blocks WHERE id=1`,
		`DELETE FROM dialogue_text WHERE label='_NeedYourPokemonText'`,
	} {
		backup := t.TempDir() + "/backup.db"
		if _, err := database.Exec(`VACUUM INTO ?`, backup); err != nil {
			t.Fatal(err)
		}
		broken, err := sql.Open("sqlite", backup)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := broken.Exec(mutation); err != nil {
			t.Fatal(err)
		}
		if _, err := loadTextMacroCandidates(context.Background(), broken); err == nil {
			t.Fatalf("accepted malformed source: %s", mutation)
		}
		broken.Close()
	}
}
