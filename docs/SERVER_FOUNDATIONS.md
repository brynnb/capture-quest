# Server foundations goal

Status: active. Started 2026-09-25 from `02c51ba`.

Working branch: `codex/server-foundations`. Latest implementation checkpoint:
source-authorized merchant opening and scene-bound replies, following injected merchant reads and owned-map selection `5ac9f66`, then `21fd084` — durable shop revisions and correlated command recovery, explicit isolated simulator targeting `74defe0` and committed shop inventory snapshots `5082a39` and coherent inventory/wallet/party/flag recovery `129463c` and ordinary step crash acceptance `e0ecf41`, issued cutscene and creation-only fixtures `8c95d40`, move-choice acceptance `e0da0c4` and terminal Safari acceptance `9bf3630`
(2026-10-03), following rendered capture recovery `fbe744e`, simulator contract migration `9f59dd3`, expiry presentation recovery `54dbef6`, guarded Safari commands `d673aea`, durable capture placement and terminal login retention `58d0b85`, rendered move-choice recovery `7eb3a7e`, move-choice storage/coordinator acceptance `072ad71`, blackout scene ownership `04579dc`, terminal dismissal/post-battle plans `8ce43ff`, current-owned gameplay recovery `c38a74c`, correlated battle recovery `30fa1bb` and network battle command identity `8a5ba4a`, coherent gameplay recovery `c0d31f9` and durable cutscene issuance/completion `e9eb834`, pending trainer encounters `84f2d91` and ordinary-step receipts `80a544c`, following atomic movement-step effects `cfdeb9e`, retirement of the client coordinate setter `2df1db0`, correlated cutscene completion `6638a63`, issued cutscene source binding `18ebc34`, facing/server-path projection `be79129`, source collision/issued-step overlap `4177378` and issued ordinary steps `9fd9b84`, owned-only MapLoad
`0585dde`, committed
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

### Checkpoint handoff (2026-10-03)

Implementation checkpoints are committed locally on `codex/server-foundations`.
The goal remains active; no push or deployment is part of these checkpoints.
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
durable identity/revision guards and terminal catch placement recovery. Terminal Safari Run, party/PC captures, pending move choices and one issued Oak's Lab cutscene now have actual crash/restart acceptance. Ordinary walking now has acceptance for abandoning uncompleted tokens and recovering committed receipts without repeating Safari counters. Local/test party and inventory setup both belong to creation; reentry preserves intentionally empty parties.
The coherent recovery response now includes inventory, wallet, the full party and
sorted flags. Inventory reads share one bounded transactional reader, and shop
buy/sell replies include their complete committed bag without client stack guessing.
Shop mutations now require character-bound durable revisions and correlated replies;
the client waits for acknowledgement or current-state recovery without resending. Merchant opening now uses source clerk reach and fresh script eligibility with scene-bound correlated replies. Integration with remaining legacy mutation timeouts and remaining
process-recovery coverage still need work. Evidence and verification limits
appear in the checkpoint sections below.

There is no reliable overall completion percentage: the remaining endpoint
and ownership audits can reveal additional work. Use the five-area status
table and the acceptance checks below to assess completion, rather than the
number of commits or passing tests. All five areas still have outstanding work.

| Area | Implemented | Still required |
| --- | --- | --- |
| Request/session boundary | Packet and connection limits, centralized session prerequisites, removal of insecure session takeover, actual transport closure, location/visibility checks for merchant opening, scripted clicks, dialogue choices and direct trainer battles, client destination catalog validation, server-resolved normal warp activation, explicit Instant Warp commands, committed teleport notification contracts and read-only map metadata, retired coordinate/map setters, and preserved command deadlines/disconnect cancellation in migrated operations. | Audit remaining interaction/mutation endpoints; propagate cancellation through legacy managers and remaining database/network work. |
| Durable gameplay | Shared bounded transactions; atomic shops/inventory, stable Pokémon row identities, party/item changes, battle persistence, script rewards/completion, trade rollback/deduplication, atomic Vermilion puzzle transitions, item-ball collection, Silph doors, Game Corner prizes and bounded coin/slot/hidden-coin operations, atomic Escape Rope/FLY positions, durable Repel counters, Safari entry/turn/capture state and exhaustion destinations, atomic blackout/recovery destinations and map-load position/Safari/flag/visibility/boulder effects, atomic movement-step counters, encounters and recovery, durable latest ordinary-step receipts, durable sight-trainer plans/resumption and atomic readiness resolution, durable cutscene snapshots/completion receipts/cancellation, coherent battle/Safari/pending-plan recovery, mandatory ordinary/Safari battle command identity, correlated battle timeout recovery and retained Safari terminal/capture state, recoverable terminal dismissal and atomic post-battle plans, and commit-before-publication in migrated paths. | Finish remaining dynamic puzzles, pickups, prize/field-effect paths; durable duplicate protection and recovery for remaining mutations across reconnects; finish recovery integration for remaining inventory/wallet/flag mutations and presentation, finish process-death acceptance for remaining movement/script/trainer plans and choices; finish queued-plan/source/catalog ordering acceptance coverage and remaining command recovery. |
| Character ownership | Bounded serialized session commands, exclusive character ownership and drained handoff, stale-cleanup guards, immutable cross-session presence, movement ticks coordinated with the owner, and immediate retirement of battle-scene command admission/subscriptions. | Finish timer/callback/shared-state and legacy position-writer audits; prove remaining concurrent/reconnect behavior across real transports. |
| Domains and wire contracts | Injected content-query service; typed inventory, shop opening and mutation successes, character/wallet/bind, Pokédex/card, content detail, map-script, map-info/list, sight-trainer notification/readiness, coherent gameplay recovery, ordinary/Safari battle command replies, shared battle events and learnset contracts generated from explicit JSON names. | Migrate remaining gameplay/query families and global dependencies; retire `StructToMap` and the casing postprocessor after every consumer moves. |
| Lifecycle and verification | Owned HTTP/listeners, readiness, listener failure propagation, joined periodic workers, sealed session admissions, fail-closed staged preload, startup cancellation, atomic scripted-event publication, and deadline-aware shutdown waits with returned failure results. | Audit cancellation of remaining legacy work, define durable final-save recovery, and complete transport/rendered integration coverage. Owned HTTP and player transport retirement and isolated active-player shutdown checks have landed. |

### Next work and completion criteria

Merchant opening now requires a reachable visible source clerk, checks current
script eligibility, and returns a typed reply correlated to the live character
and scene. The immediate next audit is purchase/sale interaction policy: opening
is a read and does not issue a durable merchant permission for later mutations.
Purchases currently validate the owned map and offer; sales validate ownership
and item policy. Determine and enforce the source merchant boundary for both,
then finish rendered sale and actual shop process-death acceptance. Continue
party/field item commands through the same typed ownership and recovery boundary.

Continue with recovery integration for the remaining mutation commands and the
remaining script/trainer plans and their queue/source/catalog ordering. Ordinary
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
