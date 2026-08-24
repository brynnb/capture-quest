#!/usr/bin/env python3

from __future__ import annotations

import sqlite3
import tempfile
import unittest
from pathlib import Path

import generate_dungeon_hole_warps as generator


class DungeonHoleWarpGeneratorTest(unittest.TestCase):
    def test_parse_dungeon_warp_landings_asm(self) -> None:
        raw = """
DungeonWarpList:
    db SEAFOAM_ISLANDS_B4F, 1
    db VICTORY_ROAD_2F,     2
    db -1 ; end

DungeonWarpData:
    fly_warp SEAFOAM_ISLANDS_B4F,  4, 14
    fly_warp VICTORY_ROAD_2F,     22, 16
"""

        self.assertEqual(
            generator.parse_dungeon_warp_landings_asm(raw),
            [
                generator.DungeonWarpLanding("SEAFOAM_ISLANDS_B4F", 1, 4, 14),
                generator.DungeonWarpLanding("VICTORY_ROAD_2F", 2, 22, 16),
            ],
        )

    def test_parse_direct_dungeon_warp_source_triggers_asm(self) -> None:
        raw = """
ExampleScript:
    ld a, SEAFOAM_ISLANDS_B4F
    ld [wDungeonWarpDestinationMap], a
    ld hl, ExampleHolesCoords
    call IsPlayerOnDungeonWarp

ExampleHolesCoords:
    dbmapcoord  3, 16
    dbmapcoord  6, 16
    db -1 ; end
"""

        triggers = generator.parse_dungeon_warp_source_triggers_asm(
            "SEAFOAM_ISLANDS_B3F",
            "scripts/Example.asm",
            raw,
        )

        self.assertEqual(
            triggers[0],
            generator.DungeonWarpSourceTrigger(
                source_map="SEAFOAM_ISLANDS_B3F",
                destination_map="SEAFOAM_ISLANDS_B4F",
                warp_index=1,
                x=3,
                y=16,
                source_file="scripts/Example.asm",
            ),
        )
        self.assertEqual(len(triggers), 2)

    def test_parse_conditional_dungeon_warp_source_triggers_asm(self) -> None:
        raw = """
PokemonMansion3FDefaultScript:
    ld hl, .holeCoords
    call .isPlayerFallingDownHole
    ld a, [wWhichDungeonWarp]
    and a
    jp z, CheckFightingMapTrainers
    cp $3
    ld a, POKEMON_MANSION_1F
    jr nz, .fellDownHoleTo1F
    ld a, POKEMON_MANSION_2F
.fellDownHoleTo1F
    ld [wDungeonWarpDestinationMap], a
    ret

.holeCoords:
    dbmapcoord 16, 14
    dbmapcoord 17, 14
    dbmapcoord 19, 14
    db -1 ; end
"""

        triggers = generator.parse_dungeon_warp_source_triggers_asm(
            "POKEMON_MANSION_3F",
            "scripts/PokemonMansion3F.asm",
            raw,
        )

        self.assertEqual([trigger.destination_map for trigger in triggers], [
            "POKEMON_MANSION_1F",
            "POKEMON_MANSION_1F",
            "POKEMON_MANSION_2F",
        ])
        self.assertEqual(
            triggers[2],
            generator.DungeonWarpSourceTrigger(
                source_map="POKEMON_MANSION_3F",
                destination_map="POKEMON_MANSION_2F",
                warp_index=3,
                x=19,
                y=14,
                source_file="scripts/PokemonMansion3F.asm",
            ),
        )

    def test_write_dungeon_hole_warp_seeds(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            db_path = Path(tmp) / "pokemon.db"
            with sqlite3.connect(db_path) as conn:
                conn.execute("CREATE TABLE maps (id INTEGER PRIMARY KEY, name TEXT NOT NULL)")
                conn.executemany(
                    "INSERT INTO maps (id, name) VALUES (?, ?)",
                    [
                        (161, "SEAFOAM_ISLANDS_B3F"),
                        (162, "SEAFOAM_ISLANDS_B4F"),
                    ],
                )

            generator.write_dungeon_hole_warp_seeds(
                db_path,
                [
                    generator.DungeonHoleWarpSeed(
                        source_map="SEAFOAM_ISLANDS_B3F",
                        source_x=3,
                        source_y=16,
                        destination_map="SEAFOAM_ISLANDS_B4F",
                        destination_x=4,
                        destination_y=14,
                        source_file="scripts/SeafoamIslandsB3F.asm",
                        destination_warp_index=1,
                    )
                ],
            )

            with sqlite3.connect(db_path) as conn:
                row = conn.execute(
                    """
                    SELECT source_map, source_x, source_y,
                           destination_map, destination_x, destination_y,
                           destination_warp_index, source_file
                    FROM script_event_dungeon_hole_warps
                    """
                ).fetchone()

        self.assertEqual(
            row,
            (
                "SEAFOAM_ISLANDS_B3F",
                3,
                16,
                "SEAFOAM_ISLANDS_B4F",
                4,
                14,
                1,
                "scripts/SeafoamIslandsB3F.asm",
            ),
        )


if __name__ == "__main__":
    unittest.main()
