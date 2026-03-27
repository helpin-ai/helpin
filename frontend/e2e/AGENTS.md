# Frontend Support E2E

This directory holds the Playwright support inbox suite for `frontend`.

## Scope

- `support-presence.spec.ts`: isolated support presence harness. It renders the real conversation list and realtime hooks inside a lightweight fixture page.
- `support-full-app.spec.ts`: authenticated full-app support route test. It drives the real `/w/:slug/support/:conversationId` route through the app shell with mocked API and mocked websocket transport.
- `support-presence-harness.tsx`: browser-only harness app used by the presence suite.
- `supportE2E.ts`: shared Playwright fixtures, API mocks, and websocket controller setup for support tests.

## Rules

- Keep support e2e deterministic. Do not depend on a real backend, database, or external websocket server in these tests.
- Prefer mocking browser transport at the page boundary instead of adding test-only branches in production React code.
- If you add a new support query in the real route, update `supportE2E.ts` so the full-app spec continues to boot cleanly.
- If you add new presence precedence behavior, cover it in `support-presence.spec.ts` before changing UI code.
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

- Add harness-only state cases by extending the query params or controller in `support-presence-harness.tsx`.
- Add full-app cases by reusing `installSupportAppMocks()` from `supportE2E.ts`.
- Keep assertions focused on real user-visible precedence: customer typing, teammate typing, unread state, viewer state, reconnect snapshots.
