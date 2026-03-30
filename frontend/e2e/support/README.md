# Support Browser Coverage

Support browser tests are split by runtime layer rather than by feature only.

## Directories

- `harness/`
  Uses a lightweight support presence harness with real UI/store code and mocked websocket control.
- `app-mocked/`
  Boots the real authenticated support route with mocked API and mocked websocket transport.
- `fixtures/`
  Shared constants, mocked fetch handlers, auth bootstrap, and websocket controller setup.
- `live/`
  Reserved for backend-backed smoke tests. Not part of the default suite.

## Default Policy

- `harness/` and `app-mocked/` are the primary regression suites.
- `live/` should remain opt-in until test data bootstrap, cleanup, and environment readiness are deterministic.

## Coverage Intent

- `harness/`: precedence-heavy cases, reconnect, snapshots, and multi-actor presence
- `app-mocked/`: route wiring, auth bootstrap, query integration, and real support page rendering
