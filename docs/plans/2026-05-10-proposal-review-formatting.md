# Docs Change Proposal Review — Preserve Formatting in Diff

> Source review, 2026-09-17

Historical May proposal-review analysis. The current
[review component](../../frontend/src/components/docs/proposals/ProposalReviewView.tsx)
uses `buildMarkdownDiff` and prefers the saved `base_markdown`, falling back to
plain text for older proposals. The described unconditional plain-text baseline
and `buildSimpleMarkdownDiff` call are obsolete. Worker-tool paths below are also
historical; consult the [tool guide](../internal-tools-framework.md). This check
does not assert lossless rendering of every rich-text construct.

**Date:** 2026-05-10
**Owner:** Follow-up to docs change proposal review UI
**Status:** Plan, not yet implemented
**Related work:**
- `4de980e7` — Add Docs change proposal review experience (initial UI)
- `2026-05-10-docs-change-proposal-review-ui.md` — original plan

---

## 1. Why this matters

The apply path already preserves TipTap marks byte-for-byte: `proposal.Content` (jsonb) is written straight to `docs_contents.content` with no markdown round-trip (`service/docs_content.go:50–62` → `repository/docs_content.go upsertTx`). After apply, the editor re-renders from the cached JSON and **all formatting is intact**.

The review pane is the gap. `frontend/src/components/docs/proposals/ProposalReviewView.tsx:69` calls `buildSimpleMarkdownDiff(currentText, proposedText)` where:
- `currentText` is `content.content_text` — plain text already stripped of formatting
- `proposedText` is `proposal.content_markdown` — raw markdown source

The result: reviewers see `**bold**`, `## Heading`, `- item`, `[label](url)` literally instead of rendered. They are reviewing the right *content* but not the right *presentation*. For a documentation tool, that breaks the "looks like Google Docs Suggesting" goal stated in the original plan.

This is preview-only — no data is lost on apply — but it makes reviewers approve blind for any change that hinges on formatting (table conversion, heading restructure, code-block addition, list reformatting).

---

## 2. Goals

- Reviewer sees the proposed document with full TipTap formatting (bold, italic, code, links, headings, lists, code blocks, tables).
- Changes between current and proposed are visually distinguishable without forcing a structural-diff library if the simple path is enough.
- Block-scope proposals show only the affected block, not the whole doc.
- No regression on data fidelity — apply path is already correct and stays correct.
- No new heavy deps unless they're earning their keep.

## 3. Non-goals

- Inline accept-per-mark or accept-per-paragraph (still atomic apply).
- Server-side diff computation.
- Real-time collaborative diff.
- Replacing the existing review header / source chips / actions.

---

## 4. Approach options

### Option A — Side-by-side rendered TipTap (simple)

Two read-only TipTap views in the review pane:
- **Current**: render `content.content` (the doc's TipTap JSON)
- **Proposed**: render `proposal.content` (the TipTap JSON we already store)

Reviewer scans for differences. No diff highlighting, no new deps, ~80 LOC.

**Pros:** trivial to ship, formatting is faithful, reuses the existing read-only TipTap renderer used elsewhere in the app.
**Cons:** for long docs the reviewer has to spot the change themselves. Acceptable for `scope='block'` (block is small) and proposals with `summary` text already explaining the intent. Marginal for whole-document scope on long articles.

### Option B — TipTap with `prosemirror-changeset` decorations (rich)

Use [`prosemirror-changeset`](https://github.com/ProseMirror/prosemirror-changeset) to compute step-level changes between current and proposed docs. Render in a single TipTap view with decorations: red strikethrough for deletions, green underline for insertions, marks intact within both states.

**Pros:** reviewer sees exactly what changes inline. Matches the Google Docs Suggesting feel. Handles long docs well.
**Cons:** ~20kb gzipped dep. Schema must match between editor and changeset (both are ProseMirror so this is fine). ~250 LOC including the wrapper component. Some integration work for marks rendering.

### Option C — Side-by-side TipTap + per-top-level-node "changed" badge

Option A plus: walk the top-level nodes of both docs (matching by `block.id` if present, otherwise positional) and tag each as `unchanged | changed | added | removed`. Show a small chip next to each node in the proposed pane.

**Pros:** no diff library, points reviewer to the actually-changed paragraphs/headings without computing inline diffs.
**Cons:** matching-by-position breaks for reorder-heavy proposals. Block-id matching only works if Quill emits stable block IDs, which it should via the existing `block_id` field on `DocsChangeProposal` for block-scope proposals but not necessarily for document-scope.

---

## 5. Recommendation

**Phase 1: Option A.** Ship side-by-side rendered TipTap to fix the formatting-preview problem. ~80 LOC, no new deps, addresses the immediate "I can't see formatting" complaint. Most proposals are short or block-scope where side-by-side is genuinely sufficient.

**Phase 2 (only if reviewers ask for it):** Option B with `prosemirror-changeset`. Justified once we have signal that side-by-side isn't enough for long-doc whole-document proposals. Add the dep then, not preemptively. This is the deferred work originally listed as Phase 4 in the parent plan; the deferral was correct — bring it back when there is a real reviewer pain point that side-by-side does not solve.

**Skip Option C** unless someone makes a concrete case for it. It splits the difference but adds matching logic that breaks on edits Quill is likely to make (rewrites, reorders).

---

## 6. Phase 1 implementation sketch

### Files to touch

- `frontend/src/components/docs/proposals/ProposalReviewView.tsx` — replace `buildSimpleMarkdownDiff` call with two `<ProposalContentView>` panes.
- `frontend/src/components/docs/proposals/ProposalContentView.tsx` — **new**, ~40 LOC. Read-only TipTap renderer that takes a TipTap JSON `content` prop and renders it with the same extensions used by the editor. Re-uses whatever `DocsEditor` factors out (or extracts a small helper if needed).
- For block-scope proposals: when `proposal.scope === 'block'`, render only the matching block from current content (lookup by `proposal.block_id`) plus the proposal's content. Otherwise render full docs.

### Type changes

None. `proposal.content` (jsonb) is already on `DocsChangeProposal` and already returned by the single-proposal endpoint.

### Backend

None. The data is already shipped to the client.

### Data needs

`DocsChangeProposal.Content` is currently `json.RawMessage` and includes the full TipTap JSON for `scope='document'` proposals. Confirm Quill writes the **full updated doc body** for document-scope (not just the new section). If Quill writes only the diff fragment, the side-by-side view will show a tiny proposed pane against a full current pane — misleading. Audit `worker/tools_docs.go publish_document_change_proposal` and the system prompt to confirm. If Quill writes fragments, fix the writer (or document the contract that `content` must be the full updated doc).

### Tests

- Unit test: `ProposalContentView` renders bold, italic, code, headings, lists, links, code blocks given representative TipTap JSON.
- Integration test: open a proposal where the current doc has formatting, confirm both panes render with formatting (snapshot test on the rendered HTML is sufficient).
- Block-scope proposal: confirm only the targeted block renders in the current pane.
- Manual smoke: re-run the seed script with one proposal containing all common marks (bold, italic, code, h2, bullet list, link), confirm visual fidelity.

### Cleanup / migration

- Remove `buildSimpleMarkdownDiff` and any markdown-string-diff helpers from the proposal review path.
- Keep `proposal.content_markdown` on the model — it is still useful as the raw text for the agent run history and notification preview. Don't touch the schema.

---

## 7. Risks and open questions

- **Quill content contract**: confirmed above as needing a check. If Quill emits fragments not full docs for document-scope proposals, Phase 1 either misleads the reviewer or needs a small "merge fragment with current doc" step. Audit before starting Phase 1.
- **Editor extension coupling**: the read-only renderer must use the same TipTap extensions as the editor or marks will be silently dropped at render time. Look for a shared extension list in the existing editor module; if there isn't one, factor it out before duplicating.
- **Long docs**: side-by-side view may overflow on narrow viewports. Existing review surface already constrains width — confirm the two-column layout works at typical reviewer widths (1280px+) and falls back to stacked on smaller screens.

## 8. Out of scope (do later if needed)

- `prosemirror-changeset` integration (Phase 2 trigger: real reviewer feedback that A is not enough).
- Inline comments on a proposal.
- Resolving multiple proposals in batch.
- Diff for source chips (showing which sources are net-new vs carried forward).
