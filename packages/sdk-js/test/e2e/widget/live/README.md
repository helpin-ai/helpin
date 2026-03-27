# Widget Live Browser Smoke

This directory is reserved for backend-backed widget browser tests.

## Purpose

Later, this suite should prove a narrow live contract:

- widget boot against a live support backend
- session create or restore
- real inbound message delivery
- one representative realtime flow

## Rules

- Keep the suite opt-in and thin.
- Do not duplicate the full mocked matrix here.
- Require deterministic bootstrap and isolated data before adding cases.
