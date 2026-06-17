package world

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

const route18Gate2FYoungsterTextConstant = "TEXT_ROUTE18GATE2F_YOUNGSTER"

var errInGameTradeNotFound = errors.New("in-game trade not found")

type inGameTradeDefinition struct {
	TradeKey             string
	TextConstant         string
	MapName              string
	SourceFile           string
	ScriptLabel          string
	RequestedPokemonID   int
	RequestedPokemonName string
	OfferedPokemonID     int
	OfferedPokemonName   string
	OfferedNickname      string
	DialogueSet          string
	OriginalTradeIndex   int
}

type inGameTradeOutcome struct {
	Dialogue         string
	Traded           bool
	AlreadyCompleted bool
	WrongPokemon     bool
}

func resolveInGameTradeDialogueEntries(textConstant string, charID int64) ([]PhaserDialogueEntry, bool) {
	trade, err := loadInGameTradeDefinitionByText(textConstant)
	if err != nil {
		return nil, false
	}

	completed := false
	if charID > 0 {
		if done, err := characterCompletedInGameTrade(db.GlobalWorldDB.DB, charID, trade.TradeKey); err == nil {
			completed = done
		}
	}

	dialogue := trade.offerDialogue()
	if completed {
		dialogue = trade.afterTradeDialogue()
	}

	return []PhaserDialogueEntry{{
		Label:      trade.ScriptLabel,
		SourceFile: trade.SourceFile,
		Dialogue:   dialogue,
		IsTrainer:  0,
	}}, true
}

func checkInGameTradeBranchingDialogue(textConstant string, charID int64) *BranchingDialogue {
	if charID == 0 {
		return nil
	}
	trade, err := loadInGameTradeDefinitionByText(textConstant)
	if err != nil {
		return nil
	}
	completed, err := characterCompletedInGameTrade(db.GlobalWorldDB.DB, charID, trade.TradeKey)
	if err != nil || completed {
		return nil
	}
	return &BranchingDialogue{
		PromptTextConstant: textConstant,
		PromptText:         trade.promptDialogue(),
	}
}

func handleInGameTradeDialogueChoice(ses *session.Session, req DialogueChoiceRequest) bool {
	trade, err := loadInGameTradeDefinitionByText(req.TextConstant)
	if err != nil {
		return false
	}
	if !ses.HasValidClient() {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "not logged in",
		}, opcodes.DialogueChoiceResponse)
		return true
	}

	charID := int64(ses.Client.CharData().ID)
	outcome := inGameTradeOutcome{Dialogue: trade.noTradeDialogue()}
	if req.Choice {
		var tradeErr error
		outcome, tradeErr = performInGameTrade(db.GlobalWorldDB.DB, charID, trade)
		if tradeErr != nil {
			log.Printf("[InGameTrade] Failed to complete %s for char %d: %v", trade.TradeKey, charID, tradeErr)
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   "trade failed",
			}, opcodes.DialogueChoiceResponse)
			SendSystemMessage(ses, "That trade could not be completed. Please try again.")
			return true
		}
	}

	ses.SendStreamJSON(map[string]interface{}{
		"success":              true,
		"choice":               req.Choice,
		"followUpDialogue":     outcome.Dialogue,
		"followUpTextConstant": "",
		"tradeCompleted":       outcome.Traded,
		"alreadyCompleted":     outcome.AlreadyCompleted,
		"wrongPokemon":         outcome.WrongPokemon,
	}, opcodes.DialogueChoiceResponse)

	if outcome.Traded {
		sendPartyUpdate(ses)
	}

	log.Printf("[InGameTrade] Dialogue choice %s choice=%t traded=%t wrongPokemon=%t",
		trade.TradeKey, req.Choice, outcome.Traded, outcome.WrongPokemon)
	return true
}

func loadInGameTradeDefinitionByText(textConstant string) (inGameTradeDefinition, error) {
	if db.GlobalWorldDB == nil || db.GlobalWorldDB.DB == nil {
		return inGameTradeDefinition{}, errInGameTradeNotFound
	}
	return queryInGameTradeDefinitionByText(db.GlobalWorldDB.DB, textConstant)
}

func queryInGameTradeDefinitionByText(myDB pokebattle.DBTX, textConstant string) (inGameTradeDefinition, error) {
	var trade inGameTradeDefinition
	err := myDB.QueryRow(`
		SELECT trade_key, text_constant, map_name, source_file, script_label,
		       requested_pokemon_id, requested_pokemon_name,
		       offered_pokemon_id, offered_pokemon_name, offered_nickname,
		       dialogue_set, COALESCE(original_trade_index, -1)
		FROM phaser_in_game_trades
		WHERE text_constant = $1`, textConstant).Scan(
		&trade.TradeKey,
		&trade.TextConstant,
		&trade.MapName,
		&trade.SourceFile,
		&trade.ScriptLabel,
		&trade.RequestedPokemonID,
		&trade.RequestedPokemonName,
		&trade.OfferedPokemonID,
		&trade.OfferedPokemonName,
		&trade.OfferedNickname,
		&trade.DialogueSet,
		&trade.OriginalTradeIndex,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return inGameTradeDefinition{}, errInGameTradeNotFound
	}
	if err != nil {
		return inGameTradeDefinition{}, err
	}
	return trade, nil
}

func characterCompletedInGameTrade(myDB pokebattle.DBTX, charID int64, tradeKey string) (bool, error) {
	var completed bool
	err := myDB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM character_in_game_trades
			WHERE character_id = $1 AND trade_key = $2
		)`, charID, tradeKey).Scan(&completed)
	return completed, err
}

func performInGameTrade(myDB *sql.DB, charID int64, trade inGameTradeDefinition) (inGameTradeOutcome, error) {
	if charID <= 0 {
		return inGameTradeOutcome{}, fmt.Errorf("character id is required")
	}

	tx, err := myDB.Begin()
	if err != nil {
		return inGameTradeOutcome{}, fmt.Errorf("begin trade transaction: %w", err)
	}
	defer tx.Rollback()

	completed, err := characterCompletedInGameTrade(tx, charID, trade.TradeKey)
	if err != nil {
		return inGameTradeOutcome{}, fmt.Errorf("check trade completion: %w", err)
	}
	if completed {
		return inGameTradeOutcome{Dialogue: trade.afterTradeDialogue(), AlreadyCompleted: true}, nil
	}

	var selectedRowID int64
	var selectedLevel int
	err = tx.QueryRow(`
		SELECT id, level
		FROM character_pokemon
		WHERE character_id = $1 AND box = $2 AND pokemon_id = $3
		ORDER BY party_slot ASC
		LIMIT 1`, charID, pokebattle.BoxParty, trade.RequestedPokemonID).Scan(&selectedRowID, &selectedLevel)
	if errors.Is(err, sql.ErrNoRows) {
		return inGameTradeOutcome{Dialogue: trade.wrongPokemonDialogue(), WrongPokemon: true}, nil
	}
	if err != nil {
		return inGameTradeOutcome{}, fmt.Errorf("find requested pokemon: %w", err)
	}

	if _, err := tx.Exec(`
		DELETE FROM character_pokemon
		WHERE id = $1 AND character_id = $2`, selectedRowID, charID); err != nil {
		return inGameTradeOutcome{}, fmt.Errorf("remove traded pokemon: %w", err)
	}
	pokebattle.CompactPartySlots(tx, charID)

	var newPartySlot int
	if err := tx.QueryRow(`
		SELECT COUNT(*)
		FROM character_pokemon
		WHERE character_id = $1 AND box = $2`, charID, pokebattle.BoxParty).Scan(&newPartySlot); err != nil {
		return inGameTradeOutcome{}, fmt.Errorf("count party slots: %w", err)
	}

	received, err := pokebattle.BuildWildPokemon(tx, trade.OfferedPokemonID, selectedLevel)
	if err != nil {
		return inGameTradeOutcome{}, fmt.Errorf("build offered pokemon: %w", err)
	}
	received.IsWild = false
	received.OriginalTrainerID = trade.originalTrainerID()

	addedToParty, _, _, err := pokebattle.SavePreparedPokemonToPartyOrPC(tx, charID, received)
	if err != nil {
		return inGameTradeOutcome{}, fmt.Errorf("save offered pokemon: %w", err)
	}
	if !addedToParty {
		return inGameTradeOutcome{}, fmt.Errorf("offered pokemon was sent to PC after removing a party pokemon")
	}

	if _, err := tx.Exec(`
		UPDATE character_pokemon
		SET nickname = $1, original_trainer_id = $2
		WHERE character_id = $3 AND box = $4 AND party_slot = $5`,
		trade.OfferedNickname, trade.originalTrainerID(), charID, pokebattle.BoxParty, newPartySlot); err != nil {
		return inGameTradeOutcome{}, fmt.Errorf("set offered pokemon nickname: %w", err)
	}

	if err := markPokemonCaughtInTx(tx, charID, trade.OfferedPokemonID); err != nil {
		return inGameTradeOutcome{}, fmt.Errorf("mark offered pokemon caught: %w", err)
	}

	if _, err := tx.Exec(`
		INSERT INTO character_in_game_trades (
			character_id, trade_key, given_pokemon_id, received_pokemon_id, received_nickname
		)
		VALUES ($1, $2, $3, $4, $5)`,
		charID, trade.TradeKey, trade.RequestedPokemonID, trade.OfferedPokemonID, trade.OfferedNickname); err != nil {
		return inGameTradeOutcome{}, fmt.Errorf("record completed trade: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return inGameTradeOutcome{}, fmt.Errorf("commit trade transaction: %w", err)
	}

	return inGameTradeOutcome{Dialogue: trade.completedTradeDialogue(), Traded: true}, nil
}

func markPokemonCaughtInTx(tx pokebattle.DBTX, charID int64, pokemonID int) error {
	_, err := tx.Exec(`
		INSERT INTO character_pokedex (character_id, pokemon_id, seen, caught, first_seen_at, first_caught_at)
		VALUES ($1, $2, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (character_id, pokemon_id) DO UPDATE SET
			seen = 1,
			caught = 1,
			first_seen_at = COALESCE(character_pokedex.first_seen_at, EXCLUDED.first_seen_at),
			first_caught_at = COALESCE(character_pokedex.first_caught_at, EXCLUDED.first_caught_at)`,
		charID, pokemonID)
	return err
}

func (t inGameTradeDefinition) originalTrainerID() int64 {
	if t.OriginalTradeIndex >= 0 {
		return int64(900000 + t.OriginalTradeIndex)
	}
	return 900000
}

func (t inGameTradeDefinition) offerDialogue() string {
	switch t.DialogueSet {
	case "EVOLUTION":
		return "Hello there!"
	case "HAPPY":
		return fmt.Sprintf("Hi! Do you have\n%s?", t.RequestedPokemonName)
	default:
		return fmt.Sprintf("I'm looking for\n%s!", t.RequestedPokemonName)
	}
}

func (t inGameTradeDefinition) promptDialogue() string {
	switch t.DialogueSet {
	case "EVOLUTION":
		return fmt.Sprintf("Do you want to trade\nyour %s for %s?", t.RequestedPokemonName, t.OfferedPokemonName)
	case "HAPPY":
		return fmt.Sprintf("Want to trade it\nfor %s?", t.OfferedPokemonName)
	default:
		return fmt.Sprintf("Wanna trade one for\n%s?", t.OfferedPokemonName)
	}
}

func (t inGameTradeDefinition) noTradeDialogue() string {
	switch t.DialogueSet {
	case "EVOLUTION":
		return "Well, if you\ndon't want to..."
	case "HAPPY":
		return "That's too bad."
	default:
		return "Awww!\nOh well..."
	}
}

func (t inGameTradeDefinition) wrongPokemonDialogue() string {
	switch t.DialogueSet {
	case "EVOLUTION":
		return fmt.Sprintf("Hmmm? This isn't\n%s.\n\nThink of me when\nyou get one.", t.RequestedPokemonName)
	case "HAPPY":
		return fmt.Sprintf("...This is no\n%s.\n\nIf you get one,\ntrade it with me!", t.RequestedPokemonName)
	default:
		return fmt.Sprintf("What? That's not\n%s!\n\nIf you get one,\ncome back here!", t.RequestedPokemonName)
	}
}

func (t inGameTradeDefinition) completedTradeDialogue() string {
	thanks := "Hey thanks!"
	switch t.DialogueSet {
	case "EVOLUTION":
		thanks = "Thanks!"
	case "HAPPY":
		thanks = "Thanks pal!"
	}
	return fmt.Sprintf("Okay, connect the cable like so!\n\nYou traded %s for %s!\n\n%s",
		t.RequestedPokemonName, t.OfferedPokemonName, thanks)
}

func (t inGameTradeDefinition) afterTradeDialogue() string {
	switch t.DialogueSet {
	case "EVOLUTION":
		return fmt.Sprintf("The %s you\ntraded to me\n\nwent and evolved!", t.RequestedPokemonName)
	case "HAPPY":
		return fmt.Sprintf("How is my old\n%s?\n\nMy %s is\ndoing great!", t.OfferedPokemonName, t.RequestedPokemonName)
	default:
		return fmt.Sprintf("Isn't my old\n%s great?", t.OfferedPokemonName)
	}
}
