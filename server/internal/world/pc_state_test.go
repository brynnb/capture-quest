package world

import (
	"encoding/json"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func TestPCStateJoinsGameplayRecoveryAndOpening(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	testdb.Exec(t, wh.database, `INSERT INTO character_pc_state(character_id,current_box) VALUES(42,3);
 INSERT INTO character_pokemon(character_id,party_slot,box,box_slot,pokemon_id,level,cur_hp,max_hp) VALUES(42,NULL,3,0,25,12,10,95);
 INSERT INTO phaser_hidden_objects(id,map_constant,map_id,x,y,item_or_direction,routine,object_type) VALUES
 (10,'ROOM',50,15,7,'SPRITE_FACING_UP','OpenPokemonCenterPC','pc'),
 (11,'OTHER',51,13,3,'SPRITE_FACING_UP','OpenPokemonCenterPC','pc'),
 (12,'ROOM',50,0,4,'SPRITE_FACING_UP','PrintBenchGuyText','hidden');`)
	db.GlobalWorldDB = nil // Both consumers must use their captured world dependency.
	battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"pc","current":true}`)
	result := recoveryReply(t, messages)
	if result.PC.CurrentBox != 3 || len(result.PC.Box) != 1 || result.PC.Box[0].RowID <= 0 || result.PC.Box[0].BoxSlot != 0 || len(result.PC.Sources) != 1 || result.PC.Sources[0].ID != 10 || result.PC.Sources[0].Direction != "UP" {
		t.Fatalf("PC snapshot=%+v", result.PC)
	}
	if len(result.Party) != 1 || result.Party[0].RowID == result.PC.Box[0].RowID {
		t.Fatal("PC recovery lost stable party/box identity")
	}
	messages.streams = nil
	HandlePokemonPCOpen(ses, []byte(`{}`), wh)
	var opened struct {
		Success    bool
		CurrentBox int
		Box        []PokemonDTO
		Party      []PokemonDTO
		Sources    []PCInteractionSource
	}
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &opened) != nil || !opened.Success || opened.CurrentBox != result.PC.CurrentBox || len(opened.Box) != 1 || opened.Box[0].RowID != result.PC.Box[0].RowID || len(opened.Party) != 1 || len(opened.Sources) != 1 {
		t.Fatalf("opening=%+v", opened)
	}
}

func TestPCReadFailuresDoNotPublishPartialRecoveryOrSuccessfulOpening(t *testing.T) {
	for _, failure := range []string{"preference", "source", "box", "missing_table"} {
		t.Run(failure, func(t *testing.T) {
			wh, ses, messages := setupIssuedStep(t)
			switch failure {
			case "preference":
				testdb.Exec(t, wh.database, `INSERT INTO character_pc_state(character_id,current_box) VALUES(42,12)`)
			case "source":
				testdb.Exec(t, wh.database, `INSERT INTO phaser_hidden_objects(id,map_constant,map_id,x,y,item_or_direction,routine,object_type) VALUES(10,'ROOM',50,13,3,'SPRITE_FACING_LEFT','OpenPokemonCenterPC','pc')`)
			case "box":
				testdb.Exec(t, wh.database, `INSERT INTO character_pokemon(character_id,party_slot,box,box_slot,pokemon_id,level,cur_hp,max_hp) VALUES(42,NULL,0,0,999,12,1,95)`)
			case "missing_table":
				testdb.Exec(t, wh.database, `ALTER TABLE character_pc_state RENAME TO unavailable_pc_state`)
			}
			for _, opcode := range []opcodes.OpCode{opcodes.GameplayStateRequest, opcodes.PokemonPCOpenRequest} {
				messages.streams = nil
				battleDispatch(t, wh, ses, opcode, `{"requestId":"failed","current":true}`)
				var reply struct{ Success bool }
				if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success {
					t.Fatalf("failure=%s opcode=%d published partial success", failure, opcode)
				}
			}
		})
	}
}
