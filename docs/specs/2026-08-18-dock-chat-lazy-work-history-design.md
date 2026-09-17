# Dock Chat Lazy Work History

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
