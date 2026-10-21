# Customer portal AI replies implementation plan

Status: implementation in this branch, September 28, 2026. The portal setting, dispatch outbox, signed-in and confirmed-intake dispatch, customer reply path, manual AI handoff, and channel isolation are implemented. Private suggestions currently run for unassigned requests; assigned teammate requests remain human-owned without an automatic draft. The assigned-teammate draft path needs its own runtime ownership rules before it can be enabled safely.

## Problem and outcome

Portal requests enter the support inbox, but Echo never sees them. [Portal request creation](../../server/internal/service/customer_portal_requests.go) saves a conversation and publishes an inbox event without an AI request. [The AI channel rules](../../server/internal/model/support_ai_channels.go) and [the chat runner](../../server/internal/service/support_chat.go) accept chat or email, not portal. A workspace can therefore have Echo enabled for chat while portal requests wait for a human.

Let a support admin explicitly enable Echo for portal requests. Preserve the portal's ticket-like experience: requests remain durable inbox conversations, customers can ask for a person, and a human can take over at any time. Enabling portal AI must never silently change chat or email behavior.

## Product decisions

| Decision | Proposed behavior |
| --- | --- |
| Default | Portal AI is off, including on upgraded workspaces. Existing requests are not replayed when it is enabled. |
| Modes | `off`, `internal_note` (private suggestion for teammates), and `ai_first` (public reply in the portal). Do not create a public reply while in private mode. |
| Agent | Use the currently selected support agent by default. A portal-specific selection may choose any workspace agent validated for `support_conversation`. Creating a separate portal agent type is unnecessary. |
| Ownership | Auto reply runs only while the conversation is AI-owned and no teammate has taken it. Private suggestions leave the request in the human queue. |
| Anonymous intake | Do not run AI on an unverified, hidden request. If an eligible submitter later exchanges that request's confirmation link, evaluate it once after publication. Ineligible submissions stay human-owned and receive replies through the existing email path. |
| Customer notification | A public AI reply is visible in the portal and uses the same customer email-notification policy as a public teammate portal reply. Deduplicate notification by reply message ID. A failed notification does not hide the saved reply. |

The selected support agent may already be a custom agent with its own guidance and knowledge. The portal picker only changes which support-compatible agent handles this channel; it grants no new tools or customer permissions.

## Settings and interface

1. Add `portal_ai_mode` and nullable `portal_ai_agent_id` to the existing installation settings JSON, update request type, and frontend type. Missing mode resolves to `off`. A null agent ID means “Use support agent”; resolve it against the current support agent setting. Portal AI can be enabled independently of widget AI, but an effective agent must exist before either active mode can be saved.
2. Add an **AI replies** section to [Customer portal settings](../../frontend/src/components/settings/CustomerPortalSettings.tsx): three clearly described mode choices, an effective agent selector, and a link to edit the chosen support agent. Show why activation is unavailable when Agent Runtime, the selected agent, or required support capability is missing. The current settings PATCH remains guarded by `PermSupportAdmin`; enforce all validation server-side.
3. Show public AI messages as coming from the selected agent, with a clear AI label. While a request is processing, show a modest “working on your request” state; on handoff, show that the team will respond. Private suggestions and processing errors never appear in the customer portal.
4. Add a teammate action to ask Echo to handle an existing human-owned portal request. It requires support edit permission, checks current portal AI settings, and atomically transfers ownership from the human queue to the selected agent before queueing the latest eligible customer message once. Record who made the transfer. Reject the action if a teammate reply or another ownership change wins the race. Turning the setting on does not trigger this action automatically.

## Backend implementation

### 1. Resolve one policy for each portal turn

Add a portal-specific policy helper that returns effective mode, effective agent ID, and whether this request can enter AI processing. Use it at enqueue, at consumer start, before any public send, and in resume/follow-up paths. Validate the selected agent belongs to the workspace and supports `support_conversation`, as the current chat setting does in [settings validation](../../server/internal/service/support_inbox_settings.go).

Treat `portal` as its own channel in [AI channel classification](../../server/internal/model/support_ai_channels.go). Do not map it to chat just to pass the current `ai_reply_channels` filter. Refactor the runner's global `AIEnabled`, `AIAgentID`, and `AIResponseMode` checks into effective per-channel decisions. Audit resume, pause/return, escalation, follow-up, delayed send, and output-tool checks; each must use the same portal policy. Keep current widget and email defaults intact.

Pin the chosen agent when a conversation first enters AI handling. A later settings change affects new AI-owned conversations; an active run keeps its agent until handoff or explicit reassignment. An admin switching portal AI off prevents future turns and public sends, cancels or closes pending runs, and returns affected requests to the human queue. A run started in private mode must not become public merely because the setting changed while it was running.

### 2. Dispatch saved portal messages reliably

Create a small versioned SQL outbox for support AI dispatch, keyed uniquely by `(workspace_id, source_message_id)`. Store the conversation ID, channel, dispatch state, retry time, and attempt count. Insert the outbox row in the same transaction as the customer message or anonymous confirmation. A supervised worker publishes to the existing JetStream subject using `source_message_id` as the message ID and marks the row dispatched. Retry transient failures; after a bounded limit, leave the request with a human and record the failure. This closes the current gap where saving a request and publishing its AI event are separate operations.

For a signed-in request, return the first customer message ID from [CreateRequest](../../server/internal/repository/customer_portal.go) and enqueue it when the active mode permits. For a portal follow-up, enqueue the saved message after the same ownership checks. For anonymous intake, enqueue nothing on submission. On successful confirmation exchange, make the request visible and enqueue its first message only if it is still open, no human has answered or taken it, and the submitter is eligible. A second exchange or retry must not enqueue a second turn.

The AI consumer rechecks the saved message and conversation, current portal mode, visibility, contact eligibility, agent capability, human ownership, and whether the customer requested a person. It ignores stale or duplicate events. Preserve the existing `ai_message_processing` idempotency guard when the chat runner starts a turn.

### 3. Produce the right output

In `ai_first`, publish the answer as a public support message with `via_channel=portal`. The portal projection must show it, and the customer notification path must send at most one email for that message. Never route a portal answer through an email-only delivery mode. If the agent cannot answer confidently, asks for handoff, exceeds limits, or fails repeatedly, set `waiting_for_human` and notify the team through existing support events.

In `internal_note`, save a private suggestion and leave the request in the human queue. Give drafting its own ownership check so an assigned teammate may receive a suggestion without giving the AI public-send authority. Reuse the existing private AI message path after verifying it does not assign AI ownership or leak through portal response serialization. A teammate sends any public reply explicitly. An AI run started in private mode remains private even if the admin changes the setting before it finishes.

Before a public send, recheck that the portal remains enabled, the request is visible to its verified identity, the email remains portal-eligible, the mode still permits a public reply, and no human has taken over. If any check fails, suppress the send and return the conversation to a human. Keep Echo's existing turn caps, usage metering, and support-only tool permissions.

## Implementation order

1. Add setting types, defaults, validation, and the shared per-channel policy; cover chat/email parity with unit tests.
2. Add the SQL outbox, publisher, recovery sweep, and message-ID idempotency. Verify migrations in both editions and on an existing database.
3. Wire signed-in creation, customer follow-ups, and confirmed anonymous intake. Add the manual teammate action for an existing request.
4. Extend the Echo consumer, agent selection, private/public output, ownership controls, and send-time checks for portal conversations.
5. Add portal settings UI, request-detail status, AI attribution, and customer notification behavior.
6. Run a workspace-limited trial before enabling auto reply broadly. Leave the default off.

## Acceptance checks

- With portal AI off, new requests and replies stay in the human queue; chat and email AI behavior is unchanged.
- An `ai_first` request from a signed-in, eligible customer produces one visible Echo reply and one notification attempt. A follow-up starts one further turn while AI owns the conversation.
- `internal_note` produces one private suggestion, no public message, no customer notification, and no AI ownership of the request.
- Anonymous intake produces no AI event before confirmation. Confirmation queues one turn only when the submitter is eligible and the request is still untouched by a human. Ineligible intake remains human-owned.
- Human assignment, takeover, a request for a person, portal disablement, contact blocking, or mode `off` prevents a pending public reply. Mode `internal_note` never becomes public mid-run.
- Changing the selected agent does not silently replace the agent on an active run. Missing/deleted/non-support agents cannot be enabled or used.
- Duplicate events, repeated confirmation, worker restart, and a publish failure yield at most one AI turn or public reply. Exhausted failures leave an actionable human request.
- A customer never sees private notes, email-only messages, draft content, internal metadata, or another customer's request. A public reply is visible after reload and uses the same request reference.
- API permission tests cover portal settings and the manual teammate action. Frontend tests cover loading, disabled state, autosave, and customer-facing status. An end-to-end test submits a portal request, receives an AI reply, follows up, and takes over as a human.

## Rollout and limits

Ship settings and processing with `off` as the default. Observe dispatch backlog, AI starts, public answers, private suggestions, handoffs, skipped events, notification failures, and duplicate suppression by channel. Trial `internal_note` first, then `ai_first` in one test workspace. Only new messages are eligible; the current human-owned test request needs the explicit teammate action.

This plan does not add company-wide request visibility, arbitrary agent tools, AI replies to unverified anonymous intake, or SMTP support replies. It does not change portal access control or the existing email ingestion policy.
