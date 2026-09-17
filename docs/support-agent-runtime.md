# Support execution through Agent Runtime

Current implementation reference for the Helpin support backend. This describes
code ownership, not deployment status.

## Live conversation path

1. Helpin persists the customer message and publishes a `SUPPORT_AI` JetStream
   request. The consumer runs in the API process and calls
   `SupportChatService.HandleVisitorMessage`.
2. Helpin checks workspace/channel settings, conversation ownership and AI
   control, message idempotency, conversation locking, explicit human requests,
   turn limits, and usage budgets before starting or resuming an `agent_run`.
3. Agent Runtime executes the support agent as a chat-mode run. It owns the
   model/tool loop and execution state. Messages arriving during an active turn
   are deferred and drained through the pause/reconciliation lifecycle.
4. Runtime calls Helpin tools for product data and mutations. `search_knowledge`
   retrieves workspace-scoped evidence. `send_support_reply` checks the stored
   evidence, numeric grounding, confidence and recent confidence trend before
   publishing. `escalate_to_human` applies Helpin's handoff and routing rules.
5. Reply publication and turn settlement are atomic. Human takeover revokes
   delivery rights even if Runtime cancellation needs a retry. Runtime events
   are projected into Helpin's run records.

The API wiring is in [`server/cmd/api/main.go`](../server/cmd/api/main.go).
The Helpin worker still handles background jobs such as knowledge indexing; it
is not the support conversation executor.

## Code ownership

| Responsibility | Implementation |
| --- | --- |
| JetStream admission | [`support_ai_consumer.go`](../server/internal/service/support_ai_consumer.go) |
| Chat lifecycle and deterministic admission gates | [`support_chat.go`](../server/internal/service/support_chat.go) and adjacent `support_chat_*` files |
| Knowledge tools and persisted run evidence | [`internal_command_support_knowledge.go`](../server/internal/service/internal_command_support_knowledge.go) |
| Reply validation and delivery | [`internal_command_support_reply.go`](../server/internal/service/internal_command_support_reply.go), `support_ai_evidence.go`, `support_ai_confidence.go`, `support_ai_publish.go` |
| Human control and handoff | [`support_ai_control.go`](../server/internal/service/support_ai_control.go), [`support_ai_escalate.go`](../server/internal/service/support_ai_escalate.go) |
| Isolated Runtime previews | [`support_preview.go`](../server/internal/service/support_preview.go) |
| Scheduled follow-up runs | [`support_ai_follow_up.go`](../server/internal/service/support_ai_follow_up.go) and adjacent follow-up files |
| Teammate draft rewriting and task drafts | [`support_ai_admin.go`](../server/internal/service/support_ai_admin.go) |

`SupportAIService` remains an active collection of Helpin-owned collaborators:
retrieval, publication, handoff, admission utilities and composer assistance.
Its presence does not imply a second autonomous agent executor. Composer
rewriting, task-draft generation, knowledge query expansion and indexing can
still make bounded server-side model calls; they are separate from the support
conversation's reasoning loop.

## Retired implementation

The former in-process `generateResponse*` chain, JSON answer parser/schema,
answer-prompt builder, planner normalizers and conversation-state builder have
no live callers and have been removed. The pre-model repetition, frustration
and issue-stall heuristic chain was also disconnected from the Runtime path;
its tests did not demonstrate live protection. Current deterministic checks are
those in `SupportChatService` and the reply gate. Reply-turn counting, explicit
human-request handling, confidence validation and human-control fencing remain.

Persisted message metadata, retrieval trace contracts, migrations and current
API payloads are retained. Historical records may still contain planner-era
fields; cleanup does not rewrite those records or remove their schema.

The original [AI-first PRD](prds/PRD-ai-support-agent.md),
[native tool extraction proposal](plans/2026-04-24-support-native-tool-extraction-plan.md),
and [same-issue handoff PRD](prds/PRD-support-ai-stuck-detection-and-handoff.md)
are historical design references, not instructions to restore a second executor.

## Related behavior and checks

- [Support preview](support-ai-preview.md) documents isolation, metering and the
  retrieval-only option.
- [Human control](support-ai-human-control.md) documents pause/return, takeover
  races and deployment requirements.
- [Agents and automation](AGENTS_AND_AUTOMATION.md) describes the shared runtime
  contract.

Relevant service tests include `support_chat*_test.go`, `support_preview_test.go`,
`internal_command_support_reply_test.go`, `support_ai_control*_test.go`, and the
follow-up suites. Knowledge validation, composer rewriting and task-draft tests
exercise the retained server-side features. Validate Community and Enterprise
builds when changing these shared services.
