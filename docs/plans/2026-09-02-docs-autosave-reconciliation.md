# Docs Autosave Reconciliation Implementation Plan

> Historical implementation plan, source-compared on 2026-09-17. The reconciliation
> is implemented in [DocsEditor](../../frontend/src/components/docs/DocsEditor.tsx).
> Save snapshots use stable JSON serialization (sorted object keys), are registered
> before `onSave`, removed on failure, and consumed when matching `initialContent`
> arrives. A match updates bookkeeping without replacing newer editor content.
> An unrelated incoming document still replaces content and clears the save timer
> and queued snapshot. Saves are serialized with the newest pending content.
>
> This is snapshot acknowledgement, not collaborative conflict merging or a
> server revision protocol. Matching is by content rather than a request ID.
> The [tests](../../frontend/src/components/docs/__tests__/DocsEditor.test.tsx)
> cover an older acknowledgement preserving newer edits and an external replacement.
> Checkboxes and the instruction to leave changes uncommitted below describe the
> original session, not current worktree or fresh build/test results.

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent successful autosave responses from restoring text deleted after the saved snapshot was submitted.

**Architecture:** `DocsEditor` will track content snapshots submitted by its own save pipeline. A matching `initialContent` update is an acknowledgement and updates bookkeeping without replacing the live editor document; a non-matching update remains an authoritative external replacement.

**Tech Stack:** React, Tiptap, TanStack Query, Vitest

---

### Task 1: Protect newer local edits

**Files:**
- Modify: `frontend/src/components/docs/DocsEditor.tsx`
- Test: `frontend/src/components/docs/__tests__/DocsEditor.test.tsx`

- [x] Add a failing test that submits one snapshot, makes a newer local edit, and rerenders with the submitted snapshot as `initialContent`; assert that `setContent` is not called.
- [x] Add a failing/guard test that rerenders with unrelated external content; assert that `setContent` is still called.
- [x] Track submitted snapshots, remove failed submissions, and consume matching incoming acknowledgements before the external-content replacement path.
- [x] Run the focused Docs editor tests and TypeScript build.
- [x] Confirm `git diff --check` and leave the implementation uncommitted for review.
