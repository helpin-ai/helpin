# Docs Autosave Reconciliation Implementation Plan

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
