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
