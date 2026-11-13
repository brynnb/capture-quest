package world

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
)

func TestDuplicateNetworkBattleItemCannotSpendNextQuantity(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	original := battleTestStart(t, database, true, nil)
	instance, err := cqitems.NewStore(database).AddItemToInventory(42, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	request := fmt.Sprintf(`{"battle":{"battleId":%q,"revision":%d},"action":"item","itemId":1,"instanceId":%d,"targetSlot":0}`, original.BattleID, original.Revision, instance)
	battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, request)
	committed := getBattle(42)
	if committed.Revision != original.Revision+1 || committed.PlayerParty[0].CurHP != 21 {
		t.Fatal("first item command did not commit")
	}
	// Discard the successful response and repeat the exact packet after commit.
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, request)
	assertRejectedBattleCommand(t, messages)
	saved, err := pokebattle.ResumeBattle(context.Background(), database, 42)
	if err != nil || saved.Revision != committed.Revision || saved.PlayerParty[0].CurHP != 21 {
		t.Fatal("duplicate changed saved party/battle", err)
	}
	owned, err := cqitems.NewStore(database).FindInventoryItemByInstanceID(42, instance)
	if err != nil || owned.Instance.Quantity != 1 {
		t.Fatal("duplicate spent a second item", err)
	}
	// Reconnect restores current authority; the old wire identity still rejects.
	setBattle(42, saved)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, request)
	assertRejectedBattleCommand(t, messages)
}

func assertRejectedBattleCommand(t *testing.T, messages *recordingMessenger) {
	t.Helper()
	if len(messages.streams) != 1 {
		t.Fatalf("rejection published side effects: %+v", messages.streams)
	}
	var response struct {
		Success bool
		Error   string
	}
	if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success || response.Error == "" {
		t.Fatalf("expected explicit rejection: %+v %v", response, err)
	}
}

func TestBattleCommandIdentityIsMandatoryAcrossMutationOpcodes(t *testing.T) {
	_, wh, ses, messages := battleTestWorld(t)
	current := battleTestStart(t, wh.database, false, nil)
	registry := NewWorldOpCodeRegistry()
	registry.WH = wh
	for _, tc := range []struct {
		opcode opcodes.OpCode
		action string
	}{
		{opcodes.PokeBattleActionRequest, `"action":"fight","moveSlot":0`},
		{opcodes.PokeBattleSwitchRequest, `"action":"switch","partyIndex":0`},
		{opcodes.CQBattleItemUseRequest, `"itemId":1`},
	} {
		for _, identity := range []string{"", `"battle":{"battleId":"wrong","revision":1},`, fmt.Sprintf(`"battle":{"battleId":%q,"revision":0},`, current.BattleID), fmt.Sprintf(`"battle":{"battleId":%q,"revision":%d},`, current.BattleID, current.Revision+1), fmt.Sprintf(`"battle":{"battleId":%q,"revision":1},"unexpected":true,`, current.BattleID)} {
			messages.streams = nil
			registry.HandleWorldPacket(ses, clientPacket(tc.opcode, `{"requestId":"reject",`+identity+tc.action+"}"))
			assertRejectedBattleCommand(t, messages)
			if getBattle(42) != current {
				t.Fatal("invalid identity replaced authority")
			}
		}
	}
	next, err := pokebattle.CommitBattle(context.Background(), wh.database, 42, current, func(_ db.DBTX, b *pokebattle.BattleState) error {
		b.Phase = pokebattle.PhaseBattleEnd
		b.PendingMoveLearn = &pokebattle.PendingMove{PokemonIndex: 0, MoveID: 150, MoveName: "SPLASH"}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	setBattle(42, next)
	messages.streams = nil
	registry.HandleWorldPacket(ses, clientPacket(opcodes.PokeMoveLearnRequest, `{"requestId":"reject-learn","forgetSlot":-1}`))
	assertRejectedBattleCommand(t, messages)
	if getBattle(42).PendingMoveLearn == nil {
		t.Fatal("unbound command consumed pending move")
	}
}

func TestDelayedBattleCloseCannotDeleteReplacementFinishedBattle(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	original := battleTestStart(t, database, false, nil)
	finish := func(current *pokebattle.BattleState) *pokebattle.BattleState {
		t.Helper()
		next, err := pokebattle.CommitBattle(context.Background(), database, 42, current, func(_ db.DBTX, b *pokebattle.BattleState) error { b.Phase = pokebattle.PhaseBattleEnd; return nil })
		if err != nil {
			t.Fatal(err)
		}
		setBattle(42, next)
		return next
	}
	original = finish(original)
	oldClose := fmt.Sprintf(`{"battle":{"battleId":%q,"revision":%d}}`, original.BattleID, original.Revision)
	battleDispatch(t, wh, ses, opcodes.PokeBattleCloseRequest, oldClose)
	replacement := finish(battleTestStart(t, database, false, nil))
	battleDispatch(t, wh, ses, opcodes.PokeBattleCloseRequest, oldClose)
	saved, err := pokebattle.LoadBattleState(database, 42)
	if err != nil || saved == nil || saved.BattleID != replacement.BattleID || getBattle(42) != replacement {
		t.Fatal("delayed close deleted replacement battle", err)
	}
}

func TestBattleCommandsRequireCorrelationAndReturnItOnEveryOutcome(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	current := battleTestStart(t, database, false, nil)
	registry := NewWorldOpCodeRegistry()
	registry.WH = wh
	for _, requestID := range []string{"", strings.Repeat("x", 65)} {
		request := fmt.Sprintf(`{"requestId":%q,"battle":{"battleId":%q,"revision":1},"action":"fight","moveSlot":0}`, requestID, current.BattleID)
		messages.streams = nil
		registry.HandleWorldPacket(ses, clientPacket(opcodes.PokeBattleActionRequest, request))
		assertRejectedBattleCommand(t, messages)
		var reply BattleCommandError
		if err := json.Unmarshal(messages.streams[0].payload, &reply); err != nil || reply.RequestID != requestID || getBattle(42) != current {
			t.Fatal("invalid correlation changed authority or lost its ID", err)
		}
	}
	// The alias must return its own opcode and correlation, rather than silently
	// forwarding a response through opcode 71 that its caller cannot settle.
	request := fmt.Sprintf(`{"requestId":"alias","battle":{"battleId":%q,"revision":1},"itemId":1}`, current.BattleID)
	messages.streams = nil
	registry.HandleWorldPacket(ses, clientPacket(opcodes.CQBattleItemUseRequest, request))
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.CQBattleItemUseResponse {
		t.Fatal("item alias replied through the wrong boundary")
	}
	var noTurn BattleCommandResponse
	if err := json.Unmarshal(messages.streams[0].payload, &noTurn); err != nil || !noTurn.Success || noTurn.RequestID != "alias" || noTurn.Battle == nil || noTurn.Battle.Revision != 1 || len(noTurn.Events) != 1 {
		t.Fatal("rule rejection lost phase/correlation", err)
	}
}
