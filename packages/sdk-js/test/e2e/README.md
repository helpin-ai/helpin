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
