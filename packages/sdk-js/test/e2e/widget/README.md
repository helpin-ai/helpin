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
