# Ask Agent efficient execution design

> Historical design, source-compared on 2026-09-17. This page explains the
> original Ask efficiency change for contributors. The product execution policy
> exists, but the transcript implementation and opt-in scope have since evolved.

## Current implementation

The [prompt contract](../../server/internal/agentcontract/skill_catalog.go) defines
`Required Ask Agent execution policy v2` and `EnsureAskAgentExecutionPolicy`.
It removes exact copies of the current contract before appending it, preserving
custom text while making the current product policy explicit. The
[agent launch path](../../server/internal/service/agent.go) applies it to the
effective prompt. Its smallest-action, evidence, stopping, communication, and
direct-execution rules are prompt guidance, not proof that every model response
obeys them. “Current flash model” below describes the original scope, not a fixed
model guarantee for all current AI profiles.

The [segment collector](../../frontend/src/components/agents/transcript/segments.ts)
retains a compaction helper, but
[DockTranscript](../../frontend/src/components/agents/dock/DockTranscript.tsx)
currently calls the collector with compaction disabled and then uses
[working groups](../../frontend/src/components/agents/dock/dockWorkingGroups.ts)
when its presentation option is enabled. It does not simply discard all but the
latest prose through the original pure transformation. Final-answer handling and
completed work disclosures are part of the current presentation path.

Both [ChatView](../../frontend/src/components/agents/dock/ChatView.tsx) and
[DockRunView](../../frontend/src/components/agents/dock/DockRunView.tsx) pass
`compactAssistantProgress`; therefore the original “only root Ask” call-site
boundary and “all other consumers unchanged” acceptance claim are outdated.
The default option remains false, but defaults do not establish actual caller
behavior. See the [reviewed timeline design](2026-08-15-ask-agent-timeline-ordering-design.md)
for further changes to disclosure defaults and label content.

The original no-runtime-change constraint describes this P0's intended scope,
not a guarantee that subsequent projection/history implementations stayed frozen.
No new model benchmark, runtime execution, browser suite, or build was performed
for this source comparison. The historical verification matrix follows.

## Original P0 design

## Goal

Keep Ask Agent on the current flash model as Helpin's broad autonomous workspace executor while reducing redundant exploration and transcript noise. Do not introduce request complexity classes, a new tool-step limit, dynamic tool loading, a model change, or a delegation-first architecture.

## Scope

The P0 changes two behaviors:

1. Replace Ask Agent's duplicated direct-execution guidance with one authoritative efficiency and evidence contract.
2. Keep progress streaming live while compacting Ask turns in the Dock presentation so routine narration does not appear as a series of chat bubbles.

## Prompt contract

Ask continues to execute work directly whenever its tools and permissions cover the request. It begins with the smallest targeted action that advances the requested outcome and reassesses after every tool result. Another action is justified only when it completes the request, resolves a material uncertainty, or verifies a conclusion whose correctness matters.

The contract prohibits exploration for completeness, equivalent searches, overlapping reads without new evidence, broad reads when exact identifiers or symbols are available, and unchanged retries. Investigations maintain one concrete question or hypothesis at a time. Ask stops when the requested outcome is achieved, evidence is sufficient, further evidence would not materially change the answer, or a capability boundary is reached.

Plans remain available only for distinct deliverables or dependencies and contain at most four outcome-oriented steps. Ask distinguishes confirmed findings, inferences, and unresolved questions. It must not claim static source inspection reproduced runtime behavior.

The contract also prohibits routine tool narration. Ask communicates mid-run only for a necessary user question, an approval, a material blocker, or a meaningful user-facing result.

Ask remains direct-first. Delegation remains limited to explicit user requests, unavailable capabilities, repository mutation or specialist review, and genuinely independent parallel work.

The product-owned launch contract is the authoritative copy. It has a versioned marker and is normalized and appended at every Ask launch rather than conditionally skipped because arbitrary stored text contains a section heading. Before appending, the launcher removes an exact copy of the current contract when present, making repeated assembly idempotent. A stale or custom prompt containing an older policy still receives the current product-owned contract, whose rules explicitly override conflicting workspace instructions.

The preset prompt describes Ask's role and retains page/reference handling, domain-specific tool prerequisites, approval behavior, repository read-only boundaries, orchestration mechanics, link formatting, and agent-result handling. Generic smallest-action, planning, investigation, stopping, narration, and delegation rules live only in the product-owned contract.

## Progress and transcript behavior

The governing rule is: if Ask can continue without the user responding, assistant prose is progress rather than a conversational turn.

Agent Runtime projection and persistence remain unchanged in this P0. This deliberately preserves the full auditable runtime record and avoids deletion, reconciliation, stale-run, concurrency, terminal-interaction, and partial-deployment races.

`DockTranscript` compacts the normalized presentation for both persisted and live segments:

- Within each conversational interval, show only the latest non-empty assistant prose segment.
- Preserve every tool segment, including tools attached to assistant messages whose prose is hidden.
- Treat user messages and review/approval decision rows as interval boundaries.
- The current live interval follows the same rule, so the latest status prose replaces earlier progress prose while tool activity continues to stream.
- A clarification or approval prompt remains the latest assistant prose while the run waits. After a user response or interaction decision creates a boundary, it remains part of the settled interval.
- Child launch confirmation remains visible while it is the latest response; a later result may supersede it in the same interval.
- Failed or cancelled runs use the same presentation rule and do not manufacture a final answer from backend state.

Compaction is a pure segment transformation controlled by an explicit `compactAssistantProgress` rendering option that defaults to `false`. Only the root Ask `ChatView` passes `true`. `DockRunView`, child `ExecutionStrip` instances, the coding-session slider, and other transcript consumers retain their current full presentation. The decision uses normalized segment kinds and boundaries, never phrase matching.

When that option is enabled, live-segment reconciliation must treat both `user` and `review_decision` segments as settled-turn boundaries. This boundary handling occurs before cumulative live segments are hoisted or deduplicated, ensuring a pre-approval assistant prompt stays before the approval row while resumed live output belongs to the next interval.

## Data flow

1. Runtime events continue through the existing snapshot, websocket, projection, reconciliation, and interaction paths without modification.
2. The existing transcript collector normalizes persisted and live events into ordered segments.
3. A pure presentation function divides segments at user and review-decision boundaries.
4. It retains all non-assistant segments and only the latest assistant prose within each interval.
5. Root Ask `ChatView` opts into the compacted `DockTranscript`; all other call sites use the default full list.

## Failure and recovery behavior

Runtime storage, recovery, terminal interaction ordering, and reconciliation retain their current behavior. Because compaction is derived at render time and does not mutate stored messages, refreshes, late events, successor runs, stale-run sweeps, and repeated reconciliation cannot leave a partially compacted transcript.

## Testing

Prompt tests assert the effective Ask prompt contains the smallest-action, material-uncertainty, evidence-labeling, stopping, and no-routine-narration rules. They also assert the preset prompt no longer duplicates those generic rules. Launch tests cover a current managed prompt, a stale prompt containing the old header, a custom prompt, and repeated effective-prompt assembly.

Pure frontend tests cover persisted and live intervals, tool preservation, multiple user turns, review/approval decision boundaries, clarification prompts, child launch confirmations, failed/cancelled presentation input, and default opt-out behavior. The approval case explicitly covers an assistant approval prompt, a projected approval decision, and resumed assistant output while the live timeline remains cumulative. Call-site tests assert only root Ask `ChatView` opts in while `DockRunView` and child `ExecutionStrip` presentations remain unmodified.

Dock integration tests assert the live-to-settled transition renders the same latest response and tool rows when live inclusion turns off. This prevents a final response from disappearing during the handoff from live state to persisted messages.

## Success criteria

- Ask remains capable of all currently supported direct workspace work.
- No new numerical tool-step limit is introduced.
- Every prompt-directed tool action must resolve a material part of the request.
- Routine narration remains auditable in storage but does not appear as repeated Ask chat bubbles.
- Live tool activity remains visible while Ask works.
- Final answers, user questions, approvals, blockers, child launch confirmations, and child results remain visible.
- Existing non-Ask agent transcripts are unchanged.
