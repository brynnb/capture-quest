# Data Pipeline Overview

Warp and elevator import/runtime behavior is documented in
[`WARPS_AND_ELEVATORS.md`](WARPS_AND_ELEVATORS.md).

CaptureQuest keeps extracted static data, generated scripted events, and runtime
player state in separate streams.

## Source Artifacts

- `tools/pokemon-gameboy-extractor-tool`: bundled extractor submodule. This is
  the reproducible source for generated Pokemon maps, tiles, sprites, SQLite
  data, and script candidates.
- PokeAPI sprite URLs: bootstrap fetches the 96x96 front/back Pokemon battle
  sprites at setup time, so those generated PNGs are not committed.
- `server/schema/postgres_runtime_schema.sql`: Postgres runtime schema base.
- `server/scripted_events/*.json`: CaptureQuest-owned generated-data helpers and
  overrides that are imported alongside extractor output.
- `server/scripted_events/manual_scripts/*.json`: tracked CaptureQuest-authored
  scripted events for scenes the generator cannot safely own yet.

The generated runtime outputs under `public/phaser`, `public/assets/pokemon`,
`public/assets/trainers`, `public/sound/pokemon`,
`server/scripted_events/scripts`, and `src/constants/*audio_manifest.json` are
ignored by Git and rebuilt with `npm run bootstrap:assets` or
`npm run bootstrap:fresh`.

## Import Flow

```text
pokered source data
  -> extractor scripts + CaptureQuest asset-pipeline generators
  -> CaptureQuest generated asset folders + public/phaser/pokemon.db
  -> server/cmd/import-phaser
  -> Postgres runtime database
```

`server/cmd/import-phaser` reads SQLite and writes Postgres. It refreshes
`phaser_*` static tables, derives encounter areas, seeds CaptureQuest item/shop
runtime data, classifies warp activation metadata, and syncs scripted-event JSON
into the database.

Source-derived browser runtime helpers should be generated into SQLite before
deploy. For example, `scripts/generate_dungeon_hole_warps.py` reads pokered ASM
from the extractor submodule and writes `script_event_dungeon_hole_warps` into
`public/phaser/pokemon.db`; the live importer consumes that generated table and
does not parse raw ASM on the server.

Audio has a separate static-asset flow:

```text
pokered audio source data
  -> extractor audio manifest/rendered OGG files
  -> npm run bootstrap:assets
  -> generated files under public/sound/pokemon
  -> npm run audio:manifest
  -> client-side asset availability checks
```

The login, character select, and character creation screens intentionally keep
the CaptureQuest title music. World, battle, bike, surf, item, warp, move, and
Pokemon cry audio prefer source-derived assets when those files are present.

## Key CaptureQuest Files

- `server/cmd/import-phaser/`: SQLite-to-Postgres importer and deterministic
  runtime seed data.
- `server/internal/world/handler-phaser.go`: server handlers for maps, tiles,
  actors, warps, movement, and related Phaser requests.
- `server/internal/world/scripted_events*`: scripted-event runtime and sync.
- `src/phaser-game/`: Phaser renderer/controller.
- `TODO.md`: current working queue.

## Design Notes

- The server owns durable gameplay state; the client owns ordinary walking
  responsiveness and reports position updates for persistence and multiplayer
  visibility.
- SQLite is an import/source artifact only, not a runtime database.
- Generated scripted-event files should be changed in the extractor/import
  pipeline first. CaptureQuest-specific helper files under `server/scripted_events`
  remain repo-owned source data.
- Pokemon battles render in an overlay UI on top of Phaser rather than replacing
  the world scene.
