#!/usr/bin/env bash
# Run Go tests against an isolated, disposable PostgreSQL cluster. Never load
# application .env files or fall back to the application's DATABASE_URL.
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
for command in initdb pg_ctl go; do
  command -v "$command" >/dev/null || { echo "Required command missing: $command" >&2; exit 1; }
done
test_pg_dir="$(mktemp -d /var/tmp/capturequest-go-postgres.XXXXXX)"
cleanup() {
  result=$?
  trap - EXIT
  if [[ -s "$test_pg_dir/data/postmaster.pid" ]]; then
    # pg_ctl targets only the exact data directory created by this invocation.
    pg_ctl -D "$test_pg_dir/data" -m fast -w stop >/dev/null || result=1
  fi
  if [[ "$result" == 0 ]]; then
    rm -rf "$test_pg_dir"
  else
    echo "PostgreSQL test logs retained at $test_pg_dir" >&2
  fi
  exit "$result"
}
trap cleanup EXIT
initdb -D "$test_pg_dir/data" --auth=trust --no-locale >"$test_pg_dir/init.log"
pg_ctl -D "$test_pg_dir/data" -l "$test_pg_dir/postgres.log" \
  -o "-h '' -k '$test_pg_dir' -p 5432" -w start >/dev/null
export CAPTUREQUEST_TEST_DATABASE_URL="host=$test_pg_dir port=5432 dbname=postgres sslmode=disable"
export GOMAXPROCS="${GOMAXPROCS:-4}"
cd "$repo_dir/server"
if [[ "$#" == 0 ]]; then
  set -- ./internal/db/... ./internal/economy ./internal/world
fi
go test -race -p=4 "$@"
