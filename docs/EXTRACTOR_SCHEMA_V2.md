# Pokémon extractor schema-v2 integration

CaptureQuest consumes `pokemon-gameboy-extractor` schema version 2 through an
explicit, separately versioned adapter contract. The canonical extractor
database remains project-neutral. The JSON adapter bundle is the portable,
inspectable snapshot of that contract; the Go bulk importer reads the same
SQLite database directly after independently repeating equivalent schema and
release negotiation.

`scripts/bootstrap_capturequest.sh` performs these steps in order:

1. generates the extractor database and graphics, using the complete atomic
   audio build when `gbsplay`, `ffmpeg`, `rgbasm`, and `rgblink` are available;
2. runs `adapters.capturequest_v2` for the selected Red or Blue release and
   atomically publishes `public/phaser/capturequest-pokemon-import.json`;
3. copies the exact validated SQLite input and scoped assets;
4. validates all 561 audio manifest records and their 1,122 upstream FLAC/Ogg
   hashes, then publishes only the compact Ogg derivatives to the web tree;
5. only then starts the Postgres bootstrap.

Select a release with `CAPTUREQUEST_POKEMON_RELEASE=red` or `blue`. An explicit
`POKEMON_DB_SOURCE` is honored by validation, asset synchronization, and the
Postgres import.

The Go importer repeats schema negotiation before opening Postgres. It rejects
unknown schema versions/readers, missing release links/tables, foreign-key
errors, and inconsistent Pokémon default moves before any schema creation or
truncate. It persists the selected release and extraction run in
`phaser_import_metadata`.

Encounter terrain crosses the boundary through native source metadata:
`tilesets.grass_tile_id` and each placed square's
`tiles.raw_encounter_tile_id`. The importer derives runtime encounter areas from
those values and `source_map_id`; generated `tile_image_id` values and rectangular
map bounds are deliberately not gameplay semantics.

`import-phaser --preflight-only` performs that negotiation without reading
Postgres configuration or opening Postgres. Both the standalone database
bootstrap and deployment invoke it before database creation or reset.

Pokémon default moves cross the compatibility boundary as source constants
(`SAND_ATTACK`, `PSYCHIC_M`), resolved through `phaser_moves.constant_name`.
They are never joined to display or short names.

`LAST_MAP` warps retain `destination_kind`, `destination_warp_id`, and null
static destinations. At request time the server resolves them from each
session's own previous-map context; no shared or guessed destination is stored.
This allows two players to leave the same interior for different entry maps.

Useful checks:

```bash
cd server
go test ./cmd/import-phaser -run 'TestNegotiateExtractorSource|TestValidatePokemonDefaultMoves'
go test ./internal/world -run TestLastMapWarp
```
