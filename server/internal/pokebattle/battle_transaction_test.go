package pokebattle

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func TestBattleCommitRollsBackAndPublishesPrivateSnapshot(t *testing.T) {
	database := partyDatabase(t)
	initial := NewWildBattle(mustLoadParty(t, database, 42), &Pokemon{ID: 7, Name: "SQUIRTLE", Level: 5, CurHP: 20, MaxHP: 20})
	current, err := StartBattle(context.Background(), database, 42, initial, nil)
	if err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `INSERT INTO character_wallet(character_id,pokedollars) VALUES(42,100);
 ALTER TABLE character_battle_state ADD CONSTRAINT reject_turn CHECK((battle_json::json->>'turnNumber')::int <> 9);`)
	hp := current.PlayerParty[0].CurHP
	apply := func(tx db.DBTX, next *BattleState) error {
		next.PlayerParty[0].CurHP = hp - 1
		next.PlayerParty[0].AtkStage = 2
		next.EnemyParty[0].CurHP = 3
		next.TurnNumber = 9
		_, err := tx.Exec(`UPDATE character_wallet SET pokedollars=pokedollars-10 WHERE character_id=42`)
		return err
	}
	failed, err := CommitBattle(context.Background(), database, 42, current, apply)
	if err == nil || failed != nil {
		t.Fatalf("failure=%v state=%v", err, failed)
	}
	if current.PlayerParty[0].CurHP != hp || current.EnemyParty[0].CurHP != 20 || current.Revision != 1 {
		t.Fatal("mutated published battle on failed commit")
	}
	saved, err := ResumeBattle(context.Background(), database, 42)
	if err != nil {
		t.Fatal(err)
	}
	if saved.PlayerParty[0].CurHP != hp || saved.EnemyParty[0].CurHP != 20 || saved.Revision != 1 {
		t.Fatal("partial battle/party persisted")
	}
	var money int
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 100 {
		t.Fatalf("money=%d error=%v", money, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_battle_state DROP CONSTRAINT reject_turn`)
	next, err := CommitBattle(context.Background(), database, 42, current, apply)
	if err != nil {
		t.Fatal(err)
	}
	if next.Revision != 2 || next.BattleID != current.BattleID || next.PlayerParty[0].CurHP != hp-1 || current.PlayerParty[0].CurHP != hp {
		t.Fatal("commit did not return independent new snapshot")
	}
	saved, err = ResumeBattle(context.Background(), database, 42)
	if err != nil || saved.PlayerParty[0].AtkStage != 2 {
		t.Fatalf("restore=%+v error=%v", saved, err)
	}
}

func TestBattleConcurrentStaleRevisionsApplyOnlyOnce(t *testing.T) {
	database := partyDatabase(t)
	current, err := StartBattle(context.Background(), database, 42, NewWildBattle(nil, &Pokemon{ID: 7, CurHP: 20, MaxHP: 20}), nil)
	if err != nil {
		t.Fatal(err)
	}
	var commits, conflicts, applied atomic.Int32
	gate := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			_, err := CommitBattle(context.Background(), database, 42, current, func(tx db.DBTX, next *BattleState) error { applied.Add(1); next.TurnNumber++; return nil })
			if err == nil {
				commits.Add(1)
			} else if errors.Is(err, ErrBattleConflict) {
				conflicts.Add(1)
			} else {
				t.Errorf("commit: %v", err)
			}
		}()
	}
	close(gate)
	wg.Wait()
	if commits.Load() != 1 || conflicts.Load() != 3 || applied.Load() != 1 {
		t.Fatalf("commits=%d conflicts=%d effects=%d", commits.Load(), conflicts.Load(), applied.Load())
	}
	if _, err := StartBattle(context.Background(), database, 42, current, nil); !errors.Is(err, ErrBattleConflict) {
		t.Fatalf("replaced unfinished battle: %v", err)
	}
	if err := CloseBattle(context.Background(), database, 42, current); !errors.Is(err, ErrBattleConflict) {
		t.Fatalf("closed active battle: %v", err)
	}
}

func TestBattleResumePreservesVolatileStateAndPendingChoice(t *testing.T) {
	database := partyDatabase(t)
	enemy := &Pokemon{ID: 7, Name: "SQUIRTLE", Level: 5, CurHP: 20, MaxHP: 20, CrySFX: "cry", CryPitch: 4, CryLength: 9, EvolveLevel: 16, EvolvePokemonName: "WARTORTLE"}
	enemy.Moves[0] = MoveSlot{ID: 1, BattleSFX: "hit", SFXPitch: 7, SFXTempo: 8}
	current, err := StartBattle(context.Background(), database, 42, NewWildBattle(nil, enemy), nil)
	if err != nil {
		t.Fatal(err)
	}
	current, err = CommitBattle(context.Background(), database, 42, current, func(_ db.DBTX, next *BattleState) error {
		p := next.PlayerParty[0]
		p.Status = StatusSleep
		p.SleepTurns = 3
		p.ConfusionTurns = 2
		p.BadPoisonTurns = 4
		p.IsSeeded = true
		p.SubstituteHP = 6
		p.DireHit = true
		p.GuardSpec = true
		p.AtkStage = 2
		p.DefStage = -1
		p.SpcStage = 1
		p.SpdStage = -2
		p.AccStage = 3
		p.EvaStage = -3
		next.Phase = PhaseBattleEnd
		next.PendingMoveLearn = &PendingMove{PokemonIndex: 0, MoveID: 9, MoveName: "Choice"}
		next.PostMoveLearnEvents = []BattleEvent{{Type: EventMessage, Message: "reward"}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := ResumeBattle(context.Background(), database, 42)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(volatileState(current.PlayerParty[0]), volatileState(restored.PlayerParty[0])) || restored.PlayerParty[0].Status != StatusSleep {
		t.Fatal("resume changed volatile state")
	}
	if !reflect.DeepEqual(current.EnemyParty, restored.EnemyParty) || !reflect.DeepEqual(current.PendingMoveLearn, restored.PendingMoveLearn) || !reflect.DeepEqual(current.PostMoveLearnEvents, restored.PostMoveLearnEvents) {
		t.Fatal("resume changed enemy metadata or pending choice")
	}
	if err := CloseBattle(context.Background(), database, 42, restored); !errors.Is(err, ErrBattleConflict) {
		t.Fatalf("discarded pending choice: %v", err)
	}
	finished, err := CommitBattle(context.Background(), database, 42, restored, func(_ db.DBTX, next *BattleState) error { next.PendingMoveLearn = nil; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := CloseBattle(context.Background(), database, 42, finished); err != nil {
		t.Fatal(err)
	}
	if _, err := CommitBattle(context.Background(), database, 42, finished, nil); !errors.Is(err, ErrBattleConflict) {
		t.Fatalf("stale disconnect recreated closed battle: %v", err)
	}
	if saved, err := LoadBattleState(database, 42); err != nil || saved != nil {
		t.Fatalf("close saved=%v error=%v", saved, err)
	}
}

func TestBattleResumeRejectsMismatchedPartyWithoutDeletingRecord(t *testing.T) {
	database := partyDatabase(t)
	current, err := StartBattle(context.Background(), database, 42, NewWildBattle(nil, &Pokemon{ID: 7, CurHP: 20}), nil)
	if err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `UPDATE character_battle_state SET battle_json=jsonb_set(battle_json::jsonb,'{playerVolatile,0,rowId}','999999')::text WHERE character_id=42`)
	if _, err := ResumeBattle(context.Background(), database, 42); err == nil {
		t.Fatal("accepted wrong player identity")
	}
	saved, err := LoadBattleState(database, 42)
	if err != nil || saved == nil || saved.BattleID != current.BattleID {
		t.Fatal("deleted unrecoverable record")
	}
	testdb.Exec(t, database, `UPDATE character_battle_state SET battle_json='{"version":999}' WHERE character_id=42`)
	if _, err := ResumeBattle(context.Background(), database, 42); err == nil {
		t.Fatal("accepted unsupported version")
	}
}

func TestLegacyBattleUpgradesOnlyOnCommittedAction(t *testing.T) {
	database := partyDatabase(t)
	legacy := `{"phase":1,"battleType":0,"playerActive":0,"enemyActive":0,"enemyParty":[{"id":7,"name":"SQUIRTLE","curHp":20,"maxHp":20,"level":5}]}`
	if _, err := database.Exec(`INSERT INTO character_battle_state(character_id,battle_json) VALUES(42,$1)`, legacy); err != nil {
		t.Fatal(err)
	}
	restored, err := ResumeBattle(context.Background(), database, 42)
	if err != nil || restored == nil || restored.persistedVersion != 0 {
		t.Fatalf("legacy resume: %+v %v", restored, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CommitBattle(ctx, database, 42, restored, nil); err == nil {
		t.Fatal("accepted cancelled action")
	}
	var raw string
	if err := database.QueryRow(`SELECT battle_json FROM character_battle_state WHERE character_id=42`).Scan(&raw); err != nil || raw != legacy {
		t.Fatalf("failed action rewrote legacy save: %v", err)
	}
	committed, err := CommitBattle(context.Background(), database, 42, restored, nil)
	if err != nil || committed.BattleID == "" || committed.Revision != 1 {
		t.Fatalf("upgrade: %+v %v", committed, err)
	}
	saved, err := ResumeBattle(context.Background(), database, 42)
	if err != nil || saved.persistedVersion != 2 {
		t.Fatalf("new version restore: %+v %v", saved, err)
	}
}
