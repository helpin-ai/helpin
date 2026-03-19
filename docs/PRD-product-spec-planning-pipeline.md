# PRD: Product-Spec Planning Pipeline

## Status

Implemented in v1 behind the epic planning surface.

## Summary

Teampulse treats epic planning as a staged, docs-first pipeline:

1. An epic gets a canonical `product_spec` document in Docs.
2. A `draft_spec` product-planner run uses the epic planning repository plus optional external web research, writes a fresh cited draft into that document, and snapshots an `AI Draft` version.
3. The planner's assumptions and open questions become explicit clarification items on the epic.
4. A human resolves those clarifications, edits the document in Docs if needed, and then explicitly approves a version.
5. A `plan_stories` product-planner run reads the approved version plus resolved clarifications and proposes a dependency-aware story plan.
6. A human confirms the plan.
7. Teampulse creates stories and story dependency links.
8. Optional story-level execution starts from the created stories, never from the epic directly.

The product model is intentionally opinionated:

- one user-visible planning class: `product_planner`
- strict target mapping by agent class
- one workspace-level planning methodology
- hidden stage personas and checklists behind epic planning

This follows the staged interaction pattern from GitHub Spec Kit, the structured requirement style from OpenSpec, the role separation used by BMAD, and dependency-aware task planning similar to Taskmaster, but the runtime remains Teampulse-native.

## Goals

- Make Docs the canonical planning source of truth.
- Preserve explicit human approval boundaries between draft spec and story creation.
- Keep planning auditable through `agent_run` artifacts and immutable doc versions.
- Support dependency-aware story creation instead of flat story lists.
- Keep story titles flat and vertical; use dependency links instead of phase prefixes.
- Collapse `planner` and `orchestrator` into one product-facing `product_planner` concept.
- Make invalid agent usage impossible through strict target mapping.
- Keep planning methodology at workspace scope, not hidden inside individual planner prompts.

## Non-Goals

- Git-based spec syncing or OpenSpec file export in v1.
- Free-floating PRD authoring outside the epic workflow.
- Epic-level code branches.
- Public BMAD-style agent catalogs or persona selection.
- Separate persisted analyst, PM, architect, or scrum-master agents in v1.
- Methodology packs for `engineer`, `reviewer`, or `support` in v1.

## Source Of Truth Rules

- Canonical product-spec content lives only in Docs.
- `pm_epics.spec_document_id` points to the canonical spec document.
- `pm_epics.planning_repository_id` points to the live repository used for both repo-aware PRD drafting and code-aware story planning.
- `pm_epics.approved_spec_version_id` points to the exact doc version approved for downstream planning.
- Story plans are review artifacts on `agent_runs`, not the canonical spec.
- Story descriptions embed traceability and acceptance criteria for execution, but they do not replace the approved spec.
- Workspace-level `planning_methodology` controls hidden epic-planning prompt packs.

## Data Model

### Docs

- New doc type: `product_spec`
- System space: `Product Specs` with slug `product-specs`

### Epics

New epic fields:

- `spec_document_id`
- `planning_repository_id`
- `planning_state`
- `approved_spec_version_id`
- `last_planning_run_id`

Planning states:

- `not_started`
- `awaiting_spec_clarification`
- `awaiting_spec_approval`
- `ready_for_story_planning`
- `awaiting_plan_approval`
- `stories_created`
- `execution_started`
- `ready_for_execution`

### Workspace settings

New workspace setting:

- `planning_methodology`
- `planning_web_search_enabled`
- `planning_web_search_provider`

Current supported values:

- `structured_v1`
- `basic_v1`

### Story links

New table: `pm_story_links`

V1 link types:

- `blocks`
- `relates_to`
- `duplicates`

Current planning flow writes `blocks` links.

### Agent payloads

`draft_spec` output:

- `title`
- `summary`
- `spec_markdown`
- `risks[]`
- `assumptions[]`
- `open_questions[]`
- `sources[]`

`plan_stories` output:

- `summary`
- `spec_version_id`
- `proposed_stories[]`
- `open_questions[]`
- `risks[]`

Per proposed story:

- `ref`
- `name`
- `description`
- `story_type`
- `estimate`
- `priority`
- `acceptance_criteria[]`
- `dependency_refs[]`
- `source_refs[]`
- `assign_agent_id`

## Opinionated Agent Model

User-visible agent classes:

- `product_planner`
- `engineer`
- `reviewer`
- `support`
- `human`

Strict target mapping:

- `product_planner` -> epics only
- `engineer` -> stories only
- `reviewer` -> stories only
- `support` -> support tickets only

Compatibility aliases:

- `planner` -> `product_planner`
- `orchestrator` -> `product_planner`
- `reviewer_tester` -> `reviewer`

## Planning Methodology

The planning methodology is a workspace-level setting under `Project Settings > AI`.

### `structured_v1`

Recommended default. Internal behavior:

- `draft_spec`: analyst + PM stance
- `plan_stories`: architect + scrum-master stance
- built-in self-check before final output

### `basic_v1`

Fallback/testing option. Keeps simpler stage guidance with the same output contracts.

Important:

- methodology is applied internally through prompt packs
- users do not choose from public BMAD-style planner personas
- `planning_notes` can append context for a `product_planner`, but cannot override the methodology contract

## Workflow

### Stage 1: Draft Spec

Entry point:

- `POST /api/pm/epics/{id}/draft-spec`

Behavior:

- Ensures the epic has a linked `product_spec` doc.
- Gathers epic description, linked docs, tickets already linked to epic stories, and operator notes.
- Runs the product planner in `draft_spec` mode.
- Applies the workspace planning methodology during prompt assembly.
- Persists the returned markdown into Docs content.
- Creates a `DocsVersion` snapshot labeled `AI Draft`.
- Extracts `assumptions` and `open_questions` into persisted epic clarification items.
- Updates the epic to `awaiting_spec_clarification` when clarifications exist, otherwise `awaiting_spec_approval`.

Artifacts:

- `product_spec_draft`
- normal run logs (`conversation_log`, tool logs, etc.)

### Stage 2: Clarify Spec

Entry point:

- `POST /api/pm/epics/{id}/clarify-spec`

Behavior:

- Persists human answers to open questions and human decisions on assumptions.
- Requires rejected assumptions to include an explanatory note.
- Keeps the epic in `awaiting_spec_clarification` until every clarification item is resolved.
- Moves the epic to `awaiting_spec_approval` once all clarification items are resolved.

### Stage 3: Approve Spec

Entry point:

- `POST /api/pm/epics/{id}/approve-spec`

Behavior:

- Blocks approval while clarification items remain unresolved.
- Syncs a normalized `Clarifications` section into the current Docs content when clarification items exist.
- Approves either an explicit doc version or the current Docs content.
- Writes `approved_spec_version_id`.
- Moves the epic to `ready_for_story_planning`.
- Approves any active `draft_spec` run that is waiting on approval.

Important semantic:

- approval pins a specific doc version, not “whatever the document says later”

### Stage 4: Plan Stories

Entry point:

- `POST /api/pm/epics/{id}/plan-stories`

Behavior:

- Requires an approved spec version.
- Requires a configured epic planning repository.
- Supplies the approved spec snapshot, resolved clarifications, linked docs, linked tickets, operator notes, and ephemeral live code context from the planning repository to the product planner.
- Applies the workspace planning methodology during prompt assembly.
- Produces a reviewable proposal with stories, dependencies, risks, and open questions.
- Moves the epic to `awaiting_plan_approval`.

Code-context rules:

- the repo is cloned at run time
- a bounded targeted scan reads manifests, architecture anchors, and likely relevant files
- the resulting implementation summary is injected into the prompt only
- no code-derived doc is persisted in Docs

Artifacts:

- `story_plan_proposal`
- `orchestration_proposal` for backward compatibility

### Stage 5: Confirm Plan

Entry point:

- `POST /api/pm/agent-runs/{id}/confirm-orchestration`

Behavior:

- Validates story refs and dependency edges.
- Rejects self-dependencies and circular `blocks` graphs.
- Creates PM stories.
- Writes `pm_story_links` from dependency refs.
- Writes `blocks` story links and lets runtime read models derive active blocked state from unresolved inbound dependencies.
- Stores created story IDs back on the run summary.
- Moves the epic to `stories_created`.

Confirmation idempotency:

- re-confirming an already approved run returns the already created stories instead of creating duplicates

### Stage 5: Kick Off Execution

Entry point:

- `POST /api/pm/epics/{id}/kickoff-execution`

Behavior:

- starts story-level runs only
- includes the approved spec snapshot, planned acceptance criteria, dependencies, and source refs in `additional_context`
- skips stories without assigned agents
- skips stories without delivery targets when the assigned profile requires a repo
- moves the epic to `execution_started` or `ready_for_execution`

## UI States

Epic planning is exposed as four tabs:

- `Spec`
- `Review`
- `Stories`
- `Execute`

The docs editor remains the actual place where the spec is edited.

Other relevant surfaces:

- Agents page now creates agents from fixed class templates
- Support page only allows `support` agents to be assigned
- Story delivery only allows `engineer` and `reviewer`
- Project Settings > AI owns planning methodology selection

## Failure Modes

- No product planner assigned: draft/plan actions fail.
- No approved spec version: story planning is blocked.
- No planning repository: code-aware story planning is blocked.
- Invalid story refs or circular dependencies: confirmation fails.
- Missing story agent or delivery target: execution kickoff skips the story and reports why.
- Activity failure during draft persistence: run fails and the epic state is not advanced.

## Why No Git Sync In V1

- Docs already exists and supports editing, versioning, and linking.
- A second Git-backed spec source would create drift immediately.
- The product workflow is collaborative PM + support + CRM, not repo-only planning.
- Git export can be added later as a derived artifact without changing the canonical model.

## Acceptance Criteria

- An epic can create and link a `product_spec` doc automatically.
- `draft_spec` runs save spec content into Docs and snapshot an `AI Draft` version.
- Humans can approve a concrete doc version and pin `approved_spec_version_id`.
- `plan_stories` runs are blocked until a spec version is approved.
- Story plans include acceptance criteria, dependencies, risks, and open questions.
- Confirming a plan creates stories exactly once.
- Dependency edges are stored in `pm_story_links`.
- Story execution kickoff includes approved spec context and never creates epic-level branches.
- Agent creation and assignment only expose the fixed user-visible classes.
- Invalid agent-to-target combinations are rejected server-side.
- Workspace settings persist and expose `planning_methodology`.

## Inspiration

- OpenSpec: structured requirements, scenarios, and brownfield-friendly spec conventions
- GitHub Spec Kit: staged `specify -> plan -> tasks` workflow
- BMAD Method: analyst/PM/architect/scrum-master role separation inspiration
- Taskmaster: dependency-aware task decomposition
- GTPlanner: modular PRD-generation flow
- DevRev: support-to-product workflow benchmark

These sources inform hidden prompt-pack design and workflow shape only. They are not exposed as product-level agent personas.
Validation rules before story creation:

- every story must include acceptance criteria
- story refs must be unique
- dependency refs must be valid and acyclic
- plans should prefer vertical, user-visible slices; enabler stories are exceptions
