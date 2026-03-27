# Frontend Support E2E Notes

This folder contains the frontend Playwright setup for support inbox realtime behavior.

## What is here

- A standalone harness page for presence-heavy tests where we want exact control over websocket events and list/thread state.
- A full-app route test that boots the actual authenticated support page and exercises the real route tree, workspace shell, and `useRealtimeSync`.
- Shared fixtures that seed auth tokens, mock `fetch`, replace `window.WebSocket`, and expose a tiny browser controller for Playwright.

## Design Intent

- Harness tests are fast and target precedence logic directly.
- Full-app tests verify the actual app route still wires the same behavior correctly after layout, auth, and query changes.
- Browser mocks live at the Playwright boundary so product code keeps using the normal services, hooks, and websocket logic.

## When editing

- If a test times out before “ready”, first check missing mocked endpoints and websocket bootstrap.
- If React errors mention providers, compare the harness tree with `src/routes/__root.tsx` and the workspace route tree.
- If websocket sends appear empty, check the mock `WebSocket.OPEN` constant and reconnect behavior before touching app logic.

## Minimum bar for new support presence changes

- Keep the harness suite green.
- Keep the full-app support route spec green.
- Keep `pnpm --dir frontend run build` green.
