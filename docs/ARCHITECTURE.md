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

For active-player shutdown acceptance, run the same isolated runner with
`CQ_E2E_SHUTDOWN_MODE=success` and separately with `failure`, without a test-file
argument. These dedicated modes verify the exact owned executable and private
cluster before signalling, exercise browser movement plus final persistence,
and collect the child process exit status. Evidence includes SQL before/after
values, socket closure, logs and a screenshot. Failure injection is private and
character-specific. This proves the WebSocket path; active-character browser
WebTransport and interrupted concurrent commands require additional coverage.

For scripted event changes, prefer the script-test CLI and golden expectations
under `server/script_tests/` instead of relying only on browser inspection.

For durable gameplay changes, run `bash scripts/testing/run-go-postgres.sh` from
the repository root. It uses a disposable PostgreSQL cluster and tests rollback,
concurrency, ownership, and cancellation against the runtime database engine.

For rendered local acceptance checks, use
`bash scripts/testing/run-isolated-e2e.sh tests/e2e/game-corner.spec.ts`.
It creates a private Unix-socket PostgreSQL cluster under `/var/tmp`, bootstraps
matched local artifacts through the canonical database script, starts an owned
server/frontend on available dedicated ports, and runs local Playwright Chromium.
It does not source `.env` or connect to the normal development database. Logs,
ports, exact owned process IDs and failure traces remain in the printed evidence
directory. Cleanup stops only those owned processes and that private cluster;
a cleanup deadline failure makes the run fail. Existing generated assets and a
local Playwright browser installation are prerequisites; this runner does not
regenerate game data or deploy anything.

---

## 1. Data Ownership & Streams

Runtime foundation work and its verification milestones are tracked in
[`SERVER_FOUNDATIONS.md`](SERVER_FOUNDATIONS.md).

### Connection boundary

Every connection receives a new session ID and authenticates before account or
gameplay commands. IDs and IP addresses are not reconnect credentials. Saved
battle state is restored after authentication and character selection.

Both transports use the shared inbound frame contract in `internal/api`:
256 KiB maximum command size, JSON object payloads, and centralized session
prerequisites in the world dispatcher. Gameplay defaults to requiring a selected
character; domain handlers still validate battle rules, ownership, and GM access.
WebTransport accepts one reliable control stream. Heartbeats can use datagrams;
session expiry bounds idle connections and fixed deadlines bound partial frames
and writes. Removing a session closes only that session's transport.

The dispatcher serializes commands per session, with at most 32 running/waiting
callbacks and a five-second admission deadline. Character entry additionally
claims the world's character-owner registry. Replacement closes and drains the
old session, completes its cleanup, then reloads the character. Cleanup evicts
shared state only for the matching owner; stale disconnects cannot remove a new
owner. Concurrent handoffs fail entry and can be retried. The movement timer
enters this same gate for each player's path advancement, publication and step
effects. It skips busy owners until the next tick and rejects replaced movement
registrations. Session commands and cleanup publish a value-only `Presence`
snapshot before releasing their gate. Cross-player broadcasts, player actor lists
and NPC player-collision checks read that snapshot rather than another session's
mutable fields. They never acquire the recipient's command gate. Closed sessions
immediately return empty presence. Other background writers and actor state
publication still require auditing and coordination with this ownership boundary.

Actor, movement and session-maintenance timers share an idempotent periodic-worker
lifecycle. World shutdown seals session admissions, closes connections, joins
these workers and drains character cleanup, including removals already claimed by
transport callbacks. Final cleanup persists playtime. Chat database writes remain
inside their owning command and its drain, with a five-second query deadline.
The server owns its HTTP listener and joins active HTTP handlers before draining
the world and closing its captured database connection. Startup returns listener
and TLS failures to main rather than logging them inside detached goroutines.
`/api/ready` checks listener startup, failure/draining state and a one-second
database ping. Unexpected HTTP or WebTransport serve failures notify main, clear
readiness and initiate cleanup followed by a nonzero exit. Required world preload
errors propagate through construction before timers/listeners start. Cutscenes
require the canonical prerequisite columns; they never retry weaker query shapes.
Database ping, tile schema upgrades, scripted-event synchronization and required
world preloads share the process signal context and one minute for bootstrap and
preload together. Every database query in these stages observes cancellation;
preloads also check before cache publication. Failed bootstrap or construction
closes the opened database. Lazy collision queries have a five-second deadline.
Scripted-event sync loads all file inputs before opening a PostgreSQL transaction.
It locks the five published content tables to serialize publishers, upgrades
required columns with IF NOT EXISTS and applies scripts, coordinates, visibility,
tile overrides and conditional dialogue together. Applied statistics and success
logs follow commit; errors, cancellation and commit failure preserve the previous
family, including schema changes. Schema upgrades can block readers until commit;
sync runs during startup/import before serving traffic. Local file reads remain
synchronous. A bounded server shutdown deadline remains in progress.

We follow a **"Model-First"** architecture. Data is categorized into distinct streams to avoid massive "god-object" updates and to minimize bandwidth.

### Durable State Ownership

**The server owns durable gameplay state.**

- **Casing & Naming**: Keep server field names, database columns, and Go struct tags aligned with the runtime model.
- **Movement**: The Phaser client owns ordinary walking/pathing responsiveness and reports its current position to the server. The server persists and relays that position to other players, while still applying durable step effects such as encounters, Safari steps, cut tiles, scripted triggers, and forced movement mechanics.
- **Adaptation**: The client code and Tygo types adapt to the server's structure. Map location state should use `mapId`; `zoneId` only remains where older protocol/data aliases still need compatibility.
- **Automation**: Explicit JSON tags drive standard Go encoding and Tygo generation. Character state, wallet and bind streams use this contract. Remaining legacy world messages still pass through `StructToMap` while the documented migration proceeds.

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
- **Mutations**: `internal/economy` coordinates shop operations; the transaction-aware
  `internal/db/cqitems.Store` handles inventory storage. The shared transaction
  boundary commits payment and grants together. Join the supplied transaction;
  never query a global database from inside a transaction. Lock the character
  before reading mutable balances/quantities and publish success only after commit.
- **Offers**: Each shop offer includes its merchant ID. A department store can show
  several clerks' offers, but a purchase must identify the selected offer's owner.
- **Wire types**: Generate inventory types from `internal/db/cqitems/types.go`;
  do not duplicate those interfaces in client stores.
- **Party item use**: `internal/itemuse.Service` owns outside-battle party item
  operations. It locks the character before loading inventory and party, applies
  the existing shared effect rules, and commits consumption, Pokémon changes,
  and evolution Pokédex registration together. Its returned snapshot is safe to
  publish only after success. A TM/HM move-selection prompt writes no gameplay
  state, and the later selection revalidates the current item and moves. Field
  movement effects remain separate migration work. Battle item turns use the
  commit boundary described below.

### D. Pokémon Party, PC, and Battle State

- **Source of Truth**: `character_pokemon`, `character_pc_state`, and battle state tables.
- **Content**: Pokémon level, EXP, HP, IV/EV values, moves, PP, status, party slots, PC boxes, active battles, and capture state.
- **Rule**: Gameplay stat progression belongs to Pokémon, not the trainer avatar.
- **Persistence**: `pokebattle.Pokemon.RowID` is the stable owned Pokémon identity;
  `ID` remains the species ID. Party saves update rows in one bounded transaction,
  preserve nicknames and other storage metadata, and reject foreign, duplicated,
  or omitted existing identities. Releasing or transferring a Pokémon must use
  the explicit storage operation. A missing species/move or malformed row fails
  the load; it must never silently shrink a party that could then be saved.
- **Composition**: Use `SavePartyInTransaction` when the party change accompanies
  inventory consumption, rewards, or another durable effect. It returns a copied
  pending snapshot; publish it only after the outer commit. `SaveParty` owns its
  transaction and publishes new row IDs after commit. Storage moves, acquisitions,
  Day Care, and trades take the same character lock before mutable reads.
- **Battle commits**: `pokebattle.StartBattle`, `CommitBattle`, `ResumeBattle`, and
  `CloseBattle` own bounded transactions and the character lock. Starts reload the
  party under that lock. Turns operate on a private clone and compare the saved
  battle ID/revision before applying effects. Party changes, battle item consumption,
  captures, experience, trainer prizes/defeat records, battle flags, and blackout
  charges commit with the resumable battle. Publish only the returned snapshot.
  Forced switches and pending move choices use this same boundary.
- **Reconnect**: Every published turn is already durable. Disconnect evicts memory
  without rewriting an old snapshot. Resume retains the durable record and joins
  player battle-local state to owned Pokémon by row ID. Version 2 preserves status
  counters, stat stages, pending choices, enemy sound/evolution metadata, and party
  membership. Explicit legacy version-zero saves remain readable and upgrade on
  the next commit. Unknown versions or mismatched membership fail visibly and
  retain the record. Client close cannot discard an active battle or pending choice.
- **Limits**: Database revision checks prevent applying two competing copies of
  the same revision; they do not deduplicate sequential client commands or serialize
  transport publication. Post-battle cutscene actions join the battle transaction
  through the shared interpreter. Runtime ownership, remaining reward paths, and
  recovery of an end notification lost during disconnect remain in the foundations
  goal. Do not claim full battle/reconnect correctness from persistence tests alone.

### E. Session/Ephemeral Data

- **Content**: Current target, NPC movement state, dialogue/cutscene progress, and transient map-session state.
- **Volatility**: Never touches the database. Exists only as long as the session.

---

## 2. No "Shadow Models" (DTO Tax)

One of the primary goals is to keep wire types aligned with the server's runtime model.

- **Anti-Pattern**: Manually building a `map[string]interface{}` or a custom struct that mirrors 90% of a server model. This introduces "Shadow Models" that drift and break.
- **Standard**: Reuse tagged owned models under `server/internal/db/models` and encode them directly. A wire view belongs in `server/internal/protocol` when its shape differs from persistence. `protocol.CharacterData` embeds the base fields and adds typed parsed preferences; the stored options string is excluded from JSON. Do not add new `StructToMap` consumers.

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
   // ItemResponse is a tagged protocol view with a flattened embedded item.
   ses.SendStreamJSON(ItemResponse{Item: itemData, Success: true}, opcodes.GetItemResponse)
   ```
3. **Composite Responses**: When a single response _must_ contain multiple distinct lists (e.g., `GetRecipeDetailsResponse` with `recipe`, `components`, and `outputs`), nesting is acceptable. This is the exception, not the rule.
4. **Error Responses**: Always use a flat structure with `success: false` and `error: "..."`.

---

## 5. Type Generation & Validation

### Tygo (The Standard)

Tygo generates TypeScript from authoritative Go types. Run `npm run tygo`
after changing contracts. `server/tygo.yaml` defines generated modules and explicit
external type mappings. Encoding uses `encoding/json`; exported wire fields have
explicit `json` tags. Go anonymous fields flatten in JSON, so flattened views use
Tygo's supported `tstype:",extends"` tag and an imported base type for generation.

### Wire-contract migration

The migration is in progress. Its final state is tagged, typed messages encoded
directly, with TypeScript generated from those same declarations. No runtime
field-name conversion or generated-name postprocessor will remain.

1. **Completed: character streams.** Base model fields now have explicit JSON
   names. CharacterData uses `protocol.CharacterData` and generated
   `protocol.ts`; options use the existing authoritative `CharacterOptions`
   declaration in `character_options.ts`. Persisted `models.CharacterData.Options`
   is a JSON string and is excluded from serialization; the wire view contains
   the parsed object. Wallet and bind models encode directly. These modules are
   excluded from the legacy casing postprocessor. NetworkBridge and the player
   store consume the wire type rather than the persistence type.
2. **In progress: world queries and gameplay messages.** Pokédex species,
   status and trainer-card payloads now live in protocol declarations and encode
   directly. Their success/error union uses literal discriminators in generated
   TypeScript; nullable species fields are explicit unions, optional cry metadata
   is omitted when absent, and success collections are arrays even when empty.
   The bridge, store and Pokédex consumer share the generated declarations.
   These queries use their world's database dependency and return failure on
   query, scan or iteration errors instead of publishing partial snapshots.
   Pokémon, move and item detail requests/projections also live in protocol.
   Their flat success views embed the projections; all nullable fields match
   explicit TypeScript unions. The content query service owns the SQL reads,
   uses an injected database and caps each operation at five seconds while
   honoring caller cancellation. Transport handlers decode requests and publish
   responses without SQL scanning or reflection. The migration corrects hP to
   hp, pP to pp, isHM to isHm and defaultMove1 to defaultMove1Id (and the other
   default move fields), using their already declared JSON names. Database
   errors are distinguished from missing records. Local TM/HM preview records
   use a generated-field projection instead of pretending to be full responses.
   Map-script and learnset aggregates now also use the service and protocol
   types. They read under one read-only repeatable-read transaction and one
   five-second budget, returning no partial projection on any failure. A shared
   typed row collector closes results between queries and reports scan/iteration
   failures. Empty lists are arrays. Their direct encoding corrects rawASM to
   rawAsm and tMHMName/isHM to tmHmName/isHm. The map-ready request uses its
   generated request type. Map-ready cutscene issuance requires its requested
   name to match the server's native player location; post-battle selection uses
   the same resolver. Interior identity comes from the owned map ID. Overworld
   identity comes from original tile source-map provenance at the owned movement
   position, with a five-second query budget. Missing or conflicting provenance
   rejects issuance; editing or erasing tile art does not change script identity.
   Scripted NPC/object clicks resolve the current runtime position followed by
   per-character position overrides and reject hidden targets before puzzle/door
   mutations or token issuance. Interaction requires adjacency or a straight
   two-tile reach across an effective talk-over tile, including eligible event
   tile properties. The authorization loaders share one five-second budget and
   injected database; SQL failures reject the request. Direct trainer interaction
   and battle-start requests use that same reach/visibility boundary and recheck
   after dialogue. Their metadata queries and battle start use the captured world
   database; rebattle policy comes from the owned session's typed options.
   Dialogue choices bind the runtime actor to its actual text constant and source
   map before trade or action execution. Script-owned prompts require issued
   completion instead. Required choice flags are rechecked in the same locked
   transaction as effects. In-game trades use the bounded transaction helper and
   publish the party snapshot loaded inside that successful transaction.
   Other interaction paths and shared issuance authorization still need auditing.
   Remaining families must follow the same migration: extract substantive
   response families, reuse tagged types, replace map enrichment with flat
   typed views, and audit actual keys, nullability and empty collections against
   consumers. No permanent fallback aliases should be introduced.
3. **Remaining: retire legacy conversion.** Replace all StructToMap/ItemToMap
   consumers, require explicit tags on the published contracts, remove the helper
   and `scripts/fix-tygo-casing.sh`, and simplify the generation command. Existing
   map payloads must become typed responses as part of the same audit.
4. **Remaining: integrated verification.** Prove transport encoding, generated
   shapes, success/error cases, reconnect and stale-client behavior, then run
   frontend build and rendered gameplay checks. Breaking families require a
   coordinated frontend/backend release with explicit stale-client handling.

The first stage preserves established camelCase character keys and the existing
flat shape. Options were already sent as an object; the generated type now
represents that fact. Nil deletedAt/options fields are omitted and typed optional.
No new wire version or fallback aliases were introduced in this stage. The
legacy world casing rules remain isolated until their consumers are migrated.

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

The shared `DBTX` interface is defined in `server/internal/db/transaction.go`
(and aliased by `pokebattle`). It is satisfied by `*sql.DB`, `*sql.Tx`, and matching
test doubles:

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

### Script transaction boundary

`ApplyCutsceneScript` commits script rewards, completion flags, and final position
with the character lock. `ApplyCutsceneActionList` uses the same interpreter for
ordinary action lists; battle settlement supplies its existing transaction and
private party. The mutation context accumulates publication work and messages
until commit. Nested lists share that context. Scripted battle startup joins it
rather than opening a second database connection while the character is locked.

Event flags refresh their cached snapshot after durable writes; batches and
toggles use the character transaction. Scripted rewards no longer publish success
before a later completion write can fail. This is not durable notification
recovery: session ownership, issued-event authorization, all completion eligibility
checks, and reconnect delivery remain tracked in `SERVER_FOUNDATIONS.md`.

Cutscene completion additionally requires the session's issued completion token,
matching character and label. The server executes the retained script snapshot,
not a client-selected catalog entry. Tokens have a bounded lifetime/capacity and
are consumed only after successful transaction completion; concurrent claims
are rejected. The frontend and backend must move together for this wire change.
This authorization is distinct from durable command deduplication and delivery
recovery across reconnects, which remain unfinished.

The opcode dispatcher executes prerequisite validation and gameplay handlers
through `Session.ExecuteCommand`, shared by reliable and datagram traffic. The
bounded gate admits at most 32 running/waiting callbacks and bounds admission
waits. Callbacks must not recursively enter it. Disconnect closes transport and
uses `DrainCommands` before character cleanup. Inline character quit already runs
inside the gate. This session serialization does not replace the still-required
character owner shared across sessions and world/timer writers.

### Dynamic puzzle transactions

Vermilion trash-can interactions read durable lock flags and can indices under
the shared character lock. Initialization, selecting a replacement can and
setting/resetting lock flags commit in one bounded transaction. The live handler
injects its database; simulator wrappers invoke the same implementation.
Returned outcomes and cached flags become visible only after commit. A failed
cache refresh is logged as a publication problem, not described as a rollback.
Repeated clicks are ordinary state transitions; durable command identity and
reconnect result recovery remain tracked in `SERVER_FOUNDATIONS.md`.

Item-ball collection uses the same character lock for the collection marker and
inventory grant. Its response carries the inventory/wallet snapshot loaded
inside the successful transaction. Actor authorization shares scripted-target
visibility/position rules, with a stricter one-tile pickup reach. Silph doors
read Card Key ownership and durable unlock flags under the character lock before
committing an unlock; cache contents do not determine the durable transition.

Game Corner prize purchases load catalog identity, Coin Case ownership and the
coin balance inside one character-locked transaction. Pokémon/TM grants, Pokédex
registration and payment share its commit. The live prize handler injects its
database, checks owned reach/visibility to the selected source prize window and publishes
the transaction's inventory/wallet snapshot after commit. Simulator purchase by
name shares this operation. Remaining catalog-list dependencies are tracked in
the foundations roadmap.

Coin purchases, spins and hidden-coin collection also acquire the character lock
before eligibility/balance reads and compose through the bounded transaction
helper. Their live coin/slot handlers use the owned database and owned map
position; simulator wrappers share the operation. Wallet payment, bet/payout and
collection-marker/grant pairs each commit atomically. Failed transactions discard
speculative results. Durable replay/reconnect recovery remains tracked in the
foundations roadmap.

The live slot endpoint now names a generated hidden-object coordinate instead
of accepting luck. The server resolves source `StartSlotMachine` records, applies
availability rules and rechecks owned reach for every spin. Luck uses the bundled
ASM draw/index rule and synchronized character/session state; presence publication
resets that state on map departure, and cleanup clears it. Source hidden-object
order is preserved by exported/imported IDs. Explicit Go JSON request/response
types generate the frontend contract. Old targetless packets fail closed. The
existing React slot component replaces the retired parallel Phaser slot overlay;
rendered flow and remaining asynchronous response-correlation checks are tracked
in `SERVER_FOUNDATIONS.md`.

Prize purchases resolve the selected prize's existing source window and require
current reach/visibility to its unique imported sign. They reuse the authoritative
actor/counter interaction evaluator before invoking the atomic purchase operation.
A catalog listing alone grants no purchase authorization. The standalone simulator
still exercises the same transaction operation without pretending to simulate a
rendered player's physical interaction.

Coin purchases resolve the unique imported `TEXT_GAMECORNER_CLERK1` actor on
map `135` through the same bounded source-identity helper as prize windows.
The existing actor/counter evaluator checks current owned position and visibility
before the atomic payment/grant operation. Map membership or proximity to another
clerk grants no purchase authorization. Missing/ambiguous source identities fail
closed rather than selecting an arbitrary actor.

Escape Rope resolves its imported exit, validates and consumes the owned item,
and saves the normalized character position inside one bounded character-locked
transaction. Session/movement position and the teleport response are published
after commit. The shared teleport helper can apply an already committed position
without another independent persistence operation. A failed commit leaves both
inventory and saved/live position unchanged. This does not yet migrate every
field move or provide durable response recovery after a disconnect.

FLY uses the same committed-position publication boundary. Its character-locked
transaction checks current party move knowledge and the durable badge flag, then
matches the requested map and coordinates against the unique `poke_start_cities`
row served to the current destination UI. Unknown coordinates and ambiguous
catalog identities cannot authorize teleportation. Position publication follows
commit; cached event flags do not decide this durable effect. The shared
field-move eligibility evaluator supports both transaction-backed effects and
the existing simulator/presentation flag view. The current all-cities policy
remains; historical outdoor/visited-town rules need further eligibility work.

Repel has one durable owner: `character_repels`. Activation checks the current
counter and owned inventory inside a character-locked transaction, then consumes
the item and stores the effect together. Step updates/expiry are bounded durable
operations; notifications follow commit. The old per-manager pointer map and
disconnect deletion are retired. Runtime and simulator managers receive an
explicit database, and a replacement manager reads the committed counter.
Preload/readiness and database smoke checks reject a missing effect schema.
The existing eligible encounter-tile timing remains; broader movement timing,
throughput and durable message recovery need their own integration evidence.


Safari has one durable owner: `character_safari_state`. Its versioned JSON holds
visit counters and the current encounter. Reads return independent snapshots;
mutations reload under the shared character lock. Entry payment/flags and capture
storage/Pokédex changes join that transaction, including scripted outer commits.
Exhaustion saves the gate destination before publishing an exit. The source gate
script clears the visit; status requests read the committed battle for recovery.
Malformed/unsupported snapshots report errors rather than silently starting a
new visit. Startup requires the schema, and fixtures use the same storage API.

Field destinations and scripted moves end Safari outside the zones/gate inside
their position transaction. Recovery warp commits ordinary battle deletion,
Safari cleanup and destination together. Script movement publication performs no
second save. Reported positions and forced teleports use the captured database
and bounded character transaction before live publication. Movement flushes
release the shared player lock before saving a coordinate snapshot; only a
matching registration/position is marked clean after commit. A failed save stays
dirty for later retry. Ordinary forced-path movement and its step effects still
have separate persistence boundaries. Disconnect final-flush failure policy,
cache refresh failure, durable command/result delivery and throughput remain in
`SERVER_FOUNDATIONS.md`; fresh-manager recovery and rendered Safari checks do not
prove abrupt network/process recovery or completion of those wider requirements.

Session commands now retain the original admission deadline through
`Session.CommandContext()` and cancel it when the connection closes. Migrated
handlers use it for their context-aware queries/transactions; movement saves
accept it explicitly. Cancellation requests cooperation: the gate and cleanup
barrier still wait for the callback to return. Disconnect persistence uses a
separate context after that barrier, so closing a connection does not cancel its
final flush. Legacy managers and helpers still require cancellation migration;
unbounded lifecycle waits have not yet been replaced. Cancellation after a
successful commit does not undo that durable result.

Safari and Repel manager operations, script action/completion transactions and
battle start/resume now accept explicit execution contexts. Runtime callers
carry the session owner's context into these shared boundaries; fixture and
simulator callers choose their own context. Scripted battle creation also
requires the captured database rather than a global-database wrapper. The old
standalone Safari map-exit cleanup is retired: destination persistence owns that
cleanup in its existing transaction. Legacy reads, post-commit flag refresh and
other lifecycle operations still require migration before end-to-end shutdown
bounds can be established.

General character saves and playtime increments now use explicit database/context
arguments and own bounded transactions. A cancelled autocommit statement can
return before its backend finishes; owning the transaction prevents a blocked
cancelled write from later committing and being counted again on retry. Cache
invalidation follows commit, and zero affected rows are an error. Cleanup shares
one five-second persistence budget and returns final-position/playtime errors;
handoff propagates them before admitting a replacement. Disconnect still retires
failed session state and logs errors, without a durable recovery queue. Lifecycle
joins and shutdown failure reporting remain unfinished.

Shutdown has one owned completion per world/server. Context-aware callers can
stop waiting at a deadline and rejoin later; timeout leaves storage open beneath
unfinished work. Joined final-save errors are returned through the server, whose
standalone process reports shutdown failure with a nonzero exit. The configured
`gracePeriod` is seconds, with a 30-second default when nonpositive. Underlying
legacy operations still need cancellation/force-close coverage; a bounded wait
alone does not prove a bounded successful drain or durable final-save recovery.

At server drain start, world retirement and HTTP request cancellation begin
before HTTP joins. Ordinary HTTP connections are force-closed after the grace
period; a sealed handler-admission boundary and handler completion tracking keep
storage open even after force-close until admitted work returns. Player transports
are retired by their session owners. The optional chat bridge owns one worker
with cancellable HTTP deliveries/retry timers and safe close-before-start.
Legacy uncooperative operations and WebTransport reader/listener joins still
require coverage; force-close is reported as failed graceful drain.

Transport lifecycle ownership is shared by WebSocket and WebTransport upgrade
handlers/readers. Admissions seal at drain start; the listener and admitted tasks
join before storage closes. HTTP/3 ConnContext owns accepted QUIC connections,
which close before UDP is released so peers receive termination and blocked
stream operations wake. quic-go v0.44.0 fixes the observed datagram error race
and includes the earlier ConnContext fix; webtransport-go remains v0.8.0.
Real Go transport checks cover idle/partial control streams and datagrams; they
do not replace integrated active-player persistence or rendered browser checks.
