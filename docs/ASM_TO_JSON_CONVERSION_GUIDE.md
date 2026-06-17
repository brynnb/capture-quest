# Cutscene & Event System Guide

## Overview

This project is a Pokémon Red/Blue MMO. The original game's story events were written in Z80 assembly. The current direction is data-driven scripted events:

1. **Extracted source data** — Original ASM-derived maps, objects, text pointers, and dialogue are imported into the `phaser_*` tables.
2. **JSON event scripts** — Extractor/importer-generated scripted events are written to ignored `server/scripted_events/scripts/*.json` files. CaptureQuest-authored exceptions live in tracked `server/scripted_events/manual_scripts/*.json` files. Server startup and `script-sim` sync both folders into `phaser_cutscene_scripts`.
3. **Trigger mappings** — Script files own their trigger metadata. Extractor-owned coordinate triggers stay in `phaser_coordinate_triggers`; CaptureQuest-authored coordinates can be owned by the script file with `trigger.source = "capturequest"`.

Avoid adding one-off Go cutscenes. If a script needs a capability the JSON event system does not have yet, add a generic action/runtime primitive and convert the event as data.

**This is an MMO**, so cutscenes are **per-player**. Each player sees their own cutscene independently.

---

## Architecture: Unified Actor Movement

Players and NPCs share the same movement system. The `PhaserActorManager` provides:

- **`FindPath()`** — Shared A\* pathfinding used by both players and NPCs
- **`SpawnTemporaryActor()`** — Creates a runtime actor (e.g., Oak during a cutscene) with a unique ID, broadcast to all clients
- **`RequestActorMove()`** — A\*-paths any actor to a destination with tick-based movement and animation broadcasts
- **`DespawnTemporaryActor()`** — Removes and broadcasts despawn
- **`processPathedMovement()`** — Runs in the 250ms simulation tick, advances all pathed actors

This means **NPCs are actors just like players**. Any actor can be spawned, moved via pathfinding, and despawned using the same APIs. The client's `ActorMovementController` handles walk animation for any actor receiving position updates, regardless of whether it's a player or NPC.

Key files:

- `server/internal/world/npc-manager-phaser.go` — Actor manager with pathfinding and movement
- `server/internal/world/player-movement-phaser.go` — Player movement (delegates pathfinding to actor manager)
- `server/internal/world/actor_registry.go` — Unique ID allocation for temporary actors

---

## Runtime Triggering

Runtime scripts are started by generic triggers:

1. Coordinate steps use `phaser_coordinate_triggers.label` and resolve through `phaser_cutscene_scripts.trigger_label`.
2. Map-entry scripts run after the Phaser scene finishes rendering and sends `PhaserMapScriptsRequest`.
3. NPC/object click scripting should use the same data-driven path as it is expanded; do not add per-scene Go hooks.

`PalletTownOakStopsPlayer` is now a data-driven cutscene row again. The old `cutscene_pallet_oak.go` hardcoded escort was intentionally removed.

---

## Source Data

The original ASM scripts live in the extractor submodule and generated SQLite
database:

```
tools/pokemon-gameboy-extractor-tool/pokemon.db
```

```sql
SELECT raw_asm FROM map_scripts WHERE script_label = 'PalletTownOakHeyWaitScript'
SELECT script_label, script_index FROM map_scripts WHERE map_name = 'OaksLab' ORDER BY script_index
```

Dialogue text is in the CaptureQuest Postgres runtime database:

```sql
SELECT label, dialogue FROM phaser_dialogue_text WHERE label LIKE '%OaksLab%'
```

---

## JSON Cutscene System

### Script File And `phaser_cutscene_scripts` Fields

| JSON field                 | DB column                    | Description                                           |
| -------------------------- | ---------------------------- | ----------------------------------------------------- |
| `scriptLabel`              | `script_label`               | Descriptive name (e.g., `CeruleanCityRivalEncounter`) |
| `mapName`                  | `map_name`                   | Must match `phaser_maps.name` — UPPER_SNAKE_CASE      |
| `trigger.type`             | `trigger_type`               | How the cutscene is triggered                         |
| `trigger.label`            | `trigger_label`              | Optional source label/text constant/object key        |
| `requiresFlag`             | `requires_flag`              | Player must HAVE this flag                            |
| `requiresFlagAbsent`       | `requires_flag_absent`       | Player must NOT have this flag                        |
| `requiresItemId`           | `requires_item_id`           | Player must have this item in inventory               |
| `requiresItemAbsentId`     | `requires_item_absent_id`    | Player must not have this item in inventory           |
| `requiresPokedexCaught`    | `requires_pokedex_caught`    | Player must have at least this many caught Pokemon    |
| `setsFlags`                | `sets_flags`                 | Flags to set on completion                            |
| `actions`                  | `actions`                    | The action sequence                                   |
| `warp.mapId`, `warp.x/y`   | `warp_to_map_id`, `warp_*`   | Optional post-cutscene warp                           |

### Trigger Types

- **`coord`** — Player steps on a specific tile (rival encounters, Oak stops you)
- **`map_script`** — Player enters a map with the right flags (post-battle dialogue, NPC exits)
- **`npc_click`** — Player clicks an NPC/object. Runtime matching checks `trigger_label` against the object's text constant, object name, and `object:<phaser_objects.id>`.
- **`object_click`** — Scenario/debugger terminology for an object click. Runtime storage still uses the same click-trigger path.

### Flag Conventions

- `EVENT_BEAT_<TRAINER>` — After defeating a trainer
- `EVENT_GOT_<ITEM>` — After receiving an item
- `EVENT_<DESCRIPTION>` — General story flags

The `requires_flag_absent` column prevents replay. Almost every cutscene needs this.

### Map Name Lookup

ASM uses CamelCase, DB uses UPPER_SNAKE_CASE. If unsure: `SELECT name FROM phaser_maps WHERE name LIKE '%SILPH%'`

| ASM Name       | DB Name          |
| -------------- | ---------------- |
| PalletTown     | PALLET_TOWN      |
| OaksLab        | OAKS_LAB         |
| ViridianCity   | VIRIDIAN_CITY    |
| CeruleanCity   | CERULEAN_CITY    |
| VermilionCity  | VERMILION_CITY   |
| BillsHouse     | BILLS_HOUSE      |
| GameCorner     | GAME_CORNER      |
| SilphCo11F     | SILPH_CO_11F     |
| PokemonTower7F | POKEMON_TOWER_7F |
| ChampionsRoom  | CHAMPIONS_ROOM   |

---

## JSON Action Types

Each cutscene is a JSON array of action objects executed sequentially.

| Action        | Description                                | Example                                                                         |
| ------------- | ------------------------------------------ | ------------------------------------------------------------------------------- |
| `lockInput`   | Disable player input. Always first.        | `{"type": "lockInput"}`                                                         |
| `unlockInput` | Re-enable player input. Always last.       | `{"type": "unlockInput"}`                                                       |
| `dialogue`    | Show dialogue box. Blocks until dismissed. | `{"type": "dialogue", "speaker": "OAK", "lines": ["Hey!", "Wait!"]}`            |
| `dialogueText` | Show imported dialogue by text label.     | `{"type": "dialogueText", "label": "OaksLabText1"}`                            |
| `showActor`   | Spawn a temporary cutscene sprite.         | `{"type": "showActor", "actor": "OAK", "x": 9, "y": 6, "sprite": "SPRITE_OAK"}` |
| `hideActor`   | Remove a temporary cutscene sprite.        | `{"type": "hideActor", "actor": "OAK"}`                                         |
| `move`        | Animate actor along a fixed path.          | `{"type": "move", "actor": "OAK", "movements": ["UP", "UP", "LEFT"]}`           |
| `movePlayer`  | Animate the player along a fixed path.     | `{"type": "movePlayer", "movements": ["DOWN", "DOWN"]}`                         |
| `facePlayer`  | Change actor facing direction.             | `{"type": "facePlayer", "actor": "OAK", "direction": "DOWN"}`                   |
| `delay`       | Pause for milliseconds.                    | `{"type": "delay", "ms": 500}`                                                  |
| `screenFade`  | Visual fade (currently a delay).           | `{"type": "screenFade", "fadeType": "out", "ms": 500}`                          |
| `setFlag`     | No-op on client. Documentation only.       | `{"type": "setFlag", "flag": "EVENT_FOO"}`                                      |
| `givePokemon` | Server-side reward on cutscene completion. | `{"type": "givePokemon", "pokemonId": 4, "level": 5}`                           |
| `giveItem`    | Server-side item reward.                   | `{"type": "giveItem", "itemId": 4, "quantity": 5}`                              |
| `takeItem`    | Server-side inventory removal.             | `{"type": "takeItem", "itemId": 70, "quantity": 1}`                             |
| `hideObject`   | Server-side per-player object hide.       | `{"type": "hideObject", "triggerLabel": "TEXT_OAKSLAB_CHARMANDER_POKE_BALL"}`  |
| `showObject`   | Server-side per-player object restore.    | `{"type": "showObject", "objectMapName": "CERULEAN_CITY", "objectKey": "CeruleanCity_NPC_6"}` |

- Use `{PLAYER}` and `{RIVAL}` as dialogue placeholders.
- Use `objectMapName` on `hideObject`/`showObject` when the source script changes a missable object on a different map, such as Bill's S.S. Ticket changing Cerulean City's guard objects.
- Use `''` (two single quotes) to escape apostrophes in SQL.

---

## How to Read the ASM

You don't need deep Z80 knowledge. Key patterns:

| ASM Pattern                        | Meaning               | Cutscene Equivalent                       |
| ---------------------------------- | --------------------- | ----------------------------------------- |
| `call DisplayTextID`               | Show dialogue         | `dialogue` action                         |
| `call MoveSprite`                  | Animate NPC movement  | `move` action (or Go `RequestActorMove`)  |
| `call StartSimulatingJoypadStates` | Force player movement | `movePlayer` action (or Go `RequestMove`) |
| `SetEvent EVENT_FOO`               | Set event flag        | `sets_flags` column                       |
| `CheckEvent EVENT_FOO`             | Check event flag      | `requires_flag` / `requires_flag_absent`  |
| `predef ShowObject` / `HideObject` | Show/hide NPC sprite  | `showActor` / `hideActor`                 |
| `ld [wCurOpponent], a`             | Start trainer battle  | Handled by trainer encounter system       |

**Script chaining:** The original game chains scripts via state machine indices. Collapse multi-step chains into one data-driven event where possible, or add generic action primitives when the DSL is missing a required behavior.

**Battle splits:** Pre-battle and post-battle are always separate cutscenes.

---

## Scene & Script Debugger

A built-in dev tool for testing cutscenes. A local-dev-only button in the game
UI opens a scrollable panel listing scripted scenarios in game-progression order.

### What "Jump to Scene" Does

1. **Clears all event flags** for a clean slate
2. **Sets the exact flags** needed for that scene to trigger
3. **Replaces your party** with an appropriately-leveled Pokémon
4. **Warps you** to the scene's trigger location

### Architecture

- **Backend:** `server/internal/world/handler-debug-scenes.go` — Hardcoded scene list in Go (no DB table). Two opcodes: `DebugSceneListRequest` → list, `DebugSceneJumpRequest` → setup + warp.
- **Frontend:** `src/components/Interface/SceneDebugger.tsx` — React component with Zustand store (`DebugSceneStore.ts`).
- **Adding scenes:** Edit the `debugScenes` slice in `handler-debug-scenes.go`. Each entry has `SeqNum`, `Label`, `Description`, warp coordinates, `SetFlags`, and `Pokemon`.

---

## CLI Scenario Testing

Use the script simulator for repeatable scripted-event validation without launching Phaser:

```bash
cd server
go run ./cmd/script-sim --scenario pallet_town_oak_stops_player --check
go run ./cmd/script-sim --scenario pallet_town_oak_stops_player --update --check
go run ./cmd/script-sim --all --check
```

Scenario files live in `server/script_tests/scenarios/*.json`; expected output lives in `server/script_tests/golden/*.golden`.

Each scenario should seed the exact starting state needed by the source script: map, coordinates, facing direction, flags, party, PC boxes, Day Care state, inventory, money, coins, hidden objects, Pokedex rows, active battle state, Safari Zone session state, Repel runtime state, and any battle or mechanic-specific fixture state. The simulator applies that fixture, syncs `server/scripted_events/` into Postgres, runs the trigger, applies server-side cutscene actions, and snapshots the final state.

Useful trigger types:

- `coord` checks extracted or CaptureQuest-owned coordinate triggers.
- `map_script` checks map-entry scripts.
- `npc_click` checks click-triggered NPC/object scripts by object name, text constant, or object key.
- `click_no_script` asserts that no scripted event claims the click, so normal fallback behavior such as opening a shop can run.
- Domain-specific triggers such as `fixture_state`, `active_battle_state`, `resolve_active_battle`, `dialogue_choice`, `elevator_floors`, `elevator_select`, `field_move_permission`, `gamecorner_hidden_coin`, `gamecorner_slot_play`, `safari_step`, `safari_battle_action`, `daycare_deposit`, `daycare_step`, `daycare_withdraw`, `repel_use`, `repel_step`, and boulder triggers cover non-cutscene mechanics.

Use `partyState` expectations when a scenario needs exact party HP, status, EXP, IV/EV, or move PP validation. The golden output only prints non-default HP/status/EXP/PP details so existing full-health party snapshots remain readable.

Use `finalDirection` when a scenario needs to prove the fixture's facing direction. The simulator persists fixture direction through `character_data.heading` for deterministic CLI validation; Phaser runtime movement still carries live facing direction through actor updates.

Scenarios can also assert server messages emitted through the fake session recorder:

```json
"messages": [
  {
    "channel": "stream",
    "opcode": 1189,
    "payloadContains": ["\"success\":true", "\"stepsLeft\":100"]
  }
]
```

---

## Scripts That Don't Need Conversion

| Pattern                                 | Why                                                |
| --------------------------------------- | -------------------------------------------------- |
| `*DefaultScript` / `*NoopScript`        | No-op initialization                               |
| `DisplayEnemyTrainerTextAndStartBattle` | Handled by trainer encounter system                |
| `EndTrainerBattle`                      | Generic post-battle cleanup                        |
| `CheckFightingMapTrainers`              | Trainer sight-range, handled by server             |
| `*PCScript`                             | PC interaction system                              |
| `*PlayerSpinningScript`                 | Spin tiles system                                  |
| **SafariZoneGate**                      | Safari Zone system (money check + ball allocation) |
| **SeafoamIslands boulders**             | Map mechanic, not cutscene                         |
| **Oak's Aides**                         | System-level Pokédex count check                   |

---

## Conversion Progress

Generated JSON scripts live under `server/scripted_events/scripts`; authored
exceptions live under `server/scripted_events/manual_scripts`. CLI scenarios
and golden files cover scripted behavior and gameplay mechanics.

**0 hardcoded server-side Go cutscenes** are intentionally supported. Server-side action effects like `givePokemon`, `giveItem`, `takeItem`, `hideObject`, and `healParty` are generic cutscene primitives, and `PalletTownOakStopsPlayer` is data-driven.

### Remaining Work

All high-priority and medium-priority scripts are complete. Remaining items are system-level features, not cutscenes:

- **Safari Zone gate** — Handled by `SafariZoneManager` (Phase 11.3, done)
- **Seafoam Islands boulders** — Map mechanic
- **Oak's Aides** — Pokédex count reward system

---

## Common Pitfalls

1. **SQL apostrophe escaping** — Use `''` inside SQL strings
2. **Map name mismatch** — Verify against `phaser_maps`. ASM = CamelCase, DB = UPPER_SNAKE_CASE
3. **Missing `requires_flag_absent`** — Without this, the cutscene replays every map entry
4. **Script chain collapse** — Group related ASM scripts into one logical cutscene
5. **Battle splits** — Pre-battle and post-battle are always separate
6. **Cutscene movement limitations** — JSON `move`/`movePlayer` are visual-only with fixed paths. For real cutscene pathfinding and collision avoidance, add a reusable movement action instead of a scene-specific Go hook
7. **Multi-phase cutscenes** — Model phases as data and flags; avoid synthetic labels and one-off Go hooks
