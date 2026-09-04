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

The SQLite database and numeric tile artwork are published as one validated
runtime family:

```text
public/phaser/pokemon.db
public/phaser/tile_images/tile_*.png
public/phaser/runtime_asset_contract.json
src/constants/runtime_asset_version.ts
```

The contract records the exact SQLite hash, tile count, and complete tile-catalog
hash. The generated TypeScript constant versions every browser tile URL with the
catalog hash because numeric tile IDs are not stable across unrelated extractor
catalogs. Never publish the database, tile directory, contract, or compiled
frontend independently.

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

`server/cmd/import-script-candidates` compiles the extractor's versioned,
relational script candidates into CaptureQuest event JSON. Both SQLite
consumers use `server/internal/extractorcontract` for the same schema-v2,
release, integrity, and provenance negotiation. The script compiler additionally
rejects candidate JSON whose version is unsupported or whose duplicated
relational identity columns disagree. Runtime execution, offline simulation,
and candidate compilation share the action wire contract in
`server/internal/scriptedactions`.

Candidate publication is serialized per output root and prepared as one
rollback-capable multi-file operation. To verify generated files in CI without
changing them, run:

```bash
cd server
go run ./cmd/import-script-candidates --check \
  --sqlite ../public/phaser/pokemon.db \
  --output scripted_events/scripts \
  --release red
```

The check fails on stale output, stale extractor schemas, and any new or
increased unsupported-diagnostic category. The reviewed unsupported counts are
regression ceilings, not accepted long-term targets.

Grass encounter eligibility uses the original tileset header's grass ID and the
bottom-right native 8×8 sample exported for each placed 16×16 square. It never
uses deduplicated browser `tile_image_id` values, which can be renumbered by a
catalog rebuild.

The `phaser_tiles` refresh is a transactional, indexed, set-based merge. Imported
source values update the `original_*` snapshot; explicit manual tile mutations
remain current when `has_tile_edit = 1`. One-time cleanup is tracked separately
in `phaser_data_repairs`. Do not replace the set-based stage join with repeated
correlated lookups or truncate the runtime tile table during a normal refresh.

Source-derived browser runtime helpers should be generated into SQLite before
deploy. For example, `scripts/generate_dungeon_hole_warps.py` reads pokered ASM
from the extractor submodule and writes `script_event_dungeon_hole_warps` into
`public/phaser/pokemon.db`; the live importer consumes that generated table and
does not parse raw ASM on the server.

Audio has a separate static-asset flow:

```text
pokered audio source data
  -> extractor 48 kHz stereo FLAC masters + compact 24 kHz mono Ogg derivatives
  -> npm run bootstrap:assets
  -> only browser Ogg derivatives under public/sound/pokemon
  -> npm run audio:manifest
  -> client-side asset availability checks
```

The login, character select, and character creation screens intentionally keep
the CaptureQuest title music. World, battle, bike, surf, item, warp, move, and
Pokemon cry audio prefer source-derived assets when those files are present.
Music is streamed through an HTML media element. Only three small common UI
effects are decoded during audio startup; cries, moves, and other effects are
decoded on demand. Never put the full audio library in the manifest's `global`
array, because decoded browser PCM is vastly larger than the Ogg files.

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
- The unified overworld has a distant-view transport at
  `GET /api/overworld/overview?chunkX=<signed>&chunkY=<signed>`. Each aligned
  64-by-64 logical-tile chunk is rendered from current Postgres rows as a
  256-by-256 PNG (4 pixels per tile). Its strong ETag includes both rendered
  content and `tileCatalogSha256` from the atomic runtime asset contract.
  Successful overworld paint and erase transactions invalidate only their
  affected overview chunks. Collision and interaction always use exact tile
  rows, never overview pixels.
- The browser checks the no-cache `runtime_asset_contract.json` before every
  map load and periodically during exact chunk requests. If an already-open
  bundle's embedded tile-catalog hash differs from the newly deployed contract,
  it performs one cache-busted page reload before accepting tile IDs. The
  overview endpoint also rejects a mismatched `catalog` query with HTTP 409.
- Exact data uses camera-derived aligned 64-by-64 chunks. The resident request
  plan is capped at 3 by 3 chunks (192 by 192 tiles), includes a 50-by-50
  minimum close-view intent, and unloads GPU textures and collision data as the
  camera leaves them. Distant views use overview PNGs; a hysteresis band keeps
  pinch zoom from rapidly swapping both layers near the LOD boundary.
- Overview PNGs represent durable shared world state. Character-specific event
  tile overrides remain in the authenticated exact layer, so a door or similar
  event tile can resolve to that character's state only when the close layer
  replaces the overview; overview pixels never authorize movement or warping.
- The overview renderer reads `dist/phaser/tile_images` in deployment and
  `public/phaser/tile_images` in local development. Set
  `CAPTUREQUEST_TILE_IMAGE_DIR` and `CAPTUREQUEST_RUNTIME_ASSET_CONTRACT`
  together only for a nonstandard layout; they must identify one atomic
  generated asset family.
- SQLite is an import/source artifact only, not a runtime database.
- Generated scripted-event files should be changed in the extractor/import
  pipeline first. CaptureQuest-specific helper files under `server/scripted_events`
  remain repo-owned source data.
- Pokemon battles render in an overlay UI on top of Phaser rather than replacing
  the world scene.
