# Scripted Events

This directory is the repo-owned source for CaptureQuest scripted-event data.
The server syncs these files into Postgres on startup before world managers
load, and `script-sim` runs the same sync before scenario tests.

## Files

- `scripts/*.json` owns generated rows in `phaser_cutscene_scripts`.
- `manual_scripts/*.json` owns CaptureQuest-authored rows in
  `phaser_cutscene_scripts`. These are tracked because they cover scenes that
  are not generated safely from source data yet. If a manual script has the
  same `scriptLabel` as a generated script, the manual script is the source of
  truth.
- `object_visibility.json` owns custom rows in
  `phaser_event_object_visibility`.
- `event_tile_overrides.json` owns hand-recovered compatibility tile rows.
- `event_tile_overrides.generated.json` is generated from extractor
  `script_event_tile_overrides` rows.
- `conditional_dialogue.generated.json` is generated from extractor
  conditional-dialogue rows and hydrated from `phaser_dialogue_text`.

Generated files should be reproducible from the tracked extractor SQLite
artifact and importer code. Hand-authored files should use
`trigger.source: "capturequest"` so importer runs do not overwrite bespoke
runtime behavior before an equivalent generated candidate is ready.

## Import Flow

Run this from `server/` when refreshing source-backed script candidates:

```bash
go run ./cmd/import-script-candidates
```

The command materializes supported extractor candidates into ignored
`scripts/*.json` files and writes the ignored diagnostic report
`scripted_events/script_candidate_import_diagnostics.json`. Use that report to
see what was generated, unchanged, skipped because a CaptureQuest override owns
the behavior, or left unsupported/ambiguous for future adapter work.

`cmd/import-phaser` also imports non-cutscene source data used by script
runtime systems, including in-game trades, missable-object mappings, boulder
targets, spin/arrow tile movement, and other `phaser_*` tables.

## Runtime Rules

The server owns durable gameplay state. Scripts may display dialogue and request
client cutscene animation, but lasting gameplay changes are applied server-side:
flags, item/money/coin changes, Pokemon gifts, battles, warps, object
visibility, tile overrides, Safari state, and progression.

Extractor-owned generated scripts can share a trigger only when each branch has
explicit conditions such as `requiresFlag`, `requiresFlagAbsent`,
`requiresItemName`, `requiresMoney`, `requiresCoins`, or
`requiresPlayerFacing`. Unconditioned duplicates should be resolved in the
importer instead of relying on DB ordering.

## Common Fields

- `requiresFlag` / `requiresFlagAbsent`: event-flag gates.
- `requiresFlags` / `requiresFlagsAbsent`: multi-flag gates.
- `requiresItemId` / `requiresItemAbsentId`: runtime item-id gates.
- `requiresItemName` / `requiresItemAbsentName`: source-friendly item gates
  resolved during sync.
- `requiresPokedexCaught`: caught-Pokedex count gate.
- `requiresMoney` / `requiresMoneyBelow`: Pokédollar gates.
- `requiresCoins` / `requiresCoinsBelow`: Game Corner coin gates.
- `requiresPlayerFacing`: server-side facing gate.
- `setsFlags`: completion flags applied after the cutscene completes.
- `warp`: post-cutscene destination update.

Keep this README focused on how the pipeline works. Detailed coverage belongs
in scenario tests, importer diagnostics, or adapter comments near the code that
owns the behavior.
