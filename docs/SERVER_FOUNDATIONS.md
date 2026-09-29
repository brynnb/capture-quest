# Server foundations goal

Status: active. Started 2026-09-25 from `02c51ba`.

Keep Go, PostgreSQL, one deployable server, and the authoritative extractor,
runtime asset, and scripted-action contracts. Improve runtime safety through
small verified changes. No production deployment or push is part of this goal.

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
