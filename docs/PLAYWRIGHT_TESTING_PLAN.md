# Playwright Testing Plan

This plan is for building reliable overnight browser coverage for CaptureQuest
without depending on screenshot inspection. Screenshots and traces should remain
failure artifacts only; assertions should primarily use DOM state, test-only
diagnostic state, and server-visible gameplay state.

## Goals

- Prove the whole login pipeline works from the public title screen through
  guest login, character creation, character select, and entering the world.
- Cover the multiplayer visibility bugs that are easiest to miss manually:
  players entering interiors should disappear for outdoor viewers, and players
  leaving interiors should reappear only to viewers on the destination map.
- Cover door, warp, and movement regressions where the client is trusted for
  local movement but still reports state to the server.
- Cover high-risk gameplay flows with deterministic setup, especially field
  moves, battle keyboard navigation, dialogue choices, and scripted events.
- Keep tests stable enough to run repeatedly against one local `dev:test`
  server.

  Do not push any changes to GitHub or deploy the game to the live site.

## Test Principles

- Prefer visible user flows over direct store mutation.
- Prefer state assertions over pixel assertions.
- Use screenshots, video, and traces only for debugging failures.
- Add test-only diagnostics behind `VITE_TEST_MODE=true`; never expose them in
  production builds.
- Make cleanup part of each test. For example, if a test enters the world, it
  should quit back to character select before the browser context closes.
- Avoid timing guesses. Use `expect.poll`, visible UI markers, server events, or
  diagnostic state changes.
- Keep browser tests broad but not exhaustive. Use Go/script scenario tests for
  exhaustive flag/item/script combinations.

## Current Implementation Status

Implemented:

- Test-only browser diagnostics bridge behind `VITE_TEST_MODE=true`.
- Stable Playwright transport flag, `VITE_FORCE_WEBSOCKET=true`, for Docker
  browser runs.
- Shared helpers for guest login, character creation, state polling, tile
  clicking, dialogue dismissal, console error collection, and Scenario Debugger
  jumps.
- Login/title tests covering title entry points, guest login, character
  creation, character select, and entering the game.
- Multiplayer visibility test covering actor removal when another player
  changes maps.
- Door/warp test covering real tile clicks through Red's House warps.
- Field move tests covering Scenario Debugger Surf and Cut setups through real
  player input.
- Battle keyboard test covering Space/arrow navigation through an active wild
  battle fixture.
- Scripted-event browser test covering a real Viridian Mart parcel interaction
  and its persistent UI result.
- Asset/bootstrap smoke test covering the generated database, tile images,
  Pokémon sprites, player sprites, fonts, audio manifest, and extractor
  submodule entrypoint.
- Debug scene fixtures now preserve direction and can seed active battles for
  browser tests.
- The suite has been verified with two consecutive Docker Playwright runs
  against one local `dev:test` server.

Still good next targets:

- More scripted-event browser coverage for Oak intro/lab flow and the old man
  tutorial, using the same state/DOM assertion style.
- Dialogue choice selection diagnostics if we want to assert the selected
  Yes/No option directly instead of asserting the resulting state.
- More movement coverage around bike speed, surf transitions, and exit
  animations once those systems settle further.

## Test Diagnostics Bridge

Add a test-only browser bridge, available only when `VITE_TEST_MODE=true`, such
as:

```ts
window.__capturequestTest = {
  getState(): CaptureQuestTestState;
  waitForEvent(type: string, timeoutMs?: number): Promise<unknown>;
};
```

Suggested state shape:

```ts
type CaptureQuestTestState = {
  screen: string;
  connected: boolean;
  map: {
    id: number | null;
    name: string | null;
  };
  player: {
    id: number | null;
    name: string | null;
    x: number | null;
    y: number | null;
    direction: string | null;
    isSurfing: boolean;
    isCycling: boolean;
    isMoving: boolean;
  };
  visibleActors: Array<{
    id: number;
    name?: string;
    type: string;
    mapId: number;
    x?: number;
    y?: number;
  }>;
  dialogue: {
    isOpen: boolean;
    text: string;
    choice?: "yes" | "no" | null;
  };
  battle: {
    isOpen: boolean;
    phase?: string;
    selectedAction?: string;
    selectedMoveIndex?: number;
  };
};
```

Event hooks to emit in test mode:

- `cq:connected`
- `cq:screenChanged`
- `cq:mapChanged`
- `cq:playerPositionChanged`
- `cq:actorSpawned`
- `cq:actorDespawned`
- `cq:dialogueOpened`
- `cq:dialogueClosed`
- `cq:battleOpened`
- `cq:battleClosed`
- `cq:cutsceneStarted`
- `cq:cutsceneEnded`

This lets Playwright wait for exact state:

```ts
await expect.poll(() =>
  page.evaluate(() => window.__capturequestTest.getState().map.name)
).toBe("REDS_HOUSE_2F");
```

## Shared Playwright Helpers

Create `tests/e2e/helpers/` with:

- `auth.ts`
  - `loginAsGuest(page)`
  - `createCharacter(page, name)`
  - `enterWorld(page, name)`
  - `quitToCharacterSelect(page)`
- `state.ts`
  - `getGameState(page)`
  - `waitForMap(page, mapNameOrId)`
  - `waitForPlayerTile(page, x, y)`
  - `waitForActorVisible(page, predicate)`
  - `waitForActorAbsent(page, predicate)`
- `input.ts`
  - `pressMovement(page, direction, count?)`
  - `pressSpace(page, count?)`
  - `clickTile(page, x, y)` if tile-to-screen conversion is exposed by the
    diagnostics bridge
- `errors.ts`
  - collect console errors and page errors
  - filter known harmless font/audio warnings
  - fail tests on severe client errors

## Phase 1: Foundation

1. Stabilize the current `game-login.spec.ts`.
2. Add diagnostics bridge with map/player/visible actor state.
3. Move repeated login/character helpers out of the spec into shared helpers.
4. Add a no-severe-console-errors fixture.
5. Run browser suite twice against one `dev:test` server as the standard
   repeatability check.

Target command:

```bash
npm run dev:test
E2E_APP_URL=http://localhost:5178 npm run test:e2e:docker
E2E_APP_URL=http://localhost:5178 npm run test:e2e:docker
```

## Phase 2: Multiplayer Visibility

Add `tests/e2e/multiplayer-visibility.spec.ts`.

Tests:

- Two guests create characters and enter Red's House 2F.
- Player A leaves Red's House 2F for Red's House 1F.
- Player B should receive an actor despawn/update and no longer see Player A.
- Player A exits to Pallet/Kanto.
- A viewer on the outdoor map should see Player A appear at the outdoor tile.
- A viewer still inside Red's House 2F should not see Player A.

Assertions:

- Use `visibleActors` diagnostics, not screenshots.
- Use server logs only for debugging, not as test truth.

## Phase 3: Door And Warp Regression

Add `tests/e2e/warps.spec.ts`.

Tests:

- Start in Red's House 2F, use stairs to Red's House 1F.
- Exit Red's House 1F.
- Verify the outdoor exit animation finishes one tile away from the door.
- Press left/right after exiting and verify the player does not re-enter.
- Re-enter the house.
- If placed on the left of two door tiles, press right and verify the player
  moves sideways instead of zoning out.
- Use Warp Home and verify destination is Pallet/Kanto at `9,4`.

Assertions:

- map id/name
- player tile
- movement lock state during exit animation
- no repeated warp events

## Phase 4: Field Moves

Add `tests/e2e/field-moves.spec.ts`.

Tests:

- Scenario Debugger Surf-ready setup.
- Click water, choose Yes, verify player moves onto water and `isSurfing`.
- Move back to land, verify `isSurfing=false`.
- Scenario Debugger Cut-ready setup.
- Click cut bush, verify bush disappears locally and server allows passage.
- Enter and exit an interior, verify cut bushes regenerate.
- WASD against water/bush should not auto-use; Space should attempt the field
  move.

Assertions:

- dialogue text and choice state
- player tile
- surf/cut flags from diagnostics
- tile/object visibility from diagnostics

## Phase 5: Battle Keyboard Flow

Add `tests/e2e/battle-keyboard.spec.ts`.

Tests:

- Start a simple battle from Scenario Debugger.
- Press Space to progress opening text.
- Press Space again to choose Fight.
- Press Space again to choose first move.
- Repeat until battle closes or reaches a deterministic state.
- Use arrows/WASD to change selected action and selected move.

Assertions:

- battle is open
- selected action changes
- selected move changes
- no severe console errors

## Phase 6: Dialogue And Scripted Events

Add `tests/e2e/scripted-events.spec.ts`.

Implemented tests:

- Viridian Mart parcel fixture can be jumped to through Scenario Debugger.
- The clerk interaction runs through real browser input.
- The persistent item-receipt UI result appears.
- The scripted event does not leave the player in battle or stuck in dialogue.

Assertions:

- persistent UI result text
- dialogue closed after script progression
- battle remains closed
- no severe console errors

Future candidates:

- Oak grass intro starts when walking into grass.
- Oak/player movement cutscene completes.
- Lab flow reaches starter choice state.
- Old man tutorial can run once and then changes dialogue after flagging.
- Game Corner prize room NPCs/vendors route to distinct behavior.
- Yes/No prompts respond to Space and arrow/WASD selection.

## Phase 7: Asset And Bootstrap Smoke

Implemented in `tests/e2e/assets-smoke.spec.ts`. These do not need full browser
rendering.

Tests:

- Required generated assets exist after bootstrap.
- Key generated files exist and are non-empty:
  - `public/phaser/pokemon.db`
  - player sprites
  - trainer back sprite
  - Pokémon front/back sprite directories
  - map tile images
  - Pokémon font
  - audio manifest
  - extractor submodule entrypoint

Future candidate:

- `npm run bootstrap:assets` can be run in a temporary clean checkout once the
  public repo process is stable enough for a longer integration test.

## Overnight Run Strategy

Use a script such as `scripts/testing/run-e2e-overnight.mjs` later. First pass
can be manual:

```bash
npm run typecheck
npm run test -- --run
cd server && go test ./...
cd ..
npm run dev:test
E2E_APP_URL=http://localhost:5178 npm run test:e2e:docker
E2E_APP_URL=http://localhost:5178 npm run test:e2e:docker
```

For an actual overnight loop:

```bash
for i in $(seq 1 20); do
  echo "E2E run $i"
  E2E_APP_URL=http://localhost:5178 npm run test:e2e:docker || break
done
```

Store Playwright reports and traces only on failure. Do not commit
`test-results/` or `playwright-report/`.

## Definition Of Done

- Full Playwright suite passes twice back-to-back against the same local
  `dev:test` server.
- No test relies on manual screenshot interpretation.
- `npm run typecheck`, `npm run test -- --run`, and `cd server && go test ./...`
  pass.
- The local dev server is stopped after the run.
- No deployment happens unless explicitly requested.
