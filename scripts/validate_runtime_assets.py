#!/usr/bin/env python3
"""Fail fast when the runtime SQLite database and tile PNG set diverge."""

from __future__ import annotations

import hashlib
import json
import sqlite3
from pathlib import Path

from sync_extractor_assets import (
    CAPTUREQUEST_ONLY_PHASER_SPRITES,
    PROCEDURAL_TILE_HASHES,
)


REPO_ROOT = Path(__file__).resolve().parent.parent
PHASER_ROOT = REPO_ROOT / "public/phaser"
DATABASE = PHASER_ROOT / "pokemon.db"
TILE_DIRECTORY = PHASER_ROOT / "tile_images"
SPRITE_DIRECTORY = PHASER_ROOT / "sprites"
PHASER_STYLE = PHASER_ROOT / "style.css"
CONTRACT = PHASER_ROOT / "runtime_asset_contract.json"
PROCEDURAL_PALETTE = REPO_ROOT / "src/constants/procedural_tile_palette.json"
RUNTIME_ASSET_VERSION = REPO_ROOT / "src/constants/runtime_asset_version.ts"
AUDIO_MANIFEST = REPO_ROOT / "src/constants/audio_manifest.json"
AUDIO_RENDER_MANIFEST = REPO_ROOT / "public/sound/pokemon/audio-render-manifest.json"
EXPECTED_GLOBAL_AUDIO = {
    "/sound/SFX_TURN_ON_PC.mp3",
    "/sound/button_1.mp3",
    "/sound/buttonclick.mp3",
}


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


def validate_browser_audio() -> tuple[int, int]:
    if not AUDIO_MANIFEST.is_file() or not AUDIO_RENDER_MANIFEST.is_file():
        raise SystemExit("Generated audio manifests are missing. Run npm run bootstrap:assets.")

    runtime = json.loads(AUDIO_MANIFEST.read_text(encoding="utf-8"))
    library = runtime.get("library")
    global_audio = runtime.get("global")
    if not isinstance(library, list) or not isinstance(global_audio, list):
        raise SystemExit("Runtime audio manifest has invalid library/global arrays.")
    if set(global_audio) != EXPECTED_GLOBAL_AUDIO:
        raise SystemExit(
            "Runtime audio startup set is not the three bounded UI effects. "
            "Run npm run audio:manifest."
        )
    if any(
        segment in path
        for path in global_audio
        for segment in ("/pokemon/music/", "/pokemon/cries/", "/pokemon/moves/")
    ):
        raise SystemExit("Long-form Pokemon audio must never be globally preloaded.")

    rendered = json.loads(AUDIO_RENDER_MANIFEST.read_text(encoding="utf-8"))
    expected_profile = {
        "sampleRate": 24000,
        "channels": 1,
        "codec": "ogg-vorbis",
        "quality": 1,
    }
    if rendered.get("schemaVersion") != 2:
        raise SystemExit("Browser audio render manifest must use schema version 2.")
    if rendered.get("renderProfile", {}).get("distribution") != expected_profile:
        raise SystemExit("Browser Pokemon audio is not the compact 24 kHz mono profile.")

    artifacts = rendered.get("artifacts")
    if not isinstance(artifacts, list) or len(artifacts) != 561:
        raise SystemExit("Browser audio render manifest must contain 561 assets.")
    distribution_paths = {row.get("distribution", {}).get("path") for row in artifacts}
    if None in distribution_paths or len(distribution_paths) != 561:
        raise SystemExit("Browser audio distribution paths are missing or duplicated.")
    for logical_path in distribution_paths:
        audio_file = REPO_ROOT / "public" / logical_path.removeprefix("/")
        if not audio_file.is_file():
            raise SystemExit(f"Missing browser audio derivative: {audio_file}")
    unexpected_masters = list((REPO_ROOT / "public/sound/pokemon").rglob("*.flac"))
    if unexpected_masters:
        raise SystemExit("Archival FLAC masters must not be published to the web asset tree.")
    if not distribution_paths.issubset(set(library)):
        raise SystemExit("Runtime audio library omits generated Pokemon derivatives.")
    return len(distribution_paths), len(global_audio)


def main() -> None:
    if (
        not CONTRACT.is_file()
        or not PROCEDURAL_PALETTE.is_file()
        or not RUNTIME_ASSET_VERSION.is_file()
        or not PHASER_STYLE.is_file()
        or PHASER_STYLE.stat().st_size == 0
    ):
        raise SystemExit(
            "Runtime asset contract or procedural palette is missing. "
            "Run npm run bootstrap:assets."
        )
    contract = json.loads(CONTRACT.read_text(encoding="utf-8"))
    version_source = RUNTIME_ASSET_VERSION.read_text(encoding="utf-8")
    if f'"{contract["tileCatalogSha256"]}"' not in version_source:
        raise SystemExit(
            "src/constants/runtime_asset_version.ts does not match the runtime "
            "tile catalog. Run npm run bootstrap:assets."
        )
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
        generated_sprite_names = {
            Path(relative_path).name
            for (relative_path,) in conn.execute(
                """
                SELECT ga.relative_path
                FROM graphic_assets ga
                JOIN graphic_categories gc ON gc.id = ga.category_id
                WHERE gc.name IN ('sprites', 'tilesets')
                """
            )
        }

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

    expected_sprite_names = (
        generated_sprite_names | CAPTUREQUEST_ONLY_PHASER_SPRITES
    )
    actual_sprite_names = {
        path.name for path in SPRITE_DIRECTORY.glob("*.png") if path.is_file()
    }
    if actual_sprite_names != expected_sprite_names:
        missing = sorted(expected_sprite_names - actual_sprite_names)
        extra = sorted(actual_sprite_names - expected_sprite_names)
        raise SystemExit(
            "Runtime Phaser sprites do not match public/phaser/pokemon.db "
            f"(missing={missing[:8]}, extra={extra[:8]}). "
            "Run npm run bootstrap:assets."
        )
    audio_count, startup_audio_count = validate_browser_audio()
    print(
        f"Runtime asset contract OK: {schema_name} v{schema_version}, "
        f"{tile_count} tile images, {len(actual_sprite_names)} sprites, "
        f"{audio_count} compact audio files ({startup_audio_count} preloaded)."
    )


if __name__ == "__main__":
    main()
