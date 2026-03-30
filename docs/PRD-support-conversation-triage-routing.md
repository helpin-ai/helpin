# PRD: Support Conversation Triage And Inbox Routing

**Status:** Draft → Ready for implementation  
**Version:** v1.2 (solidified)  
**Date:** 2026-03-29  
**Owners:** Support, AI Platform, Frontend  
**Primary areas:** 
- `server/internal/model/support_inbox.go` (triage types, mailbox routing prompt)
- `server/internal/service/support_inbox_triage.go` (new)
- `server/internal/service/support_mailbox.go` (reuse `MoveConversation`)
- `server/internal/service/support_inbox_widget.go` (triage before `ai_first`)
- `server/internal/service/email_fallback.go` (triage after inbound creation)
- `server/internal/repository/` (new triage + rules repositories)
- `server/internal/llm/` (new triage classifier)
- `server/internal/service/support_inbox.go` + `email_fallback.go`
- `frontend/src/components/settings/ChatGeneralTab.tsx` + new routing tab
- `frontend/src/components/support/MessageThread.tsx` (suggestion banner)

---

## 1. Context

We now support:

- Shared Inbox plus Team Inboxes
- mailbox-specific access and assignment behavior
- widget default mailbox routing
- AI handoff mailbox routing
- manual conversation moves between inboxes

What we do not support yet is intent-based triage.

That means obvious non-support conversations like:

- guest post submissions
- partnership outreach
- affiliate requests
- press inquiries
- sales requests
- billing questions

still enter the shared support flow unless a human manually moves them.

This is operationally expensive and creates poor ownership boundaries. A workspace may already have a `Marketing`, `Sales`, `Partnerships`, or `Billing` inbox, but the system does not currently decide which one should receive a new conversation.

This PRD defines a new triage layer that classifies conversations and routes them into the right inbox, while preserving the current mailbox, assignment, and AI-reply architecture.

---

## 2. Problem Statement

Today the product has mailbox primitives but no mailbox intelligence.

Current behavior:

- widget conversations route to an explicit mailbox or workspace default mailbox
- email conversations route by email forwarding route mailbox
- internal conversations route only to the mailbox explicitly chosen by the agent
- AI support focuses on answer, clarify, and handoff, not on support-vs-marketing-vs-sales intent detection

As a result:

- the system cannot identify obvious non-support conversations automatically
- the shared inbox becomes the catch-all queue
- teams must manually move conversations after reading them
- routing knowledge is trapped in people rather than product behavior

---

## 3. Goals

1. Route inbound conversations to the most relevant configured inbox when confidence is high.
2. Suggest a relevant inbox when confidence is medium, without forcing movement.
3. Keep the existing Shared Inbox and Team Inbox model intact.
4. Reuse current mailbox assignment behavior after a move.
5. Support all conversation entry channels:
   - widget
   - email
   - internal/API-created conversations where applicable
6. Keep the system workspace-configurable and predictable.
7. Capture audit and feedback signals so routing can improve over time.

---

## 4. Non-Goals

- Replacing the current support AI answer-generation flow
- Inventing inboxes dynamically with AI
- Letting AI assign arbitrary teammates directly as the primary routing path
- Delaying widget or email intake until classification completes
- Full autonomous workflow automation beyond routing suggestion and move

---

## 5. Current Codebase Review

### 5.1 Mailbox primitives already exist

The codebase already has a proper mailbox model:

- `SupportMailbox` and `SupportMailboxMembership` in `server/internal/model/support_inbox.go`
- mailbox CRUD and member handling in `server/internal/service/support_mailbox.go`
- admin UI in `frontend/src/components/support/TeamInboxDialog.tsx`

Important behavior already exists:

- inbox access control
- linked teams
- manual vs round-robin assignment
- explicit move between inboxes

This means we do not need a new inbox concept.

### 5.2 Routing exists, but only in narrow forms

Current routing is limited to:

- explicit mailbox choice
- workspace default mailbox
- AI handoff mailbox
- inbound email route mailbox

Relevant code:

- `maybeApplyMailboxRouting()` in `server/internal/service/support_mailbox.go`
- support widget settings in `server/internal/service/support_inbox_settings.go`
- widget conversation creation in `server/internal/service/support_inbox_widget.go`
- email inbound creation in `server/internal/service/email_fallback.go`

This is not intent-based routing. It is only static mailbox selection.

### 5.3 Manual move already exists in the inbox UI

Agents can already move a conversation from the thread view:

- `Move to {Inbox}` in `frontend/src/components/support/MessageThread.tsx`
- backend action in `MoveConversation()` in `server/internal/service/support_mailbox.go`

This is important. The best triage UX should build on top of this existing move action rather than inventing a second move system.

### 5.4 The current support AI is not the right abstraction for inbox routing

The support AI service in `server/internal/service/support_ai.go` is designed for:

- answer
- clarify
- handoff
- RAG over docs/content
- confidence-based escalation

It is not designed to classify business intent across inboxes like:

- `marketing_guest_post`
- `sales_pricing`
- `billing_refund`
- `support_bug`

We should reuse the same LLM provider infrastructure, but not overload the existing answer-generation service with inbox triage responsibilities.

### 5.5 Settings already have a natural home for this feature

Support settings already expose:

- default inbox
- AI handoff inbox
- handoff behavior

in `frontend/src/components/settings/ChatGeneralTab.tsx`.

There is also already a settings section entry:

- `chat-ai` in `frontend/src/lib/settingsSections.ts`

This gives us a natural place to put routing configuration.

### 5.6 There is a real AI ordering risk today

The current widget `ai_first` flow publishes the AI request immediately after the first inbound customer message in `server/internal/service/support_inbox_widget.go`.

That means triage cannot be specified vaguely. The product must define whether triage runs:

- before AI request publication
- after AI request publication
- or in parallel

For this codebase, the correct behavior is:

- rules evaluate first
- auto-move happens first if rules or high-confidence triage require it
- only then can `ai_first` continue

Otherwise the product will race between:

- mailbox movement
- `assigned_agent_id`
- `flow_state`
- the first AI reply

---

## 6. Product Principles

### 6.1 Hybrid, not AI-only

Routing should be:

- rules first
- AI second
- human override always available

Rules should catch obvious patterns like:

- `write for us`
- `guest post`
- `partnership`
- `affiliate`
- `press`
- `demo`
- `pricing`
- `refund`
- `invoice`
- `bug`

AI should handle ambiguous or long-form inquiries after rules fail to produce a confident result.

### 6.2 AI can only choose from configured inboxes

The AI must never invent:

- an inbox that does not exist
- a category that has no configured destination

It should choose only from:

- Shared Inbox
- active Team Inboxes visible in workspace configuration

### 6.3 Triage is separate from assignment

This feature should distinguish between:

- which inbox should own the conversation
- which teammate should handle the conversation

Mailbox routing decides queue ownership.

Existing assignment behavior should remain the source of truth after a move:

- manual
- round robin
- team-based handoff behavior

### 6.4 Triage must not block intake

For widget and email:

- create the conversation first
- classify asynchronously after the first customer message
- move or suggest afterward

The system should never make the visitor wait for routing.

### 6.5 Detection and routing should stay separate

Intercom Fin uses a separation between:

- detection logic
- post-escalation routing and workflow actions

That is the right principle here too.

Helpin does not need a full workflow builder for v1, but it should still separate:

- triage detection
- routing policy
- mailbox movement

That gives us a simpler version of the same pattern:

- triage service decides what the conversation appears to be
- routing policy decides whether to auto-move or suggest
- `MoveConversation()` performs the actual move

---

## 7. User Experience

### 7.1 Admin UX

#### Settings IA

The best place for this is a visible support settings page named:

- `Conversation Routing`

This should live alongside:

- `Chat Widget`
- `Team Inboxes`
- `Email Forwarding`

This page should not be widget-only. Routing applies to support intake more broadly.

#### Routing settings page

The page should include:

1. `Enable AI triage`
2. `Auto-move high-confidence conversations`
3. `Confidence threshold`
4. `Run triage for`
   - widget
   - email
   - internal/API-created conversations
5. `Fallback behavior`
   - keep in shared inbox
   - keep in default inbox
6. `Re-run triage when conversation meaning changes`
   - off by default in v1

ASCII sketch:

```text
+----------------------------------------------------------------------------------+
| Conversation Routing                                                        [x] |
+----------------------------------------------------------------------------------+
| [ ] Enable AI triage                                                            |
|     Classify new conversations and route them to the most relevant inbox.       |
|                                                                                  |
| [ ] Auto-move high-confidence conversations                                      |
|     Otherwise, agents will see a routing suggestion in the thread.              |
|                                                                                  |
| Confidence threshold                                                             |
| [ 0.90 --------------------------------------------------------------- ]  90%    |
|                                                                                  |
| Run triage for                                                                   |
| [x] Widget conversations                                                         |
| [x] Email conversations                                                          |
| [ ] Internal/API-created conversations                                           |
|                                                                                  |
| Fallback when no rule or confident AI match exists                               |
| (•) Keep in Shared Inbox                                                         |
| ( ) Keep in Default Inbox                                                        |
|                                                                                  |
| Cost guardrails                                                                  |
| Daily AI triage budget per workspace                                             |
| [ 250 ] LLM classifications / day                                                |
| [x] Skip spam conversations                                                      |
| [x] Deduplicate repeated identical first messages                                |
+----------------------------------------------------------------------------------+
| Rules                                                                            |
| Priority | When                                                                  |
| 1        | text contains "write for us"                       -> Marketing        |
| 2        | text contains "refund"                             -> Billing          |
| 3        | domain = agency.com AND text contains partnership  -> Marketing        |
|                                                                    [+ New Rule]  |
+----------------------------------------------------------------------------------+
| Inbox routing catalog                                                            |
| Inbox        Eligible   Routing prompt                                            |
| Shared       [x]        General support and uncategorized conversations           |
| Marketing    [x]        Guest posts, partnerships, PR, affiliates                 |
| Sales        [x]        Pricing, demos, procurement, enterprise buying           |
| Billing      [x]        Refunds, invoices, payment issues                         |
+----------------------------------------------------------------------------------+
```

#### Inbox routing catalog

Each active inbox should show:

- inbox name
- description
- routing prompt
- whether it is eligible for AI triage

Example:

| Inbox | Routing prompt | Eligible |
|------|--------------|----------|
| Shared Inbox | General support and uncategorized messages | Yes |
| Marketing | Guest posts, partnerships, PR, affiliates | Yes |
| Sales | Pricing, demos, enterprise, procurement | Yes |
| Billing | Refunds, invoices, payment issues | Yes |

The mailbox `description` field should remain human-facing.

V1 should add a dedicated mailbox field:

- `routing_prompt`

This keeps inbox descriptions readable for admins while giving the classifier precise destination guidance.

#### Rule builder

V1 rules should be lightweight:

- phrase contains
- email domain equals
- channel equals
- mailbox destination
- priority
- active / inactive

Example rules:

- if text contains `write for us` -> `Marketing`
- if text contains `refund` -> `Billing`
- if sender domain equals `agency.com` and text contains `partnership` -> `Marketing`

This should be simple, not a full automation builder.

Rules should be:

- workspace-scoped
- admin-managed
- first-match-wins
- evaluated by ascending `priority`

### 7.2 Agent UX

#### Thread-level routing suggestion

The best agent UX is a compact suggestion strip in the thread header area.

Example:

```text
+--------------------------------------------------------------------------------+
| Suggested inbox: Marketing                                                     |
| Reason: guest post / write-for-us inquiry detected                             |
| Confidence: 94% · Source: AI triage                                            |
|                                                                    [Move] [x]  |
+--------------------------------------------------------------------------------+
```

Behavior:

- shown when AI or rules produce a suggestion but no auto-move happened
- `Move` uses the existing move conversation action
- `Dismiss` suppresses the suggestion for that conversation
- if the agent manually moves elsewhere, that counts as correction feedback

#### Auto-move behavior

If auto-move is enabled and confidence is high:

- move the conversation immediately
- show a system event in the thread
- show a toast to the agent if they are viewing the thread

Example system event:

- `Conversation moved to Marketing by AI triage`

#### Assignment suggestion

Teammate suggestion should be secondary.

If the inbox has:

- round robin
- linked members
- clear ownership

then mailbox movement is enough for v1.

Later, we may add:

- `Suggested teammate: Sarah`

but that should not block the inbox-routing release.

Thread header placement sketch:

```text
+----------------------------------------------------------------------------------+
| #1428                                                                [Resolve]  |
| Suggested inbox: Marketing                                          [Move] [x]  |
| Reason: guest post / write-for-us inquiry detected                               |
+----------------------------------------------------------------------------------+
|                                                                                  |
| conversation thread                                                              |
|                                                                                  |
+----------------------------------------------------------------------------------+
```

### 7.3 Customer UX

Customers should not see routing mechanics.

They should only experience:

- fast intake
- correct human handoff when needed
- the right team eventually responding

No customer-facing copy about `marketing inbox` or `routing rule` is needed.

---

## 8. Functional Requirements

### 8.1 Triage outputs

For each eligible conversation, triage should produce:

- active triage state
- historical triage record
- feedback state

The active triage state should include:

- `status`
  - `not_run`
  - `suggested`
  - `auto_moved`
  - `dismissed`
  - `overridden`
- `intent`
- `confidence`
- `suggested_mailbox_id`
- `reason`
- `classifier_source`
  - `rule`
  - `ai`
- `evaluated_at`
- `locked_at`
- `feedback_action`
  - `accepted`
  - `dismissed`
  - `corrected`

### 8.2 Channels

V1 triage should support:

- widget:
  - after first customer message
- email:
  - after conversation creation from inbound email
- internal/API-created conversations:
  - suggestion-only by default

Spam conversations must skip triage entirely.

### 8.3 Auto-move conditions

Auto-move should happen only if all are true:

- triage is enabled
- confidence >= threshold
- suggested mailbox exists and is active
- conversation is still in Shared Inbox or workspace default inbox
- no human has manually reassigned the conversation yet
- no human has already replied

Otherwise:

- show suggestion only

### 8.4 Triage and AI ordering

The ordering must be explicit.

For widget conversations:

1. persist the conversation and first customer message
2. run deterministic triage rules
3. if a rule triggers high-confidence auto-move, move first
4. if no rule move occurs and AI triage is enabled, run AI classification
5. if AI classification triggers auto-move, move first
6. only after triage routing settles may the existing `ai_first` support reply path continue

This ensures:

- mailbox selection is final before first AI reply
- `assigned_agent_id` does not oscillate
- `flow_state` remains coherent

V1 should keep AI configuration workspace-level.

The product must not assume per-mailbox AI agents yet, because the current codebase does not model mailbox-specific AI agents.

### 8.5 Re-triage behavior

V1 should classify:

- on conversation creation for email
- on first inbound customer/widget message

V1 should not continuously re-triage every new message by default.

Optional later behavior:

- rerun if the conversation remains unowned and a later message changes intent materially

### 8.6 Permissions

The following actors may act on triage suggestions:

- owner
- admin
- agents who can access the conversation's current inbox

Dismissing or accepting a suggestion must follow the same conversation access rules as moving the conversation manually.

### 8.7 Feedback capture

The system should record:

- suggestion accepted
- suggestion dismissed
- suggestion corrected by moving to another inbox
- manual move without prior suggestion

This is needed for future quality tuning.

### 8.8 Cost guardrails

The system must include:

- per-workspace daily AI triage budget
- skip for spam conversations
- skip if rules already matched
- skip duplicate first-message content bursts when possible
- optional cache for identical normalized first messages within a short time window

---

## 9. Data Model

V1 should use dedicated triage tables, not flat `triage_*` columns on `support_conversations`.

### 9.1 `support_conversation_triage`

One active row per conversation.

Recommended fields:

- `id`
- `workspace_id`
- `conversation_id`
- `status`
- `intent`
- `confidence`
- `reason`
- `source`
  - `rule`
  - `ai`
- `suggested_mailbox_id`
- `auto_moved`
- `locked_at`
- `evaluated_at`
- `created_at`
- `updated_at`

### 9.2 `support_conversation_triage_events`

Append-only history for:

- each triage evaluation
- accept
- dismiss
- correction
- auto-move

Recommended fields:

- `id`
- `workspace_id`
- `conversation_id`
- `triage_id`
- `event_type`
  - `evaluated`
  - `accepted`
  - `dismissed`
  - `corrected`
  - `auto_moved`
- `from_mailbox_id`
- `to_mailbox_id`
- `actor_user_id`
- `payload`
- `created_at`

### 9.3 `support_triage_rules`

Workspace-scoped routing rules.

Recommended fields:

- `id`
- `workspace_id`
- `priority`
- `active`
- `name`
- `channels`
- `conditions`
- `target_mailbox_id`
- `created_by_id`
- `created_at`
- `updated_at`

`conditions` can be JSONB in v1 for:

- phrase contains
- email domain equals
- channel equals

### 9.4 `support_mailboxes.routing_prompt`

Add a dedicated `routing_prompt` field on `support_mailboxes`.

Do not overload `description`.

Rationale:

- preserves triage history
- avoids bloating the already-large `SupportConversation` model
- keeps list payloads lighter and future re-triage easier
- lets us add analytics and audit trails cleanly

---

## 10. Backend Architecture

### 10.1 New service

Add a dedicated service, for example:

- `server/internal/service/support_inbox_triage.go`

Responsibilities:

- load eligible inboxes
- evaluate deterministic rules
- call LLM classifier when rules do not produce a confident result
- persist triage state
- optionally invoke conversation move
- emit websocket updates and audit events

This should not live inside the current answer-generation path in `support_ai.go`.

### 10.1.1 Industry reference

Intercom Fin separates:

- escalation detection
- post-escalation routing

Relevant references:

- [Manage Fin AI Agent's escalation guidance and rules](https://www.intercom.com/help/en/articles/12396892-manage-fin-ai-agent-s-escalation-guidance-and-rules)
- [Route conversations to a team inbox](https://www.intercom.com/help/en/articles/11868425-route-conversations-to-a-team-inbox)

Helpin should do the same in a smaller, codebase-native form:

- triage service decides what the conversation is
- routing policy decides what to do with that result
- mailbox movement remains centralized in `MoveConversation()`

### 10.2 LLM contract

The classifier should return structured output such as:

```json
{
  "intent": "marketing_guest_post",
  "target_mailbox_handle": "marketing",
  "confidence": 0.94,
  "reason": "Guest-post outreach detected from write-for-us submission language",
  "should_move": true
}
```

The classifier prompt must include:

- the conversation subject
- first customer message
- channel
- customer email/domain if present
- active mailbox options with names, handles, and routing prompts

### 10.3 Source of truth for movement

All actual mailbox changes should reuse:

- `MoveConversation()` in `server/internal/service/support_mailbox.go`

Do not duplicate mailbox movement logic in triage code.

This preserves:

- mailbox access rules
- owner recalculation
- round robin behavior
- websocket publishing

### 10.4 Trigger points

Recommended trigger points:

- widget:
  - `WidgetCreateMessage()` after first real customer message
- email:
  - after inbound conversation and first message are created in `email_fallback.go`
- internal conversation creation:
  - optional suggestion path after first external message is added

### 10.4.1 Rule evaluation semantics

Rules should be evaluated:

- only against active rules
- only for the conversation channel in scope
- in ascending priority
- first match wins

If a rule matches:

- no LLM call is made
- the rule result becomes the triage result

### 10.4.2 AI ordering with widget `ai_first`

Current code publishes `ai_first` requests immediately after the first widget message.

V1 triage must change this order:

- widget conversation/message persist
- rules evaluate
- AI triage evaluates if needed
- auto-move executes if needed
- only then may `supportAIService.PublishAIRequest(...)` run

If triage ends in suggestion-only:

- current AI flow may proceed normally

If triage ends in auto-move:

- current AI flow proceeds only after the move completes

### 10.4.3 Guardrails

The service should enforce:

- daily LLM triage budget per workspace
- spam gating
- duplicate-first-message suppression
- short-term cache for identical normalized classification inputs
- structured logs for rule hit, AI hit, auto-move, dismiss, correction, and budget exhaustion

### 10.5 Websocket behavior

When triage changes a conversation:

- publish the updated conversation
- invalidate or refresh inbox lists as current support events already do

V1 should not add a separate websocket event type.

Instead:

- include triage summary in the normal conversation payload
- reuse the existing `support_conversation updated` event path

This keeps frontend state changes aligned with the existing conversation-sync model.

---

## 11. Frontend Architecture

### 11.1 Settings

Add a dedicated support routing settings page or expand the current hidden `chat-ai` section into a visible page.

The settings page should expose:

- triage enabled
- auto-move enabled
- threshold
- eligible inboxes
- rule definitions
- channel toggles
- budget settings

### 11.2 Thread suggestion UI

Add a banner in `frontend/src/components/support/MessageThread.tsx` for:

- `suggested`
- `auto_moved`

The banner must include:

- suggested inbox
- reason
- confidence if useful
- action buttons
- source badge if useful

### 11.3 Conversation list

Do not overload the row UI.

At most, show a subtle badge like:

- `Suggested: Marketing`

only when the suggestion is pending and high-signal.

The conversation list should not become a routing dashboard.

---

## 12. Suggested UX Copy

### Settings

- `Enable AI triage`
- `Automatically classify and route new conversations to the most relevant inbox.`

- `Auto-move high-confidence conversations`
- `When disabled, agents will still see routing suggestions in the thread.`

- `Routing prompt`
- `Describe what this inbox should receive so AI triage can choose it correctly.`

### Agent banner

- `Suggested inbox: Marketing`
- `Reason: guest post / write-for-us inquiry detected`
- `Move`
- `Dismiss`

### Thread event

- `Moved to Marketing by AI triage`
- `Routing suggestion dismissed`
- `Moved to Sales by John Doe`

---

## 13. Rollout Plan

### Phase 1: Rules + Manual Suggestion

- add triage state model
- add triage tables
- add deterministic rule evaluation
- add routing prompt on inbox
- show thread suggestion banner
- no AI yet
- no auto-move yet

### Phase 2: AI Suggestion

- add LLM-based classifier fallback
- choose only from configured inboxes
- keep suggestion-only behavior for medium and high confidence

### Phase 3: Auto-Move High Confidence

- enable optional auto-move
- default off
- guard by channel and human-touch conditions

### Phase 4: Feedback And Quality Tuning

- capture accept/dismiss/correct events
- improve rule suggestions
- refine prompts and thresholds

### Phase 5: Optional Teammate Suggestion

- suggest teammate after mailbox selection
- only if it can reuse existing assignment semantics cleanly

---

## 14. Metrics

Track:

- `% conversations triaged`
- `% auto-moved`
- `% suggestions accepted`
- `% suggestions dismissed`
- `% suggestions corrected to another inbox`
- time-to-first-human-response by inbox
- shared inbox volume reduction
- routing precision for high-confidence auto-moves

Instrumentation should be emitted as structured backend events or analytics records with:

- workspace_id
- conversation_id
- triage source
- suggested mailbox
- final mailbox
- feedback outcome
- whether an LLM call was made
- budget consumed

---

## 15. Risks

### False positives

Risk:

- moving a real support issue into the wrong specialist inbox

Mitigation:

- rules first
- high-confidence threshold
- suggestion-only default
- do not auto-move after human engagement

### Overloading the current support AI

Risk:

- mixing answer-generation and routing logic into one service creates brittle prompts and unclear ownership

Mitigation:

- separate triage service
- shared LLM infrastructure only

### Admin confusion

Risk:

- teams do not understand why AI routes somewhere

Mitigation:

- explicit reasons in UI
- audit events
- clear routing prompts

### LLM cost growth

Risk:

- high-volume workspaces or spam waves create unnecessary triage cost

Mitigation:

- rules first
- spam skip
- daily workspace budget
- duplicate suppression
- cache repeated inputs

### Multi-language rule mismatch

Risk:

- English-only rules miss non-English conversations and push more traffic to the LLM

Mitigation:

- accept that AI will absorb multilingual cases in v1
- add localized rules later where needed

---

## 16. Open Questions

1. Should internal conversations created directly by agents ever auto-move, or only suggest?
2. Should dismissed suggestions permanently lock routing for that conversation?
3. Should email-domain rules be included in v1 or saved for v1.1?
4. How much of the triage budget should be admin-configurable vs fixed defaults?

---

## 17. Recommendation

The best implementation for Helpin is:

- keep the current Shared Inbox and Team Inbox architecture
- add a dedicated conversation-triage layer
- store triage in dedicated tables
- use rules first and AI second
- add mailbox `routing_prompt`
- route only into configured inboxes
- run triage before widget `ai_first` publishes its first AI request
- reuse `MoveConversation()` as the single movement path
- keep teammate assignment as a separate concern

This is the smallest solution that fits the current codebase well and solves the real operational problem without turning support AI into an unbounded router.
