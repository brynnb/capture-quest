// Package content owns static content query projections, independently of sessions
// and transport framing. Queries use an explicit database and bounded context.
package content

import (
	"context"
	"database/sql"
	"time"

	"capturequest/internal/protocol"
)

type Service struct{ database *sql.DB }

func New(database *sql.DB) *Service { return &Service{database: database} }

func (s *Service) Pokemon(ctx context.Context, id int) (protocol.PhaserPokemonFull, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var p protocol.PhaserPokemonFull
	err := s.database.QueryRowContext(ctx, `
		SELECT id, name, hp, atk, def, spd, spc, type_1, type_2, catch_rate, base_exp,
			default_move_1_id, default_move_2_id, default_move_3_id, default_move_4_id,
			base_cry, cry_pitch, cry_length, pokedex_type, height, weight, pokedex_text,
			evolve_level, evolve_pokemon, evolves_from_trade, icon_image, palette_type
		FROM phaser_pokemon WHERE id = $1`, id).Scan(
		&p.ID, &p.Name, &p.HP, &p.Atk, &p.Def, &p.Spd, &p.Spc, &p.Type1, &p.Type2, &p.CatchRate, &p.BaseExp,
		&p.DefaultMove1, &p.DefaultMove2, &p.DefaultMove3, &p.DefaultMove4,
		&p.BaseCry, &p.CryPitch, &p.CryLength, &p.PokedexType, &p.Height, &p.Weight, &p.PokedexText,
		&p.EvolveLevel, &p.EvolvePokemon, &p.EvolvesFromTrade, &p.IconImage, &p.PaletteType)
	return p, err
}

func (s *Service) Move(ctx context.Context, id int) (protocol.PhaserMoveFull, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var m protocol.PhaserMoveFull
	err := s.database.QueryRowContext(ctx, `
		SELECT id, name, short_name, effect, power, type, accuracy, pp, battle_animation, battle_sound, is_hm, field_move_effect
		FROM phaser_moves WHERE id = $1`, id).Scan(
		&m.ID, &m.Name, &m.ShortName, &m.Effect, &m.Power, &m.Type, &m.Accuracy, &m.PP,
		&m.BattleAnimation, &m.BattleSound, &m.IsHM, &m.FieldMoveEffect)
	return m, err
}

func (s *Service) Item(ctx context.Context, id int) (protocol.PhaserItemFull, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var item protocol.PhaserItemFull
	err := s.database.QueryRowContext(ctx, `
		SELECT id, name, short_name, price, is_usable, uses_party_menu, vending_price, move_id, is_guard_drink, is_key_item
		FROM phaser_items WHERE id = $1`, id).Scan(
		&item.ID, &item.Name, &item.ShortName, &item.Price, &item.IsUsable, &item.UsesPartyMenu,
		&item.VendingPrice, &item.MoveID, &item.IsGuardDrink, &item.IsKeyItem)
	return item, err
}
