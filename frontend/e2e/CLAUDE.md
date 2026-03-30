# Frontend Support E2E Notes

This folder contains the frontend Playwright setup for support inbox realtime behavior.

## What is here

- `support/harness/`: standalone harness page for presence-heavy tests where we want exact control over websocket events and list/thread state.
- `support/app-mocked/`: full-app route tests that boot the authenticated support page and exercise the real route tree, workspace shell, and `useRealtimeSync`.
- `support/fixtures/`: shared fixtures that seed auth tokens, mock `fetch`, replace `window.WebSocket`, and expose a tiny browser controller for Playwright.
- `support/live/`: placeholder for later backend-backed smoke coverage.

## Canonical Layout

This directory split is intentional and should be preserved:

- `harness/` for broad deterministic state coverage
- `app-mocked/` for route-shell integration with mocked transport
- `live/` for thin backend-backed smoke only

Do not collapse these layers back into one flat support e2e folder.

## Design Intent

- Harness tests are fast and target precedence logic directly.
- Full-app tests verify the actual app route still wires the same behavior correctly after layout, auth, and query changes.
- Live tests, when added, should stay thin and prove only high-value end-to-end integration.
- Browser mocks live at the Playwright boundary so product code keeps using the normal services, hooks, and websocket logic.

## When editing

- If a test times out before “ready”, first check missing mocked endpoints and websocket bootstrap.
- If React errors mention providers, compare the harness tree with `src/routes/__root.tsx` and the workspace route tree.
- If websocket sends appear empty, check the mock `WebSocket.OPEN` constant and reconnect behavior before touching app logic.

## Minimum bar for new support presence changes

- Keep the harness suite green.
- Keep the full-app support route spec green.
- Add a live test only if the mocked layers cannot prove the risk you are targeting.
- Keep `pnpm --dir frontend run build` green.
