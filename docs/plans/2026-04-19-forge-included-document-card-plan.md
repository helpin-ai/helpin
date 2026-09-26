---
title: "Forge initial-message links and included document cards"
date: 2026-04-19
owner: azhar
status: proposed
scope: frontend only
---

# Forge initial-message links and included document cards

This historical frontend proposal explains an included-document card and readable links for the first coding-session message. It remains a design record; the current transcript architecture differs from its component and backend assumptions.

## Source review — 2026-09-18

- The proposed `IncludedDocumentCard.tsx` and `initialMessageMarkers.ts` files are absent. [CodingTranscriptPane](../../frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx) renders shared transcript segments and working groups, and does not accept the proposed `runContext` prop. Do not treat the plan as an implemented UI contract.
- [MarkdownContent](../../frontend/src/components/pm/CodingSession/MarkdownContent.tsx) now uses Streamdown and Helpin reference resolution. Its links still use `text-primary`; it does not expose the proposed `anchorTone`. Assess any contrast issue in the actual shared transcript renderer before applying the older bubble-specific patch.
- Run inputs still expose plan/spec document IDs in the [agent model](../../server/internal/model/agent.go), but the old `worker/prompt.go` and `temporalapp/activities.go` emitters are gone. Current prompt/context sources include [agentcontract prompts](../../server/internal/agentcontract/prompt.go) and [Runtime launch context](../../server/internal/service/agent_runtime_launch_context.go). Historical line-number markers are not a reliable current parsing contract.
- The proposal resolves card clicks to a modal with a secondary Docs link; its older goal/manual-test wording says direct new tab and is internally inconsistent. Use the resolved modal decision if reviving this proposal. Claims of zero regression risk are not validated; no visual, contrast, or interaction tests were run in this review.

## Original proposal

Two small UX fixes on the Forge coding-session surface. **Backend behavior is
intentionally unchanged** — the agent still receives the plan / spec document
inlined in its prompt because it needs that content to know what task to
perform. All changes here are frontend.

This revision supersedes the first draft and corrects three issues caught in
review: (1) the coding-session surface does not actually have task / epic /
run-input metadata in its tree today, (2) several prompt markers were wrong
or missing, (3) `MarkdownContent` has consumers beyond the user bubble and
the link-color change needs to be scoped. The implementation remains
display-only so the coding agent's prompt, execution flow, transcript
artifacts, and interactive session behavior are unaffected.

## Problems (today)

### 1. Links inside the user bubble are near-black on blue

When a Forge run starts, the initial user message renders in a blue bubble
with white text. Any markdown link inside the message (e.g. the
`https://buddy.works/…` URL pulled from the task description) renders with
`text-primary`, which resolves to a near-black color. On blue-600 this is
effectively unreadable.

**Root cause**:
`frontend/src/components/pm/CodingSession/MarkdownContent.tsx:38–46`
hard-codes `className="text-primary underline underline-offset-2"` on its
`<a>` renderer.

### 2. The included document's full markdown is dumped inline into the bubble

The backend composes the initial user message by calling
`workerpkg.BuildUserPrompt` (`server/internal/worker/prompt.go:154`) with an
`initialInstructions` string and an `artifactContext`. Both can carry entire
document bodies verbatim:

**Epic runs**
- `AgentRunActivities.loadRunArtifactContext` appends an `ArtifactContextEntry`
  with `Label: "Linked epic spec document"` and the full markdown of the spec
  doc, `PreserveFull: true`
  (`server/internal/temporalapp/activities.go:1199–1215`).
- `formatArtifactContext` writes this into the prompt under the literal
  section header `"Current persisted artifacts:"`
  (`server/internal/worker/prompt.go:298–335`).
- When the agentic epic planner runs, `buildAgenticEpicPlannerInstructions`
  additionally inlines the spec content and linked docs into
  `initialInstructions` (`server/internal/temporalapp/activities.go:4538+`,
  specifically `4560`, `4586`, `4590`).

**Task runs (planner path)**
`buildTaskPlannerInstructions`
(`server/internal/temporalapp/activities.go:4349+`) inlines into
`initialInstructions`:
- `"Canonical task planning document ID: <id>"` (`:4371`) or
  `"Canonical task planning document: <title> [<id>]"` (`:4521`)
- `"Current task planning draft already in Docs:"` + body (`:4378`)
- `"Approved epic PRD version ID: <id>"` / `"Approved epic PRD snapshot:"`
  + body (`:4407–4408`)
- `"Parent epic PRD document ID: <id>"` / `"Current epic PRD draft:"`
  + body (`:4411–4412`)
- `"Other docs linked directly to this task:"` + body (`:4432`, `:4532`)
- `"Other docs linked to the parent epic:"` + body (`:4441`)

The string returned by `buildInitialInstructions` is appended to the user
prompt under the literal section header `"Additional instructions:"`
(`prompt.go:234–235`), and the result is what the bubble renders.

Result: the user sees the task name, description, and additional
instructions followed by the whole markdown body of the plan or spec doc
(and any linked docs), clipped behind a "Show more" fold. The affordance
that "Forge has this document" is lost in the noise.

## Constraints

- **Backend is not in scope.** The agent must keep receiving the inlined
  document content. This plan does not introduce a `read_plan_document`
  tool, does not change `BuildUserPrompt`, and does not split display text
  from prompt text on the server.
- **No new compose UI.** Attaching a document is not manual. When the user
  clicks "Forge" from a story / task / epic, the doc is already implied by
  the linked-object relationships on that record.
- **Frontend only.** All work lands under `frontend/src/`.

## UX goal

When a user opens a Forge coding session, they should:

1. See the task context (description, additional instructions, branch info)
   in a clean bubble — no walls of doc markdown.
2. See a dedicated **Included document** card directly below the bubble for
   each doc that the run inlined.
3. Be able to click the card to open the full document in a new tab.

## Non-goals / safety rails

- Do not change backend prompt construction or agent inputs.
- Do not mutate persisted transcript messages, event payloads, or artifacts.
- Do not strip or rewrite content before it reaches the model; strip only in
  the rendered first-user-bubble UI path.
- Do not alter assistant messages, follow-up user replies, approvals, auth,
  or failure-continuation behavior in the coding session.
- Do not block the coding-session UI if run metadata or document-title fetches
  fail; the fallback is the current bubble rendering.

## UI specification

### Included document card

- Sits directly below the first user message bubble in
  `CodingTranscriptPane.tsx`, right-aligned to match the bubble alignment.
- One card per included document, in backend-inlined order.
- Section label above the card(s): **"Included from this task"** or
  **"Included from this epic"** depending on target.

Card slots:

| Slot | Content |
|---|---|
| Icon | `FileTextIcon` from `@/lib/icons` |
| Title | Document title (resolved by ID) |
| Meta | `Plan doc · HEL-15` / `Spec doc` / `Linked doc` |
| Affordance | `ArrowUpRightIcon` |

Interaction:
- Card is a `<button>` that opens a modal showing the doc content, using the
  existing `PreviewExpandDialog` from `CodingPreviewPanels.tsx` (the same
  dialog "Expand preview" already uses). No new dialog component.
- The modal body renders the doc via a read-only `<DocContent
  documentId>` — see open questions on where this renderer comes from.
- The modal footer/header carries a small `Open in docs ↗` link to
  `/w/{slug}/docs/documents/{docId}` (`target="_blank"`) for users who want
  the full page (edit, share, deep link).
- `Esc` closes the modal. `Cmd/Ctrl+Click` still honors new-tab on the
  `Open in docs` link.
- Card hover: `border-primary/40`, `bg-muted/60`, icon turns `text-primary`.
- Card keyboard focus: same + `ring-1 ring-primary/40`.

### Bubble cleanup

The bubble strips out backend marker sections that duplicate what the cards
already show.

Markers are defined once, centrally, and kept in sync with the backend
emitters listed above. The rendering pipeline finds the **earliest
occurrence** of any marker in the content and returns the slice before it.

**Canonical marker list** (one const, one source of truth):

```ts
// frontend/src/components/pm/CodingSession/initialMessageMarkers.ts
export const FORGE_INLINED_DOC_MARKERS = [
  'Current persisted artifacts:',                  // prompt.go:304
  'Linked epic spec document',                     // artifacts entry label
  'Canonical task planning document ID:',          // activities.go:4371, 4519
  'Canonical task planning document:',             // activities.go:4521
  'Current task planning draft already in Docs:',  // activities.go:4378
  'Approved epic PRD version ID:',                 // activities.go:4407
  'Approved epic PRD snapshot:',                   // activities.go:4408
  'Parent epic PRD document ID:',                  // activities.go:4411
  'Current epic PRD draft:',                       // activities.go:4412
  'Existing canonical spec document ID:',          // activities.go:4560
  'Other docs linked directly to this task:',      // activities.go:4432, 4532
  'Other docs linked to the parent epic:',         // activities.go:4441
  'Other docs linked to this epic:',               // activities.go:4590
] as const;
```

Fragility note: this list is coupled to backend wording. Each entry carries
a line reference so a future backend change can be tracked back. If a marker
disappears, the bubble simply shows more content than intended — no crash,
no data loss. A focused unit test feeds a representative backend output and
asserts the stripped prefix ends at `"Additional instructions:"` (or the
task description, for runs without additional instructions).

### Empty state

No doc linked to the run: no card, no section label. The bubble shows the
full message as-is. No "no doc attached" hint.

## Data plumbing (required, was missing from v1)

The coding-session surface does not have task / epic / run-input data in
its tree today. `CodingSessionSurface` only loads the session + event
stream (`frontend/src/components/pm/CodingSession/CodingSessionSurface.tsx`),
`CodingTranscriptPane` only receives transcript / session props
(`frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx`), and
the `CodingSession` type itself has `target_type` and `target_id` only
(`frontend/src/lib/pm-types/codingSession.ts:70–94`).

The authoritative source for included docs is the agent run's input, which
carries top-level `plan_document_id` / `spec_document_id`
(`server/internal/model/agent.go:466–485` on `AgentRunInputPayload`), not
`run_facts` as the v1 draft claimed.

### New fetches

1. **Agent run** — for `input.plan_document_id` / `input.spec_document_id`
   / `input.trigger` / `input.target`.
   - Add `agentService.getRun(workspaceId, runId)` if it does not already
     exist, and a `useAgentRun(workspaceId, runId)` query hook.
   - Call site: `CodingSessionSurface` has `session.run_id` in scope; fetch
     alongside the session.

2. **Target task or epic** — only when the run needs task metadata for the
   task-plan card meta label.
   - `session.target_type === 'task'` → `useTask(workspaceId, session.target_id)`
     (already exists in `frontend/src/hooks/queries/`) for `task_key`.
   - `session.target_type === 'epic'` → `useEpic(workspaceId, session.target_id)`.
     Current frontend `Epic` types do not expose an epic key / display label,
     so epic cards should use `Spec doc` without a `HEL-E-*` suffix unless
     that field is added separately.

3. **Document titles** — one per doc id collected above.
   - `useDocsDocument(workspaceId, docId)` (confirm existing hook; add a
     thin wrapper around `docsService.getById` if missing).
   - Acceptable to fetch lazily inside `IncludedDocumentCard` so each card
     fetches its own title; list of cards is small (usually 1–3).

4. **Linked docs** (optional, phase 2) — for `Other docs linked to this
   task/epic` parity.
   - `useDocsLinkedDocs(workspaceId, objectType, objectId)` backed by the
     existing docs-link repository endpoint.
   - Exclude the doc already shown as plan / spec to avoid duplicates.

### Wiring

- `CodingSessionSurface` fetches `useAgentRun(run_id)` and passes the
  resulting `run` (or just `run.input` + target object) down to
  `CodingTranscriptPane` as a new prop `runContext`.
- `CodingTranscriptPane` derives the `IncludedDocument[]` list from
  `runContext` and renders cards after the first user bubble.
- All of this is additive — existing props and call sites keep working.

## ASCII mockups

### Current state (problem)

```
                                  less than a minute ago · Muhammad Azhar (🟢)
           ┌────────────────────────────────────────────────────────────┐
           │ Context: Task: 2FA via QR Code / Authenticator             │
           │                                                            │
           │ Description: … https://buddy.works/docs/…/security         │
           │ (black underlines on blue — unreadable)                    │
           │                                                            │
           │ Additional instructions:                                   │
           │                                                            │
           │ Canonical task planning document: 2FA spec v3 [doc-123]    │
           │ Current task planning draft already in Docs:               │
           │ # 2FA spec v3                                              │
           │ ## Goals                                                   │
           │ - Support TOTP via authenticator apps                      │
           │ - Encrypted recovery codes                                 │
           │ ## Non-goals                                               │
           │ - SMS-based 2FA                                            │
           │ … 40 more lines …                                          │
           │                                                            │
           │ Other docs linked directly to this task:                   │
           │ … more markdown …                                          │
           │                                                            │
           │ Show more                                                  │
           └────────────────────────────────────────────────────────────┘
```

### Proposed state

```
                                  less than a minute ago · Muhammad Azhar (🟢)
           ┌────────────────────────────────────────────────────────────┐
           │ Context: Task: 2FA via QR Code / Authenticator             │
           │                                                            │
           │ Description: I would like to have the 2FA in Helpin via    │
           │ QR Authenticator. Similar to what buddy.works is doing.    │
           │ https://buddy.works/docs/basics/workspace-settings/security │
           │ (white underlines on blue — readable)                      │
           │                                                            │
           │ Additional instructions: Repository branches: base `main`, │
           │ working `hel-15-2fa-via-qr-code-authenticator`.            │
           └────────────────────────────────────────────────────────────┘

           Included from this task
           ┌───────────────────────────────────────┐
           │ 📄  2FA spec v3                    ↗  │
           │     Plan doc · HEL-15                 │
           └───────────────────────────────────────┘

  ╭ Forge ──────────────────────────────────────────────────────────────╮
  │ I reviewed the plan doc and will reuse `security/totp.go`. Starting │
  │ the plan…                                                           │
  ╰─────────────────────────────────────────────────────────────────────╯
```

### Epic run variant

```
           Included from this epic
           ┌───────────────────────────────────────┐
           │ 📄  Security hardening Q2          ↗  │
           │     Spec doc · HEL-E-4                │
           └───────────────────────────────────────┘
```

### Multiple docs (plan + linked)

```
           Included from this task
           ┌───────────────────────────────────────┐
           │ 📄  2FA spec v3                    ↗  │
           │     Plan doc · HEL-15                 │
           └───────────────────────────────────────┘
           ┌───────────────────────────────────────┐
           │ 📄  Auth architecture overview     ↗  │
           │     Linked doc                        │
           └───────────────────────────────────────┘
```

### Hover / focus state

```
           ┌───────────────────────────────────────┐  ← border-primary/40
           │ 📄  2FA spec v3                    ↗  │  ← bg-muted/60
           │     Plan doc · HEL-15                 │  ← ring-1 ring-primary/40
           └───────────────────────────────────────┘
```

## Implementation

### Step 1 — Link color fix with scoped blast radius

**Blast radius (accurate)**: `MarkdownContent` is imported from:
- `frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx:36`
  (user and assistant bubbles in the transcript)
- `frontend/src/components/pm/CodingSession/CodingPreviewPanels.tsx:162`
  (preview panels on a light card background)
- `frontend/src/components/pm/CodingSession/PublishedToolPreviewCard.tsx:119`
  (published tool preview cards on a light card background)

A blanket change from `text-primary` to `text-current` fixes the user bubble
but removes the accent color for links in both preview consumers, which is a
minor design regression in those surfaces. To avoid that, make the link tone
explicit.

**Change (additive prop)** in
`frontend/src/components/pm/CodingSession/MarkdownContent.tsx`:

```tsx
type AnchorTone = 'primary' | 'inherit';

export function MarkdownContent({
  content,
  className,
  anchorTone = 'primary',
}: {
  content: string;
  className?: string;
  anchorTone?: AnchorTone;
}) {
  // …
  a: ({ children, href }) => (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className={cn(
        'underline underline-offset-2 hover:opacity-90',
        anchorTone === 'primary'
          ? 'text-primary'
          : 'text-current decoration-current',
      )}
    >
      {children}
    </a>
  ),
  // …
}
```

Call sites:
- `CodingTranscriptPane.tsx` — the user bubble (non-assistant branch,
  line ~726 and ~746) passes `anchorTone="inherit"`. Assistant branch stays
  default (`"primary"`).
- `CodingPreviewPanels.tsx` — no change.
- `PublishedToolPreviewCard.tsx` — no change.

Effect:
- Blue bubble, white text: white link with white underline — readable.
- White bubble, dark text: unchanged (primary-accent link).
- Preview panels: unchanged.

### Step 2 — Fetch run + target metadata

Add or confirm the following in `frontend/src/hooks/queries/`:

- `useAgentRun(workspaceId, runId)` → `{ run: AgentRun }` with
  `run.input.plan_document_id` / `run.input.spec_document_id` /
  `run.input.target` exposed.
- `useDocsDocument(workspaceId, docId)` → `{ id, title }` (title fetch).
- `useTask(workspaceId, taskId)` and `useEpic(workspaceId, epicId)` already
  exist; consume as appropriate for the `task_key` / `epic_key` meta label.

In `CodingSessionSurface.tsx`:
- After `session` is loaded, fetch the run with
  `useAgentRun(workspaceId, session.run_id)`.
- Pass a derived `runContext` prop into `CodingTranscriptPane`:
  ```ts
  type IncludedDocument = {
    id: string;
    kind: 'plan' | 'spec' | 'linked';
    taskKey?: string; // task only, e.g. "HEL-15"
  };
  type RunContext = {
    sourceKind: 'task' | 'epic' | 'other';
    includedDocuments: IncludedDocument[];
  };
  ```
  Build `includedDocuments` from the run input:
  - if `input.plan_document_id` → `{ id, kind: 'plan', taskKey }`
  - if `input.spec_document_id` and sourceKind === 'epic' →
    `{ id, kind: 'spec' }`
  - (phase 2) linked docs from `useDocsLinkedDocs`.

### Step 3 — New component `IncludedDocumentCard` + reuse `PreviewExpandDialog`

We do **not** introduce a new dialog component. The existing
`PreviewExpandDialog` in `CodingPreviewPanels.tsx:102` is a generic shell —
its props are `open`, `onOpenChange`, `title`, `children`, and nothing about
it is preview-specific. It currently lacks `export`; add the keyword in
place (smallest diff) so the card can import it:

```tsx
// frontend/src/components/pm/CodingSession/CodingPreviewPanels.tsx
export function PreviewExpandDialog({ open, onOpenChange, title, children }: { /* unchanged */ }) { /* unchanged */ }
```

If a third consumer appears later, extract `PreviewExpandDialog` into a
shared file (e.g. `CodingSessionExpandDialog.tsx`). For this PR, the
in-place export is sufficient.

**New file**:
`frontend/src/components/pm/CodingSession/IncludedDocumentCard.tsx`

```tsx
import { useState } from 'react';
import { FileTextIcon, ArrowUpRightIcon } from '@/lib/icons';
import { useDocsDocument } from '@/hooks/queries/docs';
import { PreviewExpandDialog } from './CodingPreviewPanels';
import { DocContent } from '@/pages/docs/DocContent'; // or wherever the read-only renderer lives — see open questions

type Props = {
  workspaceId: string;
  workspaceSlug: string;
  documentId: string;
  kind: 'plan' | 'spec' | 'linked';
  taskKey?: string;
};

export function IncludedDocumentCard({ workspaceId, workspaceSlug, documentId, kind, taskKey }: Props) {
  const [open, setOpen] = useState(false);
  const { data: doc } = useDocsDocument(workspaceId, documentId);
  const title = doc?.title ?? 'Loading document…';
  const metaLabel = {
    plan:   taskKey ? `Plan doc · ${taskKey}` : 'Plan doc',
    spec:   'Spec doc',
    linked: 'Linked doc',
  }[kind];

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="group flex items-center gap-3 rounded-xl border border-border/60 bg-background px-3 py-2.5 text-sm shadow-sm transition-colors hover:border-primary/40 hover:bg-muted/60 focus:outline-none focus-visible:ring-1 focus-visible:ring-primary/40"
      >
        <FileTextIcon className="h-5 w-5 shrink-0 text-muted-foreground group-hover:text-primary" />
        <div className="min-w-0 flex-1 text-left">
          <div className="truncate font-medium text-foreground">{title}</div>
          <div className="truncate text-[12px] text-muted-foreground">{metaLabel}</div>
        </div>
        <ArrowUpRightIcon className="h-4 w-4 shrink-0 text-muted-foreground group-hover:text-primary" />
      </button>

      <PreviewExpandDialog open={open} onOpenChange={setOpen} title={title}>
        <DocContent workspaceId={workspaceId} documentId={documentId} />
        <a
          href={`/w/${workspaceSlug}/docs/documents/${documentId}`}
          target="_blank"
          rel="noreferrer"
          className="mt-6 inline-flex items-center gap-1 text-xs font-medium text-primary hover:underline"
        >
          Open in docs <ArrowUpRightIcon className="h-3 w-3" />
        </a>
      </PreviewExpandDialog>
    </>
  );
}
```

The card's visuals are identical to the earlier draft; only the click
behavior changes (button → modal instead of anchor → new tab).

### Step 4 — Render cards under the first user bubble

In `CodingTranscriptPane.tsx`, after the first user `AssistantMessageBubble`
is rendered, render the card list when `runContext.includedDocuments.length
> 0`:

```tsx
{isFirstUserMessage && runContext.includedDocuments.length > 0 && (
  <div className="mt-2 flex w-full max-w-[90%] flex-col items-end gap-1.5 self-end">
    <div className="text-[11px] font-medium text-muted-foreground">
      {runContext.sourceKind === 'epic' ? 'Included from this epic' : 'Included from this task'}
    </div>
    {runContext.includedDocuments.map((doc) => (
      <IncludedDocumentCard
        key={doc.id}
        workspaceId={workspaceId}
        workspaceSlug={workspaceSlug}
        documentId={doc.id}
        kind={doc.kind}
        taskKey={doc.taskKey}
      />
    ))}
  </div>
)}
```

### Step 5 — Strip inlined doc sections from the bubble

**File**:
`frontend/src/components/pm/CodingSession/initialMessageMarkers.ts` (new)
and applied in `CodingTranscriptPane.tsx` before passing `content` to the
first user bubble only.

```ts
import { FORGE_INLINED_DOC_MARKERS } from './initialMessageMarkers';

export function stripInlinedDocSections(content: string): string {
  let earliest = -1;
  for (const marker of FORGE_INLINED_DOC_MARKERS) {
    const idx = content.indexOf(marker);
    if (idx >= 0 && (earliest === -1 || idx < earliest)) {
      earliest = idx;
    }
  }
  return earliest < 0 ? content : content.slice(0, earliest).trimEnd();
}
```

Apply only to the first user message in the transcript. Subsequent user
replies and assistant turns pass through unchanged.

## Files changed

| File | Change |
|---|---|
| `frontend/src/components/pm/CodingSession/MarkdownContent.tsx` | Add `anchorTone` prop; default `primary` preserves current look. |
| `frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx` | Pass `anchorTone="inherit"` for user bubble; strip inlined doc sections on first user message; render `IncludedDocumentCard` list. |
| `frontend/src/components/pm/CodingSession/CodingSessionSurface.tsx` | Fetch run + target metadata; derive `runContext`; pass to transcript pane. |
| `frontend/src/components/pm/CodingSession/IncludedDocumentCard.tsx` | New component. Reuses `PreviewExpandDialog` for the click-to-expand modal — no new dialog component. |
| `frontend/src/components/pm/CodingSession/CodingPreviewPanels.tsx` | Add `export` keyword to `PreviewExpandDialog` so the card can import it. No other changes in this file. |
| `frontend/src/components/pm/CodingSession/initialMessageMarkers.ts` | New const list + `stripInlinedDocSections`. |
| `frontend/src/hooks/queries/agentRuns.ts` (or equivalent) | Add `useAgentRun(workspaceId, runId)` if missing. |
| `frontend/src/hooks/queries/docs.ts` | Reuse existing `useDocsDocument(workspaceId, docId)` hook; no new wrapper needed unless call-site ergonomics require one. |
| `frontend/src/lib/services/agentService.ts` | Add `getRun(workspaceId, runId)` if missing. |

No backend files change.
No new migration.
No new endpoint.

## Rollout order

1. **Step 1 — link color fix** (scoped via `anchorTone` prop). Ship
   after checking the affected transcript and preview consumers.
2. **Steps 2 + 3 + 4 — fetch run metadata, add `IncludedDocumentCard`,
   render card list**. Visible improvement, no content is hidden yet.
3. **Step 5 — strip inlined doc sections from the bubble**. Ship only
   after the card is live, so the doc remains reachable from the UI.
4. (Phase 2, optional) Extend `runContext.includedDocuments` to include
   task / epic linked docs via `useDocsLinkedDocs`. Keep the same card
   component; only the data source grows.

## Test plan

Unit:
- `stripInlinedDocSections`:
  - input with each marker → returns prefix before the first marker
  - input with multiple markers → returns prefix before the earliest
  - input with no markers → returns input unchanged
  - input with marker at index 0 → returns empty string
- `IncludedDocumentCard`:
  - renders the correct meta label for each `kind`
  - task plan cards include `task_key` when available
  - epic spec cards render `Spec doc` without assuming an epic key
  - link target matches `/w/{slug}/docs/documents/{id}` and opens in new tab

Manual:
- Task run with a `plan_document_id`:
  - bubble shows description + instructions, no doc markdown
  - card appears below, right-aligned
  - clicking opens the doc in a new tab
  - external URL in the description is readable on blue
- Epic run with a `spec_document_id`:
  - bubble hides the inlined spec markdown
  - card appears as `Spec doc`
  - no epic key suffix is assumed unless the product later exposes one
- Run with no linked doc: no card, bubble shows as-is
- Dark mode: hover / focus states remain legible
- Mobile width: card wraps gracefully; title and meta truncate
- Preview panels (`CodingPreviewPanels`, `PublishedToolPreviewCard`): link
  color remains the current `text-primary` (not changed)

## Open questions

- Linked-doc scope: include task / epic linked docs in v1 or defer to a
  phase-2 follow-up? — Defer. Plan + spec cover the common case; linked
  docs multiply the query count and add ordering questions.
- Click behavior: new tab vs. side drawer vs. modal? — **Resolved: modal**,
  via the existing `PreviewExpandDialog` shell (same component "Expand
  preview" uses). Keeps the user in the session surface; a secondary
  "Open in docs ↗" link inside the modal preserves the deep-link escape
  hatch to the full docs page.
- Read-only doc renderer (`<DocContent>`): does a reusable read-only doc
  view already exist in the codebase, or do we extract one from
  `frontend/src/pages/docs/DocsDocumentDetail.tsx`? Options in order of
  preference: (a) extract a thin `<DocContent workspaceId documentId>` from
  `DocsDocumentDetail` that renders the TipTap content read-only, (b)
  fall back to rendering the doc's persisted markdown through
  `MarkdownContent` inside the dialog. Pick at implementation time based
  on what `DocsDocumentDetail` exposes today.
- Card position on the list view (`AgentRunsCard.tsx`): only inside the
  coding session for v1; list-view surfacing is a separate design.
- Should epic cards eventually show a human-readable epic key? — Not in this
  plan. Current frontend epic types do not expose one, so v1 uses `Spec doc`
  without a suffix.

## Known fragility

- Marker-based stripping is brittle. Mitigation: the marker list is
  centralized in one file with line references back to the backend
  emitter, and a unit test guards the shape. If the backend rewords a
  header, the bubble will show slightly more content than intended but
  will not crash.
- `useAgentRun` and `useDocsDocument` may not exist yet. If either is
  missing, add them as thin wrappers — no new service code beyond a
  fetch + `unwrap()`.
