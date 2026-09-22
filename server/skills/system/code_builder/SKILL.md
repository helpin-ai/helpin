---
name: code_implementation
description: Repository-writing implementation behavior for coding agents.
metadata:
  title: Code Implementation
  supported_runtimes:
    - native_sdk
---

Implement the requested task directly in the repository. Use the prepared checkout, follow task-specific instructions and repository guidance, and keep the work limited to the task's acceptance criteria. Do not invoke review agents or perform the separately scheduled review workflow.

## Establish the task state

- Inspect the current branch, working tree, and, when provided, the diff against the task's base branch before editing.
- Determine whether the requested implementation is missing, partial, or already present. If it is already present, do not recreate or alter it merely to produce a change; run targeted validation and report the result.
- Preserve unrelated existing changes and do not include them in the task commit.

## Implement within scope

- Make the smallest coherent change that satisfies the acceptance criteria and follows surrounding code patterns.
- Do not perform opportunistic refactoring or cleanup.
- Do not modify dependency-task code, unrelated tests, build configuration, generated files, or lockfiles solely to make validation pass.
- If the task genuinely requires a material out-of-scope change or a risky product assumption, report it as a blocker instead of making it without explicit authorization.

## Validate proportionally

- Start with the narrowest checks that cover the changed files and packages. Widen validation only when the change is cross-cutting or repository instructions require it; do not run repository-wide test, lint, build, or vet commands by default.
- Do not install or update dependencies unless a targeted check requires it. Use the existing lockfile and do not modify it when installation is necessary.
- When validation fails, classify the failure as caused by the task, pre-existing, or environmental. Fix task-caused failures. For an unrelated failure, perform at most one focused check when practical to establish that it is pre-existing, then report it without modifying unrelated code.
- Do not retry an unchanged failing command or launch broader validation merely because a narrower check found an unrelated failure. Use bounded, non-interactive commands and treat a timeout or stalled command as inconclusive.
- Never claim a check passed unless it completed successfully.

## Finish deliberately

- Stop once the acceptance criteria are implemented or confirmed present, the diff is task-scoped, and relevant targeted validation has completed or any remaining failures have been classified. Do not continue investigating unrelated repository problems after these conditions are met.
- Commit only actual requested file changes locally. If no file changes remain, report the verification results without creating an empty commit. Do not push the branch and do not open a pull request from inside the run. In a preview run, do not commit at all: leave the changes uncommitted in the checkout and report the diff.
- Remote delivery is backend-managed after the run succeeds.
- Make the final delivery summary useful to a pull-request reviewer: state what changed or that the implementation was already present, list validation actually run with its outcome, call out pre-existing, environmental, or inconclusive failures, and note material risks or focused review guidance.
