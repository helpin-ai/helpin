# Spec Planning Runtime

## Overview

This document describes the runtime behavior for epic planning in Teampulse.

The planning model is:

- epic is the workflow anchor
- Docs is the canonical spec store
- the epic planning repository is the live code source for both `draft_spec` and `plan_stories`
- `agent_run` is the execution and audit container
- story creation is a confirmation step, not a model side effect
- story execution remains story-scoped
- open questions and assumptions must be resolved by a human before spec approval

Teampulse exposes one user-visible planning class, `product_planner`, while applying stage-specific prompt packs internally.

## Planning Stages

Two product-planner run stages are currently implemented:

- `draft_spec`
- `plan_stories`

The stage is stored on `agent_runs.input.stage` and passed into the shared worker executor.

## Planning Methodology

The worker resolves `planning_methodology` for every epic planning run.

Sources in priority order:

1. explicit value on the queued run input
2. workspace setting from `workspace_settings.planning_methodology`
3. default `structured_v1`

Current supported values:

- `structured_v1`
- `basic_v1`

The chosen methodology is stamped into both run input and normalized run summary for auditability.

## Input Resolution

When an epic run starts, the worker activity resolves:

- `stage`
- `additional_context`
- `spec_document_id`
- `spec_version_id`
- `planning_methodology`
- `planning_web_search_enabled`
- `planning_web_search_provider`

If a `draft_spec` run does not already have a spec document, the activity creates one before execution.

## Prompt Assembly

Epic prompt assembly is methodology-aware and stage-specific.

Rules:

- `product_planner` methodology is authoritative
- workspace methodology controls the hidden prompt pack
- `planning_notes` append planner-specific context
- legacy planner `system_prompt` values are treated as secondary advanced notes
- JSON contracts remain fixed regardless of methodology

### `structured_v1`

`draft_spec` uses an internal analyst + PM stance:

- synthesize problem, user impact, goals, non-goals, requirements, scenarios, risks, and open questions
- keep the draft product-facing
- include a built-in self-check before final output

`plan_stories` uses an internal architect + scrum-master stance:

- align work to the approved spec and live code context
- prefer module-boundary-aware story decomposition
- require explicit dependency edges, risks, and open questions
- keep story titles flat and outcome-oriented instead of using phase prefixes
- include a built-in self-check before final output

### `basic_v1`

`basic_v1` keeps simpler stage guidance with the same JSON contracts, but omits the richer structured checklists and hidden role framing.

## Draft Spec

The runtime provides:

- epic title and description
- current canonical spec document content, if any
- docs linked to the epic
- support tickets linked to stories in the epic
- operator notes from `additional_context`
- planning repository clone plus bounded read-only repo context
- optional external web research through `web_search` when enabled in workspace settings
- a normalized section template for the spec

The product planner must return JSON with:

- `title`
- `summary`
- `spec_markdown`
- `risks`
- `assumptions`
- `open_questions`
- `sources`

## Plan Stories

The runtime provides:

- approved spec version ID
- approved spec snapshot text
- planning repository clone
- other docs linked to the epic
- support tickets linked to epic stories
- operator notes from `additional_context`
- ephemeral live code context built from the current repository state

The product planner must return JSON with:

- `summary`
- `spec_version_id`
- `proposed_stories`
- `open_questions`
- `risks`

## Run Artifacts

`draft_spec` artifacts:

- `product_spec_draft`
- `external_research_sources` when citations were used
- standard execution artifacts such as `conversation_log`

`plan_stories` artifacts:

- `story_plan_proposal`
- `orchestration_proposal`
- standard execution artifacts

The run `output_summary` is normalized after execution so downstream services see stage-aware fields instead of raw model output only.

The ephemeral code-context summary used by `plan_stories` is not stored as a first-class artifact in v1.

## Docs Persistence Rules

After a successful `draft_spec` run:

1. the markdown draft is normalized with a `Research Sources` section when structured citations are present
2. the markdown draft is converted into TipTap-compatible Docs JSON
3. current document content is upserted
4. a `DocsVersion` snapshot labeled `AI Draft` is created
5. document title and excerpt are refreshed from the draft output
6. `assumptions` and `open_questions` are converted into persisted epic clarification items
7. the epic is moved to `awaiting_spec_clarification` when clarification items exist, otherwise to `awaiting_spec_approval`

Important:

- the spec content itself only lives in Docs
- the run artifact is evidence of the AI draft, not the canonical copy

## Approved Spec Version Semantics

Approving a spec writes `pm_epics.approved_spec_version_id`.

That value means:

- all downstream planning and execution handoff should be traceable to this exact version
- later document edits do not retroactively change the approved planning snapshot
- a new draft can exist while an older version remains the approved one until a human approves a newer version
- the spec cannot be approved while unresolved clarification items remain
- when clarification items exist, Teampulse appends a normalized `Clarifications` section into the current Docs content before creating the approved version

## Clarification Loop

Clarification items are first-class workflow state on the epic.

Each item stores:

- kind: `open_question` or `assumption`
- prompt
- disposition
- optional response

Resolution rules:

- open questions must be marked `answered` and include a response
- assumptions must be marked `accepted` or `rejected`
- rejected assumptions require an explanatory response

Planning implications:

- unresolved clarification items block spec approval
- resolved clarification items are injected into `plan_stories`
- clarification items are refreshed on every successful `draft_spec` rerun

## Story Plan Validation

Before a story plan is accepted as a reviewable proposal, the runtime normalizes and validates:

- each story has a name
- each story has at least one acceptance criterion
- each story has a stable `ref`
- refs are unique
- dependency refs point to known stories
- a story cannot depend on itself
- circular `blocks` chains are rejected
- story names should remain flat and avoid phase-style prefixes

This keeps confirmation deterministic and avoids creating invalid story graphs later.

## Live Code Context

`plan_stories` is code-aware through an epic-level planning repository.

Rules:

- each epic has one canonical planning repository in v1
- `draft_spec` also uses targeted repo context, but stays product-led
- `plan_stories` fails if the epic has no planning repository configured
- `draft_spec` also fails if the epic has no planning repository configured
- the worker clones the repo at run time using the existing git integration credentials

The targeted scan reads:

- repo tree at bounded depth
- key manifests and config files
- architecture anchors such as handlers, services, routers, models, pages, and tests
- a bounded set of likely relevant files selected from approved-spec keywords

The output is a temporary prompt block describing:

- likely stack and app shape
- relevant modules and files
- existing patterns to follow
- ambiguity or conflicts that should become risks or open questions

For `plan_stories`, the approved spec snapshot is augmented with resolved clarification items so the planner decomposes from explicit human decisions instead of stale ambiguity.

This context is ephemeral by design and is never persisted as canonical Docs content.

## Story Dependency Semantics

Teampulse follows Shortcut-style dependency semantics for stories:

- `blocks` means the source story must complete before the target story can start
- `blocked_by` and `blocking` are derived read-model views on top of `pm_story_links`
- board/list `blocked` badges only represent active unresolved inbound blockers
- completed blockers remain visible in story detail/history, but no longer keep the story actively blocked
- external blocker notes remain separate from story-to-story relationships
- story relationships are managed in the shared `Associations` surface alongside support, CRM, and docs links

## External Research

`draft_spec` can optionally use external web research when enabled in workspace AI settings.

Rules:

- `web_search` is only exposed to `product_planner`
- `web_search` is only available during `draft_spec`
- Brave Search is the first supported provider
- citations are persisted into the saved spec as `Research Sources`
- external research is not exposed during `plan_stories` in v1

## Confirmation Idempotency

Story creation happens in the API service, not inside the model run.

If the same approved planning run is confirmed again:

- existing `created_story_ids` are returned
- stories are not duplicated
- dependency links are not duplicated

This matters because planning approval is a human step and retries can happen through the UI.

## Rollback And Rerun Behavior

### Draft reruns

- create a new Docs content snapshot
- create another `AI Draft` version
- update `last_planning_run_id`
- replace the current clarification item set with the new draft's assumptions and open questions
- move the epic back to `awaiting_spec_clarification` when clarification is needed, otherwise `awaiting_spec_approval`

### Plan reruns

- create a fresh proposal artifact
- do not overwrite prior approved plan artifacts
- leave already created stories untouched unless a human confirms the new run separately

### Failure behavior

- activity failures mark the run as failed
- epic planning state only advances after successful post-processing
- cancellation leaves prior approved spec/story data intact

## Linked Context Gathering

The runtime currently gathers context from:

- epic metadata
- docs linked to the epic through `docs_links`
- support tickets linked to stories already in the epic
- operator notes from the request payload

This is intentionally conservative for v1. CRM-native context can plug into the same stage input builder later.

## No Git Sync

The planning runtime does not export to Git or mirror OpenSpec files in v1.

Reasons:

- Teampulse Docs already supports editing, versioning, and linking
- a second canonical source would create drift immediately
- planning is collaborative across PM, support, and later CRM, not repo-only

If Git export is added later, it should be derived from the approved Docs version, not peer to it.

## Future CLI Execution

Future `codex_cli` or `claude_cli` execution does not require a new planning model.

The current handoff contract already contains the right inputs:

- approved spec snapshot
- planned story description
- acceptance criteria
- dependency refs
- source refs

That means CLI engineering runtimes can replace or supplement the current shared API-style story executor without changing:

- Docs as canonical spec storage
- epic planning stages
- plan confirmation semantics
- story-scoped delivery
