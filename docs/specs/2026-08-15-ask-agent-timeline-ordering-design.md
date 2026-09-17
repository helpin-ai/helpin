# Ask Agent timeline ordering design

> Historical design, source-compared on 2026-09-17. This page explains the
> timeline reconciliation intent for contributors. Current ordering helpers retain
> the snapshot coverage checks, but several disclosure/presentation guarantees
> below no longer describe the UI.

## Current implementation and differences

The [timeline helpers](../../frontend/src/components/agents/dock/dockChatTimeline.ts)
compare retained assistant/tool IDs with the durable current interval and recognize
user and review/approval decisions as boundaries. They merge persisted messages
with a newer runtime tail and use stable/client IDs for reconciliation. The
[transcript](../../frontend/src/components/agents/dock/DockTranscript.tsx) separates
runtime chronology from active styling and computes final/progress presentation
and separators per interval. Source inspection does not prove exact runtime order
for every missing-ID, delayed-event, or malformed-timestamp scenario.

[Work groups](../../frontend/src/components/agents/dock/DockWorkingGroup.tsx)
start collapsed even when active. Manual toggling is retained across activity
changes; the design's automatically open running phase is not current behavior.
Completed groups may show “Worked for …” using a supplied duration, so the old
blanket prohibition on duration-derived headers is also outdated.

Active group labels use
[tool-call presentation](../../frontend/src/components/pm/CodingSession/toolCallPresentation.ts),
which can include argument-derived document titles, search text, file paths, or
repository context. Therefore the original identity-only/static-metadata label
rule and its sensitive-string test requirement are not guarantees of current
rendering. This is a presentation-contract difference, not evidence that an
unauthorized reader can access a transcript.

[ChatView](../../frontend/src/components/agents/dock/ChatView.tsx) manages persisted
and pending messages, and the shared timeline merge uses `client_message_id` for
user identity. The verification list below remains the historical intended test
matrix, not proof that every current transition or payload-hiding assertion passed.
No new browser, runtime, or production-build verification was run for this review.

## Original ordering design

## Goal

Keep the Ask agent timeline in the exact order produced by the runtime, before and after a run pauses or completes. Internal work should remain inspectable without competing visually with the assistant's final response.

## Timeline model

- The runtime snapshot's `live_turn_segments` is the authoritative ordering source while actively streaming. After pause/completion, it is authoritative only when it contains retained runtime work and fully covers the stable assistant-message IDs and tool-call IDs represented by the durable current conversational interval. Assistant-only direct-answer handoffs are valid; a tool segment is not required when the interval contains no tools.
- Durable chat messages remain authoritative for completed assistant and user content.
- Reconciliation matches snapshot assistant segments to durable messages by runtime message ID and snapshot tool segments to durable tools by tool-call ID.
- Snapshot chronology is used even when the run is paused or completed. Run activity controls only live styling and automatic open state.
- During a running-to-paused/completed transition, the retained snapshot remains renderable while durable message projection catches up. The UI must not temporarily remove progress or the final response, and must not add broad completion polling to compensate.
- Assistant messages are hard execution-phase boundaries. Each contiguous sequence of tool calls between assistant messages becomes an independent disclosure.
- Reasoning is separate from tool disclosures. Because `live_reasoning_message` has no authoritative position within `live_turn_segments`, it is placed only in the current conversational interval, immediately before the current runtime tail; chronology is not inferred after it disappears from the snapshot.
- A conversational interval starts at a user or review-decision boundary and ends before the next such boundary. When a run is queued/running, assistant prose in its current interval is progress. When the run pauses, completes, fails, or is cancelled, the last non-streaming assistant segment in that interval is the final response. That final response is always flat and outside work disclosures; earlier assistant prose remains muted progress.

## Presentation

- Final assistant responses, as structurally defined per conversational interval above, use normal high-contrast foreground text.
- When a conversational interval contains reasoning, progress, or tool activity before its final assistant response, render a subtle horizontal separator and additional spacing immediately before that final response. Direct-answer intervals with no preceding work do not receive a separator.
- Reasoning, work summaries, and other internal progress use muted foreground text.
- A running phase is open and may use the existing activity accent. Completed phases collapse automatically but remain independently expandable.
- A phase header summarizes the tool identities and hidden item count. Headers and tool labels may use only canonical tool identity, static presentation metadata, status, and count. They must never derive text from arguments, results, errors, timestamps, or durations. Its expanded body preserves tool execution order.
- Every disclosure places its expand/collapse chevron immediately after the label text rather than at the far edge. This applies to reasoning, tool-phase, and failed-tool error disclosures.
- Successful tool rows show only a concise tool identity/action and completion status. They are not interactive.
- Failed tool rows are expandable, remain collapsed by default, and reveal only the error text.
- Tool arguments, successful output, start timestamps, completion timestamps, and per-tool durations are not rendered.

## Data flow

1. Load durable chat messages and the run snapshot.
2. Retain snapshot assistant segments that match durable runtime message IDs, not only segments already embedded in durable `turn_segments`.
3. Evaluate retained snapshot trust. Active streams use their current tail. Paused/completed streams use snapshot chronology only when the current durable interval's assistant and tool IDs are completely covered by the retained snapshot; incomplete or unmatched snapshots fall back to durable order. An assistant-only snapshot remains eligible while the durable answer is catching up.
4. Collect runtime chronology independently from visual activity. The chronology flag controls reconciliation; a separate runtime-active flag controls streaming markers, caret/live styling, reasoning activity, and automatic expansion. Stale snapshot statuses cannot make a paused/completed transcript appear live.
5. Reinsert durable message content and tool results at their snapshot positions.
6. Build working groups from the resulting ordered segments.
7. Always treat user and review-decision segments as reconciliation boundaries.
8. Use queued/running status only for spinner, live accent, and automatic expansion.
9. Render `live_reasoning_message` as a separate muted disclosure immediately before the current runtime tail. Do not place it inside a tool phase or manufacture persisted ordering after the snapshot drops it.

When no trustworthy snapshot ordering exists, retain the current durable transcript fallback without inventing chronology from missing timestamps.

## Failure behavior

- A failed tool remains a compact row until clicked.
- Expanding it displays the error only.
- Missing or malformed errors produce a short unavailable-error fallback rather than exposing raw payloads.

## Optimistic user-message reconciliation

- A pending local user echo is visible only while no durable message has the same `client_message_id`.
- The visibility check is synchronous during render; it must not rely on a later effect or timer after the durable row arrives.
- This rule applies to both the first message in a chat and subsequent messages in an ongoing conversation.

## Verification

- Regression fixture matching production: durable progress messages, all tools aggregated on the final durable message, and a correctly interleaved snapshot.
- Assert multiple tool disclosures appear in execution order before and between progress messages.
- Assert a live render updated in place from running to paused/completed retains every progress segment and the final response before durable message refresh, while removing live styling.
- Assert a paused snapshot carrying stale `streaming` statuses renders no caret, Live label, or streaming reasoning treatment.
- Assert user and review-decision boundaries keep cumulative snapshot segments attached to the correct conversational interval.
- Assert incomplete or unmatched retained snapshots do not reorder the durable transcript.
- Assert the final response remains outside all disclosures after pause/completion.
- Assert final/progress styling across multiple user intervals: only the structurally final assistant response is high contrast; active and earlier assistant prose is muted.
- Assert the final-response separator appears only after preceding work in the same conversational interval, never before a direct answer or across a user-turn boundary.
- Assert paused runs have no spinner or Live label.
- Assert live reasoning stays muted and outside tool groups while running and paused; assert completed rendering remains correct when reasoning is absent.
- Assert successful tools cannot expand and expose no payload/timing data.
- Assert failed tools are collapsed by default and reveal only their error after a click.
- Assert headers and row labels do not contain distinctive sensitive strings supplied only in tool arguments, output, errors, timestamps, or durations.
- Assert disclosure chevrons render directly after their labels for reasoning, tool phases, and failed tools.
- Assert an accepted/persisted user message and its matching optimistic echo are never rendered together, in both empty and ongoing chats.
- Run dock/transcript component suites, changed-file lint, and the production frontend build.
