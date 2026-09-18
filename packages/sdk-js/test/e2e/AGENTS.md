# SDK Widget E2E

This directory holds browser-level Playwright coverage for the SDK and widget runtime.

## Scope

- root `test/e2e/` specs: existing SDK/browser compatibility cases that are not widget-conversation specific
- `widget/mock/`: deterministic widget browser tests with mocked fetch and mocked websocket transport
- `widget/live/`: reserved for backend-backed widget smoke tests

## Directory Decision

This is the canonical widget browser-test layout moving forward.

- Put broad widget behavior coverage in `widget/mock/`.
- Put backend-backed widget smoke only in `widget/live/`.
- Keep generic SDK/browser compatibility specs at the `test/e2e/` root when they are not widget-chat specific.

Do not mix mocked widget coverage and live widget coverage in the same directory.

## Rules

- Keep the default widget browser suite deterministic.
- Prefer browser-boundary mocks instead of test-only branches in widget runtime code.
- Add new widget conversation/session/link/attachment tests in `widget/mock/` first.
- Keep `widget/live/` opt-in and thin even after it is implemented.
- Use Chromium as the required browser unless there is a specific cross-browser concern being targeted.

## Commands

Run from the repository root:

```bash
pnpm install --frozen-lockfile
pnpm --dir packages/sdk-js exec playwright install chromium
pnpm --dir packages/sdk-js run test:e2e:widget
```

## Extension Points

- Extend `widget/mock/widgetE2E.ts` for new mocked transport cases.
- Add new deterministic widget runtime cases in `widget/mock/widget.spec.ts`.
- Add backend-backed smoke tests under `widget/live/` only after deterministic bootstrap exists.
