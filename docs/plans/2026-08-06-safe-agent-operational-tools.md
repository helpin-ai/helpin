# Safe agent operational tools implementation plan

> Historical implementation plan, source-compared on 2026-09-17. The command
> adapters described here now exist. Unchecked steps record the original sequence;
> they are not evidence that the work is absent or an instruction to recreate it.

## Current implementation and review limits

Existing adapters include [PM delivery targets](../../server/internal/service/internal_command_pm_delivery.go),
[Docs metadata](../../server/internal/service/internal_command_docs_metadata.go),
[CRM operations](../../server/internal/service/internal_command_crm_operational.go),
[CRM associations](../../server/internal/service/internal_command_crm_associations.go),
and [Support operations](../../server/internal/service/internal_command_support_operational.go).
Use current [metadata](../../server/internal/commandtools/metadata.go) and the
[embedded catalog](../../server/internal/agentcontract/tool_catalog.json) for exact
aliases and schemas. Current backend Go requirements come from
[go.mod](../../server/go.mod), which declares Go 1.25.0; the original Go 1.24 line
below is historical.

Strict published schemas do not establish strict adapter decoding. PM delivery,
Docs metadata, and CRM operational inputs use `json.Unmarshal`, which ignores
unknown fields. The Docs adapter explicitly checks document workspace and loads
the space with actor context; such checks must be assessed per adapter rather
than inferred from the architecture paragraph. Explicit owner-clear conflicts are
rejected in CRM contact/company updates, but that does not prove every field
combination in the original test matrix.

The [reviewed design](../specs/2026-08-06-safe-agent-operational-tools-design.md)
documents central authorization preconditions, varying discovery limits, and the
limits of broad transaction/scope claims. This companion plan shares those
qualifications. The non-goals apply to this proposed tool batch, not to every tool
in today's larger catalog.

Operational test files exist beside these adapters. Their presence and the original
RED/GREEN instructions do not constitute a fresh full backend test run. The branch
and commit steps below are historical delivery instructions; this documentation
review changes no runtime code or release state.

## Original implementation sequence

**Goal:** Expand Automation > Allowed tools with bounded, workspace-scoped operational tools for the remaining PM, Docs, CRM, and Support surfaces while keeping destructive, administrative, identity-changing, and self-modifying actions unavailable.

**Architecture:** Reuse existing domain services behind typed internal commands, publish strict JSON schemas with `additionalProperties: false`, and keep the embedded selectable-tool catalog synchronized with executable registrations. Every mutation performs authorization and workspace/object validation before calling a domain service, returns a compact projection, and exposes explicit clear operations only where clearing is safe. Existing comprehensive PM tools remain unchanged; this work adds only missing delivery-target operations there.

**Tech Stack:** Go 1.24, GORM/PostgreSQL, embedded JSON tool catalog, existing Helpin authorization and domain services, Go tests.

**Spec:** `docs/specs/2026-08-06-safe-agent-operational-tools-design.md`

---

## Safety contract

- Contact mutation is limited to lifecycle stage, lead status, owner, and labels.
- Company mutation is limited to owner.
- Deal mutation is limited to operational pipeline/stage, owner, amount/currency, dates, probability, and next step fields already supported by the domain model.
- Document mutation is limited to metadata; body editing and collection moves remain separate tools.
- Support mutation is limited to triage and associations: subject, assignee, inbox, tags, linked PM task, and linked CRM contact.
- CRM association tools operate only on existing workspace-scoped objects and use supported object-type pairs.
- No delete, archive, permission, billing, secret, workflow-schema, identity, arbitrary custom-field, or agent-configuration mutations are added.

---

### Task 1: Lock the catalog and schema contracts

**Files:**
- Modify: `server/internal/commandtools/metadata.go`
- Modify: `server/internal/commandtools/metadata_test.go`
- Modify: `server/internal/agentcontract/tool_catalog_test.go`
- Modify later with executors: `server/internal/agentcontract/tool_catalog.json`

- [ ] Add failing table-driven tests for every approved alias, category, strict schema, required identifier, bounded list limit, and explicit clear flag.
- [ ] Add negative schema assertions that guarded CRM identity/enrichment fields and destructive/admin fields are absent.
- [ ] Run focused commandtools and agentcontract tests and confirm RED.
- [ ] Add canonical metadata definitions, keeping the frozen catalog update until executor parity can pass in the same implementation batch.

Approved aliases:

```text
PM / Delivery: update_task_delivery_target, update_epic_delivery_target
Docs: update_document_metadata
CRM / Discovery: get_crm_contact, get_crm_company, get_crm_deal,
  list_crm_companies, list_crm_pipelines, list_crm_associations
CRM / Operations: update_crm_contact, update_crm_company, update_crm_deal,
  add_crm_activity, link_crm_objects, unlink_crm_association,
  set_primary_contact_company
Support / Discovery: list_support_conversations, get_support_conversation,
  list_support_tags, list_support_inboxes, list_support_assignees
Support / Triage: assign_support_conversation, move_support_conversation,
  add_support_conversation_tag, remove_support_conversation_tag,
  link_support_conversation_task, link_support_conversation_contact,
  update_support_conversation_subject
```

### Task 2: Add the missing PM delivery-target operations

**Files:**
- Create: `server/internal/service/internal_command_pm_delivery.go`
- Create: `server/internal/service/internal_command_pm_delivery_test.go`
- Modify: `server/internal/service/internal_command_service.go`
- Modify: `server/internal/commandtools/metadata.go`

- [ ] Write failing tests for task and epic repository/branch targeting, clearing the target, cross-workspace rejection, and unavailable Git integration behavior.
- [ ] Run the focused service tests and confirm RED.
- [ ] Register commands backed by the existing Git delivery-target service methods, preserving current task/epic tool behavior.
- [ ] Run focused tests and confirm GREEN.

### Task 3: Add bounded document metadata updates

**Files:**
- Create: `server/internal/service/internal_command_docs_metadata.go`
- Create: `server/internal/service/internal_command_docs_metadata_test.go`
- Modify: `server/internal/service/internal_command_service.go`
- Modify: `server/internal/commandtools/metadata.go`

- [ ] Write failing tests for title, owner, excerpt, icon, tags, pinned state, explicit clears, visibility/workspace isolation, and forbidden collection/body fields.
- [ ] Run focused tests and confirm RED.
- [ ] Implement through `DocsDocumentService.Update`, using existing visibility and edit permission checks and compact output.
- [ ] Run focused tests and confirm GREEN.

### Task 4: Add CRM discovery and bounded entity updates

**Files:**
- Create: `server/internal/service/internal_command_crm_operational.go`
- Create: `server/internal/service/internal_command_crm_operational_test.go`
- Modify: `server/internal/service/internal_command_service.go`
- Modify: `server/internal/commandtools/metadata.go`
- Modify: `server/cmd/api/main.go`
- Modify if dependencies exist there: `server/cmd/temporal-worker/main.go`

- [ ] Write failing tests for scoped contact/company/deal lookup, bounded company and pipeline lists, owner/member validation, enum/date validation, explicit clearing, and cross-workspace denial.
- [ ] Assert `update_crm_contact` accepts only lifecycle stage, lead status, owner, and labels; `update_crm_company` accepts only owner.
- [ ] Run focused tests and confirm RED.
- [ ] Add CRM operational dependency setters and implement commands through the existing contact, company, and deal services.
- [ ] Run focused tests and confirm GREEN.

### Task 5: Add CRM activities and safe associations

**Files:**
- Create: `server/internal/service/internal_command_crm_associations.go`
- Create: `server/internal/service/internal_command_crm_associations_test.go`
- Modify: `server/internal/service/crm_association.go`
- Modify: `server/internal/service/crm_association_test.go`
- Modify: `server/internal/service/internal_command_service.go`
- Modify: `server/internal/commandtools/metadata.go`

- [ ] Write failing service tests for workspace-scoped create/delete/list, supported object types, object existence, duplicate/idempotent behavior, and primary contact-company replacement.
- [ ] Run focused association tests and confirm RED.
- [ ] Add scoped association service operations without weakening existing enrichment semantics.
- [ ] Write failing internal-command tests for one-target CRM activities, link/unlink, list, and set-primary operations.
- [ ] Implement compact command adapters using the activity and association services; reject arbitrary metadata and cross-workspace IDs.
- [ ] Run focused tests and confirm GREEN.

### Task 6: Add Support discovery and triage operations

**Files:**
- Create: `server/internal/service/internal_command_support_operational.go`
- Create: `server/internal/service/internal_command_support_operational_test.go`
- Modify: `server/internal/service/internal_command_service.go`
- Modify: `server/internal/commandtools/metadata.go`
- Modify: `server/cmd/api/main.go`
- Modify if dependencies exist there: `server/cmd/temporal-worker/main.go`

- [ ] Write failing tests for bounded conversation lists, detail, tags, inboxes, assignable users, assignment, inbox move, tag add/remove, task/contact linking, subject update, side effects, and workspace isolation.
- [ ] Run focused tests and confirm RED.
- [ ] Add Support operational dependency setters and delegate to `SupportInboxService` and `SupportTagService` so existing validation and event publication remain authoritative.
- [ ] Run focused tests and confirm GREEN.

### Task 7: Publish tools and authorize the intended agents

**Files:**
- Modify: `server/internal/agentcontract/tool_catalog.json`
- Modify: `server/internal/agentcontract/tool_catalog_test.go`
- Modify: `server/internal/agentcontract/runtime_profiles.go`
- Modify: `server/internal/agentcontract/runtime_profiles_test.go`
- Modify: `server/internal/service/agent_presets.go`
- Modify: relevant preset/policy tests
- Modify: `server/cmd/api/main.go`
- Modify if applicable: `server/cmd/temporal-worker/main.go`

- [ ] Add failing catalog/executor parity and exact profile/preset allowlist tests.
- [ ] Wire every dependency in each runtime that executes internal commands; do not advertise an unusable built-in tool.
- [ ] Mechanically update the frozen catalog from the approved metadata while preserving existing entries and ordering.
- [ ] Grant tools only to relevant built-in profiles/presets; custom agents can select them through Allowed tools.
- [ ] Run catalog, policy, profile, preset, and command parity tests and confirm GREEN.

### Task 8: Verify and commit

- [ ] Run `gofmt` on changed Go files.
- [ ] Run focused tests for commandtools, agentcontract, service, and command wiring.
- [ ] Run `cd server && go test ./...`.
- [ ] Run `git diff --check` and inspect the final diff for guarded-field or destructive-tool regressions.
- [ ] Commit the implementation on `waqar-fixes` with scoped commits and report exact verification results.
