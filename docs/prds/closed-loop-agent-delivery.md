# Closed-loop agent task delivery

**Status:** Draft for product and technical review

**Version:** v1.0

**Date:** 2026-07-10

**Owners:** Product, Agent Platform, Project Management, Automation, Frontend, Infrastructure

**Target release:** Helpin release train, phased behind workspace feature flags

**Decision scope:** Helpin product behavior, agent-runtime contracts, Temporal orchestration, task lifecycle, Agent Dock, and run detail

**Related documents:**

- [Agents and automation](../agents-and-automation.md)
- [How automation works](../automation-product-model.md)
- [Ask Agents Bar](../agent-dock.md)
- [Agent Authorization Plan](../plans/agent-authorization.md)
- [Delegated Run Finalizers](../plans/2026-07-02-delegated-run-finalizers.md)
- [Notifications System](notifications-system.md)

---

## 1. Executive Summary

Helpin should not try to become a general-purpose coding workspace or reproduce Cursor. Its advantage is that a user can define work in the product and trust Helpin to carry it through to a usable outcome.

Today, Helpin can launch durable agent runs, stream activity, collect interactions and artifacts, retry parts of execution through Temporal, and finalize several domain-specific outputs. The missing layer is an authoritative product-level answer to this question:

> Did the requested task actually get completed, verified, delivered to the right place, and reflected back in Helpin?

A runtime reaching `completed` is not sufficient. It can mean only that the executor stopped successfully. A repository push can fail after execution, an output can be malformed, an expected document can be missing, or the task can remain open with no useful receipt. Some current finalizer failures are logged without changing the green terminal status.

This PRD introduces **closed-loop agent delivery**:

1. Capture a completion contract before execution.
2. Execute durably through the existing agent-run and Temporal architecture.
3. Interrupt only for meaningful input, authorization, or irreversible actions.
4. Verify results against explicit criteria and domain rules.
5. Deliver outputs to their intended destination.
6. Persist a structured delivery manifest with evidence.
7. Update the task and notify the user only after product delivery succeeds.

The primary user-facing model becomes:

- **Working** — Helpin owns the next action.
- **Needs you** — Helpin cannot safely continue without a decision, input, or connection.
- **Delivered** — the outcome is available, verified, and attached to the task.

This is primarily a Helpin product capability. Agent Runtime remains generic and gains only optional, backward-compatible primitives for evidence, checkpoints, completion candidates, and ordered event delivery. Temporal continues to own durable orchestration, retries, signals, timeouts, and recovery.

---

## 2. Product Decision

### 2.1 Positioning

Helpin is a work completion system with agents, not an agent transcript viewer.

The transcript is evidence and a recovery tool. The product outcome is the task, document, customer action, CRM update, repository change, or other delivered work.

### 2.2 Core promise

When a user gives Helpin a sufficiently defined task, Helpin should:

- start without unnecessary ceremony
- continue without babysitting
- recover from transient failures
- ask only when the user is genuinely required
- verify the output
- place it where the user expects
- leave a clear delivery receipt
- update the source task according to workspace policy

### 2.3 Product principles

1. **Completion is a product state, not a runtime exit code.**
2. **Evidence beats confident prose.** A result must point to created or changed entities, artifacts, tests, checks, or external references.
3. **Autonomy is bounded by risk.** Safe work proceeds; consequential work pauses just in time.
4. **The task is the system of record.** Runs support the task, not the other way around.
5. **Recovery is normal product behavior.** Users should not need to understand workers, event buses, or Temporal retries.
6. **Compact by default, inspectable on demand.** The dock communicates progress and decisions; run detail holds the audit trail.
7. **Runtime neutrality is preserved.** Helpin-specific completion semantics do not leak into shared Agent Runtime.

---

## 3. Problem Statement

### 3.1 Runtime completion and task completion are conflated

`agent_runs.status = completed` currently represents terminal execution. It does not prove that required outputs exist, domain validation passed, delivery succeeded, or the task is ready to close.

### 3.2 Definition of done is not first-class

Project-management tasks have a name, description, checklist, state, and implementation brief, but no normalized acceptance-criteria or agent-completion policy. Criteria can exist in planning output, prose, or checklists, but verification cannot reliably consume them.

### 3.3 Finalization is fragmented and can fail silently from the user's perspective

Delegated-run finalizers currently handle agent status, automation rules, support drafts, planning output, repository delivery, and command-bar plan advancement. They are useful, but:

- progress is stored as flags inside `agent_runs.output_summary`
- failures are logged and later finalizers continue
- the local run can remain completed even when a required finalizer fails
- malformed delegated planning output can be logged without producing a user-visible failed delivery
- summary-dependent finalizers can be skipped when the runtime summary is unavailable
- there is no single durable delivery record

### 3.4 Users have to infer outcomes from activity

The UI is strongest at displaying execution. It is weaker at answering:

- What was actually delivered?
- Where is it?
- Which criteria passed?
- What remains unresolved?
- Did Helpin update the original task?
- Can this be retried safely?

### 3.5 Attention is represented, completion is not

The notification system supports `task.agent_attention_required`, but there is no equivalent structured delivery notification. A user may receive a useful interruption and still miss the final outcome.

### 3.6 Recovery exists at infrastructure level but is not a coherent product state

Temporal provides durable execution, heartbeats, signals, and activity retries. Helpin also reconciles stale local runs. However, the user-facing model does not distinguish execution, verification, delivery, and recovery, and delegated runtime runs follow different reconciliation paths.

---

## 4. Goals

### 4.1 Product goals

1. Make a defined task reliably reach a verified, usable outcome without routine supervision.
2. Make **Working**, **Needs you**, and **Delivered** understandable from the task surface and Agent Dock.
3. Prevent a run from appearing successfully delivered when required finalization or validation failed.
4. Provide a structured, auditable delivery receipt for every applicable completed run.
5. Close or advance tasks automatically only when their completion policy permits it.
6. Make retries and recovery idempotent and safe across process, worker, runtime, and event-delivery failures.
7. Preserve compatibility for other products using Agent Runtime.

### 4.2 Engineering goals

1. Separate execution state from product delivery state.
2. Persist completion contracts, delivery manifests, and finalizer step state outside free-form output summary JSON.
3. Give Temporal an explicit verification and delivery phase.
4. Standardize ordered, versioned run projections in the frontend.
5. Establish measurable service-level objectives for progress freshness, attention, delivery, and recovery.

---

## 5. Non-Goals

- Building a full IDE, terminal replacement, or file explorer in the Agent Dock
- Exposing raw chain-of-thought or private model reasoning
- Requiring a human approval at the start of every autonomous run
- Automatically completing every task regardless of workspace policy
- Making Agent Runtime understand Helpin tasks, CRM records, support conversations, or workflow states
- Guaranteeing semantic correctness for arbitrary work solely through another LLM judgment
- Implementing arbitrary rollback for every external system in v1
- Replacing the existing notification, authorization, automation, or artifact subsystems
- Rewriting all legacy run paths before the new contract can ship

---

## 6. Personas and Jobs to Be Done

### 6.1 Task owner

**Job:** “When I assign a defined task to an agent, finish it and tell me when the usable result is ready.”

Needs:

- clear ownership of the next action
- no unnecessary start approval
- credible completion evidence
- one-click access to delivered work
- confidence that the task state reflects reality

### 6.2 Reviewer or approver

**Job:** “Show me exactly what requires my decision, with enough context to act safely.”

Needs:

- precise action and impact
- preview of affected content or records
- clear approve, reject, and request-changes paths
- assurance that an approval has not gone stale

### 6.3 Workspace administrator

**Job:** “Set autonomy, completion, notification, and risk policies once, then audit how agents applied them.”

Needs:

- defaults by agent and domain
- explicit permission boundaries
- audit history
- failure and cost visibility

### 6.4 Operator or support engineer

**Job:** “When a run is stuck or delivered incorrectly, identify the failed phase and recover without duplicating side effects.”

Needs:

- workflow, activity, finalizer, and projection state
- idempotency keys and attempt history
- safe retry from a phase or checkpoint
- raw events and logs behind an expert view

---

## 7. Terminology

| Term | Definition |
| --- | --- |
| Task | The Helpin work item or other target that defines the requested outcome. |
| Agent run | Durable record of one execution attempt against a target. |
| Runtime completion | The executor reached a terminal successful state. It is not proof of delivery. |
| Completion contract | Versioned criteria, expected deliverables, validation methods, approvals, and completion policy captured for a run. |
| Completion candidate | Runtime-neutral structured claim and evidence produced by an executor before Helpin verification. |
| Verification | Deterministic and, where appropriate, model-assisted validation of the candidate against the contract. |
| Finalization | Idempotent product side effects required after execution, such as opening a pull request or persisting a document. |
| Delivery manifest | Authoritative, immutable record of outcome, evidence, validation, destinations, unresolved items, and timings. |
| Delivery state | Product-level state describing whether the requested outcome is still working, needs the user, is being verified, was delivered, or failed. |
| Revision run | A child run created to revise a previously delivered or partially delivered outcome. |

---

## 8. User Experience Model

### 8.1 Primary states

The product should translate detailed internal state into three dominant user states:

| User state | Meaning | Primary action |
| --- | --- | --- |
| Working | Helpin owns the next action, including retry, verification, and finalization. | Watch or leave. |
| Needs you | A decision, missing input, authentication, or policy approval blocks safe progress. | Respond to the specific request. |
| Delivered | The outcome is verified, saved to its destination, and attached to the source task. | Open result, review, or request revision. |

Failures that Helpin can recover from remain **Working** with a recovery label. Irrecoverable failure becomes **Needs you** only when a useful user action exists. Otherwise it is a distinct **Failed** terminal exception with retry or support guidance.

### 8.2 Detailed delivery states

The system stores a more precise delivery state while presenting the simpler model by default:

| Delivery state | User projection | Description |
| --- | --- | --- |
| `pending` | Working | Contract exists; execution has not started. |
| `working` | Working | The executor or orchestration is active. |
| `recovering` | Working | Helpin is retrying or reconciling a transient failure. |
| `needs_input` | Needs you | Required task information is missing. |
| `needs_auth` | Needs you | A required connection or credential must be established. |
| `needs_approval` | Needs you | A specific consequential action requires approval. |
| `verifying` | Working | Runtime work ended; criteria and outputs are being checked. |
| `delivering` | Working | Verified output is being written or published to its destination. |
| `delivered` | Delivered | All required criteria and delivery steps succeeded. |
| `partial` | Needs you | Useful output exists, but one or more required criteria did not pass. |
| `blocked` | Needs you | A non-transient dependency prevents completion and user action is available. |
| `failed` | Failed | Work cannot continue and no successful delivery occurred. |
| `cancelled` | Cancelled | The user or policy intentionally stopped the work. |
| `superseded` | Delivered/hidden | A newer revision replaced this delivery. |

### 8.3 State separation

Do not expand or reinterpret the existing runtime status enum. Persist delivery state separately.

```text
Agent Runtime             Helpin delivery                    Source task
-------------             ---------------                    -----------
queued/running/paused --> pending/working/needs_you -------> stays active
completed -------------> verifying -> delivering ---------> delivered policy
failed/cancelled -------> recovering/failed/cancelled -----> stays active
```

This separation avoids breaking existing API clients and prevents a runtime terminal transition from closing a task prematurely.

---

## 9. End-to-End Journey

### 9.1 Defined task happy path

1. User creates or selects a task and assigns an agent.
2. Helpin assembles a versioned execution context and completion contract.
3. A deterministic readiness check confirms that the task can begin.
4. Helpin performs billing and authorization preflight.
5. Helpin creates the durable `agent_run` and starts orchestration.
6. The Agent Dock shows the objective, current phase, and meaningful progress.
7. Runtime tools execute within policy. Safe actions proceed without approval.
8. Runtime emits artifacts, evidence, and a completion candidate.
9. Helpin enters `verifying` and evaluates the contract.
10. Required domain finalizers run idempotently.
11. Helpin persists the delivery manifest.
12. According to task policy, Helpin closes the task or places it in review.
13. Helpin leaves a structured task activity/comment and sends a delivery notification.
14. The user opens the exact delivered result or requests a revision.

### 9.2 Insufficiently defined task

1. Readiness finds a material ambiguity that changes the outcome or risk.
2. The run becomes `needs_input` before expensive execution.
3. Helpin asks one consolidated question with assumptions and recommended defaults.
4. The response is appended to the contract as a new version.
5. The same durable run resumes where safe; otherwise a linked attempt begins.

### 9.3 Just-in-time approval

1. The agent proceeds through read-only and reversible work.
2. Immediately before a guarded side effect, it creates an interaction containing the exact action and preview.
3. Helpin validates that the approver is authorized and that the preview is still current.
4. On approval, the workflow resumes and records the decision in the manifest.
5. On rejection or requested changes, the agent revises or exits as partial/blocked.

### 9.4 Recovery

1. A heartbeat, activity, finalizer, projection, or delivery failure is detected.
2. If retryable, delivery state becomes `recovering`; the user is not asked to intervene.
3. Temporal retries with the same idempotency key and resumes from the last durable phase.
4. If recovery exhausts its budget, Helpin exposes the specific failed phase and safe actions.

---

## 10. Task Readiness and Definition of Done

### 10.1 Readiness result

Before execution, Helpin produces one of:

- `ready` — the goal, target, constraints, and required delivery are sufficient
- `ready_with_assumptions` — safe defaults are explicit in the contract
- `needs_clarification` — one or more material decisions require the user
- `rejected` — the request is unsafe, unauthorized, unsupported, or impossible

### 10.2 Minimum task definition

A task is sufficiently defined when Helpin can determine:

- the intended outcome
- the target entity or destination
- the applicable agent and tools
- constraints and boundaries
- required deliverables
- how required deliverables will be verified
- whether a human review or approval is required

The task description does not need to contain a formal specification. Helpin may derive criteria from task fields, checklists, planning briefs, repository settings, agent defaults, and workspace policy. Derived criteria must be visible before or during the run and stored in the contract.

### 10.3 Clarification policy

Ask before starting only when the answer materially changes:

- the destination or target
- an externally visible action
- acceptance criteria
- security or data scope
- cost beyond policy
- an irreversible choice

Do not ask about low-risk implementation details the assigned agent can reasonably decide.

---

## 11. Completion Contract

### 11.1 Requirements

Every deliverable run must have an immutable, versioned completion contract. Chat-only and exploratory read-only requests may opt out and remain runtime-only.

The contract is owned by Helpin, not Agent Runtime.

### 11.2 Proposed shape

```json
{
  "schema_version": "1.0",
  "objective": "Implement and deliver the defined task",
  "target": { "type": "task", "id": "task-id", "revision": 42 },
  "criteria": [
    {
      "id": "criterion-1",
      "description": "The requested behavior works for the defined scenarios",
      "required": true,
      "verification": {
        "kind": "test_command",
        "configuration": { "command_id": "repo-test-suite" }
      }
    }
  ],
  "deliverables": [
    { "id": "code-change", "kind": "repository_change", "required": true }
  ],
  "approvals": [
    { "action_class": "publish_external", "timing": "just_in_time" }
  ],
  "completion_policy": "move_to_review",
  "assumptions": [],
  "context_manifest_id": "context-id",
  "created_from": ["task", "checklist", "agent_defaults", "workspace_policy"]
}
```

### 11.3 Criterion verification kinds

Initial supported kinds:

- `artifact_exists`
- `entity_state`
- `repository_clean`
- `repository_change`
- `test_command`
- `build_command`
- `external_delivery_status`
- `document_validation`
- `checklist_complete`
- `human_review`
- `model_assessment` as supporting evidence, never the sole verifier for a deterministic criterion

### 11.4 Contract versioning

- A contract is immutable after execution begins.
- User input that changes scope creates version `n+1`.
- The run and delivery manifest reference the exact version used.
- Material target changes should create a child/revision run rather than mutating history.

---

## 12. Verification and Completion Assessment

### 12.1 Verification order

1. Validate candidate schema and required evidence references.
2. Run deterministic checks.
3. Validate that referenced entities and artifacts are accessible in the workspace.
4. Run domain-specific checks.
5. Use model assessment only for criteria that cannot be deterministic.
6. Require human review where policy specifies it.
7. Calculate final outcome: `complete`, `partial`, `blocked`, or `failed`.

### 12.2 Domain adapters

| Domain | Minimum verification |
| --- | --- |
| Repository work | Branch/commit exists; expected changes exist; configured tests/build passed; pull request or delivery target exists when required. |
| Document work | Document/version exists; required sections and schema are valid; destination and visibility match the contract. |
| Project task work | Required artifacts exist; checklist/criteria results are recorded; state transition is permitted. |
| CRM work | Target record and requested mutation exist; mutation is within authorization and current entity revision. |
| Support work | Draft or sent message exists as requested; approval and actual delivery status are distinguished. |
| Automation/planning | Output schema is valid; referenced entities were created; downstream plan state advanced successfully. |
| Generic work | Every required deliverable has an artifact or entity reference and every required criterion has evidence. |

### 12.3 Outcome rules

- `complete`: all required criteria pass and all required deliveries succeed
- `partial`: at least one useful deliverable exists, but a required criterion or delivery remains unresolved
- `blocked`: work cannot continue because of an external dependency, missing user decision, or unavailable connection
- `failed`: no usable required outcome can be delivered, or verification infrastructure itself fails beyond retry policy

A malformed or missing completion candidate must never silently become `delivered`.

### 12.4 Validation freshness

Verification records:

- input/target revision
- verifier version
- execution attempt
- start and finish time
- evidence IDs
- result and failure code

If the target changes after verification and before finalization, Helpin re-verifies or pauses for review.

---

## 13. Delivery Manifest

### 13.1 Purpose

The manifest is the authoritative receipt shown by the task, dock, run detail, notifications, APIs, and automations. It should not be reconstructed by parsing transcript prose.

### 13.2 Proposed shape

```json
{
  "schema_version": "1.0",
  "run_id": "run-id",
  "contract_id": "contract-id",
  "contract_version": 1,
  "outcome": "complete",
  "summary": "Implemented the requested behavior and opened pull request #123.",
  "criteria": [
    {
      "criterion_id": "criterion-1",
      "result": "passed",
      "evidence_ids": ["artifact-test-log"]
    }
  ],
  "deliverables": [
    {
      "kind": "pull_request",
      "title": "PR #123",
      "uri": "provider-reference",
      "artifact_id": "artifact-pr",
      "status": "delivered"
    }
  ],
  "validations": [
    { "kind": "tests", "result": "passed", "artifact_id": "artifact-test-log" }
  ],
  "mutations": [
    { "entity_type": "task", "entity_id": "task-id", "change": "moved_to_review" }
  ],
  "approvals": [],
  "unresolved_items": [],
  "timings": {
    "execution_ms": 1000,
    "verification_ms": 200,
    "delivery_ms": 300
  }
}
```

### 13.3 Manifest rules

- Store pointers to artifacts instead of large logs or document bodies.
- Sanitize secrets, credentials, private reasoning, and unapproved external URLs.
- Make delivered manifests immutable. Corrections create a superseding manifest.
- A manifest is visible only to users authorized for both the run and referenced entities.
- Webhooks and automation events use the manifest ID and outcome, not raw summary text.

---

## 14. Durable Finalization

### 14.1 Required behavior

Finalization becomes an explicit durable phase between verification and delivery.

Each finalizer step stores:

- stable step name and version
- required or optional classification
- idempotency key
- status: `pending`, `running`, `succeeded`, `retrying`, `failed`, `skipped`
- attempt count
- input/evidence references
- output references
- error code and safe message
- timestamps

### 14.2 Failure behavior

- A required finalizer failure prevents `delivered`.
- A retryable failure moves delivery to `recovering`.
- An exhausted required failure produces `partial` when usable output exists, otherwise `failed`.
- An optional enrichment failure is recorded in the manifest but may still allow delivery.
- Downstream finalizers run only when their prerequisites succeed.

### 14.3 Idempotency

All external side effects must use a deterministic idempotency key derived from workspace, run, contract version, and step. Where an external provider lacks idempotency, Helpin stores a create-before-call intent and reconciles by provider reference.

### 14.4 Migration from output-summary markers

Existing marker keys remain readable during rollout. New runs dual-write finalizer step rows and legacy flags until all consumers migrate. The free-form output summary must no longer be the authoritative finalization ledger.

---

## 15. Task Lifecycle and Completion Policy

### 15.1 Workspace and task policies

Supported completion policies:

- `auto_complete` — mark the task complete only after successful delivery
- `move_to_review` — move to a configured review state after delivery
- `leave_open` — attach the delivery but preserve task state
- `require_human_review` — delivery waits for a final human acceptance interaction

Defaults can be configured by workspace, team, agent, or task. The most specific policy wins and is copied into the completion contract.

### 15.2 Task activity

On delivery, Helpin writes one structured activity entry containing:

- delivery summary
- agent and run
- criteria pass count
- primary artifacts/destinations
- task state change
- unresolved items, if any
- action to open run detail

Do not paste a full transcript into the task.

### 15.3 Revisions

“Request changes” creates a child run linked to the prior manifest. It carries forward the previous contract and evidence, with a clear delta. The prior result remains auditable and becomes `superseded` only after the new revision is delivered.

---

## 16. Approval, Input, and Autonomy Policy

### 16.1 Default behavior

Autonomous agents should not ask for permission merely to begin. The current task assignment and run action are sufficient user intent unless workspace policy says otherwise.

### 16.2 Risk classes

| Action class | Default | Examples |
| --- | --- | --- |
| Read-only | Proceed | Read task, repository, document, CRM record. |
| Reversible internal write | Proceed and audit | Update a draft, create an internal artifact, add a task note. |
| Scoped repository write | Proceed under repository policy | Commit/push to a run branch. |
| External communication | Just-in-time approval unless explicitly delegated | Send customer email, publish message, post externally. |
| Production/publish action | Just-in-time approval | Deploy, publish document, merge protected branch. |
| Destructive/security-sensitive | Approval or deny | Delete data, change access, rotate credentials, alter billing. |

Agent-level `approval_mode = never` remains valid. Helpin must not rewrite an existing agent's configured approval mode as part of unrelated changes. Workspace authorization still enforces actions that cannot legally or safely be delegated.

### 16.3 Interaction quality

Every approval request must include:

- exact action
- target and current revision
- user-visible preview or diff
- why approval is required
- expected consequence
- approve, reject, and request-changes actions
- freshness/expiry behavior

The approval must be shown both in the dock and inside run detail. The action cannot exist only outside the drawer.

---

## 17. Execution Context

### 17.1 Context manifest

Each run references a versioned context manifest containing:

- target entity type, ID, and revision
- task description, checklist, brief, links, and attachments
- related epic, sprint, team, and dependencies
- selected repository and base branch
- agent version, tools, skills, and policy versions
- workspace/user principal and authorization scope
- referenced documents, CRM records, support conversations, or prior manifests
- explicit user-added context

### 17.2 Context UX

The dock should support explicit context selection rather than relying only on the current page:

- task, document, CRM record, support conversation, or prior run
- repository/file reference where applicable
- attachment or screenshot
- multiple context chips with removal and visibility

Context access must be revalidated at execution time; a stored manifest is not a permission bypass.

---

## 18. Temporal Orchestration and Recovery

### 18.1 Target workflow phases

```text
prepare -> execute -> verify -> finalize -> deliver -> update task -> notify
             |          |          |
             +------ recover/retry +
             |
             +------ wait for input/auth/approval signal
```

### 18.2 Temporal responsibilities

Temporal owns:

- phase orchestration
- activity retries and timeouts
- heartbeat-based liveness
- durable waits for input, authorization, and approval
- cancellation propagation
- parent/child and command-plan dependencies
- retry budget and backoff
- compensation or reconciliation activities
- continue-as-new for long histories

### 18.3 Retry policy

- Execution activities are not blindly retried if tool side effects are unknown.
- Verification is safe to retry with the same target revision.
- Finalizers are retryable only with enforced idempotency.
- Notifications are deduplicated by manifest and recipient.
- User-visible state stays `recovering` during automatic recovery.

### 18.4 Stalled run detection

Unify local and delegated liveness into a periodic reconciler that considers:

- workflow execution state
- runtime run state
- last event sequence and heartbeat
- current finalizer step
- projection lag
- expected activity timeout

The reconciler must not mark a run failed solely because one projection is delayed. It should repair projection state when authoritative upstream state is known.

### 18.5 Cancellation

Cancellation must propagate to Temporal, Agent Runtime, active tools, child runs, and pending interactions. Completed irreversible side effects are recorded rather than pretended to be rolled back. The delivery manifest for cancellation lists what already happened.

### 18.6 Checkpoints

V1 checkpoints are phase-level recovery points, not arbitrary filesystem snapshots:

- prepared context/contract
- execution continuation token where supported
- completion candidate
- verified candidate
- each finalizer output

Fine-grained workspace or repository rollback can be introduced later for runtimes that support it safely.

---

## 19. Agent Runtime Contract

### 19.1 Ownership boundary

Agent Runtime remains a shared, product-neutral execution service. It should not know Helpin workflow states or task completion policy.

| Layer | Owns |
| --- | --- |
| Agent Runtime | Run execution, tools, messages, interactions, artifacts, usage, heartbeats, optional checkpoints, completion candidate. |
| Temporal | Durable orchestration, retries, waits, signals, cancellation, recovery, compensation. |
| Helpin | Definition of done, context policy, verification, product finalization, task transition, delivery manifest. |
| UI | Working/Needs you/Delivered projection, actions, previews, evidence, audit. |

### 19.2 Backward-compatible runtime additions

Add optional capabilities through versioned endpoints/events:

- completion-candidate artifact/event
- evidence references on artifacts and tool results
- stable event sequence number and snapshot revision
- checkpoint metadata and continuation capability flags
- explicit terminal reason and retryability classification
- interaction action revision/freshness fields

Existing consumers can ignore these fields. Helpin must feature-detect runtime capabilities and fall back to current artifacts and summary behavior during migration.

### 19.3 Compatibility guarantees

- No existing run status values change meaning.
- New fields are optional.
- No Helpin-specific schemas become mandatory for other products.
- Runtime API versions are additive during the rollout window.
- Contract tests cover old producer/new consumer and new producer/old consumer combinations.

---

## 20. Proposed Data Model

### 20.1 `agent_run_completion_contracts`

| Field | Notes |
| --- | --- |
| `id`, `workspace_id`, `run_id` | Workspace-scoped identity. |
| `version`, `schema_version` | Immutable contract version. |
| `target_type`, `target_id`, `target_revision` | Exact target snapshot. |
| `objective` | User-readable goal. |
| `criteria`, `deliverables`, `approvals`, `assumptions` | JSONB with schema validation. |
| `completion_policy` | Copied effective policy. |
| `context_manifest_id` | Versioned execution context. |
| `created_by`, `created_at` | Audit fields. |

Unique index: `(run_id, version)`.

### 20.2 `agent_run_delivery_manifests`

| Field | Notes |
| --- | --- |
| `id`, `workspace_id`, `run_id`, `contract_id` | Identity and lineage. |
| `schema_version`, `revision` | Immutable manifest revision. |
| `outcome`, `summary` | Product result. |
| `criteria_results`, `deliverables`, `validations`, `mutations`, `unresolved_items`, `timings` | Structured JSONB. |
| `supersedes_manifest_id` | Revision lineage. |
| `created_at`, `delivered_at` | Audit and UX timing. |

Unique index for the active authoritative manifest per run; workspace and target indexes for history queries.

### 20.3 `agent_run_finalizer_steps`

| Field | Notes |
| --- | --- |
| `run_id`, `step_key`, `step_version` | Stable step identity. |
| `required`, `status`, `attempt_count` | State and policy. |
| `idempotency_key` | Unique side-effect key. |
| `input_refs`, `output_refs` | Artifact/entity references. |
| `error_code`, `error_message` | Safe operational detail. |
| `started_at`, `completed_at`, `updated_at` | Recovery timestamps. |

Unique index: `(run_id, step_key, step_version)` and `(idempotency_key)`.

### 20.4 `agent_runs` additions

Add projection fields for fast listing:

- `delivery_status`
- `completion_contract_id`
- `delivery_manifest_id`
- `verification_started_at`
- `delivered_at`
- `delivery_error_code`
- `state_revision`

These fields are projections. The contract, manifest, and step tables remain authoritative.

### 20.5 Task completion policy

Store workspace/team defaults in settings and allow an optional task override. Avoid adding criteria directly as many columns on `pm_tasks`; normalize agent criteria into the completion contract while continuing to use task checklist and implementation brief as inputs.

---

## 21. APIs and Events

### 21.1 Product APIs

- `GET /agent-runs/{id}/completion-contract`
- `GET /agent-runs/{id}/delivery`
- `GET /agent-runs/{id}/finalizer-steps` for authorized detail/operator views
- `POST /agent-runs/{id}/retry-delivery`
- `POST /agent-runs/{id}/request-revision`
- `POST /agent-runs/{id}/cancel`
- `POST /agent-runs/{id}/messages` for allowed steering/input
- task APIs include latest delivery summary and user-state projection

### 21.2 Realtime events

Recommended product events:

- `agent_run.delivery_updated`
- `agent_run.attention_required`
- `agent_run.delivered`
- `agent_run.delivery_failed`
- `agent_run.revision_created`

Every event envelope includes:

- workspace ID
- run ID as `parent_id`
- entity ID
- monotonically increasing run sequence or state revision
- event ID/idempotency key
- timestamp
- compact payload or invalidation hint

The frontend accepts only newer revisions and performs snapshot reconciliation after gaps.

### 21.3 Automation event

Introduce `agent_run.delivered` separately from existing `agent_run.completed`. Existing automations keep current behavior until administrators migrate. New outcome-dependent flows should use `agent_run.delivered` and may filter by manifest outcome or deliverable kind.

---

## 22. Agent Dock Requirements

### 22.1 Dock purpose

The dock is a compact control and attention surface. It should answer:

- What did I ask Helpin to do?
- Is Helpin working or waiting for me?
- What meaningful phase is active?
- What was delivered?
- Where do I go for detail?

### 22.2 Required dock behavior

1. Show one objective and one dominant status per run.
2. Group low-level tool events into meaningful operations.
3. Stream assistant text smoothly without duplicate typing indicators.
4. Show at most one active typing/working treatment for the current response.
5. Render approvals, input, and authentication requests inline with full actionable context.
6. Show a delivery receipt card with primary artifacts and “Open details.”
7. Support cancel while active, respond while waiting, and request revision after delivery.
8. Preserve active runs when the dock is minimized.
9. Support explicit context chips and attachments.
10. Never require users to inspect raw logs to learn whether work succeeded.

### 22.3 Avoiding dock bloat

Do not add a permanent file tree, terminal, raw event list, full diff viewer, or operational dashboards to the dock. These belong in run detail. The dock may show a compact diff/artifact summary and link to the exact detailed tab.

---

## 23. Run Detail Requirements

Run detail is the audit, review, and recovery surface.

### 23.1 Required sections

- overview: objective, user state, agent, target, elapsed time, usage
- plan and current phase
- transcript with smooth streaming and grouped tool activity
- interactions and decision history
- delivery manifest and criteria results
- artifacts and domain previews
- repository branch, changed files, diff, tests, commit, and pull request when applicable
- finalizer/recovery history with user-safe errors
- context and contract versions
- child, parent, handoff, and revision lineage

### 23.2 Composer and steering

- While running, allow a message to be queued or treated as a supported steering signal.
- Clearly state whether input applies immediately, at the next safe point, or creates a revision.
- After delivery, keep a composer available for “request changes” rather than hiding it.
- Do not expose unsupported steering if the runtime capability is absent.

### 23.3 Expert diagnostics

Raw events, provider payload IDs, Temporal workflow IDs, finalizer attempts, and projection revisions belong behind an operator/developer disclosure with appropriate permissions.

---

## 24. Task and Cross-Product Surfaces

### 24.1 Task detail and board

- Show the three-state projection on task cards and task detail.
- `Needs you` must outrank generic “in progress” indicators.
- Show the latest delivery and primary artifact without opening the transcript.
- Preserve task workflow state independently until delivery policy applies.

### 24.2 Agent history

- Filter by app/workspace target, user state, agent, outcome, target type, date, and artifact kind.
- Search task title, display key, run objective, agent, and manifest summary.
- Deep links must select the requested run rather than only opening the general agent surface.

### 24.3 Multiple application configurations

When Agent Runtime serves more than one product/app configuration, the UI must make the active application explicit. The application selector scopes agents, runs, permissions, and configuration. Helpin should never appear as the only implicit configuration when another app is available.

---

## 25. Notifications

### 25.1 Notification events

- **Needs you:** immediate in-app notification; email/push according to preferences and urgency
- **Delivered:** in-app notification with primary result and task; digest eligible unless explicitly watched
- **Delivery failed:** notify only when recovery is exhausted or user action is available
- **Recovered:** generally update the existing notification rather than create noise

### 25.2 Deduplication

Notification key: `(workspace_id, run_id, manifest_revision, event_type, recipient_id)`.

Repeated projections, Temporal replays, and runtime event redelivery must not create duplicate notifications.

### 25.3 Content

Delivery notifications should say what was delivered, not merely “run completed.” Include the task, primary artifact/destination, outcome, and unresolved item count.

---

## 26. Authorization, Security, and Audit

### 26.1 Authorization

- Resolve a concrete principal and workspace scope for every run.
- Authorize each tool call and finalizer side effect against current policy.
- Recheck authorization when resuming after a long wait.
- Validate that the approver is allowed to authorize the exact action.
- Enforce artifact and manifest visibility based on referenced target permissions.

The centralized approach proposed in the Agent Authorization Plan should be completed before broad autonomous external writes.

### 26.2 Secret handling

- Never persist raw credentials in events, transcripts, manifests, or tool summaries.
- Redact sensitive inputs before realtime publication.
- Store provider tokens only in the existing secret/integration boundary.
- Make auth interactions reference connection IDs, not secret material.

### 26.3 Audit

Audit records must cover:

- contract creation and changes
- tool authorization decisions
- interactions and approvers
- finalizer attempts and provider references
- verification results
- task state mutations
- manifest creation and supersession

---

## 27. Billing, Usage, and Budgets

- Keep billing preflight on every launch surface.
- Attribute execution, verification, and repair usage separately.
- Define workspace budgets for automatic retry and model-assisted verification.
- Stop automatic loops when retry/cost policy is exhausted.
- Do not double-charge or double-count tokens when the same runtime event is projected again.
- Show total outcome cost in detail; the dock may show a compact total after delivery.

---

## 28. Observability and Service Levels

### 28.1 Metrics

Product metrics:

- task-to-delivery success rate
- first-pass delivery rate
- partial/blocked/failed rate by domain and agent version
- percentage of runs requiring user intervention
- unnecessary start-approval rate
- time in Working, Needs you, Verifying, Delivering, and Recovering
- delivery opened and revision-requested rate
- task state changed after delivery

Reliability metrics:

- runtime-to-Helpin projection lag
- realtime event gap/reconciliation rate
- stuck run detection and recovery time
- finalizer attempts and failure rate by step
- duplicate side-effect prevention count
- notification deduplication rate
- snapshot/event revision conflicts

### 28.2 Initial SLOs

| Measure | Target |
| --- | --- |
| Active progress freshness, p95 | Under 2 seconds when realtime is healthy |
| Attention propagation, p95 | Under 3 seconds |
| Runtime completion to verification start, p95 | Under 5 seconds |
| Verified result to manifest persistence, p95 | Under 10 seconds excluding external provider latency |
| Automatic recovery detection | Under 2 heartbeat windows |
| Duplicate external side effects caused by replay | Zero |
| Delivered state without all required finalizers | Zero |

---

## 29. Performance and Scalability

1. Use one workspace-level run stream store, not a polling hook per visible run.
2. Consume ordered realtime events and fetch snapshots only on initial load, detected gaps, reconnect, or explicit refresh.
3. Virtualize long transcripts and artifact lists.
4. Paginate run history and support server-side search/filtering.
5. Keep dock payloads compact; load diffs, logs, and full artifacts on demand.
6. Use state revisions to prevent an older snapshot from overwriting newer events.
7. Define retention separately for transcript tokens, tool logs, artifacts, manifests, and audit metadata.

---

## 30. Accessibility and Responsive Behavior

- Every status and approval must work without color alone.
- Streaming updates use a polite live region and must not announce every token.
- Respect reduced-motion preferences for streaming and working animations.
- Approval, cancel, retry, and delivery links must be keyboard accessible.
- Focus moves into a newly opened interaction only when initiated by the user.
- Dock and detail must work at laptop and tablet widths without fixed-width clipping.
- Preserve actionable controls inside the drawer; never render the only approval action outside it.

---

## 31. Current-State Capability Review and Gaps

This review reflects the repository on 2026-07-10. It is intentionally separate from runtime parity work.

### 31.1 What is already strong

- `agent_run` is a durable universal execution record with target, runtime, invocation, hierarchy, status, usage, repository, and artifact fields.
- Temporal workflows support preparation, execution, signals for approval/input/auth, heartbeats, retries, cancellation paths, and command-plan advancement.
- Agent Runtime projection supports shared runtime execution without placing product behavior in the runtime.
- First-class messages, interactions, artifacts, and run events exist.
- The Agent Dock supports contextual launch, plan preview, fan-out/pipeline plans, active counts, cancellation, retry/resume, recent work, and compact transcripts.
- Run detail supports transcript streaming, plans, previews, interactions, failure recovery, and usage/runtime context.
- Domain finalizers already cover important repository, support, planning, automation, and command-plan behavior.
- Attention notifications are persisted and delivered through the existing notification system.

### 31.2 P0 product and correctness gaps

| Gap | Evidence in current code | Required improvement |
| --- | --- | --- |
| Runtime complete can appear green when delivery failed | Delegated finalizer errors are logged and isolated in `agent_runtime_finalizers.go`. | Add delivery state, required finalizer outcomes, and manifest gating. |
| No authoritative delivery receipt | `AgentRun.OutputSummary` and generic artifacts carry result fragments. | Persist a structured delivery manifest. |
| No first-class completion contract | `PMTask` has no acceptance-criteria/completion-policy field; current task assessment contains only summary/follow-ups. | Create immutable completion contracts derived from task/checklist/brief/policy. |
| Invalid delegated output may not fail delivery | Delegated flow-output validation logs through a finalizer while terminal status remains completed. | Make schema validation a required verification/finalizer step. |
| No delivered notification | Only `task.agent_attention_required` is implemented for agent attention. | Add deduplicated delivered and exhausted-failure events. |
| Task transition is not consistently tied to verified delivery | Completion automations fire on `agent_run.completed`. | Introduce `agent_run.delivered` and apply task policy after manifest creation. |
| Finalizer state is embedded in output JSON | Idempotency flags are stored in `agent_runs.output_summary`. | Move authoritative step state to a finalizer ledger. |

### 31.3 P0/P1 realtime and state-consistency gaps

| Gap | Current behavior | Required improvement |
| --- | --- | --- |
| Dock realtime may miss generic event routing | Dock handling expects the run in `entity_id`, while generic created events can identify it through `parent_id`; polling masks misses. | Standardize envelope semantics and reconcile by run ID/parent ID during migration. |
| Per-run polling multiplies load | Visible active execution strips can each mount a stream hook that fetches snapshot/events and polls. | Use a shared workspace stream/cache with one connection and bounded reconciliation. |
| Snapshot/event races | Concurrent session and event loads have no monotonic revision gate. | Add `state_revision`/sequence and reject stale state. |
| Stream failures are not clearly represented | Stream hook primarily exposes loading/data and relies on polling. | Surface connected, reconnecting, stale, and failed states with automatic recovery. |

### 31.4 P1 Agent Dock gaps

| Gap | Improvement |
| --- | --- |
| Transcript is intentionally flat and low-level tools can dominate. | Group tool activity into operations with expandable detail. |
| Context is mostly inferred from the page; explicit add-context support is not fully wired. | Add multi-context chips, attachments, screenshots, and accessible source labels. |
| Approval cards in compact surfaces lack the richest domain previews. | Share one interaction renderer/preview contract between dock and detail. |
| Run history is shallow and search is title-oriented. | Add server pagination, filters, target keys, outcome, and manifest search. |
| External run deep links can open the surface without selecting the requested run. | Make run ID selection deterministic in dock and detail. |
| Fixed size and timed auto-collapse can hide active context. | Make layout responsive and never auto-collapse a run that is active or needs attention. |
| Multiple streaming/typing treatments can appear. | Derive one active response indicator from canonical stream state and use token/chunk animation. |

### 31.5 P1 run-detail gaps

| Gap | Improvement |
| --- | --- |
| Composer is disabled while running and absent after completion. | Support queued steering and post-delivery revision requests. |
| Repository/diff services and components exist but are not fully wired into the main surface. | Add an on-demand Changes section with files, diff, tests, commit, and PR. |
| Completion evidence is scattered across transcript, artifacts, and summary. | Make the delivery manifest the default completion tab. |
| Checkpoint/rollback capability is not represented coherently. | Show phase checkpoints and safe retry/recovery actions first; add fine-grained rollback only where supported. |
| Safe reasoning state is underused. | Show concise phase labels, decisions, and durations; never raw chain-of-thought. |

### 31.6 P1 backend and orchestration gaps

- Unify delegated and Temporal-run reconciliation into one product liveness view.
- Add verification/finalization phases to Temporal rather than completing immediately after executor return.
- Persist finalizer dependencies and outcomes durably.
- Add completion-policy resolution and task transition service.
- Add target revision checks before side effects and final delivery.
- Add retryability/error taxonomy instead of relying on strings.
- Add provider reconciliation for side effects where response delivery can be lost.
- Add revision runs and manifest lineage.

### 31.7 P2 platform improvements

- Retention and archival policies for large transcripts and artifacts
- Workspace admin views for stuck/recovering runs and finalizer health
- Domain-specific completion analytics and agent version comparisons
- Fine-grained checkpoints/rollback for supported runtimes
- Offline/digest delivery notifications
- Cross-app configuration selector and scoped operations for shared Agent Runtime UI

---

## 32. Functional Requirements

### P0 — closed-loop correctness

- **FR-001:** Helpin must create a versioned completion contract before a deliverable run executes.
- **FR-002:** Execution status and delivery status must be stored independently.
- **FR-003:** Runtime completion must transition to verification, not directly to delivered.
- **FR-004:** All required criteria must have a recorded result and evidence before delivery.
- **FR-005:** Required finalizer failures must prevent delivered state.
- **FR-006:** Finalizer steps must be durable and idempotent.
- **FR-007:** Helpin must persist an immutable delivery manifest for complete and partial outcomes.
- **FR-008:** Task completion or review transition must follow the effective completion policy.
- **FR-009:** `agent_run.delivered` must be emitted only after manifest persistence and required task mutation.
- **FR-010:** Attention and delivery notifications must be deduplicated across replay and redelivery.
- **FR-011:** Autonomous agents must start without a generic approval unless configured policy requires one.
- **FR-012:** Consequential approvals must occur just in time with a current preview.
- **FR-013:** Automatic recovery must remain Working until retry is exhausted or user action is necessary.
- **FR-014:** Existing Agent Runtime clients must continue working without adopting Helpin completion schemas.

### P1 — top-tier product experience

- **FR-101:** Task, dock, and run detail must use consistent Working/Needs you/Delivered projection.
- **FR-102:** Dock updates must use a shared ordered realtime state store with snapshot reconciliation.
- **FR-103:** Run detail must expose criteria, artifacts, validations, destinations, and unresolved items.
- **FR-104:** Repository work must expose files, diff, tests, commit, and pull-request delivery on demand.
- **FR-105:** Users must be able to queue supported steering while active and request revision after delivery.
- **FR-106:** Context selection must support multiple explicit entities and attachments.
- **FR-107:** Deep links must open the exact run and relevant attention/delivery section.
- **FR-108:** Run history must support server-side pagination, search, and outcome/target filters.
- **FR-109:** Approval controls must be available inside every surface that presents the blocking interaction.
- **FR-110:** Streaming UI must render one canonical active-response treatment without duplicate indicators.

### P2 — operational maturity

- **FR-201:** Operators can inspect phase attempts, idempotency, workflow/runtime IDs, and projection state.
- **FR-202:** Supported runtimes can advertise checkpoint and continuation capabilities.
- **FR-203:** Admins can configure completion policy, retry budget, and notification behavior by team/agent.
- **FR-204:** The shared runtime UI can scope agents and runs by application configuration.

---

## 33. Non-Functional Requirements

- **NFR-001 Reliability:** No required side effect may execute twice because of Temporal replay or event redelivery.
- **NFR-002 Consistency:** Older snapshots/events may not overwrite a newer run state revision.
- **NFR-003 Security:** Every run, artifact, manifest, interaction, and action is workspace-scoped and permission-checked.
- **NFR-004 Privacy:** Secrets and raw private reasoning are excluded from persisted user-visible data.
- **NFR-005 Performance:** Realtime and completion latency meet the SLOs in section 28.
- **NFR-006 Scale:** UI subscriptions and polling scale with workspace activity, not visible run-row count.
- **NFR-007 Accessibility:** Core task monitoring and approval flows meet WCAG 2.2 AA.
- **NFR-008 Compatibility:** Runtime contract changes are additive and versioned.
- **NFR-009 Auditability:** Every delivered claim can be traced to a contract, verifier, evidence, and finalizer result.
- **NFR-010 Operability:** Errors use stable codes and retryability classifications.

---

## 34. Rollout Plan

### Phase 0 — instrumentation and contract foundation

- Add delivery state projection without changing current run status.
- Add event sequence/state revision and frontend reconciliation metrics.
- Persist shadow completion contracts for selected task runs.
- Persist finalizer step outcomes alongside existing summary markers.
- Add dashboards for projection lag, finalizer failures, and false-green candidates.

Exit criterion: no current behavior changes; data demonstrates contract coverage and state correctness.

### Phase 1 — verified repository and document delivery

- Add manifest, verifier, and durable finalization workflow.
- Support repository and document domain adapters first.
- Introduce `agent_run.delivered` behind a feature flag.
- Show delivery receipt in run detail and task activity.
- Keep task transitions in `move_to_review` or `leave_open` for initial cohorts.

Exit criterion: required finalizer failure cannot produce delivered state; replay tests show no duplicate side effects.

### Phase 2 — closed-loop task UX

- Ship Working/Needs you/Delivered across task, dock, and detail.
- Add delivered/failed notifications.
- Add shared realtime store, deep links, history filters, and explicit context.
- Enable configured `auto_complete` for trusted agents/workspaces.
- Add revision runs.

Exit criterion: cohort task-to-delivery success and intervention metrics meet launch thresholds.

### Phase 3 — domain expansion and recovery

- Add support, CRM, planning, automation, and generic adapters.
- Add unified liveness reconciliation and operator recovery tools.
- Add phase checkpoints and provider reconciliation.
- Add multi-app scoping to shared runtime UI.

### Feature flags

- `agent_delivery_contracts`
- `agent_delivery_verification`
- `agent_delivery_task_transition`
- `agent_delivery_notifications`
- `agent_shared_realtime_store`
- `agent_revision_runs`

Flags should be independently reversible except after an external side effect has been delivered.

---

## 35. Migration and Compatibility

1. Do not rewrite historical run statuses.
2. Historical completed runs display “Execution completed” unless a manifest is backfilled from trustworthy structured evidence.
3. During dual-write, finalizer ledger rows are authoritative for new delivery flows; legacy output-summary markers remain for old consumers.
4. Existing `agent_run.completed` automations continue to fire unchanged. New outcome-sensitive recipes use `agent_run.delivered`.
5. API clients that do not know delivery state continue to see existing run payloads.
6. The runtime capability handshake determines whether completion candidates/checkpoints are available.
7. If a runtime does not support the new candidate schema, Helpin can build a limited candidate from structured artifacts but must not invent deterministic evidence.

---

## 36. Testing Strategy

### 36.1 Unit and contract tests

- contract derivation and versioning
- task completion-policy resolution
- manifest schema and immutability
- verification outcome calculation
- finalizer dependency graph and retryability
- idempotency-key generation
- event revision ordering
- runtime old/new compatibility matrices
- permission and redaction rules

### 36.2 Repository/service tests

- workspace isolation for all new tables and APIs
- unique constraints under concurrent finalization
- replayed runtime terminal event
- process crash before/after external call and before/after ledger update
- missing runtime summary
- malformed candidate and missing artifact
- target revision changes during run
- duplicate notification delivery

### 36.3 Temporal tests

- prepare, execute, verify, finalize, deliver happy path
- each activity retry and exhausted retry path
- approval/input/auth signals at every wait state
- cancellation during every phase
- worker restart and workflow replay
- finalizer idempotency after activity timeout
- command-plan parent advancement only after child delivery policy
- continue-as-new for long interactive runs

### 36.4 Frontend tests

- event arrives before/after snapshot and stale snapshots are ignored
- event gap triggers one reconciliation
- parent-ID and entity-ID event routing during migration
- multiple visible runs use one stream subscription
- one streaming/typing treatment per response
- approval appears and works inside dock and drawer
- delivery receipt links to exact artifact and run
- composer queues input while active and creates revision after delivery
- reduced motion, keyboard navigation, screen-reader announcements
- responsive dock/drawer behavior

### 36.5 End-to-end scenarios

1. Repository task delivers tested pull request and moves to review.
2. Runtime succeeds but push fails; UI shows recovering, then delivered after retry.
3. Tests fail; useful patch exists; run becomes partial with evidence and does not close task.
4. Customer message requires approval only immediately before send.
5. Agent needs OAuth; sign-in resumes the same workflow.
6. Runtime terminal event is delivered three times; one task transition and one notification occur.
7. Worker dies after provider success but before response persistence; reconciliation finds the existing provider object.
8. User changes target during verification; stale delivery is prevented.
9. User requests revision after delivery; linked child run supersedes prior manifest only on success.
10. Other Agent Runtime product ignores optional completion fields without regression.

### 36.6 Chaos and load tests

- NATS disconnect/reconnect and out-of-order delivery
- Temporal worker loss
- runtime API timeout
- database failover during finalization
- websocket reconnect storms
- thousands of active runs with one shared workspace subscription

---

## 37. Success Metrics and Launch Gates

### 37.1 Success metrics

- At least 90% of eligible completed runtime runs produce a manifest in the first release cohort.
- At least 80% of sufficiently defined eligible tasks reach Delivered without user intervention, excluding policy-required approval.
- Less than 5% of Delivered runs receive a “result missing/wrong destination” report.
- False-green delivery rate is below 0.5% and trends toward zero.
- Median user attention requests per run decreases without increasing failed delivery.
- Duplicate external side effects caused by replay remain zero.
- Delivered notification open-through to the primary artifact is measurable and improves over transcript-only completion.

### 37.2 Launch gates

- Required finalizer failure cannot create delivered state.
- All new side effects have idempotency/reconciliation tests.
- Authorization review is complete for enabled domain adapters.
- Realtime state ordering tests pass under event/snapshot races.
- Operational dashboard and runbook exist.
- Feature flags and rollback behavior are tested.
- Helpin and at least one non-Helpin Agent Runtime consumer pass compatibility tests.

---

## 38. Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Completion contracts make simple tasks feel heavy. | Derive contracts automatically and show a concise summary; formal UI only when needed. |
| Verification adds latency and cost. | Prefer deterministic checks, parallelize independent checks, cache immutable evidence, set budgets. |
| Users trust “Delivered” too broadly. | Define domain-specific guarantees and show criteria/evidence; do not claim semantic certainty beyond checks. |
| Finalizer ledger and state model increase backend complexity. | Keep state ownership explicit, use one orchestration path, and ship adapters incrementally. |
| Shared runtime changes break another product. | Make capabilities optional/additive and maintain cross-version contract tests. |
| Automatic task completion surprises teams. | Default initial rollout to review/leave-open; require explicit opt-in for auto-complete. |
| External provider success is lost after timeout. | Use idempotency keys and provider reconciliation before retry. |
| Dock becomes an IDE-like surface. | Enforce compact dock scope and move deep inspection to run detail. |
| Model verifier rubber-stamps model output. | Require deterministic evidence where possible and mark model-only criteria clearly. |

---

## 39. Decisions Required Before Implementation

1. Which domains enter Phase 1? Recommendation: repository delivery and document creation/versioning.
2. What is the initial default task policy? Recommendation: `move_to_review` for eligible tasks, not `auto_complete`.
3. Does a partially passing contract always require user action, or can optional criteria produce Delivered with warnings? Recommendation: only required criteria gate delivery.
4. Which existing `agent_run.completed` automations should be offered a one-click migration to `agent_run.delivered`?
5. What target revision mechanism is authoritative for task, document, CRM, and support entities?
6. How long should completion candidates, detailed tool logs, and validation artifacts be retained?
7. Which steering modes can each runtime safely advertise: immediate, next-safe-point, or revision-only?
8. Should human final acceptance create a new manifest revision or complete an existing pending-review manifest? Recommendation: new immutable revision recording the reviewer.

---

## 40. Recommended Implementation Workstreams

### Workstream A — product completion model

- schemas and migrations
- contract derivation
- verification engine
- manifest service
- task completion policy

### Workstream B — durable orchestration

- Temporal phases
- finalizer ledger
- idempotency and provider reconciliation
- unified liveness and recovery

### Workstream C — runtime contract

- sequence/revision contract
- completion candidate and evidence
- capability handshake and compatibility tests
- checkpoint metadata

### Workstream D — user experience

- shared realtime store
- task/dock/detail three-state model
- delivery receipt and domain previews
- steering/revision flows
- history, deep links, context selection

### Workstream E — trust and operations

- centralized authorization
- notifications
- observability/SLO dashboards
- operator diagnostics and runbook
- billing and retry budgets

These workstreams can proceed in parallel after the state model and ownership boundaries are accepted, but delivery state, manifest schema, and event ordering must be agreed before UI and adapter implementation diverge.

---

## 41. Final Recommendation

The most important improvement is not more visible agent activity. It is making Helpin own the gap between **the executor stopped** and **the user's work is done**.

The release should prioritize, in order:

1. Separate delivery state from runtime state.
2. Add completion contracts and delivery manifests.
3. Make verification and required finalization durable Temporal phases.
4. Tie task transitions and notifications to delivered state.
5. Present Working, Needs you, and Delivered consistently.
6. Fix realtime state correctness and use a shared stream store.
7. Add domain-specific evidence and previews in run detail.

This creates a top-tier experience without bloating the dock or turning Helpin into an IDE. Users see less machinery, receive fewer unnecessary interruptions, and gain a much stronger reason to trust that assigned work will actually be finished.
