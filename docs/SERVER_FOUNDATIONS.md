# Server foundations goal

Status: **active; broader roadmap incomplete**. Originally started 2026-09-25
from `02c51ba`. On 2026-10-03 the user authorized a new tracked goal to continue
the full roadmap from local implementation checkpoint `3a6d68d`. The goal tool
confirmed it is active. Earlier paused states and bounded stopping rules below
describe historical checkpoints; they do not limit this renewed authorization.

Working branch: `codex/server-foundations`. Latest implementation checkpoint:
conditional dialogue rejects malformed conditions and load failures,
following `e68fce5`: name validation uses owned filtering, explicit identity and cancellable application,
following `58b871e`: real local Chromium native QUIC login/movement/reentry and restart acceptance,
following `91c6015`: legacy FIFO timeout/send failure retires ambiguous transport and settles its caller,
following `aa0e219`: native transport setup/readers/writes are fenced to their captured owner,
following `9e872ad`: WebSocket setup uses the existing deadline and FIFO requests retire with transport,
following `849b797`: WebSocket attempts settle on retirement and callbacks use the owned instance,
following `208c5a2`: owned reconnect timers and rendered informational process-replacement acceptance,
following `82c87cc`: informational read boundary review rejects incomplete/wrong-kind data and
suppresses same-turn abandoned dispatch, following `6600612`:
Pokédex/trainer client reads reuse shared correlation and transport retirement,
following `eb5baf4`: Pokédex/trainer responses share bounded repair and coherent read snapshots,
following `3ddac87`: entry persists only last-login metadata and recovery preserves its full pose
through the existing destination transaction, following `5939513`:
camp no longer replays cached positions; trainer cards reuse the owned wallet
reader and its established empty-wallet policy, following `267bd9d`:
retirement of the deferred position saver after its movement-manager producer audit,
following `6367929` (committed teleport/headless staging retirement),
`c5c8f5f` (sealed shutdown reconciliation) and `c58118e`:
failed-cleanup recovery behind the existing character admission barrier, with
idempotent cumulative playtime saves, following `a343ac5` (clean position rewrite
retirement), shared character-lock enforcement and native source consolidation.

The latest field-command prerequisite is Escape Rope source fencing, recorded
below; Bicycle now has a movement-owned desired-state command. Escape Rope transport now uses a correlated revision-fenced command and movement-owned current recovery.
The preceding PC migration and
source-authorized PC commands and Indigo failure/restart acceptance (2026-10-07);
PC permission/source retirement review is recorded below. This follows
stable row-ID storage primitives and coherent PC recovery, and center healing through the shared scripted-event
boundary and its rendered/recovery acceptance plus shared issuance/visibility
fixes (`4d18056`, `58b3d53`), following `0d6b640`
(party ordering through the shared inventory command boundary) and `cf0aafb`
(transaction diagnostics and remaining-family inventory), `3a6d68d` (Repel),
`a96eaa7` (commit/presentation review fixes), `5c8da47` (shop/party consolidation), the
owned-item dispatch checkpoint and committed shop crash/restart acceptance. The shop runtime
implementation is `f118916` (per-command clerk authorization and source sale
policy), following `d638d8b` (source-authorized opening), `5ac9f66` (injected menu
reads) and `21fd084` (durable shop revisions and correlated recovery).
Earlier checkpoints and their verification limits are recorded below and in this
branch's Git history. The goal alone does not authorize push or deployment. The user separately authorized branch pushes for stopping checkpoints on 2026-10-07 and 2026-10-08; production deployment remains unauthorized.

## Dialogue migration prerequisite: explicit condition failure (2026-10-08)

The active dialogue audit found its resolver still uses global data dependencies,
and its conditional helper hid query/scan failures as no override. More seriously,
malformed `requires_flags`/`requires_flags_absent` JSON became an empty list, which
could enable a branch without its intended conditions.

The shared condition decoder now returns errors. Conditional query, scan,
iteration and JSON failures propagate through the resolver instead of selecting
a default or unconstrained override. Error messages identify text constant, row
ID and condition field. Nullable/empty lists and existing scalar/multi-flag
selection semantics remain supported. This is a prerequisite fix, not a claim
that the dialogue transport has migrated.

Focused checks cover malformed conditions at the primitive and real resolver,
missing conditional tables, and existing generated/scalar/multi-flag branches.
They passed in 1.1s; full world (61.0s), simulator and session race suites passed.
No rendered dialogue acceptance is inferred from these state/parser checks.

Next: finish injected, cancellable dialogue/branch/trade readers and shared
snapshot ownership, then client correlation and actor/cutscene lifetime. Those
global dependencies and other fallback/error paths remain explicitly open in
the command matrix. Name validation's preceding boundary has been inspected;
no new change was made to it. All five roadmap areas remain active; historical
restore timeout and Repel click remain unattributed. No wire/schema/assets,
push or deployment.

## Name validation identity and authoritative lookup (2026-10-08)

The FIFO corpus audit found `questApi` request functions with no callers and a
legacy `DialogueStore` used by nothing else. Both are retired; Pokémon dialogue
remains its authoritative store/runtime, and opcode numbers remain reserved.
Active FIFO consumers are account login/create/entry and Phaser dialogue reads.
Name validation was selected as an active migration, not retired as unused.

`HandleValidateNameRequest` previously treated any character lookup error as
"available". Its shared name filter also used the global database and returned
success on query/scan failure. The shared validator now accepts the injected
database/caller context, has a five-second bound, preserves Unicode format and
trim/lowercase substring policy, and rejects query/scan/iteration failure.
Creation and validation use that one primitive. The unscoped disallowed-word
cache is retired so another database/configuration cannot supply this policy.
Creation returns its existing rejection reply if filtering cannot be confirmed.

Availability uses one owned existence query matching the canonical UNIQUE(name),
including soft-deleted rows. Errors are terminal failures, not free names.
The check remains advisory; concurrent creation is still decided by the database
constraint. Generated success/error DTOs echo request identity, and successful
checks echo the actual name. Invalid-format/taken names are business results,
distinct from transport/SQL failure. The temporary legacy empty-ID lane remains
until coordinated client/server activation can retire it.

The client uses existing CorrelatedRequest settlement and socket retirement;
name editing/unmount aborts the request and prevents stale state application.
It no longer uses opcode FIFO or the old `valid || success` fallback. Concurrent
names can settle out of order without association errors. The rendered test
holds an earlier available-name response while the newer name is reserved by a
soft-deleted row, then proves delivery cannot replace the "taken" result.

Focused PostgreSQL checks cover injected reads, active/deleted uniqueness,
filter/lookup SQL failure, creation rejection and actual pool cancellation.
Client checks cover out-of-order correlation, edit/unmount and transport
retirement. The delayed-name browser run passed in 2.6s at
`/var/tmp/capturequest-rendered.TzHjZc`. Full native creation/movement/reentry and
real restart acceptance also passed in 17.3s at
`/var/tmp/capturequest-rendered.xYDpBN`.
Focused PostgreSQL checks passed (1.3s); full world (57.7s), protocol, session and
server race suites passed. Eighteen focused client/shared-boundary checks,
frontend typecheck, all Go package compilation, production build, runtime asset
validation and diff checks passed. Canonical wire types were regenerated;
existing build warnings remain. No production endpoint was tested.

Next: shared-boundary review and active Phaser dialogue read migration, then
remaining account command identity/recovery and the finite wider matrix. The
historical restore timeout and Repel click remain unattributed, and all five
roadmap areas remain open. No schema/assets changed, push or deployment.

## Native Chromium/QUIC acceptance (2026-10-08)

The existing isolated browser runner now supports opt-in
`CQ_E2E_TRANSPORT=native`; default WebSocket behavior is unchanged, and unsupported
mode values reject before launching services. Native mode disables forced
WebSocket selection without disabling the application's fallback. The test must
observe `{connected:true,websocket:false,native:true}`, so successful fallback
cannot be misreported as native acceptance.

Two real local Chromium checks passed in 17.2s using matched assets and private
PostgreSQL. Native login/character creation, movement from tile `(3,6)` to `(4,6)`,
trainer-card loading, quit and saved-position reentry passed. The second check
killed the exact runner-owned server and reentered from the original browser;
both before and after snapshots retained native mode. PID `2731681` exited `137`,
replacement `2732143` served restart generation 1. The recovered card screenshot
was inspected. Logs, screenshots and transport/restart receipt are retained at
`/var/tmp/capturequest-rendered.Kv6UFp`.

Run with:

```bash
CQ_E2E_TRANSPORT=native CQ_E2E_CRASH_RECOVERY=true \
  bash scripts/testing/run-isolated-e2e.sh tests/e2e/native-transport.spec.ts
```

Typechecking, runner shell syntax and diff checks passed. This checkpoint adds
test capability/evidence only; production application behavior, schemas and
assets are unchanged, so a duplicate production build or Go suite was not run.
It proves this local Chromium/QUIC path, not other browsers, production networking
or every native failure/timeout variant.

Next: audit remaining FIFO callers and migrate genuinely active command/read
paths through existing identity primitives. Continue the finite command-family,
domain/wire and lifecycle inventory; historical restore timeout and Repel click
remain unattributed. All five roadmap areas remain active. No push or deployment.

## FIFO send failure and missing-response boundary (2026-10-08)

Review reproduced two defects: FIFO timeout left its connection available for a
new same-opcode request, and synchronous send failure left the already-created
slot and timer behind. A delayed untagged reply could then resolve a newer slot.
These regressions failed before the fix.

The legacy API now treats timeout or write failure as an ambiguous association
boundary and retires its captured connection through the existing close path.
The triggering request keeps its timeout/write error; other pending requests
settle as retired, and timers/slots are removed. No mutation is automatically
retried. The existing reconnect/login/reentry flow recovers current durable state.
This deliberately changes legacy timeout recovery from continued use of an
ambiguous stream to fail-closed retirement. Tagged read coordinators retain their
own timeout/recovery policy and do not close their stream merely on read timeout.

Native request settlement no longer waits behind a backpressured write: response
timeout and retirement can reject while that write is still pending. Write
completion/failure targets only its captured, still-pending slot. A late failure
cannot close a replacement or invalidate an already confirmed reply. Controlled
stream tests cover blocked writes and delayed failures after replacement.

The actual WebSocket pending-view SIGKILL/reentry case passed in 6.7s at
`/var/tmp/capturequest-rendered.2TNC9a`; owned PID `2721226` exited `137` and
replacement `2721555` served generation 1. Sixty-six related client tests,
typecheck, production build, runtime asset validation and diff checks passed.
Existing build warnings remain. Native stream tests are not QUIC acceptance.

Next: real native QUIC/browser acceptance and migration of remaining FIFO callers
(`authService`, `questApi`, `DialogueService`) to explicit identities where still
used. Normal-operation unsolicited/out-of-order response association is not
solved by failing closed on missing replies. The original restore timeout and
Repel click remain unattributed. All five roadmap areas remain open. No Go/wire/
schema change, push, deployment or production acceptance.

## Native transport continuation and stream ownership (2026-10-08)

The native path waited for an old transport to close itself before replacing it,
used the mutable `webtransport` field across asynchronous setup, and let old
closed/read callbacks operate on current state. Datagram queue callbacks also
looked up the current writer at execution time, allowing an old queued payload
to target a replacement writer.

Setup now has one cancellable native attempt and captures its transport. The
existing timeout helper also accepts retirement cancellation; hash fetch keeps
its five-second limit and handshake/control setup retain eight-second limits.
Superseded continuations settle without creating a transport or falling back
over the replacement. Retirement closes the old transport directly, cancels
readers, releases writers, clears physical-transport FIFO requests and resets
the write queue. Control streams arriving after retirement are disposed. Old
closed/read callbacks check instance ownership, including after synchronous
dispatch. Queued datagrams capture writer/transport and reject if that owner
retired, while later demand has an independent healthy queue. Pending native
setup also blocks competing reconnect scheduling.

Seven controlled-transport tests use real JavaScript streams and cover handshake
replacement, old closure, queued control/datagram bytes after retirement, reader
lock release, old queued writes, hash-fetch cancellation, late control streams
and pending-setup retry exclusion. They do not prove real QUIC or certificate
negotiation. Sixty-one related client checks passed before the final scheduler
guard; the final native/WebSocket focused checks passed all 16 cases afterward.

The shared cleanup's real WebSocket pending-view SIGKILL/reentry check still
passed in 7.5s, with evidence at `/var/tmp/capturequest-rendered.ksEHbb`. That is
WebSocket rendered evidence, not native WebTransport acceptance. No Go, wire,
schema or generated asset change is included.
Final frontend typechecking, production build, runtime asset validation and diff
checks passed. Existing build warnings remain; no production endpoint was tested.

Next: real native browser/QUIC acceptance and shared send-failure/FIFO association
review before another family migration. Same-connection late untagged replies
remain open. The original restore timeout and Repel click remain unattributed;
all five roadmap areas remain active. No push or deployment.

## WebSocket setup deadline and FIFO transport retirement (2026-10-08)

Two regressions failed before this checkpoint: an old FIFO request remained
pending after close, and an unopened WebSocket remained unresolved after the
existing transport setup limit. Pending FIFO queues now clear and reject at
physical transport retirement, with their timers removed. The replacement's
response settles only its fresh request. Read-generation changes for
authentication on the same physical connection do not reorder its FIFO queue.

WebSocket setup now uses the existing `TRANSPORT_CONNECT_TIMEOUT_MS=8000` limit.
Expiry settles failure, retires the owning socket through the existing close
path, and cannot be revived by its captured late-open callback. The setup timer
is cleared on success, cancellation and failure; automatic reconnect remains
governed by the existing timer/attempt ownership. The pending-manual regression
requires exactly its one setup deadline, with no stale retry timer.

Nine socket-lifetime checks and 55 related network/read/preference/character
tests passed. Typechecking, production build, runtime asset validation and diff
checks passed; existing build warnings remain. The actual pending-view SIGKILL
and reentry check passed again in 6.9s at
`/var/tmp/capturequest-rendered.PMRM8E`: owned PID `2693179` exited `137`,
replacement `2693506` served generation 1. No Go or wire change required a broad
Go rerun.

Next: finish native WebTransport continuation and read-loop ownership. FIFO
consumers still include `authService`, `questApi` and `DialogueService`; late
untagged replies after timeout on the same connection remain a protocol audit,
not a solved association guarantee. The historical restore timeout and Repel
click remain unattributed. All five roadmap areas remain open. No push,
deployment or production acceptance.

## WebSocket attempt and callback ownership (2026-10-08)

The pending-attempt regression failed before the fix: replacing an unopened
WebSocket left the older `connect()` promise unresolved. Inspection also found
that late open/message/close/error callbacks consulted global state without
checking which socket owned it.

The existing `ws` field now owns its instance from construction. One retirement
method closes that instance, clears its handlers/buffer/heartbeat and settles its
pending connection promise as cancelled. All WebSocket callbacks check instance
identity before affecting state or delivering a message. A superseded automatic
attempt also checks its attempt generation before scheduling another retry;
queued retry scheduling does not compete with a pending manual attempt. No
second transport registry or connection framework is introduced.

Checks prove replacement settles the old attempt, captured stale callbacks cannot
open/send/close the replacement, explicit close during setup prevents resurrection,
and retired automatic work cannot enqueue retries over pending manual connection.
The real pending-view SIGKILL/reentry check passed again in 6.4s: owned server
PID `2684699` exited `137`, replacement `2685015` served generation 1, and the
browser recovered its fresh card. Evidence is retained at
`/var/tmp/capturequest-rendered.ucEKcg`. This is WebSocket browser acceptance.
Seven socket-lifetime tests and 53 related network/read/preference/character
tests pass. Frontend typechecking, production build, runtime asset validation
and diff checks passed; existing build warnings remain. No unrelated Go suite
was rerun for this client-only change.

Next: review native WebTransport fetch/handshake/control-stream continuations and
stale close/read callbacks, setup deadline behavior and legacy FIFO request
retirement. Those guarantees are not established by this WebSocket checkpoint.
Continue the finite command/acquisition inventory afterward. Historical restore
timeout and Repel-click attribution remain open, as do all five roadmap areas.
No Go/wire/schema/assets changed. No push, deployment or production acceptance.

## Pending informational read across server replacement (2026-10-08)

The real SIGKILL check held a completed card response while its client request
remained pending. Retirement cleared card/status and invalidated the species
catalog, but the first reentry failed: the restarted server logged authenticated
session 1, then another socket/session 2, and session 1 later timed out without
heartbeats. A socket regression reproduced manual connection succeeding followed
by a queued reconnect creating a second WebSocket one second later.

The existing socket class now owns its reconnect timer. Manual connect and close
cancel queued retries; duplicate schedules coalesce, and a queued callback cannot
replace an established connection. This fixes the verified queued-retry case;
already-running connection attempts, stale close/open callbacks and native
WebTransport replacement remain a separate ownership audit, not a claimed fix.

Rendered acceptance then passed in 6.4s. Runner-owned PID `2674558` exited `137`,
replacement PID `2674894` served restart generation 1, and the original browser
returned through guest login/reentry without a page reload. Private informational
state was `{card:null,statusCount:0,catalogLoaded:false}` after disconnect. The
replacement card showed ¥400; its screenshot was inspected. The dead transport's
captured historical envelope was replayed explicitly at the current dispatcher
and did not rewind the card. This is an application-fence check, not a claim that
a dead connection delivered bytes. Receipt, logs and protocol/state evidence are
in `/var/tmp/capturequest-rendered.qYqUNm`; the replacement log shows one new
session for the successful entry, with no observed ghost session or heartbeat
timeout in this run.
Forty-nine related network/read/preference/character tests passed; the final four
socket-lifetime tests also pass, including explicit-close cancellation and
duplicate schedule coalescing. Frontend typecheck, production build, runtime
asset validation and diff checks passed. Existing build warnings remain. No
unrelated Go suite was rerun for this client-only change.

Next: review in-flight transport attempts and callback ownership before another
family migration, then continue the finite command matrix and acquisition-writer
audits. The historical restore timeout and Repel click are still unattributed.
All five roadmap areas remain open. No Go/wire/schema/assets changed; no push,
deployment or production acceptance.

## Informational boundary review and timeout retry (2026-10-08)

Review reproduced two application defects: a success packet containing only
identity and `badges: []` became a card snapshot despite missing presentation
fields, and a status packet carrying list fields could replace the species
catalog. Both regressions failed before the fix. The shared adapter now validates
the requested response kind and required card/status/catalog fields before the
single store publication. Missing data is rejected; no defaults are fabricated.

Rendered retry investigation also observed two different request IDs at the same
timestamp on every card mount. `src/main.tsx` uses React StrictMode, whose cleanup
and remount abandoned the first demand after immediate dispatch. The adapter now
yields one microtask before dispatch and checks its existing current-owner guard.
Cleanup or supersession in that turn sends no abandoned request. The shared
correlation/timeout implementation is unchanged. Unit checks separately retain
coverage of older requests that were already sent.

The browser fault test holds all card responses during the timeout phase, because
holding only the first response lets a valid superseding refresh complete. It
requires the real timeout message, then one fresh read on reopen with a distinct
identity, current money and no rewind after browser receipt of every held reply.
Both informational browser cases reuse one observation helper at the actual
socket JSON boundary; its production dispatcher is preserved. Retry cardinality
is asserted rather than relaxed to accept duplicate abandoned sends.
Both rendered checks passed in 17.5s (`/var/tmp/capturequest-rendered.jYTF1Q`).
The timeout case verifies the visible retry message, exactly one fresh request
on reopen, distinct request identity, ¥300 current money and no rewind after
delivery of all held responses. The reentry/card and old-list/new-status case
still passes with the strengthened validators and deferred dispatch.
Twenty-two focused client/store/shared-boundary tests, frontend typecheck,
production build, runtime asset validation and diff checks passed. This checkpoint
changes no Go runtime or wire contract; its preceding PostgreSQL/Go evidence is
retained without an unnecessary broad rerun. Existing build warnings remain.

Remaining: rendered disconnect/process-replacement cases, broader acquisition
writer and prerequisite audits, and the rest of the finite command matrix. The
original restore timeout and Repel click remain unattributed. The legacy empty-ID
server lane still needs retirement after coordinated release verification. All
five roadmap areas remain active. No push or deployment.

## Correlated informational views and owned application (2026-10-08)

Three UI callers sent untagged reads, and `NetworkBridge` applied replies through
global asynchronous store callbacks. A delayed old-character or old-view reply
could therefore overwrite a later view. Cutscene badge completion also pushed a
full unrequested trainer card.

The generated family now shares request/character identity on successes and
errors. One client adapter uses existing `CorrelatedRequest` settlement, timeout
and subscription cleanup and the socket's existing retirement generation. Card
and Pokédex channels supersede older reads; application checks the current
character, screen, transport generation, request identity and response kind.
Component cleanup and hidden-view transitions cancel pending reads. Quit/character
change clears private card/status data; transport replacement also invalidates
species catalog data. Full list publication is one atomic store update.

The global bridge application callbacks and component-specific raw sends are
retired. Cutscene completion uses the existing character-scoped resource notice;
visible informational views request current snapshots instead of receiving a
historical unsolicited card. The existing inventory reconciliation consumer of
that notice remains. Unused independent card/species/status store setters are
retired; owned replies use the single aggregate publication method. No inventory
command policy is imposed on these reads.

Legacy empty request IDs remain supported on the server for the coordinated
transition; new clients never apply uncorrelated responses. Retire that request
lane after coordinated frontend/backend activation and stale-client verification.
The canonical type generator was run; the script-action/data contract is unchanged.

Client checks cover tagged success, ignored untagged/duplicate replies, superseded
reads, timeout/retry, send failure, unmount cancellation, wrong character, quit and
same-character reentry, resolved-response transport retirement, resource notices,
malformed response kind and atomic list publication. Go wire checks require exact
identity-bearing error fields and preserve unrelated strict error-shape checks.

Rendered acceptance holds a real trainer-card reply across quit/same-character
reentry, delivers it at the actual browser socket boundary and verifies the
current card stays at ¥200. It then refreshes Pokédex status from Seen 6/Caught 6
to Seen 7/Caught 6 and replays the previous full-list reply; the newer counters
remain. The final run passed in 4.3s with inspected screenshots and evidence at
`/var/tmp/capturequest-rendered.Bof1ah`. Native frame events are unavailable for
routed sockets, so the test observes the real JSON dispatch boundary while
preserving its production dispatcher. No response or assertion is suppressed.
Nineteen focused client/bridge/store checks passed, as did frontend typechecking,
the final production build and runtime asset validation. Focused PostgreSQL wire
checks passed (1.7s), then full world (52.0s), protocol, session and server race
suites passed. All Go packages compile, canonical generated types are current
and diff checks pass. The build retains static/dynamic import and chunk-size
warnings; no production endpoint was exercised.

Remaining: rendered timeout/disconnect/process-replacement variants and wider
acquisition/source writer audits, plus the rest of the finite command matrix.
The original restore timeout and Repel click remain unattributed. All five roadmap
areas remain active. No push, deployment or production acceptance. Activation of
the new client read protocol requires the matching backend.

## Owned Pokédex repair and response snapshots (2026-10-08)

All three informational handlers previously called the owned-species repair
without cancellation. Trainer card also combined independent wallet/count queries
with badge flags from the cache. The held-pool status regression failed before
the fix because the request ignored its caller deadline.

One domain adapter now shares a five-second budget across the existing bounded
character transaction for monotonic repair and the existing read-only,
repeatable-read aggregate for the response. Repair uses the shared character lock;
late commit failure rejects the request. Species/status row loaders are shared,
close their rows before subsequent queries, and reject scan/iteration failure.
Trainer wallet, durable badge flags and Pokédex counts use the same snapshot and
their existing wallet/flag primitives. Session name and current playtime remain
the selected owner's presentation values. Complete owned statuses no longer
receive redundant row updates. Successful routine read logs are retired.

Repair is intentionally a separate monotonic maintenance commit before reading;
it may remain committed if a later response read fails. The response never
publishes a partial aggregate. Character-select species access and empty-array
wire semantics remain unchanged. This does not put reads in the inventory
command coordinator or create a parallel persistence system.

PostgreSQL checks cover pool cancellation with no repair, deferred repair rollback,
complete-status reads under a rejecting update trigger, existing wire/error/empty
wallet contracts, and a real publication barrier. While the card waits on a flag
table lock, another transaction publishes wallet `100 -> 200`, a badge and a
second caught species. The held response stays entirely old (`100`, zero badges,
one caught), then the next request sees the complete new state (`200`, one badge,
two caught), including durable flags absent from the cache.
Focused Pokédex/trainer checks passed (1.7s); full world (56.4s), database
repositories, battle, session and server race suites passed. All Go packages
compile and diff checks pass. No new rendered check was run for this backend
snapshot checkpoint; the preceding new-character/playtime browser evidence is
retained only as its existing baseline.
Final review preserved the private handlers' zero-character rejection while
retaining public species access at character select. The added zero-ID regression
and final focused family checks passed; no invalid identity becomes a successful
empty private response through the public-catalog branch.

Next: complete client correlation/cancellation and stale-response/reentry handling
for this read family before claiming its full lifetime audit closed. Wider
acquisition writers, informational handler prerequisites and the finite command
matrix remain open. No wire/schema/assets changed, and no new rendered or
production acceptance is inferred. The historical restore timeout and Repel click
remain unattributed. All five roadmap areas remain active. No push or deployment.

## Entry metadata intent and recovery pose (2026-10-08)

After camp retirement, the sole `UpdateCharacter` caller was entry's last-login
update. Its SQL still rewrote `map_id`, `x`, `y`, `z` and `heading` from the loaded
snapshot. The general save API is now retired. `SetCharacterLastLogin` accepts
only identity, authenticated account and timestamp; it updates only `last_login`
for a non-deleted character, through the existing bounded transaction and cache
invalidation boundary. Entry passes its existing shared five-second context.

Recovery inspection found that the shared destination primitive already resets
`z=0` and `heading=0`. The old general save restored the snapshot heading afterward.
The first new recovery test correctly failed with `heading=0` instead of saved
`heading=9`; its assertion was preserved. The destination primitive now accepts
an explicit heading through one shared implementation. Ordinary destinations
retain zero heading; invalid-position entry supplies its post-drain saved heading.
Recovery pose, route retirement and existing destination effects share one commit,
with no later metadata write replaying those fields.

The old SQLite general-save fixture is replaced by canonical PostgreSQL checks.
They cover unchanged pose/playtime across repeated login updates, account/deletion
guards, missing database/character rejection, late commit rollback, actual pool
deadline and retry. Recovery checks inject a deferred elevation rejection and
verify full pose/playtime and route preservation on rollback, then successful
retry with the existing spawn coordinates/elevation and retained heading.
Focused repository/entry/recovery checks passed (1.3s/1.6s), followed by full
database repositories, world (55.0s), battle, session and server race suites.
All Go packages compile and diff checks pass. Existing generated contracts remain
unchanged; no rendered or process-death evidence is inferred from these checks.

Next: complete the trainer-card/Pokédex read ownership, coherent snapshot and
response-lifetime audit in the finite command matrix. All five roadmap areas
remain open; the original restore timeout and Repel click remain unattributed.
No schema, generated contract, asset or frontend behavior change. No push or
deployment, and no new rendered acceptance is claimed for this persistence change.

## Camp position authority and playtime crash policy (2026-10-08)

Following the movement-manager audit, the general character repository caller
inventory found a separate stale-pose writer: `CharacterQuitRequest` called
`UpdateCharacter` before cleanup. A real dispatcher regression showed durable
`x=9` becoming cached `x=7` on camp. Camp now retires the character and persists
final playtime through the existing cleanup owner, without replaying coordinates.
The sole remaining runtime `UpdateCharacter` caller is entry's last-login update;
narrowing that intent and checking recovery-spawn Z persistence remain next work.

The browser crash check also found that a newly created character's trainer card
failed with `sql: no rows in result set` because it duplicated the wallet query.
The authoritative currency reader already defines an absent wallet row as a zero
balance until the first currency change. It now accepts an injected query owner
and context, and the trainer card reuses that reader. Missing wallet tables and
other SQL failures still reject the response; they do not become zero balances.
The remaining card reconciliation/count reads and snapshot/lifetime audit are
not closed by this wallet fix.

Rendered process-death acceptance passed in 10.9s using the existing isolated
runner: baseline `120`, trainer-card active total `121`, rejected final save,
actual server exit `137`, restart generation `1`, and fresh entry baseline `120`.
The stored position remained `38,3,6`. Exact owned PIDs were `2565800` and
`2566235`; receipt, protocol evidence and logs are retained at
`/var/tmp/capturequest-rendered.4nKsy2`. The original server log confirms
`Camp cleanup: final playtime: commit transaction` with the injected rejection.
This verifies the metric limitation and unchanged stored position; it is not
new coverage of every gameplay mutation or a production test.

Earlier test iterations exposed an initial state-stream readiness assumption,
the genuine empty-wallet defect, and a test click blocked by the open trainer-card
overlay. The test now waits for a valid streamed identity and closes the modal
using its existing Escape handler. Its assertions were retained. The empty-wallet
wire regression covers both valid zero balance and missing-table SQL failure;
the owned reader also has a held-pool cancellation check. Camp's stale-position
regression failed at `x=7` before the fix and passes with durable `x=9` afterward.
Focused camp/reentry/shutdown checks passed (1.6s), then trainer-card/camp wire
checks passed (1.3s). Full world (56.9s), server, session, currency and character
repository race suites passed, including the new held-pool wallet deadline check.
All Go packages compile and diff checks pass. Generated contracts are unchanged.

Retained playtime policy: it is a display metric, not an eligibility or reward
input in the audited runtime consumers. Entry starts from the durable cumulative
total; active elapsed seconds can appear before persistence. Periodic/final saves
are bounded, monotonic and repeatable after unknown commit acknowledgements.
While the process lives, failed final saves fence reentry and are retried by the
existing owner; shutdown reports unresolved saves. Process death preserves the
last committed total and loses an uncommitted interval or memory-only pending
save. No offline time or guessed crash interval is credited. Healthy periodic
persistence normally limits that interval to the existing minute schedule, but
database failure can extend it. This explicitly retained metric limitation does
not relax durable gameplay command/position/route guarantees or close the broad
goal. No new journal or parallel accounting architecture is introduced.

Next: narrow entry's last-login write and finish the trainer-card/Pokédex owned
read audit, then continue the finite command matrix. Original restore-timeout
and Repel-click attribution remain open. No push, deployment, schema or generated
asset change.

## Retire obsolete deferred position persistence (2026-10-08)

The preceding caller audit removed every runtime position-staging producer.
Ordinary steps, forced-route progress, SURF, map arrival and teleports commit
through their existing transactions before publishing live state. The remaining
dirty assignment was on a detached planning candidate and never represented an
unsaved live position. Keeping a timer flush and disconnect replay for it retained
a second persistence path with no authoritative producer.

`FlushPlayerPosition`, `positionDirty`, `lastSaveAttempt`, idle dirty-retry admission
and final-position recovery are now retired. Disconnect unregisters the projection;
it cannot rewrite a newer durable pose or mutate the durable remaining route.
The existing position/route transactions and reconnect reads remain authoritative.
The public committed-position timestamp remains unchanged in the wire type.
Character-owner recovery now retains only frozen cumulative playtime, which still
has a real unpersisted interval between periodic saves.

Tests of the retired flush API, artificially dirty snapshots and mixed
position/playtime final saves are removed with that API. This is not a relaxed
failure guarantee: the replacement has no deferred position save to fail. Existing
transaction rollback/cancellation, map-arrival publication, movement ownership and
route-recovery checks remain. A real cleanup regression holds the only pool
connection, retires a stale projection without acquiring storage, and proves the
newer durable pose and stored route remain unchanged. Same-tile teleport still
clears pending intent/path without a storage read. Playtime failed handoff,
unknown-commit retry, normal reentry and sealed shutdown checks remain.
Focused position/route/map-load/cleanup/playtime checks passed (5.7s), followed
by full world (53.4s), battle, session and simulator race suites. All Go packages
compile and diff checks pass. No new rendered or process-death run is claimed;
prior committed-position/route restart evidence remains its existing baseline.

Next: resolve the remaining playtime process-death policy and acceptance, then
continue the finite command/lifecycle inventory. Playtime is persisted every
minute under normal operation; process death loses the interval since the last
successful durable save. Failed persistence can extend that interval, so the
existing timer is not a universal one-minute loss bound. Memory-only recovery
callbacks cannot survive process death. No new filesystem journal, broker or
event-sourcing system is introduced. All five roadmap areas remain open, including
the unattributed restore timeout and Repel click failure. No push or deployment.
The current consumer audit finds playtime only in character loading/tracking and
the trainer-card/sidebar display (`handler-pokedex.go`, `TrainerCard.tsx`,
`StatInfoSidebar.tsx`); no gameplay eligibility/reward reader was found. This is
evidence for evaluating an explicit metric persistence policy, not proof of a
new crash guarantee or authorization to weaken other gameplay durability.

## Committed teleport projection; retire headless staging (2026-10-08)

The complete `UpdatePosition` caller inventory found two runtime calls. Ordinary
teleport publication called it after committing, then cleared its dirty flag in
a separate publication call. The cutscene fallback called it without a valid
session, even though `cutsceneMutation.movePlayer` had already committed. A real
transaction regression reproduced a headless cutscene altering an unrelated live
movement registration and marking that committed pose as unsaved.

The fallback and its redundant helper are retired. Cutscene publication uses the
existing committed-position publisher, which requires a valid session before
touching live projections. Teleport projection is now named
`projectCommittedTeleport` and records clean state in its own locked update,
including same-tile teleports, while retaining path cancellation and map rules.
No intermediate dirty window exists between teleport publication calls. The
generic production `UpdatePosition` API is retired; existing failure fixtures now
use one explicit test-only staging helper without weakening dirty-save assertions.

The remaining production `positionDirty=true` assignment is in
`planCharacterStep`, operating on the detached candidate made by its sole runtime
caller. After the transaction commits, the installed live state is explicitly
clean. Thus the audited live position producers do not create deferred final-save
obligations; durable step/route/teleport recovery remains authoritative. Defensive
dirty-state flush/cleanup checks stay intact. This audit does not close every
movement/field authorization or presentation gate in the command matrix.

The headless regression failed before the fix, then passed alongside teleport
projection and preserved dirty failure/retry checks (1.5s). Full world (65.7s),
simulator and session race suites passed. The canonical private-bootstrap
`route23_earth_badge_blocked` CLI scenario passed its runtime expectations,
including native-coordinate movement from `(-46,-173)` to `(-46,-172)`; evidence
is `/var/tmp/capturequest-script-sim.BecgUg`. This is a scenario/runtime expectation
check, not a text-golden or full-corpus rerun. All Go packages compile and diff
checks pass.

Next: address playtime process-death policy and remaining lifecycle boundaries,
and reassess defensive staging machinery now that its runtime producers are gone.
Playtime still has the
existing one-minute periodic persistence interval and can lose time accumulated
since the last durable save after process death. Pending cleanup callbacks are
still memory-only. The original restore timeout and Repel click failure remain
unattributed. No rendered/production acceptance, push, deployment, schema or asset
change is claimed; all five roadmap areas remain open.

## Shutdown reconciles previously failed cleanup (2026-10-08)

A PostgreSQL regression reproduced a missing shutdown boundary: after
`RemoveSession` retained a failed playtime save, shutdown returned success without
retrying or reporting it. Removing the failure still left `time_played=0` instead
of the frozen three-second total. The session-only drain could not see retired
owners; its separate error list only recorded failures after draining began.

Shutdown now seals character admission, joins the existing session/worker cleanup,
then reconciles the existing owner registry with one shared five-second recovery
budget while storage remains open. Failed saves retain their immutable obligations
and return errors naming the character. Successful recovery removes the obligation
and no longer reports its historical failure as unresolved. The separate
`cleanupErrors`/`cleanupDraining` state is retired. A handoff already running when
shutdown seals admissions cannot publish a replacement afterward.

Coverage includes failed cleanup before shutdown, successful recovery after the
failure disappears, persistent failure, concurrent handoff sealing, cancellation
with retained obligations and repeated sealed recovery. A held-pool regression
also checks that a caller deadline leaves the same drain running with storage
open, then joins its successful recovery after the pool becomes available.
The pre-fix regression failed both persistent-failure reporting and recovery.
After the fix, focused shutdown/entry/owner checks passed (2.1s), full world
(53.2s), server, session and character-repository race suites passed, and the final
held-pool/sealing checks passed (1.3s). All Go packages compile and diff checks pass.

Next: audit the remaining staged-position producer and define process-death
durability for genuinely unsaved final state. Pending callbacks are still in
memory; this does not establish recovery after server death, rendered acceptance
or production behavior. All five roadmap areas remain open. No push, deployment,
schema or generated asset change.

## Failed-cleanup admission recovery (2026-10-08)

The failure inventory established two connected defects. `cleanupCharacterSession`
released character ownership, discarded registered position and called
`StopPlaytime` even when final saves failed. The first replacement received an
error, but a subsequent acquire could enter without those obligations. Separately,
the sole runtime playtime writer added an interval to `time_played`; a committed
write whose acknowledgement was lost could count the interval twice on retry.

Playtime now saves a cumulative character total, monotonically preserving a
higher durable total. This policy relies on the existing exclusive character
owner and post-drain entry reload; it does not aggregate independent concurrent
sessions. The additive repository API is retired. General character saves still
leave playtime untouched. Final cleanup freezes the total before storage work,
so a disconnected session cannot accumulate recovery-wait time.

Movement retirement atomically removes the live writer and captures only an
outstanding dirty pose. The existing character-owner entry retains an immutable
recovery callback on save failure. Every subsequent acquire retries it behind
the same handoff barrier before admitting a replacement and reloading durable
state. Cancellation/failure retains the obligation; success removes it. Neither
the retired client nor mutable movement state is retained. Both final writes can
be repeated after partial/unknown commits while replacement writers remain fenced.

Verification covers committed-but-unacknowledged playtime, repeated/later totals,
post-drain baseline progression, clean/dirty repeated rejection and recovery,
partial position failure with committed playtime, disconnect-originated recovery,
retired-tracker freezing, concurrent admission and cancellation/retry. These are
PostgreSQL/state-boundary checks, not rendered or production acceptance. A real
`EnterWorld` dispatcher test proves recovery precedes the replacement's durable
baseline reload and playtime tracker initialization. Full world (51.4s), session,
character repository and battle race suites passed. The final added focused
checks passed (1.7s; normal entry 1.1s), all Go packages compile and diff checks pass.

Remaining: pending obligations are held in server memory. Process death during a
genuine final-save failure can still lose the pending dirty pose or unsaved time.
Shutdown also needs an explicit audit of failures retained before draining began,
including retry and failure reporting. Complete these lifecycle policies and
rendered/restart recovery acceptance before treating final-save recovery as closed.
The historical restore timeout and Repel click failure remain unattributed, and
all five roadmap areas remain open. No new push or deployment is authorized by
this continuation; this is a local implementation checkpoint with no schema or
generated asset change.

## Requested branch handoff (2026-10-08)

The user requested a stopping point, commit and GitHub branch push. Implementation
is already committed through `a343ac5`; this documentation checkpoint accompanies
the branch push. No partial implementation is left in the worktree. Prior test
evidence is recorded with each checkpoint below; this documentation-only handoff
does not claim a new test run or production deployment.

Next recommended work is genuine cleanup-failure/reentry recovery through the
existing character, movement and session owners. Investigation has started, but
no recovery policy or fix has been implemented. In particular, retrying additive
playtime after an uncertain commit must not double-count it, and recovery must not
allow a retired position writer to overwrite a newer owner. The historical login
restore timeout and Repel click failure remain unattributed. All five roadmap
areas remain open; the finite remaining inventory is in `SERVER_COMMAND_AUDIT.md`.

## Skip clean position rewrites; preserve dirty cleanup obligations (2026-10-08)

Position producer inventory distinguishes committed projection from detached
planning and legacy staging. Ordinary/SURF completion commits before
`projectCommittedPosition`; published teleports call `markPositionCommitted`.
Forced planning changes a detached copy and installs it only after commit. The
flush nonetheless rewrote any registered position, even with `positionDirty=false`.

`FlushPlayerPosition` now returns without storage work for clean state. Committed
ordinary/SURF projection records clean state directly, including same-tile results,
while retaining its path/pending/owner behavior. Generic staged updates remain
explicit dirty obligations. Dirty flush still uses the owned transaction, retains
failure state for retry, and cannot mark a newer snapshot saved after an older
blocked flush. No reconnect or clean disconnect now manufactures a position write.

The PostgreSQL held-pool test proves a clean committed flush needs no connection.
Existing dirty commit-rejection/retry and newer-snapshot tests pass. Cleanup tests
now cover both clean and explicitly staged dirty positions: clean failure reports
playtime only; dirty failure reports position and playtime, rejects replacement,
retires the old writer and persists no partial save. The earlier fixture expected a
position failure from a clean registration; explicitly staging the dirty case
preserves its failure assertion instead of weakening it.

Focused movement/step/shutdown checks passed (5.5s), final clean/dirty cleanup checks
passed (1.8s), full world (67.8s), battle and session race suites passed, all Go
packages compile and diff checks pass. No new rendered or process-death acceptance
is claimed for this save-admission change. The preceding committed-movement and
crash/reentry evidence is retained as baseline; it does not prove every cleanup
failure recovery outcome.

Remaining: genuinely dirty final position and playtime can still fail during
cleanup, after which local writers retire. Define recovery/admission from existing
durable owners without guessing successful saves or retaining stale writers.
Audit the remaining staged-position fallback producer and final-playtime lifetime,
plus other matrix rows. The original restore timeout and Repel click failure remain
unattributed, and all five roadmap areas stay active. Next: complete cleanup
failure/reentry recovery policy through the movement/session ownership boundary.
This is a local checkpoint only, without push, deployment, schema or asset change.

## Shared lock transaction-contract review (2026-10-08)

Caller inventory confirms character locking is performed through owned transaction
query handles or explicit test transactions. The primitive nevertheless accepted a
plain pool: an autocommitted SELECT could release the row lock before the protected
operation and falsely advertise ownership. `LockCharacter` now uses the existing
`RequireTransaction` guard before querying. That guard also rejects nil `*sql.Tx`
and nil owned query handles instead of allowing dereference panics. The obsolete
comment claiming SQLite/no-op UPDATE semantics is removed.

The PostgreSQL regression occupies the only pool connection and invokes ownership
with a plain database. It must reject immediately without changing the pool wait
counter; nil handles also reject. Existing serialization/deadline, missing-row and
update-trigger regressions still pass. Focused db/inventory/battle checks passed
(2.0s/2.5s/4.2s), full db repositories/economy/itemuse/battle/world race suites passed
(world 70.5s), all Go packages compile and diff checks pass. No new browser run was
needed for this admission guard; the preceding actual Repel crash/reentry evidence
covers valid transaction consumers, while the guard regression covers misuse.

Lifecycle source review found a remaining distinct boundary: cleanup always invokes
`FlushPlayerPosition` for a registered state, then unregisters it even on failure.
The flush does not inspect `positionDirty`, and committed projection setters mark
that flag. Before changing recovery policy, audit every producer of dirty position
and distinguish authoritative committed projections from genuinely unsaved legacy
state. Durable final-save recovery is still open; this patch does not guess or
silently discard that obligation.

The original restore timeout and Repel click failure remain unattributed, and all
five roadmap areas stay active. Next: audit final-position producers and cleanup
recovery through the existing movement/session owner before expanding another
command family. This checkpoint is local only, without push, deployment, schema
or generated-asset mutation.

## Character ownership lock without manufactured writes (2026-10-08)

Returning to the original restore investigation exposed a concrete shared-boundary
issue: `db.LockCharacter` used `UPDATE character_data SET id=id` to acquire ownership.
Even empty battle restoration therefore issued a character write, firing UPDATE
triggers and producing row versions. The primitive now uses `SELECT id ... FOR
UPDATE`, preserving exclusive row ownership and missing-character rejection while
avoiding that manufactured mutation. Six independent ID-only read locks in
inventory, merchant, PC, actor reads, preferences and Bicycle are consolidated
through the same primitive. Position reads that also fetch fields retain their
single owned query.

`ResumeBattle` errors identify character ownership, saved-state load, party restore
and legacy identity upgrade while preserving underlying errors; begin/commit remain
classified by the existing transaction wrapper. Real PostgreSQL checks install a
rejecting UPDATE trigger and prove character locking and empty restore do not fire
it. Held row/relation locks reach ownership, saved-state and party stages under the
caller deadline; relation wait is observed in `pg_locks` before cancellation. The
historical event is still unattributed: this demonstrates a defect/diagnostic
boundary, not evidence that a trigger caused the original five-second timeout.

The first broad checks exposed legacy SQLite mutation fixtures whose fake schema
could not execute a PostgreSQL row lock. PC storage, boulder, fishing, trade and
teleport fixture helpers now use the canonical private Postgres schema, preserving
their behavioral assertions. No SQLite dialect fallback or test-only lock mode was
added. Unrelated read/parser SQLite fixtures remain. Canonical not-null map metadata
was supplied explicitly instead of weakening schema invariants.

Final db/db repositories/economy/itemuse/battle/world race suites pass (world 58.8s).
After read-lock consolidation, inventory/merchant/PC/preference/Bicycle/actor/recovery
checks pass (world 10.5s), all Go packages compile and diff checks pass. Both Repel
browser cases pass in 25.9s at `/var/tmp/capturequest-rendered.rEGhSI`, including
verified exit 137/restart generation 1 and reentry. No restore-stage/deadline failure
matched those retained server logs. A passing rerun does not close the original
incident or the earlier pre-command UI click failure.

Remaining: establish historical timeout attribution if reproducible, shared lock
transaction/admission review, durable final-save and remaining lifecycle owners,
other command matrix rows and prior source/golden gaps. All five areas remain
active. Next: review the consolidated ownership boundary and continue the finite
lifecycle/command audit, keeping restore-stage evidence distinct from root-cause
attribution. No schema, assets, production database or release publication changed.
This checkpoint is local only, without push or deployment.

## Native Silph foot semantics and complete runtime corpus (2026-10-08)

The Silph mismatch was an old whole-block collision expectation. Original
`SilphCo2F.asm` installs closed block $54. Canonical block bytes are
`08080808181818180101010101010101`; bottom-left foot samples for its four
quadrants are $18/$18/$01/$01. Original `Facility_Coll` includes $01, excludes
$18. The upper door is blocked while the lower floor remains walkable. The prior
manual approximation marked the entire 32×32 block blocked.

The corpus audit found exactly two lower closed-quadrant assertions with that old
expectation, both in `silph_card_key_2f_door1_no_key`. They now assert collision 1
from source, preserving the upper collision-0, closed-label, no-key flag and
absent-battle assertions. No runtime collision or authorization was changed.
A real PostgreSQL boundary regression proves the player can enter the lower floor,
cannot path through the closed upper door, and can cross only after the existing
Card Key transaction commits the open flag. Missing palette rows remain valid
catalog metadata as established by the earlier shared-reader fix.

Focused Silph tests passed (1.2s); final Silph/collision/owned-path race checks
passed (1.6s), and diff checks pass. The canonical isolated `script-sim --all`
runtime-expectation run completed all 484 scenarios successfully. Evidence is
`/var/tmp/capturequest-silph-source-corpus.log` and
`/var/tmp/capturequest-script-sim.524SOJ`; the private cluster is stopped. This
closes the current runtime-expectation corpus run, not every simulation lifecycle
or server roadmap requirement.

Fixed text goldens still contain old labels/catalog numbers and need deliberate
portable formatting/source review; full `--all --check`, rendered appearance and
production acceptance are not claimed. Standalone manual rules/numeric identities,
received-item dialogue hydration, Giovanni flag migration, simulator fixture/action
writers, broader command recovery and lifecycle audits remain in the finite matrix.
The original restore timeout and Repel click failure remain unattributed; all five
goal areas remain active. Next: return to remaining server ownership/lifecycle and
shared-boundary audits, using this verified corpus as the regression baseline and
retaining the unresolved timeout investigation. This is a local checkpoint only,
without push, deployment or production mutation.

## Explicit source-coordinate scenario frame (2026-10-08)

Route 23 scenarios expressed original local coordinates as world coordinates.
The Cascade guard fixture/trigger uses (8,136), but canonical tiles establish a
single offset (-50,-208); the generated trigger correctly uses (-42,-72). The
simulator looked for an eligible trigger at the untranslated fixture value.

Scenarios can now declare `coordinateSpace: "source"`. Fourteen Route 23 badge
cases do so; their coordinate literals and gameplay assertions stay unchanged.
Before fixture mutation, the already-negotiated source resolver translates fixture,
coordinate trigger and expected final position using the existing compiler
coordinate translator. Interior coordinates remain unchanged. Unknown maps,
missing offsets, unsupported frames and unsupported source trigger types reject;
there is no Route 23 offset constant or runtime location exception. Unmarked/world
scenarios retain their current interpretation. Resolved updates publish only after
source/contract checks succeed, and the in-memory frame becomes world to prevent
double translation.

Regressions verify the exact Route 23 translation, unchanged interior values and
missing-source rejection. Focused compiler/simulator checks passed (2.6s/1.5s),
full compiler/simulator/world race suites passed (world 48.9s), all Go packages
compile and diff checks pass. The canonical private corpus completes 395 scenarios,
including the Route 23 blocked/pass cases, then fails
`silph_card_key_2f_door1_no_key` on (4,5)/image 167/collision 0 while native output
reports that bottom quadrant as collision 1. Its assertion was not changed.
Evidence is `/var/tmp/capturequest-source-coordinate-corpus.log` and
`/var/tmp/capturequest-script-sim.5qpubj`; the private cluster is stopped. This is
headless source-coordinate/runtime selection evidence, not rendered actor/art or
complete golden acceptance.

Remaining: source-foot collision provenance versus old manual whole-block
expectations at Silph, standalone tile rules/text goldens, received-item text
hydration, Giovanni flag migration, simulator writers and other command rows.
The original restore timeout and Repel click failure remain unattributed; all five
areas stay active. Next: audit Silph collision semantics through original passable
lists and the shared compiler before more command migration. This checkpoint is
local only, without push, deployment or production mutation.

## Retire covered manual block tile overrides (2026-10-08)

The Mansion mismatch was a competing producing path. Native source expectations
resolved floor block $0e correctly, but tracked manual palettes painted the same
cells with catalog-specific image 758. The compiler deliberately skipped any native
candidate overlapping manual ownership, leaving those approximations authoritative.
Original `PokemonMansion1F.asm` and structured candidate data describe the same
switch block coordinates and source blocks $0e/$2d.

The full manual block corpus contains 70 declarations. Every label has an
unambiguous structured native replacement; all coordinates and required/absent
flags match exactly. Those covered declarations and their now-unused numeric
palettes are retired as one data change. The 20 standalone tile rules remain for
separate provenance review. This removes shadow ownership across Mansion, Silph
and Victory Road instead of repairing image constants individually. No assertion,
collision expectation or runtime renderer was relaxed.

Canonical generation and `import-script-candidates --check` pass. Native event
tile output grows from 64 to 344 rules; diagnostic decisions move to 440 generated
and one remaining manual skip, with no unsupported increase. Sync/compiler/world
race suites passed (world 48.0s), all Go packages compile and diff checks pass.
The canonical isolated corpus completes 331 scenarios, including Mansion native
switch states, then fails selecting a Route 23 coordinate cutscene at (8,136).
Evidence is `/var/tmp/capturequest-native-block-retirement-corpus.log` and
`/var/tmp/capturequest-script-sim.9glJsr`; the private cluster is stopped. This is
runtime data/contract evidence, not rendered tile-art or complete text-golden
acceptance.

Remaining: establish source-coordinate provenance for the Route 23 trigger failure,
standalone numeric tile rules/expectations, portable goldens, received-item text
hydration, Giovanni legacy flag migration and other roadmap rows. The original
restore timeout and Repel click failure remain unattributed, and all five goal
areas stay active. Next: audit the Route 23 source-to-world coordinate boundary
through its existing translators before another family migration. Manual override
retirement and generated output require the full-data lane for any future release;
no push, deployment or production mutation occurred. This checkpoint is local only.

## Shared-graphics tile identity and importer metadata (2026-10-08)

Original `LancesRoom.asm` selects blocks $31/$32 for the open entrance and
$72/$73 for the closed entrance, at block coordinates (6,2)/(6,3). The structured
candidate agrees. The catalog contains those images, but stores GYM-shared images
under the DOJO identity 5 while their block/2bpp data belongs to GYM 7. The compiler
signature index joined blocksets directly on image.tileset_id, dropping these rows;
its existing partial map only covered MART and DOJO. The importer had the same
join assumption and could persist null raw-foot metadata for shared images.

Compiler resolution, catalog indexing and imported foot metadata now use one
`phaserdata.BlocksetTilesetID` mapping derived from the authoritative extractor
config: 2→6, 5→7, 4→1, 9→12 and 10→12. Catalog identity stays original; only source
data lookup follows the shared blockset. All five aliases were included after the
first broader check exposed image 279 under another alias; that failed check was
not accepted or hidden. No Lance-specific image, coordinate, collision fallback
or hand-edited artifact was introduced.

Regressions prove a DOJO image participates in decoded signature lookup and the
Postgres importer preserves its original tileset ID while obtaining native foot
metadata from GYM. Alias mapping tests cover the complete source alias corpus.
Focused importer/compiler/data race checks passed (5.2s/2.5s/1.1s); full importer,
compiler, data and world suites pass (world 52.3s), all Go packages compile and
diff checks pass. Canonical generation plus `import-script-candidates --check`
passes: all 25 tile candidates are supported, rules increase from 48 to 64 and
Lance contributes 16. The old tile unsupported diagnostic is gone; no budget was
relaxed. Output was regenerated through the locked publication tooling.

The canonical private importer/corpus run completed 258 scenarios, including both
Lance entrance cases, then stopped at `pokemon_mansion_1f_switch_tiles_off` on an
existing numeric identity assertion: (12,24), expected image 167. Evidence is
`/var/tmp/capturequest-alias-final-corpus.log` and private data at
`/var/tmp/capturequest-script-sim.FIAN7j`; the cluster is stopped. This is runtime
state/contract evidence, not rendered art or complete corpus acceptance. No
production import or publication occurred. Future deployment of compiler/importer
and generated-rule changes requires the full-data lane and its backup/contract
verification.

Remaining: unmapped numeric expectations/text goldens, received-item text hydration,
Giovanni legacy flag migration, simulator writers and the remaining command matrix.
The original restore timeout and Repel click failure remain unattributed, and all
five goal areas stay active. Next: resolve the Mansion expectation through native
source identity and review the shared received-item producer before more command
migration. This is a local checkpoint only, without push or deployment.

## Source-owned gym rewards and shared map-script selection (2026-10-08)

The Lt. Surge label mismatch reflected duplicate ownership: the scenario named a
reconstructed manual reward, while the runtime selected the extractor-generated
`VermilionGymLTSurgeReceiveTM24Script`. Original `VermilionGym.asm` defines that
label and its GiveItem/TM/badge state machine. Corpus review found all eight gyms
have native `gym_leader_tm_reward_v1` candidates. Seven use the same victory flag
as the manual counterpart and guard repeat delivery by the source TM-received flag.
Their manual duplicate files are retired, and scenarios assert the native labels
while preserving badge, inventory, action and absent-battle assertions.

Giovanni is deliberately retained pending an explicit migration: the legacy manual
requires `EVENT_BEAT_GIOVANNI_GYM`, while native source requires
`EVENT_BEAT_VIRIDIAN_GYM_GIOVANNI`. The existing gym metadata supports the former as
an alternate win flag; deleting its reward now would strand that old state. This
is a remaining migration gate, not approval for permanent duplicate ownership.

Removing the other duplicates exposed map-script selection by insertion order:
Cinnabar's unconditional reset preceded its conditional reward. The map selector
now uses the same existing specificity rule as click/coordinate selection, sorting
a copied view rather than mutating shared cache. The regression proves an eligible
reward wins over reset and ceases to win once its TM flag is present. No per-gym
priority constant, new reward coordinator or handwritten replacement is added.

Focused selection/cutscene/gym tests passed (1.9s), full world (51.1s), simulator
and script-sync race suites passed, all Go packages compile and diff checks pass.
Runtime corpus completed 181 scenarios, including all eight gym reward checks,
then stopped at `lances_room_entrance_blocks_closed` on its existing numeric tile
expectation (12,6)/image 50. Evidence is
`/var/tmp/capturequest-gym-native-selected-corpus.log`, with private runtime data at
`/var/tmp/capturequest-script-sim.TVgTYr`; the private cluster is stopped.

Canonical generated-output verification initially found stale diagnostics from the
prior native resolver's revised error wording. Canonical regeneration followed by
`import-script-candidates --check` passes. Generated/skipped/unsupported decision
counts remain 422/18/1; no unsupported budget was relaxed. The unresolved Lance
source block/quadrant diagnostic remains visible. Ignored output was regenerated
through its locked publication tooling, not edited manually.

Remaining: Giovanni canonical flag migration; native received-item dialogue
hydration (the generated Surge receipt line currently omits the item name);
remaining numeric expectations/text goldens; source/compiler unsupported records;
and broader duplicate, reconnect and rendered reward acceptance. Source-native
reward semantics are tested here, not complete historical/UI fidelity. Future
production publication of the manual-script retirement requires the documented
full-data deployment lane; no push, deployment, backup/import or production state
mutation occurred. The original restore timeout and Repel click failure remain
unattributed, and all five goal areas stay active. Next: review Lance source identity
and the received-item text compiler as shared producing boundaries before another
command migration. This checkpoint is local only.

## Native tile expectation identity (2026-10-08)

The corpus audit found 156 tile expectations; 132 have unambiguous native block
and quadrant identities in the negotiated SQLite tile-override candidates. Those
expectations now name source `mapName`, `blockId` and `position` instead of stale
numeric catalog IDs. Translation was deterministic from structured candidates and
preserved coordinate, collision and label assertions. The remaining 24 unsupported
mappings retain their original numeric assertions; nothing is inferred for them.

The event compiler's existing decoded-artwork signature resolver now exposes a
native identity entrypoint, reused by compilation and simulator expectations.
The simulator CLI lazily loads `--tile-source` (default the canonical SQLite path)
in read-only mode and negotiates `extractorcontract` before resolution. It verifies
release/run/source-tree identity against imported Postgres metadata and checks each
resolved image's catalog tuple against the imported image record before fixture
mutation. Missing source fields, unknown maps/blocks/quadrants, mixed numeric/native
identity and differing catalogs reject; resolution stages its changes before
publishing the full expected set. There is no separate renderer or label-to-runtime-
rule oracle and no fallback ID.

Tests prove decoded native identity survives catalog renumbering, malformed source
and mismatched catalog tuples reject, and the final assertion still rejects the
wrong image, collision and label. Compiler, simulator and world race suites passed
(world 64.5s), final focused tests and all-package compilation pass, and diff checks
pass. The final Agatha CLI runtime expectation succeeds at
`/var/tmp/capturequest-script-sim.UGN607`. In the full corpus run, 176 scenarios
completed before scenario 177 (`gym_lt_surge_reward`) failed: expected
`VermilionGymLtSurgePostBattle`, actual `VermilionGymLTSurgeReceiveTM24Script`.
Evidence is `/var/tmp/capturequest-native-expectation-corpus.log`; its private
cluster stopped normally. That script-label assertion was not changed.

This is runtime-expectation acceptance, not complete fixed-golden acceptance.
Text goldens still contain old catalog numbers and require a deliberate portable
identity format/source review. The 24 unmapped assertions, new label failure and
remaining corpus cases remain open. Runtime gameplay, schema, opcode and generated
asset publication did not change; no rendered appearance or deployment is claimed.
The original restore timeout and pre-command Repel click failure remain unattributed,
and all five goal areas stay active. Next: trace the Lt. Surge script-label boundary
against structured source and finish portable expectation/golden identity without
weakening assertions. This checkpoint is local only, without push or deployment.

## Runtime image metadata authority correction (2026-10-08)

The apparent missing event metadata was a reader defect, not absent extractor
output. The importer populates `phaser_tile_images.raw_foot_tile_id` and
`talk_over_tile` from block data. `phaser_tile_properties` is a sparse editor table;
normal imports do not populate a palette row for each image. The shared runtime
reader nevertheless started its query from that editor table. Its earlier strict
error propagation incorrectly made an optional palette record mandatory.

The runtime query now starts from the required imported image and left-joins
optional palette properties. Native foot/talk metadata remains authoritative;
explicit event/placed collision remains unchanged, while the editor's optional
collision preference retains its declared blocked default. Truly missing catalog
images still fail. No guessed metadata, per-image exception, importer repair,
manual generated-file edit or regeneration is introduced.

The earlier missing-metadata fixture deleted a palette row. Schema/importer evidence
shows that expectation was wrong. The regression now proves native metadata and
event publication succeed without palette, then deletes the actual required image
and still demands failure. Collision rejection likewise checks a missing image
while its palette row remains. This corrects the producing assumption rather than
weakening an unexplained failing assertion.

Focused world checks passed (6.8s), full world (51.3s), simulator and importer suites
passed, all Go packages compile and diff checks pass. Canonical SQLite contains
complete 16-byte block data for image 50 (tileset 15/block 45/position 2) and image
253 (tileset 3/block 80/position 1). In the matched private Postgres catalog their
raw-foot IDs are 23 and 72, talk-over is false and both palette rows are absent.
The repaired `vermilion_gym_trash_second_lock_success --check` golden passes.
Evidence is retained under `/var/tmp/capturequest-image-metadata-*` and the private
cluster `/var/tmp/capturequest-script-sim.NbhlOA`; that cluster is stopped.

Full corpus acceptance remains unproven. The rerun now returns real Agatha tile
states without metadata failure, then rejects the scenario's old numeric identity:
(0,4) expects tile 261, current generated catalog uses 50. Tile IDs are catalog-local;
156 scenario references to `tileImageId` warrant a corpus/source-aware expectation
model rather than editing individual constants or suppressing assertions. No golden
or expectation was changed here. Runtime art/contract/publication are untouched,
and there is no rendered appearance or production deployment claim.

All five roadmap areas remain active. The original restore timeout, pre-command
Repel click failure, remaining simulator writers and other command rows stay open.
Next: audit tile expectation identity against authoritative structured source and
build a reusable catalog-aware assertion boundary before more command migration.
This checkpoint is local only, without push or deployment.

## Shared simulator snapshot read boundary (2026-10-08)

The shared snapshot reader no longer combines independent global queries with a
runtime battle cache. `CaptureSnapshot` now requires context/database and owns the
existing bounded read-only repeatable-read `db.ReadSnapshot`. Character, flags,
wallet, coins, Pokédex, party, PC, inventory, hidden objects, battle, Day Care and
Vermilion puzzle state use its one query handle. Existing domain readers are
exposed/reused rather than duplicated; persisted battle loading and the cached
summary share the same summary constructor. Metadata lookup errors propagate,
and a late failure returns no partially populated snapshot.

All initial/final snapshot consumers pass their caller context and database through
the existing scenario functions, including final pathfinding capture. Fixture and
other action writers remain global. During that migration, `Run` rejects a database
different from its initialized fixture target before mutation; it must not write
one database while inspecting another. There is no mutable global snapshot context
or second read model.

PostgreSQL regressions remove the global database, use a one-connection pool,
inspect persisted battle state without its runtime cache, fail the final puzzle
read and verify no partial result, and cancel an actual pool wait. A held coins
relation lock pauses the aggregate while another transaction commits changes to
name, wallet, coins and flags. The first snapshot sees all old values; a fresh one
sees all new values. Focused checks passed (1.4s simulator, 1.5s world), full world
(50.5s) and simulator race suites passed, all Go packages compile and diff checks
pass. Canonical type regeneration produced no wire change.

Corpus verification remains incomplete. The isolated `--all` runtime-expectation
run at `/var/tmp/capturequest-script-sim.FcoYdO` completed eight scenarios, then
failed `agathas_room_exit_block_closed` with missing event metadata for map 247,
coordinate (0,4), image 50. A matched parent-revision executable (`c43f8f5`) in
`/var/tmp/capturequest-snapshot-control.*`, using the same database and complete
generated script/metadata family, fails identically after eight scenarios.
The initial control lacked side metadata and was discarded as a matched comparison.
Three representative fixed goldens pass: `daycare_deposit_pikachu`,
`fixture_party_detailed_state`, and `game_corner_buy_coins_exact_fee`.
`vermilion_gym_trash_second_lock_success` fails on map 92, coordinate (4,4), image
253; the matched parent also reproduces that error. No goldens, source diagnostics
or assertions were weakened. Private cluster shutdown logs are retained; no
production database or generated asset publication was involved.

Remaining: establish provenance/root cause for those pre-existing event metadata
gaps before claiming corpus acceptance, migrate simulator fixture/action writers
and remaining background work, and retire dormant diagnostic wrappers. Other
command rows and reconnect/idle recovery remain open. The original restore timeout
and pre-command Repel click failure remain unattributed; all five goal areas stay
active. Next: trace the missing event-image properties through authoritative data
and importer/sync production before another command migration. This checkpoint is
local only, without push or deployment.

## Owned standalone pathfinding queries and explicit simulator failure (2026-10-08)

`FindPathForCharacter` and its options variant previously chose a database through
manager/global fallback, started a background-context collision read and converted
its error into an empty path. The simulator then treated that as a successful
`Found=false` result. Both APIs now require the caller context and query handle,
return `([]PathNode, error)` and distinguish source/deadline failure from a valid
blocked route. Context checks before/after planning reject retired results; no
new planner, cache or mutation coordinator is introduced.

The simulator pathfinding call supplies its initialized private database and
propagates errors before producing a result. The CLI owns an interrupt/SIGTERM
context through initialization, scenario dispatch, initial flag/script loads and
pathfinding. Initialization still requires the explicit disposable database DSN;
application-database fallback remains forbidden. Already-cancelled scenario runs
reject before starting fixture application. This does not imply that every legacy
fixture write or simulator action is now cancellable.

Real PostgreSQL checks use no global database and prove a valid route, legitimate
no route, missing collision source failure and actual held-pool deadline. CUT
permission/path tests now supply their source explicitly and assert query success.
The simulator error test cannot return a successful not-found result when the tile
source is unavailable. Focused checks passed (1.4s world, 1.1s simulator), full
world (49.0s) and script-simulator race suites passed, all Go packages compile and
diff checks pass. Canonical `npm run tygo` produces no generated wire change.
The existing `seafoam_1f_pathfind_avoids_visible_boulder` CLI golden passed in the
canonical isolated runner at `/var/tmp/capturequest-script-sim.94FFGS`.

Remaining simulator boundaries include global/unowned fixture application and
initial/final `CaptureSnapshot` reads (including the final pathfinding snapshot),
and other action helpers still using background contexts. The options planner's
query boundary is owned; the entire simulator run is not yet a closed lifecycle
audit. Local debug collision reads and dormant trainer diagnostic wrappers remain
as inventoried. The original restore timeout and pre-command Repel click failure
remain unattributed; all five roadmap areas stay active. Next: migrate the shared
simulator snapshot/read model with coherent injected caller ownership before
claiming wider simulator cancellation, then finish diagnostic-only wrappers.
This checkpoint is local only, without push or deployment.

## Dormant actor path retirement and shared overworld cache installation (2026-10-08)

Repository-wide caller/write inventory confirms `RequestActorMove` was the only
writer of `actorPaths`, with no runtime caller. Its actor A* callback queue, timer
processing, `ActorPathState`, unconsumed `FindPath` wrapper and unused `TileExists`
API are removed. Ambient NPC wandering and owned scripted/player movement retain
their actual existing owners. Canonical `npm run tygo` regeneration removes only
the retired `ActorPathState` interface; no runtime consumer referenced it. The ASM
conversion guide no longer recommends these obsolete APIs and instead identifies
issued JSON actions, player step/routes and ambient simulation responsibilities.

The inventory also found a coherence defect after overworld invalidation: player
reads warmed unified key 9999, while wandering NPCs looked up their source-map key.
All those keys query the same `map_id IS NULL` corpus. One shared installation
primitive now publishes the same immutable collision/raw-foot maps to unified,
map 0 and catalog overworld keys. Startup and lazy installation both use it;
interior entries stay independent. Alias invalidation and revision fencing remain
authoritative, so no reader can publish an overtaken snapshot.

The real PostgreSQL regression warms the unified view and verifies an NPC on a
source alias can select its valid next step. After changing collision, invalidating
and reloading unified data, that NPC selects the new permitted step. All aliases
retain current raw-foot data while a held one-connection pool proves cached access
needs no additional SQL. The preload regression now removes the global database;
map IDs, actor rows and collision preload use the injected world database and
existing caller context. The global startup collision wrapper is retired.

Focused checks passed (1.8s). Full world (48.4s), script-simulator and server race
suites passed for the retirement/shared-installation change. After the startup
injection follow-up, final preload/collision checks passed (2.1s), all Go packages
compile, frontend typecheck and diff checks pass. No rendered NPC movement or
performance claim follows from the headless next-step/cache checks. No schema,
opcode, runtime asset family or production state changed.

Remaining: simulator/standalone character pathfinding context/global ownership,
local debug collision reads and dormant trainer diagnostic wrappers, plus the
other matrix rows and reconnect/idle recovery. The original restore timeout and
pre-command Repel click failure remain unattributed; all five roadmap areas remain
active. Next: finish the standalone/simulator pathfinding boundary using its actual
caller context and database, then retire diagnostic-only query wrappers in favor
of tests of the authoritative planner. This checkpoint is local only, without
push or deployment.

## Late peer transport read acceptance (2026-10-08)

Two isolated browser contexts now exercise the existing scene-owned actor read and
real peer spawn/despawn/movement packets. The observer holds a successful actor
snapshot containing the peer while that peer quits. Releasing it cannot resurrect
the peer. In the replacement case, the peer reloads its connection, authenticates
again and reenters the same character, then moves from (3,6) to (4,6). The observer
receives the new stream before the old snapshot is released; it must retain exactly
one peer at (4,6), without a rewind or duplicate. Server evidence confirms the
replacement character entered through session IDs 4 then 5.

The read must settle successfully within its existing transport timeout; the test
handles pending promise settlement explicitly rather than relying on an unhandled
rejection. It does not extend the production timeout or bypass actor projection.
Initial fixture failures used `objectType` instead of the test bridge's actual
`type` field, then attempted the blocked bed at map 38 (3,5). Canonical SQLite rows
confirm (3,5) has collision 0 and (4,6) collision 1; correcting the field and source-
verified movement target preserves the lifecycle assertions and adds an exact
single-peer assertion. No application workaround was needed.

Both final peer cases passed in 13.2s at
`/var/tmp/capturequest-rendered.APG0FW`. Existing newer-boulder-stream and scene-
retirement browser cases also passed in the preceding run at
`/var/tmp/capturequest-rendered.5z5IrU`. The three focused `ActorReadView` checks and
diff checks pass. This proves real browser transport and actor registry behavior,
not screenshot pixels or complete lost-notification recovery. Runtime source,
wire/schema/generated assets remain unchanged in this acceptance checkpoint.

The shared `ActorReadView` and despawn markers already cover this peer lifetime
case; no second peer-specific coordinator or guard was introduced. Remaining:
peer read omission/publication ordering without an observed notification,
reconnect/idle resident recovery, local/simulator/startup/dormant collision APIs
and the wider command matrix. The original restore timeout and the earlier
pre-command Repel UI click failure remain unattributed. All five goal areas stay
active. Next: review the remaining collision API inventory and retire dormant
paths or propagate explicit owners through real consumers before another family
migration. This checkpoint is local only, without push or deployment.

## Peer actor metadata joins one matching movement snapshot (2026-10-08)

The query-free actor constructor still combined immutable presence position/name
with separate bicycle, surfing and speed reads keyed only by character ID. A
session replacement could therefore supply the new registration's metadata to an
older presence value; ordinary movement could also overtake the captured position.
Facing used the entry-direction rule rather than current movement facing.

Immutable `session.Presence` now carries its publishing session ID. Actor
construction obtains one existing `playerMovementSnapshot` under the movement
lock, requiring matching session, map and coordinates. Retired registrations and
overtaken positions are omitted instead of mixing frames or guessing defaults.
Sprite, speed and facing come from that same snapshot. The existing presence and
movement systems remain authoritative; no shadow actor store or SQL is added.

The private PostgreSQL regression captures old presence, replaces the movement
registration with another session, and proves the old value cannot construct an
actor. Matching replacement state supplies its actual facing, speed and surfing
sprite together; movement overtaking the captured position rejects it. Closed
sessions still return empty presence through the existing session boundary. This
is registration/snapshot evidence, not a new real-transport retirement test.
Focused actor/presence/tick checks passed (1.7s), full world (64.2s), session and
script-simulator race suites passed, all Go packages compile and diff checks pass.
The combined browser run at `/var/tmp/capturequest-rendered.hk2YSo` passed the
ordinary Repel duplicate/reentry case and house exit, but its lost-reply case
captured no successful reply before the crash phase. Retained trace inspection
shows zero `RepelUseRequest` (143) frames and two `TrainerCardRequest` (150) frames;
the UI snapshot shows the Trainer panel. One bounded isolated reproduction passed
unchanged in 20.9s at `/var/tmp/capturequest-rendered.ALCfjk`, including verified
exit 137/restart generation 1 and reentry. The initial UI interaction failure
remains unattributed; the rerun does not explain it or close the original restore
timeout. This is entry/reentry evidence, not a late peer-transport regression or
proof of sprite pixel appearance.

Remaining: peer stream/read publication ordering during close/reentry, source
ownership across transport retirement, local/simulator/startup/dormant collision
APIs, reconnect/idle resident recovery and the wider command matrix. The original
restore timeout remains unattributed, and all five roadmap areas stay active.
Next: prove late peer actor reads cannot resurrect a retired/replaced registration
through the actual transport/client boundary, and retain the pre-command UI click
failure for bounded interaction diagnosis. This checkpoint is local only,
without push or deployment.

## Entry prepares surfing; actor presentation performs no collision SQL (2026-10-08)

`createPlayerActorFromPresence` previously inferred a surfing sprite by reading
collision whenever movement reported false. The same actor constructor serves
initial spawn, map/position publication and peer actor reads, so presentation could
start its own database work after entry or a gameplay commit.

The existing `restoreMovementRoute` entry operation now prepares surfing alongside
its owned saved map/coordinates and optional persisted route. Without a route, it
uses the existing transaction water predicate/query handle. Saved routes retain
their stored surfing decision and source validation. Read failure rejects entry
preparation; successful publication requires the matching session/registration,
saved position and live command context before installing path/surfing state.
Actor construction uses prepared movement state; its collision fallback is removed.
Committed movement/SURF already carries the subsequent surfing decision. No new
storage, polling or shadow presence service was introduced.

Private PostgreSQL checks cover land/water entry with a one-connection pool,
expected actor sprite fields, and actor construction while the pool is occupied
and the collision cache is cold. They verify no connection request during
presentation. Pool cancellation and missing source preserve the live movement
state instead of projecting guessed defaults. Focused actor/entry/route checks
passed (2.0s), full world (51.6s) and script-simulator race suites passed, all Go
packages compile and diff checks pass.

The normal entry/house-exit browser case passed (8.2s) at
`/var/tmp/capturequest-rendered.KaBqLk`; its two Repel cases were skipped because that
run did not enable crash mode. A separate correctly enabled crash run passed both
Repel duplicate/lost-reply/reentry cases in 25.5s at
`/var/tmp/capturequest-rendered.6hhy7F`. The verified restart receipt records old PID
2233163, new PID 2234027, exit 137 and generation 1, with process recovery evidence.
No restore/ResumeBattle/deadline-exceeded diagnostic matched the retained server
logs in this run. This rerun does not attribute or close the original restore
timeout. Browser entry/reentry evidence is distinct from the headless sprite-field
check; no rendered water-sprite pixel claim or production acceptance is made.

Remaining: peer presence/movement metadata consistency across session replacement,
local/simulator/startup/dormant collision APIs, batching under edits, reconnect/idle
resident recovery and the remaining command matrix. All five roadmap areas remain
active. Next: review peer actor snapshot ownership before another domain migration.
This follow-up is a local checkpoint only, without push or deployment.

## Warp eligibility uses one owned collision view (2026-10-08)

Normal warp activation already owns a bounded transaction, but adjacent carpet
eligibility called the independent collision convenience reader twice. A cold
cache could borrow another connection inside that transaction, and the mat/entry
checks could observe different cache generations. Eligibility now consumes one
immutable collision map supplied by the owning transaction/context. Its rules are
pure lookups; they no longer perform SQL or reload cached state. Door and standing-
on-mat rules retain their existing behavior without adding a tile query.

The adjacent carpet read uses the shared base-collision primitive and remains
local to the transaction. Source read errors reject the warp and roll back position
rather than being mistaken for normal ineligibility. Click and keyboard policies
use the same view, preserving direction, source-map and catalog authority.

Private PostgreSQL regressions use no global database and a cold cache. With a
one-connection pool, a blocked mat activates while a walkable mat rejects; missing
source leaves position unchanged. A real relation lock reaches the caller deadline
without borrowing another connection or committing position. The pool counter is
captured immediately after the operation: a later verification query can wait for
cancelled-transaction cleanup and must not be counted as eligibility I/O. Existing
rule checks were mechanically updated to supply their actual collision fixtures.

Final focused warp checks passed (2.0s), full world (56.1s) and script-simulator race
suites passed, all Go packages compile and diff checks pass. Two rendered routing
cases passed in 15.5s: the house exit chain and Underground Path Route 6 mat.
Evidence is retained at `/var/tmp/capturequest-rendered.sVUfM6`. This verifies
browser routing, not screenshot/pixel appearance or new process-death recovery.

Remaining: map-load/peer actor construction still reads collision to infer the surf
sprite; prepare that decision through the existing entry/map-load owner instead
of independent presentation SQL. Other local/simulator/startup/dormant API entries,
reconnect/idle recovery and command-family gaps remain as inventoried. The original
login timeout remains unattributed, and all five goal areas stay active. Next:
review entry/map-load surfing ownership and remove the collision read from actor
presentation. This is a local checkpoint only, without push or deployment.

## Owned water preflight reads (2026-10-08)

Fishing facing-water and targeted SURF preflight now require the caller context
and return read errors separately from ordinary ineligibility. Both commands pass
their session command context to the existing injected base-collision reader.
The live and transaction water helpers retain the same water/warp predicate;
there is no background-context water wrapper. The shared base reader preserves
its five-second maximum while respecting a shorter caller deadline. Missing cold
read storage returns an explicit source error instead of dereferencing a nil pool;
valid cached immutable entries remain usable by existing deterministic fixtures.

A failed read produces an explicit failure response rather than claiming that
water is absent or that nothing bit. Concise local debug diagnostics include map,
position/target and the cause; production logging stays quiet. No mutation retry
or new command coordinator is introduced.

Private PostgreSQL packet-boundary tests cover both commands with an actually held
connection pool and an unavailable source table. They prove deadline cancellation,
one failure packet, distinct read-error text, unchanged position, no battle and
no poisoned cache. Existing direction/water/warp-mat behavior is retained. Focused
preflight/fishing/SURF/collision checks passed (2.2s), full world (53.4s) and script-
simulator race suites passed, all Go packages compile and diff checks pass. No
rendered, process-death or production acceptance is claimed for this checkpoint.

This does not close the field-action family: source inspection shows
`fishingPlayerPosition` still accepts request `mapId`/`x`/`y` in place of the owned
position, rod selection trusts request identity, and later encounter/party reads
still use the global database. Those facts are recorded in the finite command
audit for source authorization and battle/read ownership work. Other live collision
caller gates are warp-entry coherence/cancellation and map-load surf presentation;
local/simulator/startup/dormant APIs remain as inventoried. The original login
restore timeout is unattributed, and all five roadmap areas remain active.
Next: migrate warp-entry and map-load collision reads through their existing owner
contexts before expanding another mutation family. This checkpoint is local only,
without push or deployment.

## Forced-step cancellation and committed projection (2026-10-08)

Forced-step preparation now passes its existing tick/character context to the
base collision reader, instead of using the background-context convenience API.
A failed read returns an error before the detached candidate can publish or
commit; the live source, path and surfing state remain unchanged. No new timer
or independent owner was added.

Inspection also corrected the caller inventory: `RegisterPlayer` performs no
collision SQL. The read was in `UpdateReportedPosition`, called after ordinary
step/SURF commit while holding the player mutex. That method is replaced by the
pure `projectCommittedPosition`, which receives the committed surfing decision.
The movement transaction returns that decision alongside position, using the same
water/warp rule already needed for route persistence; validated SURF entry remains
surfing until a committed teleport. Ordinary, forced and SURF projection consume
the same result. The obsolete reported-position method is retired, and its queued
same-tile/different-tile projection checks are retained. Durable receipt and client
wire formats are unchanged; duplicate receipts still acknowledge history without
replaying or rewinding live state.

Private PostgreSQL regressions occupy the actual connection pool: forced-step
collision preparation reaches the caller deadline and preserves live source/path,
while committed projection completes without any connection request and applies
the supplied surfing state. The route regression also checks the returned committed
surfing decision. Focused movement/player-step/SURF checks passed (7.0s), full world
(63.9s) and script-simulator race suites passed, all Go packages compile and diff
checks pass. No new rendered or process-death acceptance is claimed.

Remaining live caller gates are fishing/surf preflight, warp-entry collision and
map-load surf presentation, followed by the local/simulator/startup/dormant API
entries in the finite inventory. Reconnect/idle tile recovery and other command
rows remain open. The original restore timeout is unattributed and the full
five-area goal stays active. Next: use existing command/map-load contexts for the
remaining live reads, preserving errors separately from ordinary ineligibility.
This is a local checkpoint only, with no push or deployment.

## Collision caller inventory and transaction reuse (2026-10-08)

The finite collision consumer inventory is now recorded in
`SERVER_COMMAND_AUDIT.md`. It distinguishes live player admission, transaction
planning, background movement, warp/map presentation, local fixtures, simulator
ownership and dormant diagnostic APIs. The unused player `findPath` wrapper is
removed; no live gameplay route used it.

Two live transaction callers still crossed back into the independently bounded
base reader. Trainer planning during `commitMovementStep` used the cache helper;
when cold, it could try to borrow another connection while the movement transaction
held the only available one. Surf-route persistence made the same crossing through
`isSurfableWaterTile`. They now read through the existing base-collision primitive
using the caller's transaction and context. Trainer planning propagates unavailable
source errors rather than treating a failed cache read as a clear sight line. Water
checks retain the existing shared water/warp predicate, while the transactional
variant reports source/service failure. No alternate collision store or mutation
coordinator was introduced.

Real PostgreSQL movement regressions use a cold cache, one connection and no global
database. They prove clear sight selects a trainer, a wall prevents selection,
unavailable collision source rolls position back, and a surfing route persists.
The pool wait counter verifies these reads never request a second connection.
Focused trainer/movement checks passed. Full world (55.9s) and script-simulator
race suites passed for the trainer change; after the surf-route addition, the final
movement/trainer/water/issued-step race checks passed (7.8s). All Go packages compile
and diff checks pass. No rendered or process-death acceptance is claimed here.
This establishes another independent connection-ownership defect, not attribution
for the original five-second login restore timeout.

Remaining: background-context reads in forced movement, fishing/surf preflight,
registration, warp entry and map-load presentation, plus local/simulator/startup
and dormant API audit entries. Reconnect/idle tile recovery, other matrix rows and
the unattributed login timeout remain open; all five areas stay active. Next:
propagate existing command/tick owners through the live nontransactional collision
callers and verify cancellation without hiding read failure as ineligibility.
This follow-up is committed locally only; no push or deployment.

## Base collision cache ownership review (2026-10-08)

The runtime lazy loader held the shared actor mutex while querying collision rows.
A relation lock or slow SQL therefore blocked actor access and cache invalidation.
Runtime loading now stages complete immutable collision/raw-foot maps outside that
mutex, using the same row reader as private startup staging. Publication checks
caller cancellation and a conservative manager-wide collision revision before
replacing cache entries. Invalidated reads reject rather than publishing partial
or old data. Successful startup replacement also advances that revision.

The owned character snapshot captures the revision before opening its transaction:
otherwise an old snapshot could read flags before an edit, reach the cold base
cache after invalidation, and publish old rows under the new revision. Both cold
cache publication and completed snapshot delivery are fenced. Cold reads inside
caller-owned gameplay transactions remain local and cannot warm the shared cache
with uncommitted rows. Existing immutable cached base entries remain reusable;
this does not turn ordinary steps into full-world queries.

Overworld aliases (including map 0, unified map 9999 and catalog overworld IDs)
query the same `map_id IS NULL` tile corpus. Invalidation now removes all their
collision and raw-foot entries together while retaining interior caches. The
revision is intentionally conservative: an edit can retire another map's pending
read, which rejects and requires a new owned read instead of guessing freshness.

Private PostgreSQL checks hold a real relation lock, observe the blocked read,
and prove invalidation can proceed before SQL completes. They verify overtaken
publication rejection, fresh retry with changed collision/feet, cancelled cold
reads, old-snapshot rejection, transaction rollback without cache leakage, and
alias invalidation. Existing concurrent retry/preload failure tests still pass.
Focused race-enabled checks passed; full world (49.6s) and script-simulator race
suites passed, all Go packages compile and diff checks pass. No new rendered,
process-death or production acceptance follows from these headless checks.

Remaining: global/background collision/pathfinding callers, runtime cache batching
under sustained edits, reconnect/idle resident recovery and the remaining command
matrix. The original login restore timeout remains unattributed and all five areas
stay active. Next: finish the collision caller/lifetime inventory and address
remaining background-context reads before another family migration. This is a local
checkpoint only, without push or deployment.

## Collision rule snapshot alignment (2026-10-08)

Review of the remaining collision reader found a concrete divergence from tile
presentation: `characterCollision` selected event rules with the caller's cached
flags, then read raw-foot metadata for every eligible rule. A stale flag cache
could therefore choose different collision from presentation; a superseded rule's
missing metadata could reject an otherwise valid final selection.

Database-backed collision reads now use the existing bounded `db.ReadSnapshot`.
Calls made inside an owned movement transaction retain that transaction, including
its uncommitted flag changes. Player collision selection loads durable flags and
uses the shared `eligibleEventTileOverrides` last-eligible policy, then resolves
raw-foot metadata only for selected rules. Missing winning metadata still rejects,
with map, coordinate and image identity in the error. Existing base collision,
CUT overlays, boulders and NPC blockers retain their responsibilities. Unused
global event collision/raw-foot wrappers and their unused raw-foot helper are
removed; the separate tile-mutation property helper remains for its actual caller.

Private PostgreSQL regressions prove stale-negative and stale-positive cache
rejection, superseded metadata exclusion, missing winning metadata failure,
transaction-local flag visibility, rollback without collision publication, and
actual connection-pool wait cancellation. The focused race-enabled checks passed.
World (69.8s), server and session race suites passed in the broader run; that command
failed overall because its extra `internal/simulator` target does not exist. The
correct full `internal/scriptsim` suite subsequently passed, and all Go packages
compile. No new rendered movement, process-death, deployment or production evidence
is claimed. Source changes were reviewed with `git diff --check`.

Remaining: shared base-collision cache lifetime and lock/I/O ownership, remaining
global/background pathfinding callers, reconnect/idle resident recovery and the
other open matrix rows. The original restore timeout remains unattributed; all five
roadmap areas remain active. Next: audit the base collision cache's read/invalidator
ownership before another family migration. This checkpoint is local only, without
push or deployment.

## Resident recovery acceptance and shared admission review (2026-10-08)

The private-database omitted-update regression now covers both interior and unified
views. Each case captures an actual resident tile, erases its source row without
sending a notification, proves its renderer and player collision entries remain,
and invokes the existing scene reconciliation callback. Both entries must then be
absent. Unified selection uses the runtime chunk-size constant and the player's
required chunk. This proves browser renderer/collision state, not screenshot pixels
or automatic idle/reconnect recovery. The test uses only the runner-owned database;
no production database or process-death claim is involved.

Boundary review found fresh recovery reads bypassing the stream's global network
queue, so recovery could exceed its two-request ceiling alongside camera or explicit
reads. Fresh recovery now uses that same queue while bypassing cached/pending
snapshots. Queued cancellation removes the owned work; active reads retain their
signal. A focused regression occupies both slots, checks recovery stays queued,
and proves both successful eventual admission and removal on cancellation.

41 focused chunk/cutscene/map-loader checks, typecheck, canonical asset validation,
and the production build passed (Vite 3.36s). Both final browser cases passed in
6.2s at `/var/tmp/capturequest-rendered.j9T62y`; preceding browser evidence is at
`/var/tmp/capturequest-rendered.DWDY2O`. The earlier failing test used the actor
controller for player collision inspection; correcting the actual owner preserved
both assertions. The finite command audit is updated, and its completed healing,
PC and field-command migrations are no longer described as future work.

Remaining: other owned settlement points and idle/reconnect missed notifications,
sustained revision churn and legacy collision/publication owners. The original
login restore timeout remains unattributed; the full five-area goal remains active.
Next: review reconnect/scene ownership and the remaining legacy collision readers
before expanding to another command family. This follow-up is a local checkpoint;
no additional push, deployment or production mutation is authorized by continuation.

## Unified resident chunk recovery checkpoint (2026-10-08)

The existing cutscene settlement callback now reconciles both interior and unified
resident tiles before releasing its input lock. Unified recovery stays inside
`OverworldChunkStream`: it rereads the required bounded gameplay footprint rather
than loading the entire world or using the interior renderer. Fresh reads bypass
cached chunks, stage tiles/images, check revisions and owner generation, and then
replace renderer and collision residency together. The previous resident view
remains until staging succeeds. Recovery rejects if superseded; revision retries
are bounded. Both caller cancellation and stream retirement abort pending reads.

39 focused chunk, cutscene and map-loader lifecycle checks passed, including fresh
reads, collision replacement, caller cancellation and stream retirement. Typecheck,
canonical runtime asset validation and the production build passed (Vite 3.32s).
The interior omitted-update browser regression also passed (2.8s), verifying the
renderer registry through the shared scene callback at
`/var/tmp/capturequest-rendered.7iWPc9`.
Eleven normal scripted-event/warp browser cases passed in the preceding check at
`/var/tmp/capturequest-rendered.OP1twB`. These checks do not establish pixel-level
appearance or an omitted-update fault in unified chunks.

Remaining: a unified-chunk omitted-notification browser regression, other owned
settlement points, idle/reconnect missed notifications, sustained revision churn,
and legacy collision owners. The original five-second `restoreBattleOnLogin`
timeout remains unattributed. All five roadmap areas remain active; this checkpoint
is a stopping point, not goal completion. Next: prove unified missed-update recovery
through the rendered runtime before expanding to another command family. The user
has authorized pushing this stopping checkpoint to `codex/server-foundations`;
production deployment remains unauthorized.

## Resident interior reconciliation at cutscene settlement (2026-10-08)

Cutscene completion already owns an input lock through correlated commit/position
reconciliation. That existing callback now also carries its AbortSignal and reads
the resident interior tile view before unlocking. The scene validates map/load
identity, character and retirement, prepares required images and requires a current
tile-read stamp. A retired or overtaken view rejects rather than reporting successful
recovery. Changed/missing coordinates are projected through the existing world-tile
update path, including explicit erases; actors and resource/movement coordinators
retain their own responsibilities. No polling loop, broker, new notice stream or
inventory-specific executor was introduced.

The interior read compares authoritative rows with the actual live tile lookup,
so a lost erase event can be recovered even when the initial scene array is older
than streamed updates. Unified chunks deliberately retain their separate bounded
stream owner and are not loaded as one enormous interior array. This is the first
owned settlement integration, not universal idle/live world synchronization.

17 focused cutscene/sprite/tile lifetime checks passed, preserving position values
and now asserting signal delivery. Existing failure tests keep input locked. Four
normal guest/scripted/texture-order browser cases passed in 20.6 seconds. A separate
private-database fault case erases a real resident source row without sending any
event, proves it remains in the renderer, invokes the same scene-owned reconciliation
and verifies removal. That and both scripted-event cases passed in 18.4 seconds in
`/var/tmp/capturequest-rendered.6V3YP5`. This proves the browser mutation registry,
not screenshot/pixel appearance. Typecheck, canonical asset validation, production
build (Vite 3.30 seconds) and diff checks passed. Evidence is retained under
`/var/tmp/capturequest-tile-recovery-*`. No backend/schema/runtime-asset contract
changed; no new process-death or production claim follows from these checks.

Remaining: unified resident chunks, other settlement points and idle/missed shared
world notifications, sustained churn/revision scope and legacy collision owners.
The original restore timeout is unattributed and all five roadmap areas remain
active. Next: review this owned callback boundary and add bounded unified-chunk
reconciliation through its existing stream owner. This checkpoint is local only,
without push, deployment or production mutation.

## Explicit erased-base event publication (2026-10-08)

The actor-cache interaction audit confirms `ActorReadView` captures the scene's
independent actor cache/despawn markers, not the cleared tile snapshots. The
publication audit found a concrete remaining omission: with no eligible override,
an explicitly erased base row was excluded by the base query, treated as missing
and never published. A client could retain an older override at that coordinate.

The publisher now distinguishes an existing `is_tile_erased=1` row from a genuinely
absent source record, within its authoritative snapshot. The erased case produces
an explicit erased tile state and uses the existing broadcast erase field. Missing
source or required properties still reject with no partial result; neither becomes
a guessed removal. The simulator's tile-state output preserves the same marker.
No generated image, coordinate or tile-ID identity changed.

The PostgreSQL regression verifies the state reader and actual packet carry erase,
then deletes the source row and proves that missing data remains an error. Existing
stale-cache, priority and missing-properties checks remain; their fixture restores
the base row before testing property failure, so the producing layer is isolated.
Full world/simulator race suites passed in 48.015 and 1.069 seconds. Final focused
checks passed in 1.150 seconds for world; simulator compiled successfully. Canonical
Tygo, typecheck, asset validation, production build (Vite 3.33 seconds) and diff checks
passed. Evidence is retained under `/var/tmp/capturequest-event-erase-*`. Existing
browser erase handling is reused; no new rendered/pixel or production acceptance
is claimed here.

Remaining: general resident-view missed notification recovery, publication notices/
revision ordering, sustained churn, legacy collision owners and dynamic previous-
map recovery. This fixes explicit erase omission, not all live missed updates.
The original restore timeout is unattributed and all five roadmap areas remain
active. Next: define current-view reconciliation at existing owned command/read
settlement points before closing world-presentation coverage. This checkpoint is
local only, without push, deployment or production mutation.

## Fresh tile-view ownership discards missed-update caches (2026-10-08)

The chunk audit found the eighteen-entry exact cache returning retained arrays as
current data on later map/view loads. Its local chunk revisions change only when
a notification arrives, so they cannot prove no update was missed while the view
was away. `beginOwnedTileView` now advances the shared tile-read revision and clears
retained map snapshots and exact chunks when either interior or unified loading
starts, after the previous stream is stopped. Ordinary camera plans retain their
existing bounded cache; a new owner begins with authoritative reads. Full cache
clearing uses the same boundary, so an older pending read cannot repopulate it.
No new receipt, event log or polling mechanism was introduced.

The regression seeds both map and exact-chunk caches, starts a read, replaces its
view and verifies the caches are gone and its older reply cannot settle as the
new current view. The shared read performs a new correlated read. Loader stubs were
updated for the required owner method while preserving cancellation/failure assertions.
38 focused lifetime/cache/loader/chunk checks and typecheck passed. Eleven browser
initial-publication, guest/reentry and door/stair/gate cases passed in one minute
in `/var/tmp/capturequest-rendered.yp2Dry`. This reuses the exact private-database
fixture mode; it is not process-death evidence. Production build and canonical
asset validation passed (Vite 3.37 seconds); logs are retained at
`/var/tmp/capturequest-tile-owner-build.log`. Diff checks passed. No backend, schema or runtime-asset contract changed.

Remaining: live missed notifications while a view remains resident, character/
session retirement beyond normal map transitions, read/cache/stream revision
scope, sustained churn, missing/erased base publication and other legacy collision
owners. Reentry refresh does not prove live missed-update recovery. The original
restore timeout is unattributed and all five roadmap areas remain active. Next:
review owner reset against actor-cache consumers and define live current-view
reconciliation through existing reads before closing world-presentation coverage.
This checkpoint is local only, without push, deployment or production mutation.

## Initial map rendering revalidates its tile view (2026-10-08)

Initial interior loading could retain a tile array across image, item and actor
preparation, publish that array, wait 300ms and render it after a newer committed
update. Successful tile arrays now carry weakly held read-revision stamps. The
loader revalidates immediately before the main snapshot/state/render/collision
publication. Overtaken arrays are reread and any new images prepared under the
same abort signal, with two bounded preparation attempts. Continuing churn fails
rather than publishing old data. A final stamp check covers the async return gap.
The fixed wait is retired; actor read-view preparation runs at final publication.
An unreachable nullable-actor fallback, inconsistent with the typed actor reader,
was removed. World-tile observation is explicitly installed before loading and is
separate from later editor-input registration.

The browser regression captures a real returned source tile, holds initial image
preparation, erases that exact row in the isolated test database, delivers its
committed-update event and releases preparation. It requires a second tile read
and verifies the eventual live-renderer registry excludes the row. An initial
failed run exposed revalidation mistakenly placed in the old nullable-actor branch;
that placement was corrected in the actual publication path, not by weakening the
assertion. Earlier listener-registration interpretation was not established as
that failure's cause.

SQL fixture access uses the runner's exact private database/server identity guard
with `CQ_E2E_DATABASE_FIXTURE=true`. This does not opt into process-death acceptance;
the crash lane still requires an actual restart and recovery evidence. Three
browser cases passed in 8.5 seconds in `/var/tmp/capturequest-rendered.dh76UE`,
including initial publication, delayed texture/update ordering and normal guest
creation/entry. 33 focused view/lifecycle/chunk checks and typecheck passed. Production build
and canonical asset validation passed (Vite 3.41 seconds); logs are retained at
`/var/tmp/capturequest-tile-init-build.log`. Shell syntax and diff checks passed. No backend, schema or runtime-asset contract
changed, and no screenshot/pixel or production claim follows from registry checks.

Remaining: sustained revision churn/batching, missed notifications/reconnect,
whole-map versus chunk revision policy, missing/erased base-state publication and
other legacy event/collision owners. The original restore timeout is unattributed
and all five roadmap areas remain active. Next: audit missed update recovery and
cached chunk revisions before closing this presentation family. This checkpoint
is local only, without push, deployment or production mutation.

## Client tile update/view ordering (2026-10-08)

The scene's world-tile handler applied every incoming map to its current renderer
and let an older texture promise add a tile after a newer erase/paint. It now records
cache invalidation first, then requires the current map/load-generation view before
presentation. Per-coordinate active texture tokens suppress older completions;
completed tokens are removed and scene shutdown clears them. Other-map updates can
invalidate metadata without painting the current map. Unified-map identity uses
the loaded map's explicit overworld property, not a newly invented ID range.

`MapDataService` invalidates its bounded interior snapshots on committed updates
and fences an in-flight tile read by a local update revision. An overtaken read
retries once through shared correlation; continuing churn rejects rather than
publishing older tiles. Interior arrival reads refresh tiles instead of reusing a
cached character projection. Existing chunk revision/generation guards and streamed
collision/lookup application remain. No mutation is retried, and no additional
networking or inventory coordinator was introduced.

31 focused checks passed across read lifetime, loader lifecycle and chunk stream,
including snapshot invalidation/overtaken-read retry and map/load-generation leases.
Typecheck passed. Three browser cases passed in 21.8 seconds in
`/var/tmp/capturequest-rendered.joBaP7`: guest entry/reentry, movement recovery and
an instrumented live-renderer ordering check. The latter captures a real source tile,
holds its texture promise, sends a newer erase and verifies its renderer registry
stays removed; it also verifies another-map updates leave the current registry
unchanged. This proves the browser mutation boundary, not a pixel/screenshot claim.
Production build and canonical asset validation passed (Vite 3.32 seconds); logs
are retained at `/var/tmp/capturequest-tile-order-build.log`. Diff checks passed. No backend, schema or generated asset
contract changed.

Remaining: initial map-render/image-preparation versus updates, read/cache/stream
revision scope and batching under sustained changes, missed notifications/reconnect,
other legacy event/collision owners and dynamic previous-map recovery. The original
restore timeout remains unattributed and all five roadmap areas remain active.
Next: review the larger initial-render publication window before closing world-
presentation coverage. This checkpoint is local only, without push, deployment
or production mutation.

## Event-tile publication uses the same authoritative priority (2026-10-08)

Publication tracing found `currentEventTileState` returning the first eligible
ordered rule, while tile reads and collision maps selected the last. The publisher
also used cached flags and global/background map, rule, property and base-tile
reads, with error paths that silently skipped states or guessed the session map.
`eligibleEventTileOverrides` now expresses the established last-eligible policy for
both the read projection and publication. Ordered output deduplicates coordinates
without changing that policy.

Published states now read map identity, committed flags, rules and required
properties/base tiles in one `db.ReadSnapshot`. Failed named-map lookup does not
fall back to the session map; valid map ID zero is no longer discarded. Failure
returns no partial state list or packet. The simulator explicitly supplies its
fixture database/context through the same state reader. The sole remaining caller
of the removed base-tile wrapper was CUT's preflight; it now uses the injected
snapshot read. Other field-action policies/writers remain their own audit.

The regression disables the global pool, uses one connection and deliberately
stales cached flags. Two eligible rules at one coordinate must select the same
final image/properties in the tile reply, state reader and actual publication.
A missing named map emits no guessed update. Missing required properties retain
map/coordinate/image errors. Focused world checks passed in 3.664 seconds, followed
by full world/simulator race suites in 46.464 and 1.069 seconds. Evidence is retained
under `/var/tmp/capturequest-event-publication-*`. Four rendered multiplayer,
private-puzzle and scripted-event cases passed in 43.2 seconds in
`/var/tmp/capturequest-rendered.MD64ro`. The runner compiled the changed backend
and stopped its private runtime. Diff checks passed. No frontend, schema, wire or generated asset changed.

Remaining: client cached-view/read versus streamed-update ordering, missing/erased
base-state policy, other legacy collision/publication query owners and broader
field-action migration. The original restore timeout remains unattributed, and
all five roadmap areas remain active. Next: audit the client tile update/cache
boundary before closing world-presentation coverage. This is a local checkpoint,
without push, deployment or production mutation.

## Tile event projection joins the authoritative read snapshot (2026-10-08)

The projection review found another global/background dependency after the tile
query: `ApplyEventTileOverridesToTiles` loaded override rows and image metadata
outside the owned read and decided eligibility from the shared flag cache. An
override-read error returned unchanged base tiles as successful content; image
property errors were likewise defaulted by the legacy wrapper.

The tile handler now uses the existing `db.ReadSnapshot` for the whole projection.
After consuming/closing its tile cursor, it loads committed character flags,
ordered override rules and required image properties through that same bounded
read-only snapshot. Existing flag-eligibility rules are reused. Property lookups
are shared per distinct applied image within the read. Failures return no tile
catalog, with map/coordinate/image identity for required metadata failures. The
old presentation-only fallback function is retired; independent legacy collision/
publication wrappers remain separate audit items. Projection-disabled fixtures
retain that explicit configuration; live handlers have their existing event
manager enabled. No data rows, generated IDs or image URLs were changed.

A real PostgreSQL regression disables the global database, limits the pool to one
connection and seeds an override with stored flag/image properties. Deliberately
stale negative and positive flag caches cannot change the returned projection.
Deleting required properties rejects rather than returning base tiles. Focused
checks passed in 1.304 seconds, followed by world/database race suites in 49.263
and 1.823 seconds. Logs are retained under
`/var/tmp/capturequest-tile-projection-*`. Eleven rendered multiplayer/private-
puzzle and warp cases passed in 1.5 minutes in
`/var/tmp/capturequest-rendered.91li1I`. The isolated runner compiled the changed
backend and stopped its private runtime. Diff checks passed; no new frontend
build is required for this backend-only boundary change.

Remaining: review cached tile views and event publication/revision ordering against
this authoritative projection, plus other legacy event/collision query owners and
previous-map recovery. The shared request timeout policy retains its tested 10/30-
second values; no additional timer or retirement mechanism was introduced here.
The original restore timeout is unattributed and all five roadmap areas remain
active. Next: audit publication and cached-view ordering before closing the world-
presentation family. This checkpoint is local only, without push, deployment or
production mutation.

## Active tile reads share correlation and owner cancellation (2026-10-08)

Tile reads had per-service `tiles-N` IDs, shared listeners and a separate promise/
timeout implementation. Two service instances could mint the same ID. They now
use `correlatedRequest`'s global sequence, single settlement and cleanup, with an
explicit timeout argument preserving the existing 10-second interior/30-second
unified-overworld policy. Send rejection settles immediately. The old per-instance
counter, detached tile timer and raw-array response acceptance are retired.

Tile replies have typed success/request/map/character identity and pagination.
The client validates its view, pagination and array payload before caching image
IDs. Interior loaders pass their map abort signal; chunk streams own a controller
that is aborted on stop and renewed on initialize. Deliberate stale/retired plans
return quietly instead of logging an abort as a new loading failure. Cached chunk
revision/generation guards remain. The runtime-asset contract check and versioned
tile URL helper are unchanged.

The server requires a correlated selected-character read, uses the injected pool
and command context, rejects scan failures instead of skipping malformed rows,
and closes the result before projection. Empty successful chunks remain arrays.
Old array fixtures were migrated to the explicit wire contract while retaining
paint/erase/provenance/paging checks. One stale overworld-list expectation from the
preceding retirement was also corrected to explicit rejection; its presence/global-
database invariants remain. Matched frontend/backend rollout is required.

Regressions prove distinct IDs across service instances, overlap isolation,
abort/late-response cleanup, immediate send-failure cleanup, 30-second timeout
policy, stream-stop cancellation without error logs and real injected-pool wait
cancellation. 61 focused client checks and typecheck passed. The corrected full
world race suite passed in 50.415 seconds; database/content suites passed in 1.812
and 12.424 seconds in the preceding broad run, whose only failure was the obsolete
retired-list expectation. Evidence is retained under `/var/tmp/capturequest-tiles-*`.
Eleven rendered guest/interior/overworld/warp/reentry cases passed in 1.4 minutes
in `/var/tmp/capturequest-rendered.y6I7f5`. The production build and canonical asset
validation passed (Vite 3.37 seconds); logs are at
`/var/tmp/capturequest-tiles-build.log`. Diff checks passed. No new production
availability claim follows from these local checks.

Remaining: review the shared timeout policy and caller retirement before expanding;
auditing authoritative event-flag tile projection, cached-view revisions, world
query source ordering and dynamic previous-map recovery remains open. The original
login restore timeout is unattributed, and all five roadmap areas remain active.
Next: review and exercise the tile boundary's fault/scene races, then audit its
flag projection against authoritative storage. This checkpoint is local only,
without push, deployment or production mutation.

## Unused overworld-list retirement and active tile audit (2026-10-08)

The complete caller search found no consumers of `fetchOverworldMaps`, its network
request/subscription, or the old map REST facade. The shared catalog is the active
metadata provider. The unused network list, service query, subscription/dispatch,
detached timeout helper and REST exports are retired. Opcodes 38/39 remain reserved
and return explicit unsupported/reload rejection without a database dependency.
Active individual-map information and unified-overworld tile bounds remain in the
content service. Their projection, cancellation and retry tests remain; only tests
for the removed unused list were removed with its implementation.

Focused world/content checks passed in 1.264 and 1.412 seconds, including the
reserved-list rejection with both database handles unavailable and the active map
catalog checks. 46 existing client checks passed across map requests, tile responses,
chunk cache and shared catalog loading. Canonical Tygo, typecheck, asset validation,
production build (Vite 3.36 seconds) and diff checks passed. Logs are retained under
`/var/tmp/capturequest-overworld-retirement-*`. No new rendered or production
acceptance is claimed. No extractor, schema or runtime-asset contract changed.

The active tile reader is different and remains open. Source tracing found
`HandlePhaserTilesRequest` using global `Query` without its command cancellation;
`requestTileBatch` has a separate promise/timer and no AbortSignal. Request IDs are
`tiles-${++this.tileRequestSequence}` with a per-instance counter, so different
MapDataService instances can mint the same ID while sharing response listeners.
Raw legacy arrays are still accepted without correlation. These are source-derived
risks to verify through a focused collision/retirement regression, not a claimed
production incident or attribution for the original login timeout.

Next: migrate active tile reads through shared correlation with explicit interior/
overworld timeout policy, viewer/map validation, request retirement and injected
storage; review event-flag projection at that boundary. Preserve canonical tile
IDs/URLs and chunk ownership. The broader roadmap and original restore investigation
remain active. This checkpoint is local only, without push, deployment or production
mutation.

## Snapshot review and unused map-music query retirement (2026-10-08)

The shared snapshot review now includes an attempted INSERT through its repository
handle. PostgreSQL rejects it, no row persists and the helper returns no partial
value. Existing commit/publication and nested-query deadline checks remain. The
helper's missing-database error is named for the shared read boundary rather than
its former content-only location.

The complete caller search found no calls to `requestMapMusic` or `onMapMusic`.
Actual playback is `AudioService` -> `musicTrackForMap` -> the canonical generated
`pokemon_audio_manifest.json`. The extractor/schema and pipeline documentation
confirm that browser manifest/audio lane. Instead of building another query
framework around an unused path, the old global/background database handler,
FIFO request, result type, subscriber collection and response dispatch are retired.
Opcodes 66/67 remain reserved; the registered legacy handler returns explicit
unsupported/reload rejection without reading any database. Imported source music
metadata, generated manifests and the actual playback path remain unchanged.

The reserved-query regression disables the global database and supplies no runtime
pool, then verifies rejection. Four existing source-manifest music lookup checks
passed. Focused world/database race checks passed in 1.021 and 1.199 seconds,
including dispatcher and snapshot regressions. Canonical Tygo, typecheck, runtime
asset validation, production build (Vite 3.48 seconds) and diff checks passed.
Evidence is retained under `/var/tmp/capturequest-music-retirement-*`. No new
rendered/listening or production audio claim follows from lookup/build checks.

Remaining world-presentation work includes overworld/tile read lifetimes, actor
publication/cached-view audit and dynamic previous-map recovery. Broader account,
transport, script writers and the original unattributed restore timeout remain
open; all five roadmap areas remain active. Next: inspect overworld-map and tile
read cancellation/correlation through the established content and request layers.
This checkpoint is local only, without push, deployment or production mutation.

## Warp presentation reads through the shared snapshot boundary (2026-10-08)

Warp queries used the global database and resolved `LAST_MAP` destinations while
the catalog cursor remained open. That nested query needed another pool connection,
and scan/resolution errors were silently omitted from a partial reply. The existing
content snapshot primitive is mechanically centralized as `db.ReadSnapshot`; content
aggregates and warp reads share its injected, bounded, repeatable-read transaction.
Its query wrapper also binds legacy `QueryRow`/`Query` calls to the owned deadline.
Warp rows are fully consumed and closed before dynamic resolution, using that same
transaction and the existing resolver. Any scan/resolution/commit failure returns
no partial catalog. Empty successful catalogs are arrays. Eligibility filters,
imported coordinates and normal-warp activation policy remain unchanged.

Warp replies now carry request/map/character identity and explicit typed arrays;
this reflection reply and global array normalization are retired. The client uses
`correlatedRequest`, validates its view, and accepts the MapLoader abort signal in
both interior and unified-map paths. Timeout is rejection, not an empty list;
map loading delegates failure to its existing error boundary. Resolved warps are
fetched on each arrival rather than reused from a map-only cache, because their
previous-map context can differ on return to the same map. No inventory coordinator
or additional read executor was introduced.

PostgreSQL regressions disable the global database and resolve two viewers' dynamic
destinations on a one-connection pool, prove pool-wait cancellation, reject an
unresolved destination without partial success and preserve empty arrays. Existing
aggregate publication/commit-failure checks exercise the centralized primitive.
43 focused client checks passed, including overlapping warp reads, abort/late reply,
timeout and existing actor/map request checks. Full database, content and world
race suites passed in 1.619, 12.165 and 49.412 seconds. Ten rendered door/stair/gate,
movement-recovery and reentry cases passed in 1.3 minutes in
`/var/tmp/capturequest-rendered.N6Hqit`. The additional database regression passed
(1.136 seconds), proving a nested contextless pg_sleep query stops at its 50ms
owner deadline and returns no partial value. Production build and canonical asset
validation passed (Vite build 3.40 seconds). Logs are retained under
`/var/tmp/capturequest-warps-*`; diff checks passed. Canonical Tygo and typecheck passed; no extractor/schema/runtime
asset contract changed.

Remaining: review the shared read primitive before another migration; audit dynamic
previous-map context across fresh entry/same-map changes, visibility/catalog/source
ordering and warp-read fault acceptance. Other presentation queries, especially
music's global background read, remain open. The original login restore timeout
is still unattributed; fixing this independent nested-query defect does not prove
its cause. Next: review snapshot deadline behavior and remaining world-presentation
reads. The full five-area goal remains active. This checkpoint is local only,
without push, deployment or production mutation.

## Catalog retirement review and rendered replacement acceptance (2026-10-08)

Review found `close()` notifying retirement observers while `isConnected` was
still true. A synchronous observer could attempt a read through the connection
being cleaned up. Close and connection-attempt admission now mark the connection
unavailable before notifying read owners. The regression verifies observers see
that state and that one throwing observer cannot prevent notification/cleanup.
No additional retirement coordinator or observer layer was added.

Rendered acceptance holds a catalog reply while the browser remains pending,
then replaces its account on the same WebSocket or closes that exact test socket
and lets the ordinary reconnect path establish another connection. An older
modified reply is injected during the next read and cannot publish its catalog.
Both paths proceed through character creation and entry. The account case then
switches back to the original account and verifies the new character is absent,
proving actual account replacement rather than only a local generation change.
These two cases and the existing timeout/explicit-retry case passed in 21.9 seconds
in `/var/tmp/capturequest-rendered.CvVfmt`. Seven focused client checks and typecheck
passed. Production build and canonical asset validation passed (Vite build 3.34
seconds); evidence is retained at `/var/tmp/capturequest-catalog-retirement-build.log`.
Diff checks passed. No backend, schema, wire or asset
contract changed in this review; prior backend snapshot/correlation evidence is
reused rather than rerunning unrelated suites.

The static/creation read-family coverage now includes authenticated admission,
injected coherent storage, complete typed replies, correlated overlap, safe
read retry, timeout, cancellation, explicit UI recovery, coalescing, late-view
suppression and real account/connection retirement. Transactional gameplay
rollback, mutation duplicate receipts and process-death reward recovery are
inapplicable: these endpoints only read a read-only snapshot and write no player
state. Catalog content is application-wide, so one screen's unmount does not retire
other observing consumers. Matched wire rollout and legacy-request rejection are
recorded in the preceding checkpoints. This closes the selected read-family audit;
it does not close the broader account/transport audit or prove production rollout.

The original restore timeout remains unattributed and all five roadmap areas
remain active. Next: audit the remaining world-presentation query contracts and
read ownership against the same primitives. This is a local checkpoint; no push,
deployment or production mutation is part of this continuation.

## Correlated catalog reads and shared client lifetime (2026-10-08)

Both catalog endpoints now require and echo `requestId`, including rejection/read
errors. The client uses the existing `correlatedRequest` settlement primitive and
the ordinary response dispatcher; this family's opcode/FIFO `sendJsonRequest` path
is retired. Late replies cannot settle another read, and timeout/abort removes its
listener. The store coalesces static and creation callers onto one real promise
and publishes their complete catalog together. Creation-first and static-first
loads supply the same graph. Loading flags are published only after the shared
promise exists, so synchronous observers cannot receive a placeholder settlement.

The socket exposes a read-owner generation and retirement subscription. Connection
attempts, close and the existing JWT authentication request path retire that owner;
subscriber errors cannot prevent transport cleanup. The catalog store binds once,
aborts pending work and invalidates its metadata on retirement. Flight identity,
generation and abort checks prevent an older completion from overwriting a new
catalog or its loading state. The catalog belongs to the application connection,
so unmounting one observing screen does not cancel other consumers' shared read.
`initializeMaps` now follows the current authoritative catalog instead of trusting
its own populated map view; unavailable source metadata clears that view.

The loading gate no longer automatically loops failed reads. Its existing loading
screen and ActionButton expose an explicit Retry action. A rendered fault test
holds the first reply until timeout, verifies one read and a visible Retry button,
then injects an older modified catalog during retry. The old data is ignored; the
current catalog supports creation and world entry. This and the normal guest flow
passed in 17.5 seconds in `/var/tmp/capturequest-rendered.Z9Kflt`. Final map-view
refinement is covered by its focused state test, not a new visual claim.

Seven client regressions passed, including overlap/correlation, timeout/abort,
coalescing in both directions, noncooperating late completion after retirement,
socket close/auth generation and map-view replacement. PostgreSQL endpoint checks
cover both request IDs, correlated failures and legacy request rejection. Full
world/content race suites passed in 48.197 and 12.101 seconds. Typecheck, canonical
Tygo regeneration, asset validation, production build (3.37 seconds) and diff checks
passed. Logs are retained under `/var/tmp/capturequest-static-correlation-*`.

Remaining: rendered transport reconnect/account replacement during a pending read,
shared retirement-hook review and the wider account/transport family. This remains
a verified implementation baseline rather than a closed whole-roadmap claim.
The original restore timeout is unattributed, and all five areas remain active.
Next: review the new shared retirement boundary and exercise those actual transport
races before expanding to another family. This checkpoint is local only; no push,
deployment or production mutation is part of this continuation.

## Static/creation content aggregate and typed replies (2026-10-08)

The old static loader used process-global `sync.Once`, the global database and a
second generic cache. A cancelled first load permanently poisoned subsequent
requests. Faction/map/city query failures were printed and converted into partial
success, and row-iteration errors were not checked. Both static and creation
queries now use the existing content service's `readSnapshot`/`collect` primitives:
one injected, bounded, repeatable-read snapshot, arrays for empty lists and no
partial result on failure. A failed read can be retried. The global loader, once
state, generic-cache path and duplicate scanners are retired; staticdata now holds
the shared projection types. No new cache, loader framework or content coordinator
was added. These small metadata lists are read for each request.

Explicit camelCase JSON tags and a typed `StaticDataResponse` replace this family's
`StructToMap` calls. Canonical Tygo generation produces `staticdata.ts` and the
response contract. Both endpoints return the same complete snapshot, including
maps; the creation consumer uses its existing class/faction/city subset. The client
uses generated reply types, validates required lists/map fields, and retires snake-
case aliases and missing-list defaults. Its store no longer declares failed fetches
a loaded empty catalog, and the detached five-second Promise.race timer is removed;
the existing transport timeout owns settlement. Matched frontend/backend rollout is
required because the creation response now has the shared complete contract.

PostgreSQL regressions disable the global database, cancel a real initial pool
wait, retry successfully, force a mid-aggregate map-query failure, restore the
source and verify a fresh read. Empty lists remain arrays. The client regression
proves failed-load state remains retryable and a successful retry publishes data.
Full content/world race suites passed in 12.222 and 52.627 seconds; typecheck passed.
The rendered guest creation/entry/reentry flow passed in 4.4 seconds in
`/var/tmp/capturequest-rendered.HXTXkB`. The production build and canonical asset
validation passed (Vite build 3.37 seconds); logs are retained at
`/var/tmp/capturequest-static-build.log`. Diff checks passed. No extractor, schema or generated runtime-asset contract changed.

Remaining: this family still uses the legacy opcode/FIFO `sendJsonRequest` path.
Correlation, read cancellation, stale/late replies, concurrent-store admission and
account/screen retirement must be reviewed through the shared request primitive
before closing the row. The original restore timeout remains unattributed and
all five roadmap areas remain active. Next: review this content boundary, then
finish the static/creation read lifetime. This is a local checkpoint; no push,
deployment or production mutation is part of this continuation.

## Shared account-status authorization and slash corpus (2026-10-08)

The production registration inventory contains exactly one slash command:
`/help`. There are no registered slash mutation handlers to migrate. Prior notes
about potential slash writers were audit questions, not evidence of such writers.
Help listing and generic minimum-status admission shared `getAccountStatus`, which
used the global pool, had no caller cancellation and converted every failure into
status zero. Tile editing used that same wrapper. It now takes the caller context
and runtime database, returns a wrapped read error and preserves cancellation.
Help reports unavailable permissions rather than returning a guessed listing;
privileged dispatch stops on read failure. Dispatch also checks owner retirement
before reading and immediately before executing a callback.

All five tile-editing authorization callers now supply their WorldHandler. The
established policy still permits account status above zero or character GM above
zero, but an expired/cancelled read cannot fall through to cached GM authority.
Tile mutation storage and reflection broadcasts remain their separate open audit;
this change does not claim that those global/uncancellable writers are migrated.

PostgreSQL regressions disable the global database, verify injected account status
and missing-account rejection, and exercise real single-connection pool waits.
They check privileged command rejection/admission, `/help` permission filtering,
account and character-GM policies, and cancellation without callback execution or
cached-GM fallback. Existing tile paint/erase/persist/reload/broadcast checks passed.
Full world/session race suites passed in 49.287 and 1.066 seconds; logs are retained
at `/var/tmp/capturequest-account-authority-full.log`. Final focused authorization
and tile regressions passed in 1.523 seconds after the help-filtering and early
retirement checks were added; logs are at
`/var/tmp/capturequest-account-authority-final.log`. Diff checks passed. No frontend, wire, schema or asset changed; no new rendered or
production acceptance is claimed.

Remaining: option/story writers, bridge shutdown, independent-owner cache ordering,
tile mutation storage/contracts and other command rows. The original restore
timeout remains unattributed, and all five roadmap areas remain active. Next:
review this shared authorization boundary, then audit creation/static query
contracts and cancellable reads. This checkpoint is local only; no push, deployment
or production mutation is part of this continuation.

## Chat and heartbeat ownership baseline (2026-10-08)

Chat persistence started a new background context after admission, so a pool wait
could outlive connection cancellation. Player messages now use `CommandContext`;
the verified Discord HTTP publisher carries `r.Context()` through the same bounded
persistence helper. Cancellation/deadline failure stops publication. Other history
storage failures retain the established best-effort live-chat policy, with one
error log and no automatic resend. Chat is not an exactly-once gameplay mutation:
no receipt, resource revision, inventory coordinator or durable delivery workflow
is added. Disconnect/crash can lose a live message; committed history is separate
from live delivery and does not authorize retry.

The global `chatRateLimits` map had no retirement path. Its existing 500ms throttle
now lives on the connection's Session, so session-ID reuse cannot inherit entries
and collection needs no global cleanup job. A short mutex preserves `time.Now`'s
monotonic clock. Player text now uses the same 256-character bound as the external
chat path, preventing byte truncation from splitting Unicode. Existing source
whitespace policies and censorship remain. Heartbeats require a valid nonnegative
numeric timestamp; malformed/null/missing timestamps no longer refresh liveness.
The current socket sends that numeric timestamp, including before login; the
existing session gate still permits guest keepalive and rejects closed owners.

Focused regressions cover actual pool-wait cancellation with no persisted or
published chat, identical Unicode-safe stored/broadcast text, rapid duplicate
throttling, session-ID reuse/close, malformed and valid heartbeat replies, and
HTTP request-context propagation. No new rendered or production evidence is
claimed; no frontend, schema or generated asset changed. Full world, session,
Discord and server race suites passed in 47.391, 1.065, 1.008 and 1.219 seconds.
After the final monotonic-throttle refinement, focused session/world checks passed
in 1.006 and 1.299 seconds and Discord checks reused their passing cache. Evidence
is retained in `/var/tmp/capturequest-chat-full.log` and
`/var/tmp/capturequest-chat-final.log`. Diff checks passed.

The options/chat/liveness row remains open: `getAccountStatus` and slash-command
writers still use legacy global/uncancellable queries. Remaining slash mutation
source authorization, independent bridge shutdown and stale-client audits must
not disappear because ordinary chat passes. The original login restore timeout
remains unattributed and all five roadmap areas remain active. Next: trace slash
commands and their shared account-status authorization query through the same
owned context before selecting another family. This checkpoint is local only;
no push, deployment or production mutation is part of this turn.

## Preference boundary review: publish the committed snapshot (2026-10-08)

Review found the preference handler starting a second transaction after the write
committed. Failure in that follow-up read could leave the live client's cached
preference unchanged even though PostgreSQL had accepted the write. The existing
`SetBooleanOption` transaction now returns its private snapshot only after commit
succeeds. The handler publishes that snapshot immediately; only explicit `current`
requests perform the character-locked read. Failure returns no committed snapshot.
This follows the reviewed flag-writer commit/publication rule and removes a database
round trip without introducing another coordinator or transaction helper.

The regression uses a deferred PostgreSQL trigger to change the next transaction's
default to read-only. It observes successful preference publication/cache update,
then explicitly verifies a separate current-read transaction fails. This proves
the old second-transaction dependency, rather than assuming cancellation timing.
The repository regression also asserts committed snapshots contain the new value
and revision and rejected commits return no snapshot. Focused character/world
race checks passed in 1.097 and 1.272 seconds; logs are retained at
`/var/tmp/capturequest-preference-commit-boundary.log`. Both rendered preference
cases passed in 21.5 seconds: normal persistence/reentry and dropped-reply current
recovery/reentry. Evidence is retained in `/var/tmp/capturequest-rendered.wNCuVe`.
The runner compiled the changed backend and stopped its private runtime. Diff
checks passed. No frontend, wire, schema or generated-asset
contract changed in this review, so no new frontend build is required.

Remaining: independent-owner cache ordering, legacy option/story writers and the
open command matrix. The original login timeout remains unattributed. Next: audit
chat persistence cancellation, rate-limit retirement and heartbeat handling through
the existing session gate. The broad goal remains active; no push or deployment is
part of this local checkpoint.

## Preference command and current-state recovery (2026-10-08)

The client changed `allowTrainerRebattles` before sending an uncorrelated
`SetOption`, so a rejected or lost write could leave its displayed toggle wrong.
`SetOption` now has explicit request/reply contracts with character identity,
request correlation and `preferenceRevision`. A desired boolean write checks and
increments that revision under the existing character transaction; the revision
lives in the authoritative options JSON. Patches preserve center/story/unknown
keys. Stale or duplicate revisions reject without mutation. A `current` request
reads preferences under the same character lock and never increments the revision.
No inventory coordinator, broker, new table or schema migration was introduced.

The client uses the shared `correlatedRequest` timeout, cancellation and listener
cleanup. The Options toggle shows its existing confirmed value until settlement,
displays Saving while pending and rejects rapid second clicks. Unknown outcomes
read current preferences without resending the write. Failed recovery requires a
current read before another mutation. Reply validation fences character identity,
revision and boolean values; late replies have no global application. Character
change or leaving the game retires pending work, including same-character reentry.
Closing only the Options panel leaves the character-owned operation running.
The obsolete uncorrelated send and inline server handler are retired. Canonical
Tygo generation publishes matching contracts; old requests without correlation
reject, so rollout requires matched frontend/backend code.

Five client regressions passed: confirmed settlement, rapid duplicate admission,
lost reply/read-only recovery and late acknowledgement, quit/reentry retirement,
failed-recovery admission and an older read overtaken by a newer preference view.
PostgreSQL checks cover character mismatch, stale/duplicate revisions, current
read revision stability, key preservation and commit rejection/cache consistency.
Full character, client, world and simulator race suites passed in 1.373, 1.090,
51.238 and 1.068 seconds. Typecheck and diff checks passed. Three rendered cases
passed in 39.4 seconds: normal preference/reentry, dropped reply/reentry and walking
recovery/reentry. The fault test observes exactly one preference mutation followed
by one current read. Evidence is retained in `/var/tmp/capturequest-rendered.3kCupL`.
The production build and its canonical runtime-asset validation passed (Vite build
3.57 seconds); build logs are at `/var/tmp/capturequest-preference-build.log`.

This establishes the selected preference command baseline, not closure of the
combined options/chat/liveness row. Remaining option/rival-name writers, unrelated
chat/heartbeat limits, independent-owner cache ordering and the original
unattributed restore timeout remain open. No new process-death or production
acceptance is claimed. Next: review this preference boundary, then audit the
remaining chat/liveness commands as their own non-inventory domain. This checkpoint
is local only; no push, deployment or production mutation is part of this turn.

## Flag writer commit-result boundary (2026-10-08)

`EventFlagManager.writeFlags` previously committed the mutation, then returned
`LoadFlagsContext`'s error as the write error. A caller cancellation or unavailable
connection during that second query could therefore report rejection after a
successful toggle or batch. The writer now reads its complete flag snapshot inside
the same locked transaction and publishes it through the existing committed-cache
primitive only after commit succeeds. A snapshot-read failure rolls back the
mutation; cancellation after successful commit cannot turn it into rejection.
There is no follow-up query or second transaction on this writer path. The shared
`eventFlagSnapshotIn` helper was mechanically moved from movement into the flag
domain file; movement, battle admission and current recovery keep using it.

The new PostgreSQL regression blocks cache publication, observes the committed flag
from a separate connection, cancels the caller, then releases publication. The
writer still reports success and publishes both the existing and new flags.
Previously established rollback, cancellation and concurrent-toggle checks remain;
focused flag checks passed five repetitions in 3.165 seconds before the final
commit/cancellation regression was added. Full world and simulator race suites,
including that regression, passed in 46.134 and 1.069 seconds. Logs are retained
at `/var/tmp/capturequest-flag-snapshot-full.log`; diff checks passed.
No new frontend, wire, schema or generated-data contract changed.

Live writers remain serialized by the existing session owner, whose close drains
publication before unloading flags. This change does not prove ordering between
independent owners or retire the background/simulator audit. Direct post-commit
refreshes in other families still need their domain review; missing cached flags
must not be treated as a universal authorization denial because absent-flag rules
also exist. The original login timeout remains unattributed. Next: audit preference
reply and current-read recovery as a complete command family. The full roadmap
remains active; this checkpoint is local only, without push or deployment.

## Explicit flag-writer contexts and obsolete boulder path (2026-10-08)

The writer inventory found `SetFlag`, `ResetFlag`, `ToggleFlag` and `SetFlagBatch`
creating a background transaction and then a second background cache read. They
now require the caller's context and use it for both the existing character-locked
transaction and its refresh. Debug scene setup/reset passes the session command
context through its helper chain. The remaining simulator battle and Seafoam
fixture calls explicitly choose a background fixture context; they are not live
command ownership or crash-recovery evidence. Broader simulator fixture atomicity
and debug helpers' other global/background writes remain unaudited.

`HandleVictoryRoadBoulderTarget` and its result type had no callers anywhere in the
Go tree. That stale path decided a durable transition from cached flags and wrote
only a flag, separate from object/movement state. It is removed. Live Victory Road
pushes already use `pushBoulder` and its character transaction; target data and
lookup APIs remain. The Seafoam flag-only helper still has a simulator caller and
is explicitly retained for that fixture until its simulator migration.

A PostgreSQL regression holds the actual character row lock while calling the
batch writer with a 50ms deadline. Cancellation returns before lock release, and
neither database flags nor cache state change. Existing batch rollback and
concurrent-toggle checks remain. Focused world and simulator race tests passed
in 1.861 and 1.068 seconds. Full world and simulator race suites passed in
50.636 and 1.067 seconds; logs are retained at
`/var/tmp/capturequest-flag-writers-full.log`. Diff checks passed. No wire,
schema, frontend or generated-data contract changed, and no new rendered or
production acceptance is claimed.

Remaining: post-commit refresh failure semantics and cache publication ordering,
simulator fixture migration, debug mutation atomicity/global dependencies and the
other open rows in `SERVER_COMMAND_AUDIT.md`. The original restore timeout remains
unattributed. Next: review whether cache refresh failure can make a committed flag
write appear rejected, then move to preference reply/current-read recovery. This
is a local checkpoint; no push or deployment is part of this continuation.

## Owned post-commit flag refresh (2026-10-08)

The remaining live refresh inventory found battle, cutscene, Safari, escape/home
warp, map load, item blackout and puzzle publication reaching `LoadFlags`, whose
wrapper creates a background context. Their durable transactions already had
owned contexts, but the follow-up cache query could continue waiting after command
cancellation and delay session draining. These callers now use
`LoadFlagsContext` with their existing session or execution context. The shared
Safari refresh helper explicitly takes its owning session; cutscene publication
explicitly takes the interpreter's execution context. No new lifetime owner or
refresh implementation was introduced. A committed mutation remains committed
when its cache refresh is cancelled; existing recovery reads remain authoritative.

A real PostgreSQL regression leases the only database connection and invokes the
Safari and cutscene refresh paths through `Session.ExecuteCommand`. Both return
at the 50ms command deadline, with the pool wait counter proving contention.
The regression releases its own lease and drains its callback on failure. Focused
flag, cutscene, Safari and puzzle race tests passed in 4.303 seconds. Full world
and simulator race suites passed in 48.233 and 1.069 seconds; evidence is retained
at `/var/tmp/capturequest-owned-flag-refresh-full.log`. Diff checks passed. No frontend or wire behavior changed,
and no new rendered or production acceptance is claimed.

Remaining: background-context flag mutation APIs and their debug/simulator callers,
unused legacy puzzle mutation paths, option reply/recovery lifetime and all open
command-family rows. The original login restore timeout remains unattributed;
this cancellation gap is established independently. Next: retire unused legacy
flag mutation paths and audit the surviving writers' ownership before adding a
new family. This checkpoint is local only; no push or deployment is authorized.

## Flag-cache lifetime and preference persistence checkpoint (2026-10-08)

The flag loader held the shared cache mutex during database I/O and started its
five-second timeout only after acquiring that mutex. A blocked query could delay
unrelated characters' cached flag reads and entry. Loads now perform I/O outside
the mutex under the caller deadline. Temporary per-character read tokens prevent
an older load from replacing a newer committed snapshot or resurrecting an
unloaded character. Completed and failed loads remove their tokens. Movement and
boulder commits publish through one snapshot-copying cache primitive.

`SetOption` previously changed client state before persistence and saved the whole
cached options document through a background/global database path. That document
could overwrite newer `lastPokeCenterMapId`, story or unknown keys. The handler now
validates a desired boolean preference, patches only its whitelisted JSON key in
the existing injected character transaction, and changes the client cache after
commit. The obsolete full-document `SaveOptions` API and its callers are retired.
No additional gameplay coordinator or schema was introduced.

PostgreSQL regressions verify actual pool-wait cancellation, unrelated cache reads
while a load waits, unload fencing, snapshot isolation and token cleanup. Preference
checks verify preservation of other stored keys and database/client rollback when
a deferred constraint rejects commit. Focused checks and the full character,
client, world and simulator race suites passed (1.380, 1.093, 45.241 and 1.068
seconds). Logs are retained at `/var/tmp/capturequest-flag-option-check.log`.
The rendered run covering preference persistence through fresh entry, multiplayer
warp visibility, private boulder isolation and walking recovery/reentry is marked
passed with no failed tests in
`/var/tmp/capturequest-rendered.y6Eq8q/playwright/.last-run.json`. This is local
acceptance; no production availability claim follows from it. Diff checks passed.

This is a bounded stopping point for the requested commit and branch push. It
includes the preceding local checkpoints since the last branch push. The full
five-area goal remains incomplete. In particular, the original five-second
`restoreBattleOnLogin` timeout has not been attributed or closed. Remaining flag
writer cancellation, legacy option/rival-name reads, option reply/recovery lifetime,
chat/liveness and other command families remain in `SERVER_COMMAND_AUDIT.md`.
Next: audit remaining flag writers and preference transport against the existing
ownership, transaction and current-read primitives before migrating another family.
No production deployment or production mutation is authorized by this checkpoint.

## Remaining entry options, account and flag dependencies (2026-10-07)

Entry dependency tracing found `NewClient` loading options through the global pool
and `context.Background()`, then constructing default preferences after any read
failure. `LoadOptions` also silently defaulted malformed stored JSON and missing
characters. Entry used the background event-flag wrapper and continued after its
failure; its account/name predicate still selected the global pool. The last-login
save likewise logged failure and continued initialization.

The sole client-construction caller now supplies the entry context and injected
query handle. `LoadOptionsFrom` is the shared read/parser used by the constructor
and legacy wrapper. Unset options and missing object keys keep established defaults;
missing characters, non-object JSON, invalid recognized fields, read failures and
cancellation return errors. Unknown option keys remain extensible. The constructor
cannot convert such errors into default state. Account/name validation now uses
the injected handle. Event flags use the existing `LoadFlagsContext` with the
entry lifetime and fail entry on error. A failed last-login save also stops entry.
The existing failure path closes/drains partial ownership instead of publishing
an incompletely initialized character.

PostgreSQL regressions exercise unset/default and malformed option values, missing
characters, actual single-connection pool contention, injected construction with
the global database disabled, cancellation without client creation and failed
entry without client/presence/lease. Focused checks passed, followed by complete
character (1.276 seconds), client (1.093 seconds) and world race suites (45.509
seconds). Three rendered cases passed in 47.5 seconds in
`/var/tmp/capturequest-rendered.hnHynd`: walking lost-result/reentry, multiplayer
warp visibility and character-private boulder isolation. The isolated runner
compiled the current server, validated matched local assets and stopped its
private runtime. Diff checks passed. No wire, schema or frontend production
contract changed. This checkpoint is committed locally, with no push or release.

The original restore-timeout attribution remains unproven. These are established
entry budget/initialization defects, not evidence that they caused that particular
five-second event. Event-flag cache mutex contention and remaining legacy option
save/rival-name reads still require their own audit; passing reentry does not close
the timeout investigation. Next: inspect cache-lock wait and option writer lifetime
through the same authoritative boundaries. The broader five-area roadmap remains
active. No push, deployment or production mutation is part of this local checkpoint.

## Character entry read ownership and cancellation (2026-10-07)

Presence/character-handoff review confirms the existing owner barrier closes and
drains the previous connection before replacement, reloads committed state after
that barrier, and prevents delayed cleanup from evicting replacement state. Closed
sessions return empty presence immediately; successful entry owns spawn publication.
The review found a separate concrete cancellation gap: both entry character loads
called `GetCharacterByName`, whose implementation used the global pool and
`context.Background()`.

`GetCharacterByNameContext` now accepts the runtime database and cancellation
context. Entry uses the session context for initial identity and the owned handoff
context for the post-drain reload. The established reload/account/identity checks
remain. The legacy wrapper delegates to that read for callers still awaiting their
own migration; no duplicate scanner or new entry coordinator was added.

A real PostgreSQL regression disables the global database, loads the selected row
through the injected pool, then holds its only connection. A 50ms caller deadline
terminates the blocked read with `context.DeadlineExceeded`, and the pool's wait
counter proves actual contention was exercised. Existing handoff/late-cleanup/
presence checks passed. Full character tests passed (1.143 seconds), session checks
passed, and the world race suite passed (47.763 seconds). Three rendered cases
passed in 45.5 seconds in `/var/tmp/capturequest-rendered.SzutCy`: walking lost-reply
and character reentry, multiplayer warp visibility and private boulder isolation.
The runner compiled the updated server, validated matched local assets and stopped
its private runtime. Diff checks passed. No wire, schema, frontend or generated-
asset contract changed. This checkpoint is committed locally, with no push or
production deployment.

This fixes an established way entry reads could outlive their command budget and
consume time before battle restoration. It does **not** establish attribution for
the original five-second `restoreBattleOnLogin` event. That issue remains open;
existing failure diagnostics and query/pool/stage evidence must still establish
its actual cause. Passing reentry runs alone do not close it.

Remaining: other entry global/background reads, failed-entry/final-save recovery,
remaining lifecycle and query families, source/writer audits and the full five-area
roadmap. Next: audit remaining entry dependencies against the same cancellation
and injected-storage boundary, while retaining the original timeout investigation.
No push, deployment or production mutation is part of this local checkpoint.

## Entry presence and viewer-scoped actor publication (2026-10-07)

The publisher inventory found three bare arrays still sent on
`PhaserActorsResponse`: peer actor spawns, scripted object shows and an unused
initial-player helper. That opcode now belongs exclusively to the typed correlated
read. Those unsolicited arrays were ignored after retiring global application.
Actor reads also republished player spawns on every refresh.

All unsolicited spawn/show packets now use the existing single-actor update
stream, which already renders unknown actors. Successful world entry publishes
player presence once after character/flags/battle initialization. The entering
player gets its own actor through the owned map snapshot. Actor queries only
enumerate published presence; they neither refresh presence nor broadcast entry.
The old initial-player array/recovery routine was retired rather than retained as
a second entry path. Warps and disconnects keep their existing visibility owners.

A second boundary issue was character-private boulder publication. Its object
positions and puzzle flags are stored per character, but the producing actor's
coordinates were broadcast to all viewers. A viewer without an override could
receive the producer's moved coordinates. Boulder updates, drops and affected-map
shows now target only the selected owning session, with post-commit state. Other
viewers retain their own puzzle projection. Affected-map reads use the injected
database and owned context. Shared actor projection likewise uses injected,
cancellable visibility/position helpers and fails closed on read error; direct
scripted shows use their session context. The obsolete global object loader was
removed. Standalone visibility fixtures now explicitly inject their database.

Focused tests prove one peer entry event, no origin snapshot push, no read-driven
rebroadcast, and private boulder publication with the global database disabled.
Existing visibility, boulder, entry and login checks passed; the full world race
suite passed (55.964 seconds). Seven rendered cases passed in 1.4 minutes in
`/var/tmp/capturequest-rendered.eYMtEN`: peer entry, warp departure/reentry,
two-character boulder isolation, and the existing normal/lost-world/process-death
boulder cases. The second viewer retains its boulder at (18,10) while the producer
sees (18,9). The private runner compiled the updated server, validated matched
local assets and stopped its runtime. A publisher search confirms only correlated
success/error replies remain on `PhaserActorsResponse`. Frontend production code,
wire DTO shapes and generated-data contracts were unchanged; diff checks passed.
This checkpoint is committed locally, with no push or deployment.

Remaining: broader player-presence/connection replacement and background publisher
lifetime review, other world queries and global helpers, source/route/writer audits,
the original login restore timeout and the rest of the five-area roadmap. Next:
review the remaining presence projection and character handoff boundaries before
migrating another query family. This checkpoint is local, with no push or deployment.

## Actor read-view race policy and acceptance (2026-10-07)

Refresh reconciliation guarded live events received during its query, but initial
map loading applied its actor array later, after additional sprite work, without
that captured view. Both paths now use `ActorReadView`: immutable cached actor
references identify newer moves/spawns, and despawn markers protect known and
never-cached actors from resurrection. There is no world-wide revision or need
for NPC streams to become quiet. Initial map loading captures the view before the
actor fetch and uses it at final preparation; refresh captures and resolves the
same policy before applying through the existing renderer. Scene cleanup clears
its marker state. Removed actors and newer cache state retain their existing
ownership/presentation handling.

Focused policy tests prove unchanged updates/removals, newer moves/spawns through
final projection, and known/never-cached despawn preservation. The map-loader
fixture now supplies the capture callback. All 103 focused client tests, TypeScript, runtime-asset validation, production build and diff checks passed.
Server code, wire contracts and generated assets were unchanged in this checkpoint.

Ten rendered cases passed in 1.8 minutes in
`/var/tmp/capturequest-rendered.XZdUIN`. A transport-held actor read containing
boulder y=9 is overtaken by a second real owned push publishing y=8; release of the
old read preserves y=8 while current pose recovers to (18,9). A separate held read
is released after quit; fresh entry sees committed player (18,10) and boulder y=9,
with no historical application. Neither case emits a client step completion for
server-owned movement. The existing boulder duplicate/lost-world/crash cases and
arrow/route entry/mid-route crash cases also pass. The harness uses the actual
map ID from state instead of a guessed catalog constant. The runner validated
matched local assets and stopped its private runtime.

Remaining: broader actor/player-presence and cached-view lifetime review, remaining
world query families, route-data/source/writer audits, the original login restore
timeout and the five-area roadmap. The selected late refresh/update and retirement
acceptance is verified; it is not a claim that every world-presentation race is
closed. Next: audit actor-query presence side effects and remaining global read
helpers, then continue the finite command-family matrix. This checkpoint is local;
no push, deployment or production mutation occurred.

## Owned actor reads and lost-object reconciliation (2026-10-07)

The actor read returned an uncorrelated array, queried the global pool, then
applied object positions through another global read. `fetchActors` settled on
any actor array; its timeout left the listener registered. TileViewer globally
applied every array. These paths could not safely recover a boulder whose object
notification was lost.

`PhaserActorsRequest/Response` now carry request, character and map identity and
an explicit typed actor array. The handler rejects old/malformed/foreign-owner
requests and uses the injected pool's bounded character-locked transaction for
object rows, collected-item filtering, flags, visibility and position overrides.
Scan/read errors reject the whole view. Dynamic runtime actor state and player
presence retain the existing runtime managers. The reflection response adapter
is removed from this endpoint. Its startup/visibility publication behavior is
otherwise preserved; broader presence side-effect review remains separate.

`fetchActors` uses the existing correlated request primitive, whose timeout and
abort remove the listener, then validates map and selected character. Map loading
passes its abort signal. TileViewer's global array subscriber is retired; loaded
map preparation already owns initial rendering. Unknown facing recovery and
same-map Instant Warp now call the scene's actor reconciler. It reads current
actors under captured character/map/lifetime, removes absent unchanged actors,
and applies results through the existing actor-update renderer. Immutable cache
references and a read-scoped set of live update/despawn IDs preserve newer events,
including despawns of actors not yet cached. No world-wide quiescence or additional
revision counter is required. Late sprite preload completion checks cache identity,
selected character, active scene and current view before rendering.

Facing recovery refreshes actors first, then uses the existing current gameplay
reader to project resources/pose, avoiding an older pose captured before actor
refresh. No mutation retry or global actor snapshot application remains. Player
pose stays with the movement owner; actor refreshes preserve it.

Verification: the injected PostgreSQL actor-read regression checks correlation,
viewer/map identity and committed object overrides with the global database
handle disabled. Invalid/old/foreign requests reject. The public DTO deliberately
omits internal `DbID`, so the assertion checks its registry runtime ID. The full
world race suite passed (46.800 seconds). All 77 focused map-data/movement tests
and TypeScript checks passed, including actor success/correlation, timeout,
cancellation and character replacement. Canonical wire generation and diff checks
passed. Runtime-asset validation and the production build also passed.

Eight rendered cases passed in 1.3 minutes in
`/var/tmp/capturequest-rendered.e0w8gU`: boulder duplicates, lost reply, reply plus
movement-notification loss, loss of object notifications as well, push-boundary
process death, arrow activation and forced-route entry/mid-route process death.
The all-notification-loss case recovers player (18,10) and boulder (18,9) through
current owned reads, with no client step completion or second object move.
The exact-process runner validated matched local assets and stopped its private
runtime. No push, deployment or production mutation occurred.

Remaining: dedicated late actor-read/stream-race rendered acceptance, initial map
load actor ordering, actor/warp read-family and player-presence lifetime audits,
route-data/source/writer reviews, the original login timeout and the wider five-area
roadmap. Next: review this shared actor boundary's late-read behavior before
migrating another query family. This checkpoint is local.

## Current recovery for unknown facing outcomes (2026-10-07)

Facing can now commit a boulder mutation and movement cursor, so its old blanket
position-free timeout rule was incomplete. Explicit correlated rejection still
reconciles only source-matching direction. An ambiguous transport failure now
engages the existing movement recovery lock and reads the existing current
coherent gameplay snapshot with the movement-generation fence. The same owned
resource/position projection used by Escape Rope and map loading applies it.
The turn is never resent. Failed reads preserve the lock; retired or replaced
owners cannot apply the result. The successful facing callback also checks the
captured character before reserving pending server movement.

Unit checks cover one mutation/one current read, current-pose projection, failed
read locking across stops, retirement during recovery and rejection of an old
character's successful pending-movement reply. All 47 focused client checks,
TypeScript, runtime-asset validation, production build and diff checks passed.

Four rendered boulder cases passed in 38.0 seconds in
`/var/tmp/capturequest-rendered.46YNv5`: normal duplicates, lost acknowledgement,
combined loss of facing acknowledgement and all server-movement notifications,
and pre-follow-up process death. The combined-loss case waits for the ordinary
request timeout and then reaches the committed player (18,10) through current
recovery, with the object at (18,9), no cursor and no client step completion.
Historical acknowledgement delivery and quit/reentry preserve those positions.
The exact-process runner compiled the existing server, validated matched local
assets and stopped its private runtime. The final character-success guard does
not change the covered same-character path; it has a separate focused regression.
No server, wire or generated-data contract changed in this checkpoint.

Remaining: simultaneous loss of object-position notifications is a separate world
presentation/read-lifetime audit; the current resource snapshot does not contain
actor overrides. Wider source, route-data, writer and lifecycle audits, the
unresolved login restore timeout and the rest of the five-area roadmap remain
open. Next: audit owned actor refresh/reconciliation before closing the broader
world-presentation boundary or migrating another family. No new push or deployment
is part of this local checkpoint.

## Boulder duplicate, lost-reply and process-death acceptance (2026-10-07)

Three rendered cases passed in 23.1 seconds in
`/var/tmp/capturequest-rendered.0BB8Gn`, using the existing transport fault and
exact-process crash harnesses. All use the source Seafoam facing fixture at
(18,11), with its boulder initially at (18,10). One client facing request is
transmitted twice. The first commits one object move to (18,9) and one player
follow-up cursor; the duplicate rejects while that source is owned/moving.

Normal and lost-reply cases finish at player (18,10), retain exactly one object
override at (18,9), and have no remaining cursor. The lost acknowledgement is
recovered through the existing server-movement notification, which retires the
pending facing listener. Re-delivering its historical success cannot rewind the
player or restart movement. Quit/reentry preserves both committed positions.
No client ordinary-step completion is sent for the timer-owned follow-up.

The process-death case freezes only the recorded private server at its successful
pending-movement acknowledgement. SQL verifies the moved object and one-point
cursor together while the player is still at the source. The shell kills/reaps
that child with exit 137 and restarts against the unchanged private database.
Fresh entry resumes the remaining player point, deletes the cursor and leaves the
boulder at (18,9). The crash receipt is retained in the run directory. The runner
compiled the current server, validated matched local assets and stopped its
private runtime after completion. Diff checks passed. Production code and wire
schemas were unchanged in this acceptance checkpoint; no push or deployment was
performed.

Remaining: combined loss of facing reply and movement notifications is not covered
by these cases. Facing's old timeout path still describes turning as position-free,
although a boulder turn can now commit a movement cursor. Next: review/reconcile
that unknown-outcome path through current owned state before closing the selected
family. Wider source, route-data and writer audits, the unresolved login timeout
and the full five-area roadmap remain open.

## Atomic boulder push and initial movement handoff (2026-10-07)

The old push performed separate global writes for Strength activation, object
position, hole/switch flags and override removal, then queued the player step in
memory after publication. The operation now uses one injected, bounded character
transaction. Saved source position, durable battle availability, pending route,
owned object visibility and Strength permission are checked on that handle.
Object relocation, puzzle flags and the initial movement cursor commit together.
A pending cursor rejects a duplicate source push before another object change.

The original permission, visibility, tile-override, Seafoam-hole and Victory Road
target rules remain the domain authority. Shared tile/target query helpers now
accept the transaction. Victory Road lookup errors propagate in the mutation
instead of choosing reconstructed fallback targets. The obsolete standalone
object writers and target-walkability wrapper were removed. Simulator callers
retain the public API but delegate to the same operation; the runtime supplies
its session context and database directly. Flags enter the cache and actor
publication occurs only after successful return. The existing movement timer
executes the already committed cursor, including after a fresh registration.

A PostgreSQL regression rejects the final cursor insert at commit and proves no
Strength, object, puzzle flag, cursor or cached flag escapes. Success commits the
cursor at the source, updates the flag cache, and a duplicate cannot move the
object twice. The test disables the global database handle. The older standalone
SQLite tests were extended with required ownership/schema tables, with their
permission and non-boulder assertions unchanged. Focused checks passed in 2.664
seconds; the full world race suite passed in 55.690 seconds and scriptsim package
checks passed in 1.069 seconds. The existing rendered Strength-facing and
server-follow-up interaction passed in 5.8 seconds in
`/var/tmp/capturequest-rendered.Ajgofz`, with before/after screenshots retained.
The canonical isolated simulator accepted `seafoam_1f_boulder_push_into_hole` in
`/var/tmp/capturequest-script-sim.OoAH31`, including flag and object-state
expectations. Both runners validated the local matched asset family and stopped
their private runtime. Wire payloads and frontend production code were unchanged;
diff checks passed. This checkpoint is locally committed, with no push or release.

Remaining: boulder transport/rejection/reentry and process-death acceptance at the
new push boundary, wider field-action source/data/writer audits, the original login
restore timeout and the rest of the five-area roadmap. Next: review this shared
push boundary and complete its failure/reentry acceptance before migrating a new
family. No new push, deployment or production mutation is part of this local
checkpoint.

## Durable forced-route progress and loaded-position recovery (2026-10-07)

`character_movement_routes` is one current, versioned movement cursor per character:
committed source map/X/Y, remaining unit-adjacent points and Surfing mode. It is
separate from the historical ordinary-step receipt and has no new command counter.
The existing character transaction commits each point, step effects and remaining
cursor together. Spin/current entry commits its initial cursor with the entry
point; rollback leaves both unchanged. A fresh movement registration restores the
matching cursor under the character lock before successful world entry. Unknown
versions, missing coordinates/mode, non-adjacent points, source mismatch or missing
source catalog fail explicitly. Failed entry now closes/drains the partial owner.

Completed routes, explicit teleports (including same-tile teleports), confirmed
battle ownership and NPC-blocked routes retire the cursor. Same-position persistence
flushes preserve it only when saved pose, cursor and requested pose agree. Database
failures retain existing retry behavior. Debug/simulator resets clear their own
route state. Startup requires the new table. `server/schema/README.md` documents
the additive migration: use the schema-aware full-data workflow and its backup
rules for an authorized release; no reset or production change occurred here.

The first mid-route process-death case exposed a separate presentation race.
The retained private database showed character 3 at map 200, (2,9), with no cursor,
while the browser remained at (3,9). The server had completed correctly before
scene binding. Map loading previously recovered resources/plans but ignored the
snapshot position. It now uses the existing current reader's movement-generation
fence and the same owned-position projection as Escape Rope. Pose is projected
before battle/plan restoration; retired loads cannot apply a late position. There
is no global historical position handler or additional read retry loop.

Verification: focused route/progress/cancellation/schema checks passed, followed
by the full world PostgreSQL race suite (44.731 seconds) and importer tests. The
first broad run's standalone SQLite visibility fixture lacked the required table;
its schema was updated with teleport assertions unchanged. Final NPC cancellation
and related checks passed in 2.910 seconds. All 51 focused movement/map-loader/
recovery client checks and TypeScript checks passed. Workflow YAML parsing,
runtime-asset validation, production build and diff checks passed.

Four rendered cases passed in 44.8 seconds in
`/var/tmp/capturequest-rendered.hpXJ91`: normal imported-arrow activation, process
death after route entry, process death after the first point, and ordinary walking
process-death recovery. The private crash harness freezes only its verified
recorded server PID, reads the committed source/cursor, then kills/reaps that child
and restarts against the unchanged private database. Both route boundaries reach
(2,9), delete their cursor and send no client step completion after reentry. Earlier
failed evidence is retained in `PfGpLY`; its private cluster was briefly reopened
for the exact SQL inspection and stopped again. The final runner stopped its
private runtime. No push, deployment or production mutation was performed.

Remaining migration: boulder pushes still use older separate mutations and queue
their first movement point after publication. The shared timer can persist later
progress, but that initial handoff is not atomic or crash-safe. Next: migrate the
boulder push plus route-start outcome through the existing character transaction
before declaring all forced-route producers durable. Route source/catalog and
writer audits, the original login restore timeout and the broader five-area goal
also remain open. This checkpoint is local.

## Ordinary step activation of source-driven forced routes (2026-10-07)

The source inventory has three route producers: boulder follow-up, imported spin
movement and Seafoam current rules. Inspection found `commitMovementStep` returned
for every `!c.Forced` candidate before looking up spin/current data. Only boulder
steps or already-running routes could reach that code; ordinary issued walking
never activated an imported arrow. There was no generated-script replacement:
`script_candidate_import_diagnostics.json` labels the arrow routine covered by
`spin_tile_runtime_v1` and identifies `spin_tiles` as its runtime source.

Walking now uses those same existing spin/current lookups. Surf entry keeps its
existing wild-only policy; automatic warps remain specific to forced route
endpoints. No parallel tile rule or generated asset override was added. After an
issued step commits, its movement owner installs the selected route and publishes
a source-point server-movement notification. This retires the user's future path
before timer-owned points continue. A duplicate completion remains a historical
receipt acknowledgement and cannot reinstall or republish the route.

A real PostgreSQL regression rejects the entry commit and proves no route starts;
then it commits, checks the two planned points and source-only start notification,
and verifies duplicate completion cannot restart it. Its messenger uses a real
registered session so origin publication is exercised. Focused checks passed
(3.385 seconds), followed by the full world race suite (44.970 seconds). The source
fixture at Rocket Hideout B2F (5,9) passed canonical tile validation. The local
SQLite catalog identifies arrow (4,9) with `LEFT,count=2`; the importer normalizes
its source map name to `ROCKET_HIDEOUT_B2F`. Two rendered cases passed in
23.7 seconds in `/var/tmp/capturequest-rendered.ESLLNu`: walking enters the
imported arrow and ends at (2,9), with only one user-step completion; existing
lost-completion/reentry recovery remains intact. The runner compiled the updated
server, validated the matched local assets, and stopped its private runtime.
Wire types and frontend production code were unchanged. Diff checks passed.
This checkpoint is committed locally; no push or production deployment occurred.

Remaining: restart-safe persistence/resumption of an in-progress forced route,
forced-route data validation and source/continuation audit, the unresolved login
restore timeout and the wider five-area roadmap. Runtime route progress remains
session memory; this activation fix is not a claim of crash-safe continuation.
Next: persist/recover route progress through the authoritative movement owner
before treating this family as complete. No new push or deployment is part of
this local checkpoint.

## Durable battle ownership at movement commit (2026-10-07)

`commitMovementStep` previously locked the character and checked pending trainer/
cutscene plans, saved source and destination catalog, but never consulted ordinary
or Safari battle records before position and step effects. Issued-step admission
used `getBattle(charID)`, and the forced timer had no equivalent durable fence.
Cache absence could therefore admit walking, forced movement or Surf position
work while a persisted encounter still owned the character.

All three candidates now use the existing `requireNoOwnedBattleIn` immediately
after the character lock, before position, daycare, encounter counters, plans or
receipts. The same guard already serves PC, party reorder, center healing and
Bicycle/Escape Rope. It retains ordinary terminal/pending-choice ownership until
dismissal and Safari encounter ownership until resolution. No inventory
coordinator, second battle model or new transaction wrapper was introduced.

The shared guard identifies a confirmed battle rejection with `errBattleOwnership`.
A forced timer stops that path and broadcasts its unchanged source with
`PathFinished=true`; it does not poll or resume an old route after the battle.
Database/read/commit failures keep the existing retry behavior and cannot publish
a target. Walking and Surf return their existing rejection paths. Newly selected
encounters during a valid step still commit with that step; the guard checks the
owner before selection, not after creating its own battle.

Real PostgreSQL regressions cover cache-absent ordinary/terminal and Safari owners,
read failure, walking/forced rejection without position/daycare/receipt mutation,
source-only path-stop publication and no subsequent tick publication. A Surf case
checks the same gate, and a separate issued-before-battle case verifies that
completion rechecks durable ownership. Existing rollback, encounter creation,
Safari expiry and ordinary forced-path success checks passed in the focused run
(3.705 seconds). The full world PostgreSQL race suite passed (44.225 seconds),
including the added issued-before-battle case. Two local rendered walking recovery
cases passed in 38.7 seconds in `/var/tmp/capturequest-rendered.6aWF0G`: lost
completion/reentry without rewind, and issued/committed process death without
replaying Safari counters. The runner compiled the updated server and stopped its
private runtime afterward. These rendered cases verify normal recovery remains
intact; the uncached battle rejection and source-only forced stop are PostgreSQL/
protocol evidence, not a new visual battle-state acceptance claim. Wire schemas,
frontend production behavior and generated assets were unchanged. Diff checks
passed. This checkpoint is committed locally, with no push or deployment.

Remaining: forced-path source/queue/catalog policies, wider movement and battle
writer audits, the unresolved login restore timeout and the rest of the five-area
roadmap. Next: review the forced-path source and continuation policies before
migrating another command family. No new push or deployment is authorized here.

## Outstanding-step expiry and catalog admission review (2026-10-07)

The pending authorization lifetime was encoded twice: completion used
`time.Since(issuedAt)>10s`, while facing used `<10s` and retired the pointer itself.
Escape Rope required `pendingStep==nil` and could remain blocked after that
same authorization was already too old to complete. The exact deadline edge also
differed between facing and completion.

`activePlayerStep(now)` now applies one existing ten-second lifetime and retires
at `age>=lifetime` under the movement write lock. Completion, facing and Escape
Rope use it only after session/source ownership checks. A live authorization
continues to block facing/rope use; an expired pointer cannot block the field
command or complete afterward. New issuance keeps the existing intentional
replacement rule and its old-token regression, rather than changing cancellation
or admission policy as part of this fix.

Movement-map review confirms that a collision-cache hit authorizes animation
intent, while `validateClientDestinationIn` inside the character transaction is
the commit authority for catalog map/tile existence and erased state. No second
map-validation implementation was added. New real PostgreSQL cases warm the
collision cache, issue a step, then remove its map or erase its target tile.
Both reject completion, preserve the source position and store no receipt.
A separate boundary test proves live-step rope rejection, expiry retirement,
one consumed rope/revision and no rewind after the old completion arrives.
The exact lifetime edge is tested without a wall-clock sleep.

Focused PostgreSQL checks passed (2.887 seconds), followed by the full world
race suite (45.200 seconds). Final focused checks passed again (3.135 seconds).
The first rendered run in `/var/tmp/capturequest-rendered.RUi3eH` passed six of
seven cases, including normal/lost-result/crash Rope and walking recovery. The
new expiry case failed before emitting a step intent; it used immediate key down/up.
Phaser polls `cursors.right.isDown`; the test now uses the established
`pressMovement` helper (focus blur and a held key) after map loading. The existing
failed-recovery keyboard check uses the same helper so it exercises actual game
input. No request-count, deadline, quantity or error assertion was weakened.
Five final rendered Rope cases passed in 1.1 minutes in
`/var/tmp/capturequest-rendered.LU5nV3`, including lost step issuance followed by
expiry/escape, no late completion, failed recovery lock/reentry, and normal/
timeout/process-death Rope recovery. The expected movement timeout is asserted
explicitly as the single console error in the issuance-loss case; all other page,
network and retired-coordinate errors must remain absent. The preceding run also
passed walking lost-completion/reentry and issued/committed walking process-death
acceptance. Both isolated runners stopped their private runtime. Diff checks
passed. Frontend production behavior and wire schemas were unchanged; the
isolated runner compiled the updated server. The broader roadmap, durable battle/cache and catalog/queue audits, and
original login restore timeout remain open. Next: audit movement battle ownership
at commit and the remaining forced-path/source policies before another family.
No push, deployment or production mutation is part of this local checkpoint.

## Movement recovery boundary review (2026-10-07)

Review found that `stopMovement(true)` assigned `stepRecoveryRequired=false`.
That method is also called for ordinary actor snaps and completed server paths,
so it could clear the failed-recovery lock without a successful current read.
The field admission helper also did not consult that lock, allowing another
Bicycle/Escape Rope request from an owner whose recovery had failed. Separately,
`clear()` retired only step work; field cancellation depended on TileViewer's
cleanup calling a second method.

The existing lock is now named `movementRecoveryRequired` because both completed
step and consumptive field recovery use it. Stopping prediction, cosmetic actor
snaps and server-path completion preserve it. The shared field admission helper
rejects work while recovery is required or player ownership is absent. A fresh
scene/controller begins after its ordinary owned-state load; failed owners are
not silently unlocked by a visual update. No additional recovery coordinator,
retry loop or field-specific latch was introduced.

Controller `clear()` and its shutdown hook now retire field requests directly.
TileViewer's cleanup remains idempotent. Late replies cannot apply preference or
resources after the controller's own teardown. Regression checks exercise failed
Escape Rope recovery, actor snaps/stops, blocked keyboard/click and field requests,
fresh-controller admission, clear cancellation and late preference suppression.
The existing failed-step recovery check now also verifies that a subsequent actor
snap cannot unlock it. All 50 focused movement/recovery/warp client checks and
TypeScript checks passed. Six rendered cases passed in 1.6 minutes in
`/var/tmp/capturequest-rendered.QMh5Ap`, including the new failed-read/blocked-input/
quit/reentry case and the existing Rope normal/timeout/process-death and Bicycle
normal/timeout/reentry cases. The isolated runner stopped its private runtime.
Production build, runtime-asset validation and diff checks also passed. These
are local browser and client-boundary results; server code was unchanged.

The transport fault harness can withhold a chosen response opcode after fixture
setup. The rendered failure case loses current gameplay reads, waits for the
explicit terminal recovery warning, then verifies no further item or movement
request is sent. Quit/reentry restores the committed position and remaining rope;
a historical acknowledgement after reentry has no position to replay.

The broader roadmap and original login timeout remain open. Next: inspect outstanding-step expiry and movement-map availability in the
movement admission/catalog/queue audit before selecting another field family.
This is a local review/fix checkpoint, with no new push or deployment.

## Escape Rope command and movement-owned recovery (2026-10-07)

Escape Rope now has a typed correlated command (203/204). It names the owned item
instance, character/resource revision and advertised source map/X/Y. The movement
manager requires matching session registration and source with no outstanding
step or server path. The existing executor owns the final commit of revision,
item ownership/decrement, source-fenced destination and full bag projection.
It also applies the shared durable ordinary/terminal/Safari battle policy.
The previous standalone transaction was replaced, not retained alongside it.
A stale revision cannot consume another rope even after returning to the same
source and registering a fresh movement owner. No new receipt table, counter,
transaction engine or movement coordinator was introduced.

The acknowledgement contains only success/request/character identity. Server
ownership and visibility are refreshed after commit, with a resource-change
notice; it does not send a global warp notification. The client movement owner
always reads the existing locked current gameplay snapshot after settlement,
including an uncertain reply or rejection, and projects its coherent resources
and position. Its position generation also fences that read against an intervening
authoritative snap through the existing reader's bounded retry; other resource
consumers keep their current view policy. It never retries the mutation or applies
a historical destination.
The established server-committed warp event handles the resulting scene change.
Bicycle and Escape Rope now share one movement-controller admission/abort/owner
lifetime helper; retirement and character replacement suppress late application.
Failed recovery engages the movement controller's existing input lock.
The legacy Escape Rope item-use branch and its handler are retired.

The existing destination selection policy is preserved: non-overworld maps
choose the first eligible warp, preferring an outdoor exit. This checkpoint does
not claim original cartridge Escape Rope destination fidelity; that is a separate
source/content policy audit. The debug fixture's Mt Moon (9,9) tile was verified
against the current source catalog and passed the canonical fixture collision
check. It seeds two ropes so duplicate consumption is observable.

Focused source/rollback/duplicate checks passed (2.458 seconds), followed by the
full world PostgreSQL race suite (47.530 seconds). All 42 affected client tests,
TypeScript and canonical wire generation checks passed. Current-recovery unit
evidence includes no mutation retry, no late resource application after scene
retirement, and re-reading when an owned position change overtakes a snapshot.

The first rendered run in `/var/tmp/capturequest-rendered.whzI4o` failed because
the new assertion read nonexistent `player.tileX/tileY` fields; the bridge exposes
`player.x/y`. Database commit and rope quantity assertions had passed. The
assertion now uses the actual bridge fields with the same expected coordinates
and quantity; no production behavior or assertion threshold was weakened.
Five final rendered cases passed in 1.2 minutes in
`/var/tmp/capturequest-rendered.reNIrJ`: Escape Rope duplicate admission with
normal and lost acknowledgement, exact-process death after commit/before reply,
current bag/destination recovery and ordinary quit/reentry, plus Bicycle normal
and duplicate/lost-reply/reentry acceptance after the shared owner extraction.
The private SQL assertions verify revision 1, rope quantity 1 and the selected
exit together. The crash receipt verifies the recorded server exited 137 and
restarted against the unchanged private database. The runner stopped its private
runtime after completion. The production build, runtime-asset validation and diff checks passed. The
validated local family contains 826 tiles, 92 sprites and 561 compact audio files;
no generated data family was changed or published. This checkpoint is committed
locally; no additional push or production deployment was performed. Remaining: wider movement admission/catalog/queue review, other
field actions and resource writers, the unresolved login restore timeout and the
other five-area roadmap rows. This goal remains active. Next: review this shared
command boundary before choosing another field family. No production mutation
or deployment is part of this checkpoint.

## Bicycle boundary review and shared battle admission (2026-10-07)

Review found that the Bicycle setter checked only `getBattle(charID)` and
`!IsOver()`. That granted admission when a persisted battle was absent from the
cache, after a terminal result before dismissal, and during a Safari encounter.
The authoritative records retain ownership through those states. PC storage,
party reorder and center healing already enforced that durable policy with
repeated ordinary/Safari reads. Their identical checks now use
`requireNoOwnedBattleIn` in the existing battle registry. Their transaction owners
and domain-specific source/healing policies remain unchanged.

Bicycle joins the same policy in a bounded, character-locked read transaction
that validates the owned positive-quantity Bicycle instance. It commits the read
before changing the movement preference or publishing. A failed/cancelled read
cannot advance the session preference revision. The actual desired-state update
still checks movement session identity and revision under the manager lock; it
does not consume inventory, create a durable receipt or use an inventory revision.
The session command queue owns admission ordering; player registration resets
preference, so process restart does not promise to preserve riding intent.

Focused PostgreSQL checks passed (4.255 seconds), followed by the full world
race suite (41.880 seconds). Cases include a foreign/empty/wrong item, uncached
ordinary and terminal battle owners, Safari ownership and database read failure;
recovery reads remain available after rejection. A final Bicycle-focused run
passed (1.972 seconds), including a real lock-contention cancellation: the
command deadline propagates and neither preference nor revision advances.
All 24 movement-controller
client tests and TypeScript checks passed, including character replacement
aborting admission and ignoring its late reply.

The existing transport fault helper now selects request payloads and tracks
request IDs: a shared response opcode's recovery reads pass through while only
the selected setter's reply is lost. This extends the existing harness rather
than adding a Bicycle-specific transport coordinator. Five rendered cases passed
in 1.3 minutes in `/var/tmp/capturequest-rendered.85LkMs`: ordinary Bicycle
on/off/on and indoor/outdoor behavior; exact duplicate/lost setter replies,
current-state recovery, old reply delivery and session-reset reentry; and the
existing PC normal/timeout/process-death recovery cases. The PC crash runner
stopped only its recorded private server PID and restarted against the same
private database. Its final cleanup stopped that isolated runtime. No production
mutation or deployment occurred.

This review checkpoint is locally committed separately from the preceding
user-authorized branch push. No additional push or deployment is part of this
continuation. Next: migrate Escape Rope's transport through the existing movement
owner and preserve its atomic item/position/source primitive.

Remaining: the broader movement source/catalog/queue admission and acceptance
inventory, Escape Rope command identity/recovery, the unresolved login timeout,
and other rows of SERVER_COMMAND_AUDIT.md. The five-area goal remains active.

## Bicycle stopping checkpoint (2026-10-07)

Bicycle now uses typed correlated requests (201/202) through the existing movement
controller and correlated-request transport. A setter carries desired riding
preference, owned item instance and the current movement-session revision. The
movement manager checks session identity and revision under its lock; duplicate
setters reject without flipping the preference again. Item ownership reads have a
five-second deadline. Forced-riding maps preserve the existing preference.
Preference is intentionally session-local and resets when the player registers;
this is not a durable inventory mutation or a new command coordinator.

The client reads current state before setting preference. An uncertain result
triggers a current-state read, never a mutation retry. Character replacement and
scene retirement abort pending work and suppress late application. The legacy
Bicycle item-use branch and its global reply application are retired.

Verification: the full world PostgreSQL race suite passed (41.872 seconds), all
23 movement-controller client tests passed, and canonical wire generation,
TypeScript and diff checks passed. Regression checks cover duplicate setters,
wrong session ownership, unchanged Bicycle quantity, current-state recovery,
uncertain setter replies without mutation retries, and retired-client suppression.
The initial rendered run (`/var/tmp/capturequest-rendered.S5yXEA`) passed riding
on/off/on, then stalled because the item list intercepted clicks on Done. The
Bicycle test now closes through the existing Bag toggle without bypassing pointer
checks. The covered Done button remains a separate UI defect; no UI layout fix is
included in this checkpoint. The final rendered run passed (7.4 seconds) in
`/var/tmp/capturequest-rendered.xTDl7Z`, including on/off/on, indoor pausing and
outdoor resumption. Both isolated runners stopped their private runtime.
Remaining: rendered duplicate/lost-reply and reentry fault acceptance, wider
battle/admission review, Escape Rope stable command identity and lost-result
recovery, and the remaining families in SERVER_COMMAND_AUDIT.md. The original
restoreBattleOnLogin timeout is still unresolved. All five roadmap areas remain
open. The recommended next step is to review this movement boundary and finish
its fault acceptance before migrating Escape Rope. No production deployment or
production mutation is part of this checkpoint.

## Field-command review and Escape Rope source fence (2026-10-07)

Inspection confirms Bicycle changes `PlayerMovementState.WantsBicycle` through
`ToggleBicycle(charID)`, derives active/forced riding from movement-map rules and
resets preference on player registration. It is a session-local movement
preference, not item consumption. The current client still sends the legacy
uncorrelated item-use packet and its reply handler applies bicycle/quantity fields
globally. Exact duplicate toggles can undo each other. The next migration should
express desired state under the current movement/session owner, with correlation
and recovery; do not turn movement preference into an inventory-owned engine.

Escape Rope already commits item ownership, decrement and destination together.
Review found that exit eligibility used a source map captured before the
transaction, without verifying that the character still occupied that source
after its row lock was acquired. The existing atomic operation now receives the
advertised owned map/X/Y and checks matching saved coordinates under the same
character lock before item lookup, exit selection or decrement. A changed map
or same-map tile produces an explicit rejection instead of applying an exit
chosen for an obsolete source. No fallback destination or new transaction
coordinator was added; publication remains after commit.

Real PostgreSQL regressions stage a competing transaction holding the character
lock and commit changed map or tile values before the rope operation acquires
ownership. Both reject without consuming either rope or overwriting the competing
position. Existing normal/commit-rejection rollback tests still pass. Focused
field checks passed in 1.393 seconds and the full world race suite passed in
46.095 seconds; the source assertions passed again in the focused rope run.
Diff checks passed. This is database/source-boundary evidence, not new rendered
or correlated-command acceptance. No client wire or presentation was changed.

Remaining: desired-state Bicycle command admission/session correlation, Escape
Rope stable command/source intent and lost-result recovery through the existing
movement coordinator, retirement of their legacy global reply paths, and relevant
rendered/duplicate/cancellation/reentry/crash acceptance. Preserve the rope's
atomic membership/position primitive and map policy while migrating its transport.
The full five-area roadmap and original login timeout remain incomplete. This
checkpoint is local; no push, deployment or production mutation was performed.

## Resource notifications through current owned reconciliation (2026-10-07)

The publisher inventory is migrated: login/debug, cutscene commit, battle/move
learning/blackout, in-game trade, item pickup and Game Corner now publish
`ResourceChangeNotify` through `ResourcesChangedNotify` (opcode 200). Its three
fields are success, resourcesChanged and characterId; it carries no bag, wallet,
party or PC projection. Publishers no longer re-query a mutable snapshot after
commit. Old snapshot helpers, the unused bag reply DTO and global client bag/
party application are removed. Reserved old read opcodes still reject.

The dedicated notification opcode is deliberate: reusing the old party reply
could cause an older client to treat a notice's missing party array as an empty
party. Old clients ignore the new opcode instead. Matching builds are still
needed for live updates; this is not a compatibility alias or a production
rollout. Both old untagged resource packets and foreign-character notices are
inert in the current client.

The existing scene/admission owner marks the matching character's resources
dirty, coalesces notices behind an active command/read, and performs current
locked resource reconciliation. Retirement clears queued work, including
character replacement. Notice data is never directly applied. This preserves
committed updates without letting a historical packet rewind views, and does not
retry a mutation or take over battle/movement/issued-plan presentation. A burst
during a command produces one subsequent owned read.

Verification: resource publisher regressions disable database dependencies and
assert the exact three-field payload and closed-owner suppression. Existing
post-commit/rollback checks for trade, pickup, cutscene, battle and prizes remain;
their expected publication opcode is now the dedicated notice. Client checks
cover coalescing, old/foreign/retired notices, replacement characters, actual bridge
delivery to a current read, and withholding notice payload application until its
correlated snapshot. All 185 affected client checks passed. The bridge fixture
initially had a disconnected fake transport; it now explicitly satisfies the
reader's connected precondition, with all ownership assertions unchanged.

The full world PostgreSQL race suite passed after the final wire change (47.078
seconds). Eight rendered item/PC/script cases passed in 50.5 seconds in
`/var/tmp/capturequest-rendered.WJnFCY`, covering bag refresh, item use, terminal
storage and real scripted rewards/movement. The earlier run in `jd4Fhr` passed
before the dedicated opcode change and is retained as intermediate evidence.
Canonical type/opcode generation, TypeScript/diff checks, frontend build and
runtime asset validation passed. The private
runtime was stopped by its runner. No production mutation, push or deployment
was performed.

This closes the selected standalone bag/party read and historical-payload
publication migration. It does not make every resource writer revision-aware or
close independent wallet/coins, source eligibility and lifetime audits. Next:
review/migrate the remaining Bicycle/Escape Rope dispatch using the appropriate
existing ownership model; preserve Escape Rope's atomic position/item work and
keep movement authority in its movement coordinator. The original login timeout
and full five-area roadmap remain incomplete.

## Explicit resource reads through the shared owner (2026-10-07)

The three production refresh sites (bag mount, battle item menu and the
TileViewer party refresh) now use `refreshOwnedGameplayResources`. The existing
scene/admission owner accepts a typed read transport as well as its command
transport; this calls the current locked gameplay endpoint with the same abort
signal. Resource-only publication shares one validated bag/wallet/party/PC
projection with uncertain-command recovery and full gameplay recovery. It does
not replay battle, trainer or cutscene presentation. Late resolution after scene
retirement or an overtaking resource view remains inert; the read cannot bypass
the active command's admission slot. No additional recovery coordinator was added.

Client facades no longer emit `CQInventoryRequest` or `PokemonPartyRequest`.
Their reserved server handlers return a migration error without reading state.
A regression disables both injected/global databases and verifies no unowned
snapshot can be published by either old read. The wallet-failure publication
test still exercises the existing publisher directly rather than falsely using
a retired read as evidence for query failure. Canonical opcodes remain reserved.

Verification: 185 affected client checks passed, including explicit-read
admission, late ownership retirement, overtaken views and resource-only projection
without plan presentation. Focused PostgreSQL read/retirement/recovery checks
passed in 3.768 seconds; the full world race suite passed in 41.316 seconds.
TypeScript, build/runtime-asset validation and diff checks passed. Six rendered item/PC
cases passed in 32.1 seconds in `/var/tmp/capturequest-rendered.leMYzi`, exercising
the bag and normal item/storage flows with the new refresh transport. The private
runtime was stopped by its runner.

This does **not retire unsolicited snapshots yet**. Known producers remain in
login/debug publication, cutscene commits, battle/learning, in-game trades,
pickups and Game Corner. The global party/bag reply handlers still consume those
packets. Their current snapshot/revision and lifetime behavior must be migrated
as a complete producer/consumer boundary; simply dropping them would lose real
committed updates. Next: inventory those producers and replace unowned payload
application with owner-scoped current-state reconciliation or their already-owned
command projection. Preserve mutation publication until its replacement is
verified. The full roadmap and original login timeout remain open; no push or
deployment was performed.

## PC facing review and next read-family audit (2026-10-07)

The server's source rule requires the terminal directly in front of an idle,
up-facing owned player. The client previously retired menu presentation on
map/X/Y changes but ignored facing. Some facing paths also updated the sprite
without updating `playerTileContext`. Click-source facing and blocked click/
keyboard turns now share `faceDirection`, which publishes the same location and
new direction to that existing context. The shared interaction watcher has an
explicit facing option: PCs enable it; shops retain the location-only policy
because their authorization does not impose the PC facing rule. No separate
presentation or recovery owner was added.

TypeScript, frontend build, runtime asset validation and diff checks passed.
All 161 focused checks passed (140 coordinator, 21 movement), including facing
publication and different PC/shop policies. Five rendered cases passed in 29.9
seconds in `/var/tmp/capturequest-rendered.etSgzD`: held PC opening replies remain
inert after movement, after a blocked turn at the same tile and after quit/reentry;
normal terminal/storage and NPC interaction still work. The old reply is observed
at the dispatcher before asserting that no panel reopened. No server permission
check was relaxed. The isolated runtime was stopped by its runner.

Corpus review: PCs are original static hidden-object triggers, not moving NPCs.
All 18 extractor objects with `SPRITE_CLERK` have `action_type='STAY'`. Thus moving
PC targets do not require an NPC-motion owner. Runtime visibility/position
overrides for other interactions remain part of their existing server permission
and background-owner audit; the corpus observation does not close that wider work.
The four PC mutations share one handler, response projection, revision executor
and recovery adapter. Their common client fault matrix and domain-specific
PostgreSQL checks support reuse of the rendered deposit timeout/crash lane;
additional browser variants should be driven by a changed boundary or uncovered
risk, rather than repeating the same scaffolding four times.

The planned PC implementation/acceptance review now has current evidence. This
does not close every party writer or source permission in the broader roadmap.
Next: audit standalone `CQInventoryRequest` and `PokemonPartyRequest` reads and
their unsolicited pushes for correlation, character/scene ownership and stale
projection. The matrix already identifies those remaining paths, so begin from
their real consumers rather than adding another coordinator. The original login
timeout and all five roadmap areas remain open. No push or deployment was performed.

## Source-position retirement for delayed menus (2026-10-07)

Review found that source-bound menu reads checked character/scene identity and
store freshness but could still open after the player left the original
interaction position. Shops and PCs now use one `watchInteractionPosition`
primitive in the existing coordinator. A location change retires presentation
and closes that source view. Read listeners abort; sent mutations keep their
commit/current-state reconciliation and suppress late presentation effects.
This watches map/X/Y, not facing or moving NPC visibility; those remain distinct
permission-review considerations. Server mutation authorization still rechecks
the current source, facing and ownership under the transaction.

All 138 coordinator checks passed, including delayed shop/PC opening after
movement, PC scene replacement and mutation reconciliation after leaving its
source. A fixture initially retained an unrelated shop; the fixture now closes
it before the PC-only read, preserving the no-reopening assertion. TypeScript
and diff checks, frontend build and runtime asset validation passed. Four rendered cases passed in 24.3 seconds in
`/var/tmp/capturequest-rendered.Fkxh4B`: withheld opening replies are delivered
after walking away and after quit/reentry, and cannot reopen the PC; normal
terminal/storage and NPC interaction checks still pass. A temporary subscribed
observer proves each old packet actually reaches the client dispatcher, rather
than assuming a dropped packet is inert. The owned runtime was stopped by its
runner. No backend/data changes were made in this checkpoint.

Next: finish the remaining facing/permission review and account for moving-source
visibility before closing the PC family, then choose the next unaudited command
family from the finite matrix. The full foundations roadmap and login timeout
remain incomplete; no push or deployment was performed.

## PC failure/restart acceptance and obsolete API cleanup (2026-10-07)

Three rendered cases at Indigo Plateau Lobby passed in 33.9 seconds in
`/var/tmp/capturequest-rendered.wB3phS`. The source terminal is `(15,7)`, accessed
from `(15,8)` on map 174, outside the former hardcoded center list and coordinates.
The fixture's starting/access tiles were checked against extractor collision data.
Tests use the existing inventory-command fault injector and exact-process runner,
not another PC recovery mechanism.

Every deposit packet is duplicated. Exactly one succeeds and one rejects, with
stable party/box row identities and revision one. The timeout case withholds the
success, recovers current PC/party state without resending, then performs a real
withdrawal. Delivering the historical deposit after that distinct second command
does not restore old box membership or rewind party state. Reentry preserves the
result and revision. The crash case closes the pending listener before timeout
recovery, then kills the exact owned server after commit and before its withheld
acknowledgement. Receipt: PID 1142285, replacement 1144050, exit 137. The unchanged
private database and fresh browser/session recover the same deposited row (23),
remaining party row (24) and revision one. Evidence records the private character
and row IDs; no production data is involved. Inspected screenshots show the
recovered Indigo PC box and remaining party.

The earlier two-case run in `/var/tmp/capturequest-rendered.FBPvtl` passed in 28.0
seconds but killed the server after timeout recovery, so it proves persistence
across restart rather than death during an unacknowledged result. The later
three-case run is the stronger evidence. The test adds an explicit duplicate
rejection wait before closing the crash page; the same duplicate count was already
observed in the passing run. The runner stopped its owned processes/database.

Production `DepositToPC`, `WithdrawFromPC`, `ReleasePokemon` and their private slot
resolvers are removed. Storage fixtures now capture stable target identities and
own transactions around the same authoritative primitives. Focused PostgreSQL
storage/PC checks passed (pokebattle 2.087 seconds, world 2.078 seconds); the cqitems
package compiled in that filtered run but had no matching tests. Existing SQLite
storage and fixture usable-tile checks passed. TypeScript and diff checks passed.
PC timeout presentation now asks users to check their box/party through an explicit
message option on the shared coordinator; other consumers retain their wording.

Remaining PC review: delayed opening after source/scene retirement, non-deposit
reply-loss variants where the shared consumer matrix does not suffice, and the
shared routing/permission/projection audit before expanding to another family.
Normal operations, source permission, rollback, current-state timeout recovery,
late history, actual pre-acknowledgement death, nonstandard source access and
obsolete API retirement have current evidence. The broader roadmap and original
login timeout remain incomplete. No push or production deployment was performed.

## PC command and source migration (2026-10-07)

All five PC handlers now require source hidden-object identity and correlation.
Opening takes the shared character read lock without a no-op UPDATE. Deposit,
withdrawal, release and box selection use the existing command revision executor;
source authorization, ordinary/Safari battle ownership checks, row-ID storage
mutation, box preference and complete box/party/bag projection succeed in one
transaction before one typed `PokemonPCResponse` is published. Authorization
checks the original PC routine/type/facing, exact owned map and tile in front of
the player, an idle movement phase and matching durable position. Terminal battle
records also retain ownership until dismissal. Missing identities and slot-only
packets reject without mutation. PC read failures log their stage/source only on
failure; success paths add no diagnostics.

The client resolves terminals from recovered source records, replacing the 11-map
list and fixed access coordinates. Click/keyboard interaction uses each source's
actual coordinate and faces its terminal. PC commands use the existing
scene-owned inventory coordinator for admission, correlation, stale views,
cancellation and current-state recovery. Selections hold Pokémon row IDs rather
than array indices. Pending controls disable, scene retirement closes the PC,
and closing a panel leaves sent mutation reconciliation alive. Global PC reply
handlers and independent party/box pushes are retired. Debug fixture publication
no longer emits the retired PC update; scene recovery supplies its PC cache.

Verification: real PostgreSQL dispatcher checks cover all operations, source
reach/facing/routine, battle ownership, old packets, duplicate revisions, coherent
recovery and rollback of membership/preference/revision after final commit
rejection, followed by retry. A deferred UPDATE rejection proves opening is a
read. World, pokebattle and cqitems race suites passed (41.786, 3.302 and 2.567
seconds); focused PC checks passed after read-lock hardening (2.122 seconds).
The shared coordinator has 134 passing checks, including all four PC consumers;
22 bridge checks include active forwarding and inert unsolicited PC replies.
Existing gameplay recovery/interaction tests, canonical generation, TypeScript,
build/asset validation and diff checks passed.

Both rendered PC cases passed in 12.1 seconds in
`/var/tmp/capturequest-rendered.jv0eJn`, using the new handlers and dynamic source
interaction. Earlier failures in `QnszzJ` and `WwrPGF` under the same evidence
prefix came from missing client bridge forwarding, not weakened source rules.
The captured request/response showed valid source 15 and a successful server
reply; adding the five replies to the shared bridge route fixed the timeout.
The initial movement-order hypothesis was disproven and no movement workaround
was added. The new bridge regression proves actual delivery to subscribed
listeners, not merely lack of global store mutation.

Remaining before closing this family: rendered duplicates/lost reply, actual
process death and reentry, additional source locations outside the previous
hardcoded subset, source/scene retirement races, and review of the completed
shared boundary. The old Go slot helper wrappers have only test callers now;
retire them and update those fixtures as cleanup. Debug writers and other
unmigrated families still have their separate ownership/revision audit. The full
five-area goal and original login timeout remain open. This checkpoint is local;
no push or production deployment was performed.

## Coherent PC recovery and opening reads (2026-10-07)

`GameplayStateResponse.pc` now contains the selected box, capacity, stable box
member IDs and source PC triggers for the owned map. The new shared
`readPCStorageIn` reader runs inside the same character transaction as party,
wallet, inventory and command-revision recovery. Existing PC opening uses that
reader and an injected, cancellable character transaction instead of independent
global reads. Missing preferences mean box zero; failed queries, out-of-range
preferences, malformed slots/members or unsupported source direction do not
produce an empty or successful partial snapshot.

Source records are selected from `phaser_hidden_objects` by the original
`OpenPokemonCenterPC` routine and PC type. Their IDs, map, coordinates and facing
are exposed through the generated contract for the upcoming authorization and
interaction migration. This **does not authorize access yet**: PC opening still
accepts the old request, and the four mutation/preference handlers remain legacy.
No hardcoded source actor or alternate recovery coordinator was introduced.

The existing client gameplay and inventory-command recovery paths apply PC state
without reopening a closed PC panel. Box/source changes participate in the same
stale-read detection as party and inventory changes. Snapshot validation rejects
missing PC state from an older server, malformed identities and foreign source
maps before publishing any gameplay projection. Matching server/client builds
are required; there is no fabricated empty-PC compatibility fallback.

Verification: PostgreSQL regressions exercise real recovery and opening handlers
with the global database disabled, prove coherent owned row identities and
nonstandard source coordinates, and reject preference, source, box-data and
storage-query failures without partial success. Focused checks passed in 3.629
seconds; the full world race suite passed in 46.048 seconds. All 127 affected
client recovery checks passed, including a PC refresh overtaking a read, closed
panel preservation, malformed/missing PC snapshots and foreign-map rejection.
Canonical type generation, TypeScript checks, asset validation, frontend build
and diff checks passed.

Two existing rendered PC cases passed in 12.0 seconds in
`/var/tmp/capturequest-rendered.nsCqDo`. The terminal/storage flow now also verifies
that source PC identity is present after map recovery while the panel remains
closed. Existing real opening, deposit, withdrawal, release and NPC interaction
checks passed. This is browser evidence for the recovery/read extension; it is
not duplicate/lost-reply/crash acceptance for the still-legacy PC commands.

Next: replace all five PC requests with source-authorized, correlated commands
using the existing shared revision/transaction boundary and one committed
box/party projection; move their client adapters onto the existing scene owner,
retire global PC reply handlers and slot-only requests, and verify cancellation,
stale replies, closed views and process-death recovery. The full roadmap and
original login timeout remain open. No push or deployment was performed.

## PC storage identity prerequisite and source audit (2026-10-07)

The three authoritative storage operations now target stable owned Pokémon row
IDs through `DepositPokemonToPCInTransaction`,
`WithdrawPokemonFromPCInTransaction` and `ReleasePokemonFromPCInTransaction`.
They require an existing transaction, share the character lock and retain the
existing free-slot allocation, capacity, last-party-member and party-compaction
rules. Ownership and current party/box membership are checked before changes;
malformed occupied box slots reject instead of being ignored during allocation.
No storage engine or commit/recovery coordinator was added.

The existing slot APIs resolve their selectors once under the same lock and
delegate to these operations. This preserves current callers while the transport
migration is prepared. It **does not fix slot-only network intent or duplicate
requests**: handlers still accept party/box slots, use global/background reads,
emit independent success/party/box packets and lack PC source authorization.
Those wrappers and replaced network paths must retire as the ID-based command
family migrates; they are not a permanent compatibility design.

Real PostgreSQL race regressions execute the new operations inside the existing
`cqitems.Store.ExecuteCommand` boundary. They prove stable targeting after party
reordering and PC slot reuse, rejection of old shared revisions and released,
foreign or misplaced targets, box/party capacity and last-member protection,
caller-owned transaction enforcement, blocked-lock cancellation, and rollback of
membership plus revision after bag projection, box projection or final commit
failure. Retry at the unchanged revision succeeds. All pokebattle and cqitems
race tests passed (3.117 and 2.550 seconds respectively); focused storage checks
passed again after malformed-slot validation and the added box-projection case
(2.314 seconds). Existing SQLite storage checks also passed. This is domain and
command-kernel evidence, not PC transport or rendered acceptance.

Source audit: the current extractor has **zero PC records in `objects`** but
**20 `hidden_objects` records** with `routine='OpenPokemonCenterPC'` and
`object_type='pc'`. PCs must be authorized by the hidden-object source identity,
not the NPC actor registry. The source in `data/events/hidden_objects.asm`,
`engine/overworld/hidden_objects.asm` and
`engine/events/hidden_objects/pokecenter_pc.asm` (extractor submodule revision
`ed8b7d58ab3cac46401beb7e53896a88f51b9139`) matches the coordinate in front of
the player and requires facing up. Most triggers are at `(13,3)`, but Indigo
Plateau Lobby is `(15,7)`, Silph 11F is `(10,12)`, and other triggers include
Safari rest houses, Celadon locations and the fossil room. The current client
hardcodes 11 center map IDs and one access tile. Preserve source rows as the
authority and replace that hardcoded subset as part of the migration.

Next: wire all five PC handlers through source authorization and the existing
shared command/revision boundary, with stable Pokémon IDs and one typed committed
box/party projection. Extend the current gameplay recovery model with PC state,
reuse the existing scene-owned command coordinator, and retire global PC reply
handlers and slot requests. Then verify real source interaction, rollback,
duplicates, cancellation, stale replies, closed views and crash/reentry through
the existing isolated harnesses. The full roadmap and login timeout remain open.
This prerequisite is committed locally; no push or deployment is part of it.

## Center healing implementation checkpoint (2026-10-07)

The 12 center nurses are now compiled as a complete source macro family from
`script_event_ir_blocks`, `text_pointers`, `maps` and `dialogue_text`. Translation
requires an exact supported macro body and consistent, unique map/text identity;
missing dialogue, changed bodies and duplicate records fail compilation. Events
use the canonical locked output plan. The current corpus generates 12 new scripts
and preserves 321 existing scripts; `--check` reports 333 unchanged candidates,
zero unsupported candidates and no overrides. Generated outputs remain ignored.

The client sprite shortcut, direct heal sender, fabricated dialogue and global
heal-response handler are retired. All nurse actors use the existing scripted
interaction flow, including the separately sourced Silph nurse. Reserved opcode
79 rejects stale callers without mutation. No new command/recovery coordinator
was added. Center scripts extend the shared `healParty` action with the explicit
`healingPolicy: "center"`; ordinary scripted and battle healing retain their
existing policy. Center healing rejects ordinary/Safari battle ownership,
changed source maps, empty parties and non-object options. It heals through the
existing party writer, merges center option keys without discarding unknown
options, resets defeated trainers and resolves the durable token in one commit.
In-memory trainer tracking clears only after commit.

Source provenance is the extractor submodule revision recorded in the handoff
below. Its `SetLastBlackoutMap` and `FlyWarpData` use outdoor source destinations.
This migration deliberately preserves CaptureQuest's existing center-interior
arrival `(3,4)` and trainer resets as explicit game policies; the legacy claim
that trainer resets reproduce cartridge behavior was incorrect. The dialogue
always presents the healing choice rather than introducing the cartridge's
first-visit welcome bit. Source artwork/animation fidelity is not established.

Verification so far: compiler regression tests and all compiler package tests,
focused real PostgreSQL race checks for center completion and existing durable
cutscene/interaction boundaries, nine client cutscene lifecycle tests, canonical
type generation, TypeScript checks, runtime asset validation and production
frontend build passed. Center regressions cover HP/PP/status restoration,
preserved options, cancelled tokens, stale opcode rejection, final-commit rollback,
completed-token replay after new damage/trainer wins, ordinary battle ownership,
changed maps, malformed options and unknown policy rejection.

At the implementation checkpoint, remaining acceptance was rendered nurse Yes/No and source reach/visibility checks,
lost issuance/completion delivery, actual process death and reentry, plus review
of the shared boundary before another family. Existing general cutscene recovery
evidence is reusable but does not establish those nurse-specific browser results.
The full five-area goal and login timeout investigation remain incomplete. A
future deployment needs the full-data lane because the compiler/action contract
and generated script family changed; no deployment is authorized here.

The follow-up below records that acceptance and shared-boundary review before
choosing the next command family.

### Nurse acceptance and shared issuance review (2026-10-07)

The new browser fixture places the character at the source counter opposite the
Viridian nurse. Tests use the existing private PostgreSQL/exact-process runner,
real actor clicks and rendered Yes/No controls. Injuries and trainer wins are
seeded only in the verified private database while the character is offline.
No new recovery coordinator or process-control mechanism was introduced.

The final two rendered cases passed in 31.8 seconds against the final server
source. Evidence is retained in `/var/tmp/capturequest-rendered.xOsPaR`:
remote nurse requests reject; source-counter clicks show Yes/No; No cancels the
durable token without healing, changing center options or resetting trainers;
Yes restores the party and persists the center/trainer policy. A hidden nurse
is absent from the actor view and rejects a stale actor ID. Inspected screenshots
show the nurse choice, full party HP and the nurse's absence under the override.
No retired heal command was sent. The lost-delivery case drops the first issued
script, resumes its exact token after reentry, then drops the successful completion
reply and kills the owned server before acknowledgement recovery. The receipt
records exit 137; the same private database survives restart and browser reentry
with full HP, cleared status, center options and the committed trainer reset.
The isolated runner stopped its own app/client/database processes on completion.

Earlier runs remain in `BMELm7`, `t2OPSO`, `BlbhDt` and `AIVcxP` under the same
`/var/tmp/capturequest-rendered.` prefix. They distinguish fixture corrections
(expected outage errors and an insufficient diagnostic snapshot radius) from the
actual visibility defect below. The error collector still rejects unrelated
errors; only exact WebSocket connection-refused errors and the corresponding
socket error are permitted inside the deliberate crash window. No assertion of
hidden-actor behavior was removed or relaxed. TypeScript checks, fixture usable-tile
validation and `git diff --check` passed. The complete world PostgreSQL race suite
passed again after the visibility fix (40.540 seconds).

Review found the generic scripted interaction replied `started: true` before
`SendCutsceneToPlayer` committed the durable token. The shared issuance transaction
is now exposed as a preparation step used by both that notification wrapper and
generic interactions. Generic interaction publishes success and the start payload
only after the token commit. A real deferred commit-rejection regression proves
failure produces only a negative interaction reply and no durable plan; retry
issues one recoverable plan. Focused PostgreSQL race checks and the complete world
race suite passed after this change (world: 42.657 seconds).

The rendered hidden-nurse check exposed a second shared defect: actor-list
filtering returned before loading per-character overrides when a map had no
source visibility rules. Interaction authorization did load those overrides, so
display and interaction disagreed. Removing that early return applies the same
override policy to ordinary interior and unified-overworld actors. Actor-list
requests now use the injected, cancellable visibility boundary and report its
errors; broadcast filtering logs visibility errors and withholds unverified
actors instead of publishing the unfiltered list. PostgreSQL regressions cover
maps without source rules, both map modes and missing override storage.

Special puzzle/card-key interaction handlers still perform their domain mutation
before separate dialogue issuance. Their atomic mutation/issuance audit remains
part of the wider script-writer inventory; this checkpoint does not close it.
Presentation dialogue/SFX still run before final cutscene completion, as in the
existing interpreter. Durable state, token resolution and projections depend on
the confirmed transaction; this work does not establish cartridge animation or
presentation timing fidelity.

This finishes the selected nurse migration and its planned acceptance checks,
not the full foundations goal or the entire scripted-event audit. The original
login timeout did not recur in these runs and remains unresolved. Next: audit PC
opening/source authorization and its stable Pokemon identities, then migrate PC
commands through the existing shared transaction and recovery boundaries with
coherent box/party projections. Other command and lifecycle work remains in
`SERVER_COMMAND_AUDIT.md`. These checkpoints are local; no further push or
production deployment was performed.

### GitHub handoff checkpoint (2026-10-03)

The user separately authorized committing and pushing the current stopping point
to a GitHub branch. The implementation checkpoint is `0d6b640` on
`codex/server-foundations`; the working tree was clean before this documentation
update. This handoff authorizes a branch push, with no production deployment.

Center healing remains inspection only. The current extractor SQLite database
contains 12 center nurse text blocks using the same `script_pokecenter_nurse`
macro, with source text pointers and dialogue available. The original macro and
healing sequence are in the extractor submodule's `macros/scripts/text.asm` and
`engine/events/pokecenter.asm` (submodule revision
`ed8b7d58ab3cac46401beb7e53896a88f51b9139`). The client sprite shortcut currently
bypasses that source interaction. The existing scripted-event system already
provides source-target authorization, durable issuance/completion and the shared
`applyHealPartyAction` transaction path.

Recommended next step: finish tracing choice cancellation and blackout destination
data, then compile the complete nurse macro family through the canonical script
compiler and existing scripted-event boundary. Make party healing, center options
and the explicit trainer-reset policy atomic; retire the legacy healing route and
verify rejection, rollback, duplicate completion and recovery. No nurse compiler,
runtime or client migration has been implemented or verified at this checkpoint.
The login timeout and the other remaining roadmap areas are still unresolved.

The bounded [Nakama feasibility assessment](NAKAMA_FEASIBILITY.md) is complete
(2026-10-03). It recommends consolidating purchase and party-item command handling
using the existing infrastructure before migrating another endpoint family.
Nakama's managed transactions would require a player-data migration; an RPC-only
integration would retain our SQL and recovery work. This is an evaluated
recommendation. The user subsequently authorized the bounded two-consumer
consolidation below, which is complete along with its shared-boundary review
fixes and the Repel migration. A framework migration remains outside scope.

Keep Go, PostgreSQL, one deployable server, and the authoritative extractor,
runtime asset, and scripted-action contracts. Improve runtime safety through
small verified changes. No production deployment or push is part of this goal.

## Active execution plan (2026-10-03)

Complete all five roadmap areas through shared authoritative transaction,
ownership, command identity and recovery primitives, with explicit domain rules.
Keep the reviewed shop/party-item/Repel boundary as the baseline. Remove replaced
paths as families migrate; do not accumulate independent recovery engines or
force movement and battle ownership into the inventory coordinator.

1. Investigate the unresolved five-second `restoreBattleOnLogin` timeout from
   Repel reentry. Use bounded reproduction and query, lock, connection-pool and
   session-lifecycle evidence to establish the cause. Add a focused regression
   with the fix. The later successful rerun does not close this issue.
2. Audit remaining command families and owners against the implementation and
   tests, producing a finite coverage matrix for the five-area table below.
   Distinguish completed behavior from unaudited paths and missing acceptance
   evidence rather than estimating progress from commit or test counts.
3. Choose and finish one coherent family at each checkpoint. Reuse the appropriate
   existing boundaries, retire the old route, and review changes to shared
   primitives before expanding to another family. Continue beyond an individual
   family until the full completion criteria below are satisfied.
4. Verify success, rejection, rollback, duplicates, timeout, cancellation,
   stale/late responses and scene/session/reconnect races, plus process-death
   recovery where relevant. Start with focused checks and use existing isolated
   transport/browser harnesses in proportion to risk. Distinguish headless,
   rendered and production evidence.
5. Make coherent local checkpoint commits and update this document with evidence,
   unresolved issues, remaining scope and the next step. Reassess the architecture
   or workflow when progress slows unexpectedly or repeated orchestration and
   test scaffolding begin accumulating across families.

No replacement framework, broker, event-sourcing architecture, push, deployment
or production mutation is authorized by this goal. Preserve unrelated work and
the canonical generated-data contracts. Production validation remains a separate
deployment task; the local goal must record that verification limit explicitly.

Initial investigation: source tracing confirms login calls `ResumeBattle`, which
uses `db.Transaction` and locks `character_data` before reading the saved battle,
including when no battle exists. The shared transaction starts its five-second
deadline before `BeginTx`; an earlier session deadline can shorten it. Thus the
observed error alone cannot distinguish connection acquisition, the character
lock, later queries or commit. The next diagnostic must identify the failing
stage and any blocking owner before changing deadlines or recovery policy.

### Login timeout investigation and remaining-family inventory

The first bounded reproduction completed eight Repel browser cases (four normal
reentries and four lost-reply/crash/restart cases) in 1.9 minutes without another
timeout. Evidence is retained at `/var/tmp/capturequest-rendered.p9383nht`, using
a copy of the stopped private test database and a fresh server build. PostgreSQL
logged lock waits over 100 ms and statements over 100 ms throughout the run. The
only recorded slow statements were startup tile-snapshot maintenance (645–1,059
ms); no login failure or lock wait was captured. An initial activity-sampling
command failed because separate `psql -c` calls do not retain the query for
`\watch`; corrected sampling covered only the end of the run and found no
qualifying character/battle query. It is not evidence for the earlier cases.

Shared transaction errors now identify begin, character-lock and commit failures
while retaining wrapped error identity. Login restore failures report total login
and restore durations plus database pool statistics; deltas are pool-wide, not
attribution to one character. Success-path logging and deadlines are unchanged.
Focused PostgreSQL race tests cover pool exhaustion, lock timeout, commit
rejection, rollback/recovery, inventory command execution and character handoff.
The original timeout remains **unresolved**, with better evidence available on
recurrence; a passing stress run does not establish its cause. Further blind
repetition is not justified by this result.

[SERVER_COMMAND_AUDIT.md](SERVER_COMMAND_AUDIT.md) accounts for all 87 registered
commands plus HTTP and background-owner work. It distinguishes implementation
evidence from acceptance evidence and unaudited paths. The first selected command
family was party reordering: its slot-index request could be replayed against a
different ordering, and it fits the existing party/inventory recovery projection.
That migration is recorded below. PC storage and center healing remain explicit
subsequent work, with their own authorization and projection requirements.

## Party ordering through the shared command boundary (2026-10-03)

The previous drag handler changed the shared party optimistically and sent only
slot indices. Repeating `[1,2,0]` after the first reorder permuted the new party
again. Replies were uncorrelated and applied globally, so late packets could
replace a newer party view. The server loaded the party before its save transaction
and used the global database handle.

Opcode 90/91 now carries the complete desired order as stable `pokemonIds`, the
shared character/revision identity and a correlation ID. The domain callback
validates exact current membership under the character lock and reuses the
existing atomic party writer. Ordinary and Safari battle ownership, including a
terminal record awaiting dismissal, prevents reordering. Failed domain writes,
bag projection or commit roll back both the order and the revision. The response
contains one committed party/bag snapshot; old index-only requests reject.

The HUD keeps its drag preview local. A changed source party cancels the drag,
and a pending inventory-family command prevents another admission. The existing
client coordinator applies the acknowledgement or reads current gameplay state
after a lost/rejected reply. The legacy global reply handler and optimistic store
write are removed. No new retry loop, command counter, database schema or asset
catalog was introduced.

Shared-boundary review: the executor and coordinator algorithms are unchanged;
the new consumer supplies stable-membership rules and a small projection/sound
adapter. Recovery already contains party state, so no parallel recovery model is
needed. This does not migrate independent legacy party reads or other party
writers; their broader stale-projection audit remains explicit in the matrix.

Verification:

- Full PostgreSQL race suites passed for db, cqitems, economy, itemuse, pokebattle
  and world through `run-go-postgres.sh` (world: 40.310 seconds). New dispatcher
  checks cover stable final ordering, old/duplicate requests, foreign identity,
  malformed membership, shared revisions, concurrent duplicates, blocked-lock
  cancellation, domain/projection/commit rollback and durable battle ownership.
- 109 focused client checks passed. Reordering uses the same admission, timeout,
  rejection, malformed/stale response, scene/character retirement and recovery
  matrix as the other command consumers. Invalid/missing IDs and unsolicited
  reordered-party replies are covered.
- Canonical `npm run tygo`, typecheck and production build with runtime asset
  validation passed. Existing dynamic-import/chunk-size warnings remain.
- Four rendered cases passed in `/var/tmp/capturequest-rendered.zlr6gcqp`: two
  existing party-item cases and two new party-order cases. The new tests drag the
  real HUD, duplicate each command, recover a lost reply without resending, apply
  a distinct second order, verify late-reply retirement and survive reentry. The
  lost-reply case kills the exact owned server after commit and before delivery;
  its restart receipt records exit 137. Inspected screenshots show the new party
  order. Both final party-order cases passed again in 26.6 seconds with an added
  real party refresh during a drag, proving that the stale preview cancels before
  any mutation is sent. That evidence is under `playwright-drag-retry`; the
  earlier `playwright` evidence and all three crash receipts are retained.

Next: center healing's source authorization and atomic execution. Inspect the
existing nurse scripts and `applyHealPartyAction` first: the current client
special-cases `SPRITE_NURSE` into a separate legacy handler. Consolidate that path
with the authoritative interaction/mutation system. The login timeout and all
five broad roadmap areas remain open. No push or deployment is performed.

## Repel command migration (2026-10-03)

The user authorized choosing and completing one additional family after the
shared-boundary fixes. Chosen scope: Repel, Super Repel and Max Repel activation.
The broader roadmap remains incomplete; this checkpoint adds no framework,
database schema or asset-catalog changes.

Why this family: the existing Repel transaction already owned consumption and the
durable `character_repels` counter, but two uncorrelated entry routes remained.
The active-effect guard prevented concurrent uses only until expiry. A delayed
old request could then consume another item, and lost quantity-only replies had
no scene-owned reconciliation. PC transfers need an additional box-state recovery
contract; Escape Rope, Bicycle and fishing affect movement or battle ownership.
Repel fits the current inventory boundary without widening its lifecycle policy.

Implemented:

- One typed activation route on opcode 143/144 carries the owned `instanceId`,
  correlation ID and shared `{characterId, revision}` identity. Both old request
  forms (item-ID-only opcode 143 and uncorrelated item use on opcode 100) reject
  without mutation. The inventory UI and simulator now use the same route.
- `UseRepelInventoryItem` delegates transaction, locking, revision admission and
  final bag projection to `cqitems.Store.ExecuteCommand`. Its existing ownership,
  battle, active-effect and 100/200/250-step rules remain explicit. Expiry retains
  the committed revision, so old requests remain stale after wear-off or restart.
- The existing `InventoryCommandService` owns admission, acknowledgement,
  cancellation and current-state recovery. Repel adds only its result validation,
  activation presentation and bag-close presentation watcher. No separate command
  coordinator, retry loop or recovery store is introduced. The global activation
  presenter and legacy quantity-only Repel publication are removed.
- Simulator fixtures reset their command counter along with their other owned
  state. Golden output aliases runtime inventory-instance IDs while preserving
  equality within the reply; raw packet assertions still inspect the original
  IDs and contents. This avoids dependence on sequence allocations from earlier
  scenarios. Both changed goldens were regenerated through the simulator.

Before/after: Repel's server operation replaces its own transaction/lock and two
item-lookup modes with one shared-executor callback; the old field-item adapter is
deleted. The client adds one small domain adapter to the existing coordinator and
routes its opcode through the existing listener collection. Neither the executor
nor the coordinator's admission/recovery algorithm needed new family-specific
branches or policy flags.

Verification completed:

- PostgreSQL `-race` suites for cqitems, economy, itemuse, world and scriptsim
  passed through `scripts/testing/run-go-postgres.sh`; world took 38.820 seconds.
  Repel checks cover atomic rollback on projection/commit failure, cancellation,
  concurrent duplicates, malformed/old identity, foreign instance ownership,
  battle rejection, durable expiry, all three durations, a fresh use after expiry
  and cross-family revision exclusion with shop sales.
- 90 focused client checks passed across InventoryCommandService,
  GameplayRecoveryService and NetworkBridge.inventory. Repel participates in the
  existing shared success/timeout/rejection/malformed/stale/send-failure and
  scene/character-retirement matrix. Additional cases cover bag closure during
  acknowledgement/recovery, suppressed late sounds and invalid Repel outcomes.
- All three Repel simulator scenarios passed on the private database retained
  at `/var/tmp/capturequest-script-sim.mPM4Yt`. Activation passed twice more on that
  same database, proving fixture revision reset and stable ID presentation.
- Canonical `npm run tygo`, TypeScript and the production build (including runtime
  asset validation) passed. Existing dynamic-import/chunk-size warnings remain.

Rendered acceptance covered six cases across the existing inventory suite and
`repel-command-recovery.spec.ts`. The four existing inventory cases passed in
`/var/tmp/capturequest-rendered.VIVqLl`. The two new cases initially stopped at an
incorrect test expectation for a visible `×1` label: the inventory intentionally
renders quantities only above one. The corrected assertion expects the single
item's name while retaining the exact state quantity check.

Both new cases passed on the unchanged runtime in 29.2 seconds using the already
bootstrapped private cluster at `/var/tmp/capturequest-rendered.tUpQSX`, with
screenshots under `playwright-retry` and earlier failure traces under `playwright`.
They duplicate each activation, prove
timeout recovery while the bag is closed, allow a second intention after an
explicit private-database expiry fixture, and retain bag/currency/counter/revision
after reentry. The lost-reply case kills the exact owned server after its second
commit but before acknowledgement/recovery; the restart receipt records exit 137.
Two successful runs of that case produced separate kill/restart receipts in
`process-recovery-evidence.json`. The final screenshot was inspected and shows
the recovered empty bag. Actual movement-driven expiry remains covered by the Go
tests and simulator rather than the browser's explicit expiry fixture.

Verification caveat: the first corrected run's normal-reentry case hit one
five-second `restoreBattleOnLogin` database timeout. The lost-reply/crash case in
that same run passed. Rerunning both cases on the same isolated cluster with
PostgreSQL lock-wait diagnostics passed without runtime changes or captured lock
waits. The cause of that one timeout is not established or claimed fixed here.

`git diff --check` passed. All evidence is local/disposable-database evidence;
production was not exercised. This checkpoint is committed locally on
`codex/server-foundations`.

Remaining: Escape Rope, Bicycle, fishing and other legacy inventory/party writers
still have separate contracts and are outside this revision's scope. Repel's
counter remains authoritative on the server; the browser has no counter display
or local timer to recover. Matching frontend/backend activation and a client
refresh are required for the changed request/reply contract. No push or deployment
is performed. Next: assess this completed family before choosing another bounded
migration; the broad ownership/recovery roadmap below remains incomplete.

## Shared inventory command checkpoint (2026-10-03)

Scope: complete the finite milestone from NAKAMA_FEASIBILITY.md, then stop before
migrating another command family. The broader five-area goal remains paused and
incomplete. This work adds no framework dependency, database migration, broker,
event store or parallel party-item lifecycle.

Implemented:

- `cqitems.Store.ExecuteCommand` owns one transaction/lock/revision/projection
  boundary used by purchase, sale and outside-battle party-item services. Reuse the
  existing `character_shop_state` counter and reject stale revisions across both
  consumers; recover current state instead of replaying historical results.
- One `InventoryCommandService` owns client admission, correlation, cancellation,
  stale-response checks, pending/error state and current bag/party/wallet recovery.
  Shop opening retains read-only failure behavior through that same owner.
- Party-item commands carry durable Pokémon row identity and return one correlated
  committed bag/party/outcome packet. Pending PP/TM selection retains its target.
  Existing medicine, TM/HM, evolution and Flute policies stay in `itemuse`.
- The former global party-item quantity/prompt handler and separate party/bag
  publications are retired. Unmigrated field-item effects remain explicitly
  separate on their existing protocol; correlated party commands cannot enter it.

Before/after review at `5c8da47`: the shop client module falls from 128 to 50 lines, containing
only its policy and presentation. Its scene/request/recovery ownership moves into
one 130-line coordinator that also supplies party use. This is not a net reduction
in those two files (180 versus 128 lines): party commands gain protections they
previously lacked. The removed global TM prompt handler and direct UI party sends
also reduce duplication. On the server, three consumers now call one executor;
the economy-specific revision helper and repeated lock/projection orchestration
are gone. Domain SQL and item effects remain explicit rather than encoded in
operation-specific switches inside the executor.

Verification completed:

- PostgreSQL `-race` suites: `./internal/db/cqitems`, `./internal/economy`,
  `./internal/itemuse` and `./internal/world` passed through
  `scripts/testing/run-go-postgres.sh`. Shared tests inject domain, final-read,
  commit and cancellation failures; consumer fixtures prove concurrent duplicate
  rejection, two intentional uses and cross-consumer revision exclusion. The
  world suite passed in 37.208 seconds; a final focused item/field dispatch run
  passed after removing the redundant pre-transaction party-item lookup.
- 97 focused client tests passed across InventoryCommandService,
  GameplayRecoveryService, NetworkBridge.inventory, BattleCommandService and
  PokeBattleStore.recovery. The shared consumer matrix covers success, timeout,
  rejection, malformed/overtaken replies, send failure, scene/character retirement,
  retirement during recovery, late replies and recovery failure. Shop-specific
  close/menu checks and TM target retention remain explicit.
- `npm run tygo`, `npx tsc --noEmit`, `npm run build` (including runtime asset
  validation) and `git diff --check` passed. The build retains its existing chunk
  size warning; this checkpoint makes no bundle-size claim.
- Rendered acceptance passed for all 12 selected checks across two runs:
  `inventory-items.spec.ts`, `shop-inventory-recovery.spec.ts`,
  `shop-process-recovery.spec.ts` and `party-item-recovery.spec.ts`, using
  `CQ_E2E_CRASH_RECOVERY=true bash scripts/testing/run-isolated-e2e.sh ...`.
  Shop evidence is `/var/tmp/capturequest-rendered.dm1kKd` (10 passing cases,
  two actual SIGKILL/restarts). The two initial Potion cases failed at fixture
  setup: the browser debugger's Pokemon fixture type does not support `curHp`.
  The corrected tests injure the verified private database while the character
  is offline; they do not change runtime healing or debug-fixture behavior.
- Potion rerun: `/var/tmp/capturequest-rendered.PKgmuQ`, two passing cases in
  40.3 seconds. Both duplicate each request, exercise two intentional heals
  (1 → 21 → 41 HP), preserve Pokemon row identity and currency, reject historical
  acknowledgement application, and verify an empty bag after character reentry.
  The withheld-reply case also verifies reentry after a third actual SIGKILL.
  Its restart occurs after current-state recovery; shop acceptance covers the
  stricter crash-before-acknowledgement boundary. The recovered party/bag
  screenshot was inspected. These are local rendered and isolated-DB checks,
  not production deployment evidence.

The finite milestone is complete. No additional command family is authorized by
this checkpoint.

The subsequent review found two central lifecycle gaps, fixed in the follow-up
below. Remaining after that follow-up: select the next bounded family with user
authorization. Field effects, battle/movement coordinators, legacy
standalone notifications and the five-area ownership/recovery audit below have
not been folded into this executor. The revision is scoped to migrated commands,
not every writer of inventory or party state. Browser acceptance uses Potion as
the representative party item; TM/HM/evolution/Flute retain service-level tests.
Frontend/backend wire changes require coordinated activation and refreshed clients.
No push or production deployment is authorized or performed.

### Shared boundary review follow-up (2026-10-03)

Scope: fix the two reviewed boundaries and add regression checks before any
further family migration. The broader goal remains paused and incomplete.

- Shop closure previously aborted a sent purchase/sale request and any recovery
  read. A server commit could therefore leave the browser's bag and revision
  stale; reopening the merchant refreshed only money. The shared coordinator now
  separates presentation retirement from mutation ownership: closure suppresses
  presentation, while reply handling or timeout recovery reconciles current state
  and holds admission until finished. Domain projection application is separate
  from presentation. Read-only menu closure and character/scene cancellation
  retain their existing behavior.
- The command executor previously accepted `DBTX` parent transactions through the
  repository's joining helper. It could return success before the parent's commit
  or rollback. It now requires a database pool, rejecting raw and wrapped parent
  transactions before effects or revision writes. Repository helpers inside the
  owned callback continue to join its transaction.

Regression evidence: the new tests first failed against the reviewed code for
both raw/wrapped parent handles and all five client closure cases. After the fix,
PostgreSQL race suites for cqitems, economy and itemuse passed, as did 73 focused
client tests across InventoryCommandService, GameplayRecoveryService and
NetworkBridge.inventory, plus TypeScript checking. Tests cover delayed/lost buy
and sell replies after closure, closure during recovery, fresh revision on the
next command, suppressed sounds, retained scene/character cancellation and late
menu rejection. Server checks prove rejected parent transactions remain usable
and unchanged, and an independent connection sees successful command writes as
soon as the executor returns.

Rendered verification: all seven checks in `shop-inventory-recovery.spec.ts`
passed through `bash scripts/testing/run-isolated-e2e.sh` (1.2 minutes), with
evidence retained at `/var/tmp/capturequest-rendered.ZhLhHd`. The two new cases
close the rendered shop using EXIT/Escape after a real server commit, then
deliver the delayed acknowledgement or allow timeout recovery. They verify
reconciliation while closed, reopen through the clerk and complete a second
purchase at revision 2. Duplicate sends still produce exactly one commit per
intentional purchase. The timeout-case screenshot was inspected: the reopened
merchant and trainer wallet both show ¥9,600 after two ¥200 purchases. Existing
buy/sell duplicate, lost-reply, reentry and retired-menu checks also pass.

`npm run build` (including runtime asset validation) and `git diff --check`
passed. Existing dynamic-import/chunk-size warnings remain. These checks use
local rendered clients and disposable PostgreSQL; production was not exercised.
This follow-up is a local checkpoint on `codex/server-foundations`.

Next step: review this bounded follow-up and choose one subsequent family before
resuming migration. No additional family, framework, SQL schema or wire-contract
change is included; no push or production deployment is performed.

## Current scope and status

We are making the existing server reliable when requests overlap, connections
are replaced, database operations fail, and the process starts or stops. The
intended result is one authoritative gameplay state, atomic durable changes,
explicit transport contracts, and lifecycle behavior that can be verified.
Go, PostgreSQL, the single-server deployment and the content pipeline remain
the architectural foundation. Runtime design is described in
[`ARCHITECTURE.md`](ARCHITECTURE.md).

The full roadmap is **incomplete**. A completed checkpoint proves its documented
behavior; it does not prove that every gameplay path has migrated. The summary
below is the current handoff. Later checkpoint entries preserve historical
evidence, including remaining-work notes that subsequent commits may resolve.

### Checkpoint handoff (2026-10-03)

Implementation checkpoints are committed locally on `codex/server-foundations`.
The broad goal is active and incomplete; no push or deployment is part of these checkpoints.
MapLoad now rejects supplied destinations. Ordinary walking requests a direction
from its expected owned source, animates a server-issued step, then acknowledges
its token before continuing the path. Blackout, Safari and explicit warp commands
publish server-committed destinations.

Facing now uses expected-source direction requests (191/192), without saving
client coordinates. Server-path points commit before owned-state publication and
project through a dedicated origin notification (193). CutsceneSpriteController
no longer reports animation tiles through opcode 45;
completion applies the captured script from its issued owned source. Opcode 45
is now retired at the session boundary, and its handler, request DTO and browser
send helper are removed. Surf and committed warp exit animations use
explicit server projection; local teleport events require committed-result provenance.
Cutscene completion returns a correlated committed
result and reconciles owned position before unlocking. Issued ordinary walking,
forced path points and target-based Surf entry now share one transaction for their
position and applicable step effects. The latest ordinary-step commit now retains
one durable receipt per character. Duplicate completion acknowledges that receipt
without repeating effects. Recovery reads return current owned position and the
matching historical receipt separately. Sight-triggered trainer plans now persist
with the movement transaction and
resume from map-script or owned-position reads after owner replacement. Readiness
is token-bound and resolves the plan atomically with battle creation or blackout.
Cutscene snapshots and completion outcomes now survive owner replacement;
coordinate issuance joins movement commits and retries return current ownership.
Ordinary battle commands now await correlated replies, recover current state after
a timeout without resending mutations, and retire replies when their scene or
presentation is replaced. Capture placement now survives lost replies and
character reentry, with explicit party/PC summary dismissal. Login retains
terminal battles for coherent scene recovery and atomic post-battle plan issuance.
Safari actions and dismissal now share the correlated command coordinator, with
durable identity/revision guards and terminal catch placement recovery. Terminal Safari Run, party/PC captures, pending move choices, one issued Oak's Lab cutscene and committed shop buy/sale commands now have actual crash/restart acceptance. Ordinary walking now has acceptance for abandoning uncompleted tokens and recovering committed receipts without repeating Safari counters. Local/test party and inventory setup both belong to creation; reentry preserves intentionally empty parties.
The coherent recovery response now includes inventory, wallet, the full party and
sorted flags. Inventory reads share one bounded transactional reader, and shop
buy/sell replies include their complete committed bag without client stack guessing.
Shop mutations now require character-bound durable revisions and correlated replies;
the client waits for acknowledgement or current-state recovery without resending. Merchant opening and buy/sell commands now use source clerk reach and fresh script eligibility with scene-bound correlated replies. Integration with remaining legacy mutation timeouts and remaining
process-recovery coverage still need work. Evidence and verification limits
appear in the checkpoint sections below.

There is no reliable overall completion percentage: the remaining endpoint
and ownership audits can reveal additional work. Use the five-area status
table and the acceptance checks below to assess completion, rather than the
number of commits or passing tests. All five areas still have outstanding work.

| Area | Implemented | Still required |
| --- | --- | --- |
| Request/session boundary | Packet and connection limits, centralized session prerequisites, removal of insecure session takeover, actual transport closure, location/visibility checks for merchant opening and buy/sell commands, scripted clicks, dialogue choices and direct trainer battles, client destination catalog validation, server-resolved normal warp activation, explicit Instant Warp commands, committed teleport notification contracts and read-only map metadata, retired coordinate/map setters, and preserved command deadlines/disconnect cancellation in migrated operations. | Audit remaining interaction/mutation endpoints; propagate cancellation through legacy managers and remaining database/network work. |
| Durable gameplay | Shared bounded transactions; atomic shops/inventory, stable Pokémon row identities, party/item changes, battle persistence, script rewards/completion, trade rollback/deduplication, atomic Vermilion puzzle transitions, item-ball collection, Silph doors, Game Corner prizes and bounded coin/slot/hidden-coin operations, atomic Escape Rope/FLY positions, durable Repel counters, Safari entry/turn/capture state and exhaustion destinations, atomic blackout/recovery destinations and map-load position/Safari/flag/visibility/boulder effects, atomic movement-step counters, encounters and recovery, durable latest ordinary-step receipts, durable sight-trainer plans/resumption and atomic readiness resolution, durable cutscene snapshots/completion receipts/cancellation, coherent battle/Safari/pending-plan recovery, mandatory ordinary/Safari battle command identity, correlated battle timeout recovery and retained Safari terminal/capture state, recoverable terminal dismissal and atomic post-battle plans, and commit-before-publication in migrated paths. | Finish remaining dynamic puzzles, pickups, prize/field-effect paths; durable duplicate protection and recovery for remaining mutations across reconnects; finish recovery integration for remaining inventory/wallet/flag mutations and presentation, finish process-death acceptance for remaining movement/script/trainer plans and choices; finish queued-plan/source/catalog ordering acceptance coverage and remaining command recovery. |
| Character ownership | Bounded serialized session commands, exclusive character ownership and drained handoff, stale-cleanup guards, immutable cross-session presence, movement ticks coordinated with the owner, and immediate retirement of battle-scene command admission/subscriptions. | Finish timer/callback/shared-state and legacy position-writer audits; prove remaining concurrent/reconnect behavior across real transports. |
| Domains and wire contracts | Injected content-query service; typed inventory, shop opening and mutation successes, character/wallet/bind, Pokédex/card, content detail, map-script, map-info/list, sight-trainer notification/readiness, coherent gameplay recovery, ordinary/Safari battle command replies, shared battle events and learnset contracts generated from explicit JSON names. | Migrate remaining gameplay/query families and global dependencies; retire `StructToMap` and the casing postprocessor after every consumer moves. |
| Lifecycle and verification | Owned HTTP/listeners, readiness, listener failure propagation, joined periodic workers, sealed session admissions, fail-closed staged preload, startup cancellation, atomic scripted-event publication, and deadline-aware shutdown waits with returned failure results. | Audit cancellation of remaining legacy work, define durable final-save recovery, and complete transport/rendered integration coverage. Owned HTTP and player transport retirement and isolated active-player shutdown checks have landed. |

### Next work and completion criteria

The two-consumer consolidation recommended by
[NAKAMA_FEASIBILITY.md](NAKAMA_FEASIBILITY.md#finite-next-milestone-and-stop-condition)
has landed. Shop, party-item and Repel commands now share server transaction and
revision ownership and client admission, correlation and recovery. The reviewed
boundary rejects execution inside a caller-owned transaction and keeps committed
state reconciliation alive when its presentation closes. Repel now has duplicate,
lost-reply, reentry and actual crash/restart acceptance. The unresolved login
restore timeout from that acceptance run is the first investigation in the active
plan above; its cause is still unknown.

The bounded login investigation and remaining-family inventory are recorded
above. Party ordering, center healing, PC transfers, Escape Rope and Bicycle now
have their own documented checkpoints; the finite current inventory is
[SERVER_COMMAND_AUDIT.md](SERVER_COMMAND_AUDIT.md). Follow that inventory and the
latest checkpoint sections instead of treating these completed migrations as
future work. The sale browser check exercises the existing coordinator and
rendered balance; the product still has no Sell button.

Remaining work includes recovery integration for other mutation commands and
script/trainer plans and their queue/source/catalog ordering. Ordinary
walking, pending move choices and one issued cutscene now have two-crash
acceptance as documented below; that evidence does not cover every movement or
issued-plan family.

The current-state response now covers inventory, wallet, party and flags. Audit
remaining mutation endpoints for identity, deadlines, cancellation and timeout
recovery, along with timers/callbacks and global domain dependencies. Migrate each remaining wire family before removing `StructToMap`
and the casing postprocessor. Keep the five-area table current as those audits
identify or resolve gaps.

The goal can close only when every area has an explicit resolved audit and its
applicable observable acceptance evidence. Record any intentionally retained
legacy behavior and verification limits; a narrow passing checkpoint does not
close the broad goal. Production validation belongs to a separately authorized
deployment. Current work is committed locally; nothing has been pushed or
deployed by this goal.

## Shared item ownership and bounded dispatch (2026-10-03)

The earlier instance lookup trusted `cq_character_inventory.character_id` and
the instance ID without requiring `cq_item_instances.owner_id` and `owner_type`
to agree. Party use added a later ownership check, but reusable field effects
such as Bicycle could run directly after the weaker lookup.

`cqitems.Store.FindInventoryItemByInstanceIDContext` now requires both ownership
records to agree and a positive quantity. It joins an existing transaction or
owns a bounded transaction; the legacy API delegates to the same reader. The
redundant party-only ownership query is removed. Item-use dispatch rejects
unknown JSON fields, bounds the read with the session command context and a
five-second deadline, and passes its remaining deadline into party use. Missing
items receive an inventory rejection; database failures receive a retry message
and a concise server log rather than masquerading as missing data.

Verification covers real registry dispatch for foreign-owner, non-character and
zero-quantity Bicycle instances, malformed requests, database read failure and
successful reusable ownership without consumption. Repository tests check both
reader APIs and cancellation of a PostgreSQL read blocked by a table lock.
These are packet/database checks, not rendered browser or production evidence.
Focused checks and the complete `cqitems`, `itemuse`, `economy` and `world`
suites passed with the race detector against isolated PostgreSQL; the world suite
completed in 37.318 seconds. `git diff --check` passed. No frontend source or
wire contract changed; verification for this checkpoint is Go/packet/database
coverage, with no new rendered browser or frontend production-build check.

Remaining: party/field requests still lack durable command identity, correlated
typed outcomes and lost-reply recovery. Party publication still sends separate
party/inventory notifications. Some field effects still use legacy global reads
or separate contexts; the dispatch deadline does not yet govern every field
operation. This checkpoint does not complete the party/field migration or any
of the five broad goal areas.

## Committed shop buy/sale process-death acceptance (2026-10-03)

`tests/e2e/shop-process-recovery.spec.ts` reuses the existing exact-process private
runtime helper and source shop fixture. It withholds a committed purchase reply,
checks PostgreSQL independently, then asks the shell owner to SIGKILL and reap its
recorded server child. It enters the same character through a fresh authenticated
page without rerunning the fixture. It then repeats that boundary for a committed
whole-stack sale and enters again after the second restart.

The final run passed in 19.0 seconds at
`/var/tmp/capturequest-rendered.4iV6gX`. Actual process receipts record
`3099399 -> 3100101 -> 3100392`, with exit 137 at both crash boundaries and one
unchanged private PostgreSQL cluster. The database went from money 10,000,
revision 0 and instance 139 with 95 source POKE_BALL items, to money 8,000,
revision 1 and instances 139/140 with 99/6 items. Selling instance 140 committed
600 credit, money 8,600 and revision 2, retaining instance 139 with 99 items.
Ownership and unlimited source stock stayed unchanged. These exact row IDs are
run evidence, not production identities or fixture requirements.

After each restart, the test obtains an acknowledged current runtime clerk ID
and retries the old durable revision. It requires the economy rejection, avoiding
an obsolete actor-ID rejection that could mask a missing revision guard. The
stale sale targets the still-owned 99-item stack, so a missing sold instance cannot
mask that guard either. Independent PostgreSQL snapshots stay identical after
reentry and rejection. Only the original buy/sale and their explicit stale probes
are sent; the coordinator does not resend lost mutations.

The final scene receives the original withheld purchase packet through its real
socket; a diagnostic listener proves delivery before asserting that the bag and
rendered 8,600 balance remain current. The final screenshot was inspected, the
shop exits normally, and authenticated logout succeeds. The exact final server,
client and private PostgreSQL stopped. An initial run reached those recovery
assertions but failed during cleanup because the open shop overlay blocked Quit;
the test was repaired to click the existing Exit control, not weaken assertions.
`git diff --check` passed. No runtime source changed in this acceptance checkpoint;
the canonical harness built and checked the existing runtime and asset family.

Limits: both SIGKILLs occur after a transaction committed, not during an
uncommitted transaction. Existing PostgreSQL rollback/commit-failure suites cover
those transaction boundaries separately. The unlimited-stock source fixture does
not establish finite-stock crash behavior. The sale uses the existing coordinator
because the product has no Sell button. This is isolated rendered/transport/database
acceptance, not live deployment evidence or completion of the full goal.

Remaining: party/field item identity and timeout recovery, remaining callbacks,
global/domain/wire migrations, queued script/trainer plan ordering and remaining
lifecycle acceptance in the five-area table. Recommended next step: migrate
party/field item commands through the shared ownership and recovery boundaries.
Local checkpoint only; nothing pushed or deployed.

## Per-command clerk authorization and source sale policy (2026-10-03)

The original shop engine at
`tools/pokemon-gameboy-extractor-tool/pokemon-game-data/engine/events/pokemart.asm`
provides the sell menu through the clerk interaction. It builds that menu from the
bag, independently of the shop's buy catalog, rejects key items and HMs, and halves
item prices. CaptureQuest retains its existing whole-stack sale contract and
minimum price policy; source quantity selection and a new Sell UI are not added
by this server checkpoint.

Buy and sell now require the runtime `actorId` in their generated request types.
Opening, buying and selling share one source authorization helper that rechecks
reach, visibility and fresh script eligibility on every command. Active battles
reject all three shop command families, matching field-item admission. Each command's
single session-derived five-second deadline includes authorization and the
transaction. The economy purchase uses the authorized actor's map, and the sale
transaction requires a canonical merchant on that map. An opened menu cannot
grant permission after movement, actor hiding or a script eligibility change.
The same durable character/revision guard still commits with all effects and the
complete inventory result. Source HMs are explicitly unsellable even with a
positive price and a false key-item bit. The canonical HM category is shared by
item use and economy and generated for TypeScript.

The client store records the acknowledged menu's actor ID and clears it on close.
The existing scene coordinator includes it in both commands and refuses admission
without a valid live actor context. It retains existing correlation, cancellation,
current-state recovery and no-resend behavior. Server authorization remains the
boundary; this client field is a selector, not a permission token.

Verification: complete isolated PostgreSQL race suites passed for world
(38.496 seconds), economy (2.743 seconds), item use (1.541 seconds) and inventory
repository (1.574 seconds). New packet checks prove that a prior open cannot
bypass moved/remote/non-clerk/hidden actors or newly eligible scripts, and that
rejection leaves money, inventory and durable revision unchanged. The final
packet check also proves active battle rejection. Domain checks
reject missing merchants, key items and positive-price HMs, and accept ordinary
items absent from the buy catalog. Existing rollback/duplicate/concurrency checks
remain passing. Seventy frontend tests across five files, canonical type
regeneration, typecheck, runtime asset validation, production build (3.55 seconds)
and `git diff --check` passed.

Five rendered transport cases passed in 48.6 seconds at
`/var/tmp/capturequest-rendered.QJ884Q`: delivered/lost buy replies, delivered/lost
sale replies and verified late-menu delivery after scene reentry. Each mutation
packet is delivered twice with the same identity and commits once. The sale of
the fixture's 95 POKE_BALL stack credits 9,500, removes the whole stack, advances
revision to 1, recovers the rendered balance/bag after a lost reply, ignores the
late success and persists across authenticated reentry. The test calls the
existing sale coordinator because there is no Sell button; it does not claim UI
sale selection. The final active-battle admission check was added afterward and
verified by the final PostgreSQL packet/race suite. The exact private browser
runtime and PostgreSQL cluster stopped.

Remaining: actual shop process-death acceptance, party/field item identity and
recovery, remaining callback/domain/wire migrations, plan ordering and lifecycle
work in the five-area table. The full goal remains active. Frontend and backend
must activate together through a separately authorized release; no push or
deployment was performed. Recommended next step: shop process-death acceptance.

## Source-authorized merchant opening and scene retirement (2026-10-03)

Opening requires `requestId`, `characterId` and the runtime `actorId`; map and
merchant selectors are removed from the transport. The server maps that actor
through the NPC registry and reuses the shared source visibility/position and
adjacency/counter rule. It requires `SPRITE_CLERK` and a merchant on that actor's
source map. A matching active clerk script blocks fallback shop opening. A fresh,
private flag view supplies both visibility and script eligibility, so a stale
session cache cannot grant or deny that fallback. The shared interaction reader
accepts this view without replacing another caller's cache.

The shared click-script resolver now supports a bounded transaction and returns
eligibility errors. Previously the boolean resolver hid database errors as an
ineligible script; merchant fallback must fail closed instead. The entire open
operation shares a five-second session-derived deadline across flag loading,
interaction authorization, script eligibility and the injected menu reader.
The original click resolver retains its API and shares the same candidate order.

Tagged Go request/success/error contracts generate TypeScript, including the
canonical merchant-item type. The existing shop coordinator now admits opening
as well as buy/sell commands. It correlates the menu to its selected character
and scene, validates it against the still-current bag/profile, and cancels on
character change, explicit shop close or scene retirement. Failures preserve the
bag/wallet and report an error without retrying. Unknown, unsolicited, overtaken
and late menus cannot open a shop globally. The map-only send helper and global
merchant-open handler are removed; source clerk interaction calls the coordinator.

The rendered shop fixture now faces left from `(2,5)` across the source MART
counter at `(1,5)`, whose `raw_foot_tile_id` is `0x1e`. Its golden output was
regenerated through the isolated simulator. An initial browser run correctly
failed after the intermediate fixture placed the player on that counter; no
reach or presentation assertion was weakened. The final browser path uses the
keyboard's ordinary clerk interaction, rather than a map-only transport bypass.

Verification: isolated PostgreSQL race suites passed for world (47.279 seconds),
economy (3.990 seconds) and inventory repository (2.120 seconds). Packet checks
cover correct runtime actor identity, remote/out-of-reach/hidden actors, wrong
characters and unknown selector fields, fresh flags versus stale cache, eligible
scripts and eligibility-query failures. Sixty-seven frontend tests across five
files passed, including correlation, send failure, timeout/rejection/malformed or
overtaken menus, character/close/scene cancellation and late replies. Type
regeneration and typecheck passed, and the final production build passed in
3.48 seconds with runtime asset validation. Three rendered cases passed in
28.1 seconds at `/var/tmp/capturequest-rendered.LLWzJB`: ordinary keyboard clerk
opening across the source counter, delivered/lost duplicate purchase replies,
and a delayed menu after authenticated scene reentry. This is local observable
transport/presentation evidence; it is not a production deployment check.
After adding a listener that proves the delayed packet actually reaches the new
scene before asserting a closed shop, that case passed again in 6.2 seconds at
`/var/tmp/capturequest-rendered.OidhAv`. Both exact private runtimes/PostgreSQL
clusters stopped, as did the final simulator cluster at
`/var/tmp/capturequest-script-sim.2yJUgA`. `git diff --check` passed.

Remaining: shop opening alone does not authorize a later purchase/sale. Audit
those mutation boundaries and source sale policy; actual shop process death,
rendered sale, party/field command migration and all other five-area work remain.
Frontend/backend contracts must activate together in a separately authorized
release. No push or deployment is authorized or performed here.

## Injected merchant reads and owned-map selection (2026-10-03)

Merchant opening now calls `economy.Service.Open` using the authenticated
character, server-owned map and injected database. It no longer reads the global
database or ignores merchant-item/wallet errors. Strict decoding rejects unknown
fields; invalid selectors and remote maps receive an explicit failure. A selected
merchant must belong to the owned map. The shared repository uses resolved
`cq_merchants.map_id`, matching purchase authorization, rather than granting
access through a similar display name.

Provenance: `server/cmd/import-phaser/runtime_seed.go` defines fourteen merchant
seeds and resolves their map names to IDs during import. The local extractor
SQLite's `objects.sprite_name`, `text`, `map_id`, `x` and `y` show eighteen
`SPRITE_CLERK` actors, including two each on Celadon Mart 2F and 5F. Not every
clerk sprite is a shopkeeper: that corpus also includes Game Corner clerks and
the mansion graphic artist. No source or generated assets were changed. The
reader preserves combined same-map department-store offers and merchant-specific
selection; clerk/script eligibility cannot be inferred from sprite alone.

One bounded transaction takes a `SELECT ... FOR UPDATE` character lock, loads
the selected menu and wallet, and returns only after commit. It never writes the
character or creates a wallet. The existing absent-wallet policy yields zero;
query errors and invalid balances fail the complete read. Empty offers encode
as `[]`. Failure returns an empty result instead of publishing a partial menu.

Focused isolated PostgreSQL race checks passed for merchant reads and shop
transactions: economy (2.186 seconds) and world (1.180 seconds). Packet tests
keep the global database nil and prove injected success, rejected remote map/ID,
invalid/unknown selectors and wallet failure. Domain tests cover combined offers,
specific merchants, stale display names, missing characters, offer/wallet failures,
deadline cancellation while waiting for the character lock, explicit empty offers
and zero absent wallets, and a trigger that rejects any character write.
The complete economy and inventory repository race suites also passed
(2.568 and 1.543 seconds respectively), and `git diff --check` passed.
These checks prove database/transport behavior, not rendered clerk interactions.

Remaining: same-map selectors still bypass clerk reach and active script
eligibility. The merchant-open wire remains uncorrelated and its global client
handler can reopen a retired shop. Migrate this boundary into the existing
scene-owned coordinator with typed replies, then verify actual clerk interaction,
cancellation and late responses. Sale policy, rendered sale/process-crash checks,
party/field command recovery and the other five-area gaps remain open. No push
or deployment was performed.

## Durable shop revisions and correlated recovery (2026-10-03)

Buy/sell requests now require `requestId` and `shop: {characterId, revision}`.
The tagged Go request, success and error models drive generated TypeScript.
Missing revision is distinct from the valid initial zero; wrong character,
unknown request fields and missing/oversized correlation are rejected. The
transport uses the authenticated selected character and server-owned map for
purchase validation.

`character_shop_state` retains one revision per character. Inventory and coherent
gameplay snapshots include `shopRevision`, with zero for a character that has not
committed a shop mutation. Under the existing character lock, buy/sell compare
and advance the revision in the same transaction as payment, whole-stack sale,
stock, grant and complete inventory snapshot. A failed effect, final read or
commit rolls back the revision. A committed revision stays stale across service
replacement, reconnect or process restart, and cannot be reused for either a
buy or sale. This is a rejection guard rather than a historical receipt replay;
clients read current authority after ambiguous outcomes.

Startup validates the shop-state schema before gameplay preload/readiness. The
schema path selects the canonical full-data deployment lane automatically; do
not deploy this as frontend/backend fast-lane code. Schema, frontend and backend
must move together. No deployment is authorized or performed here.

The client shop coordinator owns one pending command per scene. It uses the
shared correlated request helper, carries the current selected-character/revision,
validates the next committed revision, and applies the complete bag and both money
views. Buy stays pending until acknowledgement or recovery, replacing the 300 ms
unlock timer. Shop close, character change and scene replacement cancel listeners;
scene retirement also closes its shop presentation, and a new scene starts closed;
late replies cannot apply globally or retire a newer scene's admission. A timeout,
rejection, failed send, malformed success or overtaken result triggers a current
read without resending the mutation. Failed recovery preserves existing views and
reports a reconnect error. Success replies are the only shop mutation publication;
the obsolete independent shop inventory notification is retired. Standalone bag
reads remain for other consumers and reject older shop revisions.

Unit fixtures now carry the required recovery revision. Existing global shop
reply application assertions were replaced because these packets intentionally
require a live correlated caller; unsolicited buy/sell replies are ignored.
The new coordinator tests cover their success/application behavior and retirement.
The browser test sends each real purchase packet twice with the same identity and
covers delivered and lost acknowledgement. A lost acknowledgement also drops the
duplicate rejection so recovery must exercise the timeout. It verifies the one
committed 95-to-99/6 split, money 10,000-to-8,000, revision 1, disabled pending Buy,
late delivery and authenticated reentry without a second client mutation.

Verification: final isolated PostgreSQL race suites passed for `world`
(39.153 seconds), `economy` (2.044 seconds) and `cqitems` (1.556 seconds).
They cover four concurrent identical revision-zero buys with one commit,
stale buy/sale rejection after creating a new service, valid cross-family revision
advancement, rollback of revision/effects on deferred commit failure, required
network identity/correlation, and startup rejection of a missing shop-state table.
The focused frontend suite passed 58 tests across five files, including obsolete
scene cleanup versus a new command and lower-revision standalone packets.
Type generation, typecheck, runtime asset validation, production build and ten
deployment-classification tests passed; the schema selects `full` mode.

Four rendered cases passed in 57.1 seconds at
`/var/tmp/capturequest-rendered.YVLIlr`: delivered/lost shop replies with duplicate
requests and party/PC capture recovery. That private runtime and PostgreSQL were
stopped. After tightening scene cleanup to close the old shop presentation, both
shop cases passed again on final code in 23.2 seconds at
`/var/tmp/capturequest-rendered.DvVJ5K`; that exact private runtime/PostgreSQL also
stopped. The final production build passed in 3.57 seconds. These are local
transport/rendered checks, not live deployment evidence.
The shop service replacement test is not an actual server crash; shop process-death
and rendered sale acceptance remain separate coverage.

Remaining: merchant open still uses legacy/global reads and ignores some lookup
errors; clerk reach/eligibility and sale policy need their source audit. Party/field
item use still needs typed command outcomes and durable identity/recovery. Shop
revision advances only for shop mutations, so it does not fence every unrelated
wallet/item notification. Those families and cross-character standalone stream
retirement remain in the full goal, together with remaining callbacks, wire/domain
migration, plan ordering and lifecycle acceptance. Recommended next step: finish
merchant-open authority/error handling and acceptance, then migrate party/field item
commands through the same ownership and timeout boundaries. Local checkpoint
only; no push or deployment.

## Explicit simulator database boundary (2026-10-03)

The previous checkpoint exposed `scriptsim.InitDB` inheriting the application's
configured database and syncing scripts before scenario execution. Bare
`script-sim --check` was not read-only: it also reset and seeded a named fixture.
Initialization now requires `CAPTUREQUEST_TEST_DATABASE_URL`, the same explicit
disposable-database variable used by Go integration tests. It does not consult
`DATABASE_URL`, server defaults or local config. Missing or malformed targets
fail before a connection, script sync or fixture mutation; malformed-target
errors do not echo credentials.

`scripts/testing/run-isolated-script-sim.sh` is the ordinary entry point. It
validates matched runtime artifacts, creates a private Unix-socket PostgreSQL
cluster under `/var/tmp`, bootstraps through the canonical schema/import/script
pipeline, then forwards simulator arguments. Logs are retained and its exact
cluster is stopped on success or failure. It does not launch a game server or
browser. `--update` still intentionally writes a selected tracked golden; the
wrapper isolates database changes, not requested file output. The conversion
guide now documents this boundary and uses the wrapper in its CLI examples.

Verification: the focused simulator race tests passed, including absent/blank
explicit targets with an application `DATABASE_URL` present, and malformed
credential-bearing input. A bare CLI run exited before connection with the new
required-target error. The isolated runner passed
`debug_shop_inventory_publication --check` at
`/var/tmp/capturequest-script-sim.MHnQQS`. A second run intentionally requested a
missing fixture at `/var/tmp/capturequest-script-sim.AdtggK`; it returned failure
and its cleanup stopped PostgreSQL. Both exact data directories have no live
`postmaster.pid`. Shell syntax and `git diff --check` passed. This verifies the
new target boundary and both cleanup outcomes; no frontend/build/deployment
behavior changed.

The explicit test variable is authorization to use the named test target, not
proof that an arbitrary database is disposable. Ordinary checks should use the
wrapper. Fixture replacement is still nontransactional, and schema/data validation
before simulator sync remains a separate audit for custom harnesses. This
checkpoint closes implicit application-target selection; it does not close the
full server goal. Recommended next step: continue durable shop mutation identity,
correlated acknowledgement, timeout recovery and request duplicate acceptance.
Local checkpoint only; no push or deployment.

## Committed shop inventory snapshots and endpoint audit (2026-10-03)

The endpoint audit found that `HandleCQInventoryRequest` and
`sendCQInventorySnapshot` read inventory and money separately and discarded the
wallet error (`money, _`). A query failure could therefore publish a plausible
zero balance with a real bag. `CQInventoryStore.updateAfterBuy` also incremented
only the first affected instance or invented one new stack, although
`AddItemToInventory` can split a grant across multiple stacks. A separate
inventory notification was required to correct that intermediate view.

`cqitems.Store.GetCharacterSnapshot` is now the shared reader for standalone
inventory, coherent gameplay recovery and shop mutations. It owns or joins a
bounded transaction and takes the character lock before reading bag and wallet.
Missing wallet rows retain the canonical zero-balance policy; query errors and
out-of-range balances fail the entire read. Empty bags are explicit arrays.
The standalone request uses the injected world database. Legacy callers of
`sendCQInventorySnapshot` still use the existing global database and remain in
the dependency migration audit.

Buy and sell now capture the whole bag/balance inside the mutation transaction.
A failed final snapshot or commit returns no result and rolls back payment,
stock and inventory changes. Tagged successes drive generated TypeScript
contracts. Each reply contains `inventory: {items, money}`; the compatibility
inventory publication uses that same committed value without another query.
The browser replaces the full bag and both money views, so duplicate delivery
cannot increment a stack. The incremental shop store methods are retired.
Malformed success payloads do not synthesize an empty bag or zero money.
This changes the shop reply contract and requires the frontend and backend to
move together; no mixed-version deployment is claimed.

Final isolated PostgreSQL race suites passed for `world` (41.415 seconds),
`cqitems` (1.555 seconds) and `economy` (1.773 seconds), including purchase/sale
rollback when the final bag read encounters invalid persisted charges. An initial
broad run caught the shared reader using the mutation lock's no-op `UPDATE`;
that violated recovery's existing no-write trigger assertions. The reader now
uses `SELECT ... FOR UPDATE`, and direct snapshot plus existing recovery tests
prove that read-only publication does not fire character update triggers.

Verification: five focused frontend files passed 47 tests, including the real
network bridge's split-stack application, duplicate delivery, empty bag and
malformed top-level array/balance cases. Type generation, typecheck, runtime asset
validation and the production build passed. The rendered shop test passed in
5.9 seconds at `/var/tmp/capturequest-rendered.oSFzSP`: the source `POKE_BALL`
item is ID 4 with price 200; buying ten changed a 95 stack into 99 and 6 and
money from 10,000 to 8,000. The independent inventory notification was dropped;
the committed reply restored the bag, duplicate delivery did not increment it,
and authenticated reentry preserved it. The same browser case passed again on
the final SELECT-lock implementation (6.1 seconds) at
`/var/tmp/capturequest-rendered.61fkiE`; its private runtime and PostgreSQL were
stopped. Shop opening uses the real transport;
purchase and money display use the rendered UI. The fixture golden was generated
and checked with the canonical simulator against that private PostgreSQL database.

The initial combined browser run at `/var/tmp/capturequest-rendered.i5FHpi`
failed: the first shop fixture lost its funding because each scenario jump resets
wallet state, fixed by one funded shop fixture. The existing Bicycle test proved
three toggles, then failed because the bag list (`z-index: 2500`, `top: 570px`)
intercepts the Done button (`z-index: 1000`) at the default 720-pixel viewport.
Those layout sources are unchanged; indoor/reentry Bicycle acceptance did not
complete and is not claimed here. Both private browser runtimes and PostgreSQL
instances were stopped.

A simulator attempt without an explicit database target used the configured local
`127.0.0.1:5432/capturequest` database, synced 375 scripts (63 changed), and failed
while resetting its named fixture because `character_repels` was absent. That
attempt is not verification. `scriptsim.InitDB` still inherits application
configuration and syncs before validating the fixture schema; it needs an explicit
test-target guard. Subsequent golden generation/check used a command-scoped
`DATABASE_URL` for the private database only. Do not run the bare simulator
command as an isolated check.

### Remaining endpoint work

| Boundary | Current evidence | Still required |
| --- | --- | --- |
| Inventory request (92/93) | Bounded locked read, explicit typed success, whole-read errors, empty arrays. | Correlation, character/session retirement and revision fencing for delayed standalone replies. |
| Merchant open (94/95) | Existing merchant/map lookup; unchanged by this checkpoint. | Injected cancellable reads, propagation of item/wallet lookup failures, owned map and clerk eligibility audit, correlated presentation. |
| Merchant buy (96/97) | Atomic offer/map/stock/payment/grant and complete committed bag publication. | Durable command identity, duplicate request protection, correlated acknowledgement, timeout recovery and retirement. `PokeMartShop` still releases `buying` after 300 ms rather than awaiting acknowledgement. |
| Merchant sell (98/99) | Atomic ownership/removal/whole-stack credit and complete committed bag publication. | The same durable identity/correlation/recovery audit and merchant eligibility policy. |
| Field item use (100/101) | Party item transaction and existing field-item dispatch; item consumption still projects `newQty`. | Full action-specific audit, typed outcomes, durable duplicate protection and scene-owned timeout recovery. Rods delegate to fishing; Bicycle, Repel and Escape Rope have distinct existing paths that must be audited together. |

Duplicate **delivery** is covered here; a repeated purchase **request** can still
buy twice. Snapshots do not impose a revision fence on older standalone packets
arriving after a newer operation. These gaps remain explicit parts of the full
five-area goal. Recommended next step: migrate shop mutations to durable command
identity and correlated, cancellable acknowledgement with current-state recovery,
then extend that boundary to party/field item use. Local checkpoint only; no push
or deployment.

## Coherent owned inventory, wallet, party and flags (2026-10-03)

`GameplayStateResponse` now includes required `inventory`, `wallet`, `party` and
`eventFlags` fields alongside owned position, battle/Safari and pending plans.
The existing character-row lock covers every read through the same bounded
transaction. Inventory reuses the transaction-bound CQ repository and wallet
uses its shared currency reader; the tagged `CharacterWallet` model and CQ item
models also drive the generated TypeScript contract. Full party state is returned
even without a battle, and empty inventory/party/flags are explicit arrays.
Flags are sorted for deterministic current-state output. A failed read or invalid
party/flag data publishes only the correlated error and leaves battle ownership
unchanged. No separate state notification is required to apply recovery.

The first rendered run exposed an incorrect new assumption: zero-balance
characters can legitimately have no `character_wallet` row. The existing
`GetCharacterMoney` repository defines that absence as zero, including new
characters. Recovery now uses that canonical policy and returns a typed wallet
with the owned character ID. This is not a fallback after a failed query: actual
wallet read errors still fail the whole response. Tests distinguish the valid
absent-row zero from a missing table/query failure; recovery also validates the
balance range.
No creation backfill or production data mutation is introduced.

The scene-owned service validates the complete snapshot before updating existing
inventory, player-profile and party stores, then restores battle/pending-plan
presentation. Both money views use the same wallet value; the existing character
profile carries recovered flags, without adding a parallel gameplay rules engine.
Inventory, wallet/profile, party and battle views are checked for changes while
replies are delayed. A newer notification causes one fresh read; recovery never
resends a mutation. Cancellation, mismatch, timeout and incompatible success
payloads leave existing state intact. Correlation and scene ownership remain the
application boundary; unsolicited snapshots are not applied globally.

The capture acceptance cases now also drop independent CQ inventory, wallet and
party notifications after the action. The recovered summary must restore caught
placement/party and remove the spent Master Ball before reentry. This exercises
the actual transport and current-state response rather than allowing a separate
notification to mask missing inventory recovery.

Verification: the final isolated PostgreSQL race suites passed for `world`
(35.493 seconds) and `cqitems` (1.253 seconds). The three focused frontend files
passed all 38 tests. All 12 rendered battle/gameplay recovery cases passed in
4.4 minutes, including the capture cases with independent state notifications
dropped. Type generation, typecheck, runtime asset validation, production build
and `git diff --check` passed. Rendered evidence is retained at
`/var/tmp/capturequest-rendered.Qj4F8A`; the runner stopped its private runtime and
PostgreSQL instance. These are isolated local checks, not production validation.

This checkpoint provides coherent current-state recovery during map load and the
migrated battle command recovery path. Legacy inventory/shop/field commands still
need their endpoint/timeout/duplicate audit and recovery integration; standalone
inventory messages and other wire families retain their existing migration work.
The read guard prevents an overtaken snapshot from applying; it does not add a
revision fence to arbitrary standalone legacy notifications arriving afterward.
Server flag-cache writers, other account state such as coins, and remaining plan
presentation/queue/source/catalog recovery are separate audits. The full five-area
goal remains active. Recommended next step: audit inventory/shop/item-use commands
against the shared ownership, transaction and correlated-recovery boundary, then
continue remaining domain-contract, callback and shutdown work. Local checkpoint
only; no push or deployment.

## Ordinary walking issuance and receipt crashes (2026-10-03)

`movement-process-recovery.spec.ts` uses the existing three-step Safari fixture
and the real walking protocol. The live imported catalog must confirm that the
source/target floor tiles `(220,14,24)` and `(220,14,23)` have no encounter area;
this avoids random encounters without changing random mechanics or masking the
Safari step effect. It first withholds the actual issued step reply before any
completion, verifies that position/counters/rows are unchanged, then crashes the
server. Fresh readiness and authenticated entry preserve the source and reject
that old session's uncompleted token without writing a receipt or changing state.

A new explicit walk commits position, the Safari counter and the latest movement
receipt, but its reply is withheld for the second crash. Fresh readiness/entry
preserve the full saved state: source `(14,24)` becomes `(14,23)` and steps change
from 3 to 2 once. Explicit retry acknowledges the durable receipt without effects.
A normal committed test warp then changes current position to `(14,24)` while
retaining that receipt and counter. Retrying the old completion still reports its
historical `(14,23)` result; an owned-position read returns current `(14,24)` and
historical `committedStep` separately. Neither command rewinds the browser or DB.
A final new walk decrements 2 to 1 and replaces the bounded latest receipt; the
superseded token is rejected without effects. Pokémon rows remain unchanged.

The shell-owned crash/SQL/evidence helper is mechanically extracted to
`tests/e2e/helpers/processRecovery.ts` and shared by both process-recovery specs.
The default crash lane includes both files. Acceptance covers an uncompleted
issued intent and a fully committed step, not a crash inside an open transaction
or automatic reconnect of the original live page. Remaining forced movement,
trainer/script-plan and queue/source/catalog scenarios need separate evidence.
No production runtime, generated contract or asset implementation changed.

All seven default-lane crash cases passed (1.8 minutes), including the movement
case in 17.3 seconds, at `/var/tmp/capturequest-rendered.49R2BB`. Its two receipts record
`2849049 -> 2849474 -> 2849655`, each predecessor reaped with exit 137.
Character 1 retains Pokémon row `[7]`. The evidence JSON includes raw state,
intents, completions, replies and owned reads. The recovered movement screenshot
was inspected and the private runtime is stopped. Typecheck, shell syntax and
`git diff --check` pass. Logs are
`/var/tmp/capturequest-movement-crash-{types,rendered}.log`.

The full five-area goal remains active. Recommended next step: complete coherent
inventory/wallet/flag recovery, then continue the remaining endpoint, callback,
dependency/contract, plan and shutdown audits. Local checkpoint only; no push or
deployment.

## Issued cutscene crashes and creation-only local fixtures (2026-10-03)

A new two-crash test uses the existing `oak_lab_choose_starter_intro` scenario.
It withholds the actual issued notification, compares the saved plan/script,
source position, flags, visibility and Pokémon rows across SIGKILL and fresh
readiness, then requires fresh authenticated entry to deliver the same token and
actions. Real browser dialogue and player animation complete the script. The
committed completion reply is withheld for a second crash; the resolved plan,
three completion flags, visibility and final position survive fresh readiness
and authenticated entry without issuing the script again. After one ordinary
step, explicitly replaying the completed token returns historical completion
with the current position and changes none of those database records.

The first run exposed a real local/test reconnect mutation: this scenario's
intentional empty party (`pokemon: null`) became six rows of species
`[4,25,7,1,16,39]` on fresh entry. `ensureLocalDevFixtures` called empty-party
seeding on every login. Empty gameplay state is not evidence of missing setup.
Party setup now joins inventory setup in the newly created character's existing
creation transaction; login no longer runs either fixture. The obsolete entry
wrapper and its global database dependency are removed. The same setup requires
a transaction, preserves existing battle row IDs and damage, and rolls party and
inventory back together if the enclosing creation fails. This is a local/test
fixture fix; production's existing config gate remains in place. The legacy
character creation/cache boundary remains separately tracked.

The full world PostgreSQL race suite passed (56.426 seconds), including the
creation transaction and identity tests. All six rendered process-recovery cases
passed (1.6 minutes) at `/var/tmp/capturequest-rendered.F3KCm1`; the cutscene case
took 20.4 seconds. Its two receipts record
`2837978 -> 2838494 -> 2838835`, with exit 137 for each predecessor. Character 6
retains token `7ca322e3-9450-4041-a769-9d1da2c68abf`, source `(40,5,11)`, completed
position `(40,5,3)` and later owned position `(40,5,4)`. The party stays empty
through every boundary. Resolution intentionally refreshes receipt sequence;
replaying the resolved token leaves it unchanged. The settled screenshot was
inspected. Typecheck and `git diff --check` pass. Logs:
`/var/tmp/capturequest-plan-crash-{world,types,rendered-fixed}.log`. The initial
failing evidence is retained at `/var/tmp/capturequest-rendered.kSns3Q` and
`/var/tmp/capturequest-plan-crash-rendered.log`. Both private runtimes are stopped.
The isolated runner built and exercised the changed server binary; no frontend
asset or production build changes were needed.

This covers one issued cutscene before delivery and after committed completion,
not crashes during presentation, queued/source/catalog changes, other plan
families or automatic live-page reconnect. The full five-area goal remains active.
Recommended next step: verify issued ordinary steps and their durable receipts
through process death, then continue coherent inventory/wallet/flag recovery and
the remaining endpoint, callback, dependency/contract and shutdown audits. Local
checkpoint only; no push or deployment.

## Pending move-choice process-death acceptance (2026-10-03)

Two real transport/rendered cases extend the private exact-process crash lane to
learning or skipping LEECH_SEED. The existing fixture supplies a level-6 Bulbasaur
with four configured moves and an original level-2 Chansey; the original EXP and
learnset produce 72 earned EXP and the level-7 choice. Fixture moves are test
preparation, not a claim about the natural level-6 moveset.

The tests withhold the actual committed turn response containing `pendingMove`,
inspect `character_battle_state.battle_json` and full `character_pokemon` rows,
then crash the owned server. Fresh readiness and fresh authenticated character
entry must preserve those exact records and show the unfinished choice without
resending the turn. Each test sends one explicit learn/skip command, withholds its
committed reply and crashes again. The second restart preserves the settled
revision, row identities, EXP and moves. Fresh entry uses the existing normal
terminal dismissal path exactly once, clears the saved battle and retains all
settled Pokémon rows. A further character reentry cannot replay the choice or EXP.
The crash/SQL/evidence helper is shared with the existing Safari cases in the same
spec; the production implementation did not need a change.

All five process-recovery cases passed (1.2 minutes), including both new two-crash
cases (18.2/18.3 seconds), at `/var/tmp/capturequest-rendered.yRGAcx`. Learning used
`2815403 -> 2815773 -> 2815963`; skipping used
`2815963 -> 2816395 -> 2816559`. Every killed predecessor was reaped with exit 137.
Stable Pokémon rows `[47]` and `[54]` each retain EXP 251, up from 179. Settled move
IDs are `[75,73,45,22]` for learn and `[75,33,45,22]` for skip. The evidence JSON
stores compared records and receipts. All four pending/settled screenshots were
inspected. Typecheck and `git diff --check` pass. Logs are
`/var/tmp/capturequest-move-crash-{rendered,types}.log`; the private runtime is
stopped. A production build and broad Go suites were not repeated for this
acceptance-only change.

This proves recovery after committed boundaries through fresh authenticated
entry. Automatic live-page reconnect, pre-commit crashes, issued movement/script
plans, other unfinished choices and recovery of already-dismissed notices remain
separate work. The full five-area goal remains active. Recommended next step:
extend the crash lane to issued plans, then continue inventory/wallet/flag
resynchronization and the remaining endpoint, callback, dependency/contract and
shutdown audits. Local checkpoint only; no push or deployment.

## Terminal Safari process-death recovery acceptance (2026-10-03)

The isolated runner now has a dedicated `CQ_E2E_CRASH_RECOVERY=true` lane. It owns
and records every server PID, verifies that a test's crash request names its
current child, sends SIGKILL, reaps exit status 137 and clears that ownership slot
before starting the same private binary against the unchanged private database.
Readiness must succeed before publishing the new-PID receipt. Atomic request and
receipt files prevent partial/stale observations; no process-name scan, broad kill
or production service is involved. Shutdown and crash modes are mutually exclusive.

Three real transport/rendered cases withhold the committed terminal Run, party
catch or full-party PC catch response. They read the actual Safari record, position,
complete Pokémon rows and Pokédex records before crashing the server. Those records
must match after fresh-server readiness and again after fresh authenticated entry
from a new page with the same guest account/character. The recovered summary retains
its encounter identity/revision/counters; no gameplay command is resent. Explicit
close then clears the encounter while preserving exactly the same Pokémon row IDs,
row contents and Pokédex state. The real Safari capture helper is mechanically
shared with the existing reply-loss tests; random mechanics remain unchanged.

All three crash cases passed at `/var/tmp/capturequest-rendered.yxb7xQ` (40.5 seconds).
Receipts record `2786450 -> 2786899 -> 2787292 -> 2787823`, with exit 137 on each
predecessor. Watched Pokémon IDs remain `[7]`, `[15,16]` and `[35,36,37,38,39,40,41]`
for Run, party and PC respectively. The evidence JSON includes the compared records
and response identities. Recovered party/PC screenshots were inspected. Both
existing rendered capture tests passed after helper extraction at
`/var/tmp/capturequest-rendered.LwlSXf` (45.6 seconds). Typecheck, shell syntax and
`git diff --check` passed. Logs are `/var/tmp/capturequest-process-recovery-{types,
rendered,helper-rendered}.log`. Both orderly-shutdown modes also pass with the refactored startup: success at
`/var/tmp/capturequest-rendered.FR8zfh` returns server exit 0; injected final-save
failure at `/var/tmp/capturequest-rendered.UZenYR` returns exit 1 and preserves the
expected failure outcome. Their logs are
`/var/tmp/capturequest-process-recovery-shutdown-{success,failure}.log`. All private
runtimes are stopped. No production runtime or asset code changed; no build/broad Go
suite was repeated for this test-harness change.

Coverage is committed terminal Safari recovery with fresh authenticated reentry.
Automatic live-page reconnection, crashes before commit, pending learning choices,
issued movement/script plans and durable notice recovery after already-committed
dismissal remain acceptance work. The full five-area goal remains active.
Recommended next step: extend this exact-process lane to pending choices and
issued plans, then continue inventory/wallet/flag resynchronization and the
remaining endpoint, callback, dependency/contract and shutdown audits. Local
checkpoint only; no push or deployment.

## Rendered Safari party/PC capture recovery (2026-10-03)

Two data-only fixtures use the original Magikarp at level 5 in Safari Center with
30 balls and either one or six existing party members. Capture and flee rolls are
unchanged. The rendered test sends explicit ball actions; after a real flee it can
prepare a new independent encounter. It withholds only the actual caught response,
then requires timeout recovery to restore that identity/revision, remaining balls,
authoritative party and factual party/PC placement without resending the command.

Both cases retain the caught summary across character reentry, require explicit
correlated dismissal, release the late reply behind a read-only delivery barrier
and preserve the settled party through another reentry. The full-party case then
uses the normal committed test warp to the existing Viridian Pokémon Center PC;
it preserves earned rows, ends the visit and checks exactly one Magikarp in Bill's
PC. A fixture reset at that point would erase the earned state and invalidate the
check, so it is not used. Rendered/screenshots prove the actual summaries, party
sidebar and one caught PC entry. Stable underlying row identities remain proved
by the previously documented PostgreSQL tests; these browser DTOs do not expose
those identities.

The simulator's early `fixture_state` path also skipped Safari setup. It now uses
the existing Safari fixture seeding/read/summary functions, so the same data-only
fixtures have meaningful static preparation goldens. No capture roll is simulated
by those goldens; the real rendered actions supply that acceptance.

Verification: both rendered cases passed (40.2 seconds) at
`/var/tmp/capturequest-rendered.OMtCh9`, and all three captured screenshots were
inspected. Typecheck and the simulator race suite passed. Both new goldens were
reviewed, canonically generated and rechecked, alongside all 11 prior deterministic
Safari goldens on a matched, private PostgreSQL runtime at
`/var/tmp/capturequest-safari-sim.2M0yZa`. Both private runtimes are stopped. Logs:
`/var/tmp/capturequest-safari-capture-{rendered,types,sim-unit}.log`.
`git diff --check` passed. Production/frontend runtime code and asset generation
did not change; a production build was not repeated for this test/fixture change.

The goal remains active across all five areas. Recommended next step: verify
committed encounter/capture state after actual server-process death and a fresh
process, then continue inventory/wallet/flag resynchronization and the remaining
endpoint, callback, domain-contract/dependency and shutdown audits. No push or
deployment is included.

## Shared Safari simulator command contract (2026-10-03)

The simulator's `runSafariBattleAction` still sent only `action` after the runtime
made encounter identity and request correlation mandatory. It now builds the
shared Go request from the actual saved encounter and reads the shared response,
retiring its duplicate partial response parser. Success requires matching request
ID, encounter ID, next revision and position correlation; the response destination
must also match the committed character snapshot.

Fresh simulator fixtures use a stable `scriptsim:<scenario name>` identity for
repeatable golden output. That value is persisted before the real handler reads
it; production/browser-debugger identity generation is unchanged. Safari summary
assertions now distinguish playable encounters from retained terminal records.
Both action fixtures require the expected species/level even when `active:false`,
so disappearance cannot masquerade as terminal retention. Formatting also shows
a retained encounter on an inactive visit instead of hiding it.

Verification: the script-simulator race suite passes, all 11 existing deterministic
Safari goldens pass on a canonically bootstrapped private PostgreSQL cluster, and
Run passes again with identical golden output on the next fresh fixture. Only the
Run golden needed regeneration: its reviewed differences are correlated typed
response/position and retained terminal summary. Twelve independent last-ball
runtime-expectation runs cover one catch, one flee and ten non-flee exhaustion
outcomes. After adding response/durable-position agreement checks, all 11 goldens
and another last-ball run pass against the final executable. Evidence is retained
at `/var/tmp/capturequest-safari-sim.wZ02cK`; unit output is
`/var/tmp/capturequest-safari-simulator-unit-verified.log`. The private cluster is
stopped and no application/production database was used.

The random last-ball case is checked through invariant expectations, without a
fixed text golden, changed random mechanics or suppressed outcome fields. The
CLI guide documents that distinction; this checkpoint does not claim blanket
`--all --check` acceptance. Frontend/runtime code did not change, so rendered
checks from the preceding checkpoint were not rerun. `git diff --check` passes.

The five-area goal remains active. Next: rendered Safari party/PC capture and
process-death recovery acceptance, then inventory/wallet/flag resynchronization and
the remaining endpoint, callback, contract/dependency and shutdown audits. Local
checkpoint only; no push or deployment.

## Safari last-ball expiry presentation recovery (2026-10-03)

A rendered lost-last-ball/close test first proved that the last ball committed the
gate position and retained the terminal encounter across reentry, then failed at
the PA announcement. The close response carried `exitMessage`, but timeout recovery
had only encounter absence and therefore silently lost that presentation.
The initial failing evidence is `/var/tmp/capturequest-rendered.YRZfHB`.

The read-only gameplay snapshot now includes the shared server expiry message for
an inactive retained encounter. The battle store retains it with terminal state.
After the user's explicit close, confirmed encounter absence and completed owned
position projection permit presentation of that captured server message. Failed
close recovery retains the encounter and reports a reconnect error; retired scene
ownership suppresses the old announcement. Late replies cannot replay it. No
additional warp, mutation retry, guessed text or persistent parallel state is added.

The data-only `safari_last_ball_recovery` fixture seeds one ordinary Safari Ball;
it uses the real random capture/flee mechanics. Rendered expectations follow the
actual committed catch outcome and verify its summary at the gate after timeout
and reentry, followed by lost close acknowledgement, expiry presentation and late
reply retirement. This does not guarantee a capture in every rendered run.
PostgreSQL recovery tests separately reach actual party and full-party PC catches
with independent real rolls, then check inactive terminal identity/revision,
authoritative party, BOX 4 placement, expiry message and absence after dismissal.

Verification: world and battle PostgreSQL race suites passed, three focused
frontend suites passed (34 tests), typecheck and production build/runtime asset
validation passed, and all seven isolated rendered Safari/gameplay recovery cases
passed at `/var/tmp/capturequest-rendered.KoREbh`. Logs are
`/var/tmp/capturequest-safari-expiry-{go,front-verified,types-verified,build,rendered-final}.log`.
The first unit run used a matcher unavailable in this Vitest version; separate
count and argument assertions preserve the same requirement. The change
is additive runtime JSON and regenerated TypeScript; no SQL migration or extractor
artifact changes are involved. The full goal remains active. Remaining: rendered
Safari party/PC capture acceptance, process-death recovery and the five-area audits.
Expiry presentation is not a durable acknowledged notice after process death once
the encounter has been dismissed. A follow-up inspection also found that
`internal/scriptsim/runner.go:runSafariBattleAction` still marshals only `action`.
That consumer must migrate to the required identity/request contract, and its
Safari goldens must be regenerated/checked against the matched private runtime;
they were not run in this checkpoint. The new fixture's rendered debugger path
is verified, but its simulator trigger is not yet accepted. Recommended next
step: migrate that simulator consumer, then finish Safari capture/process checks
and continue inventory/wallet/flag resynchronization.

## Guarded Safari commands and terminal recovery (2026-10-03)

Safari actions previously carried only `action`, and replies globally mutated the
browser store. A delayed duplicate could therefore act on the current encounter;
terminal state was deleted before a lost reply could recover a catch or run result.
The existing Safari transaction now checks `battle: { battleId, revision }` under
the character lock and advances the revision once. Generated action requests and
replies also carry a bounded `requestId`. Replies include committed owned position.

The browser uses the existing `BattleCommandService` single-flight slot, scene
retirement guards and current-state timeout recovery for Safari too. It sends over
the reliable stream, ignores unrelated/late replies and never automatically resends
a failed action. Terminal encounters persist until identity-bound `close`; a live
encounter cannot be dismissed and an old close cannot erase a replacement. Catch
placement and the authoritative party are recovered without replaying turn events.
An expired visit retains its terminal encounter at the committed gate destination;
new payment cannot overwrite that unresolved result.

Safari save version 2 requires encounter identity. Supported version-1 encounters
receive an identity under the character transaction before login/read notification
can advertise it. A failed migration returns no playable snapshot. Unsupported
versions and malformed current records fail explicitly. This changes runtime JSON
and the generated browser contract, with no SQL migration or generated asset change;
server and browser must be released together.

Verification for this checkpoint:

- PostgreSQL race suites for world, battle, server and session passed. New checks
  prove concurrent identical commands settle once, live-close rejection, stale
  dismissal protection and commit-before-advertisement for version-1 upgrades.
- Seven focused frontend suites passed (86 tests), including Safari single-flight
  admission, lost capture reply recovery and lost close acknowledgement without
  resending. Typecheck passed; generated DTOs came from `npm run tygo`.
- Production build and its runtime asset validation passed (826 tiles, 92 sprites,
  561 compact audio files); Vite reported its existing large-chunk warning.
- The five existing isolated rendered Safari/gameplay recovery cases passed at
  `/var/tmp/capturequest-rendered.6Mtyec`, covering entry, Run, step exhaustion,
  Safari notification loss/reentry and sight-trainer recovery.
- A focused rendered reply-loss case passed at
  `/var/tmp/capturequest-rendered.3fPt7x`: withheld Run response restores the same
  terminal identity/revision, reentry retains it, withheld close acknowledgement
  recovers absence, and late replies cannot revive it. Exactly one Run and one
  close are sent. Two earlier attempts stopped at incorrect test locators (the
  Run test ID, then the terminal text's continuation marker), corrected against
  actual component/trace evidence without runtime changes or weaker assertions.

Check logs are under `/var/tmp/capturequest-safari-{go-final,front-final}.log`,
`/var/tmp/capturequest-safari-checkpoint-{types,build,rendered}.log` and
`/var/tmp/capturequest-safari-loss-rendered-verified.log`. `git diff --check` passed.

Remaining acceptance: dropped last-ball replies across gate/scene replacement,
Safari party/PC capture through the rendered UI, expired-visit close reply loss
(including the PA dialogue), and process-death recovery. Unit/transaction checks
do not establish these rendered outcomes. The current five-area table remains
the scope of the active goal; historical checkpoint notes below are not a second
roadmap. Recommended next step: finish those Safari failure/reentry checks, then
continue authoritative inventory/wallet/flag resynchronization and the endpoint,
callback, domain-contract and shutdown audits. This checkpoint is local only.

## Durable capture placement and terminal login retention (2026-10-03)

Capture previously persisted `PlayerCaught`, the Pokémon row and Pokédex state,
but PC placement lived only in `battleTurnResult.SentToPC/PCBox`. A lost reply
therefore recovered a finished battle without its capture summary. The capture
settlement now saves `CapturePlacement` in the same battle JSON transaction as
ball consumption and party/PC insertion. Private battle clones copy the placement;
recovery rejects contradictory capture metadata and out-of-range PC boxes.
Storage uses zero-based boxes; the generated gameplay DTO presents one-based
boxes, matching the existing ordinary end response. Recovery never guesses PC
placement from party size.

The browser restores the factual caught state and party/PC summary without
replaying turn events, and keeps it visible until explicit dismissal. Ordinary
terminal recovery still auto-dismisses as before. Old saved captures without
placement metadata can show their factual Pokédex summary; their original PC
destination is unavailable and is not invented. This is an optional additive
field within the supported save format, with no schema/import or asset change.

The first rendered checks then found a second producing cause: login still
silently deleted finished battles without pending learning choices. That erased
capture summaries during character reentry and bypassed the atomic post-battle
plan path. Login now retains the terminal record/cache for the scene's coherent
read. Normal correlated dismissal owns deletion and any eligible plan issuance.

The PostgreSQL dispatcher test covers party and full-party captures, including
late battle-save failure after consumption/insertion. Failure rolls back the
ball, caught row and placement without publishing the private battle. Success
survives discarded reply delivery/cache and restores the placement. A non-default
PC preference stores box 3 and exposes BOX 4; the test checks the actual caught
row's box. Duplicate original command identity rejects, row count and existing
Pokémon identities remain unchanged, and exactly one ball remains spent. Storage
tests cover placement round-trip, private clone isolation, invalid boxes and
placement without a catch. Browser store tests cover party/PC restoration without
event replay.

Rendered fixtures use one Master Ball and the original Magikarp at the existing
Viridian Pokémon Center PC location. Both hold the actual capture reply, restore
the summary after timeout and character reentry, require explicit dismissal,
release the late reply behind a read-only delivery barrier, and preserve the
settled party after another reentry with no ball left. The full-party case opens
the real PC UI and checks exactly one caught Magikarp. Initial reentry failures
at `/var/tmp/capturequest-rendered.7iP19u` demonstrated the login deletion. After
that fix, the PC case reached every capture assertion but its cleanup tried Quit
while the PC overlay was open; cleanup now closes that overlay normally.

Verification completed locally:

- PostgreSQL race suites for world, battle, server and session passed:
  `/var/tmp/capturequest-capture-go-final.log`. The stronger stored-row box check
  then passed in both focused capture cases:
  `/var/tmp/capturequest-capture-row-check.log`. Placement serialization/validation
  tests passed at `/var/tmp/capturequest-capture-persistence.log`.
- All 27 focused browser coordinator/recovery-store tests passed:
  `/var/tmp/capturequest-capture-front.log`. Canonical `npm run tygo`, typecheck,
  production build/runtime-asset validation (existing large-chunk warning) and
  `git diff --check` passed. Logs:
  `/var/tmp/capturequest-capture-contract.log`,
  `/var/tmp/capturequest-capture-types-final.log`,
  `/var/tmp/capturequest-capture-build.log`.
- The combined rendered run passed 9 of 10 cases: party capture, learn/skip,
  turn, close, Brock reward, blackout, trainer and Safari snapshot recovery.
  Only PC test cleanup failed after all its capture assertions passed. That
  exact case then passed with ordinary overlay cleanup on the final source.
  All ten cases thus have passing evidence, across the combined run and focused
  rerun; this is not a claim of one green combined run. Evidence:
  `/var/tmp/capturequest-rendered.1Ete6b` and
  `/var/tmp/capturequest-rendered.jd1hg7`; logs:
  `/var/tmp/capturequest-capture-rendered-final.log` and
  `/var/tmp/capturequest-capture-pc-rendered-final.log`.

Remaining: complete Safari encounter identity/correlated mutation recovery and
full inventory/wallet/flag reconciliation; process-death and real connection
replacement coverage; remaining terminal/queued-plan variants; and every open
endpoint/mutation, callback/ownership, domain/wire and lifecycle item in the
five-area table. This does not complete the full goal. Recommended next step:
audit and migrate Safari commands through the existing identity/correlation and
coherent recovery mechanisms. Changes are local only; no push or deployment.

## Rendered level-up choice recovery checkpoint (2026-10-03)

The new data-only `active_battle_fixture_learning_recovery` scenario starts a
level-six Bulbasaur with four explicit fixture moves against level-two Chansey.
Provenance is the matched local `public/phaser/pokemon.db`: `pokemon_learnset`
contains Bulbasaur's level-seven `LEECH_SEED` (move 73), and Chansey's
`base_exp = 255` awards 72 experience through the existing wild-battle formula.
The four fixture moves are deliberate debug inputs, not a claim about the moves
a level-six Bulbasaur naturally knows. No pending-state injection, extractor,
generated game-data or battle-rule edits are used.

Two rendered browser tests use the actual PostgreSQL/WebSocket server. Both
hold the turn response that naturally creates the pending choice. Timeout
recovery displays the exact prompt and does not dismiss it. Quit/character
reentry restores the prompt and full party without another battle action or
close. One test clicks Tackle to replace it; the other clicks Don't Learn. Both
hold the committed learning reply, recover settled moves through current state,
dismiss exactly once, release the late turn/choice packets behind an owned-position
round-trip delivery barrier, and verify the panel stays closed. Another character
reentry preserves the complete settled party. Exact experience remains the
original value plus 72. The command counts are unchanged: one explicit choice,
one close, and no automatic turn/choice resend.

The initial fixture used Dragon Rage against Magikarp. It spent PP without
reaching the level-up prompt: the battle engine lacks that move's fixed-damage
effect. That is an uncovered gameplay-rule gap, not evidence of recovery failure;
this checkpoint does not repair or assert Dragon Rage fidelity. The final
fixture uses the existing ordinary Razor Leaf damage path and a low-level Chansey
to avoid dependence on that unsupported effect and reduce fainting risk. Explicit
turn capacity allows accuracy misses; terminal/prompt/settlement assertions stay
required. Intermediate failures also exposed test locators that omitted the
source's underscore (`LEECH_SEED`) and the buttons' accessible cursor prefix
(`> TACKLE PP ...`). Locators were corrected against screenshots/DOM and source,
without modifying product behavior or weakening settlement assertions.

Verification: both final Chromium cases passed in the isolated environment at
`/var/tmp/capturequest-rendered.CYbX0V`. Log:
`/var/tmp/capturequest-learning-rendered-final.log`. Typecheck and
`git diff --check` passed (`/var/tmp/capturequest-learning-rendered-types-final.log`).
Matched asset validation and PostgreSQL bootstrap checks passed in the runner.
Failed evidence is retained at `/var/tmp/capturequest-rendered.VJY1Z8`,
`/var/tmp/capturequest-rendered.LoUzZ8` and
`/var/tmp/capturequest-rendered.N2yhDL`. These test/fixture/documentation-only
changes do not require a repeated production build or broad Go suite.

This proves natural prompt generation, rendered learn/skip controls, character
reentry and reply-loss settlement through the real server. It does not prove
replacement of the transport connection, process-death recovery, multiple queued
level-up prompts, capture/PC summaries or other terminal battle variants. All
five original areas remain open. No push or deployment occurred.

Recommended next step: complete capture settlement recovery for party and full-party
PC placement, then Safari identity/correlation and full inventory/wallet/flag
reconciliation. Continue the endpoint/mutation, timer/callback, domain/wire and
lifecycle audits in the current five-area table.

## Pending move-choice recovery acceptance checkpoint (2026-10-03)

The existing move-learning transaction already clears the pending choice and its
deferred presentation events atomically with party persistence. This checkpoint
adds missing acceptance evidence; it does not change runtime behavior or claim
the remaining recovery work is finished.

The PostgreSQL dispatcher test covers both learning a move and skipping it. It
retires the battle cache before recovery, reads the exact pending move and index,
rejects dismissal while the choice is unresolved, commits the choice, discards
reply delivery and retires the cache again. Recovery then reads the terminal
revision, learned move (or unchanged skipped move), cleared pending state and
`needsDismissal` from durable storage. A duplicate with the original battle ID
and revision rejects without changing experience, wallet, Pokémon row identity,
move or durable revision. Ordinary dismissal succeeds and a final read confirms
the battle is absent.

The browser coordinator tests lose both the turn reply that produced a pending
prompt and the subsequent learning reply. For learn and skip, current-state
recovery restores the prompt first, then the settled party and dismissible state.
Late turn/learning replies cannot restore a prompt or replay learning/reward text.
Only one turn, one explicit choice and one close are sent; listeners and timers
retire. The recovered presentation uses current authority rather than inventing
a lost outcome message.

Verification: focused PostgreSQL race tests passed, including existing learning
rollback/publication and read-only recovery regressions. All 25 tests in the
battle coordinator, gameplay recovery service and recovery store files passed.
Typecheck and `git diff --check` passed. Logs:
`/var/tmp/capturequest-learning-recovery-go-final.log`,
`/var/tmp/capturequest-learning-recovery-front.log`, and
`/var/tmp/capturequest-learning-recovery-types.log`.

These checks seed a pending state through the actual transaction primitive and
exercise the dispatcher/storage boundary, plus the browser coordinator with
mocked transport. They do not prove natural level-up prompt generation, rendered
choice controls, real reconnect/owner handoff, process death, or capture-to-party/
PC summary recovery. No production build or rendered suite was repeated for
test/documentation-only changes. All five original goal areas remain open.

Recommended next step: add a data-driven rendered level-up/choice reply-loss
scenario, then complete capture settlement recovery (including full-party PC
placement) and Safari command identity/correlation. Inventory/wallet/flag recovery
and the endpoint, ownership, dependency/wire and lifecycle audits remain required.
This checkpoint is local only; nothing is pushed or deployed.

## Blackout scene ownership checkpoint (2026-10-03)

A battle timeout can recover a committed blackout destination in a different map.
The old scene then retires while its position projection is still settling. The
coordinator previously aborted that operation but retained its single-flight slot
until the projection finished. A destination scene could restore the finished
battle and attempt dismissal while that old slot still rejected commands.

Scene retirement/rebinding now aborts and immediately releases command admission.
The old operation's controller identity still guards its final cleanup, so late
completion cannot clear a destination command's pending state. Stale scene cleanup
cannot remove a newer scene binding. Abort also removes the old store subscription
immediately rather than retaining the scene through an unresolved projection.
The existing correlation and durable battle identity remain the transport and
mutation boundaries; cancellation does not imply server rollback or resend.

A focused regression first failed on the old implementation because the new close
was never sent. It now holds the old projection unresolved, retires/rebinds the
scene, restores a terminal snapshot and sends its close. It verifies immediate
old subscription retirement, stale cleanup isolation, protection of the new
pending state when the old promise resolves, dismissal through the destination projection and
listener/timer retirement.

The new data-only blackout fixture uses the extracted Bruno party and a weak
Magikarp with Splash at Pewter Gym. The browser proxy drops the actual terminal
reply, captures the server's committed destination/money, then verifies map
replacement, healed party, exactly one dismissal and no turn resend. Delivering
the old reply afterward cannot revive the panel. Character reentry preserves
¥499 from the original ¥999 and the full settled party. Destination coordinates
come from the actual response, not a test-only teleport or supplied MapLoad fields.

Verification completed locally:

- The focused ownership regression failed before both fixes: first the destination
  close was suppressed, then the old store subscription remained attached.
  All 79 focused frontend tests pass with immediate retirement and protected
  destination admission.
- Six isolated rendered tests passed: lost turn and late reply, lost close,
  terminal Brock reward progression, cross-map blackout recovery, pending trainer/
  battle-start recovery and Safari snapshot recovery. After the subscription
  cleanup change, the targeted blackout test passed again on the final source.
  Evidence: `/var/tmp/capturequest-rendered.vhxkDg` and
  `/var/tmp/capturequest-rendered.wORj04`.
- Typecheck, production build (existing large-chunk warning), runtime-asset
  validation through the build/bootstrap and `git diff --check` passed.
  Final logs: `/var/tmp/capturequest-crossmap-battle-front-final.log`,
  `/var/tmp/capturequest-crossmap-battle-types-final.log`,
  `/var/tmp/capturequest-crossmap-battle-build-final.log`,
  `/var/tmp/capturequest-crossmap-battle-rendered-final.log`.
- No Go implementation or generated wire contract changed. The rendered runner
  built the current server and exercised its real PostgreSQL/WebSocket boundary;
  no redundant broad Go suite was needed for this browser ownership change.
Pending move-learning/capture-summary and other queued-plan reply-loss acceptance,
Safari identity/correlation, full inventory/wallet/flag recovery and all unfinished
endpoint, mutation, callback, dependency/wire and lifecycle work remain required.
This proves a normal blackout loss across map replacement, not every terminal
battle variant. The five-area goal remains active. No server schema, generated
asset-family, push or deployment changes are included.

Recommended next step: cover pending battle choices/capture settlement after lost
replies, then migrate Safari encounter identity and correlated command recovery.

## Terminal battle dismissal and post-battle plans checkpoint (2026-10-03)

A lost terminal turn reply previously recovered `battle: null` while the durable
finished battle remained. The browser therefore never dismissed it, and the
close-triggered map-script lookup could be skipped until another scene request.
Separately, close deleted the battle before issuing the selected script in a
second transaction; failed issuance could leave neither a battle nor a durable
follow-up plan. These are state-transition gaps, not reasons to resend a turn or
repeat its rewards.

Recovery now retains finished battle identity and current party state with
`needsDismissal: true` when no move-learning choice remains. Reads still perform
no writes, rewards or issuance. Restoration marks this presentation for the normal
correlated close command, without replaying event text or inventing a win/loss
result. Pending learning choices remain interactive. Lost close acknowledgement
reads an absent battle and the already-durable plan; it sends no second close.
If close fails and recovery still finds a terminal battle, automatic dismissal
stops with the explicit reconnect error instead of entering a retry loop.

Dismissal and eligible map-script issuance now share one bounded character
transaction. The battle engine exposes `CloseBattleIn` to recheck durable identity
and terminal eligibility on that transaction. The dismissal domain validates saved
versus owned source, resolves native map provenance through the shared query,
reads flags/items/eligibility from durable storage and issues the existing locked
cutscene plan before commit. Publication follows commit. A deferred issuance
failure rolls back battle deletion and publishes no plan. The former separate
post-close issuance helper is retired. No script content or generated game assets
are changed; the new Brock scenario is only a runtime acceptance fixture using
existing trainer and reward-script data.

The first rendered run exposed a coordinator lifecycle race: restoration reset
`battleCommandPending` before position reconciliation released the in-flight slot,
so terminal auto-dismissal could be attempted too early and then missed. Recovery
now keeps the command pending through reconciliation and releases it only during
coordinator retirement. A focused test holds projection open and proves dismissal
can take the slot afterward. The rendered assertions were preserved; the producing
lifecycle was fixed.

The next rendered run reached and completed the Brock reward script, but its
reentry assertion found five TM34s instead of the earned one. The local test
inventory replenisher ran on every world entry. It now runs only for newly
created local/test characters, inside the existing character-creation transaction;
world reentry preserves inventory, including an intentionally empty inventory.
Character creation also uses its session command context. This does not change
production starter inventory. The existing broader character-creation pipeline
and its legacy global storage/cache dependencies still need their own audit.

A further unchanged party-equality assertion found stale derived stats in the
ordinary party view: battle rewards add effort values, and `LoadParty` recalculates
stats from them, but recovery applied the fresh party only to the battle panel.
Snapshot application now refreshes the shared party store from the same recovered
battle party. No stats formula, reward rule or assertion was relaxed; absence still
carries no party data and cannot clear an independently loaded party.

Verification completed locally:

- PostgreSQL race suites for world, battle, server and session passed after the
  domain extraction and local fixture changes. The focused local-mode inventory
  and terminal-plan regression tests also passed after explicitly enabling local
  test fixtures. Logs: `/var/tmp/capturequest-terminal-recovery-go-fixtures.log`
  and `/var/tmp/capturequest-terminal-recovery-fixture-focused.log`.
- The terminal protocol test simulates lost cache/notification, proves the read
  performs no character writes, retains the exact finished identity, then verifies
  atomic dismissal/plan issuance. A deferred INSERT-trigger failure rolls back
  deletion and emits no partial plan. Lost close/start delivery recovers the same
  durable plan token. Dismissal leaves the previously awarded money unchanged.
- All 78 focused frontend tests passed, including held reconciliation before
  terminal dismissal, failed-close recovery without automatic retry, and shared
  party projection from the authoritative recovery snapshot.
- All five final isolated rendered tests passed: lost committed turn and late
  delivery after reentry, lost close acknowledgement, terminal Brock reply loss
  followed by the existing badge/TM34 script, trainer/lost battle-start recovery,
  and Safari snapshot recovery. The Brock check preserves both the exactly-one
  TM34 assertion and full party equality after reentry. Evidence is retained
  locally under `/var/tmp/capturequest-rendered.0AlUTH`.
- Canonical `npm run tygo`, typecheck, production build (existing large-chunk
  warning) and `git diff --check` passed. Final logs are under
  `/var/tmp/capturequest-terminal-recovery-{front-final,types-final,build-final,rendered-complete}.log`.

Recommended next step: verify terminal blackout/loss across map changes and
pending learning/capture recovery, then migrate Safari encounter identity and
correlated commands. No push or deployment is included.
Cross-map terminal blackout/loss and pending-learning/capture-summary reply-loss
acceptance, Safari identity/correlation, full inventory/wallet/flag recovery, queued-plan and
catalog/source-change acceptance, process-death coverage, remaining mutation and
callback audits, domain/global-dependency migration and lifecycle acceptance remain
open. The full five-area goal remains active; this checkpoint is local only.

## Current-owned gameplay recovery checkpoint (2026-10-03)

The preceding battle recovery implementation queried owned position before the
coherent gameplay snapshot. `HandleOwnedPlayerPositionRequest` also resumes
pending trainer/cutscene notifications after replying. That could deliver plans
independently before the snapshot's battle-priority decision. Battle recovery now
uses one read on the existing gameplay-state endpoint, eliminating that preceding
position query and its independent notification delivery.

The explicit request selector `current: true` follows the selected character's
owned map after a potentially committed blackout/teleport; a nonzero `mapId`
alongside it rejects as contradictory. Existing scene-bound `{mapId, requestId}`
reads still require that exact owned map. Omission or a stale map never implicitly
selects current mode. Both modes share the same character-locked read, saved/owned
source agreement, validated battle/Safari/pending-plan snapshot and all-or-nothing
publication. Current mode adds no writes or script issuance. The browser read
helper returns authority without applying presentation; the battle coordinator
retains its scene/generation ownership guard, cancellation and no-mutation-retry
policy. Canonical generation updates the TypeScript request contract.

Verification completed locally:

- PostgreSQL race suites for world, battle, server and session passed. A rejecting
  UPDATE trigger proves current recovery performs no character writes. The new
  protocol test also proves one snapshot carries the pending cutscene without an
  independent notification, contradictory/missing/stale selectors reject, and
  current mode cannot bypass saved/owned position agreement.
- All 74 focused frontend tests passed, including current-mode request selection,
  changed-map acceptance without automatic presentation, cancelled read cleanup,
  timeout/send-error recovery, late replies and scene retirement.
- All six isolated rendered tests passed: lost terminal turn and close replies,
  battle keyboard menus, pending cutscene recovery, trainer/lost battle start and
  Safari recovery. The lost-turn proxy asserts exactly one explicit current-mode
  recovery request without resending the mutation. Evidence is retained locally
  under `/var/tmp/capturequest-rendered.SejBM3`.
- Canonical `npm run tygo`, typecheck, production build (existing large-chunk
  warning) and `git diff --check` passed. Logs are under
  `/var/tmp/capturequest-current-recovery-{go,front-final,types-final,build,rendered}.log`.

These checks do not prove every queued-plan/source/catalog ordering or abrupt
process-death recovery scenario.
The terminal trainer/post-battle script progression gap, Safari correlation,
inventory/wallet/flag recovery and every unfinished area in the table remain open.
No schema/assets change, push or deployment is part of this local checkpoint.
Recommended next step: prove and complete terminal trainer progression after a
lost turn reply, then add Safari encounter identity and correlated recovery.
The five-area goal remains active.

## Correlated ordinary-battle recovery checkpoint (2026-10-03)

Ordinary turn, forced-switch, item-alias, move-learning and close commands now
require a nonempty `requestId` of at most 64 bytes alongside the durable battle
identity. Successful replies, rule rejections and save failures echo that ID.
`BattleCommandResponse` carries the current battle, events, owned position and
optional terminal or learning outcome. The item alias answers on its own opcode;
close has an explicit acknowledgement (199) after durable deletion. Ordinary
terminal outcomes travel with the correlated reply instead of a separate
uncorrelated end notification. Battle-start blackout notifications remain separate.

Go owns the event and reply schemas. The existing battle event declarations were
mechanically extracted into `pokebattle/battle_events.go`; canonical `npm run tygo`
generates their TypeScript contract and the command DTOs. This replaces the
handwritten browser event interface without changing the battle engine rules.
Server and browser must eventually be released together: old request payloads and
flat reply consumers are incompatible with this contract. No schema or generated
asset-family changes are required by this checkpoint.

The browser coordinator permits one ordinary battle command at a time and sends
it over the reliable control stream. It accepts only the matching request and
response opcode, with a compatible durable battle identity. Scene retirement or
presentation replacement aborts listeners and timers, including replacement with
the same durable battle ID. Late replies cannot revive a retired panel. Close
keeps presentation until acknowledgement and projects committed position through
the owning scene. Local retirement during quit or warp sends no dismissal RPC.
Shared correlated requests now catch asynchronous send failures as well as
synchronous failures; migrated send wrappers return their transport promises.

Timeout, rejection and send failure recover authority rather than retrying a
mutation. Recovery reads current owned position, then the coherent map-bound
gameplay snapshot, applies it only while the captured presentation still owns the
operation, and reconciles its position. If recovery also fails, input remains
locked with an explicit reconnect message. This is current-state recovery, not a
historical result receipt or guaranteed replay of every original animation.

Validation completed locally:

- PostgreSQL race suites for world, battle, server and session passed, including
  missing/oversized correlation rejection, item-alias response ownership and
  bundled blackout outcome/position without an ordinary end notification.
- All 72 focused frontend tests passed. They cover single-flight admission,
  wrong/late replies, close acknowledgement, lost reply recovery without mutation
  resend, asynchronous send failure, same-ID presentation replacement, scene
  retirement during recovery and explicit failure when recovery times out.
- All 16 isolated rendered tests passed through the real WebSocket and PostgreSQL
  boundaries: duplicated turns, lost committed turn replies, late delivery after
  reentry, lost close acknowledgement, keyboard menus, trainer/Safari/cutscene/
  movement recovery, reload retirement and scripted events. Evidence is retained
  locally under `/var/tmp/capturequest-rendered.tPZest`.
- Typecheck, production build and `git diff --check` passed. The build retains its
  existing large-chunk warning. Test logs are local evidence under
  `/var/tmp/capturequest-battle-correlation-{go-final,front-final,types-final,build,rendered}.log`.

Remaining limits and next work:

- The owned-position read also resumes pending trainer/cutscene delivery. Its
  notifications can precede the subsequent coherent gameplay read. Replace this
  two-read recovery with an explicit current-owned mode on the existing gameplay
  snapshot endpoint, retaining strict map binding for scene-bound reads. This
  producer interaction is confirmed in `HandleOwnedPlayerPositionRequest`; the
  current rendered suite does not prove every overlapping-plan ordering.
- Finished battles without a pending learning choice are omitted from the
  recovery snapshot while their durable row remains. Post-battle map scripts
  normally issued on explicit close may need a later scene/map-script request
  after a lost terminal reply. This is a source-derived coverage gap: add a real
  trainer progression acceptance check before claiming terminal progression
  recovery is complete.
- Safari commands still need encounter identity, correlated replies and timeout
  integration. Unsolicited battle starts/start-blackout replies, inventory/wallet/
  flag recovery, historical outcome receipts and legacy send cancellation/error
  handling remain to audit.
- The remaining endpoint, mutation, timer/callback, dependency/wire and lifecycle
  acceptance work in the five-area table remains required. This checkpoint
  completes none of those areas in full.

Recommended next step: consolidate battle recovery into one current-owned snapshot
read, prove terminal trainer progression after reply loss, then apply identity and
correlation to Safari commands. This is a local branch checkpoint only; nothing
is pushed or deployed and the goal remains active.

## Network battle command identity checkpoint (2026-10-03)

The existing `CommitBattle` compares a captured server battle against durable
storage under the character lock. That protects concurrent stale server copies,
but the old network request carried only its action. Once the first request
committed, an identical packet captured the new server cache and could advance
another turn or spend another inventory quantity. Closing a battle likewise
captured whichever finished battle was current, even for a delayed old dismissal.

Ordinary turn, forced switch, item-use alias, move-learning and close commands now
require one explicit `battle: { battleId, revision }` identity. The shared match
check rejects missing, wrong, zero or stale identity before any mutation. The
existing transaction rechecks the captured identity against durable state under
the row lock. Successful commits increment the durable revision; an old packet
cannot apply to the next revision or to a replacement battle. The close path
cannot delete a newer finished battle. Decoding rejects unknown fields instead
of silently accepting a malformed command.

Ordinary battle starts, turn/no-turn responses and move-learning results publish
their actual committed identity. The existing store retains that identity even
while HP/events animate, and recovery already supplies it. `BattleCommandService`
uses generated request types and binds every menu action to the store's current
identity at send time. Dismissal captures it before clearing presentation. The
unused identity-free item-send helper is removed; the server item alias forwards
the same mandatory identity through the authoritative ordinary action handler.
Canonical Tygo generation owns these TypeScript contracts.

Supported version-zero battle saves are upgraded once during resume, inside the
existing character transaction, before an ID is advertised. The upgrade stores a
new identity and the current stable party row references in version two without
saving the party or advancing a turn. Late failure leaves the old JSON intact and
returns no playable identity. Subsequent resumes keep the same ID/revision.
Unsupported versions and malformed current-version identities still fail.

This is revision rejection, not an outcome receipt. A duplicate gets an explicit
rejection rather than the old success/animation. It proves at most one accepted
mutation for the expected battle revision; it does not identify which of two
competing commands won, retain an acknowledgement after close, or automatically
recover a lost reply. Safari actions still lack encounter identity/revision.
These are prerequisites for the remaining correlated timeout/recovery work,
not completion of that work or of the five-area goal.

The wire change is deliberately mandatory. Old clients cannot mutate ordinary
battles through identity-free requests; a future authorized release must ship
matching server/client contracts together. No schema or asset regeneration is
required by this checkpoint. No push or deployment is included.

Validation:

- Final PostgreSQL race suites pass for world, pokebattle, server and session.
  New real-dispatcher tests repeat the exact item packet with two available
  quantities, discard its reply, restore the saved battle and repeat again:
  only one item and turn commit. Missing/wrong/zero/future identity and unknown
  fields reject; a missing move-learning identity preserves its pending choice.
  A delayed close cannot delete a replacement finished battle. Existing rollback,
  move-learning, item, party and transport tests remain passing.
- The legacy resume test now verifies the required earlier upgrade boundary:
  a rejecting SQL constraint rolls back the upgrade with no published identity;
  successful upgrade preserves the party and turn; repeated resume retains the
  same ID/revision; cancelled action leaves the upgraded JSON intact.
- Nine focused frontend tests pass across command binding, gameplay recovery and
  battle restoration. Canonical generation, typecheck, matched runtime-asset
  validation and production build pass. The existing large-chunk warning remains.
- All seven isolated Chromium checks pass (1.8 minutes). The new proxy duplicates
  the real menu's first action packet: exactly one successful reply advances
  revision 1 to 2 and the duplicate explicitly rejects. Keyboard combat, trainer
  and Safari lost-notification reentry, Safari run and exhaustion also pass.
  The first run caught phase validation preceding stale-identity validation when
  a turn ended the battle; validation order was corrected without weakening the
  assertion. No commit/restore/close storage failures appear in the final log.

Evidence: `/var/tmp/capturequest-battle-identity-go-final.log`,
`/var/tmp/capturequest-battle-identity-front.log`,
`/var/tmp/capturequest-battle-identity-types-final.log`,
`/var/tmp/capturequest-battle-identity-build.log`, and
`/var/tmp/capturequest-battle-identity-rendered-final.log`. Browser/server artifacts:
`/var/tmp/capturequest-rendered.UuUbYT`. These are isolated local checks, not
production or process-death evidence. Rendered move-learning and forced-switch
coverage remains to be expanded; PostgreSQL covers their mutation boundary.

Recommended next step: correlate ordinary battle replies and apply current-state
recovery on timeout with proper retirement/stale-response handling, then add
Safari encounter identity and duplicate-command recovery. The remaining endpoint,
mutation, callback/ownership, domain/wire and lifecycle audits remain in scope.

## Coherent gameplay recovery checkpoint (2026-10-03)

A committed battle or Safari encounter can outlive its notification and connection.
The old scene startup used separate Safari requests and cached battle presentation,
so lost notifications could leave the browser without the current battle. Local
world-entry fixtures also deleted and recreated the party on every entry, breaking
saved battle references to stable Pokémon row IDs. Map loading kept an older player
actor when the fresh server actor had the same ID, leaving movement bound to the
startup map while rendering the new map.

`gameplay_recovery.go` adds correlated opcodes 197/198 with explicit generated JSON
and TypeScript names. One bounded transaction locks the character through
`SELECT ... FOR UPDATE`, without firing character UPDATE triggers. It validates
saved and owned position, then reads durable ordinary battle/party/phase, pending
move learning, Safari encounter/counters, pending trainer authorization and the
oldest pending cutscene snapshot. Conflicting sources or malformed state reject
the entire response. Battle presentation takes priority over a post-battle script.
The battle cache changes only after the complete read succeeds. Recovery grants
no rewards, advances no counters and creates no battle or script authorization.

The scene requests this snapshot after loading actors and completing arrival
animation, before admitting input. Overview mode remains read-only metadata.
Scene retirement aborts the correlated listener; mismatched maps reject application.
If a newer battle event overtakes the read, the browser reads once more before
applying it. Restoration shares the existing battle store and clears stale Safari,
pending-move and ordinary-battle fields without sending a close-battle mutation.
Pending trainer/cutscene presentation uses the existing authorized handlers.
Map-script loading does not issue playback over an active saved battle.

Local party fixtures now seed only an empty party under the character lock;
existing IDs, damage and scenario parties survive reentry. Explicit scenario
replacement remains an intentional debug operation. Fresh map actor snapshots now
replace the same-ID startup actor and update the scene's player reference before
movement initialization. These fixes address the producing paths, rather than
ignoring saved battle identity errors or relaxing owned-map validation.

This checkpoint needs no additional schema or generated asset changes. A future
release must ship the matching server/client opcode contracts and apply earlier
foundation schemas through the documented deployment workflow. Nothing has been
pushed or deployed.

Validation:

- PostgreSQL race suites pass for world, pokebattle, server and session. Final
  focused recovery/fixture tests pass after the party-seeding fix. They cover
  saved phase/party restoration, pending move choice, cache replacement,
  write-trigger isolation, corrupt-state rejection without partial publication,
  pending trainer to saved battle recovery, cutscene snapshots, source rejection,
  strict framing and locked-row cancellation.
- All 21 focused frontend tests pass, including stale-response retirement,
  timeout/retry, overtaken reads, Safari/ordinary state replacement and absence
  without a close-battle request. Typecheck, matched runtime-asset validation and
  production build pass; the existing large-chunk warning remains.
- All 13 isolated Chromium checks pass (2.5 minutes). A real WebSocket proxy
  drops every trainer notification and battle-start reply, then proves the same
  encounter token resumes and the same saved battle survives another reentry.
  Dropping legacy Safari entry/step/battle notifications still restores its
  battle and exact counters. Existing keyboard, movement receipt, cutscene,
  scripted movement, Safari exhaustion and reload checks pass.
- The first browser run exposed the deleted-party-row bug (12 passed, 1 failed).
  The final run has no saved-battle restore failures. It records six rejected
  old-map recovery reads when debug scenario commands change ownership during
  initial loading; the subsequent destination load succeeds. This is fail-closed
  source validation, not evidence of automatic command-level timeout recovery.

Evidence: `/var/tmp/capturequest-recovery-final-go.log`,
`/var/tmp/capturequest-recovery-final-focused.log`,
`/var/tmp/capturequest-recovery-final-front.log`,
`/var/tmp/capturequest-recovery-final-types.log`,
`/var/tmp/capturequest-recovery-final-build.log`, and
`/var/tmp/capturequest-recovery-final-rendered.log`. Browser/server artifacts:
`/var/tmp/capturequest-rendered.cDzqGH`. These checks use isolated local services;
production behavior and process-death recovery have not been verified.

Still required: integrate current-state recovery into remaining battle/Safari
command timeout handling; inventory, wallet, flags and other presentation recovery;
durable receipts for remaining mutations; process-death acceptance tests; queued
cutscene/source/catalog-change acceptance coverage; the remaining endpoint,
shared-state/callback, global-dependency, typed-wire, cancellation and final-save
recovery audits. All five original areas remain incomplete. Inventory fixture
replenishment on local entry is unchanged and is not production recovery behavior.

Recommended next step: add command-level battle/Safari timeout and duplicate
recovery, then continue the remaining mutation and callback audits.

## Durable cutscene issuance/completion checkpoint (2026-10-03)

Previously, `Session.IssuedCutscenes` kept authorization only in memory and cleared
it on disconnect. Delivery failure erased the issued token. Completion applied
rewards atomically, but consumed the token after commit without retaining an
outcome; a lost reply could not be distinguished from an unaccepted command.

`cutscene_issuance.go` stores a versioned script snapshot, original owned source,
character-scoped UUID token and resolution in `character_cutscene_plans`. The
script is the exact server-issued snapshot, not a later catalog lookup. Duplicate
pending issuance of the same snapshot/source reuses its token. Coordinate-triggered
issuance joins the position/effects/receipt transaction, so late failure leaves
neither a moved player nor pending authorization. Other authorized triggers save
issuance before notification. Delivery and annotation failures retain the plan.
The old session registry and its claim/finish/clear paths are removed.

The existing script interpreter loads and rechecks pending authority under its
character lock. Eligibility, rewards, party/flags, movement and the completion
outcome commit together. A failure leaves the plan pending for retry. Concurrent
completion executes effects once. Retained terminal results use a read-only path;
this matters because `LockCharacter` performs a no-op character UPDATE, which
would still fire database triggers. Replays return the saved completed status and
current owned position, without rewards, relative movement or presentation-effect
publication. They can acknowledge a result after a later teleport or fresh owner
without rewinding that owner.

Pending playback blocks ordinary movement acceptance and movement commits.
Owned-position and map-script reads can redeliver a pending snapshot/token.
Native/runtime actor annotations use the injected database and command context;
missing object annotations report the affected script/object rather than quietly
publishing a degraded payload. Shared destination changes cancel incompatible
pending sources transactionally. A completing script excludes its own token while
moving, then resolves it with the script effects.

An active declined choice or interrupted animation sends `cancel: true` through
the correlated completion endpoint before unlocking input. Cancellation grants no
rewards, frees pending movement admission and is safe to retry. Scene/session
retirement leaves pending authority for reconnect. Both completion and cancellation
retry once on response timeout with the same token and fresh correlation; explicit
rejection, cancellation of the request and send failure never retry. After a second
completion timeout the existing owned-state reconciliation remains available.

Retention is bounded: eight pending plans and eight retained terminal outcomes
per character in normal execution. Pending plans survive session replacement and
have no session-clock expiry. Resolution refreshes its ordering before pruning,
so a long-pending plan's newly committed outcome survives older receipt eviction.
Destination cancellation can temporarily retain up to sixteen terminal rows; the
next issuance/resolution prunes them. An evicted terminal token rejects completion
and cannot recreate effects. The store is not an unlimited event history.

Release boundary: this additive schema is required by cutscene preload. A future
authorized release must apply the canonical schema and ship matching server/client
contracts (`cancel`, `replayed`) together through the full-data lane, without a
reset. No push or deployment is included in this checkpoint.

Validation:

- Final race-enabled PostgreSQL checks pass for `internal/db/...`, `internal/session`,
  `internal/world`, `internal/pokebattle`, `internal/protocol`, `internal/scriptsim`,
  `cmd/server` and `cmd/import-phaser`.
- Focused PostgreSQL race checks cover fresh-owner snapshot/token resumption,
  catalog replacement isolation, late resolution rollback, current-position replay
  despite a rejecting character-update trigger, four concurrent completion calls,
  bounded pending/terminal retention, long-pending completion retention, movement
  admission, cancellation retries, destination cancellation and missing schema.
  Existing coordinate-step rollback now also proves no cutscene row survives.
- All 51 focused frontend tests pass across cutscene playback, movement completion
  and correlated map requests. They cover timeout retry limits, fresh correlation,
  cancellation/rejection/send failures, listener cleanup, projection/input locks,
  retirement and explicit cancellation of an interrupted active animation.
  Canonical Tygo generation, typecheck, runtime asset validation and production
  build pass; the build retains its existing large-chunk warning.
- All seven isolated Chromium checks pass (1.3 minutes). The new real-WebSocket
  test drops an Oak Lab cutscene notification, quits/re-enters and verifies the
  same issued token resumes. It drops the first successful completion reply,
  observes exactly two requests with the same token and different correlations,
  verifies a replayed completed result, walks to a later tile, re-enters again and
  proves old completion returns the later current position. Existing scripted NPC,
  player movement, ordinary-step receipt and reload checks also pass, with severe
  page-error assertions enabled. No cutscene issuance/annotation/completion errors
  appear in the final server log.

Evidence: `/var/tmp/capturequest-durable-cutscene-go-final.log`,
`/var/tmp/capturequest-durable-cutscene-frontend.log`,
`/var/tmp/capturequest-durable-cutscene-typecheck.log`,
`/var/tmp/capturequest-durable-cutscene-build.log`, and
`/var/tmp/capturequest-durable-cutscene-rendered.log`. Browser/server artifacts:
`/var/tmp/capturequest-rendered.gFqWzC`. The final retention-order correction is
covered by PostgreSQL tests after that browser run; it changes receipt eviction,
not the tested playback/wire flow. These are local checks, not production evidence.

Still required: full current battle/Safari/presentation resynchronization after
lost notifications; process-death acceptance checks; rendered trainer handoff and
declined-choice cancellation coverage; remaining mutation/selector atomicity,
interaction authorization, shared-state ownership, injected domains, typed wire
families, legacy cancellation and final-save recovery audits. Recovery currently
redelivers the oldest source-matching pending plan; resumption of multi-plan queues
and coherent presentation after source/catalog changes need wider acceptance
coverage. All five original areas retain unfinished work.

Recommended next step: verify and centralize current gameplay-state recovery for
pending trainer encounters and saved battles/Safari state, then complete the
remaining endpoint and callback audits. The goal stays active.

## Durable pending trainer checkpoint (2026-10-03)

Previously, sight encounters were held only in the trainer manager. Disconnect
removed them, and readiness deleted the pending entry before validating the actor
or committing a battle. A wrong actor or late database failure could erase the
encounter. The browser also sent readiness from `finally` after failed or retired
animations.

`trainer_transactions.go` extends the existing movement/battle transaction path.
The movement commit stores one versioned `character_trainer_encounters` row per
character, including a random token, stable object/native-map/class/party identity,
and the committed player source. Runtime actor IDs are resolved from the current
catalog on recovery; missing or changed identities fail explicitly. The row keeps
its latest resolution (`pending`, `battle`, `blackout`, or `cancelled`). Disconnect
clears presentation tracking only. Pending plans block ordinary walking before
animation and block other movement commits. Owned-position and map-script reads
can redeliver the pending notification using the same token and current actor ID.

Readiness validates the issued token, actor, saved and owned source, current battle,
durable flags and rebattle policy under the bounded character transaction. Trainer
party, player party, name/prize, obedience and seen-state reads/writes use that
transaction. Battle creation or blackout resolves the pending row in the same
commit. Storage failures leave the plan pending and publish neither battle nor
blackout; they cannot be interpreted as a fainted party. Duplicate readiness can
resend the current saved battle for that trainer without creating another battle
or repeating blackout. Destination changes through the shared field-destination
writer cancel an incompatible pending source in the destination transaction.

The notification/readiness DTOs now live in `internal/protocol`, use explicit JSON
names and generated TypeScript, and require `encounterToken`. The presenter sends
readiness only after a successful approach. Cleanup settles its pending delay;
generation checks ignore animation completion from a retired scene and prevent it
from unlocking a newer presentation.

Release boundary: the new table is additive and required by trainer preload.
A future authorized release must apply the canonical schema and ship the matching
frontend/backend together. The schema change selects the full-data deployment
lane; it does not require a reset. This checkpoint does not deploy anything.

Validation:

- Race-enabled PostgreSQL checks passed for `internal/db/...`, `internal/world`,
  `internal/protocol`, `internal/scriptsim`, `cmd/server` and `cmd/import-phaser`.
  Regressions cover forged actor/token preservation, late movement and readiness
  rollback, fresh-owner actor remapping/resumption, pending movement rejection,
  duplicate battle readiness, blackout rollback/retry without repeated charges,
  teleport cancellation, changed catalog identity and missing required schema.
  The first broad run exposed a catalog-only SQLite fixture lacking the new
  required table; the fixture now supplies it without weakening preload checks.
- All 27 focused frontend tests passed across the trainer presenter, movement
  controller and movement service. Presenter checks cover successful token delivery,
  active duplicates, settled delay cancellation, late animation retirement and
  animation failure. Tygo generation, typecheck, runtime asset validation and
  production build passed; the build retains its existing large-chunk warning.

Evidence: `/var/tmp/capturequest-pending-trainer-go-final.log`,
`/var/tmp/capturequest-pending-trainer-frontend.log`,
`/var/tmp/capturequest-pending-trainer-typecheck.log`, and
`/var/tmp/capturequest-pending-trainer-build.log`. These checks prove transactional
and frontend-controller behavior, not rendered trainer visibility or end-to-end
reconnect presentation. The previous 27-check rendered receipt run belongs to
`80a544c`; it is not evidence for this new trainer checkpoint.

Changes are checkpointed locally on `codex/server-foundations`, without push or
deployment. All five goal areas retain outstanding work.

Still required: durable cutscene issuance/completion across session replacement;
full current battle/Safari/presentation resynchronization after lost notifications;
abrupt process/network-loss and rendered trainer recovery checks; the remaining
mutation, ownership, wire/domain and cancellation audits in the five-area table.
The row retains only the latest sight encounter, not a general command history.
An interrupted animation currently requires a subsequent map/session recovery to
redeliver the plan; automatic retry/acknowledgement remains unfinished.

Recommended next step: persist cutscene issuance and its completion outcome through
the existing authoritative script transaction, then verify trainer and cutscene
recovery through real transports and rendered re-entry.

## Durable ordinary-step receipt checkpoint (2026-10-03)

After an ordinary step committed, losing its reply used to look like rejection:
its token existed only in the movement registration and was consumed on commit.
A fresh registration could read current position, but could not establish whether
that particular step had committed. Retrying the token returned a stale error.

`movement_receipt.go` stores a versioned `CommittedPlayerStep` in
`character_movement_receipts` inside the existing position/effects transaction.
The table keeps one row per character, with a character-scoped token and final
map/coordinates/direction. Only another successfully committed ordinary step
replaces it. Failed commits and mere step acceptance cannot replace it; forced
points and teleports leave it intact. The one-outstanding-step protocol bounds
storage without keeping an ever-growing log of walking commands. An older token
is no longer recoverable after the next ordinary step commits; it still cannot
execute again without a valid outstanding issuance.

Completion checks the durable receipt before consulting an in-memory token.
A match returns correlated success with `replayed: true`, without position writes,
actor broadcasts, counters, battle creation or trigger publication. This works
with a fresh movement manager/owner. An owned-position read may include
`stepToken`; its `committedStep` is historical while the response's ordinary
`mapId/x/y/direction` describe current ownership. A subsequent teleport therefore
cannot be undone by replaying an older receipt. Missing matches omit the receipt;
malformed/unsupported stored records return an error rather than a guessed result.
Startup requires this table before exposing readiness.

The browser retries a completion once on a response timeout using the same token
and a fresh request correlation. Cancellation, an explicit rejection and other
errors do not retry. A replayed result reads current ownership, discards queued
user movement/arrival callbacks, and projects that position; it does not activate
a warp from a historical tile. A queued server path stays exclusive. Scene/session
retirement ignores late reads. Failed replay reconciliation retains the input lock
until retirement.

The real WebSocket loss/re-entry check also exposed a lifecycle defect:
TileViewer listened only for Phaser `shutdown`, but game destruction emits
`destroy` directly. Its window/store listeners could survive quitting and handle
warps in a new game through a retired ScenePlugin (`queueOp` on a null scene
manager). Cleanup now runs on both lifecycle events, removing the hook pair so
restarts do not accumulate destroy handlers. The severe-error assertion remains
in the re-entry test.

Schema/release boundary: this adds an idempotent table to
`server/schema/postgres_runtime_schema.sql`. A future authorized release must
apply the tracked schema before starting this binary; the existing deployment
classifier places schema changes in the full-data lane. No reset is needed for
this additive table. Nothing is deployed by this checkpoint.

Validation:

- Race-enabled PostgreSQL checks passed for `internal/db/...`, `internal/world`,
  `internal/protocol`, `internal/scriptsim`, `cmd/server` and `cmd/import-phaser`.
  Receipt tests cover late failure rolling back position/daycare, preserving the
  previous receipt, loss before publication with a fresh owner, historical/current
  position separation, character isolation, unsupported/corrupt records (including
  missing/null coordinates) and missing-schema startup.
- All 45 focused frontend tests passed across five files, including timeout retry
  limits, correlation, cancellation, server-path recovery, retirement and failed
  reconciliation. Tygo generation, typechecking, runtime asset validation and the
  production build passed; the build retains its existing large-chunk warning.
- All 27 isolated Chromium checks passed (3.5 minutes). The added test drops the
  first successful completion reply at the real WebSocket boundary, observes
  exactly two completions with the same token and different correlation IDs,
  verifies a replay receipt/current-position read, then re-enters the character
  and confirms a historical replay cannot rewind a subsequent Instant Warp.
  The original severe-error and retired-coordinate-packet checks remain enabled.
  The first run exposed the destroy-cleanup defect; the final rerun passes with
  that fix. No receipt-read or movement-commit failures appear in the final log.

Evidence: `/var/tmp/capturequest-movement-receipt-go-final.log`,
`/var/tmp/capturequest-movement-receipt-frontend-final.log`,
`/var/tmp/capturequest-movement-receipt-typecheck.log`,
`/var/tmp/capturequest-movement-receipt-build.log`, and
`/var/tmp/capturequest-movement-receipt-rendered-final.log`. Final browser/server
artifacts: `/var/tmp/capturequest-rendered.vbJZ9H`; the earlier failure trace is
retained in `/var/tmp/capturequest-rendered.55g1PX`. These are local checks,
not production or throughput evidence. The final server check also verifies
stricter corrupt-record decoding added after the rendered run; valid receipt
encoding and frontend behavior are unchanged by that validation.

Changes are checkpointed locally on `codex/server-foundations`, without push or
deployment.

Still required: receipts/issuance for other mutations; durable delivery/resumption
of trainer and cutscene plans; full current gameplay-state resynchronization when
notifications are lost; abrupt process/network-loss acceptance checks; and the
remaining four-area audits alongside durable gameplay. This receipt proves an
ordinary step's saved outcome, not replay of every presentation effect. Double
timeout, disconnection before recovery and non-timeout transport failures still
require later session recovery. The full five-area goal remains active.

Recommended next step: durable pending gameplay plans and current-state
resynchronization, sharing the existing authoritative battle/Safari stores rather
than replaying old notification payloads.

## Atomic movement-step checkpoint (2026-10-03)

The previous completion paths saved `character_data.map_id/x/y` and published
movement before separately advancing daycare, Repel, Safari or a wild battle.
A later failure could leave the player at the new tile with only some effects
saved. Trainer/cutscene selection also changed live state before the complete
step had succeeded.

`movement_step.go` now coordinates one bounded, character-locked transaction.
It checks the saved source against the owned candidate and joins daycare EXP,
Repel counters, battle creation/seen registration, Safari steps/expiry, applicable
flags and recovery destination/wallet/party healing. Trainer and coordinate
cutscene selection use the transaction and a private durable flag snapshot;
publication waits for the outer commit. Forced paths also plan spin/current
continuation and automatic warp destinations here. Ordinary warps still use the
explicit warp command. Surf entry retains its existing wild-only step policy and
revalidates party/badge, adjacency and water inside this transaction.

The transaction adapter exposes context-aware queries without escaping to the
pool: both the outer deadline and a caller's cancellation apply. Failed steps
retain their source and publish no committed movement/effects. A consumed issued
step cannot replay after recovery returns to its own source tile. Forced recovery
publishes from the previous owned map, preserving departure provenance. Surf
blackout responses skip water animation and let the committed recovery project
the final position.

Verification completed:

- Race-enabled PostgreSQL Go checks passed for `internal/db/...`, `internal/world`,
  `internal/protocol`, `internal/scriptsim` and `cmd/server`. New boundary tests
  cover deferred failures, retry/duplicate acknowledgement, trainer/cutscene
  publication, stable daycare row identity, combined battle/Repel rollback,
  Safari expiry, blackout, Surf and cancellation of a blocked effect query.
  The recovery-to-source token and forced departure-map regressions passed.
- All 42 focused frontend tests passed across six files. Tygo regeneration,
  TypeScript checking, matched runtime asset validation and production build
  passed. The build retains its existing large-chunk warning.
- All 26 isolated Chromium checks passed (3.3 minutes): scripted events, normal
  and Instant Warp, multiplayer visibility, Surf/Cut/Strength, Safari and blackout.
  The test server log contained no movement-commit failures. Strength and Safari
  dialogue screenshots were inspected; this is local rendered evidence.

Logs: `/var/tmp/capturequest-atomic-step-go-final.log`,
`/var/tmp/capturequest-atomic-step-frontend-final.log`,
`/var/tmp/capturequest-atomic-step-typecheck.log`,
`/var/tmp/capturequest-atomic-step-build.log`, and
`/var/tmp/capturequest-atomic-step-rendered.log`. Browser/server evidence is under
`/var/tmp/capturequest-rendered.IoS5Xu`. These temporary local artifacts do not
prove production behavior, throughput or abrupt process-loss recovery.

This checkpoint is committed locally on `codex/server-foundations`; nothing was
pushed or deployed. Recommended next step: durable movement/result recovery
across transport loss, followed by the remaining mutation/domain audit.

Still required: durable command/result replay and reconnect recovery; durable
trainer/cutscene issuance; the remaining legacy mutations, callbacks, global
dependencies and wire families; final-save recovery and remaining lifecycle and
transport acceptance checks. Automatic warp map-load arrival effects still run
in the following MapLoad transaction. Legacy no-target Surf and unrelated field
mechanics have not all moved into this boundary. This checkpoint completes none
of the five goal areas by itself.

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
   saved gate destination. Issued ordinary steps and forced teleports now commit
   with applicable daycare, Repel, encounter, Safari and recovery effects in one
   transaction before live publication; movement saves retain dirty state on failure and
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

## Retirement of the client coordinate setter (2026-10-03)

Opcode 45 accepted client map/X/Y values after only catalog validation, then saved
and published them and applied facing/step effects. Catalog existence did not prove
that the character could move there. Ordinary movement, facing, cutscene completion
and explicit teleport commands already have authoritative owned-source boundaries;
the remaining coordinate reports came from a generic animation fallback, Surf
presentation and an uncommitted local teleport-event branch. The previous rendered
server log recorded opcode 45 during normal warp exit animations.

The server now rejects opcode 45 in every session stage, even if a handler is
accidentally registered. Its handler and request DTO are removed; wire number 45
remains reserved. The browser coordinate-send helper is removed. A completed
ordinary animation without an issued step stops with an observable error and cannot
acknowledge, activate local arrival effects or write coordinates. Surf and committed
warp exit animations explicitly use the existing server-controlled actor projection.
Warp exit completion/timeout callbacks check the movement generation before updating
presentation, so a retired animation cannot move a newer scene's controller.
Local teleport events reject missing committed-result provenance before sound or
scene changes. This marker is a presentation contract; server admission supplies
the security boundary.

The old setter's success test is replaced by dispatcher denial tests for valid
cross-map coordinates, same-map movement, facing and malformed packets. They verify
unchanged saved/owned/session position, direction, queued movement, dirty state and
multiplayer publication. Session-stage tests deliberately register a dummy legacy
handler to prove it remains unreachable. MapLoad's late-commit failure coverage is
retained; the blocked-transaction disconnect test now uses real issued-step completion.
Existing issued-step rollback/retry/duplicate checks continue to cover accepted moves.

Verification: race-enabled world/protocol/simulator suites and server compilation
pass in `/var/tmp/capturequest-retired-position-go-final.log`. 82 frontend tests pass
across seven movement, cutscene, map-request and map-loader files. Typecheck,
production build, runtime asset validation and stable protocol regeneration pass.
The rendered suite additionally observes outgoing binary WebSocket frames, decodes
the existing length/opcode layout and rejects any opcode 45 emission. A normal warp
case requires observed issued-step completion traffic to verify the observer.
Rendered acceptance: all 26 checks pass in
`/var/tmp/capturequest-rendered.Fp5O24`, with terminal output in
`/var/tmp/capturequest-retired-position-rendered.log`. They cover ordinary/Instant
Warp, Surf/Cut/Strength, scripted events/Oak movement, multiplayer visibility,
Safari and blackout. All packet-denial assertions pass; the observer also sees
actual issued-step completion traffic. The server log contains no opcode 45
requests, rejected movement intents/completions or failed cutscene applications
for this run. Strength before/after screenshots were inspected and retain the
visible player/boulder follow-up movement. This verifies the migrated local
flows, not every interaction or recovery requirement of the full goal.

Limits: coordinate setter retirement does not make step effects atomic with the
position commit. Ordinary completion still publishes its committed position before
separate daycare, encounter, Safari and script-trigger work. Durable result recovery,
remaining timer/callback ownership, endpoint/domain/contract migration and the full
five-area integration acceptance remain unfinished. Older clients relying on opcode
45 can no longer establish position; frontend and backend changes must be released
together when deployment is authorized. No push or deployment was performed.

Recommended next step: consolidate issued and timer-driven step effects around
one authoritative transaction/result boundary. Both `HandlePlayerStepCompleteRequest`
and `PlayerMovementManager.applyMovementStepEffects` call separate effect operations
after saving the point. `AdvanceDayCareSteps` starts an uncancelled global-database
transaction; `CheckSafariStep` and `tickRepel` start their own transactions. Reuse the
existing character-locked operations with an injected transaction/context, preserving
trainer/Safari/script/encounter ordering and publishing only after the combined
commit. Verify late effect failure, duplicate acknowledgement, cancellation and
Safari exhaustion before extending durable retry/reconnect recovery.

## Correlated cutscene completion and position recovery (2026-10-02)

The client formerly sent completion and unlocked without observing commit. Failed
transactions sent only chat text, leaving animated coordinates in presentation.
Request 121 now requires a correlation ID and accepts only label/token/ID fields;
response 194 contains committed owned map/position/direction, queued-movement phase
and whether the script
completed. Errors use the shared owned-position error contract. Claims and durable
script mutation still run under the existing character owner/transaction boundary.
The result is published after commit; a rollback reports the unchanged source.
Start and action types now come from the generated protocol and existing shared
scripted-action contract; the handwritten frontend duplicates are retired.

Position recovery requests 195/196 read the selected character's owned snapshot
under its command gate. They accept no destination, save no coordinates and apply
no arrival/step effects. The client uses the existing correlated request primitive
for completion and recovery, with timeout, cancellation and stale-response cleanup.
A correlated failure reconciles its owned snapshot. An unconfirmed outcome reads
owned state rather than replaying the event or assuming rollback. If both result
and recovery fail, input remains locked until scene/session retirement; this is
observable in a terminal diagnostic, not treated as success.

Playback keeps input locked through the result and position projection, including
nested `unlockInput` actions. Only confirmed completed results advance the completed
script marker. A following issued script arriving before the result is queued and
deduplicated by token, then starts after reconciliation. Retirement clears that
queue, aborts listeners, stops only the controller's own active tweens and settles
its pending movement promises; stale tween callbacks cannot update player tiles.

Reconciliation compares the movement controller's current owned map, correcting
an unchanged-map actor in place. Reusing warp routing initially compared the view
registry's map identity and caused repeated map loads and empty Seafoam map-script
issuance. A regression test varies those identities and requires no scene reset.
Cross-map recovery uses the existing committed destination presentation path.

The stricter rendered Strength check then caught two ordinary intents while the
server still owned a queued push step: `map=192 x=18 y=11 path=1`. Boulder updates
made its source tile look empty before the first committed player point arrived.
Facing results now expose `serverMovementPending`, and pending facing excludes
ordinary keyboard/click/path intents until the result arrives. A queued result
reserves server movement through its final projected point. Owned success/error
snapshots carry the same phase atomically with their location; position correction
and rejected ordinary steps preserve that phase. This fixes the acceptance-to-first
point interval without weakening the severe-error assertion.

Verification: 78 focused frontend checks pass across completion lifecycle,
owned/correlated reads, actor/player movement, map loading and committed-position
presentation. They cover delayed success/projection, rollback, unknown outcome,
failed recovery, duplicate queued scripts, cancellation during completion and
animation, stale tween updates, map-identity mismatch and the acceptance-to-first
server-point interval. Race-enabled world/protocol/simulator suites and server
compilation pass in `/var/tmp/capturequest-cutscene-ack-go-final.log`. Typecheck,
production build, runtime asset validation and stable protocol regeneration pass.
The read-only recovery test runs with a trigger rejecting all character updates;
late completion failure and retry tests verify owned/durable position and result
packets. Dispatcher tests reject both operations before character selection.

Rendered acceptance: all 26 checks pass in
`/var/tmp/capturequest-rendered.9TabcV`: scripted events/Oak movement, ordinary and
Instant Warp, multiplayer visibility, Surf/Cut/Strength, Safari and blackout.
Inspected Strength screenshots confirm the visible follow-up step. The final
server log has no rejected movement intents or failed cutscene applications for
these cases. It still contains opcode 45 packets during normal warp tests, proving
that the remaining animation/teleport producers have not been retired. This is
checkpoint acceptance, not completion of the full five-area goal.
Initial integrated runs retained at `/var/tmp/capturequest-rendered.LngOaS` and
`/var/tmp/capturequest-rendered.5rd7Sh` each passed 25/26 cases. Their Strength
failures respectively exposed the map-reload loop and the two rejected ordinary
intents during queued server movement. Assertions were preserved. The final run above verifies the corrected behavior; these initial runs remain
failure evidence.

Limits: session token consumption still does not recover a committed reward result
after reconnect or process failure. Cancellation does not revoke the server's
issued token; its existing bounded lifetime/capacity remain. Failed position recovery
has no retry UI yet, and reliable-result loss/cross-map retirement and transport
ordering need broader recovery work. Opcode 45 remains available through its last
generic producers. Field/step-effect atomicity, other callback ownership, remaining
domain/contracts and the complete five-area acceptance checks remain unfinished.
No push or deployment was performed.

Recommended next step: audit the remaining generic coordinate-report producers,
migrate genuine intent to authoritative commands, and retire opcode 45; continue
with durable result recovery and atomic field/step effects afterward.

## Issued cutscene movement source binding (2026-10-02)

The cutscene scene callback sent every animated player tile through opcode 45.
Completion then applied the issued `movePlayer` relative movements from the current
owned location. This created two position writers for one action sequence and
allowed animation reports to alter the interpreter's input position.

The scene callback now updates presentation only. Server issuance stores a private
snapshot containing the script and its owned map/source coordinates. Completion
claims the existing character-bound token and validates both live and saved source inside the
character-locked script transaction before applying any rewards or relative moves.
Nested movements share the detached transaction position, starting from the issued
source. A changed source rejects completion; failed transactions preserve the token
for retry. The simulator and battle interpreter continue using the existing shared
transaction without an issued-session source requirement.

Verification: the isolated rendered run
`/var/tmp/capturequest-rendered.3LMDSZ` passes all seven scripted-event/field-move
cases, including the real Oak Lab player movement and the next ordinary issued
step, parcel reward, Surf/Cut and Strength. Its server log contains no opcode 45
reports. The final durable-source guard also passes both scripted cases in
`/var/tmp/capturequest-rendered.gvG0kq`; no opcode 45 reports or completion
errors appear in its server log. A corpus audit of 380 script JSON files finds
24 `movePlayer` actions and no `move` actions targeting `__PLAYER__`. The source-bound
transaction test proves stale live state and stale saved state grant no rewards,
late failure rolls movement/rewards back, retry applies nested movement from the
original source, and duplicate completion adds nothing. Final race-enabled
world/protocol/simulator results and server compilation are recorded in
`/var/tmp/capturequest-cutscene-source-go-final.log`. Fifty-two focused frontend
tests, typecheck, production build and runtime asset validation pass; protocol
regeneration leaves generated files unchanged.

Limits: this removes the cutscene callback's coordinate reports; it does not disable
the generic opcode 45 endpoint. Remaining producers include the no-issued-step
animation fallback and uncommitted teleport event handling. Cutscene completion
still lacks a correlated success/error acknowledgement, so client unlock timing,
failed-completion reconciliation, cancellation of player tweens, notification loss,
and durable reconnect recovery remain unfinished. Issued tokens authorize session
completion; they are not a durable replay log. Complete those lifecycle paths and
the five-area roadmap before claiming the goal complete. Nothing was pushed or
deployed.

Recommended next step: add a typed correlated completion result and keep cutscene
input locked until committed success or explicit failure/reconciliation, then audit
and retire the last generic coordinate-report producers.

## Owned facing and committed server-path projection (2026-10-02)

Facing previously shared opcode 45 with coordinate reports. Requests 191/192 now
contain expected owned map/source coordinates and a direction, with no destination.
The character command gate rejects stale sources, replacement sessions, active
issued steps, queued server paths, battle state, malformed payloads and cancellation.
A pure turn changes facing only; it neither saves position nor executes step effects.
The frontend coalesces identical pending turns, retires listeners on scene cleanup,
and reconciles rejected facing only while still idle at the response's owned source.

The new rendered Strength case exposed another defect: the boulder moved from
`(18,10)` to `(18,9)` and the server saved the player at `(18,10)`, but TileViewer
preserved the local player's `(18,11)` for every actor update. Dedicated notification
193 distinguishes committed server-controlled path points from ordinary actor refreshes.
Its generated fields include actor/map identity, position, direction, sprite, speed
and path completion. Local presentation retires prediction, animates the committed
point and stays busy through unfinished paths. Its completion neither echoes a
coordinate write nor runs a second local warp activation.

The timer previously advanced live position before saving, ignored save errors,
and could defer saving intermediate points. It now plans a detached point, reads
NPC blockers without the shared player lock, commits through the existing bounded
character transaction, and only then updates/publishes owned state. Failure retains
source/path for retry and executes no step effects. A blocked path terminates without
pretending a step completed. Tests inject a late database failure and prove both
saved and owned sources stay unchanged, then verify a successful retry's typed
origin notification. Pure-facing tests run with a trigger rejecting all character
updates, proving turns do not write position.

Verification: 57 focused frontend checks pass across correlated requests,
player/actor movement, map loading and teleport presentation. Typecheck, production build, runtime asset
validation and stable protocol regeneration pass. The isolated rendered run
`/var/tmp/capturequest-rendered.jE6KYf` passes 23 of 24 cases, including Strength,
Surf/Cut, Safari, blackout, multiplayer visibility and normal warps. Inspected
Strength before/after screenshots show the boulder moving and the player following.
The one failure is Instant Warp's immediate click path: it times out waiting for
`cq:playerPositionChanged`; the server receives intent 187 but no completion 189.
That evidence does not establish whether acceptance, response handling or animation
failed. Keep this previously intermittent ordering issue open. This run therefore
is not an all-green integrated acceptance result.

Final race-enabled world/protocol/simulator tests and server compilation are
recorded in `/var/tmp/capturequest-facing-projection-go-final.log`.
Bicycle cosmetic refreshes retain the ordinary actor-update contract so they
cannot retire local prediction as if they were committed server path points;
the final server test asserts that opcode boundary.

Limits: adjacent Strength still invokes the legacy field subsystem. Its boulder
mutation and following player point are separate transactions, and step effects
(Day Care, encounters, scripts and counters) remain separate from position commit.
This checkpoint does not establish durable notification recovery after disconnect,
all delayed/burst notification orderings, server-path changes during scene/warp
retirement, every callback writer, or movement throughput. Legacy script/field
coordinate reports, global dependencies, final-save recovery and the other five-area
acceptance checks above remain open. No push or deployment was performed.

Recommended next step: bind scripted animation reports to the existing issued
cutscene/action authority and retire that remaining coordinate writer; then finish
field/step-effect atomicity and reconnect result recovery. Keep the full goal active.

## Source collision and issued-step overlap follow-up (2026-10-02)

Authoritative steps exposed a producing-pipeline defect previously hidden by
opcode 45. Red's House exit mats `(2,7)` and `(3,7)` were exported with
`collision_type = 2` and `raw_foot_tile_id = 20`, so ordinary walking required
Surf on an indoor carpet. Original source `data/tilesets/collision_tile_ids.asm`
includes `$14` in `RedsHouse1_Coll`; `data/tilesets/water_tilesets.asm` excludes
`REDS_HOUSE_1`. `engine/items/item_effects.asm:IsNextTileShoreOrWater` gates water
by that tileset list and the tile in front of the player, including the `$32`
Ship Port platform exception. The extractor instead classified any `$14`/`$48`
subtile in a square as water without checking its tileset.

The bundled extractor now derives the water tileset list from that original
source and classifies the collision foot sample. It retains source walkable
lists and rejects unsupported/missing water-list records. Walkable sets are
loaded once instead of querying SQLite for every quadrant. This is a pipeline
repair, with no per-map collision override or generated-file hand edit.
Canonical asset bootstrap regenerated the complete family and passed validation;
script candidates remained unchanged (321 unchanged). The temporary client
blocked-carpet shortcut was removed before validating this root-cause repair.

Corpus comparison: 284 rows across 101 maps change: 113 water-to-land,
130 water-to-blocked and 41 blocked-to-water (source shore semantics). All 826
tile-art identities and hashes remain identical. Local SQLite hash changed from
`d0c5fa0b58f2f9d950cbce079cf509d994dd02b39a2cd7cfb990587d045d5709` to
`005940406542b401972489f4d1a2e30d4e614df397fe3ea66c69b3573de0628e`;
the new runtime contract records the latter. These hashes identify local runs,
not a deployed release or expected CI byte identity.

The client also keeps an issued animation/completion exclusive when future input
is discarded. `stopMovement()` preserves its token and busy state; ordinary
clicks cannot replace acceptance, animation or completion while outstanding.
Explicit scene retirement/snaps still retire issued work. A deferred-completion
test exercises keyboard/click overlap both during animation and before the reply.

Verification: four extractor tests pass, including a real source map export and
shore/platform cases. Every one of the 94,876 regenerated squares matches the
final classifier. All eight targeted repeats pass in
`/var/tmp/capturequest-rendered.buu8t9`; all 23 integrated checks pass in
`/var/tmp/capturequest-rendered.vD72sg` (warps, Instant Warp, multiplayer visibility,
Surf/Cut, Safari and blackout). Forty-one focused frontend tests, typecheck,
production build, runtime asset validation, workflow YAML parsing and nonmutating
script-candidate verification pass. Final importer/world/protocol/simulator race
results are recorded in `/var/tmp/capturequest-water-final-go.log`.

The first repeated run passed all four Instant Warp cases but failed all four
house cases at sideways mat entry; this led to the source diagnosis above rather
than a collision exception. The previous intermittent Instant Warp source mismatch
did not reproduce in either repeated run or the final integration run. These
checks establish current tested behavior; they do not explain that earlier race
or prove every delayed actor/scene/transport ordering.

Extractor commit `ed8b7d5` is local on `codex/source-water-collision`; the parent
checkpoint records that submodule commit. A future authorized publication must
publish the extractor commit before a checkout/deployment can fetch the parent
pointer. Collision data changed, so a future deployment requires the canonical
full-data lane, not a code-only release. No publication or deployment was run. The full five-area goal remains active.
Next: retire legacy facing and issued script/field coordinate reports, then
finish step-effect atomicity, durable result recovery and delayed/session races. No push or deployment is authorized.

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
