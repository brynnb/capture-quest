# Database Bootstrap

CaptureQuest uses one runtime database: Postgres. Fresh local and deployment
databases are built from the repo schema plus reproducible extractor output, not
from private database dumps or development migration history.

## Source Artifacts

- `server/schema/postgres_runtime_schema.sql`: canonical fresh schema.
- `tools/pokemon-gameboy-extractor-tool`: bundled extractor submodule that
  generates `public/phaser/pokemon.db`, runtime map assets, battle sprites, and
  scripted-event JSON.
- `server/scripted_events/*.json`: file-backed object visibility, conditional
  dialogue, and tile override data.
- Runtime seed code in `server/cmd/import-phaser`.

## Local Setup

From the repo root:

```bash
npm run bootstrap:fresh
```

That command applies the Postgres schema, imports deterministic static data from
SQLite, seeds CaptureQuest runtime data, syncs scripted events, and runs database
smoke checks. It also initializes and runs the extractor submodule before the
database import.

To refresh only generated assets and scripted-event files without touching
Postgres:

```bash
npm run bootstrap:assets
```

To rerun the lower-level server bootstrap directly:

```bash
cd server
./scripts/bootstrap_postgres.sh --create --reset
```

Omit `--reset` when you need to preserve local account/character state.

During development, schema changes are handled by rebuilding the local database
from the flattened schema instead of mutating tables at server startup. If the
server reports a missing column or stale constraint after a schema edit, rerun
the fresh bootstrap path.

## Runtime Shape

The Postgres schema intentionally contains the tables CaptureQuest currently
uses: accounts, characters, Pokemon party/PC/progression, inventory/shops,
scripted events, Phaser static data, chat/filter tables, and local character
creation seed tables.

Systems that have no Pokemon gameplay role are not part of the fresh schema. If
a future feature needs schema evolution after real deployment history matters,
add a small native Postgres migration layer on top of this flattened base.
