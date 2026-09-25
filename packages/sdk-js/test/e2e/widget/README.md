# Widget Browser Coverage

Widget browser coverage is organized by environment layer.

## Directories

- `mock/`
  Default deterministic widget suite. Uses browser-level fetch and websocket mocks.
- `live/`
  Reserved for later backend-backed smoke tests.

## What belongs in `mock/`

- session create/restore behavior
- reconnect behavior
- unread state
- typing indicators
- links and link previews
- transcript requests
- attachment flows

## What belongs in `live/`

- one or two narrow smoke paths proving the real widget contract still works against a live backend

## Upload regression suite

From the repository root:

```sh
pnpm install --frozen-lockfile
pnpm --dir packages/sdk-js exec playwright install --with-deps chromium firefox
pnpm --dir packages/sdk-js run test:e2e:uploads
pnpm --dir packages/sdk-js exec vitest run test/unit/transport/attachment-upload.test.ts
pnpm --dir packages/widget-core exec vitest run src/__tests__/attachmentUpload.test.tsx
```

`uploads.spec.ts` exercises the built SDK and real browser XHR in Chromium and
Firefox. API/WebSocket responses and storage outcomes are deterministic mocks;
no production credentials, customer data, or external storage are used. The
suite starts its own server on port 3017 and refuses to reuse an unrelated app.
CI runs both browsers for package changes.

Coverage includes real PNG bytes and Unicode/spaced filenames, a 4.9 MB PDF,
private PUT headers (no public ACL or session credential leakage), storage 403,
network failure, stalled transfer, cancellation, confirmation failure, delayed
session connection, retained drafts, blocked sending, and successful retries.
Unit tests additionally cover all three cancellation stages, inactivity resets,
overall deadlines, malformed responses, unknown MIME, empty/oversize files,
batch failure isolation, conversation changes, and widget teardown.

Browser network failures simulate the observable result of a blocked request;
they do not validate a real bucket's CORS configuration or reproduce every
Firefox/macOS networking condition. Live deployment/storage checks remain a
separate opt-in smoke test concern. Screenshots and traces are retained locally
on browser test failures.
