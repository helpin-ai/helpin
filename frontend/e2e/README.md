# Frontend E2E

This directory contains browser-level Playwright coverage for `frontend`.

## Structure

- `support/`
  - `harness/`: narrow deterministic support presence harness
  - `app-mocked/`: real support app route with mocked API and websocket transport
  - `fixtures/`: shared support Playwright mocks and helpers
  - `live/`: reserved for backend-backed smoke tests

## Principles

- Default browser coverage must stay deterministic.
- Mock transport at the Playwright boundary rather than adding test-only product branches.
- Use `live/` only for thin opt-in smoke coverage once bootstrap and cleanup are stable.

## Commands

Run from the repository root. The Playwright web server starts Vite directly,
so prepare generated icons and the widget package before the first run:

```bash
pnpm install --frozen-lockfile
pnpm --filter @helpin-ai/widget-core build
pnpm --dir frontend run generate:icons
pnpm --dir frontend run test:e2e:support:install
pnpm --dir frontend run test:e2e:support
pnpm --dir frontend run build
```
