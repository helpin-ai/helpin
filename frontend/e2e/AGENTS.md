# Frontend Support E2E

This directory holds the Playwright browser suites for `frontend`, with support coverage organized by runtime layer.

## Scope

- `support/harness/`: isolated support presence harness. It renders the real conversation list and realtime hooks inside a lightweight fixture page.
- `support/app-mocked/`: authenticated full-app support route tests. They drive the real `/w/:slug/support/:conversationId` route with mocked API and mocked websocket transport.
- `support/fixtures/`: shared Playwright fixtures, API mocks, and websocket controller setup for support tests.
- `support/live/`: reserved for backend-backed browser smoke tests. This layer is intentionally opt-in and not part of the default suite.

## Directory Decision

This structure is the canonical support browser-test layout moving forward.

- Put precedence-heavy state tests in `support/harness/`.
- Put real route-shell coverage in `support/app-mocked/`.
- Put reusable mock/bootstrap code in `support/fixtures/`.
- Put backend-backed smoke coverage only in `support/live/`.

Do not add new support specs back at the flat `frontend/e2e/` root.
Do not mix mocked and live cases in the same directory.

## Rules

- Keep support e2e deterministic. Do not depend on a real backend, database, or external websocket server in these tests.
- Prefer mocking browser transport at the page boundary instead of adding test-only branches in production React code.
- If you add a new support query in the real route, update `support/fixtures/supportE2E.ts` so the full-app spec continues to boot cleanly.
- If you add new presence precedence behavior, cover it in `support/harness/support-presence.spec.ts` before changing UI code.
- Keep `support/live/` thin even after it is implemented. It is for smoke coverage, not the full matrix.
- If a scenario can be covered deterministically, prefer `harness/` or `app-mocked/` over `live/`.
- Use Chromium as the required browser for this suite. Add other browsers only after the Chromium path is stable.

## Commands

```bash
pnpm --dir frontend run test:e2e:support:install
pnpm --dir frontend run test:e2e:support
```

On Linux CI or fresh machines, Playwright may also need system deps:

```bash
pnpm --dir frontend exec playwright install --with-deps chromium
```

## Extension Points

- Add harness-only state cases by extending the query params or controller in `support/harness/support-presence-harness.tsx`.
- Add full-app cases by reusing `installSupportAppMocks()` from `support/fixtures/supportE2E.ts`.
- Add backend-backed smoke tests under `support/live/`, but keep them opt-in until bootstrap and cleanup are deterministic.
- Keep assertions focused on real user-visible precedence: customer typing, teammate typing, unread state, viewer state, reconnect snapshots.
