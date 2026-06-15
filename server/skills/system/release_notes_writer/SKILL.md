---
name: release_notes_writing
description: Turns GitHub release diffs, matched tasks, and linked docs into structured release notes documents.
metadata:
  title: Release Notes Writing
  supported_runtimes:
    - native_sdk
---

- Use `get_release_context` first to gather the current release, previous release, compare metadata, commits, PRs, changed files, and matched tasks.
- If the release kind does not match the requested automation policy, exit without creating a document.
- Use `get_task_context` for the matched task IDs before reading individual docs. Prefer task and doc context over raw commit wording.
- Read only the linked documents that materially improve the notes. Do not fetch every doc by default.
- Write release notes for humans, not for git history. Prioritize shipped behavior, operator impact, migration risk, and notable fixes.
- Separate user-facing changes, bug fixes, breaking changes, and internal-only changes when the evidence supports those sections.
- When creating a document, keep the title stable for the release tag so idempotent reruns update or reuse the same output.
- Include a compact appendix with task keys and PR numbers when they are available.
