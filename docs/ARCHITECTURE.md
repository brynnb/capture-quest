# CaptureQuest Architecture & Data Philosophy

> [!IMPORTANT]
> **Data Consistency Warning**: Avoid manual patches directly to the `capturequest` database for structural or extraction issues (e.g., missing Warp IDs, incorrect NPC directions). These changes **will be overwritten** the next time the data is reprocessed. All permanent fixes should be implemented in the respective extraction pipeline scripts (e.g., `pokemon-gameboy-extractor-tool`).

This document outlines the core architectural principles of CaptureQuest to ensure consistency, performance, and maintainability across the Go backend and React frontend.

CaptureQuest uses its own Postgres runtime database. Do not depend on private
local database dumps; use the bundled extractor submodule and repo-owned import
paths.

---

## Development Workflow

### Runtime Database

CaptureQuest's runtime database target is Postgres. Prefer `DATABASE_URL` for
local development:

```bash
export DATABASE_URL=postgres://localhost:5432/capturequest?sslmode=disable
```

Static Pokemon, map, trainer, encounter, and tile data comes from the extractor
submodule through the generated SQLite artifact at `public/phaser/pokemon.db`.
The usual setup path is:

```bash
npm run bootstrap:fresh
```

For asset-only regeneration without touching Postgres:

```bash
npm run bootstrap:assets
```

Generated scripted-event JSON under `server/scripted_events/scripts` and
tracked authored scripts under `server/scripted_events/manual_scripts` are
synced into Postgres by the importer and server startup paths. Pipeline fixes
should start in the extractor/import layer unless the data is explicitly
CaptureQuest-owned.

### Validation

Use focused validation for the touched area. Good defaults are:

```bash
cd server && go test ./...
npm run tygo
npm run build
git diff --check
```

For scripted event changes, prefer the script-test CLI and golden expectations
under `server/script_tests/` instead of relying only on browser inspection.

---

## 1. Data Ownership & Streams

We follow a **"Model-First"** architecture. Data is categorized into distinct streams to avoid massive "god-object" updates and to minimize bandwidth.

### Backend Authority (The "Golden Rule")

**The server is the absolute source of truth.**

- **Casing & Naming**: Keep server field names, database columns, and Go struct tags aligned with the runtime model.
- **Adaptation**: The client code and Tygo types adapt to the server's structure. Map location state should use `mapId`; `zoneId` only remains where older protocol/data aliases still need compatibility.
- **Automation**: We rely on the `StructToMap` utility in Go and `tygo` in TypeScript to handle the bridge—we do not manually mangle the backend to accommodate the frontend.

### A. Persisted Base Data (`CharacterData`)

- **Source of Truth**: Postgres runtime tables exposed through owned Go model structs in `server/internal/db/models`.
- **Content**: Character identity, cosmetic selections, map position, login metadata, and options.
- **Update Frequency**: Persisted on logout/zoning/interval.
- **Rule**: Do not add trainer combat stats here. Pokémon combat state belongs in Pokémon party, PC, and battle tables.

### B. Persistent Configuration (`Options`)

- **Source of Truth**: The `options` JSON field in the database.
- **Content**: UI preferences, local development toggles, and other explicit player options.
- **Sync**: Managed by `useGameStatusStore` on the client and `Client.SaveOptions()` on the server.

### C. Inventory & Items

- **Source of Truth**: `cq_character_inventory`, `cq_item_instances`, and `cq_items` tables.
- **Strategy**: Managed as a high-volume independent stream. The client may derive display-only sorting, grouping, and filtering, while the server validates inventory changes.

### D. Pokémon Party, PC, and Battle State

- **Source of Truth**: `character_pokemon`, `character_pc_state`, and battle state tables.
- **Content**: Pokémon level, EXP, HP, IV/EV values, moves, PP, status, party slots, PC boxes, active battles, and capture state.
- **Rule**: Gameplay stat progression belongs to Pokémon, not the trainer avatar.

### E. Session/Ephemeral Data

- **Content**: Current target, NPC movement state, dialogue/cutscene progress, and transient map-session state.
- **Volatility**: Never touches the database. Exists only as long as the session.

---

## 2. No "Shadow Models" (DTO Tax)

One of the primary goals is to keep wire types aligned with the server's authoritative runtime model.

- **Anti-Pattern**: Manually building a `map[string]interface{}` or a custom struct that mirrors 90% of a server model. This introduces "Shadow Models" that drift and break.
- **Standard**: Prefer the owned model structs under `server/internal/db/models` and the `StructToMap` utility in `world-utils.go`. `StructToMap` converts Go's `PascalCase` fields to the frontend's `camelCase` shape while respecting the model's structure.

### When to use a DTO (The "Last Resort" Exceptions)

We only use explicit DTO structs when one of the following is true (otherwise, use fine-grained streams of raw models):

1. **Derived Gameplay Views**: The data has no direct database table equivalent and is calculated live, such as active battle summaries or encounter previews.
2. **Specialized Views (Need-to-Know)**: You must expose a tiny fraction ( < 10%) of a model for massive bandwidth savings, such as the `CharacterSelectEntry`.
3. **Handshake/Meta**: Non-state data like Success/Failure markers, JWT tokens, or heartbeat timestamps.

**Rule**: **Avoid "Aggregation Blobs."** Do not create giant DTOs that combine character data, inventory, party, and battle state into one message. Instead, send multiple independent streams of the raw models.

**Rule**: If a DTO is just a "pretty version" of a database model, **delete it** and use the model directly.

---

## 3. The "Need-to-Know" Principle (Performance)

We fetch the bare minimum data required for the current user context.

### Character Selection Screen

- **Goal**: Show a list of characters quickly.
- **Constraint**: Do **not** fetch inventory, party, PC, or battle details.
- **Columns**: Only select identity, selected faction/class, gender, map, and last-login fields.
- **Reasoning**: Prevents N+1 query problems where logging in would otherwise trigger dozens of sub-queries for data the user has not chosen to play yet.

---

## 4. Flat Payloads (Network Protocol)

When the server sends a response, **the opcode already identifies the data type**. We do not nest the data inside a redundant property.

### Anti-Pattern (Nested)

```json
// OpCode: CharacterData
{
  "character": { "id": 1, "name": "Elara", "mapId": 38 }
}
```

The client must then "unwrap" this: `const char = response.character;`

### Standard (Flat)

```json
// OpCode: CharacterData
{ "id": 1, "name": "Elara", "mapId": 38 }
```

The client can use the response directly: `const char = response;`

### Guidelines

1. **State Streams**: For `CharacterData`, inventory, party, PC, and battle data, send the model or map directly.
2. **Query Responses**: For request/response APIs such as `GetItemResponse`, include `success: true` on the root object alongside the data fields.
   ```go
   // Example: GetItemResponse
   res := StructToMap(itemData).(map[string]interface{})
   res["success"] = true
   ses.SendStreamJSON(res, opcodes.GetItemResponse)
   ```
3. **Composite Responses**: When a single response _must_ contain multiple distinct lists (e.g., `GetRecipeDetailsResponse` with `recipe`, `components`, and `outputs`), nesting is acceptable. This is the exception, not the rule.
4. **Error Responses**: Always use a flat structure with `success: false` and `error: "..."`.

---

## 5. Type Generation & Validation

### Tygo (The Standard)

We use `tygo` to generate TypeScript interfaces directly from Go structs (stored in `src/net/generated/`). This ensures 100% type alignment between the backend and frontend without manual duplicating fields.

- **Workflow**: When a Go struct is modified, run `npm run tygo` from the project root. **Never run `tygo generate` directly** because the npm script also runs `scripts/fix-tygo-casing.sh`, a post-processor that converts PascalCase field names to camelCase in the generated files. Running tygo directly will produce PascalCase output that breaks the client.
- **Why the post-processor exists**: Tygo outputs Go field names as-is for structs without `json` tags, including the owned database model structs in `server/internal/db/models`. These are PascalCase in Go, but `StructToMap` sends them as camelCase on the wire. The fix script bridges this gap by lowercasing the first letter and handling the `ID` suffix convention.
- **Use**: Always use `generated/world_api.ts` or `generated/models.ts` for casting incoming network data.
- **Config**: `server/tygo.yaml` — maps Go packages to output TS files.

### Casing Pipeline (CRITICAL)

There are **two independent casing systems** that must stay in agreement:

1. **Tygo** generates TS interfaces from Go structs using `json` tags for field names.
   - Go `MapID int \`json:"mapId"\``→ TS`mapId: number`
2. **`StructToMap`** (`world-utils.go`) converts Go structs to `map[string]interface{}` at runtime using **Go field names** (ignores json tags):
   - `ID` → `id`
   - Fields ending in `ID` (e.g., `MapID`) → `mapId` (special suffix rule)
   - Everything else: lowercase first letter (e.g., `SightRange` → `sightRange`)

**These two systems must produce the same keys.** They agree by convention because:

- Go field `MapID` with `json:"mapId"` → Tygo produces `mapId`, StructToMap produces `mapId` ✓
- Go field `Name` with `json:"name"` → Tygo produces `name`, StructToMap produces `name` ✓

**Rules to avoid casing bugs:**

- **Always add `json` tags** to exported struct fields, and ensure the tag matches what StructToMap would produce from the Go field name.
- **Never use acronyms in field names** except `ID` at the end (e.g., use `MapId` not `MapID` only if you want `mapId` — but the convention is `MapID` which StructToMap handles specially).
- **Manual `map[string]interface{}`** payloads bypass both systems — keys must be hand-written in camelCase matching the client's expectations. Double-check these against Tygo types.
- **After adding/modifying Go structs**, always re-run `npm run tygo` (from project root) and verify the output in `src/net/generated/world_api.ts`.

## 6. Client Communication & Logic

### NetworkBridge (Dispatch)

The `NetworkBridge` is the central "Air Traffic Controller" for the application.

- It listens to `WorldSocket.onJson`.
- It casts incoming data to Tygo types.
- It dispatches data to the appropriate `Zustand` stores or `Services`.
- **Static Handlers**: High-level routing (like handling `CharacterData`) should happen here to keep stores focused on state rather than networking.

### Stores vs. Services

- **Stores (Zustand)**: Responsible for **State and Reactivity**. If the UI needs to re-render when a value changes, it belongs in a store (e.g., `PlayerCharacterStore`).
- **Services (Classes)**: Responsible for **Orchestration and Logic**. If a task involves sequential network requests, complex timers, or logic that isn't strictly "state" (e.g., `CombatService`), it belongs in a service.

### Flattened Store Logic

The `PlayerCharacterStore` should ingest server data by spreading it into the `characterProfile`.

- Favor: `{ ...state.characterProfile, ...serverData }`
- Avoid: Deeply nested "response" objects that force the UI through needless wrapper properties. Maintain a flat access pattern for UI components.

---

## 7. Database Access Pattern

### Preferred: Dependency-Injected `DBTX` Interface

New server-side code should accept a `DBTX` interface parameter rather than reaching for the `db.GlobalWorldDB.DB` singleton directly.

```go
// GOOD — testable, explicit dependency
func LoadPokemonFromDB(db DBTX, pokemonID int) (*Pokemon, error) {
    err := db.QueryRow(`SELECT ... FROM phaser_pokemon WHERE id = $1`, pokemonID).Scan(...)
}

// BAD — hidden global dependency, untestable
func LoadPokemonFromDB(pokemonID int) (*Pokemon, error) {
    myDB := db.GlobalWorldDB.DB  // don't do this in new code
    err := myDB.QueryRow(...).Scan(...)
}
```

The `DBTX` interface (defined in `server/internal/pokebattle/dbloader.go`) is satisfied by both `*sql.DB` and any test mock:

```go
type DBTX interface {
    Query(query string, args ...interface{}) (*sql.Rows, error)
    QueryRow(query string, args ...interface{}) *sql.Row
    Exec(query string, args ...interface{}) (sql.Result, error)
}
```

**Why**: Functions that accept `DBTX` can be unit-tested with a test database or mock without touching the global singleton. Callers (handlers) pass `db.GlobalWorldDB.DB` at the call site.

**Legacy code** (e.g., `cqitems.go`, `character-db.go`, handler files) still uses the global directly. No need to refactor existing code, but **all new functions should use the `DBTX` pattern**.

---

## 8. Coding Safeguards

4. **No Manual Edits to Generated Files**:
   - **Backend**: Do not manually modify clearly automated directories or generated files. `server/internal/db/models` is source, not generated.
   - **Frontend**: Never manually edit `src/net/generated/`. These are derived from the server's source of truth. If a type is wrong, fix it on the server (or the generator/mapping logic) and re-run the generation tool.
   - **Automation is absolute**: We adapt the client to the server, not the other way around.

---

## 9. Asset Infrastructure & Pipeline

CaptureQuest serves game assets from generated public asset folders and the deterministic data/import pipeline.

### A. Storage & Distribution

- **Structure Standard**: Phaser assets live under `public/phaser` and are served by the app/server in development and deployment.
- **Optimization**: Generated manifests and cacheable static assets should be reproducible from the extractor submodule and CaptureQuest source data.

### B. Sprite Sheets & Texture Atlases

Character and NPC animations are driven by 2D sprite sheets and texture atlases.

- **Format**: Animation frames are packed into single texture files (`.png`) and indexed via JSON metadata.
- **Efficiency**: Phaser's Animation Manager handles frame sequencing and texture swapping, allowing for high-performance rendering of hundreds of entities.
- **Dynamic Loading**: Assets are loaded on-demand based on the current zone or visible entities to optimize memory usage.

---

## 10. 2D Rendering (Phaser)

The `src/phaser-game/` directory contains the core 2D rendering engine, built with Phaser 3.

### A. Architecture Overview

- **React Integration**: The `PhaserEngine.tsx` component wraps the Phaser game instance, managing its lifecycle within the React application.
- **Scene-Based**: Phaser uses a scene-based architecture where `TileViewer.ts` is the main game scene handling map rendering, NPC display, and user interaction.
- **Manager Pattern**: Separate managers handle distinct concerns:
  - `UiManager`: HUD elements, loading indicators, and info panels
  - `TileManager`: Tile image loading and caching
  - `NpcManager`: NPC sprite management and animations
  - `MapRenderer`: Tile, item, warp, and NPC rendering to the game world
  - `CameraController`: Camera movement, zoom, and input handling

### B. Data Services

- **MapDataService**: Fetches map tiles, items, warps, and NPC data from the server.
- **WebSocketService**: Real-time updates for NPC movement and tile changes.
- **Asset Loading**: Tile images are loaded from the CDN via the `TileManager` and cached for performance.

---

## 11. Pokemon Data Pipeline

The project uses an extraction and reprocessing pipeline to transform original
Game Boy data (ASM, 2bpp, BLK files) into generated CaptureQuest assets, a
SQLite import artifact, and a Postgres runtime database.

### A. Master Reprocessing Pipeline (`reprocess.py`)

To ensure data integrity and correct coordinate calculation, all data reprocessing should be handled via the master script in `pokemon-gameboy-extractor-tool/export_scripts/reprocess.py`.

This script orchestrates the following sequence:

1. **`export_map.py`**: Extracts raw map data, tilesets, and collision info.
2. **`update_zone_coordinates.py`**: The "Spreader" script. It calculates global (X, Y) offsets for overworld maps based on Pallet Town (0,0) and saves them to `overworld_map_positions`.
3. **Map/tile expansion scripts**: Generate 16x16 Phaser tiles. They treat Game Boy collision data as a "whitelist" of walkable tiles and use the spreader offsets to apply global coordinates.
4. **`export_objects.py`**: Extracts NPCs, Signs, and Items from assembly files.
5. **`update_object_coordinates.py`**: Finalizes NPC placement by mapping their local map coordinates to the global (X,Y) grid established by the spreader.

### B. Coordination & Consistency

- **Source of Truth**: The `pokemon.db` SQLite file acts as the intermediate source of truth for all game assets.
- **Global Grid**: The "spreader" ensures that Route 1 is placed exactly above Pallet Town, matching the seamless transition logic used in the game engine.
- **Walkability Whitelist**: Unlike modern engines, the original engine defines walkability as a specific list of "passable" tile IDs. The pipeline respects this "inclusive" collision model.

---

## 12. Overworld Map ID Consolidation

> [!WARNING]
> **Goal**: All overworld references should use the unified map ID **9999**. Individual sub-map IDs (e.g., `0` for Pallet Town, `19` for Route 8) are a legacy artifact of the extraction pipeline and should **not** be relied upon in game logic.

The original Game Boy game stores each town and route as a separate map with its own ID. Our extraction pipeline stitches these into a single seamless overworld rendered as map `9999`. Some source/import tables still retain original map names or IDs for lookup and coordinate derivation:

- **`phaser_warp_events`**: Warp events still reference sub-map IDs internally for coordinate resolution, but the client should never see these.
- **Server handlers**: The server normalizes all overworld map IDs to `9999` before sending data to the client.

**Rule**: Any new code that deals with overworld maps should use `9999` exclusively. If you find yourself checking for a specific sub-map ID (e.g., `mapId === 0` or `mapId === 19`), that's a sign something upstream isn't normalizing correctly. Fix the data source, don't add special cases.

---

## 13. Common Debugging Pitfall: Name Casing & Formatting

> [!CAUTION]
> **Map and constant name mismatches are the single most common source of silent data bugs in this project.** Always double-check casing, underscores, and suffix formatting when debugging missing or incorrect data.

The project juggles multiple naming conventions across the pipeline:

| Format           | Example        | Where Used                                          |
| ---------------- | -------------- | --------------------------------------------------- |
| CamelCase        | `Route8Gate`   | ASM filenames, `warp_events.map_name`               |
| UPPER_SNAKE_CASE | `ROUTE_8_GATE` | `maps.name`, `warp_events.dest_map`, game constants |
| lowercase        | `route_8_gate` | Some older scripts                                  |

**Known problem patterns**:

- **Floor suffixes** (`B1F`, `B2F`, `1F`, `2F`): A naive CamelCase→UPPER_SNAKE converter splits `Route8Gate` correctly to `ROUTE_8_GATE` but mangles `MtMoonB1F` into `MT_MOON_B_1_F` instead of the correct `MT_MOON_B1F`. This caused 35 maps (141 warps) to silently fail resolution.
- **Number-adjacent letters**: `Route11Gate2F` must become `ROUTE_11_GATE_2F`, not `ROUTE_11GATE_2F` or `ROUTE_11_GATE_2_F`.
- **Compound names**: `SSAnne` → `SS_ANNE`, `UndergroundPath` → `UNDERGROUND_PATH`.

**Debugging checklist** when data appears missing or incorrect:

1. Check if the map/constant name resolves to a valid `maps.id` — a `NULL` `map_id` in `warp_events` or `phaser_warp_events` means the name lookup failed silently.
2. Compare the exact string in the database against the expected UPPER_SNAKE_CASE form — copy-paste and diff, don't eyeball it.
3. If adding a new name converter or lookup, test it against the known tricky names: `MtMoonB1F`, `SSAnneB1FRooms`, `Route16Gate1F`, `SeafoamIslandsB3F`.

---
