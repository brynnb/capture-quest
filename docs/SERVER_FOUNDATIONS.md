# Server foundations goal

Status: active. Started 2026-09-25 from `02c51ba`.

Working branch: `codex/server-foundations`. Latest implementation checkpoint:
running-command cancellation (2026-10-02), following shared position persistence
checkpoint `d70813e`, durable Safari checkpoint `19203f2`, Repel checkpoint
`adaa047` and FLY checkpoint `b785d58`.
All earlier foundation checkpoints are retained in this
branch's history. No push or production deployment is authorized by this goal.

Keep Go, PostgreSQL, one deployable server, and the authoritative extractor,
runtime asset, and scripted-action contracts. Improve runtime safety through
small verified changes. No production deployment or push is part of this goal.

## Current scope and status

We are making the existing server reliable when requests overlap, connections
are replaced, database operations fail, and the process starts or stops. The
intended result is one authoritative gameplay state, atomic durable changes,
explicit transport contracts, and lifecycle behavior that can be verified.
Go, PostgreSQL, the single-server deployment and the content pipeline remain
the architectural foundation. Runtime design is described in
[`ARCHITECTURE.md`](ARCHITECTURE.md).

The full goal is **in progress**. A completed checkpoint proves its documented
behavior; it does not prove that every gameplay path has migrated. The summary
below is the current handoff. Later checkpoint entries preserve historical
evidence, including remaining-work notes that subsequent commits may resolve.

| Area | Implemented | Still required |
| --- | --- | --- |
| Request/session boundary | Packet and connection limits, centralized session prerequisites, removal of insecure session takeover, actual transport closure, location/visibility checks for scripted clicks, dialogue choices and direct trainer battles, and preserved command deadlines/disconnect cancellation in migrated operations. | Audit remaining interaction/mutation endpoints; propagate cancellation through legacy managers and remaining database/network work. |
| Durable gameplay | Shared bounded transactions; atomic shops/inventory, stable Pokémon row identities, party/item changes, battle persistence, script rewards/completion, trade rollback/deduplication, atomic Vermilion puzzle transitions, item-ball collection, Silph doors, Game Corner prizes and bounded coin/slot/hidden-coin operations, atomic Escape Rope/FLY positions, durable Repel counters, Safari entry/turn/capture state and exhaustion destinations, atomic recovery warps, and commit-before-publication in migrated paths. | Finish remaining dynamic puzzles, pickups, prize/field-effect paths; durable duplicate protection and recovery of committed results across reconnects. |
| Character ownership | Bounded serialized session commands, exclusive character ownership and drained handoff, stale-cleanup guards, immutable cross-session presence, and movement ticks coordinated with the owner. | Finish timer/callback/shared-state and legacy position-writer audits; prove remaining concurrent/reconnect behavior across real transports. |
| Domains and wire contracts | Injected content-query service; typed character/wallet/bind, Pokédex/card, content detail, map-script and learnset contracts generated from explicit JSON names. | Migrate remaining gameplay/query families and global dependencies; retire `StructToMap` and the casing postprocessor after every consumer moves. |
| Lifecycle and verification | Owned HTTP/listeners, readiness, listener failure propagation, joined periodic workers, sealed session admissions, fail-closed staged preload, startup cancellation, and atomic scripted-event publication. | Bounded shutdown with active players and running work; complete transport/rendered integration and failure/retry/cancellation coverage. |

## Remaining work, in recommended order

1. **Finish atomic dynamic mechanics.** Vermilion puzzle state and its lock
   flags now share one character-locked transaction, as do item-ball grants/collection
   markers and Silph Card Key checks/unlocks. Game Corner prizes, coin purchases,
   slots and hidden-coin collection now share this boundary; slot requests validate
   imported machine availability/reach and use server-owned luck; prize buys require
   reach/visibility to the source window selling the selected prize, and coin buys
   require reach/visibility to the source coin clerk. Escape Rope consumption and
   saved destination now commit together before live teleport publication. FLY
   validates the destination catalog and durable party/badge eligibility inside
   its position transaction. Repel consumption and activation now share a durable
   transaction, with committed step updates and expiry. Safari payment, visit/battle
   state and captures now share this boundary, as do runtime exhaustion and its
   saved gate destination. Reported positions and forced teleports now commit
   before live publication; movement saves retain dirty state on failure and
   release the shared player lock before database work. Audit remaining field
   effects and other mutation paths for the same requirements.
   Extend the shared transaction/domain operations already in use. Acceptance:
   a late failure leaves all affected state unchanged; retry and concurrent
   requests cannot duplicate a reward or publish uncommitted success.
2. **Complete the mutation and interaction audit.** Check every remaining opcode
   against authoritative session state, location, ownership and eligibility.
   Cover ordinary metadata/presentation separately from requests that grant
   durable effects. Acceptance: forged IDs, stale sessions and remote/hidden
   targets cannot change gameplay, while legitimate interactions still work.
3. **Make retries and reconnect recovery durable.** Extend existing trade/battle
   and issued-event protections to remaining mutating commands. Define how a
   client recovers a result committed just before transport loss or process
   failure. Acceptance: replay does not duplicate effects, and reconnect can
   recover the committed outcome without relying on an old session token.
4. **Finish ownership and bounded shutdown.** Audit remaining NPC callbacks,
   timers and shared world writers; define disconnect behavior when the final
   position flush fails and propagate cancellation into running work.
   Replace unbounded shutdown waits with a documented drain deadline and
   failure policy that preserves persistence ordering. Acceptance: shutdown
   during gameplay terminates predictably, persists accepted work as specified,
   and does not close the database while owned work still uses it.
5. **Complete domain and contract migration.** Extract cohesive gameplay services
   as their operations move, inject their dependencies, and migrate remaining
   wire families through the existing protocol generator. Audit real JSON keys,
   nullability and empty collections with their frontend consumers. Acceptance:
   one source defines each contract; legacy casing/reflection paths are removed,
   regeneration is stable, and frontend typecheck/build and boundary tests pass.
6. **Verify the integrated result.** Exercise real transport and rendered local
   gameplay: overlapping/duplicate requests, reconnect in battle, slow clients,
   cancellation/timeouts, failure/retry and shutdown with active players.
   Acceptance: evidence covers the original five milestones, including visible
   behavior where relevant. Only then mark the full goal complete.

## Running-command cancellation checkpoint (2026-10-02)

The session gate previously used the request context only while waiting for
admission. An admitted callback lost its caller's deadline, and disconnect did
not cancel its database work. The gate now owns a connection cancellation context
and exposes the current owner's `CommandContext()`. It preserves the admission
deadline and cancels on connection closure. Queued requests wake immediately on
close; running callbacks retain the gate until they actually return. Cleanup
never overlaps a callback merely because cancellation was requested.

Migrated handlers pass that context into economy/inventory, battle turns/close,
position/field destinations, healing, item pickup, puzzle/door operations,
content queries, dialogue/trainer/source authorization and context-aware
account/character operations. Movement ticks and Surf pass it explicitly to
position persistence. Disconnect's final flush deliberately uses a separate
context: the closed connection must not cancel persistence cleanup.

`CommandContext()` is for synchronous code within the session owner. It must not
be retained for later callbacks or cleanup. Calls outside the command gate have
no request context; fixture/setup callers retain their existing behavior. A
cancelled command result does not prove rollback: a commit completed before
cancellation remains durable and still needs the planned result-recovery policy.

Session tests prove preservation of the original deadline, cancellation of a
running periodic owner, immediate rejection of queued closed work, and cleanup
waiting for callback completion. A real opcode/database test holds the character
row lock, closes the session during a reported-position write, verifies prompt
transaction cancellation with unchanged saved/live position, then drains cleanup.
Race-enabled world, session, server and script-simulator suites pass.
All Go packages compile, TypeScript typecheck passes, and canonical Tygo
regeneration leaves generated contracts unchanged.
The isolated rendered run at `/var/tmp/capturequest-rendered.Uldwvm` passed
six field-move/Safari checks. The keyboard Cut case timed out before guest login:
its trace records `ERR_NETWORK_CHANGED` loading local JavaScript modules and
its screenshot is blank. It never reached the scenario or sent a gameplay
request. The failed run is retained; this is not a seven-test passing run.
The unchanged Cut case passed in a fresh isolated rerun (5.9 seconds), retained
at `/var/tmp/capturequest-rendered.AAPUPp`. All seven cases therefore have passing
rendered evidence across those two runs, with the initial load failure preserved.

This is a prerequisite for bounded shutdown, not completion of it. Safari/Repel
manager methods, script/battle-start helpers, cached/global reads and other
legacy operations still start independent contexts or lack cancellable queries.
Non-cooperative callbacks cannot be forcibly stopped safely. HTTP/world shutdown
and periodic-worker joins still wait without a drain deadline. Final-flush failure
handling, database-close ordering and active-player shutdown verification remain
required. Next: migrate those running operations, define the cleanup failure
policy, then implement a bounded lifecycle that never closes storage beneath
owned work. No push or deployment is included.

## Position persistence checkpoint (2026-10-02)

The old movement saver used the global database while holding the shared player
lock. It could record `LastSaveTime` despite a failed save. Several teleport
callers changed session/movement state or sent a destination before persistence.

Position writers now use the world's captured database and the existing bounded
character-locked transaction. Destination changes and Safari exit cleanup commit
together. `publishCommittedPlayerPosition` only publishes a previously committed
position; it performs no second write. DIG, TELEPORT, elevator travel, forced warp
tiles, local debug jumps and invalid-position recovery propagate save failures
before publishing destination success. Client position reports and map requests
with destination coordinates also save before changing live state.

Movement flushes snapshot coordinates under the shared lock, release it for
database work, and mark only the same registration and matching coordinates as
saved after commit. Failed saves retain dirty state for a later tick retry.
Ordinary forced-path movement remains responsive before its periodic save;
walking, encounters and other step effects are not one atomic transaction.

Verification and limits are recorded here with this checkpoint. PostgreSQL
failure tests cover deferred commit rejection, Safari exit rollback, rejected
position/map requests through the opcode dispatcher, destination preservation
when the old saved position is invalid, failed-flush retry, blocked
database writes releasing the movement lock, stale-snapshot bookkeeping and
caller cancellation while waiting for the character row lock. These are runtime
state/database checks. Separately, all seven rendered field-move/Safari tests
passed (43.0 seconds), covering Surf input, Cut interaction, Safari entry,
battle run and step exhaustion. Evidence is retained at
`/var/tmp/capturequest-rendered.iSHhFr`. These checks do not establish throughput
or complete transport recovery.

Remaining work includes transactional eligibility/catalog reads for legacy
field moves, validation of client-authorized map/position changes, durable command
identity and result recovery, and end-to-end cancellation. Disconnect cleanup
still logs a failed final flush and continues releasing ownership; its failure
policy needs to be defined with bounded active-player shutdown. The other full
goal requirements in the roadmap above remain active.

Recommended next step: define the final-flush failure policy and implement
bounded shutdown with cancellation and database-close ordering, then prove it
through active-player integration tests. This checkpoint is local only; it does
not push or deploy the branch.

## Pre-Safari branch handoff (2026-10-02)

This records the handoff before the Safari implementation. The checkpoint below
supersedes its Safari plan; the full remaining roadmap above stays current.

The implementation through `adaa047` is committed on
`codex/server-foundations`. The working tree was clean when this handoff was
prepared. The latest implementation and its verification are recorded in the
Durable Repel checkpoint below. This handoff adds documentation only; no push,
deployment, or normal local/production database migration has occurred.

The next checkpoint is Safari ownership and transaction safety. Inspection of
`handler-safari.go` confirms that `GetSession` returns a mutable session pointer
after releasing the manager lock, and `SetSession` retains the supplied battle
pointer. Direct entry deducts money before creating the in-memory session.
Scripted entry pays inside its existing transaction but creates the session as
an after-commit action. Capture marks the Pokédex separately from saving the
Pokémon; save errors are logged while the response can still report a catch.
These paths have not yet been migrated or verified against transaction failure.

The proposed change is one durable Safari visit/battle state, replacing the
mutable session map. Entry payment, counters, capture storage and Pokédex updates
should use the existing bounded character transaction, with notifications only
after commit. Scripted actions must join their existing transaction. Migrate
movement/exit guards, local fixtures and simulator consumers together; propagate
storage errors rather than treating them as an inactive visit. Audit direct
entry authorization against the imported gate script before changing that route.

Acceptance requires concurrent entry to charge once, rollback to preserve money,
counters and capture state, recovery through a fresh manager, and existing Safari
simulator scenarios to retain their intended behavior. Verify the rendered entry,
battle and exit flow where affected. This is the next implementation plan, not a
completed checkpoint. Durable command/result recovery, cancellation, bounded
shutdown, remaining domain/contracts and integrated transport checks still remain
in the full roadmap above.

## Verification and release boundary

For the position checkpoint, the disposable PostgreSQL runner passed the new
failure/cancellation tests and the race-enabled `internal/world`,
`internal/scriptsim` and `internal/session` suites. All Go packages compiled;
`npm run typecheck` passed, and seven isolated rendered field-move/Safari checks
passed. Runtime assets passed the isolated runner's preflight validation;
canonical `npm run tygo` left generated contracts unchanged. Earlier checkpoints
record their relevant build, asset and rendered checks below.

These checks establish the tested local behavior. They do **not** establish a
complete rendered gameplay flow, production deployment, or completion of the
whole roadmap. No production behavior has been verified for these new changes.
Reproducible PostgreSQL commands are at the end of this document. Read
[`DEPLOYMENT.md`](DEPLOYMENT.md) immediately before any separately requested
deployment, and complete its applicable workflow and live checks.

Continue with the position checkpoint's recommended shutdown/final-flush work.
Keep this current summary synchronized with coherent
checkpoint commits; retain the original milestone acceptance criteria below.

## Durable Safari checkpoint (2026-10-02)

Safari previously returned live session/battle pointers after releasing a map
lock. Direct entry charged before allocating the visit; scripted entry committed
payment before creating its memory-only session. Capture marked the Pokédex and
saved the Pokémon through separate operations, logging save errors while still
reporting success. Disconnect/process loss had no durable visit owner to recover.

`character_safari_state` now owns the versioned visit/battle snapshot. The old
pointer map is retired. Reads return independent snapshots and report corruption
or storage errors. Mutations reload under the shared character lock and use the
bounded transaction. Payment, activation and flags commit together; scripted
entry/exit join their outer transaction, including entry position. Capture stores
the prepared wild Pokémon in the first free party slot or PC and marks its
Pokédex in the same turn commit, without replacing existing party identities.
Notifications follow commit. Managers, fixtures and simulator consumers receive
an explicit database and propagate state errors. Startup and database smoke
checks require the new table.

Runtime step exhaustion and out-of-balls turns save the gate destination and
`EVENT_SAFARI_GAME_OVER` with the counter/turn. Gate transfer retains the visit
until the source exit script clears it. The bundled
`engine/events/hidden_objects/safari_game.asm` ends the visit on zero balls,
including a last-ball catch; this repairs the old handler's caught exception.
The current 500-step allowance and encounter probabilities are retained rather
than claiming complete historical timing parity. Status requests recover the
committed active battle. Direct entry now requires reach/visibility to the
imported gate worker; battle actions require the character to be in a Safari map.

The shared durable field-destination writer and scripted movement end Safari
when moving outside the zones/gate, in the same position transaction. Recovery
warp also deletes ordinary battle state, ends Safari and saves its destination
in one commit through the captured database. Script position publication no
longer performs a second independent save. Legacy movement/client-reported map
paths still need a complete position-persistence audit: their Safari cleanup is
bounded and error-reporting, but every such position change has not yet moved
into the shared transaction. Cache refresh failures and post-commit delivery
recovery remain broader foundation work.

PostgreSQL tests cover concurrent entry charged once, recovery through a fresh
manager, late entry/script/turn/capture/expiry/recovery-warp commit failure,
retry, PC capture with a full party and preserved identities, remote/hidden-worker
rejection, and missing/unsupported state rejection. Capture checks use the actual
random roll with bounded attempts and no production RNG override. The initial
broad run found a fixture using duplicate `box_slot=0` values; the fixture was
corrected to respect the real storage constraint. Final race-enabled suites for
`internal/world`, `internal/pokebattle`, `internal/scriptsim` and `cmd/db-smoke`
passed. All Go packages compiled, canonical `npm run tygo` regenerated the moved
Safari definitions, and TypeScript checks passed.

All 11 Safari simulator goldens passed on the canonically bootstrapped private
database retained at `/var/tmp/capturequest-rendered.mTtObI`. Two entry scenarios
now require `EVENT_IN_SAFARI_ZONE`; their canonically regenerated goldens change
only that final flag, reflecting the newly atomic activation. The gate-exit
trace retains its prior counters from the transaction snapshot. No failing
behavior assertion was removed.

The first rendered run passed battle-run and expiry but never triggered gate
entry: fixture placement is not a coordinate step. The test now walks off and
back onto the source trigger tile. Failure evidence remains in the directory
above. Final rendered evidence is `/var/tmp/capturequest-rendered.Lia1qg`: all
three tests passed (19.3 seconds), with screenshots showing the zone/HUD, Safari
battle action menu and expiry announcement, and checks confirming gate return.
These are real WebSocket/browser flows on a private database. They do not prove
abrupt reconnect, process restart, duplicate delivery recovery, or loaded-player
throughput. Fresh-manager recovery tests prove storage independence only.

The new runtime table must be applied through the documented deployment lane
before activating this code; no normal local or production database was changed.
Next: finish legacy position/callback ownership and cancellation, then bounded
shutdown and durable request/result recovery. Remaining domains/contracts and
integrated transport acceptance retain their full scope. The goal stays active;
no push or deployment.

## Durable Repel checkpoint (2026-10-02)

Repel previously checked an in-memory counter, consumed an inventory item in a
separate transaction, then activated that counter. Disconnect cleanup deleted
the effect. Overlapping activations could both consume items, and an effect could
be lost after a committed consumption without a durable record to recover.

`character_repels` is now the single authoritative counter. Both item-use routes
recheck active state and inventory identity under the character lock, then consume
and activate in the same bounded transaction. Step updates and expiry use that
same durable counter; wear-off notifications follow commit. The in-memory map
and disconnect deletion are retired. A newly constructed manager recovers the
committed effect directly from its injected database. The simulator and local
scenario reset/setup paths use the same state and propagate read/write errors.
The ordinary battle guard now also covers the legacy Repel opcode. Expected
rejections stay player-facing; database details stay in diagnostics.

The canonical runtime schema adds the table with a positive-step constraint and
character ownership foreign key. Database smoke checks require it, and wild
encounter preload checks its columns before readiness. An eventual deployment
must apply this schema through the applicable documented lane before activating
this code. No schema mutation was made to a normal local or production database.

PostgreSQL dispatcher tests cover deferred activation failure on both opcodes,
retry, recovery through a new manager, deferred expiry failure and one wear-off
notification. Four concurrent managers consume exactly one item and leave one
active counter. Missing-table preload rejects without replacing the encounter
cache. Race suites for item use, world, simulator and database smoke passed;
all Go packages compiled and TypeScript checks passed. The three existing Repel
simulator scenarios passed their unchanged golden files on the canonically
bootstrapped private database. That run exposed a stale fixture insert using
`character_data.level`; the fixture now follows the canonical character schema.

The first rendered attempt used a stale `Inventory` button selector; the actual
`BottomHUD` menu and screenshot show `Bag`. The known inventory-opening selectors
were updated across inventory, bicycle, fishing and audio tests. The next run
passed Repel, potion cursor and Coin Case checks, then exposed a misleading
non-usable-item message. Actual imported rows are `NUGGET` (`id=49`,
`is_usable=false`, `item_type=0`) and `X ATTACK` (`id=65`, `is_usable=true`,
`item_type=3`). The shared item-use validator now distinguishes an unusable item
from a battle-only item; the original rejection assertion was preserved.
Failure evidence remains at `/var/tmp/capturequest-rendered.ubkegx` and
`/var/tmp/capturequest-rendered.hggpJh`. A subsequent re-entry test found two
Repel stacks: the local-only fixture deliberately tops total inventory up to
five on entering the world. The test now tracks the original instance ID across
re-entry, verifying its retained quantity and a fresh active-effect rejection;
it does not hide the extra stack with an arbitrary first-row selector. That
investigated run is retained at `/var/tmp/capturequest-rendered.qzNHgr`. Final
rendered evidence is `/var/tmp/capturequest-rendered.b4DScR`: all four inventory
tests passed (21.8 seconds), including consumption, active-effect rejection,
leaving/re-entering the world with the original stack unchanged, medicine cursor,
Coin Case and blocked-item behavior. This tests character detach/re-entry through
the live WebSocket server, not abrupt socket loss or process restart. Other
selector-only test files have not been rendered here.

Current encounter-step timing is preserved: the counter advances on the existing
eligible encounter-tile hook before the probability roll. This checkpoint does
not claim historical every-tile timing or durable delivery of wear-off messages.
Database failure leaves the counter unchanged and skips that encounter check
with a diagnostic. Durable updates add database work to this hook; performance
under many moving players remains part of integration verification.

Next: Safari payment/session/battle ownership and remaining field eligibility.
Cancellation through running commands, durable request/result recovery, bounded
shutdown, remaining dependencies/contracts and broad transport/reconnect/rendered
acceptance remain required. The full goal stays active; no push or deployment.

## FLY catalog and position checkpoint (2026-10-02)

The UI lists destinations from `poke_start_cities`, then sends `mapId`, `targetX`
and `targetY`. Previously the handler passed these coordinates directly to the
teleport helper. Knowing FLY and having the Thunder Badge therefore permitted
an arbitrary client-selected map/position rather than one of the offered towns.

FLY now locks the character in a bounded transaction, loads the current party
and durable badge flag through that transaction, and requires exactly one catalog
destination for the submitted map with matching spawn coordinates. Unknown maps
and forged coordinates reject; ambiguous catalog rows reject with diagnostics.
The normalized destination is saved before committing. FLY and Escape Rope
share the committed-teleport publisher and transaction-only position writer;
failed commits publish no live position or teleport. All field-move requests
now reject during an active ordinary battle, matching the item-use boundary.

Eligibility reuses the existing field-move rule/party evaluator rather than
creating a second ruleset. Simulator/presentation callers retain their current
flag view; the durable FLY operation uses `queryEventFlag` and its injected
database instead of cached eligibility or the process-global database.

This preserves the project's current all-cities policy and catalog. The bundled
`engine/menus/start_sub_menus.asm` and `engine/items/town_map.asm` also describe
outdoor-only FLY and visited-town restrictions. Those historical restrictions
are not implemented by this checkpoint; the server has no town-visit progression
model, and the current UI offers all home towns. Do not claim historical parity.

PostgreSQL dispatcher tests with the global database disabled cover missing
party moves/badges, stale cached flags, forged map/coordinates, duplicate catalog
rows, deferred commit failure, active battle and successful retry. Rejections
leave saved character, session and movement position unchanged and publish only
an error. Race-enabled `internal/world` and `internal/scriptsim` suites passed,
as did the focused FLY/Escape Rope rerun after the shared publication extraction,
compilation of all Go packages and `git diff --check`. This is state/dispatcher
evidence, not a rendered FLY check.

Next: Repel/Safari state ownership and remaining field-effect persistence;
finish authoritative field eligibility, including defining progression before
introducing visited-town restrictions. Remaining cancellation, durable result
recovery, bounded shutdown, domain/contract migration and broad transport/rendered
acceptance retain their full scope. No push or deployment occurred.

## Escape Rope transaction checkpoint (2026-10-02)

The previous Escape Rope handler ignored `DecrementItemQuantity` errors, then
teleported and reported success. Its destination lookup, consumption and position
save used separate database operations; a consumption failure could therefore
grant a free escape, and a position-save failure could spend the rope without
saving the destination.

The live handler now uses its captured database. A bounded character-locked
transaction rechecks the item identity and owned consumption, resolves the exit
from the imported warp data, consumes one rope and saves the normalized position.
Only a successful commit updates session/movement state and emits the teleport
and item-use response. The common teleport helper now supports publication of an
already committed position without issuing a second independent save. DIG shares
the extracted exit lookup; its own persistence migration remains open.

The existing rule (reject overworld maps, prefer an overworld exit, otherwise the
first eligible imported warp) is preserved. It is not a new claim of exact
historical Escape Rope behavior. Missing map records and failed reads now reject
instead of treating an unknown map as eligible. Ordinary blocked-map/no-exit
rejections retain their player-facing message; database details stay in diagnostics.

A PostgreSQL dispatcher test with the global database disabled rejects a deferred
commit after consumption and position writes, proving the rope and saved/live
positions remain unchanged and no teleport is published. Removing the failure
allows a retry to consume and move together; replay of the consumed instance
does not teleport again. The older party-item fixture now supplies the handler's
explicit database dependency. Race-enabled `internal/world` and
`internal/scriptsim` suites, compilation of all Go packages and `git diff --check`
passed. This is dispatcher/state evidence, not a rendered Escape Rope check.

Next: finish field-effect eligibility/persistence and ownership. The inspected
FLY handler still accepts client-provided destination coordinates without checking
an authoritative destination catalog; Repel activation follows consumption as
separate state; Safari entry/session operations and pointer ownership still need
their audit. Initial item classification and other legacy queries still need
bounded cancellation. Durable result recovery, bounded shutdown, contract
migration and broad transport/rendered acceptance remain required. No push or
deployment occurred; the full goal remains active.

## Coin-clerk interaction checkpoint (2026-09-30)

Coin purchases now require current reach and visibility to the unique imported
`TEXT_GAMECORNER_CLERK1` actor in map `135`, identified by the bundled
`scripts/GameCorner.asm` source. Being elsewhere in the Game Corner or standing
at the other clerk no longer authorizes a purchase. Coin clerks and prize windows
share a bounded source-identity lookup and the existing actor/counter interaction
evaluator; missing or ambiguous source identities fail closed with diagnostics.
The existing atomic payment/grant operation and payment/cap rules are unchanged.

Dispatcher tests use the handler's captured database with the global database
disabled. They reject distant, wrong-clerk and hidden-clerk purchases without
changing either balance, then allow the visible clerk across a real talk-over
tile. The late-failure fixture now supplies valid clerk/machine targets and checks
the transaction failure response, so authorization cannot accidentally mask its
rollback assertion. Race-enabled PostgreSQL suites for `internal/world` and
`internal/scriptsim` passed; all Go packages compiled and `git diff --check`
passed. The isolated Chromium Coin Case/coin purchase/slot flow also passed
(13.0 seconds), with evidence retained at
`/var/tmp/capturequest-rendered.Bicvfv`. This proves the focused local flow,
not full-goal acceptance or production behavior.

Next: continue the remaining mutation/field-effect and callback/ownership audits.
Durable replay/reconnect recovery, bounded shutdown, remaining domain/contract
migration and broad transport acceptance remain open. This is a local checkpoint;
the full goal remains active, with no push or deployment.

## Prize windows and rendered Game Corner checkpoint (2026-09-30)

Being anywhere in the prize room no longer authorizes a purchase. The live
handler resolves the selected prize's existing window through
`GameCornerPrizeWindowForID`, requires its unique imported sign text/map identity,
and reuses the actor visibility/position/counter-reach evaluator before calling
the atomic transaction operation. Catalog-only requests remain presentation;
they grant no purchase authorization. Missing/ambiguous source sign records
produce diagnostics rather than an invented target. Simulator purchases continue
to exercise the shared transaction operation without claiming physical reach.

PostgreSQL dispatcher tests cover the wrong window in the correct room, hidden
signs, remote rooms, valid source-sign reach, commit failure and retry with the
global database disabled. Real local Playwright checks now cover Coin Case
acquisition, coin purchase, visible slot modal/spin/close, TM purchase and Pokémon
purchase into party and full-party PC storage through the live WebSocket server.
The slot/coin test passed individually and in the four-test run. That run's two
prize failures were investigated; the corrected two prize tests then passed.
This is evidence across focused reruns, not a claim that the whole original
foundations integration suite is complete.

The initial prize tests assumed player `y=4` could reach a sign at `y=2` across a
counter at `y=3`. Actual imported sign records are at `(2,2)`, `(4,2)`, `(6,2)`.
Tile row `y=3` has `tile_image_id=485`, `talk_over_tile=false`,
`collision_type=1`; the adjacent playable floor is `y=3`. The tests now stand
there and wait for each completed tile step instead of sending overlapping taps.
Production reach/data rules were not weakened to accommodate those assumptions.

Reproduce in an isolated local environment:

```bash
bash scripts/testing/run-isolated-e2e.sh tests/e2e/game-corner.spec.ts
```

The runner validates the existing atomic asset family, bootstraps a fresh private
Unix-socket Postgres cluster through the canonical preflight/schema/import/smoke
workflow, starts exact owned app/frontend processes on dedicated available ports,
and uses the installed local Playwright Chromium. It never sources `.env` or
reuses the normal development database. `/var/tmp` evidence includes logs, ports,
owned PIDs, and failure screenshots/traces; cleanup touches only these processes
and the private cluster and fails if an owned process exceeds its deadline.
Successful evidence: `/var/tmp/capturequest-rendered.d9vNsl` (slot/coins),
`/var/tmp/capturequest-rendered.k42UYM` (prizes). The investigated full-file failure
is preserved in `/var/tmp/capturequest-rendered.W6PHey`. These local paths are
handoff evidence, not repository artifacts or production verification.

Next: finish coin-clerk reach authorization (coin buying currently checks the
Game Corner map), then continue mutation/field-effect and ownership/timer audits;
complete durable request replay/reconnect recovery, bounded shutdown, contract
migration and broader transport/rendered acceptance. No push or deployment
occurred, and the full goal remains active.

## Server-owned slot boundary checkpoint (2026-09-30)

The live slot endpoint no longer accepts client luck. Requests name machine
coordinates; the server resolves `phaser_hidden_objects` on the Game Corner map,
requires `routine = StartSlotMachine`, validates `item_or_direction`, and rechecks
owned player reach on every spin. Targetless packets from old clients fail closed.
Unavailable machines (`SLOTS_OUTOFORDER`, `SLOTS_OUTTOLUNCH`,
`SLOTS_SOMEONESKEYS`) cannot spend coins. The existing two-tile Manhattan reach
is retained; machine coordinates never bypass the server-owned map/position.

Provenance: bundled pokered `data/events/hidden_objects.asm` contains the slot
records; `engine/slots/game_corner_slots.asm` defines availability and compares a
one-based hidden-object index with the lucky index. `scripts/GameCorner.asm`
draws a byte, promotes values below seven to eight and shifts three bits. Export
preserves source order as SQLite IDs and the existing Postgres import preserves
those IDs; the runtime orders the map's hidden objects by ID. The server applies
that source draw rule once per character/map visit, stores it in synchronized
session-owned state, and resets on published map departure, character change or
session cleanup. Modal reopening cannot reroll luck. Reconnect is a fresh visit;
this is not durable command/reconnect recovery.

The client sends generated typed coordinate/bet requests and consumes a generated
typed response. It displays luck only from the accepted server result. The old
parallel Phaser slot overlay and blanket sign-to-slot handler are retired; the
existing slot component remains the UI. Closed/nonspinning modals ignore results.
Further response correlation, spin timeout/cancellation and close/reopen race
verification remain part of the integrated networking acceptance work.

PostgreSQL world/session/simulator race suites pass, including forged luck,
missing/remote/unavailable/non-machine targets and reach after movement. Session
tests prove concurrent single draw, map reset, character reset and cleanup.
Canonical contract generation, frontend typecheck, runtime asset validation and
production build pass. The expected local test frontend was not running during
this checkpoint; rendered slot opening/spinning/closing and stale-client behavior
remain unverified. No generated game-data pipeline, schema, push or deployment
changed.

Next: run the rendered Game Corner check in the isolated test environment, then
finish issued prize-window/merchant interaction authorization and the remaining
field-effect/opcode/ownership audits. The full foundations goal remains active.

## Game Corner coin checkpoint (2026-09-30)

Coin purchases, slots and hidden-coin collection now use the shared bounded
transaction and acquire the character lock before reading eligibility/balances.
Coin Case checks, wallet reads and collection checks propagate query errors.
Wallet payment/coin grant, spin bet/payout and hidden-coin marker/grant commit as
one operation; a failed commit discards speculative success, balances and reels.
The reel layouts, payout rules, bet normalization and coin cap are preserved.

Live coin purchase/slot handlers inject their database and check the owned player
location against the Game Corner. Coin balance queries also use a bounded owned
database read. Simulator entry points share these operations. Unused standalone
coin setter and amount-only pickup callback were removed; script grants continue
to compose through the transaction-aware coin-grant primitive.

PostgreSQL world/simulator race suites pass. Tests cover coin-payment rollback,
retry/cancellation, hidden-coin marker rollback and concurrent collection, slot
rollback and concurrent spending, dispatcher failure publication and room
rejection with global database access disabled. This proves tested local durable
boundaries, not rendered slot/coin interactions or recovery after transport loss.

Still required: the slot request accepts client `isLucky`; the client currently
chooses that flag when opening the slot modal. Move machine eligibility/luck and
actor reach into a server-owned issued interaction contract while preserving the
supported gameplay rules. Complete hidden-object runtime integration/audit,
remaining global catalog/presentation reads, durable request replay, and the full
ownership/shutdown/domain/contract acceptance work. These transaction changes
do not establish those requirements. No push or deployment occurred.

Next: implement the server-owned slot/prize interaction boundary, then continue
the remaining opcode/field-effect and ownership audits.

## Game Corner prize checkpoint (2026-09-30)

Prize purchases previously committed the Pokémon/TM grant (and Pokémon Pokédex
registration) before coin payment. A late payment failure left a free prize;
competing purchases could both spend a stale balance. The shared operation now
locks the character, loads the authoritative prize and durable Coin Case/balance,
and commits reward, Pokédex registration and payment together. Missing coin rows
mean zero; query failures propagate instead of becoming an eligible balance.
Malformed price/type/identity data aborts with the affected prize in diagnostics.
Existing prize levels, names and item/Pokémon creation rules are unchanged.

The live handler injects its owned database and requires the server-owned player
position to be in the prize room. It publishes the inventory/wallet snapshot
loaded in the transaction only after commit. Simulator purchase-by-name uses the
same operation. Existing catalog/list wrappers remain to migrate; prize-window
actor reach and issued dialogue context also remain part of the interaction audit.

PostgreSQL race suites for world/simulator pass; all Go packages compile, frontend typecheck passes, and
canonical contract generation leaves generated files unchanged. New tests cover late payment
failure for Pokémon and TM rewards (including Pokédex rollback), deferred commit
failure through the dispatcher, retry, remote-room rejection, owned dependencies,
lock-wait cancellation, and competing purchases that cannot overspend. Repeated
purchase commands still represent separate paid purchases; durable request replay
identity/reconnect result recovery is not claimed by this checkpoint.

Next: migrate Game Corner coin purchases, slots and hidden coins through bounded,
error-propagating operations under the shared character lock; finish the remaining
interaction/domain/contract work and integrated acceptance checks. No push or
production deployment is part of this checkpoint.

## Item collection and Silph checkpoint (2026-09-30)

Item-ball pickup previously committed its inventory grant before inserting the
collection marker, ignored marker failures, and still reported success. Its
collection lookup also ignored database errors. Repeated/concurrent requests
could grant the same object again. The new operation locks the character and
loads the object/item, checks the collection marker, grants the item, records
collection and loads the inventory/wallet snapshot in one bounded transaction.
The handler publishes that snapshot after commit through its owned database.
Pickup authorization reuses server-owned actor position, visibility and map
checks while retaining immediate cardinal adjacency (no pickup over counters).
The obsolete parallel position fallback helpers are removed.

Silph doors now check Card Key ownership and durable door flags while holding
the same character lock as inventory mutations, then commit the unlock before
returning an outcome or refreshing cached flags. Live handlers inject the owned
database; simulator wrappers share the operation. Existing dialogue and key
retention rules are preserved.

PostgreSQL race tests cover pickup late-write rollback through the dispatcher,
successful retry and inventory publication, duplicate/concurrent collection,
remote/hidden/counter rejection, missing/removed Card Key, deferred door commit
failure and retry, and repeated opens with an unloaded flag cache. World and
script-simulator suites pass; all Go packages compile. These are local durable
and dispatcher checks, not rendered gameplay or reconnect recovery evidence.

Audit finding still to resolve: Game Corner prize grants and coin payment use
separate commits. Coin purchase/slot/hidden-coin paths also need review for
bounded queries, error propagation, shared character locking and owned database
injection. Next: migrate those operations, preserving their existing data-driven
prize and slot rules. No push, deployment, schema or generated content changes
are part of this checkpoint.

## Atomic Vermilion puzzle checkpoint (2026-09-30)

Previously the first-lock flag committed before selecting and saving the second
can; resetting likewise cleared the flag before saving the replacement state.
A later error could leave the flag and can indices inconsistent. Cached flag
reads also allowed stale state to choose a transition.

The live handler now supplies its owned database to one bounded transaction.
It locks the character, loads durable flags and puzzle state, chooses the next
state, and writes the state and flags together. Initialization joins that same
transaction. Success is returned only after commit; the flag cache refreshes
from committed storage. A cache refresh failure is logged without falsely
reporting rollback of an accepted click. Simulator entry points share the same
operation; fixture state writes also acquire the character lock.

PostgreSQL race tests cover late state-write failure, reset rollback, deferred
commit failure, successful retries, stale/unloaded caches, concurrent
initialization and transitions, and lock-wait cancellation followed by retry.
The world and simulator suites pass. This is durable-state evidence, not a
rendered puzzle check or proof of reconnect recovery. Repeated clicks remain
separate gameplay actions; request replay identity and recovery across transport
loss still require the goal's durable retry work. No schema, generated content,
frontend contract, push or deployment is part of this checkpoint.

Next: audit Silph doors, pickups, prizes and field effects, then migrate unsafe
operations through the existing character-locked transaction boundary.

## Milestones and acceptance checks

1. **Request and session boundary** (initial checkpoint complete): bound incoming messages and
   connection waits, enforce session prerequisites centrally, remove the unused
   session-ID/IP takeover mechanism, and close the actual transport on session
   removal. Exercise malformed/oversized/partial messages and requests made
   before character selection through the real dispatcher and socket boundary.
2. **Atomic gameplay persistence**: move purchases, inventory consumption,
   rewards, and party updates into shared transaction-aware operations; preserve
   Pokémon identities; update caches and publish success only after commit.
   Prove rollback, concurrent updates, ownership validation, and duplicate-command
   behavior using PostgreSQL where its semantics matter.
3. **Character state ownership**: bounded serialized commands, one active owner
   per character, safe connection replacement and disconnect cleanup, explicit
   coordination with movement, battle, timers, and shared world state. Verify
   concurrent requests, stale connections, cancellation, and reconnect in battle
   with the race detector and real transport integration tests.
4. **Domain and wire boundaries**: extract cohesive gameplay domains as their
   operations move, inject dependencies, keep handlers thin, and use explicit
   JSON names as the single source for wire data and generated TypeScript.
   Document and complete the migration away from redundant casing conversions;
   preserve existing public payloads unless a coordinated change is necessary.
5. **Service lifecycle and verification**: fail startup on required initialization
   errors, report readiness, drain work and persist active players on shutdown,
   bound database/network waits, and provide concise operational diagnostics.
   Verify success, startup failure, slow clients, timeouts, cancellation, retries,
   duplicate requests, and shutdown during active gameplay. Run appropriate Go,
   frontend, generated-contract, and integration checks before completion.

## Evidence and decisions

- Baseline focused race tests passed for `internal/session`, `internal/server`,
  and `internal/pokebattle`; these do not cover concurrent live sessions.
- The checked-in browser creates fresh `/cq` or `/ws` connections without a
  `sid` query. Reconnect will require fresh authentication; remove the unused
  takeover path rather than inventing a second authentication protocol.
- The largest existing inbound structured batch is the tile editor's 500-tile
  request. Packet bounds must accommodate that existing contract and must not
  limit larger outbound map/catalog responses.
- Existing transactions in daycare, trades, tile editing, and imports provide
  reusable patterns. Extend the authoritative systems instead of duplicating them.

## Progress

- Initial boundary checkpoint: 256 KiB incoming packet limit (including a
  500-tile batch regression), outer WebSocket message limits, partial-frame and
  stream-handshake deadlines, serialized reliable writes with deadlines, one
  reliable control stream per WebTransport session, centralized session-stage
  and JSON-object validation, unique IDs across transports, and actual transport
  closure on removal. New sessions participate in idle expiry immediately.
- The reliable stream can remain idle while WebTransport heartbeat datagrams
  arrive. Only a started frame gets a frame deadline; session expiry handles idle
  connections. Applying a blanket reliable-stream idle timeout would disconnect
  healthy WebTransport players.
- Focused race checks passed across API framing, sessions, server transports,
  and world handlers. Tests include real loopback WebSocket and WebTransport
  connections, oversized messages, partial-frame timeout, extra control streams,
  session-ID takeover rejection, session closure, and character prerequisites.
- Remaining: serialized character execution and cleanup, atomic gameplay
  operations, dependency/wire migration, lifecycle/readiness, and broader real
  gameplay/PostgreSQL/browser verification. The goal is not complete.
- Economy checkpoint: purchases validate the exact merchant/map/offer, honor
  price overrides and stock, and commit payment and inventory placement together.
  Selling credits the entire removed stack and rolls back deletion on payment
  failure. Inventory mutations share bounded transactions and character locking,
  enforce ownership, preserve all grant quantities across stacks, and propagate
  failures. Inventory queries now use an explicit database/transaction Store;
  the global unsynchronized item cache is removed. World composition still has
  global database dependencies to migrate in later checkpoints.
- Department-store offers carry their source merchant ID; purchases send that ID
  and publish a fresh inventory snapshot after commit. An old client selecting
  a sibling clerk's offer may receive a rejection and must reload; the server
  does not guess another merchant or price. Inventory TypeScript is generated
  from the Go types. Tygo is pinned, and the existing shared scripted-action alias
  is explicitly mapped so regeneration no longer degrades it to `any`.
- PostgreSQL verification covers full rollback after a grant failure, competing
  purchases, stock exhaustion, foreign ownership, concurrent grants/consumption,
  stack overflow, whole-stack sale, repeated sale, nested rollback, and lock-wait
  cancellation. Dispatcher tests verify failed commits do not publish success.
  All Go packages, frontend typechecking, runtime asset validation, and the
  production frontend build passed at this checkpoint. The PostgreSQL runner
  was also exercised end to end, including cleanup of its private cluster.
- Party persistence checkpoint: loaded Pokémon carry stable row IDs and explicit
  nicknames. Saves update rows atomically, reorder under the existing immediate
  uniqueness constraints, and reject stale membership, missing members, foreign
  rows, duplicates, and nil entries. New IDs become visible only after commit;
  transaction composition returns copied pending state. Missing species/moves
  and scan failures abort the load rather than silently dropping party data.
- PC transfers, acquisitions, starter creation, Day Care, and trades share the
  character lock. PC transfers roll back failed compaction, preserve the last
  party member, and reject attempts to release or withdraw non-PC storage.
  Party saving and storage operations have separate cohesive source files.
  The deliberate local fixture reset remains explicit and transactional.
- PostgreSQL race checks prove stable identity/nicknames, reorder, all-row and
  outer-transaction rollback, uncommitted ID isolation, malformed-load rejection,
  simultaneous deposits, and acquisition overflow into PC storage. Evolution
  preserves identity and nickname. All Go packages passed with PostgreSQL tests
  enabled. No frontend wire payload or database schema changed in this checkpoint.
- Party item checkpoint: outside-battle medicine, vitamins, PP Up, Rare Candy,
  stones, TMs/HMs, and the Poké Flute now use the injected `itemuse.Service`.
  Inventory consumption, party effects, and evolution Pokédex registration share
  one transaction. Existing effect rules moved out of transport handlers into
  the shared item domain; the Pokédex write has one repository implementation.
  Failed operations publish neither success nor a changed party snapshot. TM/HM
  prompts preserve pending context and revalidate ownership and moves on choice.
  The outside-battle handler rejects use during an already active battle.
- PostgreSQL tests cover competing uses with no lost healing, exhausted-instance
  retries, effect failure and retry, evolution rollback including the Pokédex,
  reusable HMs, changed moves after a prompt, and party-wide flute use. Real
  dispatcher checks cover failure/success messages and the active-battle guard.
  The default disposable PostgreSQL runner passed with the race detector; all
  Go packages, type generation, and frontend typechecking passed as well.
- A broad race run exposed an existing asynchronous chat writer reading the
  global database and mutable session after its caller returned. Both player
  and Discord chat persistence now use the world's injected database and captured
  values, with a five-second query deadline. Shutdown still needs to drain this
  work as part of the lifecycle milestone. The affected race tests now pass.
- Next: make battle item turns, rewards/flags, and remaining field effects atomic,
  then establish serialized runtime character ownership. Checkpoints so far are
  local commits only. Nothing has been pushed or deployed.
  Command deduplication, reconnect gameplay, shutdown, and rendered verification
  are still required; these inventory tests do not establish those properties.

- Battle persistence checkpoint: every start is durable before publication, and
  actions, item turns, forced switches, and move choices work on private copies.
  A stored battle ID/revision rejects a competing stale copy before its effects
  run. Party changes, consumption, capture/PC placement, Pokédex, experience,
  trainer prizes/defeat records, battle flags, and blackout charges share the
  battle commit. Medicine turns now go through normal victory/loss settlement.
  Shared response construction replaces the separate nontransactional item path.
- Resume preserves player status counters and stages by stable row identity,
  enemy metadata, faint-switch phase, and pending move choices. It retains the
  saved record; disconnect no longer rewrites it. Unknown versions and mismatched
  parties fail without deletion. Version-zero records upgrade on a successful
  commit. Client close rejects unfinished battles and pending choices.
- Real PostgreSQL checks cover late battle-save failure after party/wallet writes,
  concurrent stale revisions, cancellation, legacy upgrade, preserved volatile
  state and metadata, rejected party mismatch, item rollback/retry/exhausted-instance
  reuse, medicine-triggered victory, capture, move learning, and blackout rollback.
  World tests use the real dispatcher and inspect emitted messages and durable
  state. The full Go suite passed with the race detector and disposable
  PostgreSQL enabled, followed by type generation and frontend typechecking.
  These are not rendered or live-production checks.
- Next: migrate the shared cutscene interpreter's durable rewards/flags/object
  visibility into transactions with effects published after commit. The inspected
  post-battle corpus uses hide/show object, set/reset flag, heal party, give item,
  and presentation actions; extend shared primitives rather than add a second
  battle-only interpreter. General event-flag writes still update cache too early.
  End-of-battle notification recovery, sequential command deduplication, character
  ownership/connection replacement, field effects, wire migration, lifecycle,
  and rendered/live-transport gameplay verification remain required.

- Shared script transaction checkpoint: ordinary cutscenes, dialogue choices,
  the simulator, and post-battle actions now use the same interpreter with a
  transaction-owned mutation context. Item grants/removal, coins, money, party
  grants/healing, flags, object visibility, and scripted battle startup join the
  caller's transaction. Completion flags and the final warp join script rewards.
  Network messages, flag-cache refresh, actor visibility and movement publication
  are deferred until commit. Post-battle script failure rolls back the turn.
- Flag batches are atomic and toggle reads occur under the character lock;
  storage failures cannot poison the cache. Coin grants use an atomic database
  update. Script lookups propagate missing item/species errors instead of supplying
  invented names. The previous post-commit battle-only script invocation is removed.
- PostgreSQL race tests verify failed completion rolls back items, coins, healing
  and flags without messages; concurrent guarded completion awards once; failed
  post-battle scripts roll back victory; and scripted battle startup is rolled
  back when a later action fails. General flag batch/toggle failures are covered.
  The full Go suite passed with disposable PostgreSQL and the race detector;
  generated types and frontend typechecking also passed.
- Still required: authorization of script completion against the session's issued
  event and all eligibility conditions; durable request deduplication beyond
  existing absent-flag guards; recovery of committed notifications on reconnect;
  consistent movement/Safari ownership and lifecycle; transactional pickups,
  Game Corner prizes and remaining field effects; explicit wire/dependency
  migration; rendered and live-transport gameplay validation. Deferred runtime
  publication is not a durable outbox and does not establish crash recovery.

- Issued-cutscene authorization checkpoint: the server stores an immutable issued
  script snapshot against a random completion token, session and character. Client
  completion echoes that token; naming a catalog label alone cannot execute a
  script. A claimed token rejects competing completions, successful completion
  consumes it, and transaction failure permits retry. Session close/character
  cleanup clears pending grants. Dynamic dialogue scripts execute their actual
  issued content rather than resolving another catalog script with the same label.
- Pending authorization is bounded to eight events with a 30-minute lifetime;
  it is session-local and is not durable command deduplication or reconnect
  recovery. The client returns the token from its original playback payload.
  Old clients cannot complete events and must reload with the updated frontend.
  Cancelled/ignored playback authorization expires; an explicit cancellation and
  completion acknowledgment protocol remains part of session lifecycle work.
- Race-enabled PostgreSQL/dispatcher tests prove unissued requests do not grant,
  issued snapshots cannot be replaced by catalog edits, committed tokens cannot
  replay, and failed commits can retry. Session tests cover competing claims,
  wrong character/label, expiry and bounded storage. Type generation, frontend
  typechecking, runtime asset validation and the production build passed.
  This is not yet rendered or real-socket cutscene verification. Next: complete
  eligibility checks and serialized character/session ownership.

- Completion eligibility checkpoint: required and absent flags, owned item presence
  and absence, caught count, minimum and exclusive-upper money/coin thresholds
  are rechecked under the reward transaction's character lock. Database errors
  abort; invalid inventory ownership cannot satisfy a prerequisite. Facing and
  trigger proximity remain issuance-time conditions because playback may move
  the player. Issuance paths still need the broader authorization/ownership audit.
- Race-enabled world/simulator PostgreSQL tests passed, including rejected
  prerequisites, exact balance thresholds, absent-item duplicate prevention and
  foreign-owner inventory links. No wire, generated-data or schema change in this
  checkpoint. Next: serialize packet execution and disconnect cleanup, then bind
  each character to one active session and coordinate world/timer writers.

- Session execution checkpoint: the real opcode dispatcher serializes prerequisite
  validation and handler execution through a per-session gate shared by both
  reliable and datagram readers. At most 32 callbacks may execute/wait; admission
  waits have a five-second deadline and overload closes the connection. Callbacks
  run on the transport caller, without creating a goroutine per packet. Existing
  handlers retain their own operation deadlines; gate cancellation bounds waiting,
  not an already running callback.
- Disconnect closes first, rejects queued work, and drains the running callback
  before character cleanup. Periodic/shutdown playtime flushes enter the same gate.
  Inline camp/character cleanup already executes inside the dispatcher and must
  not recursively enter the gate. Session/server/world race tests passed with
  PostgreSQL; focused tests cover bounded admission, concurrent handler updates,
  cancellation, close versus queued/running work, and cleanup ordering.
- This does not yet establish single-character ownership across sessions, ordered
  publication across different character sessions, timer/world synchronization,
  or bounded shutdown of every running handler. Those are the next required
  ownership/lifecycle changes; the goal remains active.

Character connection ownership now has a single registry per world. Authenticated
entry reserves the character, closes the old connection, waits for its running
command and cleanup, and reloads database state before initializing the new client.
Only the current owner can flush/evict shared character state. A delayed old
disconnect retires only its local client. Concurrent handoffs fail entry rather
than creating a queue of competing logins; the client can retry. Waiting for the
old command is bounded to five seconds, but the cleanup callback and legacy
database reads still need the broader operation/lifecycle deadline work.

Entry checks account ownership before handoff and again on the reloaded record.
`PostEnterWorld` success now follows client initialization; failed entry returns
`value: 0`. The ownership map removes inactive records and preserves the closed
old owner when handoff admission times out, allowing normal disconnect cleanup.
This boundary does not yet serialize world timers or protect every session field
read by broadcasts. Those remain required work, as do startup/shutdown and the
wire-contract migration.

Validation: race-enabled session, world and server suites passed against disposable
PostgreSQL. The real dispatcher entry test holds an old command open, commits its
position change, and verifies the replacement reloads that value after cleanup.
Focused tests cover competing handoffs, cancellation, late cleanup and registry
retirement. These are headless boundary checks; live movement timers, browser
reconnect presentation and production behavior have not been verified here.

Movement ownership checkpoint: the timer snapshots candidate registrations under
the movement lock, releases that lock, then tries each session's command gate.
Busy sessions wait until a later tick without queued timer goroutines. The gate
remains held across path advancement, persistence, broadcasts, encounters,
daycare/Safari steps, scripted triggers and warps. Ownership and registration
identity checks reject stale timer candidates. Forced warp mutations also take
the movement lock used by readers. The step logic was mechanically extracted
into advancement and effect helpers without changing its gameplay ordering.
The final race-enabled session/world suites passed against disposable PostgreSQL,
including busy-owner deferral, eventual position publication, stale registration
rejection, and refusing ticks after ownership release.

This does not yet remove raw session reads from cross-player broadcasts, bound
the duration of every executing callback, or drain background tasks at shutdown.
Those remain required work. Movement tests are headless; no rendered movement
claim or deployment is implied.

Presence publication checkpoint: session command completion and cleanup publish an
immutable value projection of authentication, map/position and visible character
identity. World broadcast recipients, player enumeration and NPC player collision
checks consume this projection without reading another session's mutable client
or taking its command lock. Owner-side actor creation can explicitly publish an
intermediate map transition. Closed connections expose empty presence immediately.
The unused SessionManager.UpdateMap mutation bypass was removed. Player actor
payload names are copied rather than pointing into mutable character data.

The PostgreSQL-backed session/world/server race suites passed. Additional tests
check coherent position/map publication and broadcasts while a recipient has an
unpublished map change inside a running command. Existing broadcast fixtures now
explicitly publish their setup state, matching the production command boundary.
This does not prove all NPC actor pointers, external callbacks, or shutdown
writers safe; those remain part of the unfinished ownership/lifecycle audit.

Timer lifecycle checkpoint: actor simulation, player movement and session timeout/
playtime maintenance now use one shared periodic-worker lifecycle. Start is
one-shot; stop is safe before start and across repeated/concurrent callers. Stop
signals the loop, stops its ticker and joins the running callback. World shutdown
joins all three workers before its final playtime flush. Tests hold each callback
open and verify both worker stop and world shutdown wait for release.
Race-enabled session, world and server suites passed against disposable PostgreSQL.
The runner exited nonzero during cleanup because the shutdown checkpoint needed
71 seconds of filesystem sync, exceeding pg_ctl's wait. Subsequent pg_ctl status
reported no server running and the cluster log confirmed completed shutdown.

This is not yet complete orderly server shutdown: transport admission, active
session removal/cleanup, background chat persistence and database-close ordering
still require a coordinated drain. Running callbacks also retain their existing
operation deadlines; this join does not introduce a global shutdown deadline.

Session shutdown checkpoint: the manager seals admissions before the world closes
connections, joins timers and drains every character command/cleanup. Cleanup
registration is synchronized with removal so a disconnect that already removed
its session from the manager cannot escape the shutdown wait. World shutdown is
one-shot and repeated callers wait for the same drain. Both transports reject a
failed admission and clean up a connection closed during transport registration.
Chat persistence now runs within the owning command with its existing five-second
query deadline, eliminating detached per-message database goroutines. This can
add database latency to chat delivery, but makes persistence part of the command
lifecycle rather than unbounded background work.

PostgreSQL-backed race tests passed for active-character shutdown and a disconnect
that already claimed cleanup, including final playtime persistence and ownership
retirement. Full session/world/server suites passed. HTTP handlers/listener drain,
startup admission/readiness and a server-wide shutdown deadline remain unfinished;
the database-close boundary is not yet proven safe for non-session HTTP work.

HTTP lifecycle checkpoint: the server owns an explicit http.Server, bound listener
and serve-completion channel for both HTTP and HTTPS. Startup binds synchronously
and returns TLS/configuration/bind errors to main, which performs cleanup before
exiting. Repeated start and start after stop are rejected. Constructor failures
stop world workers already created. HTTP header and idle timeouts are explicit.

Stop seals session admission, joins ordinary HTTP handlers and the HTTP serve
loop, stops external chat/QUIC, drains world sessions and finally closes the
specific database captured at construction. It no longer closes whichever global
database happens to be installed at shutdown. Shutdown is idempotent. A real local
HTTP request held during shutdown verifies that its query and world cleanup both
finish before database close, using an in-memory SQLite connection solely to
observe connection lifetime. Server/session race tests passed; this does not
substitute for PostgreSQL transaction or production verification.

A server-wide shutdown deadline, cancellation propagation, runtime serve-error
reporting and explicit readiness remain unfinished. HTTP shutdown currently waits
for handlers, and world cleanup waits for commands; a handler without a bounded
operation can still delay shutdown indefinitely.

Readiness checkpoint: `/api/ready` returns uncached 503 before listener startup,
after unexpected listener failure, during shutdown or when a one-second database
ping fails. Both deployment retry loops now use this endpoint. HTTP/WebTransport
serve failures publish the first error to main; main drains the server and exits
nonzero. Expected listener shutdown is not reported as a failure. This endpoint
must be deployed with the backend before a frontend-only lane can rely on it.

Race tests passed for readiness states, a real HTTP listener failure and normal
shutdown. Importer/contract tests, workflow YAML parsing and runtime asset
validation passed; PostgreSQL-specific importer cases were not exercised by the
plain Go test invocation. No live deployment or readiness claim is made. Content
preload methods that only log errors still need fail-closed startup propagation,
and shutdown deadlines/cancellation remain incomplete.

Required preload checkpoint: coordinate triggers, map scripts, spin tiles, warp
tiles and ordinary map warps now return query/scan/iteration errors; malformed
spin movement JSON returns the affected map and coordinate instead of silently
skipping it. Their staged caches publish only after complete success, preserving
the old cache when reload fails. World construction returns these errors to the
server constructor. Actor data loading is separated from actor timer startup,
and all world timers start only after the current preload sequence finishes.
Simulator coordinate-trigger consumers also propagate load errors.

World/server PostgreSQL race suites passed, including failed world construction
when a required table is unavailable. Tests cover query error propagation and
preservation of a previously valid cache after a malformed spin-tile row. All Go
packages compile. Actor/collision, trainer/wild encounter and cutscene preload
error propagation remains unfinished; readiness is not yet proof that every
content loader succeeded. Preload query deadlines and shutdown cancellation also
remain required work.

Encounter preload checkpoint: trainer and wild-encounter loaders use the world's
explicit database dependency and return errors for missing dependencies, failed
queries/scans and row-iteration failures. Trainers build their complete list and
map index before publication. Wild areas, slots and tile references stage as one
family; a failed later query cannot leave a new area list beside old tiles.
Missing encounter-area references report the area ID and affected tile coordinates
when available. The importer uses NULL for tiles without encounters; that normal
case remains excluded from encounter-cache loading. These immutable data loads
run before timers and do not provide a concurrent runtime hot-reload interface.

Validation includes PostgreSQL cases for valid loading, orphan slots, orphan tile
references and a failed tile query while retaining the previous complete cache.
The existing trainer-map alias fixture now supplies the explicit database rather
than relying on the global. Actor/collision and cutscene initialization remain the
next startup gaps, alongside preload deadlines and bounded shutdown.
All packages compiled. The broad world run passed the encounter checks but hit
two battle-fixture schema-creation timeouts during high filesystem I/O pressure;
the server race suite passed. A fresh focused PostgreSQL race run passed both
timed-out battle tests and all affected preload checks with unchanged deadlines.

Actor/collision preload checkpoint: actor loading stages overworld IDs, actors,
action timers, collision maps and raw-foot-tile provenance in a temporary manager.
Query/scan/iteration failures propagate to world construction; missing actor
coordinates report the object/map identity. The published manager is replaced
only after complete success. Actor rows are exhausted before collision queries,
so a single-connection pool can load without waiting on its own open cursor.

Lazy collision loading now takes the manager lock and stages both collision and
raw-foot-tile maps before publication. Failed queries no longer install an empty
cache entry; later requests can retry. Runtime callers log the error and decline
the lookup rather than treating a failed query as a valid loaded map. Actor data
still uses the legacy database access path; moving that dependency and validating
all preload deadlines remain part of the broader goal. Cutscene preload failure
propagation is the next outstanding initialization boundary.
Final world/server PostgreSQL race suites passed, including preservation on failed
actor preload, retry after a failed collision query, concurrent lazy readers and
single-connection loading. Existing actor identity checks passed through the full
preload entry point. These are headless checks, not rendered NPC verification.

Cutscene preload checkpoint: the loader now uses one canonical-schema query;
the chain of older queries that dropped prerequisite columns after arbitrary
errors is removed. Map names and scripts stage together and publish only after
complete success. Query/scan/iteration errors propagate through world construction
and the simulator. Invalid prerequisite/completion-flag arrays (including null or
empty entries) identify the script and field instead of becoming empty conditions.
Actions must be a typed array with nonempty action types, including nested action
lists; this is structural validation, not a claim of complete action semantics.

PostgreSQL race suites passed for world, simulator and server. New tests exercise
malformed conditions, completion flags and nested actions, rejected world startup,
cache preservation and a missing prerequisite column that previously could have
selected a weaker query. All packages compiled. No new schema or generated data
was introduced. Preload query deadlines, shutdown cancellation and the remaining
wire/domain migration still keep the full goal incomplete.

Preload cancellation checkpoint: all required world loaders accept a caller
context and use QueryContext throughout their database reads. World construction
caps its preload phase at one minute, matching the documented startup retry
window, and checks cancellation before starting timers. Startup receives the
process signal context; termination during world preload cancels its queries.
Lazy collision queries use a five-second deadline. Script/warp loader dependencies
are explicit *sql.DB handles rather than the older query interface without contexts.
Offline simulator calls explicitly supply their own context.

World/simulator/server PostgreSQL race suites passed. Tests cancel every required
loader, reject cancelled world construction, and hold a PostgreSQL table lock to
verify deadline interruption followed by a successful retry. Cache publication
also checks cancellation. All Go packages compile. Earlier schema/bootstrap work
before world construction and server-wide shutdown cancellation still require
coverage; this checkpoint does not claim that every startup/shutdown operation
is bounded. Lazy collision mutex acquisition is also separate from query timeout.

Database bootstrap cancellation checkpoint: database initialization now uses
PingContext, tile schema upgrades use BeginTx/ExecContext, and all scripted-event
sync queries explicitly receive their caller's context. The server gives database
bootstrap and world preload one shared minute, linked to SIGINT/SIGTERM. Failure
closes the opened handle; signal cancellation during bootstrap exits normally.
The startup deadline is separate from the running server's process context.
Importer and offline simulator callers explicitly supply their context; this
checkpoint does not claim deadlines for those standalone tools.

PostgreSQL race suites passed for database bootstrap, scripted events, simulator
and importer. Tests prove cancelled initialization preserves the existing global
handle, locked schema/sync queries observe deadlines, and retries succeed after
locks release. All Go packages compile. No schema definitions, generated assets
or content rules changed. Sync still needs an atomic publication boundary across
its stages, synchronous file loading is not interruptible, and bounded shutdown,
remaining state ownership, domain/wire migration and rendered integration remain
unfinished. Changes are local only.

Atomic scripted-event publication checkpoint: Sync now owns a PostgreSQL
transaction for all five content families and its required-column upgrades. It
loads the source file families first, locks the published tables before reading
prior state to serialize concurrent publishers, and returns applied statistics
only after commit. Duplicate-column error suppression was removed in favor of
ADD COLUMN IF NOT EXISTS, which does not abort the transaction. The existing
shared sync helpers execute against the same transaction; no second interpreter
or parallel content pipeline was introduced. Schema upgrades can block readers
until commit, consistent with startup/import publication before serving traffic.

PostgreSQL race tests prove rollback after a late dialogue lookup error, deferred
commit constraint failure, and cancellation after earlier writes. Snapshots cover
all five families and stored row identities; the late-error test also proves a
required-column upgrade rolls back. Successful retry publishes every family and
repeated publication preserves identities. Four concurrent publishers serialize
and only one applies changes. Scripted-event, simulator and importer suites
passed; all Go packages compile. The original goal remains incomplete, including
bounded shutdown, remaining ownership/mutation audits, domain separation, the
explicit wire-contract migration and rendered integration. No push/deploy.

Character wire-contract migration checkpoint: base database models now declare
explicit JSON tags and are excluded from the legacy generated-name postprocessor.
CharacterData uses a typed protocol view that embeds base fields and exposes the
existing parsed CharacterOptions object. Persistence retains its stored string,
which is excluded from JSON. The client interface returns typed options; the
network bridge and player store use the generated wire view. Wallet and bind
streams serialize tagged models directly. Tygo's supported inheritance tag and
explicit imported base mapping preserve the flat payload without copying fields.
The generator had previously represented options as a string despite the runtime
sending an object; this checkpoint fixes that observed disagreement.

Focused race tests verify actual framed Session.SendStreamJSON output, flat keys,
parsed options, timestamps, omitted optional fields and explicit tags;
character-adjacent world tests pass. PostgreSQL race suites passed for world,
simulator and server. All Go packages compile. Canonical generation is byte-stable
on a repeated run; frontend typecheck, runtime asset validation and production
build passed. Existing bundling warnings remain. No rendered gameplay check or
production deployment was performed.
The full casing migration remains incomplete: StructToMap and world postprocessing
still exist, and other query/gameplay families require typed declarations, field
and nullability audits, coordinated consumer migration and rendered integration.
The complete migration and retirement plan is documented in ARCHITECTURE.md.

Pokédex query contract checkpoint: species, status and trainer-card types were
mechanically extracted from handlers into protocol declarations, with typed
success/error responses. The handlers encode directly and use the world's
injected database. The bridge/store/consumer use generated types; handwritten
interfaces and field casts were removed. Existing nullable species fields remain
explicit null values with matching TypeScript unions. Absent optional cry values
are omitted, and empty collections remain arrays. JSON now publishes crySfx,
which the client expects; legacy StructToMap incorrectly published crySFX.

Query, reconciliation, scan and iteration failures now return typed errors rather
than partial success. Trainer-card database query failures also reject the
snapshot. Event-flag badge checks still use the existing boolean API and require
the broader failure/ownership audit. PostgreSQL race suites passed for world, protocol and simulator. A dispatcher-to-
framed-message test verifies cry metadata, nullability, empty status/badge arrays,
injected database use and typed errors for query/scan/reconciliation failures.
Frontend typecheck, focused Pokédex store tests, runtime asset validation and
production build passed; canonical regeneration is byte-stable. All Go packages
compile. Existing bundling warnings remain. Query cancellation and cohesive domain
services remain part of the unfinished original goal. This checkpoint does not
claim completion of the wire migration or rendered cry/UI verification.

Content detail contract/service checkpoint: Pokémon, move and item request and
projection types were extracted into protocol. Flat typed success views embed
the existing fields; required nullable values use matching TypeScript unions.
The direct encoder now respects hp, pp, isHm and defaultMove1Id through
defaultMove4Id, correcting legacy converter disagreements without aliases.
SQL reads were mechanically extracted into an injected content service with
caller cancellation and a five-second operation cap. Handlers decode/publish
and distinguish missing records, invalid requests and database read failures.
Local TM/HM previews now declare the generated-field subset they actually store;
no content records or generation inputs changed.

Focused PostgreSQL race checks prove flat framed responses through dispatch,
correct acronym keys/nullability, explicit database ownership, typed failures,
query cancellation, lock deadlines and retry. A locked background query also
proves the service's own five-second limit. Content and world/protocol/simulator
race suites passed. Frontend typecheck, runtime asset validation, production
build and repeat-generation stability passed; all Go packages compile. Existing
bundling warnings remain. The full migration, domain cleanup and server-wide
shutdown cancellation remain unfinished. No production or rendered gameplay
verification is claimed.

Map/learnset aggregate checkpoint: request, entry and response declarations were
extracted into protocol. Handlers delegate all metadata reads to the injected
content service and encode typed success/error responses. The service uses one
read-only repeatable-read transaction and one five-second context per aggregate;
every query receives that bounded context. A reusable typed collector closes rows
between queries, rejects scan/iteration errors and preserves empty arrays. Late
query or commit failure discards the entire projection. Direct JSON tags correct
rawASM, tMHMName and isHM disagreements while retaining source values and explicit
nullable fields. Existing map-ready cutscene handling stays after successful
metadata publication; failed publication no longer starts a cutscene.

Focused PostgreSQL checks prove snapshot consistency across a concurrent update,
late query failure, cancellation/retry, cancelled commit, empty arrays, nullable
fields, source movement strings and framed dispatcher responses. A locked final
query also proves the aggregate's own five-second budget for background callers.
Content/world/protocol/simulator race suites passed. Frontend typecheck, runtime
asset validation, production build and byte-stable regeneration passed; all Go
packages compile. Existing bundling warnings remain; no rendered check was run.
The map-ready handler still
passes the client's requested map name to cutscene selection; shared issuance
and native player-location authorization need the next audit. Other metadata
families, wire-helper retirement, domain cleanup, ownership/shutdown work and
rendered integration remain incomplete. No push or deployment.

Map-script location authorization checkpoint: map-ready metadata remains readable
for arbitrary maps, but its request cannot issue a cutscene unless the requested
name equals the player's server-resolved native map. Post-battle map scripts use
the same resolver, including local scripts in the unified overworld. Owned
movement state takes precedence over the character snapshot. Interiors resolve
by map ID; overworld coordinates resolve through the imported original tile
source-map identity, retained across edits/erasure. User tiles cannot manufacture
native identity, and missing/ambiguous provenance or SQL failures prevent issuance.
Queries use the injected world database with one five-second budget.

Dispatcher/framing PostgreSQL tests cover remote requests, valid interior and
overworld issuance, stale character position, edited/erased provenance, missing
identity, deliberately corrupted conflicting provenance and SQL failure. A
locked-table test verifies the query deadline and successful retry. World,
server and session PostgreSQL race suites passed. No frontend protocol change,
rendered gameplay check, push or deployment. NPC-click location/proximity checks
must still precede durable puzzle/door effects; broader shared issuance,
ownership/shutdown, domain/wire migration and integration work remain incomplete.

Scripted interaction authorization checkpoint: the actor ID now resolves through
the registry to a server-loaded object. Before any ordinary script issuance,
Vermilion trash state creation or Silph door flag write, the handler validates
effective visibility, movement-map identity and reach. It reuses locked/cloned
NPC runtime state, per-character position overrides and existing visibility
evaluators. The existing client rule is enforced: cardinal adjacency, or exactly
two cardinal tiles across an effective talk-over tile. Eligible event tile art
can introduce or remove that permission. Native overworld actor IDs and unified
player IDs share global coordinates; interiors remain separate.

The existing object/position/visibility/tile loaders now also accept context and
an injected database without duplicating their SQL. Authorization has one shared
five-second budget; missing data, SQL errors or hidden/distant targets cannot
fall back to issuing events. Expected denials stay quiet; terminal load failures
are logged with the object ID. Trigger keys come from the authorized object.

Dispatcher/framing PostgreSQL tests prove valid adjacency/counters, event counter
changes, diagonal/distant/other-map rejection, runtime and per-character position
precedence, visibility rules/overrides, unified overworld reach and SQL failure.
Remote puzzle and door requests leave durable state/flags untouched; valid
requests still create puzzle state and unlock a door. Locked visibility queries
time out and retry successfully. World/server/session/simulator PostgreSQL race
suites passed and all Go packages compile. No rendered check, push or deployment.
Other dialogue/trainer interaction authorization, atomic dynamic puzzle updates,
durable duplicate/reconnect delivery, broader domain/wire migration and bounded
shutdown remain part of the active original goal.

Dialogue/trainer authorization checkpoint: direct trainer clicks and battle-start
requests share the existing server object reach/visibility check. Battle start
rechecks after dialogue; the separate server-issued sight encounter path is
unchanged. Metadata queries and direct battle transactions use the captured world
database with bounded queries, and rebattle policy uses the session's loaded
typed options. Battle responses reuse the committed trainer name instead of
querying the global database again. Missing trainer dialogue rejects success.

Dialogue choices require the reachable runtime actor's actual text constant and
matching catalog map identity before any trade/action. This also removes the old
map lookup that confused runtime actor IDs with database IDs. Script-owned
prompts cannot bypass issued completion through the legacy choice opcode.
Required flags are checked against durable state under the same character lock
as the effects, using the existing transactional interpreter. Follow-up lookup
errors reject success instead of silently returning empty dialogue. Authorization
and catalog reads share one five-second context; mutations use the existing
bounded transaction boundary. Trades propagate that caller context throughout
their transaction and return a committed party snapshot, with no post-commit
global reload. Existing trade completion rows still deduplicate retries.

The real PostgreSQL trainer-start test exposed nested queries while trainer party
rows were open on a transaction's single connection. BuildTrainerParty now stages
and closes those rows before species/move reads, and rejects scan/iteration errors
instead of accepting partial parties. Other inspected dbloader loops do not nest
queries inside row iteration; their remaining error-handling audit is unfinished.

Dispatcher tests with the global database removed cover wrong actor/prompt/map,
distant/hidden trainers, movement after dialogue, durable flags disagreeing with
cache, valid choices/trades/battles and repeated battle start. PostgreSQL trade
checks prove late-failure rollback preserving Pokémon row identity, cancellation,
retry and durable duplicate protection with commit-before-party publication.
World/pokebattle/simulator/server/session race suites passed; canonical generated
contracts remain unchanged. All Go packages compile and frontend typecheck passes.
No rendered gameplay check, push or deployment. Atomic dynamic puzzles, durable
choice duplicate/reconnect delivery, remaining interaction/mutation audits,
domain/wire migration and bounded shutdown remain in the active original goal.

## Reproducible PostgreSQL tests

With `initdb`, `pg_ctl`, and Go available, run:

```bash
bash scripts/testing/run-go-postgres.sh
```

The runner creates its own cluster under `/var/tmp`, listens only on its private
Unix socket, runs race-enabled Go tests, and stops only that cluster. Each test
uses an isolated schema built from the canonical runtime schema. No application
configuration or production database is read. A CI PostgreSQL service can instead
set `CAPTUREQUEST_TEST_DATABASE_URL` explicitly; tests skip without that variable.
