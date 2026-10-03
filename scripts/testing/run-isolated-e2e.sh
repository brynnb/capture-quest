#!/usr/bin/env bash
# Bootstrap and run browser checks only against a fresh private Unix-socket
# PostgreSQL cluster. Never source application .env files or reuse a live DB.
# Each run records its exact app/client PIDs and retains logs/traces under /var/tmp.
set -euo pipefail
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
shutdown_mode=${CQ_E2E_SHUTDOWN_MODE:-}
case "$shutdown_mode" in ""|success|failure) ;; *) echo "Invalid shutdown mode" >&2; exit 1;; esac
run_dir=$(mktemp -d /var/tmp/capturequest-rendered.XXXXXX)
app_pid=''
client_pid=''
cleanup() {
 result=$?
 trap - EXIT
 for owned_pid in "$client_pid" "$app_pid"; do
  if [[ -n "$owned_pid" ]] && kill -0 "$owned_pid" 2>/dev/null; then
   kill -TERM "$owned_pid"
   for attempt in $(seq 1 100); do
    kill -0 "$owned_pid" 2>/dev/null || break
    sleep 0.1
   done
   if kill -0 "$owned_pid" 2>/dev/null; then
    echo "Owned process $owned_pid exceeded cleanup deadline" >&2
    kill -KILL "$owned_pid"
    result=1
   fi
   wait "$owned_pid" 2>/dev/null || true
  fi
 done
 if [[ -s "$run_dir/pg/postmaster.pid" ]]; then pg_ctl -D "$run_dir/pg" -m fast -w stop >"$run_dir/pg-stop.log" || result=1; fi
 echo "Evidence retained: $run_dir"
 exit "$result"
}
trap cleanup EXIT
echo "Evidence directory: $run_dir"
cd "$repo_dir"
if [[ "$#" == 0 ]]; then
 if [[ -n "$shutdown_mode" ]]; then set -- tests/e2e/server-shutdown.spec.ts; else set -- tests/e2e/game-corner.spec.ts; fi
fi
eval "$(HTTP_PORT=18280 WT_PORT=18433 HASH_PORT=18100 VITE_DEV_PORT=15178 node scripts/dev-port-env.mjs)"
export LOCAL=true CAPTUREQUEST_TEST_MODE=true VITE_TEST_MODE=true VITE_FORCE_WEBSOCKET=true VITE_LOCAL_DEV=true VITE_OFFLINE_ASSETS=false
export GOMAXPROCS=4
export DATABASE_URL="postgresql:///postgres?host=$run_dir&sslmode=disable"
printf 'HTTP_PORT=%s\nVITE_DEV_PORT=%s\nWT_PORT=%s\n' "$HTTP_PORT" "$VITE_DEV_PORT" "$WT_PORT" > "$run_dir/ports"
python3 scripts/validate_runtime_assets.py >"$run_dir/assets.log"
initdb -D "$run_dir/pg" --auth=trust --no-locale >"$run_dir/init.log"
pg_ctl -D "$run_dir/pg" -l "$run_dir/postgres.log" -o "-h '' -k '$run_dir' -p 5432" -w start >"$run_dir/pg-start.log"
echo 'Bootstrapping isolated runtime database from matched local artifacts'
bash server/scripts/bootstrap_postgres.sh >"$run_dir/bootstrap.log" 2>&1
(cd server && go build -o "$run_dir/cq-server" ./cmd/server)
(cd server && exec "$run_dir/cq-server") >"$run_dir/server.log" 2>&1 &
app_pid=$!
printf 'server=%s\n' "$app_pid" > "$run_dir/pids"
ready=false
for attempt in $(seq 1 120); do
 if curl --max-time 1 --silent --fail "http://localhost:$HTTP_PORT/api/ready" >"$run_dir/readiness.json"; then ready=true; break; fi
 if ! kill -0 "$app_pid" 2>/dev/null; then echo 'Server exited before readiness'; exit 1; fi
 sleep 0.5
done
[[ "$ready" == true ]] || { echo 'Readiness deadline expired'; exit 1; }
node node_modules/vite/bin/vite.js --host 127.0.0.1 --port "$VITE_DEV_PORT" --strictPort >"$run_dir/client.log" 2>&1 &
client_pid=$!
printf 'client=%s\n' "$client_pid" >> "$run_dir/pids"
for attempt in $(seq 1 40); do
 if curl --max-time 1 --silent --fail "http://localhost:$VITE_DEV_PORT" >/dev/null; then break; fi
 sleep 0.25
done
if [[ -n "$shutdown_mode" ]]; then
 export E2E_ISOLATED_SERVER_PID="$app_pid" E2E_ISOLATED_RUN_DIR="$run_dir" E2E_ISOLATED_DATABASE_URL="$DATABASE_URL"
fi
echo "Running rendered check on http://localhost:$VITE_DEV_PORT"
E2E_APP_URL="http://localhost:$VITE_DEV_PORT" npx playwright test -c playwright.config.ts "$@" --output "$run_dir/playwright"

if [[ -n "$shutdown_mode" ]]; then
 [[ -s "$run_dir/shutdown-evidence.json" ]] || { echo "Shutdown test did not produce terminal evidence" >&2; exit 1; }
 # The dedicated test signals this exact child and proves it is terminal.
 # Only its shell parent can collect the real exit status.
 app_exit=0
 wait "$app_pid" || app_exit=$?
 app_pid=''
 printf 'exit_code=%s\n' "$app_exit" > "$run_dir/server-exit"
 expected_exit=0
 [[ "$shutdown_mode" == failure ]] && expected_exit=1
 if [[ "$app_exit" != "$expected_exit" ]]; then
  echo "Server exit $app_exit; expected $expected_exit" >&2
  exit 1
 fi
fi
