package pokebattle

import (
	"encoding/json"
	"fmt"
)

// --- JSON-serializable battle state for DB persistence ---

type persistedBattle struct {
	Phase                int      `json:"phase"`
	BattleType           int      `json:"battleType"`
	TurnNumber           int      `json:"turnNumber"`
	PlayerActive         int      `json:"playerActive"`
	EnemyActive          int      `json:"enemyActive"`
	RunAttempts          int      `json:"runAttempts"`
	PlayerCaught         bool     `json:"playerCaught"`
	PlayerTrainerID      int64    `json:"playerTrainerId,omitempty"`
	PlayerObedienceLevel int      `json:"playerObedienceLevel,omitempty"`
	AllowedActions       []string `json:"allowedActions,omitempty"`
	GuaranteedCatch      bool     `json:"guaranteedCatch,omitempty"`
	// Player party is NOT stored here — it's persisted in character_pokemon
	// and reloaded via LoadParty on restore to avoid sync issues.
	EnemyParty          []persistedPokemon `json:"enemyParty"`
	Trainer             *persistedTrainer  `json:"trainer,omitempty"`
	WildWinFlag         string             `json:"wildWinFlag,omitempty"`
	WildPostWinMapName  string             `json:"wildPostWinMapName,omitempty"`
	WildPostWinActions  json.RawMessage    `json:"wildPostWinActions,omitempty"`
	PendingMove         *persistedPending  `json:"pendingMove,omitempty"`
	PostMoveLearnEvents []BattleEvent      `json:"postMoveLearnEvents,omitempty"`
}

type persistedPokemon struct {
	ID             int              `json:"id"`
	Name           string           `json:"name"`
	Level          int              `json:"level"`
	IsWild         bool             `json:"isWild"`
	Type1          int              `json:"type1"`
	Type2          int              `json:"type2"`
	BaseStats      [5]int           `json:"baseStats"` // HP, Atk, Def, Spc, Spd
	IVs            [4]int           `json:"ivs"`       // Atk, Def, Spc, Spd
	EVs            [5]int           `json:"evs"`       // HP, Atk, Def, Spc, Spd
	MaxHP          int              `json:"maxHp"`
	CurHP          int              `json:"curHp"`
	Attack         int              `json:"attack"`
	Defense        int              `json:"defense"`
	Special        int              `json:"special"`
	Speed          int              `json:"speed"`
	Moves          [4]persistedMove `json:"moves"`
	Status         int              `json:"status"`
	SleepTurns     int              `json:"sleepTurns"`
	BadPoisonTurns int              `json:"badPoisonTurns,omitempty"`
	ConfusionTurns int              `json:"confusionTurns"`
	IsSeeded       bool             `json:"isSeeded"`
	SubstituteHP   int              `json:"substituteHp"`
	DireHit        bool             `json:"direHit"`
	GuardSpec      bool             `json:"guardSpec"`
	AtkStage       int              `json:"atkStage"`
	DefStage       int              `json:"defStage"`
	SpcStage       int              `json:"spcStage"`
	SpdStage       int              `json:"spdStage"`
	AccStage       int              `json:"accStage"`
	EvaStage       int              `json:"evaStage"`
	CatchRate      int              `json:"catchRate"`
	BaseSpeed      int              `json:"baseSpeed"`
	BaseExp        int              `json:"baseExp"`
	GrowthRt       int              `json:"growthRate"`
	Exp            int              `json:"exp"`
}

type persistedMove struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Type       int    `json:"type"`
	Power      int    `json:"power"`
	Accuracy   int    `json:"accuracy"`
	PP         int    `json:"pp"`
	MaxPP      int    `json:"maxPp"`
	BasePP     int    `json:"basePp"`
	PPUps      int    `json:"ppUps"`
	Effect     string `json:"effect"`
	IsHighCrit bool   `json:"isHighCrit"`
}

type persistedTrainer struct {
	ClassName        string          `json:"className"`
	Name             string          `json:"name"`
	PrizeMoney       int             `json:"prizeMoney"`
	TrainerObjectID  int             `json:"trainerObjectId"`
	WinFlag          string          `json:"winFlag,omitempty"`
	LoseFlag         string          `json:"loseFlag,omitempty"`
	LossMessage      string          `json:"lossMessage,omitempty"`
	NoBlackoutOnLoss bool            `json:"noBlackoutOnLoss,omitempty"`
	PostWinMapName   string          `json:"postWinMapName,omitempty"`
	PostWinActions   json.RawMessage `json:"postWinActions,omitempty"`
	PostLoseMapName  string          `json:"postLoseMapName,omitempty"`
	PostLoseActions  json.RawMessage `json:"postLoseActions,omitempty"`
}

type persistedPending struct {
	PokemonIndex int    `json:"pokemonIndex"`
	MoveID       int    `json:"moveId"`
	MoveName     string `json:"moveName"`
}

// MarshalBattleState serializes a BattleState to JSON for DB storage.
func MarshalBattleState(b *BattleState) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf("nil battle state")
	}
	pb := persistedBattle{
		Phase:                int(b.Phase),
		BattleType:           int(b.BattleType),
		TurnNumber:           b.TurnNumber,
		PlayerActive:         b.PlayerActive,
		EnemyActive:          b.EnemyActive,
		RunAttempts:          b.RunAttempts,
		PlayerCaught:         b.PlayerCaught,
		PlayerTrainerID:      b.PlayerTrainerID,
		PlayerObedienceLevel: b.PlayerObedienceLevel,
		AllowedActions:       b.AllowedActions,
		GuaranteedCatch:      b.GuaranteedCatch,
		WildWinFlag:          b.WildWinFlag,
	}
	if b.WildPostWinMapName != "" {
		pb.WildPostWinMapName = b.WildPostWinMapName
	}
	if len(b.WildPostWinActions) > 0 {
		pb.WildPostWinActions = b.WildPostWinActions
	}

	// Player party is NOT serialized — it's already in character_pokemon.
	for _, p := range b.EnemyParty {
		pb.EnemyParty = append(pb.EnemyParty, pokemonToPersisted(p))
	}

	if b.Trainer != nil {
		pb.Trainer = &persistedTrainer{
			ClassName:        b.Trainer.ClassName,
			Name:             b.Trainer.Name,
			PrizeMoney:       b.Trainer.PrizeMoney,
			TrainerObjectID:  b.Trainer.TrainerObjectID,
			WinFlag:          b.Trainer.WinFlag,
			LoseFlag:         b.Trainer.LoseFlag,
			LossMessage:      b.Trainer.LossMessage,
			NoBlackoutOnLoss: b.Trainer.NoBlackoutOnLoss,
			PostWinMapName:   b.Trainer.PostWinMapName,
			PostWinActions:   b.Trainer.PostWinActions,
			PostLoseMapName:  b.Trainer.PostLoseMapName,
			PostLoseActions:  b.Trainer.PostLoseActions,
		}
	}

	if b.PendingMoveLearn != nil {
		pb.PendingMove = &persistedPending{
			PokemonIndex: b.PendingMoveLearn.PokemonIndex,
			MoveID:       b.PendingMoveLearn.MoveID,
			MoveName:     b.PendingMoveLearn.MoveName,
		}
	}

	if len(b.PostMoveLearnEvents) > 0 {
		pb.PostMoveLearnEvents = b.PostMoveLearnEvents
	}

	return json.Marshal(pb)
}

// UnmarshalBattleState deserializes a BattleState from JSON.
func UnmarshalBattleState(data []byte) (*BattleState, error) {
	var pb persistedBattle
	if err := json.Unmarshal(data, &pb); err != nil {
		return nil, fmt.Errorf("unmarshal battle state: %w", err)
	}

	b := &BattleState{
		Phase:                BattlePhase(pb.Phase),
		BattleType:           BattleType(pb.BattleType),
		TurnNumber:           pb.TurnNumber,
		PlayerActive:         pb.PlayerActive,
		EnemyActive:          pb.EnemyActive,
		RunAttempts:          pb.RunAttempts,
		PlayerCaught:         pb.PlayerCaught,
		PlayerTrainerID:      pb.PlayerTrainerID,
		PlayerObedienceLevel: pb.PlayerObedienceLevel,
		AllowedActions:       pb.AllowedActions,
		GuaranteedCatch:      pb.GuaranteedCatch,
		WildWinFlag:          pb.WildWinFlag,
	}
	if b.PlayerObedienceLevel <= 0 {
		b.PlayerObedienceLevel = MaxObedienceLevel
	}
	if pb.WildPostWinMapName != "" {
		b.WildPostWinMapName = pb.WildPostWinMapName
	}
	if len(pb.WildPostWinActions) > 0 {
		b.WildPostWinActions = pb.WildPostWinActions
	}

	// Player party is NOT in JSON — caller must reload from character_pokemon via LoadParty.
	for _, pp := range pb.EnemyParty {
		b.EnemyParty = append(b.EnemyParty, persistedToPokemon(pp))
	}

	if pb.Trainer != nil {
		b.Trainer = &TrainerMeta{
			ClassName:        pb.Trainer.ClassName,
			Name:             pb.Trainer.Name,
			PrizeMoney:       pb.Trainer.PrizeMoney,
			TrainerObjectID:  pb.Trainer.TrainerObjectID,
			WinFlag:          pb.Trainer.WinFlag,
			LoseFlag:         pb.Trainer.LoseFlag,
			LossMessage:      pb.Trainer.LossMessage,
			NoBlackoutOnLoss: pb.Trainer.NoBlackoutOnLoss,
			PostWinMapName:   pb.Trainer.PostWinMapName,
			PostWinActions:   pb.Trainer.PostWinActions,
			PostLoseMapName:  pb.Trainer.PostLoseMapName,
			PostLoseActions:  pb.Trainer.PostLoseActions,
		}
	}

	if pb.PendingMove != nil {
		b.PendingMoveLearn = &PendingMove{
			PokemonIndex: pb.PendingMove.PokemonIndex,
			MoveID:       pb.PendingMove.MoveID,
			MoveName:     pb.PendingMove.MoveName,
		}
	}

	if len(pb.PostMoveLearnEvents) > 0 {
		b.PostMoveLearnEvents = pb.PostMoveLearnEvents
	}

	return b, nil
}

func pokemonToPersisted(p *Pokemon) persistedPokemon {
	pp := persistedPokemon{
		ID:             p.ID,
		Name:           p.Name,
		Level:          p.Level,
		IsWild:         p.IsWild,
		Type1:          int(p.Type1),
		Type2:          int(p.Type2),
		BaseStats:      [5]int{p.BaseStats.HP, p.BaseStats.Attack, p.BaseStats.Defense, p.BaseStats.Special, p.BaseStats.Speed},
		IVs:            [4]int{p.IVs.Attack, p.IVs.Defense, p.IVs.Special, p.IVs.Speed},
		EVs:            [5]int{p.EVs.HP, p.EVs.Attack, p.EVs.Defense, p.EVs.Special, p.EVs.Speed},
		MaxHP:          p.MaxHP,
		CurHP:          p.CurHP,
		Attack:         p.Attack,
		Defense:        p.Defense,
		Special:        p.Special,
		Speed:          p.Speed,
		Status:         int(p.Status),
		SleepTurns:     p.SleepTurns,
		BadPoisonTurns: p.BadPoisonTurns,
		ConfusionTurns: p.ConfusionTurns,
		IsSeeded:       p.IsSeeded,
		SubstituteHP:   p.SubstituteHP,
		DireHit:        p.DireHit,
		GuardSpec:      p.GuardSpec,
		AtkStage:       p.AtkStage,
		DefStage:       p.DefStage,
		SpcStage:       p.SpcStage,
		SpdStage:       p.SpdStage,
		AccStage:       p.AccStage,
		EvaStage:       p.EvaStage,
		CatchRate:      p.CatchRate,
		BaseSpeed:      p.BaseSpeed,
		BaseExp:        p.BaseExp,
		GrowthRt:       int(p.GrowthRt),
		Exp:            p.Exp,
	}
	for i, m := range p.Moves {
		pp.Moves[i] = persistedMove{
			ID:         m.ID,
			Name:       m.Name,
			Type:       int(m.Type),
			Power:      m.Power,
			Accuracy:   m.Accuracy,
			PP:         m.PP,
			MaxPP:      m.MaxPP,
			BasePP:     m.BasePP,
			PPUps:      m.PPUps,
			Effect:     m.Effect,
			IsHighCrit: m.IsHighCrit,
		}
	}
	return pp
}

func persistedToPokemon(pp persistedPokemon) *Pokemon {
	p := &Pokemon{
		ID:     pp.ID,
		Name:   pp.Name,
		Level:  pp.Level,
		IsWild: pp.IsWild,
		Type1:  PokemonType(pp.Type1),
		Type2:  PokemonType(pp.Type2),
		BaseStats: BaseStats{
			HP:      pp.BaseStats[0],
			Attack:  pp.BaseStats[1],
			Defense: pp.BaseStats[2],
			Special: pp.BaseStats[3],
			Speed:   pp.BaseStats[4],
		},
		IVs: IVs{
			Attack:  pp.IVs[0],
			Defense: pp.IVs[1],
			Special: pp.IVs[2],
			Speed:   pp.IVs[3],
		},
		EVs: EVs{
			HP:      pp.EVs[0],
			Attack:  pp.EVs[1],
			Defense: pp.EVs[2],
			Special: pp.EVs[3],
			Speed:   pp.EVs[4],
		},
		MaxHP:          pp.MaxHP,
		CurHP:          pp.CurHP,
		Attack:         pp.Attack,
		Defense:        pp.Defense,
		Special:        pp.Special,
		Speed:          pp.Speed,
		Status:         StatusCondition(pp.Status),
		SleepTurns:     pp.SleepTurns,
		BadPoisonTurns: pp.BadPoisonTurns,
		ConfusionTurns: pp.ConfusionTurns,
		IsSeeded:       pp.IsSeeded,
		SubstituteHP:   pp.SubstituteHP,
		DireHit:        pp.DireHit,
		GuardSpec:      pp.GuardSpec,
		AtkStage:       pp.AtkStage,
		DefStage:       pp.DefStage,
		SpcStage:       pp.SpcStage,
		SpdStage:       pp.SpdStage,
		AccStage:       pp.AccStage,
		EvaStage:       pp.EvaStage,
		CatchRate:      pp.CatchRate,
		BaseSpeed:      pp.BaseSpeed,
		BaseExp:        pp.BaseExp,
		GrowthRt:       GrowthRate(pp.GrowthRt),
		Exp:            pp.Exp,
	}
	for i, m := range pp.Moves {
		p.Moves[i] = MoveSlot{
			ID:         m.ID,
			Name:       m.Name,
			Type:       PokemonType(m.Type),
			Power:      m.Power,
			Accuracy:   m.Accuracy,
			PP:         m.PP,
			MaxPP:      m.MaxPP,
			BasePP:     m.BasePP,
			PPUps:      m.PPUps,
			Effect:     m.Effect,
			IsHighCrit: m.IsHighCrit,
		}
		if p.Moves[i].BasePP == 0 {
			p.Moves[i].BasePP = p.Moves[i].MaxPP
		}
	}
	return p
}

// SaveBattleState persists a battle to the database.
func SaveBattleState(db DBTX, characterID int64, battle *BattleState) error {
	data, err := MarshalBattleState(battle)
	if err != nil {
		return fmt.Errorf("marshal battle for char %d: %w", characterID, err)
	}
	_, err = db.Exec(`
		INSERT INTO character_battle_state (character_id, battle_json)
		VALUES ($1, $2)
		ON CONFLICT (character_id) DO UPDATE SET
			battle_json = EXCLUDED.battle_json,
			updated_at = CURRENT_TIMESTAMP`,
		characterID, string(data))
	if err != nil {
		return fmt.Errorf("save battle state for char %d: %w", characterID, err)
	}
	return nil
}

// LoadBattleState loads a saved battle from the database. Returns nil, nil if none exists.
func LoadBattleState(db DBTX, characterID int64) (*BattleState, error) {
	var data string
	err := db.QueryRow(`SELECT battle_json FROM character_battle_state WHERE character_id = $1`, characterID).Scan(&data)
	if err != nil {
		return nil, nil // No saved battle (or error — treat as no battle)
	}
	battle, err := UnmarshalBattleState([]byte(data))
	if err != nil {
		return nil, fmt.Errorf("unmarshal saved battle for char %d: %w", characterID, err)
	}
	return battle, nil
}

// DeleteBattleState removes a saved battle from the database.
func DeleteBattleState(db DBTX, characterID int64) error {
	_, err := db.Exec(`DELETE FROM character_battle_state WHERE character_id = $1`, characterID)
	return err
}
