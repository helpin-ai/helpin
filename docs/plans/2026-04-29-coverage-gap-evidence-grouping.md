# Coverage-gap evidence grouping

> Historical plan, source-compared on 2026-09-17. This page explains the
> evidence-card grouping change for contributors. Grouping and sender-role
> enrichment are implemented; the current display differs from several original
> presentation and counting requirements below.

## Current implementation

The [read-only evidence DTO](../../server/internal/model/support_coverage.go)
adds `sender_role` without adding it to the persisted evidence model. The
[detail query](../../server/internal/repository/support_coverage.go) joins messages
by message ID and workspace ID, uses an empty string for missing senders, returns
errors, and fetches at most 50 rows ordered by descending creation time. Grouping
therefore covers the fetched evidence subset, not an entire conversation history.

The [grouping helper](../../frontend/src/components/support/coverage/evidenceGrouping.ts)
preserves first-seen conversation order and leaves rows without conversations
standalone. The [conversation card](../../frontend/src/components/support/coverage/EvidenceConversationCard.tsx)
sorts items oldest to newest but initially shows the **latest three**, with a
“Show N earlier items” control. Rows with a message ID render Markdown bubbles;
conversation events without message IDs render compact event rows. User, agent,
and AI senders align right; other roles align left. There is no separate centered
`system`-sender branch as proposed below.

[GapDetailPane](../../frontend/src/components/support/coverage/GapDetailPane.tsx)
uses these groups and displays a conversation count based on `evidence_all`, with
`evidence_count` as a fallback. Repository aggregate queries also use distinct
conversation IDs (or evidence IDs for standalone rows) for recent evidence counts.
The original “still per-message” statement should not be read as the semantics
of all current displayed counts. Grouping does not itself deduplicate stored rows.

The [repository test](../../server/internal/repository/support_coverage_test.go)
checks sender-role enrichment, while
[grouping tests](../../frontend/src/components/support/coverage/__tests__/evidenceGrouping.test.ts)
cover helper behavior. These are inspected source evidence, not a fresh browser
or full-suite test run. The original verification steps remain historical.

## Original implementation plan

**Date:** 2026-04-29
**Branch:** waqar-work
**Owner:** Waqar

## Problem

In Support → Coverage → Gap detail pane, the Evidence section renders one
card per `support_gap_evidence` row. Because the backend stores one row per
message, multiple cards from the same conversation appear stacked, each with
its own redundant "Conversation" link. The excerpts are also plain text, so
it's hard to tell at a glance whether a snippet was the customer's question
or our agent's reply.

**Files involved:**
- `frontend/src/components/support/coverage/GapDetailPane.tsx:338–385` — render
- `frontend/src/lib/supportCoverageTypes.ts:45` — `SupportGapEvidence` type
- `server/internal/repository/support_coverage*.go` — evidence query
- `server/internal/model/support_coverage*.go` — evidence DTO

## Goals

1. **One conversation = one card.** Per-message excerpts grouped under a
   single header with a single conversation link.
2. **Chat-bubble layout.** Customer messages left-aligned (muted), our
   replies right-aligned (primary tint), so the role of each excerpt is
   visible without reading.
3. **Future-proof.** A reusable `EvidenceConversationCard` component, plus a
   `sender_role` field on the evidence payload that future gap-analytics
   views can also consume.

## Non-goals

- No arrow-paging between evidences.
- No change to backend gap detection / scoring logic.
- No change to `evidence_count` semantics (still per-message).

## Implementation

### Backend

1. **Add `sender_role` (string) to evidence response without adding a DB column.**
   Values come from `support_messages.sender_type`, not `author_type`.
   Current message sender values are `customer`, `user`, `agent`, and `ai`.
   - Do **not** naively add a normal persisted field to the AutoMigrated
     `SupportGapEvidence` model. `SupportGapEvidence` is included in
     `cmd/api/main.go` AutoMigrate, so an ordinary field risks creating a real
     `sender_role` column.
   - Add a read-only response/scan DTO for gap-detail evidence, e.g.
     `SupportGapEvidenceView`, that embeds or mirrors `SupportGapEvidence` and
     includes `SenderRole string json:"sender_role" gorm:"column:sender_role"`.
     Use this DTO for `SupportCoverageGapDetail.Evidence`.
   - In repository, replace the current evidence fetch with an explicit
     `Table("support_gap_evidence AS e")`, `LEFT JOIN support_messages sm ON
     sm.id = e.message_id AND sm.workspace_id = e.workspace_id`, and
     `Select("e.*, COALESCE(sm.sender_type, '') AS sender_role")`.
   - Keep the existing evidence ordering as `e.created_at DESC` and limit 50.
     The frontend will preserve this order for conversation cards, so cards
     are ordered by most recent evidence first.
   - Nil-safe: when `message_id` is null or the message no longer exists,
     `sender_role` is empty string.
   - While touching this query, stop ignoring the `.Find(...).Error`; return
     `fmt.Errorf("list gap evidence: %w", err)` on failure.
2. **Add a unit test** that an evidence row with a customer message returns
   `sender_role: "customer"`, and an agent/user/AI-side message returns its
   actual `sender_type` (for example `"agent"` or `"ai"`).

### Frontend

1. **Type:** add `sender_role: string` to `SupportGapEvidence` in
   `frontend/src/lib/supportCoverageTypes.ts`.
2. **New component** `frontend/src/components/support/coverage/EvidenceConversationCard.tsx`:
   - Props: `conversationId: string | null`, `wsSlug: string`,
     `evidenceItems: SupportGapEvidence[]`, `evidenceTypeLabel: string`.
   - Header: type label, relative time of latest item, single
     "Conversation" link if `conversationId` set.
   - Body: ordered list of bubbles sorted by `created_at` ascending.
     - `customer` / unknown / empty → left-aligned, `bg-muted` bubble.
     - `user` / `agent` / `ai` → right-aligned, `bg-primary/10` bubble.
     - If a future `system` value appears, render centered, italic muted text.
   - Bubble uses `<Markdown>{excerpt}</Markdown>`, `max-w-[80%]`, small
     padding, `rounded-md`, and a tiny timestamp underneath.
   - Collapse beyond the first 3 messages behind a "Show N more" button
     (local state).
3. **Document evidence (no `conversation_id`)** keeps the existing flat card
   layout — render via a separate small inline branch or a sibling
   `EvidenceArticleCard`.
4. **GapDetailPane:**
   - Group `gap.evidence` by `conversation_id` (preserve insertion order of
     first occurrence). Because the backend list remains `created_at DESC`,
     this makes conversation cards appear by latest evidence first.
   - For each group with a `conversation_id`, render
     `EvidenceConversationCard`.
   - Items without `conversation_id` render with the existing card markup
     (or the new `EvidenceArticleCard`).
   - Remove the inline link / excerpt code that's now in the new component.
5. **Tests:**
   - `__tests__/EvidenceConversationCard.test.tsx`: renders one link per
     conversation; correct alignment per role; collapses past 3 messages.
   - Update any snapshot/coverage UI tests in
     `__tests__/coverageUi.test.ts` if they touch the evidence section.

### Migration / Rollout

- No DB migration; field is computed via join.
- Confirm the implementation does not cause AutoMigrate to add
  `support_gap_evidence.sender_role`.
- Backward compatible — frontend tolerates missing `sender_role` (treat as
  unknown → left-aligned).

## Verification

- `cd frontend && npx tsc -b` clean.
- `cd server && go build ./... && go test ./internal/repository/... ./internal/handler/...`
  for affected packages.
- Manual: open a gap with multiple message evidences from the same
  conversation; confirm one card, one link, alternating bubble alignment
  matching message senders.

## Out of scope / follow-ups

- Pagination of evidence at the API level (currently capped server-side).
- Linking to a specific message anchor inside the conversation thread.
- Reusing the bubble component in suggestion review pane (deferred).
