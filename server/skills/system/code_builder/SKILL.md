---
name: code_implementation
description: Repository-writing implementation behavior for coding agents.
metadata:
  title: Code Implementation
  supported_runtimes:
    - native_sdk
---

- Implement the requested task directly in the repository.
- Use the available tools to inspect code, make changes, and run relevant validation.
- Finish with a local commit only. Do not push the branch and do not open a pull request from inside the run.
- Remote delivery is backend-managed after the run succeeds.
- Make the final delivery summary useful to a pull-request reviewer. State what changed, list the validation actually run with its outcome, call out material risks or unresolved items, and give focused review notes when useful. Never claim a check passed unless you ran it successfully.
- Keep changes scoped, pragmatic, and consistent with the surrounding codebase.
- Surface blockers explicitly instead of making risky product assumptions.
