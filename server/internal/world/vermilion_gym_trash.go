package world

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"capturequest/internal/db"
)

const (
	EventVermilionGymFirstLockOpened  = "EVENT_1ST_LOCK_OPENED"
	EventVermilionGymSecondLockOpened = "EVENT_2ND_LOCK_OPENED"
)

type VermilionGymTrashState struct {
	FirstLockCanIndex  int
	SecondLockCanIndex *int
}

type VermilionGymTrashOutcome struct {
	CanIndex     int
	TextConstant string
	Dialogue     []string
	Changed      bool
	OpenedFirst  bool
	OpenedSecond bool
	ResetLocks   bool
	State        VermilionGymTrashState
}

type VermilionGymTrashPicker interface {
	PickFirstLockCanIndex() (int, error)
	PickSecondLockCanIndex(firstLockCanIndex int) (int, error)
}

type RandomVermilionGymTrashPicker struct{}

type FixedVermilionGymTrashPicker struct {
	FirstLockCanIndex  *int
	SecondLockCanIndex *int
}

var vermilionGymTrashSecondLockCandidates = map[int][]int{
	0:  {1, 3},
	1:  {0, 2, 4},
	2:  {1, 5},
	3:  {0, 4, 6},
	4:  {1, 3, 5, 7},
	5:  {2, 4, 8},
	6:  {3, 7, 9},
	7:  {4, 6, 8, 10},
	8:  {5, 7, 11},
	9:  {6, 10, 12},
	10: {7, 9, 11, 13},
	11: {8, 10, 14},
	12: {9, 13},
	13: {10, 12, 14},
	14: {11, 13},
}

func IsVermilionGymTrashTextConstant(textConstant string) bool {
	_, ok := VermilionGymTrashCanIndexForTextConstant(textConstant)
	return ok
}

func VermilionGymTrashCanIndexForTextConstant(textConstant string) (int, bool) {
	const prefix = "TEXT_VERMILIONGYM_TRASH_CAN_"
	if !strings.HasPrefix(textConstant, prefix) {
		return 0, false
	}
	idx, err := strconv.Atoi(strings.TrimPrefix(textConstant, prefix))
	if err != nil || idx < 0 || idx > 14 {
		return 0, false
	}
	return idx, true
}

func HandleVermilionGymTrashCan(charID int64, canIndex int, efm *EventFlagManager) (*VermilionGymTrashOutcome, error) {
	return HandleVermilionGymTrashCanWithPicker(charID, canIndex, efm, RandomVermilionGymTrashPicker{})
}

func HandleVermilionGymTrashCanWithPicker(charID int64, canIndex int, efm *EventFlagManager, picker VermilionGymTrashPicker) (*VermilionGymTrashOutcome, error) {
	if canIndex < 0 || canIndex > 14 {
		return nil, fmt.Errorf("vermilion gym trash can index %d out of range", canIndex)
	}
	if picker == nil {
		picker = RandomVermilionGymTrashPicker{}
	}
	state, err := ensureVermilionGymTrashState(charID, picker)
	if err != nil {
		return nil, err
	}

	if efm != nil && efm.CheckFlag(charID, EventVermilionGymSecondLockOpened) {
		return trashOutcome(canIndex, state), nil
	}

	firstOpen := efm != nil && efm.CheckFlag(charID, EventVermilionGymFirstLockOpened)
	if !firstOpen {
		if canIndex != state.FirstLockCanIndex {
			return trashOutcome(canIndex, state), nil
		}
		if efm != nil {
			if err := efm.SetFlag(charID, EventVermilionGymFirstLockOpened); err != nil {
				return nil, err
			}
		}
		second, err := picker.PickSecondLockCanIndex(canIndex)
		if err != nil {
			return nil, err
		}
		state.SecondLockCanIndex = &second
		if err := saveVermilionGymTrashState(charID, state); err != nil {
			return nil, err
		}
		return &VermilionGymTrashOutcome{
			CanIndex:     canIndex,
			TextConstant: "TEXT_VERMILIONGYM_TRASH_SUCCESS_1",
			Dialogue: []string{
				"Hey! There's a switch under the trash! Turn it on!",
				"The 1st electric lock opened!",
			},
			Changed:     true,
			OpenedFirst: true,
			State:       state,
		}, nil
	}

	if state.SecondLockCanIndex != nil && canIndex == *state.SecondLockCanIndex {
		if efm != nil {
			if err := efm.SetFlag(charID, EventVermilionGymSecondLockOpened); err != nil {
				return nil, err
			}
		}
		return &VermilionGymTrashOutcome{
			CanIndex:     canIndex,
			TextConstant: "TEXT_VERMILIONGYM_TRASH_SUCCESS_3",
			Dialogue: []string{
				"The 2nd electric lock opened!",
				"The motorized door opened!",
			},
			Changed:      true,
			OpenedSecond: true,
			State:        state,
		}, nil
	}

	if efm != nil {
		if err := efm.ResetFlag(charID, EventVermilionGymFirstLockOpened); err != nil {
			return nil, err
		}
	}
	first, err := picker.PickFirstLockCanIndex()
	if err != nil {
		return nil, err
	}
	state = VermilionGymTrashState{FirstLockCanIndex: first}
	if err := saveVermilionGymTrashState(charID, state); err != nil {
		return nil, err
	}
	return &VermilionGymTrashOutcome{
		CanIndex:     canIndex,
		TextConstant: "TEXT_VERMILIONGYM_TRASH_FAIL",
		Dialogue: []string{
			"Nope! There's only trash here.",
			"Hey! The electric locks were reset!",
		},
		Changed:    true,
		ResetLocks: true,
		State:      state,
	}, nil
}

func LoadVermilionGymTrashState(charID int64) (*VermilionGymTrashState, error) {
	var state VermilionGymTrashState
	var second sql.NullInt64
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT first_lock_can_index, second_lock_can_index
		FROM character_vermilion_gym_trash_state
		WHERE character_id = $1`, charID).Scan(&state.FirstLockCanIndex, &second)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if second.Valid {
		value := int(second.Int64)
		state.SecondLockCanIndex = &value
	}
	return &state, nil
}

func SetVermilionGymTrashState(charID int64, state VermilionGymTrashState) error {
	if state.FirstLockCanIndex < 0 || state.FirstLockCanIndex > 14 {
		return fmt.Errorf("first trash can index %d out of range", state.FirstLockCanIndex)
	}
	if state.SecondLockCanIndex != nil && (*state.SecondLockCanIndex < 0 || *state.SecondLockCanIndex > 14) {
		return fmt.Errorf("second trash can index %d out of range", *state.SecondLockCanIndex)
	}
	return saveVermilionGymTrashState(charID, state)
}

func (RandomVermilionGymTrashPicker) PickFirstLockCanIndex() (int, error) {
	n, err := cryptoRandomInt(8)
	if err != nil {
		return 0, err
	}
	return n * 2, nil
}

func (RandomVermilionGymTrashPicker) PickSecondLockCanIndex(firstLockCanIndex int) (int, error) {
	candidates := vermilionGymTrashSecondLockCandidates[firstLockCanIndex]
	if len(candidates) == 0 {
		return 0, fmt.Errorf("no second-lock candidates for trash can %d", firstLockCanIndex)
	}
	n, err := cryptoRandomInt(len(candidates))
	if err != nil {
		return 0, err
	}
	return candidates[n], nil
}

func (p FixedVermilionGymTrashPicker) PickFirstLockCanIndex() (int, error) {
	if p.FirstLockCanIndex == nil {
		return RandomVermilionGymTrashPicker{}.PickFirstLockCanIndex()
	}
	if *p.FirstLockCanIndex < 0 || *p.FirstLockCanIndex > 14 || *p.FirstLockCanIndex%2 != 0 {
		return 0, fmt.Errorf("fixed first trash can index %d must be even and in 0..14", *p.FirstLockCanIndex)
	}
	return *p.FirstLockCanIndex, nil
}

func (p FixedVermilionGymTrashPicker) PickSecondLockCanIndex(firstLockCanIndex int) (int, error) {
	if p.SecondLockCanIndex == nil {
		return RandomVermilionGymTrashPicker{}.PickSecondLockCanIndex(firstLockCanIndex)
	}
	candidates := vermilionGymTrashSecondLockCandidates[firstLockCanIndex]
	for _, candidate := range candidates {
		if candidate == *p.SecondLockCanIndex {
			return *p.SecondLockCanIndex, nil
		}
	}
	return 0, fmt.Errorf("fixed second trash can index %d is not valid after first can %d", *p.SecondLockCanIndex, firstLockCanIndex)
}

func ensureVermilionGymTrashState(charID int64, picker VermilionGymTrashPicker) (VermilionGymTrashState, error) {
	state, err := LoadVermilionGymTrashState(charID)
	if err != nil {
		return VermilionGymTrashState{}, err
	}
	if state != nil {
		return *state, nil
	}
	first, err := picker.PickFirstLockCanIndex()
	if err != nil {
		return VermilionGymTrashState{}, err
	}
	newState := VermilionGymTrashState{FirstLockCanIndex: first}
	if err := saveVermilionGymTrashState(charID, newState); err != nil {
		return VermilionGymTrashState{}, err
	}
	return newState, nil
}

func saveVermilionGymTrashState(charID int64, state VermilionGymTrashState) error {
	var second interface{}
	if state.SecondLockCanIndex != nil {
		second = *state.SecondLockCanIndex
	}
	_, err := db.GlobalWorldDB.DB.Exec(`
		INSERT INTO character_vermilion_gym_trash_state
			(character_id, first_lock_can_index, second_lock_can_index)
		VALUES ($1, $2, $3)
		ON CONFLICT (character_id) DO UPDATE SET
			first_lock_can_index = EXCLUDED.first_lock_can_index,
			second_lock_can_index = EXCLUDED.second_lock_can_index,
			updated_at = CURRENT_TIMESTAMP`,
		charID, state.FirstLockCanIndex, second)
	return err
}

func trashOutcome(canIndex int, state VermilionGymTrashState) *VermilionGymTrashOutcome {
	return &VermilionGymTrashOutcome{
		CanIndex:     canIndex,
		TextConstant: "TEXT_VERMILIONGYM_TRASH",
		Dialogue:     []string{"Nope, there's only trash here."},
		State:        state,
	}
}

func cryptoRandomInt(max int) (int, error) {
	if max <= 0 {
		return 0, fmt.Errorf("random max must be positive")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}
