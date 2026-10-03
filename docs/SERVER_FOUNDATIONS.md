# Server foundations goal

Status: active. Started 2026-09-25 from `02c51ba`.

Working branch: `codex/server-foundations`. Latest checkpoint: issued ordinary
player steps (implementation checkpoint; rendered acceptance incomplete, 2026-10-02), following owned-only MapLoad `0585dde`, committed
blackout/recovery `2f62595`, teleport notification projection `65a5581`, Instant
Warp `e1f54a8`, normal warps `3899660`, owned-position loading `64cf970`,
provenance `057f758` and atomic map-load `b8f5ccd`.
Earlier foundation checkpoints remain in this branch's history. No push or production deployment
is authorized by this goal.

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

### Checkpoint handoff (2026-10-02)

Implementation checkpoints are committed locally on `codex/server-foundations`.
The goal remains active; no push or deployment is part of these checkpoints.
MapLoad now rejects supplied destinations. Ordinary walking requests a direction
from its expected owned source, animates a server-issued step, then acknowledges
its token before continuing the path. Blackout, Safari and explicit warp commands
publish server-committed destinations.

Before the next migration, resolve the two rendered movement failures documented
below and recheck input cancellation while an issued step is completing. Then
continue retiring legacy opcode 45: scripted/field animation reports and
facing updates still share its broader coordinate authority. Bind those to issued
script/field results and owned facing state before retiring that writer. Step
side effects and durable result recovery also need further work. Evidence and
verification limits appear in the checkpoint sections below.

There is no reliable overall completion percentage: the remaining endpoint
and ownership audits can reveal additional work. Use the five-area status
table and the acceptance checks below to assess completion, rather than the
number of commits or passing tests. All five areas still have outstanding work.

| Area | Implemented | Still required |
| --- | --- | --- |
| Request/session boundary | Packet and connection limits, centralized session prerequisites, removal of insecure session takeover, actual transport closure, location/visibility checks for scripted clicks, dialogue choices and direct trainer battles, client destination catalog validation, server-resolved normal warp activation, explicit Instant Warp commands, committed teleport notification contracts and read-only map metadata, and preserved command deadlines/disconnect cancellation in migrated operations. | Audit remaining interaction/mutation endpoints; propagate cancellation through legacy managers and remaining database/network work. |
| Durable gameplay | Shared bounded transactions; atomic shops/inventory, stable Pokémon row identities, party/item changes, battle persistence, script rewards/completion, trade rollback/deduplication, atomic Vermilion puzzle transitions, item-ball collection, Silph doors, Game Corner prizes and bounded coin/slot/hidden-coin operations, atomic Escape Rope/FLY positions, durable Repel counters, Safari entry/turn/capture state and exhaustion destinations, atomic blackout/recovery destinations and map-load position/Safari/flag/visibility/boulder effects, and commit-before-publication in migrated paths. | Finish remaining dynamic puzzles, pickups, prize/field-effect paths; durable duplicate protection and recovery of committed results across reconnects. |
| Character ownership | Bounded serialized session commands, exclusive character ownership and drained handoff, stale-cleanup guards, immutable cross-session presence, and movement ticks coordinated with the owner. | Finish timer/callback/shared-state and legacy position-writer audits; prove remaining concurrent/reconnect behavior across real transports. |
| Domains and wire contracts | Injected content-query service; typed character/wallet/bind, Pokédex/card, content detail, map-script, map-info/list and learnset contracts generated from explicit JSON names. | Migrate remaining gameplay/query families and global dependencies; retire `StructToMap` and the casing postprocessor after every consumer moves. |
| Lifecycle and verification | Owned HTTP/listeners, readiness, listener failure propagation, joined periodic workers, sealed session admissions, fail-closed staged preload, startup cancellation, atomic scripted-event publication, and deadline-aware shutdown waits with returned failure results. | Audit cancellation of remaining legacy work, define durable final-save recovery, and complete transport/rendered integration coverage. Owned HTTP and player transport retirement and isolated active-player shutdown checks have landed. |

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
   Map-load arrival/recovery and script effects now also share one transaction,
   using the owned movement location for no-destination loads;
   post-commit cache recovery remains unfinished. Extend the shared transaction/domain operations already
   in use. Acceptance:
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
   Complete underlying cancellation and transport retirement beneath the
   deadline-aware shutdown result API, preserving persistence ordering. Acceptance: shutdown
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

## Issued ordinary player steps (2026-10-02)

Ordinary walking previously started a local path and sent arbitrary coordinates
through opcode 45 after each animation. The server accepted catalog membership
without proving a permitted step from owned location. Fresh entry did not even
register movement: the first legacy report created that state. This checkpoint
registers movement when character ownership and durable location are established,
before publishing entry success, including after a drained connection handoff.

New protocol pairs 187/188 (intent/result) and 189/190 (completion/result) are
generated from explicit Go JSON fields. Intent contains the expected source and
direction, never a destination. The server resolves one cardinal step or a source
ledge jump using the shared character collision model, including event/Cut
changes, boulders, NPC visibility/runtime positions and surfing eligibility.
Acceptance issues one random token bound to the current movement registration.
It does not save or publish a new position. A replacement intent at the same
source retires the previous token, so a cancelled request does not block retry.

Completion contains only token and correlation ID. It checks the session/source,
expiry and current dynamic collision, waits under the command context until the
issued movement duration has elapsed, then saves position in the existing bounded
character-locked transaction. Commit precedes owned-state refresh, response,
multiplayer broadcast and step effects. A failed commit publishes only the owned
location in an error response; retry requires a new intent. Replay, connection
replacement, map/position replacement and stopped server movement invalidate the
old token. One pending step has a ten-second lifetime; it is session state, not
a durable replay/result log.

The shared collision and visibility loaders now expose injected, context-aware,
error-returning paths. A failed event/NPC/boulder/property read rejects movement
instead of treating the obstacle as absent. Boulder result rows are closed before
reading visibility rules, avoiding nested connection acquisition. Existing
character pathfinding uses the same collision model. Two Cut tests now supply an
empty database-backed blocker catalog while preserving their path assertions.
Legacy wrapper/global consumers remain on the broader dependency roadmap.

The client waits for acceptance before queuing the visual step and waits for
completion before starting the next path step or activating its warp. Reads,
warps and movement share one correlated request settlement primitive with timeout,
abort, listener cleanup and stale-response filtering. Scene retirement and snaps
abort and retire pending work. Discarding future input preserves a currently issued
animation's token so its completion cannot become a legacy position report.
Rejected completion reconciles from the server-owned error location; a timeout
stops the path and does not prove rollback or automatically replay the command.

Verification: 40 focused frontend tests pass across movement, correlated requests,
map loading and committed warp presentation. Typecheck, production build, runtime
asset validation and canonical protocol regeneration pass. Asset validation checks
826 tile images, 92 sprites and 561 compact audio files. The final isolated
PostgreSQL race-enabled world/protocol/simulator suites and server compilation are
recorded in `/var/tmp/capturequest-checkpoint-go.log`.

Rendered acceptance is **incomplete**. The initial isolated run passed 19 of 23
cases and revealed missing fresh-entry movement registration. After that fix,
`/var/tmp/capturequest-rendered.TB5rDF` passed 21 of 23 cases. The unresolved cases
are movement immediately after Instant Warp (server rejected a stale source) and
Red's house exit (client requested an ordinary step onto a blocked carpet tile).
A focused Instant Warp repeat in `/var/tmp/capturequest-rendered.M3WDf8` passed;
that single pass does not resolve the intermittent failure. Detailed owned/requested
source diagnostics are now available behind debug logging. Initial broader checks
also caught database-free Cut fixtures and a lost old-map despawn during
actor-broadcast extraction; both were corrected without weakening assertions.
These evidence directories are local scratch records, not permanent release receipts.

Immediate follow-up:

1. Route blocked carpet entry through the existing server-resolved warp activation
   boundary rather than permitting an ordinary step onto blocked collision.
2. Reproduce the Instant Warp race with repeated rendered checks and inspect the
   owned versus requested map, coordinates and session. Fix the actual lifecycle
   ordering before claiming success.
3. Check cancellation/new-input overlap: `stopMovement()` preserves an issued token
   but clears `isMoving`. Prove a second input cannot replace an in-flight animation
   or completion, and cover reconciliation after rejection and lost responses.
4. Rerun the integrated rendered cases before treating this checkpoint as ready
   for a separately authorized release.

Remaining: retire opcode 45 coordinate writes for facing/script/field animations;
authorize movement against issued interaction/cutscene phases; make position and
applicable durable step effects/recovery coherent across failure; recover lost
acceptance/completion results and reconnects; measure latency and query load over
realistic transports. Day Care, Safari and encounter/script effects still run
through their existing operations after position commit. This checkpoint does not
make that full sequence atomic or prove every callback race. The five-area roadmap
above remains active. Next: resolve the rendered failures and input-overlap audit,
then migrate facing/issued script acknowledgements and step-effect recovery. No push or deployment is included.

## Owned-only MapLoad requests (2026-10-02)

MapLoad still accepted arbitrary supplied coordinates after the active teleport
producers had moved to server-committed destinations. That compatibility path
could overwrite owned location and apply arrival effects selected by a client.
The generated request now contains only `mapId` and `requestId`. The handler
rejects unknown fields (including null or partial destination fields) and trailing
JSON, requires the current normalized map, and obtains coordinates from owned
movement. Existing server-selected zero-position recovery remains; ordinary
loads preserve path, facing, surfing and previous-map state.

The client no longer supplies coordinates or carries the `warpServerCommitted` /
`destinationServerCommitted` registry compatibility state into map loading.
Destination coordinates remain presentation data for committed teleport snaps.
Map-load persistence and effects continue through the existing transaction.
Legacy teleport effects still occur in a subsequent load transaction, rather
than becoming atomic with the initiating teleport in this checkpoint.

Tests reject remote loads and current-map forged coordinates without changing
stored/live/movement state, exercise trailing JSON at the handler boundary, and
assert that client requests contain only map identity and correlation ID.
Transaction fixtures now begin from owned movement while retaining stale
published/durable snapshots, so late failure, retry, native provenance and
recovery assertions continue to test persistence rather than request rejection.

Verification: all 25 focused frontend request/loader/warp-store tests passed,
including cancellation, timeout and stale-response coverage. Frontend typecheck,
production build, runtime asset validation (826 tiles, 92 sprites, 561 audio
files) and stable canonical protocol regeneration passed. Isolated race-enabled
world/protocol/simulator suites passed (world 27.569 seconds); the server compiled.
All 19 isolated rendered normal-warp, Instant Warp, multiplayer visibility,
Safari and blackout cases passed in 2.5 minutes. Evidence is retained at
`/var/tmp/capturequest-rendered.A8Bjng`.

Initial verification found fixture issues: native coordinates needed float64
conversion for the character snapshot, movement registration seeded the session
map before the rollback assertion, and malformed frames were rejected before
handler dispatch. The corrected fixtures preserve the intended late-failure and
unchanged-state assertions. Trailing-data handler checks are direct calls;
valid forged destination fields run through the packet dispatcher. No assertion
or expected gameplay destination was relaxed.
The next movement migration needs an accepted intent boundary: current
`queuePredictedPathMove` starts a local path, `moveToNextTile` queues its visual
step, and `onStepComplete` reports coordinates through opcode 45. The cutscene
sync callback also sends that report. Simply comparing every report to owned
position would stop legitimate walking because these paths do not submit a
server-accepted ordinary movement command first. Introduce that shared boundary
and bind animation acknowledgements to it and issued script sequences.

Remaining: opcode 45 walking/scripted location authority, post-commit cache
recovery, durable replay/reconnect outcomes, delayed/session races and the full
five-area roadmap above. Next: bind movement reports to accepted movement and
issued script sequences. No push or deployment is included.

## Blackout recovery, Safari presentation and explicit test probes (2026-10-02)

The shared blackout operation charged the wallet but did not save its returned
Pokémon Center destination. Both committed battle losses and the older battle-start
recovery path relied on a later browser position report. The older path also sent
fallback destination coordinates after persistence failed and used an unbounded
global database/context wrapper. The shared transaction now saves the destination
and Safari exit with the wallet deduction. Battle settlement already heals the
party in its transaction; standalone recovery now heals and saves the stable party
rows in that same bounded, character-locked transaction using injected storage and
the command context. A failure sends no blackout destination. Publication refreshes
Safari state and updates owned position before sending the result and the typed wallet snapshot. Existing options
and their documented default center remain the destination source.

The blackout store uses shared committed warp presentation, with no position echo
or supplied MapLoad destination. Battle-start recovery can arrive before any battle
panel opens; the bridge presents that committed destination directly instead of
waiting for an unopened panel to be dismissed. Existing battle losses retain their
panel dismissal flow. Safari exhaustion notifications now use an explicit generated
DTO with required destination coordinates; delayed dialogue dismissal marks its
warp presentation committed and no longer invents fallback coordinates. The two
server Safari expiry producers already save/publish the gate before notification.

Both test warp producers now await the explicit Instant Warp command before
presentation, including engine-probe commands forwarded through TileViewer. Failed
probes reject without locally changing position. Test setup therefore exercises the
same catalog validation and commit-before-publication boundary as the active tool.
The new `debug_blackout_empty_party` scenario is a synthetic integration fixture,
not recovered historical source data; map 51, center 41 and the usable fixture tile
(6,10) were verified against the authoritative local extractor catalog.

Focused PostgreSQL checks prove standalone late party-save rollback of wallet,
position and Safari with no destination publication, successful retry/party healing,
and battle-loss position rollback/publication within the existing battle transaction.
All 25 focused frontend store/request/loader checks, typecheck, asset validation,
production build and stable canonical generation passed. Eight isolated rendered
blackout, Instant Warp and Safari cases passed (55.6 seconds), with evidence at
`/var/tmp/capturequest-rendered.5TMFFz`. The focused rendered test-warp probe also passed (4.4 seconds), proving valid
movement and rejection without local position changes, at
`/var/tmp/capturequest-rendered.k2yYn5`. The broader check identified and migrated
the simulator blackout caller to the same explicit recovery transaction; it no
longer uses the removed global blackout wrapper. Fixture validation then identified
blocked Viridian Forest (5,10); its catalog collision_type was 0. The synthetic
fixture now starts at (6,10), collision_type 1, and the fixture validation passes.
The corrected blackout browser rerun passed (5.7 seconds) at
`/var/tmp/capturequest-rendered.CDUT9p`. The nine applicable browser cases therefore
have passing evidence across these three runs; the full set was not repeated after
the fixture-only correction and addition of wallet publication. Final isolated race-enabled world/simulator/protocol suites passed and the server
package compiled.

Remaining: reject supplied MapLoad coordinates and remove the client compatibility
plumbing now that these producers have migrated. Walking/facing and scripted
animation reports still retain opcode 45 location authority. Legacy teleport arrival
effects remain a subsequent MapLoad transaction. Audit battle-start eligibility,
repeated recovery commands (especially an empty party), delayed dialogue/session
races and durable result recovery; this checkpoint does not prove replay protection
or reconnect recovery. The original five milestones remain active. No push or
deployment occurred.

Next: retire supplied MapLoad destinations with forged-field boundary tests, then
bind walking and scripted animation acknowledgements to accepted server movement.

## Committed teleport notifications and snap projection (2026-10-02)

The production `WarpTileTeleportNotify` producers were audited: movement warp
pads, elevator selection, DIG/TELEPORT, committed Escape Rope/FLY results,
cutscene transaction publication, emergency Warp Home and debug scenario jumps.
Each producer commits its destination before notification. They now share one
explicit protocol DTO and notification function, with `mapId`, `x`, `y` and
`direction` generated directly from JSON tags. The field teleport helper also
rejects invalid sessions instead of bypassing persistence and still notifying.
The network bridge marks this opcode's presentation as committed. Its scene
transition no longer reports the destination through opcode 45 or supplies
MapLoad coordinates; loading reads the server-owned location.

A second echo existed inside the shared actor renderer: snapping an actor invoked
the walking step-completion callback. That callback updated context and sent a
position report, even when the snap merely projected a committed teleport.
Movement callbacks now identify `step` versus `snap`; both refresh local context,
but a snap clears stale predicted movement and issues no position report or warp activation. Committed teleport
presentation stops the queued user path and snaps immediately, retiring the old
tween instead of waiting for a source step report after the server arrival.
Actual walking step completion retains its existing behavior pending migration.

Verification: isolated race-enabled world/simulator/server/protocol checks and
44 focused frontend tests passed, along with typecheck, runtime asset validation,
the production build and stable canonical generation. All 18 isolated rendered
field-move, Instant Warp, normal-warp, multiplayer and Warp Home cases passed
(2.3 minutes), with evidence at `/var/tmp/capturequest-rendered.MC8dBU`. Final snap
prediction cancellation has focused test coverage; the rendered run preceded
that small addition and did not simulate server catch-up under network lag.
Existing bundling warnings remain. These are local checks, not production evidence.

Remaining boundaries are explicit: legacy server teleport producers commit
position/Safari changes first, while arrival script effects still run in the
subsequent MapLoad transaction. This checkpoint does not make those effects
atomic with every initiating action. Blackout and delayed Safari-exit presentation
use separate response/store paths and retain their client reports until audited.
The test bridge's direct warp probes also retain the older destination path and
must migrate to explicit Instant Warp. Ordinary walking/facing and scripted
animation acknowledgements still share opcode 45; supplied MapLoad coordinates
cannot be retired until every remaining producer moves. Replay/reconnect and
post-commit cache recovery remain required across the full five-part goal.

Next: migrate blackout/Safari presentation and test warp probes, then bind walking
and scripted animation acknowledgements to accepted server movement and remove
the remaining arbitrary destination paths. No push or deployment occurred.

## Explicit Instant Warp command (2026-10-02)

Instant Warp intentionally remains available to ordinary players for arbitrary
non-erased catalog tiles. The old controller changed the local movement origin
before a position report had committed, and cross-map warps used both a position
report and supplied map-load coordinates. It now sends an explicit destination
command (185/186), awaits a correlated committed result, then uses the shared
warp presentation with `serverCommitted: true`. It stops and settles source
movement before admission, holds input during the request, preserves the choice
on errors for retry and aborts the local wait on scene cleanup. Same-map results
restore camera follow and request refreshed actors. Cross-map loads supply no
destination coordinates. The old optimistic position update/report is removed.

The server strictly decodes the request, requires both coordinates (zero and
negative values remain valid), rejects battle activation and normalizes native
overworld IDs using catalog data within the transaction. A bounded,
character-locked arrival transaction validates the tile, saves the destination,
ends Safari when appropriate and applies map-load effects. Only a successful
commit updates live presence/movement and sends the result. PostgreSQL packet
tests cover absent/erased/unknown destinations, missing and extra fields, battle,
deferred effect failure with position/Safari/cache rollback, retry, zero
coordinates and native/negative overworld destinations. Shared client request
tests cover correlation, timeout, errors, retry, synchronous send failure and
abort. Controller checks prove single admission and no presentation before the
result or after scene retirement.

Verification: isolated PostgreSQL race-enabled world/simulator/server/protocol
checks and 26 focused frontend tests passed, along with typecheck, runtime asset
validation and the production build. All five rendered Instant Warp and
multiplayer cases passed (47.1 seconds) at
`/var/tmp/capturequest-rendered.ZqU6N6`, including immediate keyboard/click movement,
far overworld destinations and old-map visibility removal. Canonical protocol
regeneration is stable. Existing bundling warnings remain. This is a local
checkpoint; no push or deployment occurred.

Still required: ordinary walking/facing and cutscene completion reports retain
legacy position authority, and other teleport producers still need auditing.
Supplied MapLoad destinations and opcode 45 cannot yet be retired. Request IDs
correlate local waits; they do not supply durable replay protection or recover a
commit lost with the response. Post-commit cache refresh failures remain logged
without guaranteed recovery. The original five-part goal remains active.

Next: bind walking and scripted animation acknowledgements to accepted server
movement, remove redundant reports from server teleport producers, then retire
supplied MapLoad destinations once every producer has migrated.

## Server-resolved normal warp activation (2026-10-02)

Normal browser warps previously selected a destination from loaded metadata,
added the building-exit step locally and reported the result as unrestricted
position coordinates. The active WarpManager now sends only `warpId`, direction,
click/keyboard intent and a request ID through opcode 183; result opcode 184 is
an explicit generated protocol contract. The server resolves the catalog row and
per-player `LAST_MAP` context using the transaction handle. It reuses the existing
door/carpet reach and facing rules, rejects inactive/elevator rows, checks keyboard
direction and durable Safari entry eligibility, and rejects activation during battle.
The current owned location supplies source authorization. The destination, Safari
transition and map-load effects commit together through the existing arrival
transaction. Missing/erased destination tiles reject instead of publishing success.

The existing building-exit step is now part of the committed destination; the
response includes its animation start. The client waits for source movement to
settle, holds input while activation is pending, and changes the scene only after
its correlated success. Errors preserve the scene; timeout, abort or scene shutdown
release the wait. Normal-warp events skip the redundant position report and their
map load reads the committed location without supplying destination coordinates.
The old local normal-warp destination calculation is removed. Shared request
settlement tests cover concurrent requests, mismatched/late replies, timeout,
retry, synchronous send failure and cancellation.

Race-enabled world/simulator/server/protocol suites passed. PostgreSQL packet tests
cover remote and distant sources, forged destination fields, inactive/elevator rows,
late effect failure with position/Safari rollback, retry, dynamic `LAST_MAP`,
committed building-exit coordinates, keyboard direction, battle and Safari access.
Frontend focused checks (18 tests), typecheck, asset validation, production build
and stable canonical protocol regeneration passed. The first rendered run passed ten of
fourteen cases and exposed two causes: adjacent walkable doors were incorrectly
rejected by the shared facing rule (warp 245, owned Kanto tile 5,6), and test
`waitForMap` returned on metadata identity while rendering was still loading
(the mart trace clicked viewport -84,-84). The shared door rule now accepts the
adjacent facing tile; carpets retain their collision restrictions. The test helper
now additionally waits for map loading completion before input. The next run
passed 13 of 14 cases at `/var/tmp/capturequest-rendered.XbckVJ`. Its remaining
Cinnabar Lab failure exposed a same-map scenario-jump race: the old scene briefly
reported ready before the replacement began loading, so the helper pressed UP
while the replacement was loading. Scenario jumps now observe the replacement's
`cq:mapChanged` event before waiting for loading completion. The focused Cinnabar
rerun passed (11.7 seconds) at `/var/tmp/capturequest-rendered.Tpl7P5`. All 14
normal-warp, Instant Warp and multiplayer cases therefore have passing rendered
evidence across these two runs; the complete set was not rerun after the final
helper change. No expected warp destinations or assertions were relaxed.

This is a local checkpoint on `codex/server-foundations`, with architecture and
warp-boundary documentation updated alongside the implementation. No push or
deployment occurred. Next: introduce an explicit Instant Warp command, then
retire the remaining position-report and supplied map-load destination authority
as the audited producers migrate.

The producer audit for the next retirement is:

| Producer | Current role | Required migration |
| --- | --- | --- |
| `WarpManager.activateWarp` | Correlated server-resolved normal warp | Active migration complete; add durable result recovery. |
| `TileViewerInteractionController` Instant Warp | Explicit correlated catalog destination command (185/186) | Active migration complete; add durable result recovery. |
| `PlayerMovementController.onStepComplete` and direction updates | Ordinary steps use issued direction commands and token completions; turning/boulder and script/field callbacks retain opcode 45 | Retire legacy coordinate authority after the remaining issued-animation and facing migration. |
| TileViewer cutscene movement callback | Reports scripted animation coordinates | Bind acknowledgement to issued script movement instead of accepting coordinates as authority. |
| Blackout/Safari store events and test warp probes | Committed recovery presentation and explicit Instant Warp probes | Active destination producer migration complete; verify remaining delayed/session races. |
| `MapLoader.prepareMapLoad` | Loads current owned position using mapId/requestId only | Destination fields retired; retain failure/retry and stale-session coverage. |

Remaining migration: Instant Warp now uses the explicit command documented above. Walking and scripted
animation reports still share that position opcode; audit and replace their
location-changing authority with accepted movement/command results. Other server
teleport producers must stop redundant reports/load destinations too. Opcode 45 and
supplied map-load destinations are therefore not retired by this checkpoint.
Correlation is not durable replay protection: define how retries/reconnects recover
a committed normal warp. Do not claim the complete movement boundary is secured
until these remaining paths and failure races are migrated and verified. The full
server-foundations roadmap remains active; no push or deployment is authorized.

## Owned-position map loading (2026-10-02)

A current-map load previously authorized the map against movement state but read
coordinates from the character snapshot. It could recover a stale `(0,0)` snapshot,
run effects at an older tile and acknowledge that older location. Map loading now
reads one owned position through the shared movement-first helper used by script
issuance, interactions, pickups, Game Corner and Safari eligibility. Before movement
registration it reads the selected character. Current-map authorization, recovery
selection and effects use that same snapshot.

Every accepted map-load command now persists the selected location together with
its effects, including a load with no supplied destination. A shared post-commit
projection helper refreshes character/session coordinates and marks the matching
movement position committed. A current-location load leaves queued movement,
facing, surfing and previous-map intent intact. Supplied destinations and genuine
zero-position recovery retain the existing teleport publication behavior. Replies
use the transaction's accepted map and coordinates. A stale view cannot authorize
another map's effects. This does not add destination eligibility or durable request
deduplication.

Focused PostgreSQL packet tests passed for stale zero/nonzero character snapshots,
a remote stale map, deferred position/effect commit failure, retry and preservation
of movement intent. They deliberately keep the movement registration ahead of the
character/session projections. A separate test checks that genuine owned zero
coordinates recover to the existing spawn and run that destination's effects.
Race-enabled suites passed for `internal/world`, `internal/scriptsim`,
`internal/server` and `internal/protocol`; `git diff --check` passed. All five
isolated rendered Instant Warp and multiplayer visibility checks passed, with
evidence retained at `/var/tmp/capturequest-rendered.CoXMLC`.

Remaining: audit other legacy position readers/writers and callback ownership;
replace shared client position authority with issued warp grants and explicit
Instant Warp intent; define committed-result and flag-cache recovery after loss or
cancellation; complete domain/contract and lifecycle integration work. The full
roadmap remains active. No push or deployment is part of this checkpoint.

## Native provenance for map-load effects (2026-10-02)

Arrival no longer identifies Route 20 with a coordinate rectangle. Runtime arrival
and script issuance now share an original-tile provenance resolver. Arrival resolves
that native map inside its character-locked transaction, including the existing
Pallet Town effects that the old overworld selector skipped. The native map's ID
and name come from the catalog rather than caller-supplied effect metadata.
`original_source_map_id` takes precedence over edited `source_map_id`; erased
original tiles retain script identity. A pure user-added location has no native
load effects and cannot manufacture them by setting a source map ID. Script
issuance still requires a native identity. Missing, empty, non-overworld or
conflicting original identities fail closed; arrival position/effect writes roll
back together. The unused map-entry display lookup and its global SQL shim were
removed; display-name formatting remains unchanged.

The inspected local SQLite source contains 43,380 overworld tiles, all linked to
an overworld map; Pallet Town has 360 and Route 20 has 1,800 tiles. This is source
artifact evidence, not production verification. Focused PostgreSQL boundary tests
cover Route 20 outside the old rectangle, an edited neighbor inside it, user-only
tiles, Pallet Town effects, missing/conflicting provenance rollback and corrected
retry. Existing script-location checks cover edited/erased originals, ambiguity,
cancellation and retry. Race-enabled suites passed for `internal/world`,
`internal/scriptsim`, `internal/server` and `internal/protocol`.
`git diff --check` passed. All five isolated rendered Instant Warp and
multiplayer visibility checks passed, with evidence retained at
`/var/tmp/capturequest-rendered.T587LI`.

Remaining: audit no-destination arrival against the owned movement snapshot;
replace broad client destination authority with issued warp grants and explicit
Instant Warp intent; define post-commit flag-cache and result recovery; migrate
remaining global dependencies and wire families. The full roadmap above remains
active. No source schema, generated assets, import pipeline or production state
changed in this checkpoint.

## Atomic map-load persistence checkpoint (2026-10-02)

Previously arrival could commit a position or flag before a later visibility or
boulder update failed. Conditions also read cached flags and helpers used the
global database. Map-load arrival now locks the character and commits destination
validation, saved position, Safari transition, durable flag decisions, flag writes,
object visibility overrides and boulder resets through one injected transaction.
Runtime and simulator use the same effect implementation. Recovery applies the
actual recovery map's effects. Live position, movement/presence publication and
successful acknowledgement follow commit; rolled-back effects never enter the
flag cache. The existing bounded transaction honors command cancellation.

Race-enabled PostgreSQL suites passed for `internal/world`, `internal/scriptsim`,
`internal/server` and `internal/protocol`; the additional focused boulder-reset
test also passed. New PostgreSQL fixtures exercise deferred commit failure across position/Safari/flags,
compound Daisy visibility/flags using durable eligibility despite a stale cache,
boulder deletion/flag rollback with unrelated-position preservation, and
cancellation while an effect query is blocked, followed by successful retry.
The fixture preloads actor collision residency before disabling the global DB;
this proves injected persistence, not removal of the actor manager's legacy global
collision dependency. All five isolated rendered Instant Warp and multiplayer
visibility checks passed; evidence is retained at
`/var/tmp/capturequest-rendered.gEqWOt`. `git diff --check` passed. No frontend or
generated-asset files changed in this checkpoint.

Remaining: Route 20 still uses the existing coordinate rectangle to select its
native load effect; migrate that selection to authoritative tile provenance.
Normal warp producers still report positions alongside map-load requests; replace
that shared client position authority with issued warp grants and explicit Instant
Warp intent. Flag-cache refresh happens after commit: a refresh failure is logged
and cannot undo durable work. Define cache/result recovery after cancellation,
transport loss or reconnect. Other managers still retain global dependencies.
These limits and the full remaining-work list above keep the goal active.

## Read-only metadata and correlated map-load commands (2026-10-02)

`PhaserMapInfoRequest` (34) now reads catalog metadata only, including the
current map. It cannot recover a position, change session coordinates/presence,
or execute map-load flags. It accepts the typed `mapId`/`requestId` contract and
rejects legacy destination/unknown fields and trailing JSON. Responses retain
flat metadata fields and add explicit success and request correlation.

The existing gameplay arrival/recovery/effect body now has one explicit command,
`PhaserMapLoadRequest` (181), with result opcode 182. Both require a selected
character through the existing session admission boundary. Loading a saved
location requires the player's current visible map; a supplied destination
requires a complete coordinate pair and the existing catalog-validated bounded
position transaction. Live position, presence and the arrival acknowledgement
follow a successful destination commit. Late-commit fixtures now target the new
command and still prove rollback rather than failing early on retired fields.
This is a migration of the active setter, not a second permanent mutation path.

The shipped loader awaits this command before metadata and actor queries for
ordinary gameplay loads, even when metadata is cached. An interior-to-overworld
map overview reads metadata without executing the command. Position recovery and
existing load effects remain gameplay responsibilities. Normal warp producers
still also send position reports; issued grants and explicit Instant Warp intent
must replace that broader shared position authority next.

Map reads and load acknowledgements share one client correlation/settlement
primitive. IDs are unique across service instances, and only a matching response
can settle a request. Success, error, timeout, synchronous send failure and local
abort all remove the response subscription, timer and abort listener. Errors
reject promptly; timeouts/retries and late responses cannot satisfy a newer
request for the same map. The map loader aborts its outstanding map requests when
a newer load supersedes them or scene cleanup runs. A local abort does not undo
an already committed server command; these IDs do not provide durable command
retry deduplication or reconnect recovery.

The new flat success/error unions generate from protocol JSON tags, including
explicit TypeScript extension metadata for the embedded map projection. The
bridge dispatches the new result opcode. No old destination alias remains on the
metadata endpoint. A future deployment must coordinate frontend/backend versions;
old clients sending destination fields or omitting correlation receive rejection
and require refresh/reconnect. No deployment is authorized or performed here.

Verification: world/server/protocol PostgreSQL race suites and all Go packages
compile; the final focused destination/metadata/contract checks pass. Frontend
request/lifecycle tests cover cancellation, supersession, cleanup, timeout, retry,
concurrent same-map queries, late responses and identity disagreement.
TypeScript typecheck, runtime asset validation and the production build pass.
All five isolated warp/multiplayer browser cases pass in
`/var/tmp/capturequest-rendered.nnVvMk` (45.6 seconds). Canonical `npm run tygo` regeneration is byte-stable.

Remaining: make map-load effects atomic and propagate cancellation through their
legacy helpers; coordinate input release with exact collision residency; validate
ordinary movement and issued warp/Instant Warp intent; make committed-result
recovery durable across disconnect/process failure. Other map-data queries still
need response correlation and timeout/cancellation cleanup. Preserve the full
six-item remaining-work roadmap above. The goal remains active.

## Bounded map content queries and explicit map DTOs (2026-10-02)

`PhaserOverworldMapsRequest` previously queried the global database without a
context, skipped scan failures, used `StructToMap`, and assigned
`session.MapID = maps[0].ID`. Reading a map list could therefore change the
player's visible-map tracking to the first native overworld map. The list is now
a pure read through the existing injected `content.Service`: it preserves
session/character presence, orders maps by ID, returns `[]` for an empty catalog,
and rejects scan/iteration errors rather than publishing partial success.

Map-info and unified-overworld bounds use the same service and captured database.
Each query respects caller cancellation and a maximum five-second deadline.
The bounds query preserves negative coordinates and excludes erased tiles and
interior rows. The world handler supplies runtime map ID 9999 for the synthetic
bounds projection; it is not a physical catalog row. Terminal failures are logged
server-side and return the shared typed query error without SQL text.

`PhaserMapInfo` and `PhaserMapInfoRequest` now live in `internal/protocol` and
regenerate directly from explicit JSON tags. All shipped frontend consumers
import the DTO from generated `protocol.ts`; the old `world_api.ts` declarations
are removed rather than aliased. Successful map/list wire shapes retain existing
field names and explicit zero/negative coordinates. Map-info preserves optional
bounds/tileset omission. Lists now also honor those `omitempty` tags instead of
serializing absent pointers as `null` through reflection; frontend consumers
already declare these fields optional. Empty list output remains `[]`. Lists
bypass reflection/casing conversion. No opcode or asset
contract changed.

PostgreSQL tests prove empty/nonempty sorted lists, interior metadata,
NULL-map active bounds, cancellation, blocked-query deadlines and retry. Real
packet dispatch with the global database removed proves list success/error
responses do not claim presence; framed JSON tests cover actual key names and
omitted/zero/negative values. The destination transaction tests still pass after
injection. Content/world/server/protocol PostgreSQL race suites and all Go
package compilation pass. Eight focused frontend response/lifecycle tests,
TypeScript typecheck, runtime asset validation and the production build pass.
Canonical `npm run tygo` regeneration is byte-stable. All five isolated Instant
Warp/multiplayer rendered cases pass in
`/var/tmp/capturequest-rendered.P5TqpG` (44.8 seconds).

Map-list separation does not make the destination-bearing map-info
handler read-only: current-map recovery/load effects and destination writes
remain there. Error-response correlation and cleanup of map-query listeners on
failure/timeout/supersession also remain part of the networking migration.

Next: retire the destination-bearing metadata path through explicit gameplay
intent, coordinate server acceptance with client arrival/collision residency, and
finish the remaining cancellation, reconnect and domain/wire roadmap. The goal
remains active; no push or deployment.

## Client destination catalog boundary (2026-10-02)

The active client position writers now validate destination catalog membership
inside the existing bounded, character-locked position transaction. Interiors
require an existing `phaser_maps.id` and a non-erased `phaser_tiles` coordinate.
The synthetic `UnifiedOverworldMapID` is **9999**; its tiles have `map_id IS NULL`,
and negative coordinates are valid. Map ID 0 is not an overworld alias. A missing
or erased destination fails before position or Safari state changes. Trusted
runtime destinations retain their existing source-specific checks and share the
same persistence primitive.

`PhaserMapInfoRequest` rejects a partial `destX`/`destY` pair. A metadata-only
request for another map can return its description but cannot change session
presence or execute that map's load effects. Current-map metadata still runs its
existing recovery and load effects; fully separating reads from gameplay
commands remains required. Neither catalog membership nor a successful commit
proves movement eligibility. Ordinary movement, issued warp acknowledgements and
Instant Warp still share the client position contract. This checkpoint adds no
collision restriction or GM gate to ordinary-player Instant Warp.

PostgreSQL tests cover erased/missing tiles, unknown maps, the rejected ID-0
alias, valid negative overworld coordinates, Safari rollback/end, remote
metadata and partial destinations through packet dispatch, and unchanged live,
movement and saved state on rejection. Late-commit fixtures now include valid
catalog tiles so they continue reaching their injected commit failure. Existing
visibility assertions are retained with the catalog fields their fixture needs.
The final world/server PostgreSQL race suites pass, all Go packages compile,
and `git diff --check` passes.

### Far-overworld input diagnosis

The initial rendered run `/var/tmp/capturequest-rendered.F3D2cM` passed interior
keyboard/click movement, indoor overview-to-overworld Instant Warp and
multiplayer. Its far-overworld case arrived at `(190,-81)` but timed out waiting
for the next ArrowLeft position event. A focused repeat at
`/var/tmp/capturequest-rendered.d8pdl7` reproduced the failure without a server
destination rejection.

A private build of prior server checkpoint `7732412` reproduced the same failure
in `/var/tmp/capturequest-rendered.PG7WAf`, before client catalog validation.
Diagnostics against the current server in `/var/tmp/capturequest-rendered.QkfVQN`
showed **no exact tiles** in the one-tile radius around the destination when warp
mode ended. ArrowLeft was received: the actor turned RIGHT to LEFT while staying
at `(190,-81)`. The canonical source catalog gives both `(190,-81)` and
`(189,-81)` `collision_type=1`. The client had not installed that collision data
when the test pressed the key.

`ensureTileAvailable` fetches/verifies a catalog chunk; it does not install the
camera plan's renderer/collision residency. Same-map Instant Warp updates the
actor and restores camera follow immediately, then the camera stream installs
exact chunks asynchronously. The movement-origin test now waits for both actual
exact tiles to report collision type 1 before sending its native key press.
The original keyboard/click destination and stale-origin assertions are retained;
this is a data-readiness condition rather than an increased input delay or a
relaxed coordinate assertion. Temporary diagnostic logging was removed.

Verification: all four Instant Warp rendered cases pass in
`/var/tmp/capturequest-rendered.iBp95V` (28.7 seconds), including native keyboard
and click-path movement after the far warp. `npm run typecheck` and
`git diff --check` pass. Multiplayer passed in the earlier catalog-boundary run;
it was not rerun in this four-case check. No production code or assets changed
in this verification checkpoint.

Client input still appears unfrozen during that brief missing-residency window.
The test change does not repair or prove user-facing arrival readiness. The
explicit warp-flow migration must coordinate server acceptance, actor arrival,
exact collision residency and input release, including cancellation, failed tile
loads, superseded targets and reconnect. Metadata, movement reports, cutscene
visual reports, warp acknowledgements, blackout and Instant Warp currently share
position writers; preserve each producer's semantics during migration.

Next: separate metadata, movement reports and explicit warp intent at the shared
authoritative boundary, including state restrictions and server-issued destination
recovery. Continue all six remaining-work items above. The goal remains active;
this checkpoint is local only, with no push or deployment.

## Retired legacy map setter (2026-10-02)

The only caller of `PerformMapChange` was the registered legacy
`MapChangeRequest` opcode `176`. Repository-wide source search found no shipped
client sender or other server caller. That handler accepted `mapId`/`zoneId`,
`instanceId`, arbitrary floating coordinates, `z` and `heading`; it changed live
state before its general save and continued publishing on failure. It also
bypassed the active movement/Safari destination boundary.

The obsolete setter and its registration are removed. Central session admission
explicitly rejects opcode `176`, and the Go/generated TypeScript constants retain
that number with a reservation comment so it cannot be reassigned to another
operation. The shipped client's Phaser map and position contracts remain the
supported path. This retires a redundant mutation path rather than adding a
second authorization or persistence implementation for an unused client API.

Verification: the selected-character registry test sends both payload naming
forms and a same-map instance/coordinate mutation. Live character/session fields,
registered movement, PostgreSQL position and outbound messages all remain
unchanged. Focused position/teleport tests and the full PostgreSQL-backed
world/server race suites pass; all Go packages compile, TypeScript typecheck
passes, and canonical `npm run tygo` changes only the opcode reservation comment.
No assets, schema, normal gameplay presentation or deployed services changed.

The active paths still need systemic destination validation. In particular,
`PhaserMapInfoRequest` accepts `destX`/`destY`, persists them, and updates session
map tracking; `PhaserPlayerPositionUpdate` accepts `mapId`/`x`/`y` without proving
a move or destination grant. Their commit-before-publication ordering alone does
not prove eligibility. Current producers include ordinary visual-step reports,
map loading, server warp acknowledgements, blackout handling and Instant Warp.
The HUD currently presents Instant Warp to ordinary players without a GM gate;
that existing gameplay policy must be preserved or deliberately changed with
user direction, not silently reclassified as admin functionality.

Next: separate map metadata reads, ordinary movement reports and explicit
Instant Warp intent through the authoritative destination boundary. Validate
real catalog coordinates and state restrictions while preserving normal warps,
server-issued destinations and the current Instant Warp policy. Then continue
the full mutation, cancellation, reconnect and wire-contract roadmap. The goal
remains active; nothing is pushed or deployed.

## Integrated active-player shutdown verification (2026-10-02)

The dedicated isolated browser test now exercises rendered login, actual player
movement, PostgreSQL position persistence, signal shutdown, transport closure,
final playtime and the standalone server exit result together. It validates the
private cluster path and the exact running executable before signalling the
runner-owned PID. Fault injection targets only its freshly created character in
the disposable database. Ordinary test runs skip this process-control case;
use the dedicated runner modes:

```bash
CQ_E2E_SHUTDOWN_MODE=success bash scripts/testing/run-isolated-e2e.sh
CQ_E2E_SHUTDOWN_MODE=failure bash scripts/testing/run-isolated-e2e.sh
```

The runner collects the child process's real exit status and requires terminal
test evidence before waiting. It retains `shutdown-evidence.json`, `server-exit`,
server logs and the pre-shutdown screenshot, and cleans up only its owned
processes/private cluster. No normal development or production database is used.

Verified success: `/var/tmp/capturequest-rendered.8UzH8Z`, one Chromium case
passed in 6.0 seconds. Character `1` retained map `38`, position `(4,6)`; active
playtime increased from `0` to `2`; its WebSocket closed; server exit was `0`.
Verified deferred save failure: `/var/tmp/capturequest-rendered.wvvMXz`, one case
passed in 6.3 seconds. Position remained `38,4,6`, playtime remained `0`, the
socket closed, and `final playtime` propagated into `Shutdown failed` and exit
`1`. The failure run's screenshot was inspected: the player is visibly present
in Reds House 2F before shutdown. TypeScript typecheck, shell syntax and diff
whitespace checks passed. Both runners completed their cleanup.

This proves an idle active player's accepted movement and final playtime/error
handling over the shipped WebSocket path. It does not prove interrupted in-flight
gameplay over that transport, an active character over browser WebTransport,
concurrent-player shutdown, or recovery of a failed final save after process exit.
The failure result makes loss observable; it does not add durable recovery.
Those coverage/recovery requirements remain in the full goal.

Next: migrate the known legacy `PerformMapChange` publication/authorization path
to the authoritative position transaction, then complete remaining mutation and
reconnect/result recovery audits. Nothing is pushed or deployed.

## Owned transport shutdown checkpoint (2026-10-02)

The server now owns completion of the WebTransport listener and a sealed set of
transport upgrade handlers and reader tasks. WebSocket control readers and
WebTransport control acceptance, reliable reading, extra-stream detection and
datagram reading register before launch. Once draining starts, no new transport
work is admitted. Shutdown joins admitted readers and upgrade handlers before
closing storage, even if a callback is still returning from world work. The
listener uses the same failure reporting and explicit completion ownership as
HTTP. WebTransport closure begins alongside world retirement instead of waiting
for ordinary HTTP completion.

Initial real-transport tests proved that joining local readers did not close the
client session. Inspection of pinned `webtransport-go v0.8.0` and quic-go HTTP/3
showed that server close cancelled local session management and closed listeners;
accepted QUIC connections also needed an explicit owner. `ConnContext` now
registers those connections, removes terminated connections, and rejects late
connections during drain. Shutdown closes owned QUIC connections before releasing
UDP, interrupting blocked close-capsule/stream operations and notifying peers.
The client-closure assertions remain unchanged.

A later race run exposed `http3.datagrammer.receiveErr` being read after unlocking
while `SetReceiveError` wrote it. quic-go is upgraded from `v0.43.0` to `v0.44.0`:
source comparison verifies the receive error is copied under the mutex in that
version. This also includes the `ConnContext` regression fix documented in
[upstream v0.43.1](https://github.com/quic-go/quic-go/releases/tag/v0.43.1).
webtransport-go stays at `v0.8.0`; the existing stream/JSON contracts are unchanged.
No race detection or client closure assertion was weakened.

Verification: PostgreSQL-backed world/server race suites passed after the
upgrade; all Go packages compiled. Twenty repeated race runs of WebTransport and
owned-transport storage-ordering tests passed. Real QUIC tests cover no control
stream, idle reliable reading, partial frames, active datagrams, extra-stream
rejection, client closure, listener completion, sealed reader admission and
unexpected listener failure reporting. A blocked owned task proves caller
expiry leaves storage open until a later join. These are Go transport and
headless persistence checks; browser rendering and simultaneous real-player
persistence over transport are not proven by this checkpoint.

Next: verify integrated active-player shutdown over the real transport with
accepted gameplay and final-save failure, then finish cancellation of remaining
legacy callbacks and durable reconnect/result recovery. The full five-part goal
and remaining roadmap stay active. Nothing is pushed or deployed.

## Shutdown cancellation and HTTP retirement checkpoint (2026-10-02)

Server shutdown previously waited for ordinary HTTP work before closing player
transports. A blocked HTTP handler could therefore keep player connections and
running commands alive throughout the wait. World retirement now starts before
HTTP joining; its cleanup registration precedes session closure, retaining final
save failures from racing transport callbacks. HTTP requests inherit a server
context cancelled at drain start. Gameplay cleanup retains its separate save
context, and storage joins both world and HTTP completion before closing.

Context cancellation alone does not interrupt a blocked socket/body read. The
HTTP grace-period deadline now force-closes ordinary connections and returns the
graceful-drain error. Admitted handlers are tracked through completion. A sealed
handler-admission boundary prevents new work from being added while shutdown
joins that set. Force-close does not imply handler completion: an uncooperative
handler still keeps storage open and causes the caller to report an unfinished
drain. Hijacked player transports remain owned by session/world retirement.

The optional Discord bridge also had lifecycle gaps: close-before-start waited
forever, repeated start could create multiple workers, and delivery/retry waits
ignored shutdown. It now owns one cancellable worker, joins safely before or
after start, and cancels active HTTP delivery and retry timers on close. Shutdown
cancels it before waiting for ordinary HTTP completion. Queued chat messages are
best effort and are discarded on shutdown; no new delivery is started after the
worker observes cancellation.

Verification: race suites passed for world, server and Discord bridge. Current
focused PostgreSQL-backed shutdown checks also passed. Tests cover HTTP request
cancellation, force-close of a partial body read with storage held open beneath
the returning handler, real WebSocket retirement during blocked HTTP shutdown,
close-before-start and repeated start, active local delivery cancellation, and
retry-delay cancellation without another attempt. All Go packages compile.
Tests use local endpoints and synthetic delivery responses; no external messages
were sent. No rendered client behavior changed or production deployment occurred.

Remaining: cancellation of legacy gameplay/database callbacks, joined and
bounded WebTransport shutdown, integrated active-player/slow-client transport
coverage, and durable recovery when final saves fail. An HTTP force-close and a
bounded caller wait do not prove those requirements. Next, trace WebTransport
reader/listener ownership and exercise shutdown with active transport work.

## Shutdown wait and result checkpoint (2026-10-02)

World and server shutdown now expose context-aware result APIs. Each starts one
owned drain; repeated callers join the same completion and receive its final
result. A caller deadline returns an explicit unfinished-drain error. HTTP
handlers, periodic callbacks, commands and cleanup retain their ownership until
they finish; storage closes only after HTTP and world work have joined. A timeout
does not close storage beneath unfinished work. Final session persistence errors
from shutdown cleanup propagate through world and server results instead of only
being logged. Joined cleanup failures still use the documented best-effort
retirement policy below; no recovery queue has been added.

The process uses `gracePeriod` as a shutdown wait in seconds (nonpositive means
30 seconds). The previously unused field was passed as a raw Go duration, which
would have interpreted `5` as five nanoseconds. Signal/listener shutdown now
reports a nonzero process exit for deadline or persistence failure. In an embedded
server the drain continues after the caller times out; exiting the standalone
process terminates remaining work. This is an explicit failure outcome, not a
successful persistence guarantee.

Verification: race tests exercise active character commands, a disconnect
already claimed by another callback, blocked HTTP handlers, later rejoin,
exactly-once drain and storage ordering. Deferred PostgreSQL trigger failure
proves final playtime errors reach repeated world callers; a server boundary test
proves joined world errors propagate after storage closes. All Go packages
compile. These tests do not prove bounded completion of the underlying work:
legacy HTTP/database/manager operations and transport closure still need caller
cancellation or explicit force-close policies. No rendered behavior changed.

Next: cancel/retire those remaining operations within the drain budget and test
real transport shutdown, while preserving storage ordering and defining durable
recovery for failed final saves. The full roadmap remains active; nothing is
pushed or deployed.

## Lifecycle persistence checkpoint (2026-10-02)

Periodic playtime increments and general character saves previously used the
global database without caller cancellation. Cleanup ignored its final position
save error and could wait indefinitely in the playtime write. These saves now
receive the world's captured database and the caller's context, require exactly
one affected character row, and invalidate character-select cache only after a
successful commit.
The unused global `UpdateCharacterPosition` writer is retired; runtime position
transactions remain the authoritative position boundary.

A PostgreSQL test exposed an additional cancellation failure: an autocommit
playtime update returned an error while blocked on a row lock, but committed
after the lock was released. Retrying counted a three-second interval twice.
Both general character saves and playtime increments now own the existing
bounded transaction wrapper. The cancellation test verifies a blocked write
rolls back, retains the interval for retry, and two retries save it once through
the captured database even when the global database is absent. This does not
prove recovery from an ambiguous result lost during commit; durable command/result
recovery remains required.

Persistence cleanup now shares a five-second context across final position and
playtime saves and returns stage-labelled errors. Disconnect starts cleanup
after draining the command owner with a separate context, so closing the
connection does not cancel its final saves. Character handoff carries its caller
budget into cleanup and propagates persistence failure instead of admitting the
replacement as if cleanup succeeded. Periodic playtime saves carry the active
command context.

The current retirement policy remains best effort: failed final saves are
reported, the old movement/client state is retired, and a later login reloads
the last durable state. Unsaved position/playtime from that retired session is
not retained in a recovery queue. A failed handoff rejects that attempt; it does
not permanently bar a fresh retry. Tests inject late failures into both saves
and verify aggregate errors, rejected replacement, and removal of the retired
movement writer. This policy and its limits are explicit; the full goal's durable
recovery requirements are not complete.

World/HTTP shutdown and command/timer joins still have unbounded waits.
The persistence context does not bound waiting on unrelated mutexes or transport
closure. Shutdown currently logs cleanup failures; it does not return an
aggregate failure to the server caller. Next: implement a context-aware shutdown
result and drain policy that joins owned work or reports a deadline while keeping
storage open beneath unfinished work, then verify active-player shutdown.
The full remaining roadmap above stays active. No push or deployment is included.
The opcode audit also found that legacy `PerformMapChange` mutates live map and
coordinates before its general save succeeds. Its failure ordering and client
destination authorization still need migration to the shared position boundary;
this checkpoint does not claim every position writer publishes after commit.

Verification: PostgreSQL-backed race tests passed for world, character
persistence, sessions and server; the character suite also passed after adding
missing-row assertions. All Go packages compiled, TypeScript typecheck passed,
and canonical protocol regeneration produced no contract changes. The rendered
multiplayer visibility case passed in 15.1 seconds using an isolated database
and local services (evidence: `/var/tmp/capturequest-rendered.vnCLdc`). Its
original return-step setup clicked the door tile already occupied by the player
and timed out. The test now uses the existing shared warp helper to move onto
adjacent floor and enter the stair, and asserts the exact return map. Visibility
assertions remain intact. This is local verification, not production evidence.

## Effect and script cancellation checkpoint (2026-10-02)

Safari, Repel and script/battle helpers previously started independent background
contexts after their callers had entered a bounded session command. A blocked
character write could therefore continue waiting after disconnect or the
original request deadline.

The shared Safari read/setup/entry/turn/step/exit APIs and Repel activation,
status/setup/step APIs now require an execution context. Runtime callers pass
`Session.CommandContext()`; fixtures and the standalone simulator explicitly
choose their setup context. Script action/completion transactions receive a
context separately from interpreter dependencies. Nested script operations
continue to use their outer transaction's query handle and commit boundary.
Ordinary battle start/resume and scripted battle creation also use caller
contexts. Scripted trainer/wild battle helpers now require an explicit database;
the redundant global-database trainer wrapper is retired.

The unused standalone Safari map-exit helper is removed. Actual destination
writes already end Safari within the position transaction, so maintaining a
second independent cleanup transaction was obsolete. Tests now exercise this
authoritative boundary and verify gate/zone retention and non-Safari cleanup
together with the saved destination.

PostgreSQL cancellation tests hold the character row lock and apply a 100 ms
caller deadline to Safari entry and turns, Repel activation, script reward
completion and scripted wild battle start. Each operation returns before the
independent five-second transaction budget, with unchanged wallet, inventory,
flags, battle state and visit counters; cancelled starts publish no battle cache.
Existing failure/retry and lifecycle tests pass in race-enabled world,
script-simulator, battle, session and server suites. All Go packages compile;
TypeScript typecheck and canonical Tygo regeneration pass without contract
changes.
All seven isolated rendered field-move/Safari checks pass (42.6 seconds),
covering Surf input, Cut interaction, Safari entry, battle run and step
exhaustion. Evidence is retained at `/var/tmp/capturequest-rendered.ytn0RK`.

The full goal remains active. Legacy party/encounter/catalog reads, post-commit
cache refresh, flag writes, blackout and lifecycle/playtime helpers still need
cancellation/dependency work. This checkpoint does not prove cancellation through
every handler or bounded shutdown. Final-flush failure policy, HTTP/world drain
deadlines, database-close ordering, durable replay/reconnect results, remaining
domain/wire migrations and integrated transport verification remain in the
roadmap above. Next: move lifecycle persistence and remaining running queries to
explicit contexts, then implement and test the drain/failure policy. No push or
deployment is included.

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
