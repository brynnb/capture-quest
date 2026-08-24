#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DEFAULT_EXTRACTOR_ROOT="${REPO_ROOT}/tools/pokemon-gameboy-extractor-tool"
EXTRACTOR_ROOT="${POKEMON_EXTRACTOR_ROOT:-${DEFAULT_EXTRACTOR_ROOT}}"
PHASER_DB_DEST="${REPO_ROOT}/public/phaser/pokemon.db"
DB_SOURCE="${POKEMON_DB_SOURCE:-${EXTRACTOR_ROOT}/pokemon.db}"
CAPTUREQUEST_RELEASE="${CAPTUREQUEST_POKEMON_RELEASE:-red}"
ADAPTER_BUNDLE_DEST="${REPO_ROOT}/public/phaser/capturequest-pokemon-import.json"

ASSETS_ONLY=0
SKIP_EXTRACTOR=0
SKIP_AUDIO_RENDER=0
BOOTSTRAP_ARGS=()

validate_extractor_artifact() {
  local db_path="$1"
  local python_bin="${EXTRACTOR_ROOT}/.venv/bin/python"
  if [[ ! -x "${python_bin}" ]]; then
    python_bin="$(command -v python3)"
  fi

  "${python_bin}" - "${db_path}" <<'PY'
import sqlite3
import sys

db_path = sys.argv[1]
required_tables = (
    "tiles",
    "tile_images",
    "script_event_candidates",
    "script_event_candidate_diagnostics",
    "script_event_in_game_trades",
    "script_event_tile_overrides",
    "script_event_boulder_targets",
    "script_event_dungeon_hole_warps",
    "spin_tiles",
)

with sqlite3.connect(db_path) as conn:
    missing = []
    empty = []
    for table in required_tables:
        exists = conn.execute(
            "SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?",
            (table,),
        ).fetchone()
        if not exists:
            missing.append(table)
        elif conn.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0] < 1:
            empty.append(table)

if missing or empty:
    details = []
    if missing:
        details.append("missing: " + ", ".join(missing))
    if empty:
        details.append("empty: " + ", ".join(empty))
    raise SystemExit("Extractor artifact validation failed (" + "; ".join(details) + ")")
PY

  local bundle_stage="${ADAPTER_BUNDLE_DEST}.stage"
  mkdir -p "$(dirname "${ADAPTER_BUNDLE_DEST}")"
  rm -f "${bundle_stage}"
  PYTHONPATH="${EXTRACTOR_ROOT}/export_scripts" \
    "${python_bin}" -m adapters.capturequest_v2 \
    "${db_path}" --release "${CAPTUREQUEST_RELEASE}" --output "${bundle_stage}"
  mv -f "${bundle_stage}" "${ADAPTER_BUNDLE_DEST}"
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
  CAPTUREQUEST_POKEMON_RELEASE  red or blue. Defaults to red.

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

GENERATE_SCRIPT="generate"
if [[ "${SKIP_AUDIO_RENDER}" -eq 0 ]]; then
  render_mode="${CAPTUREQUEST_RENDER_AUDIO:-auto}"
  if [[ "${render_mode}" != "0" && "${render_mode}" != "false" ]]; then
    missing_tools=()
    for tool in gbsplay ffmpeg rgbasm rgblink; do
      command -v "${tool}" >/dev/null 2>&1 || missing_tools+=("${tool}")
    done
    if [[ ${#missing_tools[@]} -eq 0 ]]; then
      GENERATE_SCRIPT="generate:complete"
    elif [[ "${render_mode}" == "1" || "${render_mode}" == "true" ]]; then
      echo "Missing complete audio tool(s): ${missing_tools[*]}" >&2
      exit 1
    else
      echo "Missing complete audio tool(s): ${missing_tools[*]}; generating metadata only."
    fi
  fi
fi

if [[ "${SKIP_EXTRACTOR}" -eq 0 ]]; then
  echo "Running extractor pipeline (${GENERATE_SCRIPT})..."
  (cd "${EXTRACTOR_ROOT}" && npm run "${GENERATE_SCRIPT}")
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

echo "Syncing extractor assets into CaptureQuest..."
SYNC_PYTHON_BIN="${EXTRACTOR_ROOT}/.venv/bin/python"
if [[ ! -x "${SYNC_PYTHON_BIN}" ]]; then
  SYNC_PYTHON_BIN="$(command -v python3)"
fi
"${SYNC_PYTHON_BIN}" "${REPO_ROOT}/scripts/sync_extractor_assets.py" \
  --extractor-root "${EXTRACTOR_ROOT}" \
  --repo-root "${REPO_ROOT}" \
  --database "${DB_SOURCE}" \
  --viewer-public "${EXTRACTOR_ROOT}/pokemon-phaser/public" \
  --audio-root "${EXTRACTOR_ROOT}/build/audio"

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
CAPTUREQUEST_POKEMON_RELEASE="${CAPTUREQUEST_RELEASE}" \
  "${REPO_ROOT}/server/scripts/bootstrap_postgres.sh" "${BOOTSTRAP_ARGS[@]}"

echo "Generating TypeScript API bindings..."
npm run tygo

echo "CaptureQuest bootstrap complete."
echo "Next: npm run dev:all"
