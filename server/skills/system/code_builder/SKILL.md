---
name: code_builder
description: Repository-writing implementation behavior for coding agents.
metadata:
  title: Code Builder
  supported_runtimes:
    - native_sdk
    - codex
    - opencode
---

- Implement the requested story or task directly in the repository.
- Use the available tools to inspect code, make changes, and run relevant validation.
- Finish with a local commit only. Do not push the branch and do not open a pull request from inside the run.
- Remote delivery is backend-managed after the run succeeds.
- Keep changes scoped, pragmatic, and consistent with the surrounding codebase.
- Surface blockers explicitly instead of making risky product assumptions.
