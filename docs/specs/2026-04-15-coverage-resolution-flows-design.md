# Coverage resolution flows design

This historical design describes the original coverage-gap suggestion workflow.
Use it as context for the retained legacy draft/apply path; current coverage also
has V2 topic/finding/signal workflows, so this is not the complete product contract.

## Source review — 2026-09-18

- [Draft generation](../../server/internal/service/support_coverage_drafts.go) still converts generated Markdown to TipTap and creates a draft suggestion after a successful model response. [CreateSuggestion](../../server/internal/repository/support_coverage.go) inserts that suggestion without changing the gap to `drafted`; the original transition table is obsolete.
- Apply creates a document or appends suggestion nodes with [AppendContent](../../server/internal/tiptap/append.go), then updates the suggestion, marks an **open** gap `done`, and links the article. `fixed`/`drafted` remain deprecated model constants. Draft preparation is not evidence that a published help article now resolves the customer issue.
- These apply operations are sequential service/repository writes, not one transaction or a claimed-idempotent operation. A content write can succeed before a later suggestion/gap/link failure. The update path attempts a snapshot but logs and continues if it fails; it has no content-version compare-and-swap in this method. Do not infer atomic application or guaranteed protection of concurrent edits from append-only node merging.
- Discard transactionally rejects the suggestion and sets the gap to `open`, but the repository method does not first require a draft suggestion or currently-open gap. The proposed preview-only discard flow is narrower than that backend operation.
- [Routes](../../server/internal/router/router.go) require both support-edit and docs-edit for generation/apply, and support-edit for discard. [The retained detail pane](../../frontend/src/components/support/coverage/GapDetailPane.tsx) uses current `/docs/documents/{id}` links, generation controls, and suggestion previews. [The coverage page](../../frontend/src/pages/support/coverage/SupportCoveragePage.tsx) additionally exposes V2 flows and documentation-agent investigation.
- The original approved status records the design decision, not completion of every requirement. No model calls, document writes, live publication checks, or end-to-end acceptance tests were performed in this review. JSON-mode configuration also does not remove the need to validate model output.

## Original design

**Status:** Approved  
**Date:** 2026-04-15  
**Area:** Support Coverage Module  

---

## 1. Problem

The coverage detection system (V1) identifies docs gaps from AI failures, but the detail panel is a dead end. Users see evidence of what failed but have no path to fix it. The "Generate Draft" and "Suggest Improvements" capabilities exist in the backend but have no UI. The result is a gap dashboard that reports problems without enabling solutions.

## 2. Goal

Transform the gap detail panel into a **resolution workspace** where users can:
1. Understand the gap (evidence, linked article, confidence)
2. Generate a fix (LLM-powered draft or improvement suggestions)
3. Preview the fix inline
4. Apply the fix to the docs system
5. Continue editing in the docs editor

All without leaving the coverage module.

---

## 3. Detail Panel Layout

Top-to-bottom sections:

### 3.1 Header
- Gap title
- Gap type badge (color-coded: red=missing, amber=weak, orange=outdated, blue=needs_review)
- Confidence label: "High confidence" (green, ≥0.7), "Medium confidence" (amber, 0.4–0.69), "Low confidence" (muted, <0.4)
- Status badge (right side)
- Close button (X icon with hover)

### 3.2 Metadata
- **Topic**: issue_key displayed as clean title case (underscores→spaces)
- **First seen**: relative time
- **Evidence**: count of conversations
- **Status change info** (when status ≠ open): "Marked as [status] by [name] [timeAgo]"
  - For `human_only` status: also shows "Customer issue resolved/unresolved"

### 3.3 Linked Article (weak/outdated article gaps only)
- Displayed when the gap has a `related_article` with a resolved document title
- Uses the **first** related article entry when multiple exist
- Shows: article title + "Open in Editor ↗" link (opens `/w/{slug}/docs/{documentId}` in new tab)
- Not shown for missing article or needs_review gaps
- If a weak/outdated gap has zero related articles, fall back to needs_review behavior (no linked article section, no generate button — user must triage manually)

### 3.4 Evidence
- Scrollable list of evidence items
- Each item shows:
  - **Evidence type** as human-readable label (not raw enum). Mapping uses the model constants (`model.SupportEventAIHandoffTriggered`, etc.):
    - `ai_handoff_triggered` → "AI Handoff"
    - `article_feedback_submitted` → "Article Feedback"
    - `widget_search_performed` → "Widget Search"
    - `docs_issue_feedback` → "Agent Feedback"
    - `human_reply_after_ai` → "Human Reply"
  - Relative timestamp
  - Excerpt text
  - **Links** (when IDs exist):
    - "View conversation ↗" when `conversation_id` is present → `/w/{slug}/support/inbox?conversation={conversationId}`
    - "View article ↗" when `document_id` is present → `/w/{slug}/docs/{documentId}`
    - Both open in new tabs

### 3.5 Suggestion Preview (when generated)

**For weak/outdated articles (additive sections):**
```
┌ Suggested additions ──────────────────────────┐
│ New section: "Okta SAML Configuration"        │
│ > Steps for configuring SAML metadata URL...  │
│                                               │
│ New section: "Troubleshooting SSO Errors"     │
│ > Check certificate expiry dates...           │
│                                               │
│ [Discard]                [Apply to Article]   │
└───────────────────────────────────────────────┘
```

**For missing articles (full draft):**
```
┌ Draft: Setting up SAML SSO ───────────────────┐
│ Introduction: This guide covers...            │
│ Section: Prerequisites...                     │
│ Section: Configuration steps...               │
│                                               │
│ [Discard]            [Create as Draft in Docs]│
└───────────────────────────────────────────────┘
```

**Error state (LLM failure):**
```
┌ Generation failed ────────────────────────────┐
│ Could not generate suggestions. This may be   │
│ due to a temporary service issue.             │
│                                               │
│ [Try Again]                                   │
└───────────────────────────────────────────────┘
```

A failed LLM call returns an error before any suggestion is created — the gap remains in its current state with no DB side effects.

### 3.6 Generate Actions (when no suggestion exists)

**Weak/outdated article gaps (with linked article):**
- "Suggest Improvements" button
- Clicking triggers LLM generation, shows loading state, then reveals suggestion preview (3.5)
- The `target_document_id` is derived from the first entry in `related_articles`

**Missing article gaps:**
- Inline space dropdown + collection dropdown (fetched from existing docs API)
- "Draft New Article" button (disabled until space is selected)
- Clicking triggers LLM generation, shows loading state, then reveals suggestion preview (3.5)

**Needs review gaps:**
- No generate button
- User must reclassify to missing/weak to unlock generation

**Weak/outdated gaps with zero related articles:**
- No generate button (same as needs_review behavior)
- User can reclassify or manually fix

**Permission gating:**
- Generate/apply buttons require both `support.edit` and `docs.edit` permissions
- Hide buttons when user lacks either permission (don't show buttons that will 403)

### 3.7 Confirmation Dialog

Inline confirmation (not a full modal) shown when user clicks Apply/Create:

**Weak/outdated article:**
> This will add [N] new sections to "[Article Title]".
> The article will remain unpublished.
> [Cancel] [Apply Changes]

**Missing article:**
> This will create a new draft article "[Title]" in [Space] > [Collection].
> [Cancel] [Create Draft]

### 3.8 Post-Apply State

After successful apply, the suggestion section transforms:

**Weak/outdated article:**
> ✓ [N] sections added to "[Article Title]"
> [Open in Editor ↗]

**Missing article:**
> ✓ Draft created: "[Article Title]"
> in [Space] > [Collection]
> [Open in Editor ↗]

Gap status badge updates to **"Fixed"**. Generate buttons disappear. "Open in Editor" link persists.

**Status transitions:**
- Generate → gap status becomes `drafted` (suggestion exists, not yet applied)
- Apply → gap status becomes `fixed` (content created/updated in docs)
- Discard → gap status reverts to `open` (suggestion rejected)

### 3.9 Discard Flow

"Discard" button sets the suggestion status to `rejected` via a new backend endpoint and reverts the gap status to `open`. The suggestion preview disappears and the generate button reappears, allowing the user to regenerate or take a different action.

**New backend endpoint:** `POST /coverage/suggestions/{suggestionId}/discard`
- Sets suggestion status to `rejected`
- Reverts gap status to `open`

### 3.10 Status Actions Bar
- Status badge (left)
- Spacer
- Action buttons (right), contextual:
  - **Open**: [Ignore] [Requires Human (with tooltip)] [Mark Fixed]
  - **Ignored/Human Only/Fixed**: [Reopen]

---

## 4. Flows by Gap Type

### 4.1 Weak Article / Outdated or Conflicting Article
1. Panel shows linked article title + "Open in Editor ↗" (first related article)
2. Evidence shows what customers asked that the article didn't cover
3. User clicks "Suggest Improvements"
4. LLM generates additive sections only (not a rewrite)
5. Preview appears in panel (or error state with retry)
6. Gap status → `drafted`
7. User clicks "Apply to Article" → inline confirmation
8. Sections **appended** to existing article content (existing content untouched)
9. Gap status → `fixed`
10. Success state with "Open in Editor ↗" to review and publish

### 4.2 Missing Article
1. Panel shows no linked article
2. Evidence shows customer questions + human agent answers
3. User selects target space + optional collection from inline dropdowns
4. User clicks "Draft New Article"
5. LLM generates full article content
6. Preview appears in panel (or error state with retry)
7. Gap status → `drafted`
8. User clicks "Create as Draft in Docs" → inline confirmation
9. Unpublished draft document created in target space/collection
10. Gap status → `fixed`
11. Success state with "Open in Editor ↗"

### 4.3 Needs Review
1. Panel shows evidence only
2. No generate actions available
3. User triages: Ignore, Requires Human, Mark Fixed, or reclassify gap type
4. Reclassifying to missing/weak unlocks the corresponding generate flow

---

## 5. Backend Changes

### 5.1 Append-Based Content Update (new deliverable)

New function in `server/internal/tiptap/` package:

```go
// AppendContent merges additional TipTap nodes into an existing document.
// It parses both JSON documents, appends the new content nodes after the
// existing ones, and returns the merged JSON. Existing content (formatting,
// images, callouts, code blocks, embeds) is fully preserved.
func AppendContent(existing, additions json.RawMessage) (json.RawMessage, error)
```

**Contract:**
- Both inputs are valid TipTap JSON with `{ "type": "doc", "content": [...] }` structure
- Returns a new document with `existing.content` + `additions.content` concatenated
- If `existing` is nil/empty, returns `additions` as-is
- If `additions` is nil/empty, returns `existing` as-is
- Must have test coverage: verify existing images, callouts, code blocks survive the merge

**Integration:** `applyUpdateArticle` in `support_coverage_drafts.go` changes from:
```go
// Before: full replacement with pre-built TipTap JSON
s.contentSvc.Save(ctx, docID, suggestion.Content, userID)

// After: convert Markdown to TipTap, append to existing, save merged
existing, _ := s.contentSvc.Get(ctx, docID)
newNodes := tiptap.MarkdownToJSON(suggestion.MarkdownContent)
merged, err := tiptap.AppendContent(existing.Content, newNodes)
s.contentSvc.Save(ctx, docID, merged, userID)
```

For missing articles, `applyCreateArticle` converts Markdown to TipTap and saves directly (no append needed).

### 5.2 LLM Output Format: Markdown (replaces JSON sections)

**Current approach (being replaced):** LLM outputs structured JSON `{ sections: [{ heading, paragraphs[] }] }`, then `buildTipTapJSON` manually constructs TipTap nodes. This produces only headings + paragraphs — no lists, code blocks, bold, links, etc.

**New approach:** LLM outputs standard **Markdown**. The existing `tiptap.MarkdownToJSON` function converts it to TipTap JSON with full formatting support (headers, lists, code blocks, bold/italic, links, blockquotes, etc.).

**LLM response format:**
```json
{
  "title": "Article title",
  "content": "## Section heading\n\nParagraph text with **bold** and [links](...).\n\n- List item 1\n- List item 2\n\n```code block```"
}
```

- `title`: used for the document title (missing article) or display only (weak article)
- `content`: Markdown string, converted via `tiptap.MarkdownToJSON`

**Prompt changes:**
- **Weak/outdated article**: "Generate ONLY new sections in Markdown that address these unanswered customer questions. Do not rewrite existing content. Output sections that will be appended to the existing article."
- **Missing article**: "Write a complete help center article in Markdown."
- Both prompts request JSON with `title` and `content` fields. `JSONMode: true` ensures parseable response.

**Removes:** `buildTipTapJSON`, `parseDraftResponse`, the `sections[]` output schema. Replaced by `json.Unmarshal` + `tiptap.MarkdownToJSON`.

**V1 scope:** Additive only (append new sections). In-place content modification is a V2 concern.

### 5.3 Gap Detail DTO Enhancement
- Enrich `GetGapDetail` in the repository: join `support_coverage_gap_articles` with `docs_documents` table to resolve article title
- Update `SupportCoverageGapArticle` response to include `article_title string`
- Update both Go DTO (`model.SupportCoverageGapArticle`) and TypeScript interface to include `article_title`

### 5.4 Discard Suggestion Endpoint
- New endpoint: `POST /coverage/suggestions/{suggestionId}/discard`
- Sets suggestion status to `rejected` (`model.SupportCoverageSuggestionStatusRejected`)
- Reverts gap status to `open`
- Requires `support.edit` permission

### 5.5 Evidence Type Labels
- Mapped on the frontend (no backend change)
- Raw enum → human-readable display label
- Mapping verified against `model.SupportEvent*` constants

### 5.6 Confidence Display
- Mapped on the frontend (no backend change)
- Numeric value → "High" / "Medium" / "Low" label with color

### 5.7 Topic Display
- issue_key formatted on the frontend: replace `_` and `-` with spaces, title case
- Label changed from "Issue" to "Topic"

---

## 6. What Is NOT Changing

- No new database tables or migrations
- No changes to gap detection rules or event recording
- No changes to the gap list view or summary cards
- No diff/merge UI (V2 consideration)
- No inline editing of suggestions before apply (user edits in docs editor after apply)
- No semantic clustering or LLM-based gap classification

---

## 7. Edge Cases

| Scenario | Behavior |
|----------|----------|
| LLM call fails (timeout, provider down) | Error state with "Try Again" button. No DB state created. |
| Article deleted after gap was created | "Open in Editor" link shows 404. User can reclassify gap. |
| Weak gap with zero related articles | Falls back to needs_review behavior (no generate button). |
| Weak gap with multiple related articles | Uses first related article. |
| User lacks `docs.edit` permission | Generate/apply buttons hidden. Evidence and triage still available. |
| Only one external space exists | Space dropdown pre-selects it. |
| No spaces exist | "Draft New Article" button disabled with tooltip "Create a docs space first". |

---

## 8. Dependencies

- Existing docs API for space/collection listing (already available)
- LLM provider configured for draft generation (already wired)
- New `tiptap.AppendContent` function (new deliverable)
- New discard suggestion endpoint (new deliverable)
- Article title resolution in gap detail query (enhancement to existing query)
