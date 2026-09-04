#!/usr/bin/env python3
"""Classify a CaptureQuest change into the smallest safe deployment lane."""

from __future__ import annotations

import argparse
import subprocess
from pathlib import Path


FULL_DATA_PREFIXES = (
    "tools/pokemon-gameboy-extractor-tool",
    "scripts/bootstrap_capturequest.sh",
    "scripts/generate_dungeon_hole_warps.py",
    "scripts/generate_audio_manifest.mjs",
    "scripts/sync_extractor_assets.py",
    "scripts/sync_extractor_audio_manifest.mjs",
    "server/cmd/import-phaser/",
    "server/cmd/import-script-candidates/",
    "server/internal/scriptcandidateimport/",
    "server/schema/",
    "server/scripts/",
    "server/scripted_events/",
    "public/phaser/sprites/",
)

BACKEND_PREFIXES = ("server/",)

NON_DEPLOY_PREFIXES = (
    ".github/",
    "docs/",
    "tests/",
)

NON_DEPLOY_FILES = {
    ".gitignore",
    "AGENTS.md",
    "DATA_AND_ASSET_NOTICE.md",
    "DOCUMENTATION.md",
    "LICENSE",
    "MIGRATING.md",
    "README.md",
}


def classify_paths(paths: list[str], requested_mode: str = "auto", reset: bool = False) -> dict[str, str]:
    if requested_mode not in {"auto", "frontend", "backend", "full"}:
        raise ValueError(f"Unsupported deployment mode: {requested_mode}")

    if reset or requested_mode == "full":
        frontend = backend = full = True
    elif requested_mode == "frontend":
        frontend, backend, full = True, False, False
    elif requested_mode == "backend":
        frontend, backend, full = False, True, False
    else:
        full = any(path.startswith(FULL_DATA_PREFIXES) for path in paths)
        backend = full or any(path.startswith(BACKEND_PREFIXES) for path in paths)
        frontend = full or any(
            path not in NON_DEPLOY_FILES
            and not path.startswith(NON_DEPLOY_PREFIXES)
            and not path.startswith(BACKEND_PREFIXES)
            for path in paths
        )

    if full:
        mode = "full"
    elif backend and frontend:
        mode = "code"
    elif backend:
        mode = "backend"
    elif frontend:
        mode = "frontend"
    else:
        mode = "none"

    return {
        "mode": mode,
        "deploy_frontend": str(frontend).lower(),
        "deploy_backend": str(backend).lower(),
        "full_data": str(full).lower(),
        "deploy_needed": str(frontend or backend).lower(),
    }


def changed_paths(before: str, after: str) -> list[str]:
    if not before or set(before) == {"0"}:
        return ["scripts/bootstrap_capturequest.sh"]  # First push: use the safe full lane.
    result = subprocess.run(
        ["git", "diff", "--name-only", before, after],
        check=True,
        capture_output=True,
        text=True,
    )
    return [line for line in result.stdout.splitlines() if line]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--before", default="")
    parser.add_argument("--after", default="HEAD")
    parser.add_argument("--mode", choices=("auto", "frontend", "backend", "full"), default="auto")
    parser.add_argument("--reset", action="store_true")
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()

    paths = changed_paths(args.before, args.after)
    result = classify_paths(paths, args.mode, args.reset)
    result["changed_paths"] = ",".join(paths)
    lines = [f"{key}={value}" for key, value in result.items()]
    if args.output:
        args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    else:
        print("\n".join(lines))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
