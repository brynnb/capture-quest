# Server command and ownership audit

Inventory baseline: 2026-10-03, `f1e7aec` plus the login diagnostic checkpoint.
This is the finite work inventory for [SERVER_FOUNDATIONS.md](SERVER_FOUNDATIONS.md),
not a completion claim. The registry contains 87 inbound commands; each appears
once below. A grouped family is not an estimate of equal effort. Registered
commands are only one boundary: HTTP routes, script-internal writers and
background owners are listed separately.

For each family, close the audit only after checking source authorization,
character ownership, database cancellation/atomicity, stable command/entity
identity, publication after commit, typed contracts and client recovery. Record
applicable rejection/duplicate/rollback/timeout/cancellation/stale-response and
reconnect evidence; use process-death and rendered acceptance where the behavior
needs them. Mark inapplicable gates with the reason. Existing checkpoint evidence
in the foundations document is reusable; do not reimplement or retest it without
a changed dependency or uncovered risk.

## Registered command families

The source inventory is `server/internal/world/world-handler-registry.go`.
“Baseline” means implemented scope with existing checkpoint evidence, **not** a
closed audit of the entire row. “Queued” means it still needs that explicit audit.

| Family and status | Commands | Current evidence and remaining work |
| --- | --- | --- |
| Account and character lifecycle — investigation open | `JWTLogin`, `CharacterCreate`, `DeleteCharacter`, `EnterWorld`, `ValidateNameRequest`, `CharacterQuitRequest` | Session prerequisites and drained character handoff exist. Login has an unresolved restore timeout; shared transaction stage diagnostics and bounded reproduction are recorded in the foundations document. Audit remaining global/uncancellable reads, deletion, failed-entry cleanup and final-save recovery. |
| Options, chat and liveness — queued | `SetOption`, `SendChatMessage`, `Heartbeat` | Central session gate exists. Audit option persistence and lost updates, chat bounds/cancellation and lifetime rules; these do not all require gameplay mutation identity. |
| Creation/static queries — queued | `StaticDataRequest`, `CharCreateDataRequest` | `world-query-handlers.go` still uses `StructToMap`. Migrate explicit wire contracts and injected/cancellable data reads. |
| Movement and position recovery — baseline, acceptance gaps | `PhaserMapInfoRequest`, `PhaserWarpActivateRequest`, `PhaserInstantWarpRequest`, `PhaserMapLoadRequest`, `OwnedPlayerPositionRequest`, `PlayerFacingRequest`, `PlayerStepRequest`, `PlayerStepCompleteRequest` | Owned movement commands, typed replies and durable step receipts exist. Finish the remaining source/catalog/queue ordering and process-death cases recorded in the roadmap. Keep movement authority in its existing coordinator. |
| Gameplay recovery — baseline, dependent-family gaps | `GameplayStateRequest` | Coherent inventory/wallet/party/flags and battle/plan recovery exists. Audit every remaining writer against these projections and admission rules; PC box state is not included. |
| World presentation queries — queued | `PhaserTilesRequest`, `PhaserOverworldMapsRequest`, `PhaserActorsRequest`, `PhaserWarpsRequest`, `PhaserMapMusicRequest` | Several queries are typed/injected; actor and warp responses still use `StructToMap`. Audit visibility and cancellation, then retire the remaining reflection adapters. |
| Content and script reads — partial baseline | `PhaserDialogueRequest`, `PhaserWildEncountersRequest`, `PhaserTrainerDataRequest`, `PhaserPokemonDataRequest`, `PhaserMoveDataRequest`, `PhaserMapScriptsRequest`, `PhaserLearnsetRequest`, `PhaserItemDataRequest`, `PhaserHiddenObjectsRequest` | Injected content service and several explicit DTOs exist. Dialogue/encounters/trainer/hidden-object replies still have reflection consumers. Map-script reads also issue/resume plans, so their ordering needs mutation-grade review. |
| Battle and pending learning — baseline, final audit queued | `PokeBattleStartRequest`, `PokeBattleActionRequest`, `PokeBattleSwitchRequest`, `CQBattleItemUseRequest`, `PokeMoveLearnRequest`, `PokeBattleCloseRequest` | Durable battle identity, correlated recovery and selected crash/restart acceptance exist. Audit start eligibility, remaining global dependencies and all terminal/pending-choice variants without duplicating the battle coordinator. |
| Trainer interaction and plans — partial baseline | `TrainerEncounterReady`, `TrainerInteractRequest`, `TrainerBattleStartRequest` | Source interaction authorization and durable sight-trainer plans exist. Remaining issued-plan process-death and source/queue/catalog ordering acceptance is open. |
| Party read/reorder — selected next | `PokemonPartyRequest`, `PokemonPartyReorderRequest` | Reorder currently reads through the global DB, accepts slot indices and publishes an uncorrelated reply. A delayed duplicate reapplies the permutation to changed slots. Use stable row identities and the existing command/recovery boundary; audit independent party reads for stale projection. |
| Center healing — queued, confirmed gaps | `PokeCenterHealRequest` | `handler-pokecenter.go` trusts request `mapId`; party healing, options and defeated-trainer reset are separate operations, with persistence failures followed by success. Establish source eligibility and one atomic domain command before publishing. |
| Inventory, shops and Repel — reviewed baseline; legacy reads/items remain | `CQInventoryRequest`, `CQMerchantOpenRequest`, `CQMerchantBuyRequest`, `CQMerchantSellRequest`, `CQItemUseRequest`, `RepelUseRequest` | Shop, outside-battle party items and Repel share the reviewed transaction/revision/coordinator boundary. Legacy inventory reads and Bicycle/Escape Rope dispatch still need explicit review/migration. Preserve current clerk authorization and existing atomic Escape Rope work. |
| Field actions and pickups — queued, partial atomicity | `PokeFishingRequest`, `PokeSurfingRequest`, `FieldMoveUseRequest`, `ItemPickupRequest` | Atomic pickup/position primitives already exist. Audit field-state, battle creation and source permission through their respective owners; wire replies and recovery remain separate. Do not put all field actions into the inventory coordinator. |
| PC storage — queued, confirmed contract gaps | `PokemonPCOpenRequest`, `PokemonPCDepositRequest`, `PokemonPCWithdrawRequest`, `PokemonPCReleaseRequest`, `PokemonPCSwitchBoxRequest` | Storage helpers already lock and transact. Transport still selects by slot, uses global DB/background context, splits party/box/success packets and ignores some read/preference-write errors. Needs stable row IDs, command identity, coherent box/party recovery and battle/source authorization audit. |
| Script/dialogue/cutscene execution — partial baseline | `DialogueChoiceRequest`, `CutsceneEndRequest`, `ScriptedEventInteractRequest` | Durable cutscene receipts, atomic rewards/trades and source eligibility exist. Audit every action/writer and remaining puzzle/prize variants; complete issued-plan and queued-plan recovery acceptance. |
| Recovery warp and elevators — queued | `WarpHomeRequest`, `ElevatorFloorsRequest`, `ElevatorSelectRequest` | Inspect eligibility, destination authority, cancellation and whether committed movement uses the established projection and recovery rules. |
| Safari — partial baseline | `SafariZoneEnterRequest`, `SafariBattleActionRequest` | Battle actions/terminal dismissal have durable identity and crash acceptance. Audit entry/reentry/exhaustion admission and remaining cache/projection/lifetime behavior. |
| Game Corner — partial atomicity/authorization baseline | `GameCornerCoinBalanceRequest`, `GameCornerBuyCoinsRequest`, `GameCornerSlotPlayRequest`, `GameCornerPrizeListRequest`, `GameCornerPrizeBuyRequest` | Atomic/bounded coin, slot and prize operations and clerk eligibility checkpoints exist. Audit remaining duplicate/reconnect protection and coherent client recovery. |
| Player informational queries — baseline, final audit queued | `PokedexListRequest`, `PokedexStatusRequest`, `TrainerCardRequest` | Typed/injected paths exist. Final read cancellation and stale-response/lifetime audit remains. |
| Debug fixtures — gated baseline, final audit queued | `DebugSceneListRequest`, `DebugSceneJumpRequest`, `DebugGivePowerPokemonRequest`, `DebugWarpProbeCasesRequest` | Retain production gating and local fixture ownership. Audit handler registration and mutation privileges alongside HTTP debug surfaces; debug setup must not weaken production invariants. |
| Tile authoring — authorization/persistence baseline, wire audit queued | `TileEditorPlaceRequest`, `TileEditorEraseRequest`, `TileEditorFillRequest`, `TileEditorUndoRequest`, `TilePropertiesRequest`, `TilePropertyUpdateRequest` | Preserve server GM/admin authorization and existing real persistence/reload/broadcast checks. Tile property/edit broadcasts still use `StructToMap`; inspect cancellation and remaining contracts. |

## Other boundaries that must close

| Boundary | Source anchors | Remaining audit |
| --- | --- | --- |
| HTTP and transport ownership | `internal/server/server.go`, `websocket.go`, `admin.go`, `transport_test.go`, `lifecycle_test.go` | Inventory `/register`, `/cq`, `/ws`, readiness/hash/player-count/online, optional Discord bridge, admin routes, overview and local tile-authoring/static routes. Verify auth/method/body/time limits, cancellation, connection replacement and retirement. Existing owned-listener/drain tests are baseline evidence. |
| Script-internal durable writers | `internal/world/script_interaction.go`, `daycare.go`, `in_game_trades.go`, `gamecorner_prizes.go`, `party_healing.go`, `field_effect_transactions.go` and their callers | Not every write has its own opcode. Follow each caller to the owning transaction/command and publication boundary; include flags, wallet, party, inventory, puzzles and pickups. Ordinary content stays data-driven. |
| Background and cached owners | `internal/world/periodic_worker.go`, `world.go`, movement/actor/encounter managers, `internal/session/commands.go` | Enumerate worker callers, callbacks and shared caches; verify single character ownership and no stale writes after handoff, cancellation or shutdown. Finish durable final-save recovery and deadline propagation. |
| Generated wire boundary | `server/tygo.yaml`, `server/internal/world/world-utils.go`, `scripts/fix-tygo-casing.sh`, `src/net/NetworkBridge.ts` | Retire `StructToMap` and casing postprocessing only after the full consumer inventory migrates. Canonical regeneration and matching frontend/backend checks remain required. |

All five roadmap areas remain open. New findings belong in the affected row;
unresolved issues must not disappear when a nearby family passes. Local acceptance
does not establish production availability or authorize deployment.
