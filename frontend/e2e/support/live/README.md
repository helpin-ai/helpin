# Support Live Browser Smoke

This directory is reserved for backend-backed support browser tests.

## Purpose

These tests should prove a small number of high-value live integration paths:

- authenticated support route boot
- real websocket presence between agents
- reconnect and resync against a live backend

## Rules

- Do not add broad or flaky coverage here.
- Keep this suite opt-in until bootstrap and cleanup are deterministic.
- Use uniquely namespaced users, workspaces, and conversations.
- Prefer user-visible assertions over exact internal frame sequences.

## Prerequisites For Activation

- deterministic auth/bootstrap helper
- isolated seeded data
- API/websocket/worker readiness checks
- clear command and environment documentation
