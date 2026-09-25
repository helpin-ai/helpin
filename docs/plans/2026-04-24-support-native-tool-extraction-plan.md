# Support Native Tool Extraction Plan

> **Historical / superseded execution design.** The proposal to retain a separate AI-first executor was superseded by the full Agent Runtime migration. Current support execution uses agent_run and Helpin-owned knowledge, delivery and handoff tools.
> See [Support execution through Agent Runtime](../support-agent-runtime.md) for current ownership.

## Status

Superseded by the Agent Runtime migration.

## Goal

Extract the most reusable, deterministic parts of the current AI-first support stack into native agent tools without converting the full Echo `ai_first` path to `agent_run` / `native_sdk`.

This plan is intentionally narrower than a full runtime migration. The purpose is to:

- make more of Echo's current behavior reusable by native support agents
- let system and custom agents participate in support workflows through stable tool contracts
- preserve current backend safeguards, routing logic, and product-specific orchestration where those should remain backend-owned

## Current Baseline

Today the support native profile already exposes a small tool set:

- `list_conversation_messages`
- `draft_support_reply`
- `update_conversation_status`

The richer AI-first support behavior still lives in `SupportAIService` and related support services, including:

- support query planning
- hybrid support knowledge retrieval
- confidence and escalation logic
- human handoff flow
- support triage routing
- support draft rewriting
- support task draft generation
- support conversation to PM task creation

That means there is already a substantial service layer we can reuse without changing the top-level Echo architecture.

## Direction

Do not try to turn every support behavior into a tool.

The first pass should separate:

- reusable model-facing capabilities
- authoritative business mutations
- backend-only orchestration and safeguards

Recommended boundary:

- `runtime-only tool`
  - read/query behavior
  - dry-run analysis
  - helper generation that does not mutate product state
- `command-backed tool`
  - durable product mutations
  - support and PM changes that must preserve invariants
- `backend-only orchestration`
  - dedupe
  - locking
  - token-budget enforcement
  - assumed-resolution scans
  - internal state-machine plumbing that should not be model-directed directly

## Proposed Tool Set

### First-wave cross-agent shortlist

These four have the strongest reuse value outside live support chat itself:

1. `plan_support_response`
2. `search_support_knowledge`
3. `generate_support_task_draft`
4. `create_task_from_conversation`

The remainder of this section covers the broader support-native surface, but these four are the ones to prioritize for the next implementation pass.

### Runtime-only tools

These should be exposed as normal LLM-facing tools and should not be internal-command-backed by default.

#### 1. `plan_support_response`

Purpose:
- expose the current support query planner as a reusable decision step

Backed by:
- `SupportAIService.planSupportQuery(...)`

Why now:
- this is the clearest extraction point from Echo's internal loop
- it lets a native agent decide whether to answer, clarify, or hand off using the same planner logic Echo already uses

Suggested input:

```json
{
  "message": "customer message text",
  "history": [
    {
      "sender_type": "customer",
      "message_type": "reply",
      "content": "..."
    }
  ]
}
```

Suggested output:

```json
{
  "decision": "answer",
  "issue_key": "password_reset",
  "issue_summary": "Customer needs help resetting their password",
  "progress_signal": "new_issue",
  "standalone_query": "resetting password in helpin",
  "search_queries": ["helpin password reset", "reset password email"],
  "clarifying_question": "",
  "reason": "resolved_from_context"
}
```

#### 2. `search_support_knowledge`

Purpose:
- expose support knowledge retrieval without forcing the model to rely on generic docs tools

Backed by:
- `SupportAIService.loadKnowledgeChunks(...)`

Why now:
- this preserves current hybrid retrieval quality
- it gives native support agents the same retrieval substrate Echo uses today

Suggested input:

```json
{
  "queries": ["helpin password reset", "reset password email"],
  "max_results": 6
}
```

Suggested output:

```json
{
  "results": [
    {
      "reference_id": "docs:doc-123",
      "source_type": "docs",
      "title": "Reset your password",
      "url": "https://...",
      "chunk_index": 0,
      "combined_score": 0.92,
      "vector_score": 0.84,
      "lexical_score": 0.31,
      "snippet": "..."
    }
  ]
}
```

Notes:
- keep the result shape close to `SupportAIPreviewSearchResult`
- allow the support preset to rely on one canonical support retrieval tool instead of composing multiple lower-level docs/content tools

#### 3. `preview_support_reply`

Purpose:
- dry-run the current support AI planner + retrieval + answer pipeline without side effects

Backed by:
- `SupportAIService.PreviewSupportReply(...)`
- `model.SupportAIPreviewRequest`
- `model.SupportAIPreviewResponse`

Why now:
- this is already productized and stable
- it is the easiest parity bridge between Echo behavior and native support runs

Suggested contract:
- reuse the existing request and response DTOs as closely as possible

#### 4. `rewrite_support_draft`

Purpose:
- rewrite a human-authored support draft using the current support rewrite logic

Backed by:
- `SupportAIService.RewriteSupportDraft(...)`
- `model.SupportAIRewriteDraftRequest`
- `model.SupportAIRewriteDraftResponse`

Why now:
- already implemented and bounded
- useful for both human-support and native-agent support flows

Suggested input:

```json
{
  "content": "draft reply",
  "operation": "more_friendly"
}
```

#### 5. `generate_support_task_draft`

Purpose:
- generate a PM-task draft from a support conversation without creating the task yet

Backed by:
- `SupportAIService.GenerateTaskDraftFromConversation(...)`

Why now:
- keeps the generation step inspectable
- gives native support agents a bridge into PM without immediately mutating PM state

Suggested input:

```json
{
  "conversation_id": "support-conversation-id"
}
```

Suggested output:

```json
{
  "title": "Investigate password reset flow",
  "summary": "Multiple customers are failing to reset passwords",
  "description_markdown": "## Problem\n...",
  "task_type": "bug",
  "priority": "high"
}
```

## Detailed Design For The First-Wave Cross-Agent Tools

This section narrows the plan to the four tools we want to focus on first and records the exact boundary, defaults, and contract recommendations.

### 1. `plan_support_response`

Classification:
- runtime-only

Primary backing:
- `SupportAIService.planSupportQuery(...)`

Existing response shape:
- `SupportQueryPlanContract`

Why it is broadly useful:
- reusable support reasoning primitive
- useful for support agents, CRM/deal agents, QA/review agents, and automation flows that need to classify a support situation before deciding what to do next

Recommended target behavior:
- usable on `support_conversation` targets
- optionally usable with explicit `history` even outside a bound support target for dry-run analysis

Recommended input:

```json
{
  "message": "latest customer message",
  "conversation_id": "optional support conversation id",
  "history": [
    {
      "sender_type": "customer",
      "message_type": "reply",
      "content": "..."
    }
  ]
}
```

Recommended rules:
- require either:
  - current run target is `support_conversation`
  - or explicit `history`
  - or explicit `conversation_id`
- if `conversation_id` is omitted on a `support_conversation` target, use the current target
- if both `conversation_id` and `history` are provided, prefer explicit `history` for deterministic dry-run behavior
- if `message` is omitted and a `support_conversation` target is bound, default to the latest customer reply in that conversation

Recommended output:
- return the normalized planner result directly:
  - `decision`
  - `issue_key`
  - `issue_summary`
  - `progress_signal`
  - `standalone_query`
  - `search_queries`
  - `clarifying_question`
  - `reason`

Key edge cases:
- no customer message found
- planner fallback path if the LLM planner fails
- planner should remain a dry-run decision primitive only; it must not perform handoff or sending itself

Implementation notes:
- add a worker bridge method rather than calling the unexported service helper directly from the tool layer
- return compact JSON only

### 2. `search_support_knowledge`

Classification:
- runtime-only

Primary backing:
- `SupportAIService.loadKnowledgeChunks(...)`

Data sources:
- `agent_knowledge_sources` for docs-space RAG sources
- `agent_content_sources` for crawled content sources

Existing result shape to reuse:
- close to `SupportAIPreviewSearchResult`

Why it is broadly useful:
- any agent that needs grounded support/help-center context can use this
- useful for support answer generation, docs-gap review, CRM context gathering, and support-to-PM conversion flows

Recommended target behavior:
- usable on `support_conversation`
- also usable on `workspace`, `crm_deal`, or other targets as long as the run has an agent with support knowledge sources attached

Recommended input:

```json
{
  "queries": ["reset password", "password reset email"],
  "max_results": 6
}
```

Recommended rules:
- `queries` required
- trim, dedupe, and drop empty queries
- `max_results` optional; clamp to a small bounded range such as `1..12`
- use the current run agent ID as the knowledge-source scope
- if the current agent has no linked support knowledge sources or content sources, return an empty `results` array rather than an error

Recommended output:

```json
{
  "results": [
    {
      "reference_id": "docs:doc-123",
      "source_type": "docs",
      "title": "Reset your password",
      "url": "https://...",
      "chunk_index": 0,
      "combined_score": 0.92,
      "vector_score": 0.84,
      "lexical_score": 0.31,
      "snippet": "..."
    }
  ]
}
```

Key edge cases:
- agent has no support knowledge sources
- embedding provider unavailable, causing lexical-only fallback
- mixed docs and crawled-content results in one response

Implementation notes:
- this tool should not try to expose internal scoring internals beyond what helps downstream reasoning
- keep `snippet` short and bounded
- preserve `reference_id` and `source_type` because later tools may depend on them

### 3. `generate_support_task_draft`

Classification:
- runtime-only

Primary backing:
- `SupportAIService.GenerateTaskDraftFromConversation(...)`

Internal draft shape today:
- `supportConversationTaskDraft`
  - `title`
  - `summary`
  - `description`
  - `task_type`
  - `priority`

Why it is broadly useful:
- support-to-PM bridge without immediate mutation
- useful for support agents, CRM operators, and backlog-curation or triage agents

Recommended target behavior:
- primarily for `support_conversation`
- if `conversation_id` omitted and current target is `support_conversation`, use bound target

Recommended input:

```json
{
  "conversation_id": "optional support conversation id"
}
```

Recommended rules:
- require a support conversation either from target context or explicit `conversation_id`
- load conversation plus message history using the same access rules as `CreateTaskFromConversation`
- reuse the same fallback and validation flow that support task creation already uses
- return draft output even if later task creation would be blocked by PM routing omissions

Recommended output:

```json
{
  "title": "Investigate password reset failures",
  "summary": "Customers are unable to complete password reset emails",
  "description_markdown": "## Problem\n...",
  "task_type": "bug",
  "priority": "high",
  "is_valid": true
}
```

Recommended extra field:
- `is_valid`
  - derived from `validateSupportTaskDraft(...)`
  - lets downstream agents know whether the draft is strong enough to convert into a real task without extra context

Key edge cases:
- insufficient conversation context
- LLM draft generation falls back to deterministic summary/description
- title/summary too weak to support task creation

Implementation notes:
- return markdown, not rich-text HTML
- if validation fails, still consider returning the draft plus `is_valid=false` instead of erroring outright, unless the calling flow requires hard failure
- this makes the tool more reusable for review and backlog-prep agents

### 4. `create_task_from_conversation`

Classification:
- command-backed

Primary backing:
- `SupportInboxService.CreateTaskFromConversation(...)`

Existing request DTO:
- `model.CreateTaskFromConversationRequest`

Existing response DTO:
- `model.CreateTaskFromConversationResponse`

Why it is broadly useful:
- strongest support-to-PM mutation bridge in the current system
- useful for support agents, CRM operators, and automation flows that turn customer pain into tracked work

Recommended target behavior:
- primarily for `support_conversation`
- if `conversation_id` is omitted on a support target, use the current target

Recommended command name:
- `support.create_task_from_conversation`

Recommended runtime alias:
- `create_task_from_conversation`

Recommended input:
- keep the existing DTO shape, but prefer a smaller canonical subset in prompt examples:

```json
{
  "conversation_id": "optional support conversation id",
  "name": "Investigate password reset failures",
  "team_id": "team-123",
  "workflow_id": "workflow-123",
  "workflow_state_id": "state-123",
  "epic_id": "optional epic id",
  "owner_member_id": "optional owner member id",
  "priority": "high"
}
```

Recommended rules:
- default `conversation_id` from target context
- preserve existing draft-generation and validation behavior
- preserve description normalization into PM rich-text HTML through the existing task-description path
- preserve association copying:
  - linked contacts
  - linked companies
  - linked deals
- preserve automatic conversation-task linking

Recommended output:
- reuse `CreateTaskFromConversationResponse`:
  - `task_id`
  - `display_id`
  - `task_key`
  - `task_name`
  - `summary`
  - copied association counts

Key edge cases:
- insufficient support context to form a useful task
- PM routing fields omitted when no sensible defaults exist
- task creation succeeds but association copy or linking fails

Implementation notes:
- this should be command-backed because it is a durable cross-module mutation
- the tool contract should stay smaller than the underlying full DTO in prompt guidance, even if the backend accepts the full payload

## Preset And Exposure Recommendations For The Four Focus Tools

### Support Agent preset

Add first:
- `plan_support_response`
- `search_support_knowledge`
- `generate_support_task_draft`
- `create_task_from_conversation`

### CRM Operator preset

This preset already allows `support_conversation` targets.

Add:
- `plan_support_response`
- `search_support_knowledge`
- `generate_support_task_draft`
- `create_task_from_conversation`

Reason:
- CRM operator is the strongest non-support preset candidate for these tools because it already spans deals, documents, and support conversations

### Custom agents

Recommended default custom-agent availability:
- `plan_support_response`
- `search_support_knowledge`
- `generate_support_task_draft`

Delay general custom-agent access to:
- `create_task_from_conversation`

until we confirm PM/support permission expectations and whether custom agents should mutate linked support conversations by default.

## Command-backed tools

These should be model-visible runtime tools backed by internal commands because they mutate durable product state.

### 6. `send_support_reply`

Purpose:
- send a support reply directly from a native support run

Backed by:
- `SupportInboxService.CreateConversationMessage(...)`

Why:
- this is a durable support mutation
- customer-visible sending should stay backend-owned and validated

Suggested command:
- `support.send_reply`

Suggested input:

```json
{
  "conversation_id": "support-conversation-id",
  "content": "reply text",
  "is_internal": false
}
```

Suggested output:

```json
{
  "message_id": "msg-123",
  "conversation_id": "support-conversation-id",
  "sender_type": "ai",
  "message_type": "reply"
}
```

Notes:
- do not replace `draft_support_reply`
- keep both:
  - `draft_support_reply` for approval-required workflows
  - `send_support_reply` for intentionally autonomous support agents
- initial exposure should be limited to the system `support_agent` preset

### 7. `handoff_to_human`

Purpose:
- escalate a support conversation to human handling

Backed by:
- `SupportAIService.EscalateToHuman(...)`

Why:
- this is one of Echo's core durable actions
- the routing, mailbox selection, state updates, handoff analytics, and system message creation should remain backend-owned

Suggested command:
- `support.handoff_to_human`

Suggested input:

```json
{
  "conversation_id": "support-conversation-id",
  "reason": "low_confidence"
}
```

Suggested output:

```json
{
  "conversation_id": "support-conversation-id",
  "ai_state": "escalated",
  "flow_state": "assigned_to_human",
  "mailbox_id": "mailbox-123"
}
```

### 8. `move_support_conversation`

Purpose:
- move a conversation to a different mailbox

Backed by:
- `SupportInboxService.MoveConversation(...)`

Why:
- routing changes are durable support mutations
- access checks and system-message side effects already exist and should stay backend-owned

Suggested command:
- `support.move_conversation`

Suggested input:

```json
{
  "conversation_id": "support-conversation-id",
  "mailbox_id": "mailbox-123"
}
```

### 9. `create_task_from_conversation`

Purpose:
- create and link a PM task from a support conversation

Backed by:
- `SupportInboxService.CreateTaskFromConversation(...)`
- `model.CreateTaskFromConversationRequest`
- `model.CreateTaskFromConversationResponse`

Why:
- this is a cross-module business mutation
- it already encapsulates support summarization, task creation, linking, and association copying

Suggested command:
- `support.create_task_from_conversation`

Suggested contract:
- reuse the existing request and response DTOs as closely as possible

### 10. `dismiss_conversation_triage`

Purpose:
- dismiss the current triage suggestion for a conversation

Backed by:
- `SupportInboxService.DismissConversationTriage(...)`

Why:
- lower priority than the other mutations
- still a real, explicit user-visible product mutation

Suggested command:
- `support.dismiss_conversation_triage`

## Optional second-wave tool

### 11. `evaluate_and_route_support_conversation`

Recommendation:
- optional
- only add if we want faster parity with Echo's current routing behavior

Backed by:
- `SupportInboxTriageService.EvaluateAndRoute(...)`

Why it is optional:
- it is useful for parity
- but it exposes more classifier-driven orchestration directly to the model than the first-wave tools do
- `move_support_conversation` is the cleaner base mutation

If added, this should likely be command-backed because it can cause durable routing changes through auto-move behavior.

## Tools Not Recommended Right Now

Do not expose these as first-pass model-facing tools:

- per-message dedupe
- per-conversation Redis lock control
- token budget accounting or overrides
- direct `ai_state` setters
- raw typing-indicator control
- assumed-resolution scan controls
- low-level source metadata writing

These are backend orchestration and guardrail concerns, not good model-facing contracts.

## Exposure Policy

### System support agent

First exposure should go here:

- `plan_support_response`
- `search_support_knowledge`
- `preview_support_reply`
- `rewrite_support_draft`
- `generate_support_task_draft`
- `handoff_to_human`
- `move_support_conversation`
- `create_task_from_conversation`

Consider delaying `send_support_reply` until we are comfortable with the autonomy boundary.

### Custom agents

Recommended first custom-agent exposure:

- `plan_support_response`
- `search_support_knowledge`
- `preview_support_reply`
- `rewrite_support_draft`
- `generate_support_task_draft`

Do not expose customer-visible autonomous mutations broadly at first.

Delay broad custom-agent access to:

- `send_support_reply`
- `handoff_to_human`
- `move_support_conversation`

until permission policy and UX around support autonomy are clearer.

## Implementation Plan

### Phase 1: Extract read/dry-run tools

Implement:

1. `plan_support_response`
2. `search_support_knowledge`
3. `preview_support_reply`
4. `rewrite_support_draft`
5. `generate_support_task_draft`

Files likely touched:

- `server/internal/worker/tools.go`
- `server/internal/worker/tools_teampulse.go` or a new `tools_support.go`
- `server/internal/worker/tool_catalog.go`
- `server/internal/worker/runtime_profiles.go`
- `server/internal/worker/context.go`
- `server/internal/temporalapp/activities.go`
- tests under `server/internal/worker/`

Notes:
- these can ship without changing support command infrastructure
- these are the safest building blocks for native support agents

### Phase 2: Add first support command-backed mutations

Implement:

1. `support.handoff_to_human` + runtime alias `handoff_to_human`
2. `support.move_conversation` + runtime alias `move_support_conversation`
3. `support.create_task_from_conversation` + runtime alias `create_task_from_conversation`

Files likely touched:

- `server/internal/commandtools/metadata.go`
- `server/internal/service/internal_command_service.go`
- `server/internal/worker/tools.go`
- `server/internal/worker/tools_*.go`
- `server/internal/worker/context.go`
- `server/internal/temporalapp/activities.go`
- tests in `server/internal/service/` and `server/internal/worker/`

Notes:
- this phase gives native agents the main durable Echo-adjacent actions without migrating Echo itself

### Phase 3: Consider autonomous send

Implement only if product wants autonomous native support runs:

1. `support.send_reply` + runtime alias `send_support_reply`

Additional work before rollout:

- preset policy review
- approval/autonomy policy review
- explicit permission model for customer-visible sends
- product decision on whether custom agents may ever send customer-visible support replies directly

## Contract Rules

For all new tools:

- use `snake_case`
- use top-level JSON object inputs
- use explicit `properties`
- use explicit `required`
- set `additionalProperties: false`
- keep identifiers as `*_id`
- return compact structured JSON
- keep validation errors short and field-specific

For support tools specifically:

- prefer `conversation_id` for explicit overrides
- default to the current support target when the run already has a `support_conversation` target
- return normalized IDs and resulting status/state fields after mutations

## Recommended First Shipping Order

1. `preview_support_reply`
2. `plan_support_response`
3. `search_support_knowledge`
4. `generate_support_task_draft`
5. `handoff_to_human`
6. `move_support_conversation`
7. `create_task_from_conversation`
8. `send_support_reply` only after explicit autonomy review

## Expected Outcome

After Phase 2, Echo can remain on its current AI-first backend path, while native support agents gain:

- the same support planning and retrieval substrate
- human handoff capability
- mailbox-routing capability
- support-to-PM task creation

That gives us meaningful convergence through tools without prematurely collapsing the support-specific orchestration path into generic native runtime behavior.

## Support Coverage Agent Extension

This section covers the adjacent use case:

- collect where Echo could not answer because docs were missing or weak
- review the evidence behind those failures
- generate doc-fix suggestions
- optionally apply those suggestions into Docs

For this use case, the correct substrate is not raw support conversations alone. It is the existing `support_coverage` system, which already stores:

- durable coverage gaps
- evidence linked to those gaps
- related articles
- generated fix suggestions

That means a gap-analysis agent should primarily use `support_coverage` tools, not re-derive gaps from raw conversations on every run.

### Why `support_coverage` should be the source of truth

The backend already has:

- gap and evidence models in [support_coverage.go](../../server/internal/model/support_coverage.go)
- read/query service methods in [support_coverage.go](../../server/internal/service/support_coverage.go)
- docs-suggestion generation and apply flows in [support_coverage_drafts.go](../../server/internal/service/support_coverage_drafts.go)

This is exactly the durable product layer a native gap-analysis agent should build on.

## Additional Tools For Gap Analysis

### Runtime-only tools

These are the primary read surfaces for a support-coverage or docs-gap agent.

#### 12. `list_support_coverage_gaps`

Purpose:
- list known support coverage gaps that came from AI failures, handoffs, weak retrieval, or related signals

Backed by:
- `SupportCoverageService.ListGaps(...)`

Existing filter shape:
- `model.SupportCoverageGapFilter`

Recommended input:

```json
{
  "status": "open",
  "v1_gap_type": "missing_article",
  "issue_key": "",
  "search": "",
  "page": 1,
  "per_page": 20
}
```

Recommended output:
- close to the existing list item DTO:
  - `id`
  - `title`
  - `issue_key`
  - `v1_gap_type`
  - `status`
  - `confidence`
  - `failure_mode`
  - `source_signal`
  - `evidence_count`
  - `suggestion_count`
  - `topic_title`
  - `related_article_id`

Why it matters:
- this becomes the main inbox for “where Echo failed because docs were not good enough”

#### 13. `get_support_coverage_gap_detail`

Purpose:
- load the full detail for one gap, including evidence, existing suggestions, and related articles

Backed by:
- `SupportCoverageService.GetGapDetail(...)`

Recommended input:

```json
{
  "gap_id": "gap-123"
}
```

Recommended output:
- reuse the existing detail DTO as closely as possible:
  - gap metadata
  - `evidence`
  - `suggestions`
  - `related_articles`

Why it matters:
- this is the main reasoning surface for deciding:
  - whether a gap is truly docs-related
  - whether it should create a new article or update an existing one
  - whether a previous suggestion already exists

#### 14. `get_support_coverage_summary`

Purpose:
- retrieve summary counts and aggregate coverage metrics for prioritization

Backed by:
- `SupportCoverageService.GetSummary(...)`

Recommended input:

```json
{}
```

Recommended output:
- reuse the existing summary DTO

Why it matters:
- less important than list/detail, but useful for prioritization and reporting agents

#### 15. `get_conversation_coverage_state`

Purpose:
- check whether docs-issue feedback has already been submitted for a conversation and whether it is already linked to a gap

Backed by:
- `SupportCoverageService.GetConversationCoverageState(...)`

Recommended input:

```json
{
  "conversation_id": "support-conversation-id"
}
```

Why it matters:
- lower priority than gap list/detail
- useful if a support agent wants to understand whether a conversation already contributed to the coverage system

### Command-backed tools

These are the mutation surfaces for generating and applying docs-fix suggestions from gaps.

#### 16. `create_article_draft_suggestion`

Purpose:
- generate a draft-doc suggestion for a missing-article gap

Backed by:
- `SupportCoverageDraftService.GenerateArticleDraft(...)`

Recommended command:
- `support.create_article_draft_suggestion`

Recommended input:

```json
{
  "gap_id": "gap-123",
  "target_space_id": "space-123",
  "target_collection_id": "optional-collection-id"
}
```

Recommended output:
- reuse `SupportGapSuggestion`

Why it matters:
- this is the core “suggest docs” mutation for missing-article coverage gaps

#### 17. `create_article_update_suggestion`

Purpose:
- generate an additive article-update suggestion for an existing weak/outdated article

Backed by:
- `SupportCoverageDraftService.GenerateArticleUpdate(...)`

Recommended command:
- `support.create_article_update_suggestion`

Recommended input:

```json
{
  "gap_id": "gap-123",
  "target_document_id": "document-123"
}
```

Why it matters:
- this is the core “improve docs” mutation for weak/outdated article gaps

#### 18. `apply_support_gap_suggestion`

Purpose:
- apply a previously generated suggestion to Docs

Backed by:
- `SupportCoverageDraftService.ApplySuggestion(...)`

Recommended command:
- `support.apply_gap_suggestion`

Recommended input:

```json
{
  "suggestion_id": "suggestion-123"
}
```

Why it matters:
- this closes the loop from detected gap to concrete docs artifact

Important behavior already preserved by the backend:
- create-article suggestions create a draft document
- update-article suggestions append new content instead of replacing existing content
- applying a suggestion marks the gap fixed and links the resulting article/doc

#### 19. `discard_support_gap_suggestion`

Purpose:
- reject a suggestion and revert the gap back to `open`

Backed by:
- `SupportCoverageService.DiscardSuggestion(...)`

Recommended command:
- `support.discard_gap_suggestion`

Recommended input:

```json
{
  "suggestion_id": "suggestion-123"
}
```

Why it matters:
- agents need a clean way to reject poor suggestions instead of only applying them

#### 20. `reclassify_support_coverage_gap`

Purpose:
- change a gap’s `v1_gap_type`

Backed by:
- `SupportCoverageService.ReclassifyGap(...)`

Recommended command:
- `support.reclassify_coverage_gap`

Recommended input:

```json
{
  "gap_id": "gap-123",
  "v1_gap_type": "missing_article"
}
```

Why it matters:
- many useful docs-fix flows depend on getting the gap type right before generating a suggestion

#### 21. `merge_support_coverage_gaps`

Purpose:
- merge overlapping gaps when they represent the same underlying issue

Backed by:
- `SupportCoverageService.MergeGaps(...)`

Recommended command:
- `support.merge_coverage_gaps`

Recommended input:

```json
{
  "source_gap_id": "gap-123",
  "target_gap_id": "gap-456"
}
```

Why it matters:
- valuable for cleanup and deduplication once an agent is allowed to curate the coverage backlog

## Recommended Gap-Analysis Agent Bundle

If the product goal is “find where Echo cannot answer due to lacking docs and suggest docs fixes,” the recommended first tool bundle is:

### Read/analyze

- `list_support_coverage_gaps`
- `get_support_coverage_gap_detail`
- `get_support_coverage_summary`
- `search_support_knowledge`

### Suggest/fix

- `create_article_draft_suggestion`
- `create_article_update_suggestion`
- `apply_support_gap_suggestion`
- `discard_support_gap_suggestion`

### Optional backlog-curation tools

- `reclassify_support_coverage_gap`
- `merge_support_coverage_gaps`

## Recommended Preset Exposure For Gap Analysis

### New docs-gap or coverage-review system agent

This is the cleanest long-term home for the coverage-specific bundle.

Recommended tools:
- `list_support_coverage_gaps`
- `get_support_coverage_gap_detail`
- `get_support_coverage_summary`
- `search_support_knowledge`
- `create_article_draft_suggestion`
- `create_article_update_suggestion`
- `apply_support_gap_suggestion`
- `discard_support_gap_suggestion`
- optionally `reclassify_support_coverage_gap`
- optionally `merge_support_coverage_gaps`

### Support Agent preset

Do not overload the general `support_agent` with all coverage-edit tools by default.

Recommended additions only if needed:
- `list_support_coverage_gaps`
- `get_support_coverage_gap_detail`

The docs-mutation coverage tools are better kept in a more specialized review/fix agent unless support is explicitly meant to edit docs continuously.

### Custom agents

Recommended first custom-agent exposure:
- `list_support_coverage_gaps`
- `get_support_coverage_gap_detail`
- `get_support_coverage_summary`

Delay broad custom-agent access to:
- suggestion generation
- suggestion application
- gap merging/reclassification

until the product decides how much autonomous authority custom agents should have over docs and coverage backlog state.

## Recommended Shipping Order For Gap Analysis

1. `list_support_coverage_gaps`
2. `get_support_coverage_gap_detail`
3. `get_support_coverage_summary`
4. `create_article_draft_suggestion`
5. `create_article_update_suggestion`
6. `apply_support_gap_suggestion`
7. `discard_support_gap_suggestion`
8. optionally `reclassify_support_coverage_gap`
9. optionally `merge_support_coverage_gaps`

This order gives the fastest path to a useful “coverage gap analyst” agent:

- first it can identify and inspect gaps
- then it can generate docs fixes
- then it can apply or reject them
- only after that does it need backlog-curation tools like merge/reclassify
