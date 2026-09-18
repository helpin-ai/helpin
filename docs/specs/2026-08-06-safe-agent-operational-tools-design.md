# Safe agent operational tools design


> Historical design, source-compared on 2026-09-17. This page describes the
> intended boundaries of operational agent tools. Core command families are
> implemented; the safety requirements below are not a completed security audit.

## Current implementation and qualifications

The [command service](../../server/internal/service/internal_command_service.go)
registers focused PM, Docs, CRM, and Support command families. Current aliases and
schemas live in [shared metadata](../../server/internal/commandtools/metadata.go)
and the [Automation catalog](../../server/internal/agentcontract/tool_catalog.json).
For example, the label discovery alias is `list_pm_labels`, not the proposed
`list_task_labels`. CRM operational registration also includes deal creation.

The common unknown-property rejection rule is not uniform. Some adapters use
strict decoding, while [CRM contact updates](../../server/internal/service/internal_command_crm_operational.go)
and several [Support operations](../../server/internal/service/internal_command_support_operational.go)
use `json.Unmarshal`, which ignores unknown fields. Central `Execute` does not
perform general JSON-schema validation before invoking the adapter. A bounded
published schema alone therefore does not establish rejection on every internal
entry path. Ignored fields do not automatically become editable fields.

Actor authorization also has preconditions: `authorizeCommandActor` returns
without checking permissions when the authorization service is absent or
`ActorRole` is empty. Merely having an actor ID is insufficient for the statement
below that module permissions are enforced “when an actor is present.” Runtime
policy, Dock mutation checks, and domain validations provide other layers, whose
coverage must be verified for the actual caller. These observations do not prove
that an untrusted caller can reach an unguarded operation.

Discovery pagination varies by command family; use each tool's current schema
and adapter rather than treating default 50/cap 100 as a universal rule. Likewise,
transactionality and side effects depend on the specific delegated service.
The requirements that cross-workspace IDs are never accepted and all scope rules
match the UI remain invariants to test, not guarantees established by this design.

The branch/commit delivery statement below records the original author's context;
it is not evidence of today's branch, release availability, or clean worktree.
Test files exist for operational commands, but no full backend regression run or
end-to-end permission audit was performed during this documentation comparison.

## Original design

## Summary

Helpin automation agents can create tasks and perform a handful of narrow state transitions, but they cannot safely maintain many existing PM, Docs, CRM, and Support records. This change adds native, bounded operational tools and the discovery tools needed to resolve valid IDs before mutation.

The tools must reuse existing service-layer behavior, permissions, team scoping, validation, activity logging, notifications, and realtime events. They must not expose destructive or administrative operations.

## Goals

- Let agents update safe, explicitly enumerated fields on existing tasks, epics, sprints, objectives, key results, documents, CRM records, and support conversations.
- Let agents discover the IDs required by those updates without using database or HTTP escape hatches.
- Support explicit association clearing without confusing omitted fields with empty values.
- Preserve the same authorization and scope rules as the corresponding product UI and API.
- Keep each runtime tool schema bounded, auditable, and suitable for the Automation “Allowed tools” picker.

## Non-goals

- Delete or archive product records.
- Transfer records between teams.
- Edit workspace, organization, workflow, pipeline, mailbox, or automation configuration.
- Let agents modify their own definitions or automation rules.
- Expose arbitrary CRM custom-property replacement.
- Provide a generic `update_entity` or unrestricted API tool.

## Architecture

Each tool is a native internal command registered with `InternalCommandService`. Shared metadata in `internal/commandtools` defines its stable alias, category, description, and JSON schema. The frozen `agentcontract/tool_catalog.json` remains the Automation UI contract and is updated to match the registered tools.

Module-specific command registration and execution live in focused files rather than expanding `internal_command_service.go`. New service dependencies are wired through narrow setters or interfaces so commands delegate to existing domain services. The command layer is responsible only for parsing the bounded tool payload, resolving the current target when an ID is omitted, translating explicit clear flags, and returning a compact result.

Authorization remains layered:

1. The agent’s `allowed_tools` policy must contain the tool.
2. Internal command authorization enforces the module permission when an actor is present.
3. Domain services enforce workspace membership, team visibility, entity scope, and field validation.

## Common Contract Rules

- Object IDs may default from the current compatible agent target; otherwise they are required.
- Omitted fields remain unchanged.
- Empty arrays replace an association set with an empty set.
- Nullable scalar and date fields use explicit `clear_*` booleans.
- Update payloads reject unknown properties.
- Discovery responses default to 50 items and cap at 100.
- Discovery queries are workspace- and team-scoped using the current actor context.
- Mutations return the affected object ID and the updated bounded representation.
- Existing activity, notification, and WebSocket side effects remain owned by domain services.

## PM Tools

### Discovery

- `list_epics`: ID, name, state, team, owner, dates, health; optional team/search filters.
- `list_sprints`: ID, name, team, dates, archived state; optional team/search filters, excluding archived by default.
- `list_objectives`: ID, name, state, health, teams, dates; optional team/search filters.
- `list_key_results`: key results for one objective.
- `list_task_labels`: ID, name, color, team scope; optional team/search filters.
- `list_workspace_members`: workspace member ID, user ID when active, display name, role, and team memberships; optional team/search filters.

### Mutations

- `update_task`: name, description, task type, epic, sprint, owners, estimate, priority, severity, deadline, blocked state, blocker, and labels. Workflow state remains the responsibility of `update_task_state`. Team, archive, requester, follower, template, external ID, and assigned-agent changes are excluded.
- `update_epic`: name, description, epic state, owner, planned start date, deadline, color, health, health comment, and labels. Team, archive, planning repository, and assigned-agent changes are excluded.
- `update_sprint`: name, description, start date, end date, and labels. Team and archive changes are excluded.
- `update_objective`: name, description, objective type, state, planned start date, deadline, health, health comment, owners, teams, labels, and linked epics. Existing service-level team-management checks apply.
- `update_key_result`: name, result type, initial/current/target values, note, and position.
- `update_task_delivery_target`: selected repository, base branch, and working branch.
- `update_epic_delivery_target`: selected repository, base branch, and epic branch.

Delivery-target tools only select from repositories already enabled for the workspace and reuse current Git service validation. They do not create repositories, integrations, branches, commits, or pull requests.

## Docs Tool

- `update_document_metadata`: title, owner, excerpt, icon, tags, and pinned state.

Document movement remains in `move_document`; content remains in `write_document_content` and `update_document_block`. Template reassignment and publication/help-center administration are excluded.

## CRM Tools

### Discovery

- `get_crm_contact`, `get_crm_company`, and `get_crm_deal` return bounded current state and associations.
- `list_crm_companies` supports bounded search and owner filtering.
- `list_crm_pipelines_with_stages` returns valid pipeline/stage IDs.
- `list_crm_associations` returns direct associations for one supported object.

### Mutations

- `update_crm_contact`: lifecycle stage, lead status, owner, and labels only.
- `update_crm_company`: owner only.
- `update_crm_deal`: name, pipeline, stage, amount, currency, close date, owner, and probability. Stage-only flows may continue using `update_deal_stage`.
- `add_crm_activity`: append a note, call, meeting, or email activity to a contact, company, or deal. This avoids overwriting descriptions.
- `link_crm_objects`: create one validated association between supported CRM/product objects.
- `unlink_crm_association`: remove one exact association by association ID.
- `set_primary_contact_company`: create or promote a contact-company association as primary.

Contact identity fields and researched fields remain protected. Email, phone, job title, location, avatar, social URLs, company domain, industry, headcount, revenue, description, logo, LinkedIn, and headquarters continue through the guarded enrichment tools, including evidence, confidence, fill-only, dry-run, and audit behavior. Arbitrary custom properties remain unavailable.

## Support Tools

### Discovery

- `list_support_conversations`: bounded search with status and mailbox filters.
- `get_support_conversation`: current conversation metadata, assignments, tags, linked task/contact, and mailbox.
- `list_support_tags`: IDs and names available to the workspace.
- `list_support_mailboxes`: IDs and names visible to the actor.
- `list_conversation_assignees`: assignable workspace users for one conversation.

### Mutations

- `assign_conversation_user`: assign or clear the teammate.
- `move_conversation`: move to a valid mailbox/inbox destination supported by the existing service.
- `add_conversation_tag` and `remove_conversation_tag`.
- `link_conversation_task`.
- `link_conversation_crm_contact`.
- `update_conversation_subject`.

Conversation deletion, message deletion, mailbox administration, email-recipient editing, customer identity editing, and agent assignment are excluded.

## Error Handling

- Missing or inaccessible entities return the same not-found/forbidden semantics as their domain services.
- Cross-workspace IDs are never accepted.
- Cross-team epic, sprint, label, objective, and task associations are rejected by existing scope validators.
- Invalid enum, date, branch, pipeline, stage, or member values return actionable validation errors.
- Conflicting payloads such as `deadline` plus `clear_deadline` are rejected.
- Tools fail atomically wherever the existing service operation is transactional. Multi-step association operations must use existing transactional services or add a transaction at the service layer, not in the command adapter.

## Testing

Development follows test-first cycles. Tests cover:

- Every new alias and schema appearing in shared metadata and the frozen Automation catalog.
- Unknown-property rejection and required/optional field behavior.
- Target-ID fallback and incompatible target rejection.
- Explicit clearing semantics.
- Delegation into existing services with the correct workspace and actor.
- Workspace/team scope and permission failures.
- CRM identity/enrichment fields remaining unavailable.
- Bounded discovery limits and archived filtering.
- Existing service side effects where the command delegates to a real service in integration tests.
- Full backend regression tests and catalog consistency checks.

## Delivery

The work is implemented and committed on `waqar-fixes`. Existing unrelated worktrees and untracked files outside this worktree are not modified.
