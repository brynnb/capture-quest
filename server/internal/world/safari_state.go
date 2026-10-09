package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"

	"capturequest/internal/db"
	"capturequest/internal/db/pokedex"
	"capturequest/internal/itemuse"
	"capturequest/internal/pokebattle"
	"github.com/google/uuid"
)

const (
	SafariZoneEntryFee  = 500
	SafariZoneMaxBalls  = 30
	SafariZoneMaxSteps  = 500
	SafariZoneGateMapID = 156 // SAFARI_ZONE_GATE

	SafariZoneCenterMapID   = 220
	SafariZoneDefaultEntryX = 14
	SafariZoneDefaultEntryY = 25
	SafariZoneGateReturnX   = 3
	SafariZoneGateReturnY   = 4

	EventInSafariZone   = "EVENT_IN_SAFARI_ZONE"
	EventSafariGameOver = "EVENT_SAFARI_GAME_OVER"
	SafariExpiryMessage = "PA: Ding-dong! Your SAFARI GAME is over!"
)

// Safari Zone map IDs (the 4 zones where encounters happen)
var safariZoneMapIDs = map[int]bool{
	217: true, // SAFARI_ZONE_EAST
	218: true, // SAFARI_ZONE_NORTH
	219: true, // SAFARI_ZONE_WEST
	220: true, // SAFARI_ZONE_CENTER
}

// SafariSession tracks a player's current Safari Zone visit.
type SafariSession struct {
	VisitID   string                        `json:"visitId"`
	Revision  int64                         `json:"revision"`
	BallsLeft int                           `json:"ballsLeft"`
	StepsLeft int                           `json:"stepsLeft"`
	Active    bool                          `json:"active"`
	Battle    *pokebattle.SafariBattleState `json:"battle,omitempty"` // Non-nil if in a safari battle
	Capture   *pokebattle.CapturePlacement  `json:"capture,omitempty"`
}

type SafariEntryResult struct {
	Success       bool
	Message       string
	Money         int
	BallsLeft     int
	StepsLeft     int
	AlreadyActive bool
}

// IsInSafariZone checks if a map ID is one of the safari zone maps.
func IsInSafariZone(mapID int) bool {
	return safariZoneMapIDs[mapID]
}

// Safari has one durable owner. Returned snapshots are independent decoded
// values; mutation always reloads under the shared character lock.
type SafariZoneManager struct{ database *sql.DB }

func NewSafariZoneManager(database *sql.DB) *SafariZoneManager {
	return &SafariZoneManager{database: database}
}

// Bump this version and define a migration before changing persisted meaning.
const safariStateVersion = 3

type storedSafariState struct {
	Version int            `json:"version"`
	Visit   *SafariSession `json:"visit"`
}

func validateSafariState(s *SafariSession, legacy bool) error {
	if s.BallsLeft < 0 || s.BallsLeft > SafariZoneMaxBalls || s.StepsLeft < 0 || s.StepsLeft > SafariZoneMaxSteps {
		return fmt.Errorf("invalid safari counters balls=%d steps=%d", s.BallsLeft, s.StepsLeft)
	}
	if s.Active && (s.BallsLeft == 0 || s.StepsLeft == 0) {
		return fmt.Errorf("active safari requires balls and steps")
	}
	if b := s.Battle; b != nil {
		if (!legacy || b.BattleID != "" || b.Revision != 0) && (b.BattleID == "" || b.Revision < 1) {
			return fmt.Errorf("invalid safari encounter command identity")
		}
		if b.WildPokemon == nil || b.WildPokemon.ID < 1 || b.WildPokemon.ID > 151 || b.WildPokemon.RowID != 0 || (!b.IsOver() && (!s.Active || b.Phase != pokebattle.SafariPhaseAction || b.Caught || b.Fled)) || b.BallsLeft != s.BallsLeft || b.StepsLeft != s.StepsLeft {
			return fmt.Errorf("invalid safari battle identity/phase/counters")
		}
	}
	if p := s.Capture; p != nil && (s.Battle == nil || !s.Battle.Caught || !s.Battle.IsOver() || (p.SentToPC && (p.PCBox < 0 || p.PCBox >= 12))) {
		return fmt.Errorf("invalid safari capture placement")
	}
	return nil
}

func safariSessionIn(database db.DBTX, charID int64) (*SafariSession, error) {
	var encoded string
	err := database.QueryRow(`SELECT state_json FROM character_safari_state WHERE character_id=$1`, charID).Scan(&encoded)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var saved storedSafariState
	if err := json.Unmarshal([]byte(encoded), &saved); err != nil {
		return nil, fmt.Errorf("safari character %d: %w", charID, err)
	}
	if saved.Version != 1 && saved.Version != 2 && saved.Version != safariStateVersion {
		return nil, fmt.Errorf("safari character %d unsupported state version %d", charID, saved.Version)
	}
	if saved.Visit == nil {
		return nil, fmt.Errorf("safari character %d missing visit", charID)
	}
	if err := validateSafariState(saved.Visit, saved.Version == 1); err != nil {
		return nil, fmt.Errorf("safari character %d: %w", charID, err)
	}
	if saved.Version < safariStateVersion {
		// Supported legacy state is upgraded once under the existing owner lock.
		// Never repair a malformed current-version identity by inventing another one.
		if err := db.RequireTransaction(database); err != nil {
			return nil, fmt.Errorf("Safari legacy upgrade requires transaction: %w", err)
		}
		saved.Visit.VisitID = uuid.NewString()
		saved.Visit.Revision = 0
		if saved.Visit.Battle != nil && saved.Visit.Battle.BattleID == "" {
			saved.Visit.Battle.BattleID = uuid.NewString()
			saved.Visit.Battle.Revision = 1
		}
		if err := saveSafariSessionIn(database, charID, saved.Visit); err != nil {
			return nil, err
		}
	} else if id, err := uuid.Parse(saved.Visit.VisitID); err != nil || id == uuid.Nil || saved.Visit.Revision < 1 {
		return nil, fmt.Errorf("Safari character %d invalid visit identity/revision", charID)
	}
	return saved.Visit, nil
}

func saveSafariSessionIn(tx db.DBTX, charID int64, s *SafariSession) error {
	if err := db.RequireTransaction(tx); err != nil {
		return err
	}
	if s == nil {
		_, err := tx.Exec(`DELETE FROM character_safari_state WHERE character_id=$1`, charID)
		return err
	}
	if id, err := uuid.Parse(s.VisitID); err != nil || id == uuid.Nil || s.Revision < 0 || s.Revision == math.MaxInt64 {
		return fmt.Errorf("invalid Safari visit identity/revision")
	}
	s.Revision++
	if err := validateSafariState(s, false); err != nil {
		return err
	}
	encoded, err := json.Marshal(storedSafariState{Version: safariStateVersion, Visit: s})
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO character_safari_state(character_id,state_json) VALUES($1,$2) ON CONFLICT(character_id) DO UPDATE SET state_json=EXCLUDED.state_json,updated_at=CURRENT_TIMESTAMP`, charID, string(encoded))
	return err
}

func (m *SafariZoneManager) Load(ctx context.Context) error {
	rows, err := m.database.QueryContext(ctx, `SELECT character_id,state_json FROM character_safari_state LIMIT 0`)
	if err != nil {
		return fmt.Errorf("safari state schema: %w", err)
	}
	return rows.Close()
}
func (m *SafariZoneManager) GetSession(ctx context.Context, charID int64) (*SafariSession, error) {
	var s *SafariSession
	err := db.Transaction(ctx, m.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		var err error
		s, err = safariSessionIn(tx, charID)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}
func (m *SafariZoneManager) mutate(ctx context.Context, charID int64, apply func(db.DBTX, *SafariSession) error) error {
	return db.Transaction(ctx, m.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		s, err := safariSessionIn(tx, charID)
		if err != nil {
			return err
		}
		if err := apply(tx, s); err != nil {
			return err
		}
		return saveSafariSessionIn(tx, charID, s)
	})
}

// SetSession is explicit fixture setup, never an unlocked runtime writeback.
func (m *SafariZoneManager) SetSession(ctx context.Context, charID int64, s SafariSession) error {
	if s.VisitID == "" {
		s.VisitID = uuid.NewString()
		s.Revision = 0
	}
	return db.Transaction(ctx, m.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		return saveSafariSessionIn(tx, charID, &s)
	})
}
func endSafariSessionIn(tx db.DBTX, charID int64) error {
	if err := saveSafariSessionIn(tx, charID, nil); err != nil {
		return err
	}
	if err := writeEventFlag(tx, charID, EventInSafariZone, false); err != nil {
		return err
	}
	return writeEventFlag(tx, charID, EventSafariGameOver, false)
}
func (m *SafariZoneManager) EndSession(ctx context.Context, charID int64) error {
	return db.Transaction(ctx, m.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		return endSafariSessionIn(tx, charID)
	})
}
func advanceSafariStep(s *SafariSession) (expired bool) {
	s.StepsLeft--
	if s.StepsLeft == 0 {
		s.Active = false
		return true
	}
	return false
}
func (m *SafariZoneManager) DecrementStep(ctx context.Context, charID int64) (int, int, bool, error) {
	var steps, balls int
	var expired bool
	err := m.mutate(ctx, charID, func(tx db.DBTX, s *SafariSession) error {
		if s == nil || !s.Active || s.Battle != nil {
			return nil
		}
		expired = advanceSafariStep(s)
		steps, balls = s.StepsLeft, s.BallsLeft
		return nil
	})
	if err != nil {
		return 0, 0, false, err
	}
	return steps, balls, expired, nil
}

// The caller owns the character lock and outer commit, including script warps.
func startSafariVisitIn(tx db.DBTX, charID int64) (SafariEntryResult, error) {
	if err := db.RequireTransaction(tx); err != nil {
		return SafariEntryResult{}, err
	}
	s, err := safariSessionIn(tx, charID)
	if err != nil {
		return SafariEntryResult{}, err
	}
	if s != nil && s.Battle != nil && !s.Active {
		return SafariEntryResult{Message: "Finish your previous Safari encounter first."}, nil
	}
	if _, err := tx.Exec(`INSERT INTO character_wallet(character_id,pokedollars) VALUES($1,0) ON CONFLICT(character_id) DO NOTHING`, charID); err != nil {
		return SafariEntryResult{}, err
	}
	var result SafariEntryResult
	if err := tx.QueryRow(`SELECT COALESCE(pokedollars,0) FROM character_wallet WHERE character_id=$1`, charID).Scan(&result.Money); err != nil {
		return SafariEntryResult{}, err
	}
	if s != nil && s.Active {
		result.Success = true
		result.AlreadyActive = true
		result.Message = "already in safari session"
		result.BallsLeft = s.BallsLeft
		result.StepsLeft = s.StepsLeft
		return result, nil
	}
	if result.Money < SafariZoneEntryFee {
		result.Message = "not enough money"
		return result, nil
	}
	if err := tx.QueryRow(`UPDATE character_wallet SET pokedollars=pokedollars-$1 WHERE character_id=$2 AND pokedollars>=$1 RETURNING pokedollars`, SafariZoneEntryFee, charID).Scan(&result.Money); err != nil {
		return SafariEntryResult{}, err
	}
	s = &SafariSession{VisitID: uuid.NewString(), Active: true, BallsLeft: SafariZoneMaxBalls, StepsLeft: SafariZoneMaxSteps}
	if err := saveSafariSessionIn(tx, charID, s); err != nil {
		return SafariEntryResult{}, err
	}
	if err := writeEventFlag(tx, charID, EventInSafariZone, true); err != nil {
		return SafariEntryResult{}, err
	}
	if err := writeEventFlag(tx, charID, EventSafariGameOver, false); err != nil {
		return SafariEntryResult{}, err
	}
	result.Success = true
	result.BallsLeft = s.BallsLeft
	result.StepsLeft = s.StepsLeft
	return result, nil
}
func TryStartSafariZoneVisit(ctx context.Context, charID int64, m *SafariZoneManager) (SafariEntryResult, error) {
	var result SafariEntryResult
	err := db.Transaction(ctx, m.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		var err error
		result, err = startSafariVisitIn(tx, charID)
		return err
	})
	if err != nil {
		return SafariEntryResult{}, err
	}
	return result, nil
}

type safariActionResult struct {
	Party    []*pokebattle.Pokemon
	Closed   bool
	Visit    *SafariSession
	Battle   *pokebattle.SafariBattleState
	SentToPC bool
	PCBox    int
}

func (m *SafariZoneManager) act(ctx context.Context, charID int64, action string, identity BattleCommandIdentity) (safariActionResult, error) {
	var result safariActionResult
	err := m.mutate(ctx, charID, func(tx db.DBTX, s *SafariSession) error {
		if s == nil || s.Battle == nil {
			return &itemuse.Rejection{Message: "not in safari battle"}
		}
		b := s.Battle
		if identity.BattleID == "" || identity.Revision < 1 || b.BattleID != identity.BattleID || b.Revision != identity.Revision {
			return &itemuse.Rejection{Message: "Safari encounter changed. Reconnect to recover its current state."}
		}
		if action == "close" {
			if !b.IsOver() {
				return &itemuse.Rejection{Message: "Safari encounter is not finished"}
			}
			b.Revision++
			result.Visit, result.Battle, result.Closed = s, b, true
			s.Battle, s.Capture = nil, nil
			return nil
		}
		if !s.Active || b.IsOver() {
			return &itemuse.Rejection{Message: "Safari encounter is finished"}
		}
		switch action {
		case "ball":
			b.ThrowBall()
			s.BallsLeft = b.BallsLeft
		case "bait":
			b.ThrowBait()
		case "rock":
			b.ThrowRock()
		case "run":
			b.Run()
		default:
			return &itemuse.Rejection{Message: "invalid action"}
		}
		b.Revision++
		result.Visit = s
		result.Battle = b
		if b.Caught {
			b.WildPokemon.IsWild = false
			party, box, _, err := pokebattle.SavePreparedPokemonToPartyOrPC(tx, charID, b.WildPokemon)
			if err != nil {
				return err
			}
			result.SentToPC = !party
			result.PCBox = box
			s.Capture = &pokebattle.CapturePlacement{SentToPC: !party, PCBox: box}
			if err := pokedex.MarkCaught(tx, charID, b.WildPokemon.ID); err != nil {
				return err
			}
			result.Party, err = pokebattle.LoadParty(tx, charID)
			if err != nil {
				return err
			}
		}
		// Keep the terminal encounter until identity-bound dismissal. Reply loss
		// must not erase a catch summary or permit another encounter to overwrite it.
		// Exhaustion ends the visit even when its last ball catches a Pokémon.
		if s.BallsLeft == 0 {
			s.Active = false
			return expireSafariVisitIn(tx, charID)
		}
		return nil
	})
	if err != nil {
		return safariActionResult{}, err
	}
	return result, nil
}

// Preserve the source gate-exit script flags and save its forced destination in
// the same commit as exhaustion. Reconnect must not resurrect an expired visit
// at its old position just because the client missed the exit notification.
func expireSafariVisitIn(tx db.DBTX, charID int64) error {
	if err := writeEventFlag(tx, charID, EventSafariGameOver, true); err != nil {
		return err
	}
	return saveFieldDestinationIn(tx, charID, SafariZoneGateMapID, SafariZoneGateReturnX, SafariZoneGateReturnY)
}

func endSafariForDestinationIn(tx db.DBTX, charID int64, mapID int) (bool, error) {
	if IsInSafariZone(mapID) || mapID == SafariZoneGateMapID {
		return false, nil
	}
	s, err := safariSessionIn(tx, charID)
	if err != nil || s == nil {
		return false, err
	}
	return true, endSafariSessionIn(tx, charID)
}
