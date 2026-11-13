# Nakama feasibility: purchase and party-item commands

Evaluated 2026-10-03 against CaptureQuest `67af5ee`. This completes the bounded
architecture evaluation requested after the server-foundations review. It is a
source/API assessment, not a Nakama deployment or runtime benchmark.

## Decision

Use Nakama and Colyseus as architectural references. **Do not start a full Nakama
migration for these two operations.** With our current relational player data,
using Nakama for RPC would retain the existing SQL transactions and most command
recovery work. Using its managed transaction API would instead require migrating
inventory, party and dependent writers into its storage model. Neither route
directly removes the repeated client command lifecycle identified in this review.

The evidence supports a smaller next milestone: consolidate command execution
and client reconciliation across purchase and party-item use, keeping the current
Go/PostgreSQL persistence and content pipeline. This recommendation is an engineering
judgment based on the mappings below, not a measured implementation-time estimate.
Nakama remains a credible full-backend option if replacing authentication,
transport, social services and persistence becomes an explicit project objective.

## Evidence and limits

- Nakama [v3.41.0](https://github.com/heroiclabs/nakama/releases/tag/v3.41.0),
  released 2026-09-18, resolves to commit
  `cab8af584947ff0d7ebee91d6c33e67d87fd3e47`. Source links below pin that commit.
- Its [Go module](https://github.com/heroiclabs/nakama/blob/cab8af584947ff0d7ebee91d6c33e67d87fd3e47/go.mod)
  requires Go 1.27.1 and `nakama-common` 1.48.0. CaptureQuest's module declares
  Go 1.24.2. Adopting the Go plugin requires a matching server/build toolchain;
  this assessment did not upgrade either project or compile a plugin.
- Inspected transaction, storage, wallet, RPC and match-handler source, plus
  CaptureQuest's handlers, stores, schema and existing tests. Thirteen upstream
  files and a SHA-256 manifest were saved under
  `/var/tmp/cq-nakama-evaluation-atnqy8bm`; pinned links make the evidence usable
  after that temporary directory is removed.
- Colyseus comparison uses its current official
  [state synchronization](https://docs.colyseus.io/state) and
  [reconnection](https://docs.colyseus.io/room/reconnection) documentation.
  It is a documented design comparison, not a source audit of its implementation.
- No Nakama/Colyseus server, migration, load test, crash test or client integration
  was run. Compatibility and performance of a deployed replacement are unverified.

## What Nakama centralizes, and what remains ours

| Concern | Verified mechanism | Consequence here |
| --- | --- | --- |
| Atomic writes | `MultiUpdate` starts its own transaction for account, storage and wallet operations. | It cannot join our caller's `*sql.Tx` or execute an arbitrary SQL callback over `cq_item_instances` and `character_pokemon`. Keeping those tables means retaining our SQL boundary. |
| Conflicting storage writes | A supplied storage `Version` is checked; `"*"` means create only. | Useful shared primitives for state and a command receipt. The receipt policy is still application code. Versions are content hashes, not increasing sequence numbers. They cannot replace our `revision + 1` wire contract without redesign. |
| Wallet debit | Nakama locks and updates `users.wallet`, rejecting negative balances. | Our `character_wallet` is per character, while accounts can own multiple characters. Mapping one Nakama user to an account does not preserve wallet semantics automatically. |
| RPC response matching | WebSocket RPC echoes the envelope's `Cid`. | The inspected RPC path invokes the registered function on each request. Correlation does not provide durable duplicate suppression or replay a committed result. |
| Gameplay ownership | Each match has a queued callback loop and in-memory state. | This is a useful model for one owner of state, but arbitrary RPCs do not automatically enter that match queue. A per-match owner also differs from our per-character owner. |
| Crash recovery | Managed storage persists explicitly written data. | The match's in-memory state and browser presentation still need an application recovery design. A dropped connection and a dead server process are different acceptance cases. |

Sources: [MultiUpdate](https://github.com/heroiclabs/nakama/blob/cab8af584947ff0d7ebee91d6c33e67d87fd3e47/server/core_multi.go#L27),
[public Go implementation](https://github.com/heroiclabs/nakama/blob/cab8af584947ff0d7ebee91d6c33e67d87fd3e47/server/runtime_go_nakama.go#L2458),
[storage versions](https://github.com/heroiclabs/nakama/blob/cab8af584947ff0d7ebee91d6c33e67d87fd3e47/server/core_storage.go#L74),
[conditional writes](https://github.com/heroiclabs/nakama/blob/cab8af584947ff0d7ebee91d6c33e67d87fd3e47/server/core_storage.go#L775),
[wallet ownership](https://github.com/heroiclabs/nakama/blob/cab8af584947ff0d7ebee91d6c33e67d87fd3e47/server/core_wallet.go#L116),
[RPC dispatch](https://github.com/heroiclabs/nakama/blob/cab8af584947ff0d7ebee91d6c33e67d87fd3e47/server/pipeline_rpc.go#L27),
[match ownership](https://github.com/heroiclabs/nakama/blob/cab8af584947ff0d7ebee91d6c33e67d87fd3e47/server/match_handler.go#L175).

## Operation 1: buy ten Potions

The existing [purchase test](../server/internal/economy/shop_test.go) starts with
1,000 currency, 95 Potions in one stack, an offer price override of 10 and stock
of 100. Buying ten yields 900 currency, stacks of 99 and 6, and stock of 90.
[Buy](../server/internal/economy/shop.go) also advances the shop revision in the
same transaction. The [handler](../server/internal/world/handler-cqitems.go)
authorizes the current source clerk and publishes the complete committed bag.

**Keep existing tables:** a Nakama RPC authenticates the account, validates its
selected character and calls the current economy service. Its SQL transaction,
stack allocation, stock lock, revision guard and recovery snapshot remain.
Custom socket/request plumbing could be replaced when the entire transport moves,
but adding a Nakama RPC around `Buy` alone removes little of this operation.

**Use managed storage:** represent each character's bag and currency in a
server-written storage object owned by its Nakama account; put finite merchant
stock in a system-owned object. Read their versions, apply the existing pricing
and stack rules, then submit the changed objects together. A separate character
wallet object can participate in that same batch. This preserves per-character
currency without treating every character as a new authenticated user.

Concrete API mapping, with application values prepared beforehand:

```text
StorageRead: character bag/currency, merchant stock, receipt for command ID
validate account -> character, clerk, offer, stock, quantity and balance
compute bag with stable instance IDs and remaining stock
MultiUpdate(ctx, nil, writes, nil, nil, false)
  writes: character object at observed Version
          finite stock object at observed Version
          command receipt with Version="*"
return committed outcome; recover current state separately if delivery is uncertain
```

This is an API sketch, not executed code. Each storage write would use
`runtime.StorageWrite` with explicit `Collection`, `Key`, `UserID`, JSON `Value`,
`Version` and server-only write permission. Receipt contents, retention, command
payload matching and replay behavior are application decisions. Two attempts with
the same command ID must not become two purchases after rereading newer state.
On a version conflict, inspect the matching receipt before deciding the outcome;
do not automatically retry the player's mutation with a fresh expected version.

The current relational IDs and stack behavior must survive conversion. Price,
clerk visibility/reach and imported catalog interpretation remain CaptureQuest
rules. Finite stock must move into the same transaction family as bag/currency;
splitting that mutation between custom SQL and `MultiUpdate` loses atomicity.

## Operation 2: use a Potion on the party

The existing [party-item tests](../server/internal/itemuse/party_test.go) use a
Pokémon at 1/95 HP and a Potion that heals 20. One successful use yields 21/95 HP
and consumes one item; failure rolls both back. Concurrent distinct uses on a
two-item stack may legitimately both succeed. The current request has no durable
command ID, so it cannot distinguish those distinct intentions from duplicate
delivery of the same intention. Transaction safety alone does not resolve that.

**Keep existing tables:** a Nakama RPC calls
[`itemuse.Service.UsePartyItem`](../server/internal/itemuse/party.go).
`db.Transaction`, character locking, owned-instance lookup and party persistence
remain. The RPC wrapper does not add duplicate suppression or automatically
reconcile the browser's separate party/inventory notifications.

**Use managed storage:** read the versioned character bag and party, verify the
selected stable Pokémon identity, apply the existing medicine effect and submit
bag, party and create-only command receipt in one `StorageWrite` batch (or the
same `MultiUpdate` wrapper used for purchases). One shared application executor
can build these writes for both operations; each operation supplies its rules.

This requires preserving `character_pokemon.id`/`Pokemon.RowID`, nickname,
ownership, party order and move state. All writers of these records must migrate
coherently: battle settlement, PC movement, trade, daycare, rewards and scripts.
An evolution item also changes Pokédex state; a TM prompt remains read-only and
its later selection revalidates the current Pokémon/moves. Those are boundaries
of the conversion, not extra features to implement during this evaluation.
Static `phaser_*` content can remain in its authoritative SQL/import pipeline.

## What would actually disappear

| Route | Can retire | Must retain or replace |
| --- | --- | --- |
| Nakama RPC with current SQL | Our authentication/session/socket plumbing only after a complete transport cutover. | Both transaction services, character authorization, command identity/recovery, game rules and browser state handling. A second parallel transport is extra maintenance. |
| Nakama with managed player storage | Relational bag/party/wallet repositories after all affected readers/writers move; some custom authentication and transport code. | Data conversion, identity mapping, shared receipt policy, content rules, client synchronization and deployment tooling. |
| Consolidate current runtime | Repeated command admission/cancellation/recovery orchestration and legacy item quantity-only publication. | Existing transaction/ownership primitives and domain rules. This is the recommended bounded follow-up. |

Adoption would need a verified one-time conversion and a write cutover with an
explicit rollback boundary. A permanent dual-write arrangement between Nakama
storage and our relational player tables is not an acceptable migration plan.
Nakama supplies a PostgreSQL Docker configuration and PostgreSQL transaction code;
this assessment has not validated our production schema or deployment against it.

## Client model to borrow from Colyseus

Colyseus gives the server ownership of a typed state tree and synchronizes changes
to clients. Its reconnection flow sends a complete current snapshot. That is the
relevant design principle: UI stores consume one authoritative state projection;
each command does not invent another bag/party repair procedure. The documented
reconnection flow requires a retained seat and is not proof of persistent state
recovery after killing the server.

In CaptureQuest, keep `CorrelatedRequest` and `GameplayRecoveryService`, and put
their use behind one scene/character-owned command coordinator. Domain adapters
provide validation, expected identity and presentation. Committed state and
one-time presentation (sound, animation, dialogue) must remain distinguishable
so recovering state does not replay old effects. Full snapshots are sufficient
for the initial bag/party proof; a general delta protocol is outside this scope.
Adopting Colyseus itself requires a TypeScript/JavaScript server and its protocol,
which is a larger choice than consolidating the existing Go transport.

## Finite next milestone and stop condition

Implement purchase and the existing outside-battle party-item handler through a
common command lifecycle, using Potion as the representative item acceptance
case. Preserve all existing item rules in the shared item service; do not create
a separate Potion path. Extend the existing session, transaction and recovery primitives.
Keep domain authorization, pricing, stack rules and medicine effects explicit.
Define one durable duplicate-result policy and one client uncertain-result flow;
do not introduce an independent party-item coordinator.

Completion requires:

1. Both operations use the common executor/coordinator. The old implementations
   of the migrated orchestration are removed; existing protections stay covered.
2. Payment/item grant and consumption/healing remain atomic with stable IDs.
   The same command cannot execute twice; two distinct valid uses still can.
3. A lost reply or reconnect recovers current bag/party/currency without
   resubmitting a mutation or applying a historical snapshot over newer state.
4. Generic cancellation, duplicate, rollback and late-result behavior is tested
   through a shared contract, with the two operations providing fixtures and
   assertions for their own rules. Existing shop crash acceptance is reused.
5. A before/after review shows the repeated lifecycle code was removed and
   records any necessary per-domain policies. Stop at these two consumers;
   assess the reduction before authorizing migration of another family.

Do not create a universal movement engine, add event sourcing, adopt a broker,
or migrate the database as part of that milestone. If the abstraction requires
many operation-specific flags or duplicates the existing owner, revise it at
this boundary rather than extending it across more endpoints.

## Assessment verification record (before implementation)

Source/API mapping is complete. Existing tests passed against disposable
PostgreSQL with `-race` using:

```sh
bash scripts/testing/run-go-postgres.sh ./internal/economy ./internal/itemuse \
  -run 'TestPurchaseUsesOfferAndPreservesOverflow|TestShopRevisionRejectsConcurrentDuplicatesAndSurvivesNewService|TestPartyItemRollbackAndRetry|TestConcurrentPartyItemsDoNotLoseEffectsOrConsumeExtraItems'
```

Economy completed in 1.498 seconds and itemuse in 1.505 seconds; the wrapper
stopped and removed its private database. These tests validate the current
CaptureQuest behavior, not a hypothetical Nakama port. `git diff --check` passed.
This assessment commit changed no game runtime code, generated assets, dependencies
or production state. The subsequently authorized two-consumer implementation is
recorded in [SERVER_FOUNDATIONS.md](SERVER_FOUNDATIONS.md#shared-inventory-command-checkpoint-2026-10-03).
