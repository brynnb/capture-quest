#!/usr/bin/env python3
"""Copy generated extractor outputs into CaptureQuest runtime asset folders."""

from __future__ import annotations

import argparse
import json
import os
import shutil
import sqlite3
import urllib.error
import urllib.request
import warnings
from pathlib import Path

from PIL import Image

warnings.filterwarnings("ignore", category=DeprecationWarning)


CAPTUREQUEST_ONLY_PHASER_SPRITES = {
    "bluenb.png",
    "cavern_sign.png",
    "forest_sign.png",
    "red_surf.png",
    "red_surf_original.png",
    "sign.png",
}


def clean_dir(path: Path, keep_names: set[str] | None = None) -> None:
    keep_names = keep_names or set()
    path.mkdir(parents=True, exist_ok=True)
    for child in path.iterdir():
        if child.name in keep_names:
            continue
        if child.is_dir():
            shutil.rmtree(child)
        else:
            child.unlink()


def copy_file(source: Path, destination: Path) -> None:
    if not source.exists():
        raise FileNotFoundError(source)
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, destination)


def copy_tree_contents(source: Path, destination: Path) -> int:
    if not source.exists():
        raise FileNotFoundError(source)
    count = 0
    for source_file in sorted(source.rglob("*")):
        if not source_file.is_file():
            continue
        relative = source_file.relative_to(source)
        copy_file(source_file, destination / relative)
        count += 1
    return count


def make_color_transparent(path: Path, color: tuple[int, int, int]) -> None:
    image = Image.open(path).convert("RGBA")
    pixels = []
    for red, green, blue, alpha in image.getdata():
        if (red, green, blue) == color:
            pixels.append((red, green, blue, 0))
        else:
            pixels.append((red, green, blue, alpha))
    image.putdata(pixels)
    image.save(path)


def make_border_color_transparent(path: Path, color: tuple[int, int, int]) -> None:
    image = Image.open(path).convert("RGBA")
    width, height = image.size
    pixels = image.load()
    stack = []
    seen = set()

    for x in range(width):
        stack.append((x, 0))
        stack.append((x, height - 1))
    for y in range(height):
        stack.append((0, y))
        stack.append((width - 1, y))

    while stack:
        x, y = stack.pop()
        if (x, y) in seen or not (0 <= x < width and 0 <= y < height):
            continue
        seen.add((x, y))
        red, green, blue, alpha = pixels[x, y]
        if alpha == 0 or (red, green, blue) != color:
            continue
        pixels[x, y] = (0, 0, 0, 0)
        stack.extend(((x + 1, y), (x - 1, y), (x, y + 1), (x, y - 1)))

    image.save(path)


def save_centered_transparent_sprite(source: Path, destination: Path, canvas_size: int = 96) -> None:
    image = Image.open(source).convert("RGBA")
    pixels = []
    for red, green, blue, alpha in image.getdata():
        if (red, green, blue) == (255, 255, 255):
            pixels.append((0, 0, 0, 0))
        else:
            pixels.append((red, green, blue, alpha))
    image.putdata(pixels)

    bbox = image.getbbox()
    if bbox:
        image = image.crop(bbox)

    canvas = Image.new("RGBA", (canvas_size, canvas_size), (0, 0, 0, 0))
    x = (canvas_size - image.width) // 2
    y = (canvas_size - image.height) // 2
    canvas.paste(image, (x, y), image)
    destination.parent.mkdir(parents=True, exist_ok=True)
    canvas.save(destination)


def source_stem_candidates(name: str) -> list[str]:
    normalized = name.lower()
    candidates = [
        normalized,
        normalized.replace("_", ""),
        normalized.replace("_", "."),
        normalized.replace("_f", "f").replace("_m", "m"),
        normalized.replace("female", "f").replace("male", "m"),
    ]
    if normalized == "mr_mime":
        candidates.append("mr.mime")

    unique: list[str] = []
    for candidate in candidates:
        if candidate not in unique:
            unique.append(candidate)
    return unique


def first_existing_png(directory: Path, stems: list[str], suffix: str = "") -> Path | None:
    for stem in stems:
        candidate = directory / f"{stem}{suffix}.png"
        if candidate.exists():
            return candidate
    return None


def copy_pokemon_battle_sprites(extractor_root: Path, repo_root: Path) -> None:
    db_path = extractor_root / "pokemon.db"
    front_source = extractor_root / "pokemon-game-data/gfx/pokemon/front"
    back_source = extractor_root / "pokemon-game-data/gfx/pokemon/back"
    front_dest = repo_root / "public/assets/pokemon/front"
    back_dest = repo_root / "public/assets/pokemon/back"
    fetch_pokeapi = os.environ.get("CAPTUREQUEST_FETCH_POKEAPI_SPRITES", "1").lower()
    fetch_pokeapi = fetch_pokeapi not in {"0", "false", "no"}

    clean_dir(front_dest)
    clean_dir(back_dest)

    conn = sqlite3.connect(db_path)
    try:
        rows = conn.execute("SELECT id, name FROM pokemon ORDER BY id").fetchall()
    finally:
        conn.close()

    missing: list[str] = []
    pokeapi_copied = 0
    extractor_copied = 0
    copied = 0
    for pokemon_id, pokemon_name in rows:
        front_output = front_dest / f"{pokemon_id}.png"
        back_output = back_dest / f"{pokemon_id}.png"
        if fetch_pokeapi and copy_pokeapi_battle_sprite(pokemon_id, "front", front_output):
            pokeapi_copied += 1
        else:
            stems = source_stem_candidates(str(pokemon_name))
            front = first_existing_png(front_source, stems)
            if front is None:
                missing.append(f"{pokemon_id}:{pokemon_name}:front")
            else:
                save_centered_transparent_sprite(front, front_output)
                extractor_copied += 1

        if fetch_pokeapi and copy_pokeapi_battle_sprite(pokemon_id, "back", back_output):
            pokeapi_copied += 1
        else:
            stems = source_stem_candidates(str(pokemon_name))
            back = first_existing_png(back_source, stems, "b")
            if back is None:
                missing.append(f"{pokemon_id}:{pokemon_name}:back")
            else:
                save_centered_transparent_sprite(back, back_output)
                extractor_copied += 1

        if front_output.exists() and back_output.exists():
            copied += 2

    if missing:
        preview = ", ".join(missing[:8])
        raise RuntimeError(f"Missing Pokemon battle sprites: {preview}")

    print(
        f"Copied {copied} Pokemon battle sprites "
        f"({pokeapi_copied} from PokeAPI, {extractor_copied} from extractor fallback)."
    )


def copy_pokeapi_battle_sprite(pokemon_id: int, side: str, destination: Path) -> bool:
    if side == "front":
        url = f"https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/{pokemon_id}.png"
    elif side == "back":
        url = f"https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/back/{pokemon_id}.png"
    else:
        raise ValueError(f"unknown sprite side: {side}")

    try:
        with urllib.request.urlopen(url, timeout=15) as response:
            data = response.read()
    except (urllib.error.URLError, TimeoutError):
        return False

    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_bytes(data)
    return True


def copy_trainer_battle_sprites(extractor_root: Path, repo_root: Path) -> None:
    source = extractor_root / "pokemon-game-data/gfx/trainers"
    destination = repo_root / "public/assets/trainers"
    clean_dir(destination)

    copied = 0
    for source_file in sorted(source.glob("*.png")):
        trainer_dest = destination / source_file.name
        copy_file(source_file, trainer_dest)
        make_border_color_transparent(trainer_dest, (255, 255, 255))
        copied += 1

    copy_file(extractor_root / "pokemon-game-data/gfx/player/red.png", destination / "player_front.png")
    make_border_color_transparent(destination / "player_front.png", (255, 255, 255))
    copy_file(extractor_root / "pokemon-game-data/gfx/player/redb.png", destination / "player_back.png")
    make_border_color_transparent(destination / "player_back.png", (255, 255, 255))
    copied += 2
    print(f"Copied {copied} trainer battle sprites.")


def copy_phaser_assets(extractor_root: Path, repo_root: Path) -> None:
    viewer_public = extractor_root / "pokemon-phaser/public"
    phaser_root = repo_root / "public/phaser"

    copy_file(extractor_root / "pokemon.db", phaser_root / "pokemon.db")
    copy_file(viewer_public / "style.css", phaser_root / "style.css")

    clean_dir(phaser_root / "assets")
    copy_tree_contents(viewer_public / "assets", phaser_root / "assets")

    clean_dir(phaser_root / "tile_images")
    tile_count = copy_tree_contents(
        viewer_public / "viewer-assets/tile_images",
        phaser_root / "tile_images",
    )

    sprite_dest = phaser_root / "sprites"
    clean_dir(sprite_dest, CAPTUREQUEST_ONLY_PHASER_SPRITES)
    sprite_count = copy_tree_contents(
        viewer_public / "viewer-assets/sprites",
        sprite_dest,
    )
    for sprite_file in sprite_dest.glob("*.png"):
        if sprite_file.name not in CAPTUREQUEST_ONLY_PHASER_SPRITES:
            make_color_transparent(sprite_file, (255, 255, 255))

    tileset_count = 0
    for source_file in sorted((extractor_root / "pokemon-game-data/gfx/tilesets").glob("*.png")):
        copy_file(source_file, sprite_dest / source_file.name)
        tileset_count += 1

    print(f"Copied {tile_count} tile images.")
    print(f"Copied {sprite_count} overworld sprites and {tileset_count} tilesets.")


def sync_audio_metadata(extractor_root: Path, repo_root: Path) -> None:
    source_manifest = extractor_root / "audio_manifest.json"
    destination_manifest = repo_root / "src/constants/pokemon_audio_manifest.json"

    if source_manifest.exists():
        destination_manifest.parent.mkdir(parents=True, exist_ok=True)
        data = json.loads(source_manifest.read_text(encoding="utf-8"))
        destination_manifest.write_text(
            json.dumps(data, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
        print("Copied Pokemon audio manifest.")
    else:
        fallback_manifest = {
            "mapMusic": [],
            "music": {},
            "sfx": {},
            "pokemonCries": {},
        }
        destination_manifest.parent.mkdir(parents=True, exist_ok=True)
        destination_manifest.write_text(
            json.dumps(fallback_manifest, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
        print("No extractor audio_manifest.json found; wrote empty Pokemon audio manifest.")

    rendered_audio = extractor_root / "rendered-audio/sound/pokemon"
    destination_audio = repo_root / "public/sound/pokemon"
    if rendered_audio.exists():
        clean_dir(destination_audio)
        count = copy_tree_contents(rendered_audio, destination_audio)
        print(f"Copied {count} rendered Pokemon audio files.")
    elif destination_audio.exists() and any(destination_audio.rglob("*.ogg")):
        print("Rendered Pokemon audio already exists in CaptureQuest public/sound/pokemon.")
    else:
        print("No rendered Pokemon audio found.")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--extractor-root", type=Path, required=True)
    parser.add_argument("--repo-root", type=Path, required=True)
    args = parser.parse_args()

    extractor_root = args.extractor_root.resolve()
    repo_root = args.repo_root.resolve()

    copy_phaser_assets(extractor_root, repo_root)
    copy_pokemon_battle_sprites(extractor_root, repo_root)
    copy_trainer_battle_sprites(extractor_root, repo_root)
    sync_audio_metadata(extractor_root, repo_root)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
