# Agent Skill System Plan

## Status

Phased rollout for replacing heuristic system-prompt refresh logic with a unified agent-skill system.

Execution order:

- PR1: packaged built-in skill foundation
  - explicit prompt ownership and template versioning
  - system skills stored as packaged Agent Skills directories
  - embedded built-in skill loader/parser
  - presets declare ordered built-in skills
  - built-in skill catalog endpoint
  - managed prompts stored as `nil` and materialized for response/runtime use
- PR2: workspace skill library and import flow
  - users can import downloaded Agent Skills packages into Helpin and store them in a workspace library
- PR3: typed `agent.Skills` references
  - agents attach built-in and workspace skills through one typed contract with backward-compatible decoding
- PR4: frontend skill library and skill picker
- PR5: runtime-native skill staging/loading for Codex and OpenCode
  - before launching the runtime, Helpin stages or installs only the skills allowed for that agent into the runtime-visible skill location

This plan supersedes the narrower "instruction module" framing. Built-in prompt fragments are still part of the solution, but they are treated as first-class packaged agent skills and designed to become the same system users will later import and attach.

## Goal

Fix three connected problems in the current agent system:

1. prompt refresh is currently detected through brittle string-marker heuristics in `agent_system_prompts.go`
2. planner prompts, especially Epic Planner, are monolithic hardcoded strings even though they contain several reusable instruction blocks
3. `agent.Skills` exists on the model but is passive metadata only; it has no catalog, no validation, no typed contract, and no runtime effect

The target outcome is one skill system where:

- presets reference built-in skills
- built-in system agents use those preset skill bundles
- custom agents can attach built-in skills and workspace skills
- users can import skills downloaded from elsewhere into Helpin
- system skills and imported skills share the same on-disk authoring format: `SKILL.md` plus optional `agents/openai.yaml`
- all runtimes consume one Helpin-resolved effective prompt in phase 1
- Codex and OpenCode can later receive only the agent-allowed skills staged into their visible skill directories before execution

## Scope

The document describes the full end state, but implementation starts with PR1 only:

- explicit prompt ownership and template versioning
- built-in system skills stored as packaged Agent Skills directories
- an embedded loader/parser for packaged skills
- presets referencing ordered built-in skills
- prompt compilation from built-in skills
- a built-in skill catalog API

Later phases add:

- a workspace skill library for imported and Helpin-authored skills
- typed `agent.Skills` references
- frontend surfaces for listing, importing, and attaching skills
- runtime-native skill staging/loading for runtimes that support filesystem skill discovery

## Non-Goals

This phase does not aim to:

- execute arbitrary code shipped inside external skill packages
- auto-grant tools when a skill is attached
- build a public marketplace or remote package index
- implement runtime-native Codex/OpenCode skill staging in PR1
- remove custom freeform `system_prompt` overrides

## Current Code-Backed Findings

### 1. `agent.Skills` is present but unused as a real contract

Today `agent.Skills` is stored as `json.RawMessage`, decoded as `[]string`, and only rendered as a markdown bullet list in `server/internal/worker/prompt.go`. It is not validated and has no effect on runtime behavior.

### 2. Presets already behave like implicit skill bundles

`AgentPresetDefinition` already owns runtime, tools, targets, modes, and a system prompt. In practice, the preset prompt is the real behavior bundle. This makes presets the correct place to declare default skill composition.

### 3. Epic Planner and Task Planner contain obvious reusable skills

The current planner prompts already split naturally into reusable blocks:

- approval/checkpoint protocol
- PRD authorship guidance
- task decomposition methodology
- planner state routing
- general behavior rules
- task-planner-specific framing

### 4. Versioning alone does not solve prompt upgrades

The current `*NeedsRefresh` helpers are doing two jobs:

- detecting legacy prompts
- deciding it is safe to overwrite a stored prompt

That means the new system must track prompt ownership explicitly, not just prompt version.

### 5. All runtimes already rely on one Helpin-owned prompt shape

`native_sdk`, `codex`, and `opencode` all consume the same Helpin-built prompt in the end. This means phase 1 can resolve skills into one effective prompt without runtime-specific adapter work.

## Core Decisions

### 1. One unified skill system

Do not create separate concepts for:

- internal prompt modules
- agent skills
- imported runtime skills

These are one system.

Use the term `skill` everywhere.

Built-in planner prompt fragments are built-in skills. Imported external skills become workspace skills after normalization. Agents reference skills through one contract.

### 2. Separate skill definitions from skill references

A skill definition describes reusable behavior.

An agent skill reference says which skills an agent uses.

Recommended split:

- `SkillDefinition`
  - the reusable skill record or built-in definition
- `AgentSkillRef`
  - the typed reference stored on an agent or preset

### 3. Presets become default skill bundles, not raw prompt blobs

Presets still own:

- runtime kind
- allowed tools
- allowed commands
- allowed targets
- approval mode
- default invocation mode

But preset behavior should be authored through:

- `instruction_preamble`
- ordered `instruction_skills`

The preset `SystemPrompt` remains a derived compatibility field, not the primary authored source.

### 4. Explicit prompt ownership

Prompt ownership must be explicit.

For agent rows:

- `system_prompt = nil` means managed prompt
- `system_prompt != nil` means custom override

Managed prompts are derived from skills and preset framing.

Custom prompts remain user-authored and are never auto-overwritten.

### 5. Template version applies only to managed prompts

`instruction_template_version` is meaningful only when the prompt is managed.

When a user sets a custom `system_prompt`:

- keep the custom prompt
- clear `instruction_template_version`
- stop automatic prompt upgrades for that agent until it returns to managed mode

### 6. Agent Skills packages are the source of truth

Helpin should adopt the Agent Skills package format as the canonical source of truth for both:

- built-in system skills
- imported workspace skills

Canonical package structure:

- `SKILL.md`
- optional `agents/openai.yaml`
- optional `scripts/`
- optional `references/`
- optional `assets/`

Helpin may normalize parsed metadata into database records for indexing and policy checks, but the authored workflow itself should still come from the packaged skill contents.

### 7. Skills are behavioral only

Skills contribute instruction text and metadata only.

They may declare:

- required tools
- supported runtimes
- description and provenance metadata

They must not:

- grant tools implicitly
- change runtime policy directly
- bypass target or tool restrictions

Capability policy remains owned by presets and agent policy.

### 8. Runtime delivery is phased

Phase 1:

- Helpin resolves skills into one effective prompt for all runtimes

Later phases:

- runtime-native loading for Codex/OpenCode style runtimes by staging only the resolved allowed skills for the agent before launch
- optional export for Claude Code and other compatible runtimes
- optional filesystem/package generation when a runtime can consume skills natively

This phase should be designed so later adapter-native loading does not require redefining the skill contract.

## Target Model

## Skill sources

The system should support three skill sources:

- `built_in`
  - product-owned packaged skills shipped with Helpin
- `imported`
  - packaged skills downloaded elsewhere and installed into a Helpin workspace
- `workspace`
  - skills authored directly in Helpin

## Agent skill references

Change `agent.Skills` from loose `[]string` semantics to a typed skill-reference contract.

Recommended shape:

```go
type AgentSkillRef struct {
    SkillID     *string         `json:"skill_id,omitempty"`
    Key         *string         `json:"key,omitempty"`
    VersionKey  *string         `json:"version_key,omitempty"`
    Config      json.RawMessage `json:"config,omitempty"`
}
```

Guidance:

- built-in skills can be referenced by `key`
- workspace/imported skills should be referenced by durable `skill_id`
- keep compatibility decode for legacy `[]string` values and normalize them into built-in refs

## Workspace skill record

Add a persisted workspace skill model for imported and Helpin-authored skills.

Recommended fields:

```go
type WorkspaceSkill struct {
    ID                string
    WorkspaceID       string
    SourceKind        string // imported | workspace
    SourceRuntime     *string // codex | opencode | claude_code | generic
    Key               string
    VersionKey        string
    Title             string
    Description       *string
    Instructions      string
    RequiredTools     json.RawMessage
    SupportedRuntimes json.RawMessage
    InstallSource     JSONBlob
    IsArchived        bool
    CreatedBy         *string
    CreatedAt         time.Time
    UpdatedAt         time.Time
}
```

Built-in skills are packaged on disk and embedded into the server binary. They do not need DB rows, but their package contents remain the canonical authored source.

## Preset definition

Extend `AgentPresetDefinition` with:

- `instruction_preamble string`
- `instruction_skills []AgentSkillRef` or `[]string` for built-in keys in phase 1
- `instruction_template_version string`

Preset `SystemPrompt` remains populated in API responses as a derived compiled value for backward compatibility.

## Prompt ownership on agents

For agent rows:

- managed prompt:
  - `SystemPrompt = nil`
  - `InstructionTemplateVersion = <compiled version>`
- custom prompt:
  - `SystemPrompt != nil`
  - `InstructionTemplateVersion = ""`

This keeps the overwrite rule simple and deterministic.

## Built-In Skill Loader And Registry

Built-in system skills should live on disk as packaged Agent Skills directories and be loaded through one parser. The in-memory registry is derived from those packaged files, not handwritten Go prompt constants.

Recommended layout:

- `server/skills/system/<skill-key>/SKILL.md`
- `server/skills/system/<skill-key>/agents/openai.yaml`

Recommended implementation files:

- `server/internal/worker/skill_loader.go`
- `server/internal/worker/skill_catalog.go`

Recommended type:

```go
type SkillDefinition struct {
    Key               string
    Title             string
    Description       string
    SourceKind        string // built_in
    PackagePath       string
    Instructions      string
    RequiredTools     []string
    SupportedRuntimes []string
    Policy            SkillPolicy
    Interface         SkillInterface
}
```

Add helpers:

- `GetBuiltInSkill(key string) (SkillDefinition, bool)`
- `ListBuiltInSkills() []SkillDefinition`
- `CompileSkills(preamble string, skills []ResolvedSkillDefinition) string`
- `SkillTemplateVersion(preamble string, skills []ResolvedSkillDefinition) string`
- `LoadBuiltInSkills(fs embed.FS) ([]SkillDefinition, error)`

Notes:

- `SKILL.md` frontmatter and body are canonical
- `agents/openai.yaml` is optional Codex/OpenCode metadata and should be parsed when present
- Helpin policy enforcement remains separate from package metadata such as tool dependencies

## Initial built-in skills

### Shared planner skills

- `approval_protocol`
- `task_decomposition`
- `general_agent_behavior`

### Planner-specific skills

- `prd_authorship`
- `epic_state_routing`
- `task_planner_context`

### Single-skill built-ins for smaller presets

- `code_builder`
- `review_agent`
- `crm_operator`
- `support_agent`

## Preset composition

### Epic Planner

Preamble:

- `You are Epic Planner. You run the full PRD-to-tasks loop inside a single interactive agent run.`

Skills:

- `approval_protocol`
- `prd_authorship`
- `task_decomposition`
- `epic_state_routing`
- `general_agent_behavior`

### Task Planner

Preamble:

- `You are Task Planner. Run a single interactive planning conversation for one task.`

Skills:

- `task_planner_context`
- `approval_protocol`
- `task_decomposition`
- `general_agent_behavior`

### CRM Operator

Preamble:

- `You are CRM Operator.`

Skills:

- `crm_operator`

### Support Agent

Preamble:

- `You are Support Agent.`

Skills:

- `support_agent`

### Code Builder

Preamble:

- `You are Code Builder.`

Skills:

- `code_builder`

### Review Agent

Preamble:

- `You are Review Agent.`

Skills:

- `review_agent`

## API Surfaces

## Skill catalog

Add:

- `GET /api/automation/library/skills`

This should return a merged catalog of:

- built-in skills
- active workspace/imported skills for the current workspace

Recommended response type:

```go
type SkillCatalogEntry struct {
    ID                *string     `json:"id,omitempty"`
    Key               string      `json:"key"`
    VersionKey        string      `json:"version_key"`
    SourceKind        string      `json:"source_kind"`
    SourceRuntime     *string     `json:"source_runtime,omitempty"`
    Title             string      `json:"title"`
    Description       string      `json:"description"`
    RequiredTools     []string    `json:"required_tools,omitempty"`
    SupportedRuntimes []string    `json:"supported_runtimes,omitempty"`
    IsBuiltIn         bool        `json:"is_built_in"`
    IsPresetOwned     bool        `json:"is_preset_owned,omitempty"`
}
```

## Workspace skill import and management

Add minimal workspace skill APIs:

- `POST /api/automation/library/skills/import`
- `POST /api/automation/library/skills`
- `PUT /api/automation/library/skills/{id}`
- `DELETE /api/automation/library/skills/{id}`

Recommended semantics:

- `import` accepts normalized external skill content or importer-specific payloads
- `create` creates a Helpin-authored workspace skill
- `update` edits a workspace/imported skill metadata or instructions
- `delete` archives or removes a workspace skill from the workspace library

## Agent APIs

Keep `create/update agent` endpoints but change the `skills` payload contract to accept typed skill refs.

Compatibility rule:

- legacy `string[]` skill payloads are still accepted for built-in keys during migration

## Import contract

The import path should support two levels:

### Level 1: normalized import

A client can send already-normalized fields:

- title
- description
- instructions
- source runtime
- original source metadata

### Level 2: runtime-shaped import

Later, or where already easy, accept importer-specific payloads for formats downloaded from:

- Codex
- OpenCode
- Claude Code
- generic skill markdown/json packages

The backend normalizes all of them into the same workspace skill record.

## Workstreams

## Workstream 1: Prompt ownership and versioning

### Objective

Replace heuristic prompt refresh with explicit managed-vs-custom prompt handling.

### Tasks

1. Add `InstructionTemplateVersion` to `Agent`, `WorkspaceAgentPresetVersion`, and `AgentPresetDefinition`.
2. Treat `SystemPrompt=nil` as managed prompt mode.
3. Treat `SystemPrompt!=nil` as custom override mode.
4. Replace the five active `*NeedsRefresh` helpers with:
   - one explicit version-based managed prompt path
   - one transitional `legacyPromptIsManaged` helper for legacy rows
5. Clear template version when a user sets a custom prompt.

### Primary files

- `server/internal/model/agent.go`
- `server/internal/service/agent.go`
- `server/internal/service/agent_policy.go`
- `server/internal/service/agent_system_prompts.go`

### Exit criteria

- managed prompts upgrade by version only
- custom prompts are never auto-overwritten
- legacy prompt migration behavior is isolated and test-covered

## Workstream 2: Packaged built-in skills and preset composition

### Objective

Replace monolithic preset prompts with packaged built-in skill bundles.

### Tasks

1. Add packaged built-in system skills under `server/skills/system/`.
2. Add a parser/loader for `SKILL.md` and optional `agents/openai.yaml`.
3. Build the in-memory built-in skill catalog from packaged system skills.
4. Extract planner prompt sections into reusable built-in skills.
5. Rewrite preset definitions to declare:
   - preamble
   - ordered built-in skill keys
   - derived system prompt
   - derived template version
6. Remove hardcoded planner prompt authoring from `defaultSystemPromptForPreset`.

### Primary files

- `server/skills/system/...`
- `server/internal/worker/skill_loader.go`
- `server/internal/worker/skill_catalog.go`
- `server/internal/service/agent_presets.go`
- `server/internal/service/agent_system_prompts.go`

### Exit criteria

- Epic Planner and Task Planner are composed from skills
- smaller presets map to one or more built-in skills
- preset `SystemPrompt` is derived, not primary authored text

## Workstream 3: Workspace skill library and import flow

### Objective

Let users bring downloaded skills into Helpin and use them on custom agents.

### Tasks

1. Add a persisted workspace skill model and repository.
2. Add catalog merge logic so built-in and workspace skills show up in one list.
3. Add import endpoint for downloaded Agent Skills packages.
4. Add create/update/delete endpoints for workspace skills.
5. Preserve source metadata so imported skills remain traceable.
6. Store imported package contents in a way that preserves the authored skill package shape.
7. Validate imported skills into Helpin's normalized contract using the same parser as built-in skills.

### Primary files

- `server/internal/model/agent.go` or new skill model file
- new repository for workspace skills
- new service methods for skill catalog/import/manage
- `server/internal/handler/automation.go`
- `server/internal/router/router.go`

### Exit criteria

- a workspace can store imported skills
- imported skills appear in the skill catalog
- imported skills can be referenced by agents

## Workstream 4: Typed agent skill refs and runtime prompt resolution

### Objective

Make `agent.Skills` a real execution contract.

### Tasks

1. Add typed decode/normalize logic for `AgentSkillRef`.
2. Accept legacy `string[]` built-in skills during migration.
3. Resolve an agent's effective skill set from:
   - preset defaults for managed system agents
   - explicit agent skill refs for custom agents
4. Compile the resolved skill set into the effective managed prompt before execution.
5. Remove the old raw skills bullet-list rendering from `prompt.go`.

### Primary files

- `server/internal/model/agent.go`
- `server/internal/service/agent.go`
- `server/internal/worker/prompt.go`
- execution-context preparation paths that build the runtime prompt

### Exit criteria

- `agent.Skills` affects execution behavior
- legacy string skill lists still migrate safely
- the runtime prompt is compiled from resolved skills instead of a raw skill-name list

## Workstream 5: Frontend skill library and agent skill picker

### Objective

Expose the skill system so users can actually import skills and attach them to agents.

### Tasks

1. Add skill catalog types and services in the frontend.
2. Add a Skills library surface under automation/library.
3. Add import flow for downloaded skills.
4. Add skill picker support to custom-agent editing.
5. Show preset-owned skills as read-only on system agents.

### Primary files

- `frontend/src/lib/pm-types/agents.ts`
- `frontend/src/lib/services/automationService.ts`
- `frontend/src/lib/services/agentService.ts`
- automation library pages/components
- `frontend/src/pages/automation/Agents.tsx`

### Exit criteria

- users can view built-in and workspace skills
- users can import downloaded skills into the workspace library
- users can attach skills to custom agents from the skill picker

## Workstream 6: Future runtime-native loading

### Objective

Run Codex and OpenCode with only the skills allowed for that agent visible to the runtime.

### Design rules

- canonical normalized skill shape in Helpin
- packaged Agent Skills source preserved for built-in and imported skills
- runtime compatibility metadata per skill
- runtime-native staging must be per-run or per-agent isolated, not global
- only the resolved allowed skills for the agent should be staged or installed
- disallowed or unrelated skills must not be exposed to the runtime
- skill dependencies declared in package metadata are advisory until Helpin explicitly supports them

### Tasks

1. Define a runtime staging contract for Codex and OpenCode.
2. Materialize the resolved allowed skills into a temporary skill root before runtime launch.
3. Point the runtime at that isolated skill root so discovery only sees those skills.
4. Keep prompt compilation as the fallback path when runtime-native loading is disabled or unsupported.
5. Add cleanup rules for staged skill directories after execution where appropriate.

### Later phases may add

- Claude Code exporter paths
- per-runtime package generation or install flows
- adapter-native loading where the runtime supports richer metadata handling

## Execution contract

For phase 1 runtime execution:

1. resolve the agent's effective skill refs
2. resolve skill definitions from:
   - built-in registry
   - workspace skill repository
3. compile one effective managed prompt
4. pass that prompt through the existing Helpin runtime path
5. let `BuildSystemPrompt` continue to add capability-derived rules, target context, and workflow context on top

This keeps Codex, OpenCode, and `native_sdk` aligned without introducing runtime-specific skill loaders yet.

For later Codex/OpenCode runtime-native execution:

1. resolve the agent's effective skill refs
2. filter to the skills allowed for that agent after preset and policy resolution
3. materialize those packaged skills into an isolated runtime skill directory
4. launch Codex/OpenCode with that directory configured as the visible skill root
5. still pass the Helpin-built effective prompt unless and until runtime-native skill behavior fully replaces prompt compilation

## Files Expected To Change

### Backend

- `server/internal/model/agent.go`
- new workspace skill model file if split out
- new workspace skill repository files
- `server/internal/service/agent.go`
- `server/internal/service/agent_policy.go`
- `server/internal/service/agent_presets.go`
- `server/internal/service/agent_system_prompts.go`
- `server/internal/worker/skill_loader.go`
- `server/internal/worker/skill_catalog.go`
- `server/internal/worker/prompt.go`
- `server/internal/handler/automation.go`
- `server/internal/router/router.go`
- `server/skills/system/...`

### Frontend

- `frontend/src/lib/pm-types/agents.ts`
- `frontend/src/lib/services/automationService.ts`
- `frontend/src/lib/services/agentService.ts`
- automation library UI files
- `frontend/src/pages/automation/Agents.tsx`

## Tests

Add or update tests for:

- built-in skill registry contents
- built-in packaged skill parsing and catalog generation
- deterministic skill compilation and template versioning
- legacy prompt migration and custom prompt preservation
- legacy `string[]` skill normalization into typed refs
- workspace skill import and catalog merge behavior
- agent effective skill resolution
- planner compiled prompt content assertions
- runtime skill staging filters only agent-allowed skills
- frontend agent-skill payload typing if covered by existing test patterns

## Verification

1. `cd server && go build ./...`
2. `cd server && go test ./...`
3. `cd frontend && npm run build`
4. verify built-in skill catalog endpoint returns built-in skills
5. verify imported workspace skills appear in the merged skill catalog
6. verify a custom agent with skill refs resolves to the expected effective prompt
7. verify custom `system_prompt` overrides are preserved during preset changes
8. later, verify Codex/OpenCode staging exposes only the resolved allowed skills for the launched agent

## What This Plan Does Not Change Yet

- no automatic tool grants from skills
- no remote marketplace sync
- no Codex/OpenCode runtime-native skill staging in PR1
- no removal of custom prompt overrides
- no immediate deprecation of all legacy `string[]` skill payloads without a migration window
