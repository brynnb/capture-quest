#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: server/scripts/bootstrap_postgres.sh [--create] [--reset] [--skip-smoke] [--sqlite path]

Bootstraps a CaptureQuest Postgres database from repo-owned sources:
  - server/schema/postgres_runtime_schema.sql
  - public/phaser/pokemon.db
  - server/scripted_events/scripts/*.json
  - server/scripted_events/manual_scripts/*.json

Environment:
  DATABASE_URL        Postgres URL. Defaults to postgres://postgres@localhost:5432/capturequest?sslmode=disable
  DATABASE_NAME       Local database name for --create. Defaults to capturequest
  IMPORT_SQLITE_PATH  Override SQLite import path.

Options:
  --create      Run createdb before applying schema. Intended for default local Postgres.
  --reset       Drop and recreate the public schema before bootstrapping.
  --skip-smoke  Skip go run ./cmd/db-smoke.
  --sqlite      Path to the extracted Pokemon SQLite database.
USAGE
}

CREATE_DB=0
RESET_DB=0
SKIP_SMOKE=0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVER_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${SERVER_DIR}/.." && pwd)"

repair_local_psql_library_path() {
  if ! command -v psql >/dev/null 2>&1; then
    return
  fi
  if ! command -v ldd >/dev/null 2>&1; then
    return
  fi
  local psql_path
  psql_path="$(command -v psql)"
  if ! ldd "${psql_path}" 2>/dev/null | grep -q 'libpq\.so\.5 => not found'; then
    return
  fi
  local libpq_path
  libpq_path="$(find "${HOME}/.local/share/capturequest-tools" -name 'libpq.so.5' -print -quit 2>/dev/null || true)"
  if [[ -z "${libpq_path}" ]]; then
    return
  fi
  export LD_LIBRARY_PATH="$(dirname "${libpq_path}"):${LD_LIBRARY_PATH:-}"
}

DATABASE_NAME="${DATABASE_NAME:-capturequest}"
export DATABASE_URL="${DATABASE_URL:-postgres://postgres@localhost:5432/${DATABASE_NAME}?sslmode=disable}"
SCHEMA_PATH="${SERVER_DIR}/schema/postgres_runtime_schema.sql"
SQLITE_PATH="${IMPORT_SQLITE_PATH:-${REPO_ROOT}/public/phaser/pokemon.db}"

postgres_admin_database_url() {
  local base="${DATABASE_URL}"
  local query=""
  if [[ "${base}" == *\?* ]]; then
    query="?${base#*\?}"
    base="${base%%\?*}"
  fi
  printf '%s/postgres%s\n' "${base%/*}" "${query}"
}

validate_local_database_name() {
  if [[ ! "${DATABASE_NAME}" =~ ^[A-Za-z0-9_]+$ ]]; then
    echo "DATABASE_NAME must contain only letters, numbers, and underscores for --create: ${DATABASE_NAME}" >&2
    exit 2
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --create)
      CREATE_DB=1
      shift
      ;;
    --reset)
      RESET_DB=1
      shift
      ;;
    --skip-smoke)
      SKIP_SMOKE=1
      shift
      ;;
    --sqlite)
      if [[ $# -lt 2 ]]; then
        echo "--sqlite requires a path" >&2
        exit 2
      fi
      SQLITE_PATH="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if ! command -v psql >/dev/null 2>&1; then
  echo "psql is required to apply the Postgres schema" >&2
  exit 1
fi
repair_local_psql_library_path
if ! psql --version >/dev/null 2>&1; then
  echo "psql was found but could not run. Check your Postgres client installation." >&2
  exit 1
fi

if [[ ! -f "${SCHEMA_PATH}" ]]; then
  echo "Schema not found: ${SCHEMA_PATH}" >&2
  exit 1
fi

if [[ ! -f "${SQLITE_PATH}" ]]; then
  echo "SQLite source database not found: ${SQLITE_PATH}" >&2
  exit 1
fi

if [[ "${CREATE_DB}" -eq 1 ]]; then
  validate_local_database_name
  ADMIN_DATABASE_URL="$(postgres_admin_database_url)"
  if psql "${ADMIN_DATABASE_URL}" -tAc "SELECT 1 FROM pg_database WHERE datname = '${DATABASE_NAME}'" | grep -qx '1'; then
    echo "Local database ${DATABASE_NAME} already exists"
  else
    psql "${ADMIN_DATABASE_URL}" -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"${DATABASE_NAME}\";"
    echo "Created local database ${DATABASE_NAME}"
  fi
fi

echo "Using DATABASE_URL=${DATABASE_URL}"

if [[ "${RESET_DB}" -eq 1 ]]; then
  echo "Resetting public schema"
  psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -c 'DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;'
fi

echo "Applying ${SCHEMA_PATH}"
psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -f "${SCHEMA_PATH}"

echo "Importing ${SQLITE_PATH}"
(
  cd "${SERVER_DIR}"
  go run ./cmd/import-phaser "${SQLITE_PATH}"
)

if [[ "${SKIP_SMOKE}" -eq 0 ]]; then
  echo "Running database smoke checks"
  (
    cd "${SERVER_DIR}"
    go run ./cmd/db-smoke
  )
fi

echo "CaptureQuest Postgres bootstrap complete."
