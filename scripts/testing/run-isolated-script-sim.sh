#!/usr/bin/env bash
# Bootstrap canonical data in a private Unix-socket PostgreSQL cluster. Script
# sync and fixture replacement never inherit the application's target or .env.
set -euo pipefail
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
if [[ "$#" == 0 ]]; then
 echo 'Usage: scripts/testing/run-isolated-script-sim.sh --scenario NAME [--check|--update]' >&2
 exit 2
fi
for command in initdb pg_ctl go; do
 command -v "$command" >/dev/null || { echo "Required command missing: $command" >&2; exit 1; }
done
sim_run_dir=$(mktemp -d /var/tmp/capturequest-script-sim.XXXXXX)
cleanup() {
 result=$?
 trap - EXIT
 if [[ -s "$sim_run_dir/pg/postmaster.pid" ]]; then
  pg_ctl -D "$sim_run_dir/pg" -m fast -w stop >"$sim_run_dir/pg-stop.log" || result=1
 fi
 echo "Evidence retained: $sim_run_dir"
 exit "$result"
}
trap cleanup EXIT
echo "Evidence directory: $sim_run_dir"
cd "$repo_dir"
python3 scripts/validate_runtime_assets.py >"$sim_run_dir/assets.log"
initdb -D "$sim_run_dir/pg" --auth=trust --no-locale >"$sim_run_dir/init.log"
pg_ctl -D "$sim_run_dir/pg" -l "$sim_run_dir/postgres.log" \
 -o "-h '' -k '$sim_run_dir' -p 5432" -w start >"$sim_run_dir/pg-start.log"
export DATABASE_URL="postgresql:///postgres?host=$sim_run_dir&sslmode=disable"
export CAPTUREQUEST_TEST_DATABASE_URL="$DATABASE_URL"
export LOCAL=true GOMAXPROCS=4
bash server/scripts/bootstrap_postgres.sh >"$sim_run_dir/bootstrap.log" 2>&1
cd server
go run ./cmd/script-sim "$@" 2>&1 | tee "$sim_run_dir/simulator.log"
