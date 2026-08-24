# Warps And Elevators

This note exists so future agents can find the source-of-truth for CaptureQuest warp behavior without re-discovering it from the database.

## Playable Warp Contract

`phaser_warps` rows are playable by the browser client only when they have:

- `destination_map_id`
- `destination_x`
- `destination_y`
- `warp_type` other than `elevator` or `inactive`

The server and client both assume complete destination data for playable rows:

- `server/internal/world/handler-phaser.go` only sends complete, playable `phaser_warps` rows to the client.
- `server/internal/world/phaser_warps.go` only loads complete, playable rows into the runtime warp manager.
- `src/phaser-game/managers/WarpManager.ts` ignores warps that do not have destination coordinates.

Do not invent fallback destinations for incomplete warp rows. Fix the source/importer data, or explicitly mark the row as non-playable metadata.

## Warp Type Meanings

- `door`: activated from or next to the tile.
- `carpet`: stepped-on or direction-sensitive warp tile.
- `elevator`: dynamic elevator placeholder. These are not playable `phaser_warps`; destination comes from `phaser_elevator_floors`.
- `inactive`: source row kept as metadata but not exposed as a playable browser warp. This includes unresolved `LAST_MAP` rows and static source rows that fail the original Game Boy activation checks.

`elevator` and `inactive` rows are intentionally allowed to have incomplete destination coordinates. Smoke tests fail if any other warp type is incomplete.

## Import Order

The relevant Postgres import order in `server/cmd/import-phaser/postgres.go` is:

1. Import raw maps, tiles, objects, warps, and warp events.
2. Resolve deterministic `LAST_MAP` destinations with `resolveLastMapWarpDestinationsPostgres`.
3. Clear placeholder `(0,0)` destination coordinates.
4. Resolve destination coordinates from source warp event ordinals.
5. Bake overworld global coordinates.
6. Classify warp activation metadata (`door`, `carpet`, or `inactive`).
7. Seed generated dungeon hole warps from `script_event_dungeon_hole_warps`.
8. Seed runtime data, including `phaser_elevator_floors`.
9. Mark dynamic elevator placeholders with `markDynamicElevatorWarpPlaceholdersPostgres`.
10. Mark unresolved `LAST_MAP` placeholders with `markUnresolvedLastMapWarpPlaceholdersPostgres`.

This ordering matters: Silph Co elevator placeholder rows can only be marked after `phaser_elevator_floors` exists.

## LAST_MAP

In the original Game Boy engine, `LAST_MAP` means `wLastMap`, not "the map a SQL query can guess". Most normal building exits can be resolved deterministically from incoming warp source data and destination warp ordinals. Those should become complete playable warps.

If a `LAST_MAP` row cannot be resolved to a destination map and coordinates after the normal importer pass, it is marked `inactive`. That prevents a broken row from appearing as a clickable or stepped-on warp.

Known current inactive row:

- `SILPH_CO_11F` at `(5,5)`, source `LAST_MAP, 10`. The imported row had no deterministic destination coordinates. The rendered tile is ordinary floor, not a Silph teleporter pad tile, so the importer marks it inactive instead of guessing.
- `SILPH_CO_1F` at `(16,10)`, source `SILPH_CO_3F, 7`. The row is present in source data, but the source tile is ordinary floor on a Facility map. In the original engine, Facility maps use the edge-facing extra warp check for non-door/non-warp tiles, so this non-edge row is not activatable during normal play.

## Elevators

Elevator floors are seeded in `server/cmd/import-phaser/runtime_seed.go` (`elevatorFloorSeeds`) into `phaser_elevator_floors`.

Current elevator maps:

- `CELADON_MART_ELEVATOR`: 5 floors, no key requirement.
- `ROCKET_HIDEOUT_ELEVATOR`: 3 floors, each requires item `74` (`LIFT KEY`).
- `SILPH_CO_ELEVATOR`: 11 floors, no key requirement.

The browser flow is:

1. Elevator room sign actor is clicked.
2. `src/phaser-game/scenes/tile-viewer/TileViewerInteractionController.ts` recognizes elevator map IDs `127`, `203`, and `236`.
3. Client sends `ElevatorFloorsRequest`.
4. `server/internal/world/handler-elevator.go` calls `AvailableElevatorFloors`.
5. The menu calls `ElevatorSelectRequest`.
6. `ElevatorDestination` validates the selected floor against the character's current elevator map, then teleports the player.

Silph Co is special because the static source warps inside `SILPH_CO_ELEVATOR` point at `UNUSED_MAP_ED`. That is a Game Boy dynamic placeholder. CaptureQuest keeps those rows as `warp_type='elevator'` metadata and uses `phaser_elevator_floors` for the actual selected destination.

Celadon and Rocket elevator rooms also have ordinary static exit rows in `phaser_warps`, but floor selection still uses the same sign-driven elevator request/select flow.

## Dungeon Holes

Dungeon hole/fall-through warps are one-way playable warps derived from Game Boy script data, not guessed from reciprocal map exits.

`npm run bootstrap:assets` runs `scripts/generate_dungeon_hole_warps.py`
against the pokered submodule and writes the derived rows into the extractor
SQLite database as `script_event_dungeon_hole_warps`; the normal asset sync then
copies that table into `public/phaser/pokemon.db`.

The generator reads:

- `tools/pokemon-gameboy-extractor-tool/pokemon-game-data/data/maps/special_warps.asm`
- map script calls to `IsPlayerOnDungeonWarp`

`server/cmd/import-phaser` reads only the generated SQLite table and then seeds
those source-derived hole coordinates into `phaser_warps` as playable `carpet`
rows. Live deployment should not require a raw pokered checkout or parse ASM on
the server.

## To Fix

- `VIRIDIAN_FOREST_NORTH_GATE` duplicate north exit: source row `warp_event 4, 0, LAST_MAP, 2` imports as playable warp `#199` at `(4,0) -> ROUTE_2 (3,-133)` with `carpet UP`, but that tile renders as the wall-side/top-edge tile. The adjacent source row `warp_event 5, 0, LAST_MAP, 2` imports as playable door warp `#200`, and Route 2's exterior entrance targets that row. Could fix at importer level by marking the duplicate non-door edge row inactive when an adjacent real door row shares the same destination and the inverse/exterior entrance resolves to the real door row. But this may introduce errors and needs further analysis.

## Validation

Useful checks:

```sh
cd server
go run ./cmd/import-phaser ../public/phaser/pokemon.db
go run ./cmd/db-smoke
go test ./...
```

`db-smoke` validates:

- Silph elevator dynamic placeholders are present.
- Silph elevator has 11 floor rows.
- Silph elevator has a clickable sign actor.
- All playable warps have complete destination map and coordinates.
