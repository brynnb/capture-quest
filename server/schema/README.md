# CaptureQuest Database Schema

`postgres_runtime_schema.sql` is the canonical schema for a fresh CaptureQuest
runtime database. Keep this file focused on tables the current server,
importer, and scripted-event pipeline actually use.

Normal setup starts from an empty Postgres database:

```bash
npm run bootstrap:fresh
```

The bootstrap path initializes the extractor submodule, generates
`public/phaser/pokemon.db` and related runtime assets, applies this schema,
seeds deterministic CaptureQuest runtime data, syncs file-backed scripted
events, and runs database smoke checks.

Use `docs/DATABASE_BOOTSTRAP.md` for setup details.

The server-foundations movement checkpoint adds `character_movement_receipts`
with one versioned latest ordinary-step result per character. The table is added
idempotently and cascades on character deletion. Apply the tracked schema before
starting a binary that requires receipts; startup fails if it is absent. Ordinary
step receipt writes share the position/effects transaction. This addition requires
no database reset. For an authorized production release, use the schema-aware
full-data workflow in `docs/DEPLOYMENT.md`.

The forced-route checkpoint adds `character_movement_routes`, one versioned current
remaining path and committed source pose per character. Its writes join the
position/effects transaction; the historical step receipt stays separate. Apply
the tracked schema before starting the updated server; startup requires this
table. This is an additive migration with no player reset. Use the schema-aware
full-data deployment workflow and backup rules for an authorized release.
