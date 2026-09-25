# SDK Browser E2E

This directory contains Playwright coverage for the SDK and widget browser runtime.

## Structure

- root specs in this folder cover existing SDK/browser compatibility cases
- `widget/mock/` contains deterministic widget browser tests with mocked transport
- `widget/live/` is reserved for backend-backed widget smoke tests

## Principles

- Keep the default widget browser suite deterministic and transport-mocked.
- Use backend-backed widget tests only as a thin opt-in smoke layer.
- Prefer assertions on visible widget behavior first, then transport details second.

## Run widget tests

From the repository root:

```bash
pnpm install --frozen-lockfile
pnpm --dir packages/sdk-js run test:e2e:widget:install
pnpm --dir packages/sdk-js run test:e2e:widget
```

The package script builds the SDK before starting Playwright. Its Chromium suite
serves local assets on port 3017 and excludes `widget/live/`. Calling Playwright
directly skips that build, so it can use missing or stale assets. See the
[configuration](../../playwright.widget.config.ts) for browser projects and server
settings. The broader `test:e2e` script uses a separate configuration.
