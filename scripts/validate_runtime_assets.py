#!/usr/bin/env python3
"""Fail fast when the runtime SQLite database and tile PNG set diverge."""

from __future__ import annotations

import hashlib
import json
import sqlite3
from pathlib import Path

from sync_extractor_assets import PROCEDURAL_TILE_HASHES


REPO_ROOT = Path(__file__).resolve().parent.parent
PHASER_ROOT = REPO_ROOT / "public/phaser"
DATABASE = PHASER_ROOT / "pokemon.db"
TILE_DIRECTORY = PHASER_ROOT / "tile_images"
CONTRACT = PHASER_ROOT / "runtime_asset_contract.json"
PROCEDURAL_PALETTE = REPO_ROOT / "src/constants/procedural_tile_palette.json"


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def tile_catalog_sha256() -> tuple[int, str]:
    digest = hashlib.sha256()
    files = sorted(TILE_DIRECTORY.glob("tile_*.png"))
    for tile_file in files:
        digest.update(tile_file.name.encode("utf-8"))
        digest.update(b"\0")
        digest.update(bytes.fromhex(sha256_file(tile_file)))
    return len(files), digest.hexdigest()


def main() -> None:
    if not CONTRACT.is_file() or not PROCEDURAL_PALETTE.is_file():
        raise SystemExit(
            "Runtime asset contract or procedural palette is missing. "
            "Run npm run bootstrap:assets."
        )
    contract = json.loads(CONTRACT.read_text(encoding="utf-8"))
    if sha256_file(DATABASE) != contract["pokemonDbSha256"]:
        raise SystemExit(
            "public/phaser/pokemon.db does not match its tile asset contract. "
            "Run npm run bootstrap:assets."
        )

    with sqlite3.connect(DATABASE) as conn:
        row_count, min_id, max_id = conn.execute(
            "SELECT COUNT(*), MIN(id), MAX(id) FROM tile_images"
        ).fetchone()
        schema_name, schema_version = conn.execute(
            "SELECT schema_name, schema_version FROM schema_metadata LIMIT 1"
        ).fetchone()
        tile_hashes = dict(conn.execute("SELECT id, image_hash FROM tile_images"))

    tile_count, tile_digest = tile_catalog_sha256()
    expected = contract["tileCount"]
    if (
        row_count != expected
        or min_id != 1
        or max_id != expected
        or tile_count != expected
        or tile_digest != contract["tileCatalogSha256"]
        or schema_name != contract["schemaName"]
        or schema_version != contract["schemaVersion"]
    ):
        raise SystemExit(
            "Runtime tile images do not match public/phaser/pokemon.db. "
            "Run npm run bootstrap:assets."
        )

    palette = json.loads(PROCEDURAL_PALETTE.read_text(encoding="utf-8"))
    for block_index, positions in PROCEDURAL_TILE_HASHES.items():
        for position, expected_hash in positions.items():
            image_id = palette.get(block_index, {}).get(position)
            if tile_hashes.get(image_id) != expected_hash:
                raise SystemExit(
                    "The procedural terrain palette does not match the runtime "
                    f"tile catalog at block {block_index}, position {position}. "
                    "Run npm run bootstrap:assets."
                )
    print(
        f"Runtime asset contract OK: {schema_name} v{schema_version}, "
        f"{tile_count} tile images."
    )


if __name__ == "__main__":
    main()
