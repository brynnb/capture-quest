#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DEFAULT_EXTRACTOR_ROOT="${REPO_ROOT}/tools/pokemon-gameboy-extractor-tool"
EXTRACTOR_ROOT="${POKEMON_EXTRACTOR_ROOT:-${DEFAULT_EXTRACTOR_ROOT}}"
PHASER_DB_DEST="${REPO_ROOT}/public/phaser/pokemon.db"
DB_SOURCE="${POKEMON_DB_SOURCE:-${EXTRACTOR_ROOT}/pokemon.db}"

ASSETS_ONLY=0
SKIP_EXTRACTOR=0
SKIP_AUDIO_RENDER=0
BOOTSTRAP_ARGS=()

validate_extractor_artifact() {
  local db_path="$1"

  if ! command -v python3 >/dev/null 2>&1; then
    echo "python3 is required to validate the extractor SQLite artifact." >&2
    exit 1
  fi

  python3 - "$db_path" <<'PY'
import sqlite3
import sys

db_path = sys.argv[1]
required_counts = {
    "tiles": 1,
    "tile_images": 1,
    "script_event_candidates": 1,
    "script_event_candidate_diagnostics": 1,
    "script_event_in_game_trades": 1,
    "script_event_tile_overrides": 1,
    "script_event_boulder_targets": 1,
    "script_event_dungeon_hole_warps": 1,
    "spin_tiles": 1,
}

try:
    conn = sqlite3.connect(db_path)
except sqlite3.Error as exc:
    print(f"Could not open extractor artifact {db_path}: {exc}", file=sys.stderr)
    sys.exit(1)

missing = []
empty = []
try:
    for table, min_count in required_counts.items():
        exists = conn.execute(
            "SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?",
            (table,),
        ).fetchone()
        if not exists:
            missing.append(table)
            continue
        count = conn.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0]
        if count < min_count:
            empty.append(f"{table} ({count})")
finally:
    conn.close()

if missing or empty:
    if missing:
        print("Extractor artifact is missing required tables:", file=sys.stderr)
        for table in missing:
            print(f"  - {table}", file=sys.stderr)
    if empty:
        print("Extractor artifact has empty required tables:", file=sys.stderr)
        for table in empty:
            print(f"  - {table}", file=sys.stderr)
    sys.exit(1)
PY
}

usage() {
  cat <<EOF
Usage:
  npm run bootstrap:fresh
  npm run bootstrap:assets

Top-level CaptureQuest bootstrap:
  1. Initializes the bundled extractor submodule if needed.
  2. Runs the extractor pipeline to rebuild pokemon.db and viewer assets.
  3. Syncs generated assets into CaptureQuest runtime folders.
  4. Imports supported script candidates.
  5. Bootstraps Postgres unless --assets-only is used.

Options:
  --assets-only        Generate/sync assets and scripts without resetting Postgres.
  --skip-extractor     Reuse the existing extractor outputs.
  --skip-audio-render  Do not render original audio, even if the extractor supports it.
  -h, --help           Show this help.

Any remaining args are passed to server/scripts/bootstrap_postgres.sh.
If no Postgres args are passed, --create --reset is used.

Environment:
  POKEMON_EXTRACTOR_ROOT     Extractor checkout. Defaults to tools/pokemon-gameboy-extractor-tool.
  POKEMON_DB_SOURCE          Explicit pokemon.db source path. Defaults to \$POKEMON_EXTRACTOR_ROOT/pokemon.db.
  CAPTUREQUEST_RENDER_AUDIO  auto, 0, or 1. Defaults to auto.

Extractor artifact:
  ${DB_SOURCE}
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      echo
      "${REPO_ROOT}/server/scripts/bootstrap_postgres.sh" --help
      exit 0
      ;;
    --assets-only)
      ASSETS_ONLY=1
      shift
      ;;
    --skip-extractor)
      SKIP_EXTRACTOR=1
      shift
      ;;
    --skip-audio-render)
      SKIP_AUDIO_RENDER=1
      shift
      ;;
    --)
      shift
      BOOTSTRAP_ARGS+=("$@")
      break
      ;;
    *)
      BOOTSTRAP_ARGS+=("$1")
      shift
      ;;
  esac
done

if ! command -v npm >/dev/null 2>&1; then
  echo "npm is required to bootstrap CaptureQuest." >&2
  exit 1
fi

cd "${REPO_ROOT}"

if [[ "${EXTRACTOR_ROOT}" == "${DEFAULT_EXTRACTOR_ROOT}" ]]; then
  if [[ ! -e "${EXTRACTOR_ROOT}/.git" ]]; then
    echo "Initializing extractor submodule..."
    git submodule update --init --recursive tools/pokemon-gameboy-extractor-tool
  fi
fi

if [[ ! -d "${EXTRACTOR_ROOT}" ]]; then
  cat >&2 <<EOF
Extractor checkout not found:
  ${EXTRACTOR_ROOT}

Use the bundled submodule or set:
  POKEMON_EXTRACTOR_ROOT=/path/to/pokemon-gameboy-extractor-tool
EOF
  exit 1
fi

if [[ "${SKIP_EXTRACTOR}" -eq 0 ]]; then
  echo "Running extractor pipeline..."
  (cd "${EXTRACTOR_ROOT}" && npm run generate)
else
  echo "Skipping extractor generation; reusing ${EXTRACTOR_ROOT}"
fi

if [[ -f "${DB_SOURCE}" && ! -s "${DB_SOURCE}" ]]; then
  cat >&2 <<EOF
Extractor artifact exists but is empty:
  ${DB_SOURCE}

Rebuild it with:
  npm run bootstrap:assets
EOF
  exit 1
fi

if [[ ! -f "${DB_SOURCE}" ]]; then
  cat >&2 <<EOF
Extractor artifact was not found:
  ${DB_SOURCE}

Run the extractor through CaptureQuest:
  npm run bootstrap:assets
EOF
  exit 1
fi

echo "Generating dungeon hole warp seeds..."
python3 "${REPO_ROOT}/scripts/generate_dungeon_hole_warps.py" \
  --extractor-root "${EXTRACTOR_ROOT}" \
  --sqlite "${DB_SOURCE}"

if ! validate_extractor_artifact "${DB_SOURCE}"; then
  cat >&2 <<EOF

Rebuild a complete extractor artifact before bootstrapping CaptureQuest:
  npm run bootstrap:assets

If tile tables are empty, install RGBDS so rgbgfx can convert tilesets.
EOF
  exit 1
fi

render_extractor_audio() {
  local render_mode="${CAPTUREQUEST_RENDER_AUDIO:-auto}"

  if [[ "${SKIP_AUDIO_RENDER}" -eq 1 || "${render_mode}" == "0" || "${render_mode}" == "false" ]]; then
    echo "Skipping original audio render."
    return 0
  fi

  if ! node -e "const p=require(process.argv[1]); process.exit(p.scripts && p.scripts['render:audio'] ? 0 : 1)" "${EXTRACTOR_ROOT}/package.json" >/dev/null 2>&1; then
    if [[ "${render_mode}" == "1" || "${render_mode}" == "true" ]]; then
      echo "Extractor does not provide npm run render:audio." >&2
      exit 1
    fi
    echo "Extractor does not yet provide npm run render:audio; skipping original audio render."
    return 0
  fi

  local missing_tools=()
  for tool in gbsplay oggenc rgbasm rgblink rgbfix; do
    if ! command -v "${tool}" >/dev/null 2>&1; then
      missing_tools+=("${tool}")
    fi
  done
  if [[ ${#missing_tools[@]} -gt 0 ]]; then
    if [[ "${render_mode}" == "1" || "${render_mode}" == "true" ]]; then
      echo "Missing audio render tool(s): ${missing_tools[*]}" >&2
      exit 1
    fi
    echo "Missing audio render tool(s): ${missing_tools[*]}; skipping original audio render."
    return 0
  fi

  if [[ -f "${EXTRACTOR_ROOT}/export_scripts/export_audio_manifest.py" ]]; then
    local python_bin="${EXTRACTOR_ROOT}/.venv/bin/python"
    if [[ ! -x "${python_bin}" ]]; then
      python_bin="$(command -v python3)"
    fi
    echo "Generating source audio manifest..."
    (cd "${EXTRACTOR_ROOT}" && "${python_bin}" export_scripts/export_audio_manifest.py)
  fi

  echo "Rendering original music..."
  (cd "${EXTRACTOR_ROOT}" && npm run render:audio -- \
    --build-rom \
    --out-dir "${REPO_ROOT}/public" \
    --kind music \
    --seconds "${CAPTUREQUEST_AUDIO_MUSIC_SECONDS:-90}" \
    --fade "${CAPTUREQUEST_AUDIO_MUSIC_FADE:-3}")

  echo "Rendering original sound effects..."
  (cd "${EXTRACTOR_ROOT}" && npm run render:audio -- \
    --build-rom \
    --out-dir "${REPO_ROOT}/public" \
    --kind sfx \
    --seconds "${CAPTUREQUEST_AUDIO_SFX_SECONDS:-8}" \
    --fade "${CAPTUREQUEST_AUDIO_SFX_FADE:-0}")

  echo "Rendering original Pokemon cries..."
  (cd "${EXTRACTOR_ROOT}" && npm run render:audio -- \
    --build-rom \
    --out-dir "${REPO_ROOT}/public" \
    --kind cries \
    --seconds "${CAPTUREQUEST_AUDIO_CRY_SECONDS:-4}" \
    --fade "${CAPTUREQUEST_AUDIO_CRY_FADE:-0}")
}

render_extractor_audio

echo "Syncing extractor assets into CaptureQuest..."
SYNC_PYTHON_BIN="${EXTRACTOR_ROOT}/.venv/bin/python"
if [[ ! -x "${SYNC_PYTHON_BIN}" ]]; then
  SYNC_PYTHON_BIN="$(command -v python3)"
fi
"${SYNC_PYTHON_BIN}" "${REPO_ROOT}/scripts/sync_extractor_assets.py" \
  --extractor-root "${EXTRACTOR_ROOT}" \
  --repo-root "${REPO_ROOT}"

if [[ -f "${EXTRACTOR_ROOT}/audio_manifest.json" ]]; then
  echo "Syncing Pokemon audio metadata..."
  npm run audio:sync
fi

if [[ -f ".env" ]]; then
  set -a
  # shellcheck disable=SC1091
  . ".env"
  set +a
fi

if [[ ! -d "node_modules" ]]; then
  echo "Installing npm dependencies..."
  npm ci
fi

echo "Importing extractor script candidates..."
(cd "${REPO_ROOT}/server" && go run ./cmd/import-script-candidates \
  --sqlite "${PHASER_DB_DEST}" \
  --output "${REPO_ROOT}/server/scripted_events/scripts")

echo "Generating audio manifest..."
npm run audio:manifest

if [[ "${ASSETS_ONLY}" -eq 1 ]]; then
  echo "CaptureQuest asset bootstrap complete."
  exit 0
fi

if [[ ${#BOOTSTRAP_ARGS[@]} -eq 0 ]]; then
  BOOTSTRAP_ARGS=(--create --reset)
fi

echo "Bootstrapping CaptureQuest Postgres..."
"${REPO_ROOT}/server/scripts/bootstrap_postgres.sh" "${BOOTSTRAP_ARGS[@]}"

echo "Generating TypeScript API bindings..."
npm run tygo

echo "CaptureQuest bootstrap complete."
echo "Next: npm run dev:all"
