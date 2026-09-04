# Deployment

CaptureQuest deploys as a Vite static frontend plus a Go server. The runtime
database target is Postgres.

## Services

| Service | Purpose | Notes |
| --- | --- | --- |
| CaptureQuest frontend | Static Vite build | Serve `dist/` through the edge/reverse proxy. |
| CaptureQuest server | Go binary | Handles HTTP API, WebSocket fallback, and WebTransport. |
| Postgres | Runtime database | Set through `DATABASE_URL`. |

## Required Environment

| Variable | Purpose |
| --- | --- |
| `DATABASE_URL` | Postgres connection string. |
| `LOCAL` | Must be exactly `false` in production; local npm scripts explicitly default it to `true`. |
| `HTTP_PORT` | HTTP API port. Defaults to the configured value. |
| `DISCORD_CHAT_WEBHOOK_URL` | Optional private webhook for the CaptureQuest Discord channel. |
| `DISCORD_CHAT_SHARED_SECRET` | Optional 32+ character secret shared only with the IdleQuest Discord hub. |
| `HASH_PORT` | WebTransport certificate-hash HTTP port. |
| `ADMIN_KEY` | Unique backend-only admin API secret, at least 32 characters. Never expose it to browser code or reuse another service's key. |
| `VITE_ASSET_URL` | Optional CDN/base URL for remote assets. |

Configure both Discord values or neither. A partial configuration prevents
startup. When enabled, public game chat is mirrored to Discord and signed
messages from the shared hub enter through `/api/discord/game-chat`; unsigned
requests are rejected. Keep both values out of Git and browser code.

The HTTP API binds to loopback in production and is published only through the
reverse proxy. `/api/admin/*` is additionally limited inside the Go server to
direct loopback requests without proxy forwarding headers. The dashboard must
call it server-to-server over `127.0.0.1` with CaptureQuest's unique key. Caddy
must return `404` for public `/api/admin/*` requests. CaptureQuest intentionally
does not provide a raw-SQL admin endpoint.

## GitHub Actions Deploy

The deploy workflow builds and uploads only the components selected by its
deployment lane. Configure these values in the GitHub repository settings
before enabling the workflow:

| Name | Type | Purpose |
| --- | --- | --- |
| `DEPLOY_SSH_KEY` | Secret | Private SSH key used by the action. |
| `DEPLOY_HOST` | Secret | SSH destination, for example `ubuntu@example.com`. |
| `DEPLOY_APP_DIR` | Repository variable | Absolute path to the app directory on the host. |

The workflow is the canonical deployment path. In `auto` mode it selects the
smallest safe lane from the files changed by the push:

| Lane | Used for | Work performed |
| --- | --- | --- |
| Frontend | React, Phaser, CSS, and browser-only code | Restore the exact cached build inputs, build Vite, upload only the application shell, atomically activate it. Never run extraction. |
| Backend | Go server code without schema/import changes | Build and replace the server binary, restart, and verify health. |
| Code | A push containing frontend and backend code | Stage both code artifacts and activate them together without touching Postgres. |
| Full data | Extractor, asset/audio pipeline, importer, schema, or generated scripted-event changes | Generate or restore the asset family, test, back up and import Postgres, then activate everything together. |

Documentation-only and workflow-only pushes do not redeploy the application.
Manual runs can override `auto` with `frontend`, `backend`, or `full`.
`run_tests` adds broad frontend and focused backend tests to a manual fast run;
full-data runs always execute them. `reset_database` always selects the full
lane and remains an explicitly destructive operation.

The full-data lane performs this order:

1. install pinned Node, Python, and extractor dependencies;
2. restore a validated generated-asset cache or rebuild all graphics, SQLite,
   scripted events, and audio;
3. validate the runtime asset contract, build the frontend, run focused importer
   and warp tests, and build the Linux server/importer binaries;
4. upload the frontend into a non-public staging directory, upload the binaries,
   stop `cq-server`, and create a compressed Postgres backup under
   `$DEPLOY_APP_DIR/backups`;
5. negotiate the extractor contract before touching Postgres, apply the schema,
   and run the deterministic importer;
6. replace the live `dist` directory with the complete staged frontend while the
   API remains stopped, then restart `cq-server` and Caddy, validate tile rows
   and the served asset contract, and retry the public health endpoint until
   startup completes.

Do not reorder schema negotiation, backup, schema application, or import. Do not
start a second deployment while an importer from the first one may still exist.
Never upload directly into the live `dist` tree: exposing a new JavaScript tile
catalog while the old server/database is still active can render every tile with
the wrong artwork. Generated-asset cache keys are immutable; bump the cache
generation when its saved path set changes, and validate the exact DB-backed
tile and sprite catalogs before accepting a restored cache.

The frontend fast lane does not upload the large generated directories again.
It copies the live `dist` to a staging directory on the host, removes stale
hashed application files, overlays the new small application shell, and
atomically renames the stage. Before doing so, it requires the SHA-256 of the
staged `runtime_asset_contract.json` to equal production. A mismatch fails with
an instruction to run the full lane; it must never be bypassed. This should turn
an ordinary code-only deployment from roughly twelve minutes into a
one-to-three-minute build and upload. The lane also requires an exact generated
asset cache hit for its compile-time manifests and contract. If that cache is
missing, it fails with an instruction to run the full lane; it never silently
turns an unrelated frontend change into a full extraction.

The backend fast lane does not run the importer, schema, or database backup. It
is valid only because schema and importer paths are classified as full-data
changes. It stages the new binary, stops the service for replacement, restarts
it, and runs the public health and production debug-route checks.

If a full-data stage fails after `cq-server` has stopped, the cleanup trap leaves
it stopped. Do not restart it until the database, server binary, staged frontend,
and runtime asset contract have been verified as one coherent release. Fast code
deployments retain the previous binary/static tree and roll back to them when
post-activation verification fails.

## Build

Generated runtime assets are ignored by Git. They must exist before Vite builds
the frontend; Vite copies `public/` into `dist/` at build time.

```bash
npm ci
CAPTUREQUEST_RENDER_AUDIO=1 npm run bootstrap:assets
npm run build

test -s dist/sound/pokemon/music/pallet_town.ogg
test -s dist/phaser/pokemon.db
test -s dist/phaser/runtime_asset_contract.json
cmp public/phaser/runtime_asset_contract.json dist/phaser/runtime_asset_contract.json

cd server
go test ./...
go build -o /var/tmp/capturequest-server ./cmd/server
```

If the generated assets were restored from a known-good CI cache, the bootstrap
step can be skipped. Never skip the runtime-asset checks. The contract binds the
SQLite database, the complete tile-image catalog, and the cache version compiled
into the frontend. Tile artwork URLs use that version so browsers cannot reuse a
numeric tile ID from an older extractor catalog.

The full contract is specific to one generated run. To validate production,
compare the SHA-256 of the database served at `/phaser/pokemon.db` with
`pokemonDbSha256` in the contract served at
`/phaser/runtime_asset_contract.json`. Do not require a separately generated
local SQLite database to have the same whole-file hash. Its catalog is compatible
only when `tileCatalogSha256` and `tileCount` also match.

Tile images intentionally keep numeric filenames. Their URLs must include the
generated tile-catalog hash from `src/constants/runtime_asset_version.ts`; do not
remove that query parameter. Without it, a browser can display cached artwork
from an older catalog under a newly assigned numeric ID.

The CI cache key should contain only inputs that can change generated runtime
content: extractor revision, generation/sync scripts, dependency lockfiles,
release, and source assets. Do not include unrelated workflow formatting in that
hash. A restored cache is accepted for a full-data release only after readiness
checks find the database, runtime contract, generated catalog version, graphics,
sprites, scripted events, and complete audio metadata; a partial or older cache
must regenerate in that lane. A frontend-only release instead requires the exact
cache key and its compile-time manifests, and fails rather than regenerating.
Freshly generated assets are saved immediately after those checks pass, before
frontend tests or deployment, so a later unrelated failure does not force the
extractor to repeat the same work on the next run.

`scripts/bootstrap_capturequest.sh` must generate
`src/constants/audio_manifest.json` before calling the combined runtime asset
validator. Otherwise a successful multi-minute extraction fails at its final
validation step and GitHub cannot save the completed cache.

## Generated Audio

Pokemon world, battle, SFX, and cry audio is generated under
`public/sound/pokemon/`. The directory is intentionally ignored by Git, so a
checkout alone is not a deployable frontend. The extractor retains 48 kHz
stereo FLAC masters in its build output, but CaptureQuest publishes only 24 kHz
mono Ogg Vorbis derivatives. Do not copy the FLAC masters into `public/` or
`dist/`.

The required flow is:

```text
extractor/bootstrap -> public/sound/pokemon -> vite build -> dist/sound/pokemon
```

Verify both sides of the Vite build:

```bash
test -n "$(find public/sound/pokemon -name '*.ogg' -type f -print -quit)"
test -n "$(find dist/sound/pokemon -name '*.ogg' -type f -print -quit)"
test -z "$(find public/sound/pokemon -name '*.flac' -type f -print -quit)"
test -z "$(find dist/sound/pokemon -name '*.flac' -type f -print -quit)"
```

When audio files are absent, Caddy's single-page-app fallback may return
`index.html` with HTTP 200 for an `.ogg` URL. A successful status code alone is
therefore not sufficient; verify the MIME type and a realistic content length:

```bash
curl -sSI https://capturequest.net/sound/pokemon/music/pallet_town.ogg
```

The response must contain `Content-Type: audio/ogg`. A response containing
`Content-Type: text/html` means the deployed file is missing.

## Manual Frontend Deployment

Do not manually upload a complete frontend into the live `dist` directory.
The JavaScript bundle, Postgres tile IDs, tile images, and
`runtime_asset_contract.json` are one release unit; replacing only the static
tree can give active clients valid numeric IDs paired with the wrong artwork.
Use the canonical GitHub Actions deployment. Its frontend lane verifies that the
live runtime contract is unchanged before atomically activating
`dist.next-<sha>`; its full-data lane stops the API and imports the matching data
when the contract actually changes.

For recovery, inspect the failed workflow and the importer state as described
above. Do not improvise a full tar-over-SSH deployment. The narrowly scoped
audio-only procedure below is safe because it does not replace the application
bundle or tile catalog.

### Audio-only recovery

If the application is already deployed and only generated Pokemon audio is
missing, upload only that directory. This avoids changing the frontend bundle
or restarting the game server:

```bash
test -s dist/sound/pokemon/music/pallet_town.ogg

tar --exclude='._*' --exclude='.DS_Store' -czf - -C dist sound/pokemon \
  | ssh -o BatchMode=yes -i "$CQ_DEPLOY_KEY" "$CQ_DEPLOY_HOST" \
      "cd '$CQ_DEPLOY_APP_DIR/dist' && mkdir -p sound && tar xzf -"

curl -sSI https://capturequest.net/sound/pokemon/music/pallet_town.ogg
```

Do not upload `public/sound/pokemon` directly to the document root: its correct
public location comes from the `dist/sound/pokemon` path created by Vite.

## Database Bootstrap

On a fresh database, generate assets and apply the runtime schema through the
top-level bootstrap path:

```bash
DATABASE_URL="$DATABASE_URL" npm run bootstrap:fresh
```

The importer is deterministic for extractor-generated static data and
scripted-event JSON. It should be safe to rerun after source data changes.

Ordinary imports preserve manual tile edits. Source data refreshes each row's
`original_*` snapshot, while `has_tile_edit = 1` preserves the current edited
value. Intentional historical cleanup is recorded once in
`phaser_data_repairs`; do not turn it into a recurring reset.

The production tile merge is about 95,000 rows. It must remain a set-based
`UPDATE ... FROM phaser_tiles_import_stage` operation backed by the staging
coordinate index. Multiple correlated staging-table lookups per destination row
make the merge quadratic and can hold the table lock long enough for SSH and
service startup to fail.

## Failure Recovery

If an SSH deployment session disconnects during import, do not immediately run
another importer. The remote process and its PostgreSQL statement can survive
the client connection. The workflow deliberately leaves `cq-server` stopped
when that outcome is ambiguous; do not manually restart it against the old
frontend catalog. Establish a new keepalive-enabled SSH session and inspect both
layers:

```bash
pgrep -af '[i]mport-phaser'

set -a
# The deploy account intentionally cannot read the production environment
# directly. Read it through its non-interactive sudo permission without
# printing the secrets into the shell or workflow log.
. <(sudo -n cat /etc/capturequest/cq-server.env)
set +a
psql "$DATABASE_URL" -c \
  "SELECT pid, state, wait_event_type, wait_event, query_start, left(query, 160)
   FROM pg_stat_activity
   WHERE query ILIKE '%phaser_tiles%' OR query ILIKE '%import_stage%';"
```

An active CPU-using backend may still be doing useful work. A lock-waiting server
backend is not proof the importer is dead. If a confirmed abandoned import must
be cancelled, terminate only its exact OS PID and exact PostgreSQL backend PID;
then wait for rollback before restarting or retrying. Never use broad `pkill` or
start a competing importer.

After any failed deployment:

1. confirm the predeploy `.sql.gz` backup exists and is non-empty;
2. ensure `cq-server` is active, or deliberately keep it stopped while a required
   import holds schema locks;
3. inspect `journalctl -u cq-server` for the first terminal startup error;
4. finish or safely roll back the exact importer transaction;
5. restart the service and wait for world initialization before probing HTTP;
6. run every post-deploy invariant below.

Startup normally takes roughly eight seconds because the server synchronizes
scripted events and preloads collisions, actors, encounters, scripts, and warps.
The automated health check retries for roughly one minute, and longer when an
individual request reaches its timeout. A transient 502 during that window is
expected; a persistent 502 after the retry window requires journal inspection.

## WebTransport

Route normal HTTPS traffic to the server's HTTP API port. WebTransport uses HTTP/3
over UDP and connects to `/cq`; make sure the configured UDP port is open and
reaches the Go server directly.

The frontend fetches the certificate hash through the HTTP API and pins the
WebTransport connection in the browser.

## Operational Checks

- `npm run build` succeeds before uploading frontend assets.
- `dist/phaser/runtime_asset_contract.json` exists and is byte-identical to the
  validated contract under `public/phaser/`.
- `dist/sound/pokemon` contains rendered OGG files before upload.
- A live town-music URL returns `Content-Type: audio/ogg`, not the SPA HTML.
- `cd server && go test ./...` succeeds before replacing the server binary.
- The server starts with `DATABASE_URL` set and logs a successful world-manager
  initialization.
- The bootstrap/importer has been rerun after extractor pipeline changes or
  scripted-event generation changes.
- Unedited native `phaser_tiles` rows still match their
  `original_tile_image_id` values after the import.
- The served `/phaser/pokemon.db` SHA-256 equals `pokemonDbSha256` in the served
  runtime asset contract, whose `tileCatalogSha256` and `tileCount` match the
  deployed frontend catalog version.
- An overview request for a chunk derived from an actual non-erased Postgres
  tile returns `Content-Type: image/png`, a strong `ETag`, `Cache-Control:
  public, no-cache`, and the deployed hash in `X-Overworld-Tile-Catalog`; using
  a populated chunk proves the backend can decode the deployed tile-image
  family, rather than merely rendering a transparent gap. The same request with
  an intentionally different catalog hash must return HTTP 409 so an old browser
  bundle cannot mix new IDs with old art.
- `https://capturequest.net/api/online` succeeds after the startup retry window.
- Public `/api/admin/*` requests return 404, while the dashboard's loopback
  server-to-server health request succeeds with its backend-only key.
- Production Tile Art Studio routes such as `/api/tiles/stamps` return 404.
- Recent `journalctl -u cq-server` output contains full world initialization and
  no terminal scripted-event, schema, panic, or fatal error.

The tile-row invariant is:

```sql
SELECT COUNT(*)
FROM phaser_tiles
WHERE has_tile_edit = 0
  AND original_tile_image_id IS NOT NULL
  AND tile_image_id <> original_tile_image_id;
```

It must return `0`. Also verify that `phaser_data_repairs` contains any expected
one-time repair key exactly once and that the imported tile count is plausible
for the selected extractor release.
