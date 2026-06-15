# Unified Agent Transcript Renderer

**Date:** 2026-06-15
**Status:** Proposed
**Owner:** frontend / agent surfaces

## 1. Problem

We render an agent run's transcript (assistant messages + tool calls + reasoning + status)
in two places with two unrelated implementations:

- **Command dock** — `frontend/src/components/agents/dock/DockTranscript.tsx`
  A deliberately minimal, flat, one-line-per-event readout. Tool calls are a single
  row (`ToolCallRow`, `DockTranscript.tsx:120`): status icon + `primaryLabel`, nothing
  else. Assistant text is bare `MarkdownContent`. No avatars, no rails, no expansion.
- **Coding session slider** — `frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx`
  A full virtualized timeline: avatar circles + connector rails (`AssistantTimelineRow`
  `:1017`, `ActivityToolCallRow` `:1281`), category-colored tool chrome (`toolChrome`
  `:1179`), expandable args/result/diff, tool grouping (`CollapsedToolCallGroup` `:1352`),
  reasoning (`ThinkingStrip` `:1485`), status rows, prompt/context cards.

Both already depend on the same data model (`CodingSessionStreamState` in
`frontend/src/lib/pm-types/codingSession.ts:253`) and the same label helper
(`describeToolCall` in `frontend/src/components/pm/CodingSession/toolCallPresentation.ts:60`),
but duplicate everything else. We want the **dock's clean one-line aesthetic** to become
the shared baseline, while the slider keeps depth on demand.

## 2. Goals

1. **One shared row kit** that renders every transcript event as a single line
   (`[icon] label … [right meta]`), no avatar, no connector rail.
2. **One shared segment collector** that both surfaces feed from, replacing
   `collectDockSegments` (`DockTranscript.tsx:24`) and the slider's `items` builder
   (`CodingTranscriptPane.tsx:190`).
3. **Expand-on-click depth** for the slider: collapsed rows look like the dock; clicking
   reveals args / result / `ApplyPatchDiff` / `PublishedToolPreviewCard`. The dock opts
   out (`expandable={false}`) and stays flat.
4. **Reasoning / status / context as collapsed one-liners** (e.g. `🔒 Thinking · 4s ›`).
5. **Per-surface segment scope:** dock shows `assistant + tool` (and optionally
   `reasoning`); slider shows all kinds.

## 3. Non-goals

- The dock does **not** adopt virtualization, the composer, or the interruption overlay.
- The slider keeps its own scroll container, virtualizer, auto-follow, running indicator,
  `InterruptionOverlay`, and `MessageInput` — these are **not** shared.
- No backend / stream-reconciler changes. `codingSessionStream.ts` is untouched.
- We are not redesigning `CodingInteractionCard` (approvals) — already shared and out of
  scope.
- **Tool grouping is dropped initially** (see §6). No `CollapsedToolCallGroup` in the
  unified renderer for v1.

## 4. Target architecture

New folder: `frontend/src/components/agents/transcript/`

```
transcript/
  segments.ts            // shared segment union + collectSegments(stream, opts)
  TranscriptRow.tsx      // generic one-line row: icon + label + meta + optional expand
  segmentRenderers.tsx   // maps a TranscriptSegment -> <TranscriptRow> (+ expanded body)
  toolRowChrome.ts       // status/category icon resolution (extracted from toolChrome)
  index.ts               // barrel
  __tests__/
    segments.test.ts
    segmentRenderers.test.tsx
```

### 4.1 Segment model (`segments.ts`)

A normalized, ordered union covering every renderable kind:

```ts
export type TranscriptSegment =
  | { kind: 'assistant'; id: string; content: string; streaming?: boolean }
  | { kind: 'tool'; id: string; toolCall: CodingSessionLiveToolCall }
  | { kind: 'reasoning'; id: string; reasoning: CodingSessionLiveReasoningMessage }
  | { kind: 'status'; id: string; message: CodingSessionTranscriptMessage }
  | { kind: 'context'; id: string; message: CodingSessionTranscriptMessage }
  | { kind: 'user'; id: string; message: CodingSessionTranscriptMessage }
  | { kind: 'review_decision'; id: string; message: CodingSessionTranscriptMessage };

export interface CollectSegmentsOptions {
  /** Include in-flight live_turn_segments / live reasoning (run still active). */
  includeLive: boolean;
  /** Which kinds to emit. Dock passes a subset; slider passes all. */
  include?: ReadonlySet<TranscriptSegment['kind']>;
}

export function collectSegments(
  stream: CodingSessionStreamState,
  opts: CollectSegmentsOptions,
): TranscriptSegment[];

export function hasRenderableSegments(
  stream: CodingSessionStreamState | null,
  opts: CollectSegmentsOptions,
): boolean;
```

Rules carried over from the two existing collectors:
- Prefer `message.turn_segments` to keep text/tool interleaving; fall back to
  `message.content` + `message.tool_calls` (mirror `DockTranscript.tsx:30-51`).
- Always drop `tool_name === 'update_plan'` tool calls.
- When `includeLive`, append `live_turn_segments` + `live_reasoning_message`; when terminal,
  skip them (already folded into transcript) — same guard as `DockTranscript.tsx:53`.
- Status vs. transcript classification uses the existing `isStatusTranscriptMessage`
  (`codingSessionPresentation.ts`) — reused, not reimplemented.
- `context` segment = the prompt disclosure the slider derives from `promptArtifact` /
  `session.system_prompt` (`CodingTranscriptPane.tsx:149-175`). Because that derivation
  needs `promptArtifact`/`session` (not on the stream), `collectSegments` accepts an
  optional `leadingContext?: CodingSessionTranscriptMessage` that the slider supplies and
  the dock omits.

### 4.2 `TranscriptRow` (`TranscriptRow.tsx`)

The single visual primitive. Flat, one line by default:

```ts
interface TranscriptRowProps {
  icon: ReactNode;
  iconClassName?: string;       // status/category tint
  label: ReactNode;             // truncated primary label
  meta?: ReactNode;             // right-aligned (duration, badge, chips)
  tone?: 'default' | 'muted' | 'failed';
  expandable?: boolean;         // slider true, dock false
  children?: ReactNode;         // expanded body, only rendered when open
  defaultOpen?: boolean;
}
```

- Collapsed: `flex items-center gap-1.5`, label `truncate`, no avatar, no connector line.
  Visually matches the dock's current `ToolCallRow` (`DockTranscript.tsx:124-137`).
- When `expandable` and `children` present: the whole row is a `button` toggling a
  disclosure; a small chevron appears in `meta`. When `!expandable`, `children` is never
  rendered (dock stays lossy and flat).

### 4.3 `segmentRenderers.tsx`

`renderSegment(segment, { expandable })` → `<TranscriptRow>`:

| kind | collapsed line | expanded body (slider only) |
|------|----------------|------------------------------|
| `assistant` | `MarkdownContent` (no row chrome — rendered inline, not as an icon row) | n/a (markdown shows in full; long content keeps `CollapsibleMarkdown` Show more/less) |
| `tool` | status/category icon + `describeToolCall().primaryLabel` + chips/duration in `meta` | args `CollapsibleCodeBlock` / result / `ApplyPatchDiff` / `PublishedToolPreviewCard` (moved verbatim from `ActivityToolCallRow` `:1281`) |
| `reasoning` | `🔒 Thinking · {elapsed} ›` | reasoning text + encrypted-payload note (from `ThinkingStrip` `:1485`) |
| `status` | `Loading icon + status text + relative time` | none (already terminal one-liner) |
| `context` | `Run context ›` | prompt `<pre>` (from `RunContextDisclosure` `:721`) |
| `user` | right-aligned bubble (slider only — excluded from dock scope) | Show more/less for long content |
| `review_decision` | decision bubble (slider only) | — |

Notes:
- `assistant` stays **not** an icon row — it renders markdown directly, exactly like the
  dock today (`DockTranscript.tsx:106`) and the slider's `CollapsibleMarkdown`. Avatar +
  "Assistant" label + Live/Update badge from `AssistantTimelineRow` are **dropped**.
- Tool icon/tint comes from a `toolRowChrome.ts` extracted from `toolChrome`
  (`CodingTranscriptPane.tsx:1179`), minus the rounded avatar wrapper — just the icon glyph
  + a text color class.

## 5. File-by-file changes

### New
- `transcript/segments.ts` — `collectSegments`, `hasRenderableSegments`, `TranscriptSegment`.
- `transcript/TranscriptRow.tsx` — generic row.
- `transcript/segmentRenderers.tsx` — per-kind rendering.
- `transcript/toolRowChrome.ts` — icon/tint resolution.
- `transcript/index.ts` — barrel.
- Tests under `transcript/__tests__/`.

### Modified — dock (migrate first)
- `frontend/src/components/agents/dock/DockTranscript.tsx`
  - Delete `collectDockSegments`, `DockSegment`, `ToolCallRow`.
  - `DockTranscript` becomes: `collectSegments(stream, { includeLive: active, include: DOCK_KINDS })`
    → map through `renderSegment(seg, { expandable: false })`, wrapped in the existing
    `space-y-1.5` container.
  - `dockTranscriptHasContent` → delegate to `hasRenderableSegments(stream, …)`.
  - `DOCK_KINDS = new Set(['assistant', 'tool'])` (add `'reasoning'` if we want the dock
    to show collapsed reasoning — flag it in review; default **assistant + tool only**).
  - Public API (`DockTranscript`, `dockTranscriptHasContent`) and its two call sites in
    `ExecutionStrip.tsx:258,414` are unchanged.

### Modified — slider (migrate second)
- `frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx`
  - Keep: scroll container, `useVirtualizer`, auto-follow effects, `RunningActivityRow`,
    `InterruptionOverlay`, `MessageInput`, actor resolution.
  - Replace the `VirtualItem` union + `renderItem` switch (`:178-413`) so each virtual item
    is one `TranscriptSegment` rendered via `renderSegment(seg, { expandable: true })`.
    `running` / `empty` / `bottom-spacer` / `placeholder` stay as local non-segment items
    (they are scroll-affordances, not transcript content).
  - Remove now-unused locals: `AssistantTimelineRow`, `ActivityToolCallRow`,
    `CollapsedToolCallGroup`, `CompactToolCallRow`, `ThinkingStrip`, `StatusTimelineRow`,
    `RunContextDisclosure`, `partitionTurnSegments`, `categorizeToolCall`, `toolChrome`
    (moved to shared), `toolCallTimelineKey`. Keep `ApplyPatchDiff`, `PublishedToolPreviewCard`,
    `CollapsibleCodeBlock`, `CollapsibleMarkdown` (referenced by shared renderers — either
    move to shared or import from here).
  - `user` and `review_decision` bubbles move into `segmentRenderers` but render only when
    those kinds are in scope (slider passes them; dock does not).

### Shared deps (leave in place, import from both)
- `toolCallPresentation.ts` (`describeToolCall`) — unchanged.
- `ApplyPatchDiff.tsx`, `PublishedToolPreviewCard.tsx`, `MarkdownContent.tsx` — unchanged;
  imported by `segmentRenderers`.

## 6. Decisions & rationale

- **Expand-on-click, not one-line-only.** The slider is where `apply_patch` diffs get
  reviewed; a pure one-liner would lose them. Collapsed == dock look; expanded == slider
  depth. Dock forces collapsed via `expandable={false}`.
- **Drop tool grouping for v1.** With every tool already one line, 8 reads = 8 tidy lines,
  not a wall — `CollapsedToolCallGroup` is mostly redundant. Re-introduce only behind a high
  threshold (>12 consecutive) if long runs feel noisy. Tracked as a follow-up, not v1.
- **Drop avatars + connector rails.** Per design direction. Assistant turns render as plain
  markdown; tool/reasoning/status/context render as flat one-line rows.
- **Dock stays lean.** Scope = `assistant + tool` (+ optional `reasoning`). Status, context,
  user, and review-decision segments are slider-only.

## 7. Migration order

1. Land `segments.ts` + `TranscriptRow.tsx` + `segmentRenderers.tsx` + `toolRowChrome.ts`
   with unit tests, **without wiring** either surface.
2. Migrate the dock (`DockTranscript.tsx`) — small surface, easy to eyeball. Verify
   `ExecutionStrip` still renders one-shot results identically.
3. Migrate the slider (`CodingTranscriptPane.tsx`) behind the same renderers; keep the
   container/virtualizer/composer/overlay.
4. Delete dead code from the slider.

## 8. Testing

- **New unit tests**
  - `segments.test.ts`: turn_segments interleaving, fallback path, `update_plan` filtering,
    live inclusion/exclusion on active vs terminal, `include` scoping, leading context.
  - `segmentRenderers.test.tsx`: tool row label + collapsed/expanded body; reasoning
    one-liner expands; `expandable={false}` never renders children.
- **Preserve existing tests** (must stay green, may need selector updates):
  - `CodingTranscriptPane.test.tsx` — note assertions that lock current DOM:
    running row + `data-agent-working-spinner` / `data-agent-running-halo` (`:300-303`),
    activity spacer height (`:175`), approval actor label (`:382`), long-user-message expand
    control (`:412-419`), composer focus ring (`:226`). The running row, spacer, composer,
    and interruption panel are **unchanged**, so those should pass as-is. The expand control
    test may need a new selector if the user-bubble markup moves.
  - `CodingInterruptionPanel.test.tsx`, `CodingInteractionCard.test.tsx` — unaffected
    (interaction cards untouched) but run to confirm.
  - `toolCallPresentation.test.ts` — unaffected.
- **Build gate:** `cd frontend && npx tsc -b && pnpm build` (per `pm-types/CLAUDE.md`).
- **Manual:** one-shot agent in the dock (clean one-liners), then a multi-turn coding run
  in the slider (collapsed rows, click-to-expand diff, reasoning one-liner expands,
  status/context present).

## 9. Risks

- **Virtualizer measurement** — expandable rows change height on toggle; the slider already
  uses `virtualizer.measureElement` per row (`CodingTranscriptPane.tsx:435`), so dynamic
  height is handled, but verify expanded tool bodies remeasure (no clipping).
- **Test selector churn** — DOM for tool/assistant rows changes; expect to update a handful
  of `CodingTranscriptPane.test.tsx` selectors. Behavior assertions (running, composer,
  spacer, approvals) should not change.
- **Scope creep on `user`/`review_decision`** — these bubbles are slider-only and slightly
  bespoke; keep them in `segmentRenderers` but gated by `include` scope so the dock never
  pulls them in.

## 10. Out of scope / follow-ups

- Re-introduce threshold-based tool grouping if needed.
- Optionally let the dock show collapsed `reasoning` (one-liner) once the slider migration
  is proven.
