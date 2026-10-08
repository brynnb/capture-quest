#!/usr/bin/env bash
# Bootstrap and run browser checks only against a fresh private Unix-socket
# PostgreSQL cluster. Never source application .env files or reuse a live DB.
# Each run records its exact app/client PIDs and retains logs/traces under /var/tmp.
set -euo pipefail
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
shutdown_mode=${CQ_E2E_SHUTDOWN_MODE:-}
crash_recovery=${CQ_E2E_CRASH_RECOVERY:-}
case "$crash_recovery" in ""|true) ;; *) echo "Invalid crash recovery mode" >&2; exit 1;; esac
[[ -z "$crash_recovery" || -z "$shutdown_mode" ]] || { echo 'Choose shutdown or crash recovery mode'; exit 1; }
case "$shutdown_mode" in ""|success|failure) ;; *) echo "Invalid shutdown mode" >&2; exit 1;; esac
run_dir=$(mktemp -d /var/tmp/capturequest-rendered.XXXXXX)
app_pid=''
client_pid=''
test_pid=''
cleanup() {
 result=$?
 trap - EXIT
 for owned_pid in "$test_pid" "$client_pid" "$app_pid"; do
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
 if [[ -n "$crash_recovery" ]]; then set -- tests/e2e/server-process-recovery.spec.ts tests/e2e/movement-process-recovery.spec.ts; elif [[ -n "$shutdown_mode" ]]; then set -- tests/e2e/server-shutdown.spec.ts; else set -- tests/e2e/game-corner.spec.ts; fi
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
start_server() {
 (cd server && exec "$run_dir/cq-server") >"$run_dir/$1" 2>&1 &
 app_pid=$!
 printf '%s\n' "$app_pid" > "$run_dir/current-server.pid"
 printf 'server=%s\n' "$app_pid" >> "$run_dir/pids"
 ready=false
 for attempt in $(seq 1 120); do
  if curl --max-time 1 --silent --fail "http://localhost:$HTTP_PORT/api/ready" >"$run_dir/readiness.json"; then ready=true; break; fi
  if ! kill -0 "$app_pid" 2>/dev/null; then echo 'Server exited before readiness'; return 1; fi
  sleep 0.5
 done
 [[ "$ready" == true ]] || { echo 'Readiness deadline expired'; return 1; }
}
start_server server.log
node node_modules/vite/bin/vite.js --host 127.0.0.1 --port "$VITE_DEV_PORT" --strictPort >"$run_dir/client.log" 2>&1 &
client_pid=$!
printf 'client=%s\n' "$client_pid" >> "$run_dir/pids"
for attempt in $(seq 1 40); do
 if curl --max-time 1 --silent --fail "http://localhost:$VITE_DEV_PORT" >/dev/null; then break; fi
 sleep 0.25
done
# SQL fixture tests need the same exact-run database/process identity guard,
# without opting into process-death/restart acceptance.
if [[ -n "$shutdown_mode" || -n "$crash_recovery" || "${CQ_E2E_DATABASE_FIXTURE:-}" == true ]]; then
 export E2E_ISOLATED_SERVER_PID="$app_pid" E2E_ISOLATED_RUN_DIR="$run_dir" E2E_ISOLATED_DATABASE_URL="$DATABASE_URL"
fi
echo "Running rendered check on http://localhost:$VITE_DEV_PORT"
if [[ -n "$crash_recovery" ]]; then
 E2E_APP_URL="http://localhost:$VITE_DEV_PORT" node node_modules/playwright/cli.js test -c playwright.config.ts "$@" --output "$run_dir/playwright" &
 test_pid=$!
 printf 'playwright=%s\n' "$test_pid" >> "$run_dir/pids"
 restart_generation=0
 while kill -0 "$test_pid" 2>/dev/null; do
  if [[ -s "$run_dir/crash-request" ]]; then
   read -r requested_pid < "$run_dir/crash-request"
   [[ "$requested_pid" =~ ^[0-9]+$ && "$requested_pid" == "$app_pid" ]] || { echo 'Crash request does not match owned server'; exit 1; }
   rm "$run_dir/crash-request"
   # Only this exact shell child is eligible. Reap it before replacing the
   # ownership slot, so cleanup cannot target a dead/reused predecessor PID.
   crashed_pid=$app_pid
   kill -KILL "$app_pid"
   crash_exit=0
   wait "$app_pid" || crash_exit=$?
   app_pid=''
   [[ "$crash_exit" == 137 ]] || { echo "Unexpected crash exit $crash_exit"; exit 1; }
   restart_generation=$((restart_generation + 1))
   start_server "server-restarted-$restart_generation.log"
   printf '{"oldPid":%s,"newPid":%s,"exitCode":%s,"generation":%s}\n' "$crashed_pid" "$app_pid" "$crash_exit" "$restart_generation" > "$run_dir/restart-receipt.tmp"
   mv "$run_dir/restart-receipt.tmp" "$run_dir/restart-receipt.json"
  fi
  sleep 0.1
 done
 test_exit=0
 wait "$test_pid" || test_exit=$?
 test_pid=''
 [[ "$test_exit" == 0 ]] || exit "$test_exit"
 [[ "$restart_generation" -gt 0 && -s "$run_dir/process-recovery-evidence.json" ]] || { echo 'Missing process recovery evidence'; exit 1; }
else
 E2E_APP_URL="http://localhost:$VITE_DEV_PORT" npx playwright test -c playwright.config.ts "$@" --output "$run_dir/playwright"
fi

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
