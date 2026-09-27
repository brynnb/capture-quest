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
