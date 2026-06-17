# CaptureQuest Server

The Go backend for CaptureQuest, a Pokemon Red/Blue-inspired multiplayer game
with server-owned durable gameplay state and a Phaser client.

## Requirements

- Go 1.24+
- Postgres 15+
- Node.js + npm for the Vite client

## Local Database

Bootstrap a local Postgres database from the repo-owned schema and generated
extractor output:

```bash
npm run bootstrap:fresh
```

The top-level bootstrap script initializes the bundled extractor submodule,
runs the extractor pipeline, syncs generated runtime assets, applies
`server/schema/postgres_runtime_schema.sql`, imports static Pokemon/map/trainer
data from `public/phaser/pokemon.db`, seeds deterministic CaptureQuest runtime
data, syncs scripted-event JSON, and runs database smoke checks.

If the extractor checkout is elsewhere, set
`POKEMON_EXTRACTOR_ROOT=/path/to/pokemon-gameboy-extractor-tool` or
`POKEMON_DB_SOURCE=/path/to/pokemon.db`.

To regenerate only assets and scripted-event files without touching Postgres:

```bash
npm run bootstrap:assets
```

For an already-created empty database, omit `--create`. The server expects this
schema shape at startup and does not mutate tables automatically. When schema
changes during development, rebuild the local database with `npm run
bootstrap:fresh`.

To preserve existing player/account state while only refreshing deterministic
static data, call the lower-level script directly without `--reset`:

```bash
cd server
./scripts/bootstrap_postgres.sh
```

You can run the import and smoke steps directly:

```bash
cd server
go run ./cmd/import-phaser ../public/phaser/pokemon.db
go run ./cmd/db-smoke
```

## Configuration

Config defaults live in `server/internal/config/capturequest_config_template.json`.
Local overrides can be placed in `capturequest_config.local.json`, which is
gitignored. `DATABASE_URL` takes precedence for runtime database connection
settings.

For local WebTransport certificates, generate `key.pem` in the config directory
if one is not already present:

```bash
cd server/internal/config
openssl genpkey -algorithm RSA -out key.pem -pkeyopt rsa_keygen_bits:2048
```

## Launch

From the repo root, run both server and client:

```bash
npm run dev:all
```

Or run the server directly:

```bash
cd server
go run ./cmd/server
```

## WebTransport

CaptureQuest uses WebTransport over HTTP/3 between the React client and this Go
server. Local development uses a dynamic certificate, a cert-hash endpoint, and
a pinned WebTransport connection to `https://127.0.0.1/cq`.
