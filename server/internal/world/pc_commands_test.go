package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/testdb"
	"encoding/json"
	"fmt"
	"testing"
)

func TestPCCommandsUseSourceStableIdentityAndOneCommittedProjection(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	testdb.Exec(t, wh.database, `INSERT INTO phaser_hidden_objects(id,map_constant,map_id,x,y,item_or_direction,routine,object_type) VALUES(10,'ROOM',50,7,7,'SPRITE_FACING_UP','OpenPokemonCenterPC','pc'); INSERT INTO character_pokemon(character_id,party_slot,box_slot,pokemon_id,level,cur_hp,max_hp) VALUES(42,1,1,129,5,1,20)`)
	var target int64
	if err := wh.database.QueryRow(`SELECT id FROM character_pokemon WHERE character_id=42 AND party_slot=1`).Scan(&target); err != nil {
		t.Fatal(err)
	}
	db.GlobalWorldDB = nil
	dispatch := func(opcode opcodes.OpCode, revision int, box int, want bool) PokemonPCResponse {
		t.Helper()
		messages.streams = nil
		row := target
		if opcode == opcodes.PokemonPCSwitchBoxRequest {
			row = 0
		}
		request := fmt.Sprintf(`{"requestId":"pc-%d","sourceId":10,"pokemonRowId":%d,"box":%d,"command":{"characterId":42,"revision":%d}}`, revision, row, box, revision)
		battleDispatch(t, wh, ses, opcode, request)
		var result PokemonPCResponse
		if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &result) != nil || result.Success != want {
			t.Fatalf("opcode=%d result=%+v messages=%v", opcode, result, messages.streams)
		}
		if want && result.Inventory.CommandRevision != int64(revision+1) {
			t.Fatal("revision not committed with projection")
		}
		return result
	}
	deposited := dispatch(opcodes.PokemonPCDepositRequest, 0, 0, true)
	if len(deposited.Party) != 1 || len(deposited.PC.Box) != 1 || deposited.PC.Box[0].RowID != target {
		t.Fatal("incorrect deposit projection")
	}
	dispatch(opcodes.PokemonPCDepositRequest, 0, 0, false)
	switched := dispatch(opcodes.PokemonPCSwitchBoxRequest, 1, 1, true)
	if switched.PC.CurrentBox != 1 || len(switched.PC.Box) != 0 {
		t.Fatal("incorrect box preference projection")
	}
	withdrawn := dispatch(opcodes.PokemonPCWithdrawRequest, 2, 0, true)
	if len(withdrawn.Party) != 2 || len(withdrawn.PC.Box) != 0 {
		t.Fatal("incorrect withdrawal")
	}
	dispatch(opcodes.PokemonPCDepositRequest, 3, 0, true)
	released := dispatch(opcodes.PokemonPCReleaseRequest, 4, 0, true)
	if len(released.PC.Box) != 0 || len(released.Party) != 1 {
		t.Fatal("incorrect release")
	}
	dispatch(opcodes.PokemonPCReleaseRequest, 5, 0, false)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"recover","current":true}`)
	recovered := recoveryReply(t, messages)
	if recovered.CommandRevision != 5 || len(recovered.PC.Box) != 0 || len(recovered.Party) != 1 {
		t.Fatal("PC recovery differs from commit")
	}
}

func TestPCCommandsRejectRemoteWrongFacingBattleAndOldSlotRequests(t *testing.T) {
	for _, scenario := range []string{"remote", "facing", "source", "battle", "legacy"} {
		t.Run(scenario, func(t *testing.T) {
			wh, ses, messages := setupIssuedStep(t)
			testdb.Exec(t, wh.database, `INSERT INTO phaser_hidden_objects(id,map_constant,map_id,x,y,item_or_direction,routine,object_type) VALUES(10,'ROOM',50,7,7,'SPRITE_FACING_UP','OpenPokemonCenterPC','pc')`)
			switch scenario {
			case "remote":
				testdb.Exec(t, wh.database, `UPDATE phaser_hidden_objects SET x=15 WHERE id=10`)
			case "facing":
				wh.PlayerMovement.players[42].Direction = "DOWN"
			case "source":
				testdb.Exec(t, wh.database, `UPDATE phaser_hidden_objects SET routine='PrintBenchGuyText' WHERE id=10`)
			case "battle":
				battleTestStart(t, wh.database, false, nil)
			}
			for _, opcode := range []opcodes.OpCode{opcodes.PokemonPCOpenRequest, opcodes.PokemonPCDepositRequest, opcodes.PokemonPCWithdrawRequest, opcodes.PokemonPCReleaseRequest, opcodes.PokemonPCSwitchBoxRequest} {
				payload := `{"requestId":"deny","sourceId":10,"pokemonRowId":1,"box":0,"command":{"characterId":42,"revision":0}}`
				if opcode == opcodes.PokemonPCSwitchBoxRequest {
					payload = `{"requestId":"deny","sourceId":10,"box":0,"command":{"characterId":42,"revision":0}}`
				}
				if opcode == opcodes.PokemonPCOpenRequest {
					payload = `{"requestId":"deny","sourceId":10,"characterId":42}`
				}
				if scenario == "legacy" {
					payload = `{"partySlot":0,"boxSlot":0,"box":0}`
				}
				messages.streams = nil
				battleDispatch(t, wh, ses, opcode, payload)
				var reply struct{ Success bool }
				if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success {
					t.Fatalf("scenario=%s opcode=%d accepted", scenario, opcode)
				}
			}
		})
	}
}

func TestPCCommandCommitFailureRollsBackSelectionAndMembership(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	testdb.Exec(t, wh.database, `INSERT INTO phaser_hidden_objects(id,map_constant,map_id,x,y,item_or_direction,routine,object_type) VALUES(10,'ROOM',50,7,7,'SPRITE_FACING_UP','OpenPokemonCenterPC','pc'); INSERT INTO character_pokemon(character_id,party_slot,box_slot,pokemon_id,level,cur_hp,max_hp) VALUES(42,1,1,129,5,1,20);
 CREATE FUNCTION reject_pc_response_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'PC commit rejected'; END $$; CREATE CONSTRAINT TRIGGER reject_pc_response_commit AFTER INSERT ON character_pc_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_pc_response_commit();`)
	var target int64
	if err := wh.database.QueryRow(`SELECT id FROM character_pokemon WHERE character_id=42 AND party_slot=1`).Scan(&target); err != nil {
		t.Fatal(err)
	}
	request := fmt.Sprintf(`{"requestId":"rollback","sourceId":10,"pokemonRowId":%d,"box":2,"command":{"characterId":42,"revision":0}}`, target)
	battleDispatch(t, wh, ses, opcodes.PokemonPCDepositRequest, request)
	var reply PokemonPCResponse
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success {
		t.Fatal("published failed PC commit")
	}
	var box, count int
	if err := wh.database.QueryRow(`SELECT box FROM character_pokemon WHERE id=$1`, target).Scan(&box); err != nil || box != -1 {
		t.Fatal("failed deposit changed membership")
	}
	if err := wh.database.QueryRow(`SELECT count(*) FROM character_pc_state WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed deposit changed preference")
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_pc_response_commit ON character_pc_state`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PokemonPCDepositRequest, request)
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || !reply.Success || reply.Inventory.CommandRevision != 1 || reply.PC.CurrentBox != 2 {
		t.Fatal("retry after rollback failed")
	}
}
