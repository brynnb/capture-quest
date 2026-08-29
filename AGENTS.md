# CaptureQuest contributor instructions

## Read before changing or deploying data

- Read `docs/DEPLOYMENT.md` immediately before every production deployment.
- Read `docs/EXTRACTOR_SCHEMA_V2.md` and `docs/DATA_PIPELINE_OVERVIEW.md` before
  changing the extractor boundary, generated assets, tile IDs, map data, audio,
  sprites, or the SQLite-to-Postgres importer.
- Inspect the repository, dirty state, recent history, generated contract, tests,
  and live deployment state before recommending or implementing a repair.
- Generated files under `public/phaser`, `public/sound/pokemon`, and related
  ignored output directories are products of the canonical bootstrap. Do not
  hand-edit them to repair one visible map or asset.

## Data and asset identity

- `public/phaser/pokemon.db`, `public/phaser/tile_images`, and
  `public/phaser/runtime_asset_contract.json` are one atomic artifact family.
  Never deploy or validate one member independently.
- Numeric `tile_image_id` values are catalog-local identities. A stable filename
  such as `tile_123.png` can represent different art after a catalog rebuild.
  Runtime tile URLs must retain the generated `RUNTIME_TILE_ASSET_VERSION` query
  parameter. Do not remove it or introduce an unversioned parallel URL helper.
- Validate the live database against the live contract. A locally generated
  SQLite file can have a different database hash from the CI artifact while
  still sharing the same tile catalog, so do not compare unrelated runs as if
  they were byte-identical.
- Fix generated-data defects at the earliest incorrect layer: extractor,
  CaptureQuest adapter/sync, Postgres import, or runtime presentation. Do not
  hide malformed or mismatched source data behind client fallbacks.

## PostgreSQL import invariants

- Production imports must be transactional and must run only after
  `import-phaser --preflight-only` accepts the extractor schema and release.
- Never run two importers against the same production database. An SSH client
  disconnect does not prove the remote process or PostgreSQL backend stopped;
  inspect the exact importer PID and `pg_stat_activity` before retrying.
- Keep the tile merge set-based. `phaser_tiles_import_stage` must have its
  coordinate index, and updates must use one `UPDATE ... FROM` join. Do not add
  per-column correlated staging-table subqueries: at production scale they turn
  the roughly 95,000-row merge into quadratic work and can hold locks for many
  minutes.
- Preserve current manual tile edits during ordinary imports. Source-derived
  values update the `original_*` snapshot while `has_tile_edit = 1` keeps the
  current user value. A deliberate one-time repair belongs in
  `phaser_data_repairs` with an idempotent key.
- Never use a reset deployment merely to repair static data. A reset drops
  player/runtime state and requires explicit user intent.

## Production deployment invariants

- The GitHub Actions deployment is the canonical path. A deployment includes
  dependency installation, complete asset generation or validated cache restore,
  asset-contract validation, frontend/backend builds, focused tests, a compressed
  PostgreSQL backup, preflight, schema application, import, service restart, and
  live verification.
- Always create and verify a non-empty backup in `$DEPLOY_APP_DIR/backups`
  immediately before schema/import mutations.
- SSH commands that can run imports must use client keepalives. Preserve
  `ServerAliveInterval`, `ServerAliveCountMax`, and `TCPKeepAlive` in the workflow.
- CaptureQuest needs several seconds after `systemctl start` to preload scripted
  events, collision maps, actors, encounters, and warps before binding port 8080.
  Health checks must retry for at least 60 seconds; an immediate 502 is not proof
  that startup failed.
- Generated-asset cache keys should depend on extractor and generation inputs,
  not unrelated workflow formatting. Cache readiness must still validate the
  database, contract, tile directory, generated catalog version, scripted events,
  sprites, and complete audio manifests before accepting a restore.
- Keep the service-restart cleanup trap, but restart only when the deployed
  database, frontend, and runtime asset contract are known to be one coherent
  catalog. If an import outcome or activation is ambiguous, leave `cq-server`
  stopped and follow the importer recovery procedure before serving traffic.
- Do not treat a green build as a successful deployment. Verify the live service,
  recent journal, database invariants, served SQLite hash, runtime contract,
  audio MIME type, and stale-client asset behavior.

## Tile authoring safety

- The Tile Manager and Tile Art Studio are local-development tools. Production
  must neither render their controls nor register their HTTP mutation routes.
- WebSocket world-tile mutations must continue to enforce server-side GM/admin
  authorization; hiding a button is not an authorization boundary.
- Test tile changes through the real observable boundary: paint or erase,
  persist, reload, and broadcast. Preserve native-coordinate provenance and
  explicit erased-original state.

## Testing and resource discipline

- Run focused tests while iterating, then broaden checks in proportion to risk.
  For importer/deployment changes, at minimum run the importer tests, workflow
  YAML parse, `git diff --check`, runtime asset validation, and the production
  build when frontend asset loading changes.
- Do not claim a visual behavior is fixed from a state test alone. Use a rendered
  browser check for UI behavior such as modal dismissal or map presentation.
- `/tmp` may be RAM-backed. Put large databases, downloaded asset catalogs,
  builds, and comparison trees under `/var/tmp`.
- Avoid overlapping asset generation, broad tests, or importers. CaptureQuest's
  complete graphics/audio pipeline is resource-intensive.
- Preserve unrelated dirty work. Do not use destructive Git commands or broad
  process termination. Stop only exact processes started or positively identified
  for the current operation.

## Communication and completion

- Explain the root cause with actual artifact hashes, row counts, field names,
  process state, or logs before proposing a workaround.
- A requested deployment is not complete until production health and the
  applicable data/asset invariants have been verified after restart.
- End substantial reports with the best recommended next step and clearly state
  whether changes were committed, pushed, or deployed.
