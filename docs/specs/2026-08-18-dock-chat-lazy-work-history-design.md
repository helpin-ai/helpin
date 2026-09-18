# Dock chat lazy work history design

> Historical design, reviewed against the checkout on 2026-09-17. This page
> explains the compact-history feature for contributors. The implementation
> below is current source evidence; the original design and verification list
> are not a claim that every proposed guarantee or test has been completed.

## Current implementation

The [history projection](../../server/internal/service/dock_chat_history_projection.go)
returns a synthetic assistant `status` message with
[`dock_work_summary`](../../server/internal/model/agent_run_message.go):
`message_id`, `duration_ms`, and `activity_count`. There is no `has_work_details`
field. The summary precedes the selected answer, whose turn segments and tool
invocations are omitted. Explicit progress without an explicit final answer is
preserved rather than compacted as completed work.

Final selection prefers the last nonempty `assistant_final`, even when tool
metadata arrives later. Otherwise it searches backward for assistant segment
content and then message content. This differs from a universal preference for
the durable message's content. The summary targets the last assistant message
in the group, which can differ from the visible final answer's ID.

The [service](../../server/internal/service/dock_chat.go) checks chat access for
both list and work-detail requests. The [repository query](../../server/internal/repository/agent.go)
scopes the target to the workspace and chat and loads the interval after the
preceding user message through that target, excluding failed deliveries. Work
detail can therefore contain multiple persisted messages. Compact projection
runs on each fetched page; the design should not be read as proof that pagination
always aligns with whole turns.

[DockTranscript](../../frontend/src/components/agents/dock/DockTranscript.tsx)
fetches on expansion, shows loading and retry states, and caches successful
responses in an in-memory map keyed by workspace, chat, and message. Collapsing
preserves that cache; “instant” means reuse of available cached data, not durable
offline storage. [ChatView](../../frontend/src/components/agents/dock/ChatView.tsx)
requests 50 persisted messages per page and refreshes after terminal events.
[AskAgentsDock](../../frontend/src/components/agents/AskAgentsDock.tsx) still
imports ChatView eagerly, so the separate bundle and smaller page remain future
work in this design.

## Original design

## Goal

Make completed main-chat history resemble Grok's compact transcript: show the user message, a collapsed `Worked for <duration>` row, and only the final assistant response. Historical reasoning, progress narration, tool calls, and delegated-work detail must not be transferred until the user expands the work row.

Active runs keep the existing realtime presentation unchanged.

## Scope

This behavior applies only to the main Ask Agents chat transcript in both the global dock and the Support Inbox embedded dock. Agent-run views, execution strips, and sub-agent transcript surfaces remain unchanged.

The separate `ChatView` bundle and smaller initial history page will follow after this compact-history foundation; they are not part of this change.

## Data contract

The normal dock message-list response returns a compact projection for completed assistant turns:

- the durable final assistant content;
- stable message and turn identifiers;
- work duration and a `has_work_details` signal;
- no historical turn segments or tool payloads.

User messages remain unchanged. Active-turn detail continues to arrive through the existing realtime stream.

A dedicated, chat-scoped endpoint loads the omitted working timeline for one completed assistant turn. It enforces the same workspace/chat access rules as the message-list endpoint and returns only detail belonging to the requested message. Missing or inaccessible messages return the existing API error conventions.

## Frontend behavior

Completed turns render in this order:

1. user message;
2. collapsed `Worked for <duration>` disclosure when work detail exists;
3. final assistant response.

Expanding the disclosure fetches work detail on demand. While loading, the disclosure shows an inline loading state. A failed fetch keeps the disclosure open with a retry action. Successful detail is cached by workspace, chat, and message so later expansions are instant.

The expanded timeline uses the existing transcript segment renderers and working-group presentation. Collapsing does not discard cached detail.

While an agent is working, the current live transcript remains visible. Completion switches to compact history only after the durable completed message is available, preventing the final response from disappearing during stream reconciliation.

## Final-response selection

The server derives the visible final response from the last completed assistant segment in the turn. If the durable message already has final content, that content is used. Progress messages never appear as separate completed-history responses. A direct answer without work detail renders normally and has no disclosure row.

## Testing

Backend tests cover compact projection, final-response selection, omission of work payloads, duration metadata, detail endpoint scoping, authorization, and empty/direct-answer turns.

Frontend tests cover completed compact rendering, unchanged live rendering, lazy fetch on first expansion, cache reuse, loading/error/retry states, and preservation of the final answer during completion reconciliation.

Verification includes focused Go and Vitest suites, TypeScript, Go build, and the production frontend build.
