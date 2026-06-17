package world

import (
	"encoding/json"

	"capturequest/internal/pokebattle"
)

const pokemonTower7FMapName = "POKEMON_TOWER_7F"

type pokemonTower7FExitCase struct {
	x         int
	y         int
	movements []string
}

type pokemonTower7FRocketExit struct {
	actor        string
	textConstant string
	cases        []pokemonTower7FExitCase
}

var pokemonTower7FRocketExits = map[string]pokemonTower7FRocketExit{
	"EVENT_BEAT_POKEMONTOWER_7_TRAINER_0": {
		actor:        "ROCKET_1",
		textConstant: "TEXT_POKEMONTOWER7F_ROCKET1",
		cases: []pokemonTower7FExitCase{
			{x: 9, y: 12, movements: []string{"RIGHT", "DOWN", "DOWN", "DOWN", "DOWN", "DOWN", "LEFT"}},
			{x: 10, y: 11, movements: []string{"DOWN", "RIGHT", "DOWN", "DOWN", "DOWN", "DOWN"}},
			{x: 11, y: 11, movements: []string{"DOWN", "DOWN", "DOWN", "DOWN", "DOWN"}},
			{x: 12, y: 11, movements: []string{"DOWN", "DOWN", "DOWN", "DOWN", "DOWN"}},
		},
	},
	"EVENT_BEAT_POKEMONTOWER_7_TRAINER_1": {
		actor:        "ROCKET_2",
		textConstant: "TEXT_POKEMONTOWER7F_ROCKET2",
		cases: []pokemonTower7FExitCase{
			{x: 12, y: 10, movements: []string{"LEFT", "DOWN", "DOWN", "DOWN", "DOWN", "DOWN", "DOWN"}},
			{x: 11, y: 9, movements: []string{"DOWN", "DOWN", "DOWN", "LEFT", "DOWN", "DOWN"}},
			{x: 10, y: 9, movements: []string{"DOWN", "DOWN", "DOWN", "DOWN", "DOWN"}},
			{x: 9, y: 9, movements: []string{"DOWN", "DOWN", "DOWN", "DOWN", "DOWN"}},
		},
	},
	"EVENT_BEAT_POKEMONTOWER_7_TRAINER_2": {
		actor:        "ROCKET_3",
		textConstant: "TEXT_POKEMONTOWER7F_ROCKET3",
		cases: []pokemonTower7FExitCase{
			{x: 9, y: 8, movements: []string{"RIGHT", "DOWN", "DOWN", "DOWN", "DOWN", "DOWN", "DOWN"}},
			{x: 10, y: 7, movements: []string{"DOWN", "DOWN", "DOWN", "DOWN", "DOWN"}},
			{x: 11, y: 7, movements: []string{"DOWN", "DOWN", "DOWN", "DOWN", "DOWN"}},
			{x: 12, y: 7, movements: []string{"DOWN", "DOWN", "DOWN", "DOWN", "DOWN"}},
		},
	},
}

func applyPokemonTower7FPostWinMetadata(meta *pokebattle.TrainerMeta, trainer *trainerSightData, playerX, playerY int) {
	if meta == nil {
		return
	}
	mapName, actions := pokemonTower7FPostWinActions(trainer, playerX, playerY)
	if len(actions) == 0 {
		return
	}
	meta.PostWinMapName = mapName
	meta.PostWinActions = actions
}

func pokemonTower7FPostWinActions(trainer *trainerSightData, playerX, playerY int) (string, json.RawMessage) {
	if trainer == nil || trainer.MapID != 148 {
		return "", nil
	}
	exit, ok := pokemonTower7FRocketExits[trainer.EventFlag]
	if !ok {
		return "", nil
	}

	actions := make([]CutsceneAction, 0, 2)
	for _, candidate := range exit.cases {
		if candidate.x == playerX && candidate.y == playerY {
			actions = append(actions, CutsceneAction{
				Type:      "move",
				Actor:     exit.actor,
				Movements: candidate.movements,
			})
			break
		}
	}
	actions = append(actions, CutsceneAction{
		Type:         "hideObject",
		TextConstant: exit.textConstant,
	})

	encoded, err := json.Marshal(actions)
	if err != nil {
		return "", nil
	}
	return pokemonTower7FMapName, encoded
}
