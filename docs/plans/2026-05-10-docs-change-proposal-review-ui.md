# Docs Change Proposal Review UI

**Date:** 2026-05-10
**Owner:** Quill end-to-end shipping work
**Status:** Plan, not yet implemented
**Related work:**
- `9d43ef69` — Custom agent simplification + Quill (Documentation Agent)
- `e84976c2` — Snapshot version when applying a Docs change proposal

---

## 1. Why this matters

Quill (the Documentation Agent) can already write `DocsChangeProposal` rows via the `publish_document_change_proposal` tool. The backend is fully wired: the proposal lifecycle (`pending → applied | discarded`), the apply/discard endpoints, and (as of `e84976c2`) automatic version snapshotting on apply.

There is a basic frontend surface today: `DocsDocumentDetail.tsx` lists pending proposals, shows a compact amber banner with the markdown preview, and calls the existing apply/discard mutations. That is enough to avoid a totally invisible queue, but it is not enough for a reviewer to **deep-link to a specific proposal, compare it against the current document, inspect provenance, and make a confident decision**.

This plan upgrades that existing banner/apply/discard path into a shareable review mode for the proposal lifecycle: where it lives, how it renders, and how it integrates with the existing document surface.

---

## 2. Goals

- Reviewers (ops / PM / support, not engineers) can read a Quill-authored change against the current document and decide quickly.
- The comparison feels like a document tool, not a code tool. Closer to Confluence / Google Docs Suggesting than GitHub.
- The proposal's provenance (which agent, which run, which sources) is visible without clicking around.
- Apply and Discard are one click away.
- The review surface is sharable via URL (notifications, teammate handoff).
- Deep links work for pending and already-resolved proposals, so notification links and handoffs do not break after someone acts.
- The gap-to-proposal flow can route reviewers to the created proposal once Quill finishes writing it.

## 3. Non-goals

- Per-block accept/reject inside a single proposal (proposals are atomic — applied wholesale or discarded).
- Inline commenting on a proposal.
- Workspace-wide pending-proposals queue page.
- Loop-close automation that marks a support coverage gap resolved when its proposal applies. The review UI can expose the proposal created from a gap, but it does not change gap-resolution policy.
- Real-time co-review (two reviewers acting on the same proposal at once). Optimistic locking on the backend already prevents double-apply; UI just needs to handle the "already resolved" error gracefully.
- Server-side diff pre-computation. Diff rendering runs client-side.

---

## 4. Backend surface

Endpoints already in place at `server/internal/router/router.go:1081-1083`:

- `GET    /docs/documents/{docId}/change-proposals` — list pending proposals (`DocsChangeProposalService.ListPending`)
- `POST   /docs/documents/{docId}/change-proposals/{proposalId}/apply` — apply (`Apply`); now also snapshots a `VersionTypeProposalApply` / `proposal_apply` row labeled `"Applied: <summary>"`
- `POST   /docs/documents/{docId}/change-proposals/{proposalId}/discard` — discard (`Discard`)

Add one read endpoint for stable deep links:

- `GET    /docs/documents/{docId}/change-proposals/{proposalId}` — return a single proposal by ID, including applied/discarded rows.

Permission gates: `pm.docs.read` for list/get, `pm.docs.edit` for apply/discard.

WebSocket events emitted:
- `docs_change_proposal.created` (when Quill writes one)
- `docs_change_proposal.updated` (when applied/discarded)
- `docs_version.created` (when snapshot lands on apply)

Proposal model carries most of what we need for the UI:
- `ID`, `DocumentID`, `BlockID` (nullable), `Scope` (`document` | `block`), `Revision`, `Summary`, `ContentMarkdown`, `Content` (TipTap JSON), `Sources` (JSONB array), `AgentID`, `AgentRunID`, `CreatedBy`, `CreatedAt`, `UpdatedAt`, `Status`, `ResolvedBy`, `ResolvedAt`.

Required backend refinements:
- Add repository/service/handler coverage for `GetChangeProposal(documentID, proposalID)` so `?proposal={id}` can render even after apply/discard.
- Formalize `DocsChangeProposalSource` instead of leaving `Sources json.RawMessage` fully loose. The UI should receive typed source items shaped like `{ type: "conversation" | "document" | "url" | "agent_run" | "coverage_gap", id?, label, url? }`.
  - Do this with a tolerant server-side decoder, not a data migration. Existing rows may contain loose Quill-written JSON. Unknown or malformed source entries are dropped before serialization and logged at debug level; they must not break proposal review or force each frontend to reimplement fallback parsing.
- Make `proposal_id` in the completed agent run output the authoritative gap→proposal correlation path. `docs_change_proposal.created` remains a nudge to refetch run/proposal state, not a second source of truth.
- Extend `GET /docs/documents` / `DocsDocument` with `pending_change_proposal_count` so the existing space detail tree built from `useDocsDocuments` can render counts without N+1 proposal calls. Invalidate `queryKeys.docs.documents(wsId)` and the specific proposal queries when `docs_change_proposal.created` or `docs_change_proposal.updated` arrives.

---

## 5. Surface and routing

### 5.1 Where the review lives

The review is a **mode of the document detail page**, not a separate app. Same workspace shell (sidebar, doc-tree, breadcrumbs, header) — only the main reading column swaps from "live doc" to "proposal review."

URL pattern: a query param on the existing doc detail route.

```
/w/{slug}/docs/documents/{docId}?proposal={proposalId}
```

The query param is the source of truth: presence of `?proposal=` puts the page into review mode. Browser-back drops the param and returns to the live doc; copying the URL shares the review.

Why not a sub-route like `/docs/{docId}/proposals/{proposalId}`:
- Param-based mode-switching keeps the route tree simple and avoids double-mounting the doc layout.
- Param-based is also how Helpin already handles task drawers (`?task=...`).

Why not a modal or drawer:
- Long-form document comparison needs width and vertical space.
- Modals trap focus; reviewers may want to cross-check related docs in the doc-tree without leaving review mode.
- Cannot be linked to from a notification.

### 5.2 Entry points

Reviewers reach the review mode from:

1. **Gap detail page** — the existing "Run with Quill" CTAs start a Quill run, not a proposal directly. Show the started-run state, then wait for the run output to include `proposal_id` and `document_id`. The `docs_change_proposal.created` WebSocket event only nudges the UI to refetch the run; it is not the authoritative correlation path. Once the run output has both IDs, route to `/w/{slug}/docs/documents/{docId}?proposal={proposalId}`.
2. **Doc detail page banner** — when a document has pending proposals, show a banner above the live doc:
   `[Quill avatar] 1 proposal pending • Review →`
   Click → `?proposal={firstPendingId}`.
3. **Notifications** — when Quill creates a proposal, the existing notification system carries a deep link. The link uses the same URL pattern.
4. **Doc tree/sidebar** — documents with pending proposals get a small count chip. Clicking it opens the document; the doc banner handles the proposal-specific review action.

### 5.3 Mobile

Workspace shell collapses on narrow viewports automatically (existing behavior). Review mode keeps the same single-column layout. On narrow screens, render the unified diff in one column with sticky Apply / Discard actions.

---

## 6. The review surface in detail

### 6.1 Header strip

The trust signal lives at the top of the review column:

```
┌────────────────────────────────────────────────────────────────────┐
│ [Quill] Quill proposed an update · 2 minutes ago · View run        │
│                                                                    │
│ Update billing FAQ to clarify monthly vs annual cycles             │
│                                                                    │
│ Sources: 3 conversations · billing-policy.md · Stripe docs   ▾     │
│                                                                    │
│                                            [Discard]   [Apply]     │
└────────────────────────────────────────────────────────────────────┘
```

Components:
- **Agent identity** — Quill avatar + "Quill proposed" + relative timestamp. (For non-Quill agents, swap avatar/name accordingly. Use existing `AgentAvatar` component.)
- **Summary** — `proposal.Summary`, the one-line description Quill wrote.
- **Sources chip row** — collapsible. Each source becomes a chip that opens the source in a new tab when it has a URL, or routes to the matching in-app record when it has an ID. Sources are typed in the proposal JSON (`type: "conversation" | "document" | "url" | "agent_run" | "coverage_gap"`).
- **Run link** — opens the originating `agent_run` panel/drawer in-page for full investigation; it should not navigate away from proposal review.
- **Apply / Discard buttons** — Apply is the primary; Discard is destructive-styled, asks for confirmation.

### 6.2 Diff body — unified, single-mode

One mode: **unified rich-text diff**. Reasons:

- Document-world convention: Word Track Changes, Google Docs Suggesting, Confluence, and Notion AI all default to inline review.
- Reads as one document, not two.
- Better for prose-heavy changes where most surrounding content is unchanged.
- Mobile-friendly without a separate codepath.

No side-by-side/current-proposed toggle. One mode, simpler implementation, simpler UX.

### 6.3 Color and shape grammar

Use three change categories. Color tells you *what kind* of change. A redundant shape cue carries the signal when color fails (colorblindness, print, dark mode oddities).

| Change type | Tint | Gutter bar | Inline shape | Notes |
|---|---|---|---|---|
| **Addition** | green (~10% L shift) | green | — | Block additions same colour, just larger tint area |
| **Deletion** | red (~10% L shift) | red | **strikethrough** | Strikethrough is the colorblind-safe carrier |
| **Formatting only** | blue (~6% L shift) | blue | — | Lighter tint to avoid drowning unchanged text |

Confluence convention informs the blue choice for formatting (familiar to existing doc-tool users).

Tokens (Tailwind v4 / oklch, with light + dark values):

- `--diff-add-bg: oklch(0.96 0.04 145)`, `--diff-add-bar: oklch(0.55 0.15 145)`
- `--diff-remove-bg: oklch(0.96 0.04 25)`, `--diff-remove-bar: oklch(0.58 0.18 25)`
- `--diff-format-bg: oklch(0.96 0.035 250)`, `--diff-format-bar: oklch(0.58 0.14 250)`
- Dark mode: `--diff-add-bg: oklch(0.27 0.055 145)`, `--diff-add-bar: oklch(0.72 0.16 145)`, `--diff-remove-bg: oklch(0.27 0.055 25)`, `--diff-remove-bar: oklch(0.74 0.17 25)`, `--diff-format-bg: oklch(0.27 0.05 250)`, `--diff-format-bar: oklch(0.74 0.14 250)`.

Bar tokens at full saturation; bg tokens at low saturation / high lightness. Verify WCAG AA contrast for strikethrough text on tinted background in both modes.

### 6.4 Block-scope proposals

When `proposal.Scope == "block"`, do not render two full document copies. Show:

- 2–3 blocks of the live document above the targeted block (context).
- The targeted block, unified-diffed against the proposed replacement.
- 2–3 blocks below.

Same color and shape grammar. The header strip and apply/discard behavior are identical.

### 6.5 Mostly-new content

When the proposal is essentially a brand-new document or replaces the bulk of an existing one (e.g., a support gap → new article scenario), an inline diff degenerates into "everything is added" — visually useless.

Heuristic: define these constants in `proposalDiff.ts` and cover them in tests:

```ts
export const MOSTLY_NEW_MIN_BLOCKS = 8
export const MOSTLY_NEW_ADDED_RATIO = 0.7
export const COMPLEX_DIFF_MAX_OPS = 250
```

Switch to preview mode when `totalBlocks >= MOSTLY_NEW_MIN_BLOCKS && addedBlocks / totalBlocks >= MOSTLY_NEW_ADDED_RATIO`.

- Banner: "This proposal replaces the current article."
- Below: render the proposed document in full, no diff highlights.
- Disclosure: "Show what's being replaced" → expands to render the current document below.

Reviewer still gets context; we just stop pretending an inline diff is helpful.

### 6.6 Move rendering

`prosemirror-changeset` can detect a moved block, but rendering "moved from here → to here" cleanly is hard. Render a move as **delete in old position + add in new position**, with a small "Moved" chip on the addition to soften the visual ("Moved from above").

### 6.7 Apply / Discard behavior

**Apply:**
- Single click. No confirmation dialog.
- Safety net is the version snapshot (`e84976c2`) — applied proposals are reversible via the existing version-history Revert flow.
- Button shows spinner during request.
- On success: brief toast + aria-live announcement + replace the proposal review with the live doc + "Applied just now" inline indicator near the top for ~5s.
- If a "next pending proposal" exists on the same doc, success state offers `[View next →]`.

**Discard:**
- Asks for a confirmation dialog ("Discard Quill's proposal? You'll lose this draft."). Discarding loses agent work; brief friction is correct.
- On success: drop the `?proposal=` param, return to the live doc, brief toast.

**Conflict cases:**
- Stale block proposal: backend returns `ErrDocsStaleBlockRevision`; Apply is not allowed. UI copy: "The document changed since Quill wrote this proposal. Refresh the page, or discard it and run Quill again."
- Document-scope proposal after a document update: backend currently allows Apply because document-scope proposals save full content. If `docs_document.updated` arrives while reviewing, keep Apply enabled but show this canonical warning banner: "The document changed since Quill wrote this proposal. Applying may replace newer edits. Review carefully before applying."
- Refreshing from that banner refetches the live document/content and recomputes the diff against the frozen proposal. The diff memo key must include the live content/version identity, not only `proposalId`, so a document/content refetch invalidates stale comparison output.
- Already resolved (race with another reviewer): backend returns `"proposal is already resolved"`. UI shows a toast, redirects to the live doc.

### 6.8 Loading and error states

- Proposal fetch pending: show a skeleton header and diff body, keeping breadcrumbs/sidebar visible.
- 403 / missing `pm.docs.read`: show a permission state in the main column, with no retry loop.
- 404 / proposal not found: show "This proposal is no longer available" with a button back to the live doc.
- Network failure: show an inline retry state. Do not clear the `?proposal=` param unless the user leaves review mode.

### 6.9 Real-time updates

Subscribe to the existing workspace WebSocket bus while in review mode:

- `docs_change_proposal.updated` for this proposal ID → if status moves to `applied | discarded` from another session, show toast + redirect.
- `docs_document.updated` for this document ID → if the underlying doc changes, show the canonical document-scope stale warning from §6.7.

---

## 7. Component structure (frontend)

Proposed file layout, all under `frontend/src/`:

```
pages/docs/DocsDocumentDetail.tsx       (existing — replace compact proposal banner with proposal-mode branch)
components/docs/proposals/
  ProposalReviewView.tsx                (top-level review surface; owns the ?proposal= param)
  ProposalHeader.tsx                    (header strip — agent, summary, sources, actions)
  ProposalSourcesList.tsx               (collapsible source chips)
  ProposalDiffBody.tsx                  (renders the unified diff)
  ProposalDiffBlock.tsx                 (single block within the diff; tint + bar)
  ProposalPreviewMode.tsx               (mostly-new-content fallback)
  ProposalApplyButton.tsx               (apply with loading + error state)
  ProposalDiscardButton.tsx             (discard with confirm)
  proposalDiff.ts                       (prosemirror-changeset wrapper, returns typed change ops)
  proposalDiff.test.ts                  (vitest)
  index.ts                              (barrel)
hooks/queries/
  useDocsChangeProposals                (existing hook — continue to use for pending list)
  useApplyDocsChangeProposal            (existing mutation)
  useDiscardDocsChangeProposal          (existing mutation)
lib/services/
  docsService.ts                        (existing service adapter already exposes list/apply/discard)
lib/queryKeys.ts                        (add proposal query keys)
lib/docsTypes.ts                        (DocsChangeProposal and DocsChangeProposalSource shapes)
```

`DocsDocumentDetail.tsx` already has `changeProposals`, `visibleChangeProposal`, `handleApplyChangeProposal`, and `handleDiscardChangeProposal`. Keep those data paths and upgrade the rendering. This should replace `DocsChangeProposalBanner`, not create a parallel proposal system.

`DocsDocumentDetail.tsx` reads the `?proposal=` query param via TanStack Router. If present, load the proposal by ID through the single-proposal endpoint and render `ProposalReviewView` in the main column. If the proposal is already applied/discarded, render it read-only with status context and no primary Apply action. If the ID is invalid, show a not-found state with a button back to the live doc.

`ProposalReviewView` receives the proposal + current doc content from the existing page-level queries and computes the comparison view.

Banner on the live-doc view: replace the existing compact `DocsChangeProposalBanner` with a `DocPendingProposalsBanner` that links into `?proposal={firstPendingId}`. Empty state: render nothing.

---

## 8. Comparison strategy

Use `prosemirror-changeset` for a block-aware rich-text diff. TipTap is built on ProseMirror, and `prosemirror-changeset` is the right library for comparing current TipTap JSON to proposed TipTap JSON without inventing a markdown/string diff.

Flow inside `proposalDiff.ts`:

1. Take the current TipTap doc and the proposed TipTap doc as ProseMirror `Node` objects.
2. Run `prosemirror-changeset` to get a `ChangeSet` describing the deltas.
3. Walk the change set to produce a render decision:
   - `{ mode: "diff", ops }` for normal diffs.
   - `{ mode: "preview", reason: "mostly_new" | "too_large" | "too_complex" }` when inline diffing would produce noise or poor performance.
4. For `mode: "diff"`, produce a stream of typed render ops:
   - `{ kind: "kept", node }`
   - `{ kind: "added", node }`
   - `{ kind: "deleted", node }`
   - `{ kind: "format-changed", node, formatDelta }`
5. `ProposalDiffBody` consumes the decision. Diff mode renders each op via `ProposalDiffBlock`, which applies the tint + gutter classes. Preview mode renders `ProposalPreviewMode` with a clear reason.

Edge cases the wrapper handles:
- Empty proposed doc → throw early (proposal validation should already prevent this).
- Mostly-new heuristic check for the `ProposalPreviewMode` switch.
- Too-large or too-complex diffs: switch to preview mode when block count or op count exceeds the constants above.
- Move detection → produce paired `deleted` + `added` ops with a `movedTogether: true` flag for the chip.

---

## 9. Apply confirmation policy

**No confirm dialog on Apply.** Reasoning:
- The version snapshot (`e84976c2`) provides a clean revert path.
- Confirmation friction kills the review-loop velocity (gap → review → apply → next).
- Reviewers self-select for being decisive; the diff itself is the confirmation step.

**Confirm dialog on Discard.** Reasoning:
- Discarding throws away the agent's work.
- There is no equivalent of a snapshot to recover from.
- Brief friction prevents accidental loss.

---

## 10. Performance considerations

- `prosemirror-changeset` runs client-side. For a 10k-word doc, expected runtime is well under 200ms — acceptable for an explicit review surface.
- Show a "Computing diff…" skeleton state while the worker runs.
- For documents larger than ~50k words (rare), fall back to preview mode with a clear note that the document is too large for inline diffing.
- The doc content + proposal are both already cached by TanStack Query when reachable; the diff itself can be memoised on `(docContentId, content.updated_at, proposalId)`, or an equivalent live-content version identity plus `proposalId`.

---

## 11. Accessibility

- Color is never the sole carrier:
  - Deletions: strikethrough.
  - Additions: gutter bar (3px solid, full saturation).
  - Formatting: gutter bar.
- Strikethrough on deletions verified for WCAG AA contrast against the red tint background.
- Gutter bars are 3px solid (not dashed) to be unambiguous.
- Apply / Discard buttons have explicit aria-labels naming the proposal summary.
- Header strip is a `<header>` landmark with `<h1>` for the proposal summary.
- Banner on live doc is keyboard-focusable.
- Apply, Discard, race-resolution, and fetch-error outcomes are announced through an aria-live region in addition to toasts.

---

## 12. Implementation phases

### Phase 1 — Backend foundations
- ✅ Proposal CRUD endpoints
- ✅ Apply / discard endpoints
- ✅ Snapshot-on-apply using `VersionTypeProposalApply` / `proposal_apply`
- ✅ WebSocket events
- Add `GET /docs/documents/{docId}/change-proposals/{proposalId}` for stable proposal deep links, including resolved proposals.
- Formalize `DocsChangeProposalSource` with a tolerant decoder; no migration/backfill.
- Expose `proposal_id` and `document_id` in completed Quill run output when `publish_document_change_proposal` succeeds.
- Extend `DocsDocument` / `GET /docs/documents` with `pending_change_proposal_count`.
- Backend estimate for remaining work: ~2 days.

### Phase 2 — Frontend foundations (~1 day)
- Extend existing TanStack Query hooks + `docsService` proposal methods; do not create a parallel service layer.
- TypeScript types (`DocsChangeProposal`, `DocsChangeProposalSource`)
- Query keys
- Real-time invalidation via existing `useRealtimeSync`, including `queryKeys.docs.documents(wsId)` when proposal counts change.

### Phase 3 — Review surface (~2 days)
- `ProposalReviewView` with header strip
- `ProposalSourcesList`
- `ProposalApplyButton` / `ProposalDiscardButton`
- Wire `?proposal=` query param into `DocsDocumentDetail`
- Conflict + race UI states

### Phase 4 — Unified diff rendering (~3-4 days)
- Add `prosemirror-changeset` to the frontend dependencies and verify TypeScript/build resolution before wiring the diff wrapper
- `proposalDiff.ts` wrapping `prosemirror-changeset`
- `ProposalDiffBody` + `ProposalDiffBlock`
- `ProposalPreviewMode` mostly-new/too-large/too-complex fallback
- Color tokens (light + dark) added to global CSS
- WCAG AA contrast check for deletion strikethrough text and gutter/tint visibility in light and dark mode
- Move-rendering chip (delete + add pair)

### Phase 5 — Doc-detail banner (~half day)
- `DocPendingProposalsBanner` mounted above live doc
- Pending count chip in the doc tree/sidebar using backend-provided counts
- Real-time updates when new proposals appear

### Phase 6 — Tests (~1 day)
- `proposalDiff.test.ts` — diff op generation across the change types
- `proposalDiff.test.ts` — preview-mode decisions for mostly-new, too-large, and too-complex diffs
- `ProposalReviewView.test.tsx` — apply / discard / conflict flows
- Backend test for single-proposal get returning pending/applied/discarded proposals with `pm.docs.read`
- Backend test proving existing loose `Sources` values do not break typed source decoding
- Backend/worker test or service test proving a Quill proposal created from a coverage gap exposes `proposal_id` to the run/event path
- Repository/service test proving `pending_change_proposal_count` updates correctly for proposal create, apply, and discard
- `proposalApplyLabel` test already exists on the backend.

**Total estimate: ~9-10 dev days for one engineer.**

---

## 13. Locked product decisions

These decisions are part of the plan so implementation does not stall on UX ambiguity.

1. **Discard reason.** The Discard confirmation includes an optional free-text reason, kept in UI state only for this plan. There is no `discard_reason` column today; do not add one here.
2. **Multiple pending on one doc.** The review header shows a "1 of N" stepper when multiple proposals are pending for the same document. Next/previous changes the `?proposal=` value without leaving review mode.
3. **Improve with Quill.** Do not add a re-run CTA in this plan. The optional discard reason captures reviewer intent, but starting a replacement Quill run from that reason belongs to the agent-learning loop listed as out of scope.

---

## 14. Risks and mitigations

| Risk | Mitigation |
|---|---|
| `prosemirror-changeset` produces ugly diffs for some doc shapes (tables, embeds) | Switch to preview mode if change op count exceeds threshold |
| Large doc diffs slow the page | Skeleton state + memoization; measure before optimising |
| Typed source parsing breaks existing pending proposals | Use tolerant decode/drop behavior; add regression tests with loose legacy source JSON |
| Two reviewers race on the same proposal | Backend already errors with `"already resolved"`; UI handles gracefully |
| Stale block revision after long review session | Backend returns `ErrDocsStaleBlockRevision`; UI explains and offers to re-run Quill |
| Reviewers misclick Apply | Snapshot-on-apply provides revert path through existing version history |
| Color conventions confuse Confluence-trained users | We follow Confluence's green/red/blue convention deliberately |

---

## 15. What is *not* in this plan

- Loop-close: marking a support coverage gap resolved when its proposal applies. Tracked separately because it changes support-gap lifecycle semantics.
- Workspace-wide proposals queue page (`/w/{slug}/docs/proposals`).
- Per-block accept/reject within a single proposal; requires backend lifecycle redesign.
- Inline comments on a proposal.
- Full agent-learning loop from rejected proposals. The optional discard reason can be captured, but training/reinforcement behavior is separate.

---

## 16. Definition of done

- A reviewer can land on `?proposal={id}` from a notification, the doc-detail banner, or a gap CTA.
- They see the unified diff with green/red/blue tints and gutter bars, plus the header strip.
- They can apply or discard with one click (Apply) or one confirmation (Discard).
- After Apply, the version history shows a `"Applied: <summary>"` row (already shipping via `e84976c2`).
- After Discard, the proposal moves to `discarded` and disappears from the pending list.
- Conflict and race states show clear messaging, not a raw error.
- Mobile renders the same surface in a single column.
- All touched components have vitest coverage; pure diff logic is unit-tested.

---
