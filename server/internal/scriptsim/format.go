package scriptsim

import (
	"fmt"
	"strings"
)

func FormatResult(r *Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Scenario: %s\n", r.Scenario.Name)
	if r.Scenario.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", r.Scenario.Description)
	}
	fmt.Fprintf(&b, "Character: %s\n\n", r.Initial.CharacterName)

	writeSnapshot(&b, "Initial state", r.Initial)
	fmt.Fprintf(&b, "\nResolved script\n")
	fmt.Fprintf(&b, "  label: %s\n", r.Script.ScriptLabel)
	fmt.Fprintf(&b, "  map: %s\n", r.Script.MapName)
	fmt.Fprintf(&b, "  trigger: %s\n", r.Script.TriggerType)

	fmt.Fprintf(&b, "\nActions\n")
	for _, effect := range r.ActionEffects {
		changed := ""
		if effect.Changed {
			changed = " *"
		}
		if effect.Detail != "" {
			fmt.Fprintf(&b, "  - %s%s: %s\n", effect.Type, changed, effect.Detail)
		} else {
			fmt.Fprintf(&b, "  - %s%s\n", effect.Type, changed)
		}
	}
	writeRecordedMessages(&b, r.Messages)

	fmt.Fprintln(&b)
	writeSnapshot(&b, "Final state", r.Final)
	writeElevatorSummary(&b, r.Elevator)
	writeGameCornerSummary(&b, r.GameCorner)
	writeFieldMoveSummary(&b, r.FieldMove)
	writeBoulderPushSummary(&b, r.BoulderPush)
	writePathfindSummary(&b, r.Pathfind)
	writeTileStates(&b, r.TileStates)
	writeObjectStates(&b, r.ObjectStates)
	writeSafariSummary(&b, r.Safari)
	writeRepelSummary(&b, r.Repel)
	return b.String()
}

func writeRecordedMessages(b *strings.Builder, messages []RecordedMessage) {
	if len(messages) == 0 {
		return
	}
	fmt.Fprintf(b, "\nRecorded messages\n")
	for _, message := range messages {
		fmt.Fprintf(b, "  - %s opcode=%d payload=%s\n", message.Channel, message.Opcode, string(message.Payload))
	}
}

func writeSnapshot(b *strings.Builder, title string, s *Snapshot) {
	fmt.Fprintf(b, "%s\n", title)
	fmt.Fprintf(b, "  location: %s[%d] (%d,%d)\n", s.MapName, s.MapID, s.X, s.Y)
	fmt.Fprintf(b, "  flags: %s\n", joinedOrNone(s.Flags))
	if s.Money != 0 {
		fmt.Fprintf(b, "  money: %d\n", s.Money)
	}
	if s.Coins != 0 {
		fmt.Fprintf(b, "  coins: %d\n", s.Coins)
	}
	if s.DayCare.Active {
		fmt.Fprintf(b, "  day care: #%d %s L%d start=%d grown=%d cost=%d exp=%d\n",
			s.DayCare.PokemonID,
			s.DayCare.Name,
			s.DayCare.Level,
			s.DayCare.StartLevel,
			s.DayCare.LevelsGrown,
			s.DayCare.Cost,
			s.DayCare.Exp)
	}
	if s.VermilionGymTrash != nil {
		second := "none"
		if s.VermilionGymTrash.SecondLockCanIndex != nil {
			second = fmt.Sprintf("%d", *s.VermilionGymTrash.SecondLockCanIndex)
		}
		fmt.Fprintf(b, "  vermilion gym trash: first=%d second=%s\n",
			s.VermilionGymTrash.FirstLockCanIndex, second)
	}
	fmt.Fprintf(b, "  party:\n")
	if len(s.Party) == 0 {
		fmt.Fprintf(b, "    none\n")
	} else {
		for _, p := range s.Party {
			details := pokemonSummaryDetails(p)
			if details != "" {
				details = " " + details
			}
			fmt.Fprintf(b, "    slot %d: #%d %s L%d moves %s%s\n",
				p.Slot, p.SpeciesID, p.Name, p.Level, joinedOrNone(p.Moves), details)
		}
	}
	if len(s.PC) > 0 {
		fmt.Fprintf(b, "  pc:\n")
		for _, p := range s.PC {
			fmt.Fprintf(b, "    box %d slot %d: #%d %s L%d\n",
				p.Box, p.Slot, p.SpeciesID, p.Name, p.Level)
		}
	}
	fmt.Fprintf(b, "  inventory:\n")
	if len(s.Inventory) == 0 {
		fmt.Fprintf(b, "    none\n")
	} else {
		for _, item := range s.Inventory {
			fmt.Fprintf(b, "    #%d %s x%d\n", item.ItemID, item.Name, item.Quantity)
		}
	}
	fmt.Fprintf(b, "  hidden objects:\n")
	if len(s.HiddenObjects) == 0 {
		fmt.Fprintf(b, "    none\n")
	} else {
		for _, obj := range s.HiddenObjects {
			label := obj.Text
			if label == "" {
				label = obj.Name
			}
			if label == "" {
				label = obj.Key
			}
			fmt.Fprintf(b, "    %s\n", label)
		}
	}
	fmt.Fprintf(b, "  active battle:\n")
	if s.ActiveBattle == nil {
		fmt.Fprintf(b, "    none\n")
		return
	}
	fmt.Fprintf(b, "    %s phase=%s trainer=%s name=%s winFlag=%s\n",
		s.ActiveBattle.BattleType,
		s.ActiveBattle.Phase,
		s.ActiveBattle.TrainerClass,
		s.ActiveBattle.TrainerName,
		s.ActiveBattle.WinFlag)
	if len(s.ActiveBattle.AllowedActions) > 0 {
		fmt.Fprintf(b, "    allowed actions: %s\n", strings.Join(s.ActiveBattle.AllowedActions, ", "))
	}
	if s.ActiveBattle.GuaranteedCatch {
		fmt.Fprintf(b, "    guaranteed catch: true\n")
	}
	for _, pokemon := range s.ActiveBattle.EnemyParty {
		fmt.Fprintf(b, "    enemy slot %d: #%d %s L%d\n", pokemon.Slot, pokemon.SpeciesID, pokemon.Name, pokemon.Level)
	}
}

func pokemonSummaryDetails(p PokemonSummary) string {
	details := []string{}
	if p.MaxHP > 0 && p.CurHP != p.MaxHP {
		details = append(details, fmt.Sprintf("hp=%d/%d", p.CurHP, p.MaxHP))
	}
	if p.Status != "" {
		details = append(details, fmt.Sprintf("status=%s", p.Status))
	}
	if p.Exp != p.ExpectedExp {
		details = append(details, fmt.Sprintf("exp=%d", p.Exp))
	}
	return strings.Join(details, " ")
}

func writeSafariSummary(b *strings.Builder, safari *SafariSummary) {
	if safari == nil {
		return
	}
	fmt.Fprintf(b, "\nSafari session\n")
	if !safari.Active {
		fmt.Fprintf(b, "  inactive\n")
		return
	}
	fmt.Fprintf(b, "  active: balls=%d steps=%d\n", safari.BallsLeft, safari.StepsLeft)
	if safari.Battle != nil {
		fmt.Fprintf(b, "  battle: #%d %s L%d caught=%t fled=%t over=%t\n",
			safari.Battle.PokemonID,
			safari.Battle.Name,
			safari.Battle.Level,
			safari.Battle.Caught,
			safari.Battle.Fled,
			safari.Battle.IsOver)
	}
}

func writeRepelSummary(b *strings.Builder, repel *RepelSummary) {
	if repel == nil {
		return
	}
	fmt.Fprintf(b, "\nRepel\n")
	fmt.Fprintf(b, "  initial: active=%t steps=%d\n", repel.InitialActive, repel.InitialStepsLeft)
	fmt.Fprintf(b, "  final: active=%t steps=%d\n", repel.Active, repel.StepsLeft)
	if repel.ItemID != 0 {
		fmt.Fprintf(b, "  item: %d\n", repel.ItemID)
	}
	fmt.Fprintf(b, "  success: %t\n", repel.Success)
	if repel.Message != "" {
		fmt.Fprintf(b, "  message: %s\n", repel.Message)
	}
	if repel.WoreOff {
		fmt.Fprintf(b, "  wore off: true\n")
	}
}

func writeElevatorSummary(b *strings.Builder, elevator *ElevatorSummary) {
	if elevator == nil {
		return
	}
	fmt.Fprintf(b, "\nElevator\n")
	if elevator.Message != "" {
		fmt.Fprintf(b, "  message: %s\n", elevator.Message)
	}
	if elevator.Selected != nil {
		floor := elevator.Selected
		fmt.Fprintf(b, "  selected: %s %s[%d] (%d,%d)\n",
			floor.Label, floor.MapName, floor.MapID, floor.DestX, floor.DestY)
	}
	if len(elevator.Floors) == 0 {
		if elevator.Selected == nil {
			fmt.Fprintf(b, "  floors: none\n")
		}
		return
	}
	fmt.Fprintf(b, "  floors:\n")
	for _, floor := range elevator.Floors {
		req := ""
		if floor.RequiresItemID > 0 {
			req = fmt.Sprintf(" requiresItem=%d", floor.RequiresItemID)
		}
		if floor.RequiresFlag != "" {
			req = fmt.Sprintf("%s requiresFlag=%s", req, floor.RequiresFlag)
		}
		fmt.Fprintf(b, "    %s -> %s[%d] (%d,%d)%s\n",
			floor.Label, floor.MapName, floor.MapID, floor.DestX, floor.DestY, req)
	}
}

func writeGameCornerSummary(b *strings.Builder, gameCorner *GameCornerSummary) {
	if gameCorner == nil {
		return
	}
	fmt.Fprintf(b, "\nGame Corner\n")
	if gameCorner.SlotPlay != nil {
		slot := gameCorner.SlotPlay
		fmt.Fprintf(b, "  slot play: success=%t bet=%d payout=%d coins=%d positions=%v\n",
			slot.Success, slot.Bet, slot.Payout, slot.Coins, slot.ReelPositions)
		if slot.Message != "" {
			fmt.Fprintf(b, "  message: %s\n", slot.Message)
		}
		if slot.MatchLine != "" {
			fmt.Fprintf(b, "  match: %s %s\n", slot.MatchLine, slot.MatchSymbol)
		}
		if len(slot.Reels) > 0 {
			fmt.Fprintf(b, "  reels: %v\n", slot.Reels)
		}
	}
	if gameCorner.PrizeList != nil {
		list := gameCorner.PrizeList
		fmt.Fprintf(b, "  prize list: success=%t coins=%d\n", list.Success, list.Coins)
		if list.Message != "" {
			fmt.Fprintf(b, "  message: %s\n", list.Message)
		}
		if len(list.Prizes) == 0 {
			fmt.Fprintf(b, "  prizes: none\n")
		} else {
			fmt.Fprintf(b, "  prizes:\n")
			for _, prize := range list.Prizes {
				writeGameCornerPrize(b, "    ", prize)
			}
		}
	}
	if gameCorner.PrizePurchase != nil {
		purchase := gameCorner.PrizePurchase
		fmt.Fprintf(b, "  purchase: success=%t coins=%d message=%s\n",
			purchase.Success, purchase.Coins, purchase.Message)
		if purchase.Prize.Name != "" {
			writeGameCornerPrize(b, "  prize: ", purchase.Prize)
		}
		if purchase.PrizeLevel > 0 {
			location := "PC"
			if purchase.AddedToParty {
				location = "party"
			}
			fmt.Fprintf(b, "  pokemon: level=%d location=%s", purchase.PrizeLevel, location)
			if !purchase.AddedToParty {
				fmt.Fprintf(b, " box=%d slot=%d", purchase.PCBox, purchase.PCSlot)
			}
			fmt.Fprintf(b, "\n")
		}
	}
	if gameCorner.HiddenCoin != nil {
		coin := gameCorner.HiddenCoin
		fmt.Fprintf(b, "  hidden coin: success=%t coinId=%d amount=%d coins=%d attempts=%d\n",
			coin.Success, coin.CoinID, coin.Amount, coin.Coins, coin.Attempts)
		if coin.Message != "" {
			fmt.Fprintf(b, "  message: %s\n", coin.Message)
		}
		if coin.AlreadyFound {
			fmt.Fprintf(b, "  already found: true\n")
		}
	}
}

func writeGameCornerPrize(b *strings.Builder, prefix string, prize GameCornerPrizeSummary) {
	extra := ""
	if prize.PokemonID != 0 {
		extra = fmt.Sprintf(" pokemon=%d", prize.PokemonID)
	}
	if prize.ItemID != 0 {
		extra = fmt.Sprintf(" item=%d", prize.ItemID)
	}
	fmt.Fprintf(b, "%s%s %s cost=%d%s\n", prefix, prize.Type, prize.Name, prize.CoinCost, extra)
}

func writeFieldMoveSummary(b *strings.Builder, fieldMove *FieldMoveSummary) {
	if fieldMove == nil {
		return
	}
	fmt.Fprintf(b, "\nField move\n")
	fmt.Fprintf(b, "  move: %s[%d] allowed=%t\n", fieldMove.MoveName, fieldMove.MoveID, fieldMove.Allowed)
	if fieldMove.RequiredBadgeFlag != "" {
		fmt.Fprintf(b, "  required badge: %s\n", fieldMove.RequiredBadgeFlag)
	}
	if fieldMove.KnownBySpeciesID != 0 {
		fmt.Fprintf(b, "  known by: #%d %s\n", fieldMove.KnownBySpeciesID, fieldMove.KnownByName)
	}
	if fieldMove.Message != "" {
		fmt.Fprintf(b, "  message: %s\n", fieldMove.Message)
	}
}

func writeBoulderPushSummary(b *strings.Builder, boulderPush *BoulderPushSummary) {
	if boulderPush == nil {
		return
	}
	fmt.Fprintf(b, "\nBoulder push\n")
	fmt.Fprintf(b, "  result: success=%t direction=%s object=%s\n",
		boulderPush.Success, boulderPush.Direction, boulderPush.ObjectName)
	if boulderPush.FromX != 0 || boulderPush.FromY != 0 || boulderPush.ToX != 0 || boulderPush.ToY != 0 {
		fmt.Fprintf(b, "  movement: (%d,%d) -> (%d,%d)\n",
			boulderPush.FromX, boulderPush.FromY, boulderPush.ToX, boulderPush.ToY)
	}
	if boulderPush.Dropped {
		fmt.Fprintf(b, "  dropped: true\n")
	}
	if boulderPush.FlagSet != "" {
		fmt.Fprintf(b, "  flag: %s\n", boulderPush.FlagSet)
	}
	if boulderPush.Message != "" {
		fmt.Fprintf(b, "  message: %s\n", boulderPush.Message)
	}
	if boulderPush.StrengthUsed != nil {
		fmt.Fprintf(b, "  strength: %s\n", boulderPush.StrengthUsed.Message)
	}
	if len(boulderPush.Boulders) == 0 {
		return
	}
	fmt.Fprintf(b, "  boulders:\n")
	for _, boulder := range boulderPush.Boulders {
		visibility := "hidden"
		if boulder.Visible {
			visibility = "visible"
		}
		fmt.Fprintf(b, "    %s (%d,%d): %s\n",
			boulder.Name, boulder.X, boulder.Y, visibility)
	}
}

func writePathfindSummary(b *strings.Builder, pathfind *PathfindSummary) {
	if pathfind == nil {
		return
	}
	fmt.Fprintf(b, "\nPathfind\n")
	fmt.Fprintf(b, "  result: found=%t length=%d\n", pathfind.Found, pathfind.Length)
	if len(pathfind.Path) == 0 {
		return
	}
	fmt.Fprintf(b, "  path:")
	for _, node := range pathfind.Path {
		fmt.Fprintf(b, " (%d,%d)", node.X, node.Y)
	}
	fmt.Fprintf(b, "\n")
}

func writeTileStates(b *strings.Builder, states []TileState) {
	if len(states) == 0 {
		return
	}
	fmt.Fprintf(b, "\nEvent tile states\n")
	for _, state := range states {
		label := state.Label
		if label == "" {
			label = "event tile"
		}
		fmt.Fprintf(b, "  %s (%d,%d): tile=%d collision=%d\n",
			label, state.X, state.Y, state.TileImageID, state.CollisionType)
	}
}

func writeObjectStates(b *strings.Builder, states []ObjectState) {
	if len(states) == 0 {
		return
	}
	fmt.Fprintf(b, "\nEvent object states\n")
	for _, state := range states {
		label := state.Label
		if label == "" {
			label = state.Name
		}
		visibility := "hidden"
		if state.Visible {
			visibility = "visible"
		}
		text := ""
		if state.Text != "" {
			text = fmt.Sprintf(" text=%s", state.Text)
		}
		fmt.Fprintf(b, "  %s %s/%s (%d,%d): %s%s\n",
			label, state.MapName, state.Name, state.X, state.Y, visibility, text)
	}
}

func joinedOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}
