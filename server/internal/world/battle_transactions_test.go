package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/content"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	model "capturequest/internal/db/models"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func battleTestWorld(t *testing.T) (*sql.DB, *WorldHandler, *session.Session, *recordingMessenger) {
	t.Helper()
	database := testdb.Postgres(t)
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: database}
	t.Cleanup(func() { forgetBattle(42, getBattle(42)); db.GlobalWorldDB = previous })
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(42,'battle');
 INSERT INTO character_wallet(character_id,pokedollars) VALUES(42,100);
 INSERT INTO phaser_pokemon(id,name,type_1,hp,atk,def,spd,spc,catch_rate,base_exp,growth_rate) VALUES
 (25,'PIKACHU','ELECTRIC',35,55,30,90,50,190,82,'MEDIUM_FAST'),(129,'MAGIKARP','WATER',20,10,55,80,20,255,20,'SLOW');
 INSERT INTO phaser_moves(id,constant_name,name,short_name,effect,power,type,accuracy,pp) VALUES(150,'SPLASH','SPLASH','SPLASH','SPLASH_EFFECT',0,'NORMAL',0,40);
 INSERT INTO character_pokemon(character_id,party_slot,box_slot,pokemon_id,level,exp,cur_hp,max_hp) VALUES(42,0,0,25,50,125000,1,95);
 INSERT INTO cq_items(id,name,short_name,is_usable,heal_amount,ball_modifier) VALUES(1,'Potion','POTION',true,20,0),(2,'Master Ball','MASTER_BALL',true,0,255);`)
	wh := &WorldHandler{database: database, Content: content.New(database), EventFlags: NewEventFlagManager(database)}
	messages := &recordingMessenger{}
	ses := &session.Session{Authenticated: true, Client: &testSessionClient{char: &model.CharacterData{ID: 42}}, Messenger: messages}
	return database, wh, ses, messages
}

func battleTestStart(t *testing.T, database *sql.DB, trainer bool, prepare func(*pokebattle.BattleState)) *pokebattle.BattleState {
	t.Helper()
	enemy, err := pokebattle.LoadPokemonFromDB(database, 129)
	if err != nil {
		t.Fatal(err)
	}
	enemy.Level = 5
	enemy.RecalculateStats()
	enemy.CurHP = enemy.MaxHP
	enemy.Moves[0] = pokebattle.MoveSlot{ID: 150, Name: "SPLASH", PP: 40, MaxPP: 40, BasePP: 40, Effect: "SPLASH_EFFECT"}
	battle := pokebattle.NewWildBattle(nil, enemy)
	if trainer {
		battle.BattleType = pokebattle.BattleTrainer
		battle.Trainer = &pokebattle.TrainerMeta{ClassName: "TEST", TrainerObjectID: 77, WinFlag: "WON_TEST", PrizeMoney: 250}
	}
	if prepare != nil {
		prepare(battle)
	}
	battle, err = startBattle(context.Background(), database, 42, battle)
	if err != nil {
		t.Fatal(err)
	}
	return battle
}

func battleDispatch(t *testing.T, wh *WorldHandler, ses *session.Session, opcode opcodes.OpCode, payload string) {
	t.Helper()
	registry := NewWorldOpCodeRegistry()
	registry.WH = wh
	// Older domain fixtures express the action only. Bind those valid commands to
	// their current durable identity; tests of missing/stale identity dispatch raw
	// packets or supply an explicit battle field, which is never rewritten here.
	switch opcode {
	case opcodes.PokeBattleActionRequest, opcodes.PokeBattleSwitchRequest, opcodes.CQBattleItemUseRequest, opcodes.PokeMoveLearnRequest, opcodes.PokeBattleCloseRequest:
		var request map[string]json.RawMessage
		if json.Unmarshal([]byte(payload), &request) == nil && request != nil {
			if _, explicit := request["requestId"]; !explicit {
				request["requestId"] = json.RawMessage(`"test-battle"`)
			}
			if _, explicit := request["battle"]; !explicit {
				if battle := getBattle(int64(ses.Client.CharData().ID)); battle != nil {
					identity, err := json.Marshal(BattleCommandIdentity{BattleID: battle.BattleID, Revision: battle.Revision})
					if err != nil {
						t.Fatal(err)
					}
					request["battle"] = identity
				}
			}
			data, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			payload = string(data)
		}
	}
	registry.HandleWorldPacket(ses, clientPacket(opcode, payload))
}

func TestStandaloneBlackoutCommitsRecoveryOrPublishesNoDestination(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	char := ses.Client.CharData()
	char.MapID, char.X, char.Y = 220, 7, 8
	wh.PlayerMovement = NewPlayerMovementManager(wh, nil)
	wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 220, "UP")
	wh.Safari = NewSafariZoneManager(database)
	if err := wh.Safari.SetSession(context.Background(), 42, SafariSession{Active: true, BallsLeft: 30, StepsLeft: 500}); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `UPDATE character_data SET map_id=220,x=7,y=8 WHERE id=42;
 CREATE FUNCTION reject_blackout_party_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late blackout failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_blackout_party_commit AFTER UPDATE ON character_pokemon
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_blackout_party_commit();`)
	db.GlobalWorldDB = nil
	sendStandaloneBlackout(ses, wh, 42)
	for _, message := range messages.streams {
		if message.opcode == opcodes.PokeBattleEndNotify {
			t.Fatal("failed blackout published a destination")
		}
	}
	var money, mapID, x, y, hp int
	if err := database.QueryRow(`SELECT pokedollars,map_id,x,y,cur_hp FROM character_wallet w JOIN character_data c ON c.id=w.character_id JOIN character_pokemon p ON p.character_id=c.id WHERE c.id=42`).Scan(&money, &mapID, &x, &y, &hp); err != nil || money != 100 || mapID != 220 || x != 7 || y != 8 || hp != 1 {
		t.Fatal("failed blackout changed durable state", err)
	}
	visit, err := wh.Safari.GetSession(context.Background(), 42)
	if err != nil || visit == nil || !visit.Active || char.MapID != 220 {
		t.Fatal("failed blackout changed Safari/live position")
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_blackout_party_commit ON character_pokemon`)
	messages.streams = nil
	sendStandaloneBlackout(ses, wh, 42)
	var end struct {
		Blackout      bool
		BlackoutMapID int
		BlackoutX     int
		BlackoutY     int
	}
	if len(messages.streams) < 2 || messages.streams[0].opcode != opcodes.CharacterWallet || messages.streams[1].opcode != opcodes.PokeBattleEndNotify || json.Unmarshal(messages.streams[1].payload, &end) != nil || !end.Blackout || end.BlackoutMapID != 41 {
		t.Fatal("committed blackout omitted result")
	}
	if err := database.QueryRow(`SELECT pokedollars,map_id,x,y,cur_hp FROM character_wallet w JOIN character_data c ON c.id=w.character_id JOIN character_pokemon p ON p.character_id=c.id WHERE c.id=42`).Scan(&money, &mapID, &x, &y, &hp); err != nil || money != 50 || mapID != end.BlackoutMapID || x != end.BlackoutX || y != end.BlackoutY || hp != 95 {
		t.Fatal("blackout result preceded durable recovery", err)
	}
	if char.MapID != uint32(mapID) || char.X != float64(x) || char.Y != float64(y) {
		t.Fatal("blackout did not publish owned position")
	}
	visit, err = wh.Safari.GetSession(context.Background(), 42)
	if err != nil || visit != nil {
		t.Fatal("blackout did not end Safari")
	}
}

func TestBattleItemTurnCommitFailureDoesNotSpendOrPublish(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	current := battleTestStart(t, database, false, nil)
	instance, err := cqitems.NewStore(database).AddItemToInventory(42, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_battle_state ADD CONSTRAINT reject_turn CHECK((battle_json::json->>'revision')::int=1)`)
	request := fmt.Sprintf(`{"action":"item","instanceId":%d,"itemId":1,"targetSlot":0,"moveSlot":-1}`, instance)
	battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, request)
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.PokeBattleActionResponse {
		t.Fatalf("failure published messages: %+v", messages.streams)
	}
	var failure struct {
		Success bool
		Error   string
	}
	if err := json.Unmarshal(messages.streams[0].payload, &failure); err != nil {
		t.Fatal(err)
	}
	if failure.Success || failure.Error == "" {
		t.Fatalf("failure=%+v", failure)
	}
	if getBattle(42) != current || current.PlayerParty[0].CurHP != 1 {
		t.Fatal("failure replaced or mutated published state")
	}
	owned, err := cqitems.NewStore(database).FindInventoryItemByInstanceID(42, instance)
	if err != nil || owned.Instance.Quantity != 1 {
		t.Fatalf("spent failed item: %+v %v", owned, err)
	}
	saved, err := pokebattle.ResumeBattle(context.Background(), database, 42)
	if err != nil || saved.Revision != 1 || saved.PlayerParty[0].CurHP != 1 {
		t.Fatalf("saved partial turn: %+v %v", saved, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_battle_state DROP CONSTRAINT reject_turn`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, request)
	next := getBattle(42)
	if next == current || next.Revision != 2 || next.PlayerParty[0].CurHP != 21 {
		t.Fatalf("committed battle: %+v", next)
	}
	if _, err := cqitems.NewStore(database).FindInventoryItemByInstanceID(42, instance); err != sql.ErrNoRows {
		t.Fatalf("item still present: %v", err)
	}
	// Repeating an exhausted instance cannot produce a second item effect/turn.
	battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, request)
	if getBattle(42) != next || getBattle(42).TurnNumber != next.TurnNumber {
		t.Fatal("repeated exhausted item advanced battle")
	}
}

func TestMedicineTurnSettlesTrainerVictoryAndRollback(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	current := battleTestStart(t, database, true, func(b *pokebattle.BattleState) {
		b.EnemyParty[0].CurHP = 1
		b.EnemyParty[0].Status = pokebattle.StatusPoison
	})
	instance, err := cqitems.NewStore(database).AddItemToInventory(42, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_wallet ADD CONSTRAINT reject_reward CHECK(pokedollars<=100)`)
	request := fmt.Sprintf(`{"action":"item","instanceId":%d,"targetSlot":0}`, instance)
	battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, request)
	if getBattle(42) != current || wh.EventFlags.CheckFlag(42, "WON_TEST") {
		t.Fatal("failed reward published battle or flag")
	}
	var defeated int
	if err := database.QueryRow(`SELECT count(*) FROM character_defeated_trainers WHERE character_id=42`).Scan(&defeated); err != nil || defeated != 0 {
		t.Fatalf("failed victory recorded defeat: %d %v", defeated, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_wallet DROP CONSTRAINT reject_reward`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, request)
	finished := getBattle(42)
	if !finished.IsOver() || !finished.PlayerWon() || finished.PlayerParty[0].Exp <= 125000 || !wh.EventFlags.CheckFlag(42, "WON_TEST") {
		t.Fatalf("medicine victory was not settled: %+v", finished)
	}
	var money int
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 350 {
		t.Fatalf("money=%d err=%v", money, err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM character_defeated_trainers WHERE character_id=42`).Scan(&defeated); err != nil || defeated != 1 {
		t.Fatalf("defeated=%d err=%v", defeated, err)
	}
	battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, request)
	if getBattle(42) != finished {
		t.Fatal("repeated winning action changed battle")
	}
}

func TestBattleCaptureUsesOneTransaction(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	current := battleTestStart(t, database, false, func(b *pokebattle.BattleState) { b.GuaranteedCatch = true; b.WildWinFlag = "CAUGHT_TEST" })
	instance, err := cqitems.NewStore(database).AddItemToInventory(42, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_pokemon ADD CONSTRAINT reject_capture CHECK(pokemon_id<>129)`)
	request := fmt.Sprintf(`{"action":"item","instanceId":%d,"itemId":2}`, instance)
	battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, request)
	if getBattle(42) != current {
		t.Fatal("failed capture changed runtime")
	}
	var caught int
	if err := database.QueryRow(`SELECT caught FROM character_pokedex WHERE character_id=42 AND pokemon_id=129`).Scan(&caught); err != nil || caught != 0 {
		t.Fatalf("failed capture pokedex=%d %v", caught, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_pokemon DROP CONSTRAINT reject_capture`)
	battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, request)
	finished := getBattle(42)
	if !finished.PlayerCaught || len(finished.PlayerParty) != 2 || finished.PlayerParty[1].RowID <= 0 || !wh.EventFlags.CheckFlag(42, "CAUGHT_TEST") {
		t.Fatal("capture not committed")
	}
	saveBattleOnDisconnect(42)
	restored, err := restoreBattleOnLogin(context.Background(), database, 42)
	if err != nil || !restored.PlayerCaught || len(restored.PlayerParty) != 2 || restored.PlayerParty[1].RowID != finished.PlayerParty[1].RowID {
		t.Fatalf("capture resume=%+v error=%v", restored, err)
	}
}

func TestInvalidBattleActionPreservesFaintSwitchPhase(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	current := battleTestStart(t, database, false, nil)
	next, err := pokebattle.CommitBattle(context.Background(), database, 42, current, func(_ db.DBTX, b *pokebattle.BattleState) error { b.Phase = pokebattle.PhaseFaintSwitch; return nil })
	if err != nil {
		t.Fatal(err)
	}
	setBattle(42, next)
	battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, `{"action":"fight","moveSlot":0}`)
	if getBattle(42) != next || next.Phase != pokebattle.PhaseFaintSwitch {
		t.Fatal("invalid request bypassed faint switch")
	}
	battleDispatch(t, wh, ses, opcodes.PokeBattleCloseRequest, `{}`)
	if getBattle(42) != next {
		t.Fatal("client closed active battle")
	}
	saveBattleOnDisconnect(42)
	restored, err := restoreBattleOnLogin(context.Background(), database, 42)
	if err != nil || restored.Phase != pokebattle.PhaseFaintSwitch {
		t.Fatalf("resume lost phase: %+v %v", restored, err)
	}
}

func TestMoveLearningPublishesOnlyAfterPartyAndBattleCommit(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	battle := battleTestStart(t, database, false, nil)
	pending, err := pokebattle.CommitBattle(context.Background(), database, 42, battle, func(_ db.DBTX, next *pokebattle.BattleState) error {
		next.Phase = pokebattle.PhaseBattleEnd
		next.PendingMoveLearn = &pokebattle.PendingMove{PokemonIndex: 0, MoveID: 150, MoveName: "SPLASH"}
		next.PostMoveLearnEvents = []pokebattle.BattleEvent{{Type: pokebattle.EventMessage, Message: "Trainer reward"}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	setBattle(42, pending)
	testdb.Exec(t, database, `ALTER TABLE character_pokemon ADD CONSTRAINT reject_move CHECK(move1_id<>150)`)
	battleDispatch(t, wh, ses, opcodes.PokeMoveLearnRequest, `{"forgetSlot":0}`)
	if len(messages.streams) != 1 || getBattle(42) != pending || pending.PendingMoveLearn == nil || pending.PlayerParty[0].Moves[0].ID != 0 {
		t.Fatal("failed move learning published new state")
	}
	var response struct{ Success bool }
	if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success {
		t.Fatalf("move failure response=%+v error=%v", response, err)
	}
	saveBattleOnDisconnect(42)
	restored, err := restoreBattleOnLogin(context.Background(), database, 42)
	if err != nil || restored.PendingMoveLearn == nil || len(restored.PostMoveLearnEvents) != 1 {
		t.Fatalf("lost pending choice after failure/disconnect: %+v %v", restored, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_pokemon DROP CONSTRAINT reject_move`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PokeMoveLearnRequest, `{"forgetSlot":0}`)
	if len(messages.streams) != 2 || messages.streams[1].opcode != opcodes.PokemonPartyResponse {
		t.Fatalf("move success messages=%+v", messages.streams)
	}
	var learned BattleCommandResponse
	if err := json.Unmarshal(messages.streams[0].payload, &learned); err != nil || !learned.Success || learned.Battle == nil || learned.Battle.BattleID != pending.BattleID || learned.Battle.Revision != pending.Revision+1 || learned.Learning == nil || len(learned.Learning.PostEvents) != 1 {
		t.Fatalf("learned=%+v error=%v", learned, err)
	}
	saved, err := pokebattle.ResumeBattle(context.Background(), database, 42)
	if err != nil || saved.PendingMoveLearn != nil || len(saved.PostMoveLearnEvents) != 0 || saved.PlayerParty[0].Moves[0].ID != 150 {
		t.Fatalf("move commit inconsistent: %+v %v", saved, err)
	}
	battleDispatch(t, wh, ses, opcodes.PokeBattleCloseRequest, `{}`)
	if saved, err := pokebattle.LoadBattleState(database, 42); err != nil || saved != nil || getBattle(42) != nil {
		t.Fatalf("close left state: %+v %v", saved, err)
	}
}

func TestBlackoutAndPartyHealRollBackWithBattle(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	ses.Client.CharData().MapID, ses.Client.CharData().X, ses.Client.CharData().Y = 220, 7, 8
	testdb.Exec(t, database, `UPDATE character_data SET map_id=220,x=7,y=8 WHERE id=42`)
	current := battleTestStart(t, database, false, nil)
	var result battleTurnResult
	apply := func(tx db.DBTX, next *pokebattle.BattleState) (err error) {
		next.PlayerParty[0].CurHP = 0
		next.Phase = pokebattle.PhaseBattleEnd
		result, err = settleBattleTurn(tx, 42, next, battleTurnResult{Events: []pokebattle.BattleEvent{{Type: pokebattle.EventBattleLose}}})
		return err
	}
	testdb.Exec(t, database, `ALTER TABLE character_battle_state ADD CONSTRAINT reject_end CHECK((battle_json::json->>'revision')::int=1)`)
	if _, err := pokebattle.CommitBattle(context.Background(), database, 42, current, apply); err == nil {
		t.Fatal("accepted failed blackout save")
	}
	var money int
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 100 {
		t.Fatalf("failed blackout money=%d %v", money, err)
	}
	var mapID, x, y int
	if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&mapID, &x, &y); err != nil || mapID != 220 || x != 7 || y != 8 {
		t.Fatal("failed battle blackout changed destination", err)
	}
	saved, err := pokebattle.ResumeBattle(context.Background(), database, 42)
	if err != nil || saved.IsOver() || saved.PlayerParty[0].CurHP != 1 {
		t.Fatal("failed blackout changed battle/party")
	}
	testdb.Exec(t, database, `ALTER TABLE character_battle_state DROP CONSTRAINT reject_end`)
	next, err := pokebattle.CommitBattle(context.Background(), database, 42, current, apply)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Lost || result.Blackout == nil || result.Blackout.NewMoney != 50 || result.Blackout.MapID != 41 || next.PlayerParty[0].CurHP != next.PlayerParty[0].MaxHP {
		t.Fatalf("blackout result=%+v", result)
	}
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 50 {
		t.Fatalf("blackout money=%d %v", money, err)
	}
	if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&mapID, &x, &y); err != nil || mapID != result.Blackout.MapID || x != result.Blackout.X || y != result.Blackout.Y {
		t.Fatal("battle blackout omitted durable destination", err)
	}
	publishBattleTurn(ses, wh, 42, next, result, opcodes.PokeBattleActionResponse, "test-turn")
	var commandReply *BattleCommandResponse
	for _, message := range messages.streams {
		if message.opcode == opcodes.PokeBattleEndNotify {
			t.Fatal("ordinary command published an uncorrelated end outcome")
		}
		if message.opcode == opcodes.PokeBattleActionResponse {
			var reply BattleCommandResponse
			if err := json.Unmarshal(message.payload, &reply); err != nil {
				t.Fatal(err)
			}
			commandReply = &reply
		}
	}
	if commandReply == nil || commandReply.RequestID != "test-turn" || commandReply.End == nil || !commandReply.End.Blackout || commandReply.End.BlackoutMapID != mapID || commandReply.Position.MapID != mapID || commandReply.Position.X != x || commandReply.Position.Y != y {
		t.Fatal("committed blackout is not one correlated outcome")
	}

	if int(ses.Client.CharData().MapID) != mapID || int(ses.Client.CharData().X) != x || int(ses.Client.CharData().Y) != y {
		t.Fatal("battle blackout omitted owned position publication")
	}
	if _, err := pokebattle.CommitBattle(context.Background(), database, 42, current, apply); err == nil {
		t.Fatal("replayed blackout")
	}
}
