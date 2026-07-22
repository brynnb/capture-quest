#!/usr/bin/env python3
"""Generate source-derived dungeon hole warp rows into the extractor SQLite DB."""

from __future__ import annotations

import argparse
import re
import sqlite3
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class DungeonWarpLanding:
    destination_map: str
    warp_index: int
    x: int
    y: int


@dataclass(frozen=True)
class DungeonWarpSourceTrigger:
    source_map: str
    destination_map: str
    warp_index: int
    x: int
    y: int
    source_file: str


@dataclass(frozen=True)
class DungeonHoleWarpSeed:
    source_map: str
    source_x: int
    source_y: int
    destination_map: str
    destination_x: int
    destination_y: int
    source_file: str
    destination_warp_index: int


@dataclass(frozen=True)
class AsmPoint:
    x: int
    y: int


@dataclass(frozen=True)
class AsmCoordinateBlock:
    label: str
    line: int
    points: tuple[AsmPoint, ...]


def clean_asm_line(line: str) -> str:
    return line.split(";", 1)[0].strip()


def clean_asm_lines(raw: str) -> list[str]:
    return [clean_asm_line(line) for line in raw.splitlines()]


def parse_asm_call(line: str, name: str) -> list[str] | None:
    line = line.strip()
    if line != name and not line.startswith(f"{name} ") and not line.startswith(f"{name}\t"):
        return None
    rest = line.removeprefix(name).strip()
    if not rest:
        return []
    return [part.strip() for part in rest.split(",")]


def parse_asm_int(value: str) -> int:
    value = value.strip().strip("()")
    if not value:
        raise ValueError("empty asm int")
    if value.startswith("$"):
        return int(value[1:], 16)
    return int(value, 10)


def camel_to_upper_snake(value: str) -> str:
    value = re.sub(r"[^A-Za-z0-9]+", "_", value.strip())
    value = re.sub(r"([A-Z]+)([A-Z][a-z])", r"\1_\2", value)
    value = re.sub(r"([a-z0-9])([A-Z])", r"\1_\2", value)
    value = re.sub(r"([A-Za-z])([0-9])", r"\1_\2", value)
    value = value.strip("_").upper()
    value = re.sub(r"_((?:B)?\d+)_F\b", r"_\1F", value)
    return re.sub(r"_B_(\d+)F\b", r"_B\1F", value)


def normalize_map_name(value: str) -> str:
    return re.sub(r"[^A-Za-z0-9]+", "", value).lower()


def parse_dungeon_warp_landings_asm(raw: str) -> list[DungeonWarpLanding]:
    warp_list: list[tuple[str, int]] = []
    warp_data: list[tuple[str, int, int]] = []
    section = ""

    for raw_line in raw.splitlines():
        line = clean_asm_line(raw_line)
        if line == "DungeonWarpList:":
            section = "list"
            continue
        if line == "DungeonWarpData:":
            section = "data"
            continue
        if not line:
            continue

        if section == "list":
            if line.startswith("db -1"):
                section = ""
                continue
            args = parse_asm_call(line, "db")
            if args is None or len(args) != 2:
                continue
            warp_list.append((camel_to_upper_snake(args[0]), parse_asm_int(args[1])))
        elif section == "data":
            if warp_list and len(warp_data) == len(warp_list):
                section = ""
                continue
            args = parse_asm_call(line, "fly_warp")
            if args is None or len(args) != 3:
                continue
            warp_data.append((camel_to_upper_snake(args[0]), parse_asm_int(args[1]), parse_asm_int(args[2])))

    if not warp_list:
        raise ValueError("DungeonWarpList not found")
    if len(warp_data) != len(warp_list):
        raise ValueError(f"DungeonWarpData entries={len(warp_data)}, want {len(warp_list)}")

    landings: list[DungeonWarpLanding] = []
    for index, (list_entry, data_entry) in enumerate(zip(warp_list, warp_data)):
        list_map, warp_index = list_entry
        data_map, x, y = data_entry
        if normalize_map_name(list_map) != normalize_map_name(data_map):
            raise ValueError(f"dungeon warp list/data map mismatch at {index}: {list_map} vs {data_map}")
        landings.append(DungeonWarpLanding(list_map, warp_index, x, y))
    return landings


def previous_asm_label(lines: list[str], before: int) -> str | None:
    for index in range(before - 1, -1, -1):
        line = lines[index]
        if not line:
            continue
        if re.search(r"\s", line):
            return None
        return line.removesuffix(":")
    return None


def parse_asm_coordinate_blocks(lines: list[str]) -> list[AsmCoordinateBlock]:
    blocks: list[AsmCoordinateBlock] = []
    index = 0
    while index < len(lines):
        if not lines[index].startswith("dbmapcoord "):
            index += 1
            continue
        label = previous_asm_label(lines, index)
        if label is None:
            index += 1
            continue
        start_line = index
        points: list[AsmPoint] = []
        while index < len(lines):
            line = lines[index]
            if line.startswith("db -1"):
                break
            args = parse_asm_call(line, "dbmapcoord")
            if args is not None and len(args) == 2:
                points.append(AsmPoint(parse_asm_int(args[0]), parse_asm_int(args[1])))
            index += 1
        if points:
            blocks.append(AsmCoordinateBlock(label, start_line, tuple(points)))
        index += 1
    return blocks


def find_asm_coordinate_block(
    blocks: list[AsmCoordinateBlock],
    label: str,
    reference_line: int,
) -> AsmCoordinateBlock | None:
    label = label.removesuffix(":")
    matches = [block for block in blocks if block.label == label]
    if not matches:
        return None
    return min(matches, key=lambda block: abs(block.line - reference_line))


def find_previous_ld_hl(lines: list[str], before: int, max_distance: int) -> str | None:
    for index in range(before - 1, -1, -1):
        if before - index > max_distance:
            break
        if lines[index].startswith("ld hl,"):
            return lines[index].removeprefix("ld hl,").strip()
    return None


def parse_asm_load_a_identifier(line: str) -> str | None:
    if not line.startswith("ld a,"):
        return None
    operand = line.removeprefix("ld a,").strip()
    if (
        not operand
        or any(char in operand for char in "[]$")
        or operand.lower() == "a"
        or " " in operand
    ):
        return None
    try:
        parse_asm_int(operand)
    except ValueError:
        return operand
    return None


def find_previous_dungeon_destination_map(lines: list[str], before: int, max_distance: int) -> str | None:
    for index in range(before - 1, -1, -1):
        if before - index > max_distance:
            break
        if lines[index] != "ld [wDungeonWarpDestinationMap], a":
            continue
        for load_index in range(index - 1, -1, -1):
            if before - load_index > max_distance:
                break
            operand = parse_asm_load_a_identifier(lines[load_index])
            if operand:
                return camel_to_upper_snake(operand)
    return None


def parse_direct_dungeon_warp_triggers(
    source_map: str,
    source_file: str,
    lines: list[str],
    coordinate_blocks: list[AsmCoordinateBlock],
) -> list[DungeonWarpSourceTrigger]:
    triggers: list[DungeonWarpSourceTrigger] = []
    for index, line in enumerate(lines):
        if "IsPlayerOnDungeonWarp" not in line:
            continue
        label = find_previous_ld_hl(lines, index, 8)
        destination_map = find_previous_dungeon_destination_map(lines, index, 12)
        if label is None or destination_map is None:
            continue
        block = find_asm_coordinate_block(coordinate_blocks, label, index)
        if block is None:
            continue
        for point_index, point in enumerate(block.points, start=1):
            triggers.append(
                DungeonWarpSourceTrigger(
                    source_map=source_map,
                    destination_map=destination_map,
                    warp_index=point_index,
                    x=point.x,
                    y=point.y,
                    source_file=source_file,
                )
            )
    return triggers


def parse_conditional_dungeon_warp_triggers(
    source_map: str,
    source_file: str,
    lines: list[str],
    coordinate_blocks: list[AsmCoordinateBlock],
) -> list[DungeonWarpSourceTrigger]:
    triggers: list[DungeonWarpSourceTrigger] = []
    for index, line in enumerate(lines):
        if not line.startswith("call ") or "isPlayerFallingDownHole" not in line:
            continue
        label = find_previous_ld_hl(lines, index, 4)
        if label is None:
            continue
        block = find_asm_coordinate_block(coordinate_blocks, label, index)
        if block is None:
            continue

        compare_index = 0
        default_map = ""
        branch_map = ""
        after_branch = False
        for current in lines[index + 1 : index + 21]:
            if current.startswith("cp ") and compare_index == 0:
                compare_index = parse_asm_int(current.removeprefix("cp ").strip())
                continue
            if current.startswith("jr nz,") and default_map:
                after_branch = True
                continue
            operand = parse_asm_load_a_identifier(current)
            if operand:
                if after_branch:
                    branch_map = camel_to_upper_snake(operand)
                elif compare_index > 0 and not default_map:
                    default_map = camel_to_upper_snake(operand)
                continue
            if current == "ld [wDungeonWarpDestinationMap], a":
                break

        if compare_index == 0 or not default_map or not branch_map:
            continue
        for point_index, point in enumerate(block.points, start=1):
            destination_map = branch_map if point_index == compare_index else default_map
            triggers.append(
                DungeonWarpSourceTrigger(
                    source_map=source_map,
                    destination_map=destination_map,
                    warp_index=point_index,
                    x=point.x,
                    y=point.y,
                    source_file=source_file,
                )
            )
    return triggers


def parse_dungeon_warp_source_triggers_asm(
    source_map: str,
    source_file: str,
    raw: str,
) -> list[DungeonWarpSourceTrigger]:
    lines = clean_asm_lines(raw)
    coordinate_blocks = parse_asm_coordinate_blocks(lines)
    return [
        *parse_direct_dungeon_warp_triggers(source_map, source_file, lines, coordinate_blocks),
        *parse_conditional_dungeon_warp_triggers(source_map, source_file, lines, coordinate_blocks),
    ]


def parse_dungeon_warp_source_triggers_dir(scripts_dir: Path) -> list[DungeonWarpSourceTrigger]:
    triggers: list[DungeonWarpSourceTrigger] = []
    for script_path in sorted(scripts_dir.glob("*.asm")):
        source_map = camel_to_upper_snake(script_path.stem)
        source_file = f"scripts/{script_path.name}"
        triggers.extend(parse_dungeon_warp_source_triggers_asm(source_map, source_file, script_path.read_text()))
    return sorted(triggers, key=lambda row: (row.source_map, row.y, row.x))


def load_dungeon_hole_warp_seeds(game_data_root: Path) -> list[DungeonHoleWarpSeed]:
    special_warps_path = game_data_root / "data" / "maps" / "special_warps.asm"
    scripts_dir = game_data_root / "scripts"
    if not special_warps_path.exists():
        raise FileNotFoundError(special_warps_path)
    if not scripts_dir.is_dir():
        raise FileNotFoundError(scripts_dir)

    landings = parse_dungeon_warp_landings_asm(special_warps_path.read_text())
    triggers = parse_dungeon_warp_source_triggers_dir(scripts_dir)
    landings_by_key = {
        (normalize_map_name(landing.destination_map), landing.warp_index): landing
        for landing in landings
    }

    seeds: list[DungeonHoleWarpSeed] = []
    seen: set[tuple[str, int, int]] = set()
    for trigger in triggers:
        landing = landings_by_key.get((normalize_map_name(trigger.destination_map), trigger.warp_index))
        if landing is None:
            continue
        key = (normalize_map_name(trigger.source_map), trigger.x, trigger.y)
        if key in seen:
            continue
        seen.add(key)
        seeds.append(
            DungeonHoleWarpSeed(
                source_map=trigger.source_map,
                source_x=trigger.x,
                source_y=trigger.y,
                destination_map=landing.destination_map,
                destination_x=landing.x,
                destination_y=landing.y,
                source_file=trigger.source_file,
                destination_warp_index=trigger.warp_index,
            )
        )
    return sorted(seeds, key=lambda row: (row.source_map, row.source_y, row.source_x))


def load_known_maps(conn: sqlite3.Connection) -> set[str]:
    return {
        normalize_map_name(row[0])
        for row in conn.execute("SELECT name FROM maps")
    }


def validate_seed_maps(conn: sqlite3.Connection, seeds: list[DungeonHoleWarpSeed]) -> None:
    known_maps = load_known_maps(conn)
    missing = sorted(
        {
            seed.source_map
            for seed in seeds
            if normalize_map_name(seed.source_map) not in known_maps
        }
        | {
            seed.destination_map
            for seed in seeds
            if normalize_map_name(seed.destination_map) not in known_maps
        }
    )
    if missing:
        preview = ", ".join(missing[:12])
        raise ValueError(f"Generated dungeon hole warps reference unknown maps: {preview}")


def write_dungeon_hole_warp_seeds(db_path: Path, seeds: list[DungeonHoleWarpSeed]) -> None:
    if not seeds:
        raise ValueError("no dungeon hole warp seeds generated from source asm")

    with sqlite3.connect(db_path) as conn:
        validate_seed_maps(conn, seeds)
        conn.execute("DROP TABLE IF EXISTS script_event_dungeon_hole_warps")
        conn.execute(
            """
            CREATE TABLE script_event_dungeon_hole_warps (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                source_map TEXT NOT NULL,
                source_x INTEGER NOT NULL,
                source_y INTEGER NOT NULL,
                destination_map TEXT NOT NULL,
                destination_x INTEGER NOT NULL,
                destination_y INTEGER NOT NULL,
                destination_warp_index INTEGER NOT NULL,
                source_file TEXT NOT NULL,
                UNIQUE(source_map, source_x, source_y)
            )
            """
        )
        conn.executemany(
            """
            INSERT INTO script_event_dungeon_hole_warps (
                source_map, source_x, source_y,
                destination_map, destination_x, destination_y,
                destination_warp_index, source_file
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            """,
            [
                (
                    seed.source_map,
                    seed.source_x,
                    seed.source_y,
                    seed.destination_map,
                    seed.destination_x,
                    seed.destination_y,
                    seed.destination_warp_index,
                    seed.source_file,
                )
                for seed in seeds
            ],
        )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--extractor-root", type=Path, required=True)
    parser.add_argument("--sqlite", type=Path)
    args = parser.parse_args()

    extractor_root = args.extractor_root.resolve()
    db_path = (args.sqlite or extractor_root / "pokemon.db").resolve()
    game_data_root = extractor_root / "pokemon-game-data"

    seeds = load_dungeon_hole_warp_seeds(game_data_root)
    write_dungeon_hole_warp_seeds(db_path, seeds)
    print(f"Generated {len(seeds)} dungeon hole warps into {db_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
