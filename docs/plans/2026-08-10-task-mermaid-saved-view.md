# Task Mermaid Saved View Implementation Plan

> Historical design/implementation plan, source-compared on 2026-09-17. The
> [saved rich-text renderer](../../frontend/src/components/pm/RichTextMentionContent.tsx)
> now implements direct `pre > code` detection with case-insensitive language
> classes and passes `textContent` to the diagram component. It also recognizes
> `language-nwdiag` and `language-svg`; ordinary code blocks retain the generic
> rendering path. The [regression test](../../frontend/src/components/pm/__tests__/RichTextMentionContent.test.tsx)
> covers source preservation for all three diagram types using mocked renderers.
> It does not validate actual diagram layout. Unchecked steps and expected failures
> below describe the original plan, not missing implementation or current test
> results. No browser rendering or build was rerun for this documentation review.

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render saved Mermaid blocks in task descriptions as diagrams instead of source code.

**Architecture:** Detect the persisted TipTap Mermaid code-block shape inside the existing read-only rich-text renderer and delegate rendering to the shared `MermaidBlock`. Keep storage and the edit-mode editor unchanged.

**Tech Stack:** React, TypeScript, Vitest, jsdom, TipTap HTML

---

### Task 1: Render saved Mermaid code blocks

**Files:**
- Modify: `frontend/src/components/pm/RichTextMentionContent.tsx`
- Test: `frontend/src/components/pm/__tests__/RichTextMentionContent.test.tsx`

- [ ] **Step 1: Write the failing test**

Mock `MermaidBlock`, render `<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre>` through `RichTextMentionContent`, and assert the Mermaid renderer receives `graph TD\nA-->B` while no raw `pre` remains.

- [ ] **Step 2: Run the test to verify it fails**

Run: `pnpm --dir frontend exec vitest run src/components/pm/__tests__/RichTextMentionContent.test.tsx`

Expected: FAIL because the saved view renders a raw `pre` and never mounts `MermaidBlock`.

- [ ] **Step 3: Implement the minimal renderer bridge**

Import `MermaidBlock`. In `renderNode`, detect a `pre` with a direct `code` child whose class list contains `language-mermaid`, and return `<MermaidBlock source={code.textContent ?? ''} />` in a stable wrapper.

- [ ] **Step 4: Verify the focused tests**

Run: `pnpm --dir frontend exec vitest run src/components/pm/__tests__/RichTextMentionContent.test.tsx src/components/ui/__tests__/tiptapEditorMermaid.test.ts`

Expected: PASS.

- [ ] **Step 5: Verify build and diff hygiene**

Run: `pnpm --dir frontend build`

Run: `git diff --check`

Expected: both commands exit successfully.

- [ ] **Step 6: Commit**

```bash
git add docs/specs/2026-08-10-task-mermaid-saved-view-design.md docs/plans/2026-08-10-task-mermaid-saved-view.md frontend/src/components/pm/RichTextMentionContent.tsx frontend/src/components/pm/__tests__/RichTextMentionContent.test.tsx
git commit -m "fix(pm): render saved Mermaid task diagrams"
```
