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
| `HTTP_PORT` | HTTP API port. Defaults to the configured value. |
| `DISCORD_CHAT_WEBHOOK_URL` | Optional private webhook for the CaptureQuest Discord channel. |
| `DISCORD_CHAT_SHARED_SECRET` | Optional 32+ character secret shared only with the IdleQuest Discord hub. |
| `HASH_PORT` | WebTransport certificate-hash HTTP port. |
| `ADMIN_KEY` | Admin API authentication secret. |
| `VITE_ASSET_URL` | Optional CDN/base URL for remote assets. |

Configure both Discord values or neither. A partial configuration prevents
startup. When enabled, public game chat is mirrored to Discord and signed
messages from the shared hub enter through `/api/discord/game-chat`; unsigned
requests are rejected. Keep both values out of Git and browser code.

## GitHub Actions Deploy

The deploy workflow builds the frontend and server, then uploads them to a
configured host over SSH. Configure these values in the GitHub repository
settings before enabling the workflow:

| Name | Type | Purpose |
| --- | --- | --- |
| `DEPLOY_SSH_KEY` | Secret | Private SSH key used by the action. |
| `DEPLOY_HOST` | Secret | SSH destination, for example `ubuntu@example.com`. |
| `DEPLOY_APP_DIR` | Repository variable | Absolute path to the app directory on the host. |

The workflow is the canonical deployment path. It performs this order:

1. install pinned Node, Python, and extractor dependencies;
2. restore a validated generated-asset cache or rebuild all graphics, SQLite,
   scripted events, and audio;
3. validate the runtime asset contract, build the frontend, run focused importer
   and warp tests, and build the Linux server/importer binaries;
4. upload the frontend and binaries, stop `cq-server`, and create a compressed
   Postgres backup under `$DEPLOY_APP_DIR/backups`;
5. negotiate the extractor contract before touching Postgres, apply the schema,
   and run the deterministic importer;
6. restart `cq-server` and Caddy, validate tile rows and the served asset contract,
   then retry the public health endpoint until startup completes.

Do not reorder schema negotiation, backup, schema application, or import. Do not
start a second deployment while an importer from the first one may still exist.

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
hash. A restored cache is accepted only after readiness checks find the database,
runtime contract, generated catalog version, graphics, sprites, scripted events,
and complete audio metadata; a partial or older cache must regenerate.
Freshly generated assets are saved immediately after those checks pass, before
frontend tests or deployment, so a later unrelated failure does not force the
extractor to repeat the same work on the next run.

## Generated Audio

Pokemon world, battle, SFX, and cry audio is generated under
`public/sound/pokemon/`. The directory is intentionally ignored by Git and is
about 50 MB, so a checkout alone is not a deployable frontend.

The required flow is:

```text
extractor/bootstrap -> public/sound/pokemon -> vite build -> dist/sound/pokemon
```

Verify both sides of the Vite build:

```bash
test -n "$(find public/sound/pokemon -name '*.ogg' -type f -print -quit)"
test -n "$(find dist/sound/pokemon -name '*.ogg' -type f -print -quit)"
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

The current production layout follows the same tar-over-SSH pattern as the
Vayeate website. Set these explicitly for the target environment:

```bash
export CQ_DEPLOY_HOST='ubuntu@54.68.252.253'
export CQ_DEPLOY_APP_DIR='/home/ubuntu/app/capture-quest'
export CQ_DEPLOY_KEY='/absolute/path/to/capturequest_deploy'
```

After completing the build and asset checks above, upload the complete frontend:

```bash
tar --exclude='._*' --exclude='.DS_Store' -czf - -C dist . \
  | ssh -o BatchMode=yes \
      -o ServerAliveInterval=15 -o ServerAliveCountMax=20 -o TCPKeepAlive=yes \
      -i "$CQ_DEPLOY_KEY" "$CQ_DEPLOY_HOST" \
      "mkdir -p '$CQ_DEPLOY_APP_DIR/dist' && cd '$CQ_DEPLOY_APP_DIR/dist' && tar xzf -"
```

Caddy serves `/home/ubuntu/app/capture-quest/dist` for `capturequest.net`.
Static-file additions do not require a Caddy reload.

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
the client connection. Establish a new keepalive-enabled SSH session and inspect
both layers:

```bash
pgrep -af '[i]mport-phaser'

set -a
. /etc/capturequest/cq-server.env
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
- `https://capturequest.net/api/online` succeeds after the startup retry window.
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
