# Release Notes Agent Template Plan

## Purpose

Create a reusable release-notes automation path that runs when a GitHub release is published, gathers deterministic release/task/docs context, and creates a release notes document.

This plan intentionally separates four concerns:

- DB-backed custom-agent templates scaffold user-owned custom agents.
- Skills provide reusable runtime instructions.
- Backend fact tools gather scoped, deterministic context for any agent.
- Automation rules decide when a custom agent runs.

## Product Outcome

Users can create a Release Notes Writer custom agent from a DB-backed template, attach it to a GitHub release flow, choose which release kinds should trigger it, and choose where release notes documents should be created.

Example flow:

```text
When GitHub publishes a minor release for acme/api,
run Release Notes Writer on repository acme/api,
then create a release notes document in Docs / Release Notes.
```

## Current-State Constraints

Custom agents cannot currently be created from system presets:

- `CreateAgent` rejects `preset_key` and `preset_version_key` for custom agents.
- Non-system agents have preset fields cleared during normalization.
- The UI states that custom agents do not inherit or track preset families.
- Existing presets such as Epic Planner and Task Planner are managed system-agent families.

Therefore, this feature should not use `preset_key` for custom agents.

## Target Model

Use DB-backed custom-agent templates, not system presets.

```text
agent_template
  -> creates a normal custom agent
  -> copies runtime/tools/targets/skills/prompt defaults once
  -> optionally creates a starter automation flow

skill
  -> reusable runtime instructions
  -> attached to the custom agent

backend fact tools
  -> reusable read-only context for releases, git changes, tasks, and docs

automation_rule
  -> github.release_published + release filters
  -> start_agent_run on repository target
```

After creation, the agent is custom and user-owned. Template changes do not automatically mutate existing agents.

## Data Model

### `agent_templates`

Add a DB-backed template table.

```sql
CREATE TABLE IF NOT EXISTS agent_templates (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NULL,
  key text NOT NULL,
  label text NOT NULL,
  description text,
  category text,
  runtime_kind text NOT NULL,
  default_agent_role text,
  default_invocation_mode text NOT NULL DEFAULT 'autonomous',
  approval_mode text NOT NULL DEFAULT 'never',
  trigger_mode text NOT NULL DEFAULT 'manual',
  provider text,
  model text,
  execution_config jsonb NOT NULL DEFAULT '{}',
  system_prompt text,
  skills jsonb NOT NULL DEFAULT '[]',
  allowed_tools jsonb NOT NULL DEFAULT '[]',
  allowed_commands jsonb NOT NULL DEFAULT '[]',
  allowed_targets jsonb NOT NULL DEFAULT '[]',
  default_flow jsonb NOT NULL DEFAULT '{}',
  active boolean NOT NULL DEFAULT true,
  created_by uuid NULL,
  updated_by uuid NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz NULL
);
```

Indexes:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_templates_system_key
  ON agent_templates (key)
  WHERE workspace_id IS NULL AND deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_templates_workspace_key
  ON agent_templates (workspace_id, key)
  WHERE workspace_id IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_agent_templates_workspace_active
  ON agent_templates (workspace_id, active)
  WHERE deleted_at IS NULL;
```

Rules:

- `workspace_id IS NULL` means product-owned system template.
- `workspace_id IS NOT NULL` means workspace-owned template.
- Do not keep a separate `scope` column as a second source of truth.
- Templates scaffold custom agents; they do not create inheritance.

### Agent Provenance

Add informational provenance fields to `agents`.

```sql
ALTER TABLE agents
  ADD COLUMN IF NOT EXISTS source_template_id uuid NULL,
  ADD COLUMN IF NOT EXISTS source_template_key text NOT NULL DEFAULT '';
```

Rules:

- These fields are informational.
- They must not affect runtime policy.
- They must not imply that the agent tracks future template changes.
- Do not reuse `preset_key` or `preset_version_key`.

## Release Notes Writer Template

Seed a system template.

```json
{
  "key": "release_notes_writer",
  "label": "Release Notes Writer",
  "description": "Creates release notes from GitHub releases, related tasks, and linked docs.",
  "category": "GitHub",
  "runtime_kind": "native_sdk",
  "default_agent_role": "Release Notes Writer",
  "default_invocation_mode": "autonomous",
  "approval_mode": "never",
  "trigger_mode": "manual",
  "skills": [
    {
      "type": "system",
      "name": "release_notes_writer"
    }
  ],
  "allowed_targets": ["repository"],
  "allowed_tools": [
    "get_release_context",
    "get_task_context",
    "find_tasks_for_git_changes",
    "read_document",
    "search_documents",
    "list_collections",
    "create_document",
    "write_document_content",
    "link_document_to_object"
  ],
  "allowed_commands": [],
  "default_flow": {
    "trigger_type": "github.release_published",
    "release_kinds": ["minor"],
    "target_type": "repository"
  }
}
```

Notes:

- `default_agent_role` is copied once into `Agent.Role` during template materialization. Behavior should come from the attached skill, system prompt, and tools, not from role-specific preset machinery.
- Use `release_notes_writer` for the DB template key and `release_notes_writer` for the skill name unless the skill registry requires another convention. Avoid mixing snake_case and kebab-case for the same concept.
- `default_invocation_mode = "autonomous"` uses the existing `model.InvocationModeAutonomous` value.

## Template Service

Add `AgentTemplateRepository`.

Methods:

- `ListSystemTemplates(ctx)`
- `ListWorkspaceTemplates(ctx, workspaceID)`
- `GetByID(ctx, workspaceID, id)`
- `GetByKey(ctx, workspaceID, key)`
- `CreateWorkspaceTemplate(ctx, template)`
- `UpdateWorkspaceTemplate(ctx, template)`
- `SoftDelete(ctx, workspaceID, id)`

V1 endpoint policy:

- Ship system-template listing and create-agent-from-template first.
- Gate workspace template create/update/delete behind a feature flag or leave the handlers unregistered in V1.
- Do not expose workspace template write paths until permissions, UI, and tests are in place.

Add `AgentTemplateService`.

Responsibilities:

- List system and workspace templates.
- Validate template keys, runtime, trigger mode, invocation mode, tools, targets, commands, and skills.
- Materialize templates into `model.CreateAgentRequest`.
- Create custom agents from templates.
- Set `source_template_id` and `source_template_key` on created agents.
- Optionally create a starter automation rule.
- Ensure template-created agents still pass ordinary custom-agent validation.

## Template APIs

Add routes under the automation namespace.

```text
GET    /api/automation/agent-templates
GET    /api/automation/agent-templates/{id}
POST   /api/automation/agent-templates/{id}/create-agent
```

Deferred or feature-flagged workspace-template routes:

```text
POST   /api/automation/agent-templates
PUT    /api/automation/agent-templates/{id}
DELETE /api/automation/agent-templates/{id}
```

Create-agent request:

```json
{
  "name": "Release Notes Writer",
  "team_id": null,
  "overrides": {
    "model": "optional",
    "monthly_token_budget": 1000000
  },
  "create_flow": true,
  "flow": {
    "repository_id": "repo-id",
    "repo_full_name": "acme/api",
    "release_kinds": ["minor"],
    "include_prerelease": false,
    "tag_pattern": "v*",
    "space_id": "docs-space-id",
    "collection_id": "release-notes-collection-id"
  }
}
```

Materialization rules:

- Copy template defaults into a custom agent.
- Allow safe request overrides for name, model/provider, monthly token budget, team, and destination flow config.
- `monthly_token_budget` maps to the existing `model.CreateAgentRequest.MonthlyTokenBudget` field.
- Do not set `preset_key`.
- Do not pin the agent to the template.
- Validate all tools and skills before create.

## GitHub Release Event Context

Extend agent run input with typed GitHub event context.

Current `AgentRunEventContext` is too narrow. Add a typed nested struct rather than a loose `map[string]any`.

Use a discriminated GitHub context so push, pull request, release, and check-suite contexts can be added without renaming the field later.

```go
type AgentRunGitHubEventContext struct {
	EventType   string                              `json:"event_type,omitempty"`
	RepoFullName string                             `json:"repo_full_name,omitempty"`
	RepositoryID string                             `json:"repository_id,omitempty"`
	Release     *AgentRunGitHubReleaseEventContext `json:"release,omitempty"`
	PullRequest *AgentRunGitHubPullRequestEventContext `json:"pull_request,omitempty"`
	CheckSuite  *AgentRunGitHubCheckSuiteEventContext  `json:"check_suite,omitempty"`
}

type AgentRunGitHubReleaseEventContext struct {
	TagName         string     `json:"tag_name,omitempty"`
	TargetCommitish string     `json:"target_commitish,omitempty"`
	ReleaseName     string     `json:"release_name,omitempty"`
	ReleaseURL      string     `json:"release_url,omitempty"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
	IsPrerelease bool       `json:"is_prerelease,omitempty"`
}

type AgentRunEventContext struct {
	StateID *string                     `json:"state_id,omitempty"`
	TeamID  *string                     `json:"team_id,omitempty"`
	RunID   *string                     `json:"run_id,omitempty"`
	Reason  *string                     `json:"reason,omitempty"`
	GitHub  *AgentRunGitHubEventContext `json:"github,omitempty"`
}
```

Example run input:

```json
{
  "trigger": {
    "source": "automation_rule",
    "trigger_type": "github.release_published",
    "rule_id": "rule-id",
    "fired_at": "2026-04-24T10:00:00Z"
  },
  "target": {
    "target_type": "repository",
    "target_id": "repo-uuid"
  },
  "event": {
    "github": {
      "event_type": "release_published",
      "repo_full_name": "acme/api",
      "repository_id": "repo-uuid",
      "release": {
        "tag_name": "v1.4.0",
        "target_commitish": "main",
        "release_name": "v1.4.0",
        "release_url": "https://github.com/acme/api/releases/tag/v1.4.0",
        "published_at": "2026-04-24T10:00:00Z",
        "is_prerelease": false
      }
    }
  }
}
```

Implementation notes:

- Extend `model.AutomationEvent` with explicit release metadata fields:
  - `RepositoryID string`
  - `TargetCommitish string`
  - `ReleaseName string`
  - `ReleaseURL string`
  - `PublishedAt *time.Time`
  - `IsPrerelease bool`
- Extend GitHub webhook parsing so release events extract `target_commitish`, `html_url`, `name`, `published_at`, `prerelease`, and repository ID/full name.
- Populate those fields in `GitService.ProcessWebhookRelease`.
- Pass those fields into the `AutomationRuleEngine.EvaluateEvent` call site and then through `AutomationRuleEngine.executeStartAgentRun`.
- Persist the typed GitHub context in the agent run input.
- Handle empty `target_commitish`; fall back to the repository default branch only when the GitHub API or stored repository metadata can resolve it safely.
- Do not depend on `additional_context` for event facts.

Existing fields retained on `AutomationEvent`:

- `RepoFullName`
- `TagName`

## Release Trigger Filters

Extend `TriggerConfigGitHubReleasePublished`.

```go
type TriggerConfigGitHubReleasePublished struct {
	RepoFullName      string   `json:"repo_full_name,omitempty"`
	TagName           string   `json:"tag_name,omitempty"`
	TagPattern        string   `json:"tag_pattern,omitempty"`
	ReleaseKinds      []string `json:"release_kinds,omitempty"`
	IncludePrerelease bool     `json:"include_prerelease,omitempty"`
}
```

Supported release kinds:

- `major`
- `minor`
- `patch`
- `prerelease`
- `unknown`

Matching rules:

- `repo_full_name` exact match if set.
- `tag_name` exact match if set.
- `tag_pattern` glob match if set.
- `release_kinds == nil` or `release_kinds == []` means match all release kinds for backward compatibility with existing rules.
- Implementation must check `len(cfg.ReleaseKinds) == 0`, not only `cfg.ReleaseKinds == nil`, because `encoding/json` decodes `"release_kinds": []` as a non-nil empty slice.
- `release_kinds` with values means match only after semver classification.
- prereleases are ignored when `include_prerelease` is false unless `release_kinds` contains `prerelease`.
- Existing persisted rules with empty trigger config must keep their current behavior unless migrated.

Validation rules:

- For new rules, require at least one filter: repo, tag, tag pattern, or release kind.
- Do not retroactively reject or disable existing rules with empty configs.
- If tightening validation for existing rules is required later, first backfill `repo_full_name` where possible or mark the rule for user review.

Preferred behavior:

- Filter before launching the agent.
- Do not launch patch-release runs when the flow selected only minor releases.
- Cache release classification once per GitHub release event evaluation so multiple matching rules do not each perform GitHub release-list lookups.
- Store the per-event classification cache on the evaluation context or a short-lived local object passed through matching, not in persistent rule state.
- If classification fails due to a transient GitHub/API error and `release_kinds` is configured, record/log the rule evaluation as skipped or failed; do not silently drop the event.

Fallback behavior for initial rollout:

- If release classification cannot be completed during trigger evaluation and no `release_kinds` filter is configured, launch only when other filters match and let `get_release_context` return `release_kind = "unknown"` plus warnings.
- If `release_kinds` is set and classification returns `unknown`, do not match unless `release_kinds` explicitly contains `unknown`.

## GitHub Release Comparison Service

Create a backend service around the GitHub App client. Do not put GitHub API orchestration directly in the agent tool handler.

Responsibilities:

- Resolve workspace-scoped repo and integration.
- Resolve by `repository_id` first when available.
- Resolve `repo_full_name` only through workspace-scoped repository lookup.
- Never resolve a full repo name by global GitHub-side lookup without a workspace repository record.
- Verify the repository belongs to the workspace.
- Verify the integration is active.
- Fetch the current release by tag.
- Find the previous published release for the same repository.
- Compare `previous_tag...current_tag`.
- Normalize commits, PR numbers, authors, changed files, URLs, and truncation.
- Classify release kind as `major`, `minor`, `patch`, `prerelease`, or `unknown`.

Add GitHub client methods:

- `ListReleases(ctx, installationID, owner, repo, opts)`
- `GetReleaseByTag(ctx, installationID, owner, repo, tagName)`
- `CompareRefs(ctx, installationID, owner, repo, base, head)`

Suggested `ListReleases` options:

```go
type ListReleasesOptions struct {
	IncludeDrafts       bool
	IncludePrereleases  bool
	PerPage             int
	MaxPages            int
}
```

Defaults and caps:

- `PerPage` defaults to 100 and is capped at 100.
- `MaxPages` defaults to 5 and is hard-capped at 20.
- The comparison service enforces these caps before calling GitHub.

Normalized service output should not leak raw GitHub webhook/API payloads to the agent.

## Semver Classification

Add a small semver helper for release tags.

Rules:

- Accept `1.2.3` and `v1.2.3`.
- Accept tag prefixes with path segments such as `release/v1.4.0` by parsing the final semver-looking segment.
- Treat suffixes such as `v1.2.3-beta.1` as prerelease.
- If no previous release exists, return `release_kind = "unknown"` and warning `previous_release_not_found`.
- A first release must not match a flow configured with `release_kinds = ["minor"]` unless the user also selected `unknown`.
- If current or previous tag is not semver, return `unknown`.
- If major number increased, return `major`.
- If major is the same and minor increased, return `minor`.
- If major/minor are the same and patch increased, return `patch`.
- If current has prerelease suffix, return `prerelease`.

Consider release ordering:

- Prefer GitHub `published_at` for previous release selection.
- Exclude drafts.
- Exclude prereleases by default.
- Include prereleases only when the trigger or tool requested them.
- Make prerelease inclusion an explicit parameter on the comparison service.

## Deterministic Task Matching

Create a reusable resolver that maps Git changes to Helpin tasks.

Evidence order:

- PR number match via `task_git_links.repo + pr_number`.
- Branch match via `task_git_links.repo + branch`.
- Commit SHA match if stored reliably.
- Task key parse from PR titles and commit messages, e.g. `HLP-123`.
- Optional fuzzy text search only as low confidence.

Return evidence and confidence, not just task IDs.

```json
{
  "task_id": "task-1",
  "task_key": "HLP-123",
  "confidence": "high",
  "matched_by": ["pull_request_number", "task_key"],
  "matched_refs": {
    "pr_number": 42,
    "task_key": "HLP-123"
  }
}
```

Confidence levels:

- `high`: PR number, branch, or exact task key.
- `medium`: commit SHA or multiple weak references.
- `low`: fuzzy title/message match.

Rules:

- Always scope matches by workspace.
- Always scope git evidence by repo when available.
- Deduplicate task matches and preserve all evidence.
- Do not let fuzzy matches silently appear as high confidence.

## Reusable Fact Tools

All tools should use top-level JSON object inputs, `snake_case` fields, explicit schemas, typed decode structs, validation, and compact JSON outputs.

Implementation rule:

- Tool handlers should be thin shells over reusable services.
- GitHub release comparison, task matching, and task context assembly must be testable without invoking the worker/tool runtime.
- Service bridge functions expose those services to the worker, but the service remains the source of truth.

Existing reusable docs tools:

- `read_document`
- `search_documents`
- `list_collections`
- `create_document`
- `write_document_content`
- `link_document_to_object`

New tools in this plan:

- `get_release_context`
- `find_tasks_for_git_changes`
- `get_task_context`

### `get_release_context`

Purpose:

Return normalized release comparison facts for a repository release. This should be useful to release notes, QA review, docs-drift detection, sprint reporting, and deployment agents.

Input:

```json
{
  "repository_id": "optional repo UUID; defaults from run target",
  "repo_full_name": "optional repo full name; defaults from event",
  "tag_name": "optional release tag; defaults from event",
  "include_changed_files": true,
  "include_prerelease": false,
  "max_commits": 100,
  "max_files": 200
}
```

Validation:

- If `repository_id` is provided explicitly, it wins over the run target repository ID.
- Resolve `repository_id` from repository target when omitted.
- Resolve `repo_full_name` and `tag_name` from run event when omitted.
- If explicit `repository_id` and `repo_full_name` conflict, reject the request rather than silently switching repositories.
- Require enough information to identify a workspace-scoped repository and release.
- Cap `max_commits` and `max_files`.

Output:

```json
{
  "repository": {
    "id": "repo-id",
    "full_name": "acme/api",
    "default_branch": "main"
  },
  "current_release": {
    "tag_name": "v1.4.0",
    "name": "v1.4.0",
    "url": "https://github.com/acme/api/releases/tag/v1.4.0",
    "published_at": "2026-04-24T10:00:00Z",
    "target_commitish": "main"
  },
  "previous_release": {
    "tag_name": "v1.3.2",
    "published_at": "2026-04-01T10:00:00Z"
  },
  "release_kind": "minor",
  "compare_url": "https://github.com/acme/api/compare/v1.3.2...v1.4.0",
  "commits": [],
  "pull_requests": [],
  "changed_files": [],
  "related_tasks": [],
  "warnings": []
}
```

Warnings:

- `previous_release_not_found`
- `commit_limit_reached`
- `file_limit_reached`
- `release_kind_unknown`
- `task_matching_partial`

### `find_tasks_for_git_changes`

Purpose:

Resolve Helpin tasks for arbitrary Git evidence outside the release flow.

Input:

```json
{
  "repo_full_name": "acme/api",
  "pr_numbers": [42, 43],
  "commit_shas": ["abc123"],
  "branches": ["helpin/hlp-123"],
  "texts": ["HLP-123 Improve billing webhook"]
}
```

Validation:

- Require `repo_full_name` or a repository target.
- Require at least one evidence array.
- Trim and dedupe evidence.
- Cap arrays:
  - `pr_numbers`: 50
  - `commit_shas`: 200
  - `branches`: 50
  - `texts`: 100

Output:

```json
{
  "matches": [
    {
      "task_id": "task-1",
      "task_key": "HLP-123",
      "confidence": "high",
      "matched_by": ["pull_request_number"],
      "matched_refs": {
        "pr_number": 42
      }
    }
  ],
  "unmatched": {
    "pr_numbers": [],
    "commit_shas": [],
    "branches": [],
    "texts": []
  }
}
```

### `get_task_context`

Purpose:

Return compact task context and linked docs metadata for selected task IDs.

Input:

```json
{
  "task_ids": ["task-1", "task-2"],
  "include_linked_docs": true,
  "include_document_content": false,
  "include_comments": false,
  "include_git_links": true
}
```

Validation:

- Require `task_ids`.
- Limit the number of task IDs.
- Scope all tasks by workspace.
- Do not include full document content by default.

Output should include:

- task title, key, type, priority, description text, status, team, owners
- epic/objective when available
- linked docs metadata
- git links
- compact comments/checklist only if requested

Default behavior:

- Return linked docs metadata only.
- Let the agent call `read_document` for selected docs.

## Docs Output Configuration

The agent needs destination config for created documents.

V1:

- Add typed output configuration to `ActionConfigRunAgent`.
- Store `space_id` and `collection_id` in structured action config.
- Do not parse `space_id=...` or `collection_id=...` out of freeform `additional_context`.

Proposed action config extension:

```go
type ActionConfigRunAgentOutput struct {
	Type           string `json:"type,omitempty"`
	SpaceID        string `json:"space_id,omitempty"`
	CollectionID   string `json:"collection_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type ActionConfigRunAgent struct {
	TargetType            string                      `json:"target_type,omitempty"`
	TargetID              string                      `json:"target_id,omitempty"`
	AgentID               string                      `json:"agent_id"`
	LegacyScheduleAgentID string                      `json:"legacy_schedule_agent_id,omitempty"`
	AdditionalContext     *string                     `json:"additional_context,omitempty"`
	BaseBranch            string                      `json:"base_branch,omitempty"`
	WorkingBranch         string                      `json:"working_branch,omitempty"`
	Output                *ActionConfigRunAgentOutput `json:"output,omitempty"`
}
```

Rules:

- `output.type = "docs_document"` for release notes.
- `output.space_id` is required when `output.type = "docs_document"`.
- `output.collection_id` is optional only if the docs product supports uncollected documents in the selected space.
- The rule engine derives `output.idempotency_key` once when starting the run if it is omitted and the trigger is `github.release_published`.
- The derived key uses repository and release tag: `release_notes:{repository_id}:{tag_name}`.
- `idempotency_key` is generic to output contexts and may be used by future output types; for unsupported output types it should be ignored, not rejected.
- Pass output config into the run input as typed metadata available to tools and instructions.
- Optionally add workspace/team release-notes settings later.

Suggested run input extension:

```go
type AgentRunOutputContext struct {
	Type           string `json:"type,omitempty"`
	SpaceID        string `json:"space_id,omitempty"`
	CollectionID   string `json:"collection_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type AgentRunInputPayload struct {
	Trigger *AgentRunTriggerContext `json:"trigger,omitempty"`
	Target  *AgentRunTargetContext  `json:"target,omitempty"`
	Event   *AgentRunEventContext   `json:"event,omitempty"`
	Output  *AgentRunOutputContext  `json:"output,omitempty"`
	// existing fields...
}
```

Relationship between the two output structs:

- `ActionConfigRunAgentOutput` is the user-authored rule configuration.
- `AgentRunOutputContext` is the normalized runtime context persisted into `agent_runs.input`.
- The rule engine maps config to runtime context and fills computed values such as `idempotency_key`.
- The structs may start with the same fields, but they are intentionally separate because runtime context can add resolved/computed values later without changing the rule authoring contract.

## Release Notes Idempotency

Release webhooks can be replayed, and users may re-run a flow manually. The release notes path must avoid duplicate documents by default.

Storage:

- Add a dedicated `docs_document_keys` table in V1 rather than relying on title matching.
- Key rows map `(workspace_id, key)` to `document_id`.
- `key` stores the normalized idempotency key from `AgentRunOutputContext.IdempotencyKey`.
- Use a unique index on `(workspace_id, key)`.

Suggested schema:

```sql
CREATE TABLE IF NOT EXISTS docs_document_keys (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  document_id uuid NOT NULL,
  key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT fk_docs_document_keys_document
    FOREIGN KEY (document_id)
    REFERENCES docs_documents(id)
    ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_document_keys_workspace_key
  ON docs_document_keys (workspace_id, key);
```

Rules:

- Use a deterministic idempotency key such as `release_notes:{repository_id}:{tag_name}`. `workspace_id` is intentionally not embedded in the key string because the unique index already scopes by workspace.
- The rule engine derives and persists the key into `AgentRunInputPayload.Output.IdempotencyKey`; the agent and docs tools consume it.
- If the release notes document is deleted, `ON DELETE CASCADE` removes the key row. The next webhook replay or manual rerun may create a fresh document.
- Before creating a document, look up an existing release-notes document for the same idempotency key.
- If an existing document is found for a webhook replay, no-op and return the existing document.
- If an existing document is found for a manual rerun, update the existing document by default.
- Manual reruns should copy `Output` from the parent run. As a defensive fallback, docs output tooling may derive the key from typed GitHub event context when `Output.IdempotencyKey` is empty.
- Do not create a second document silently for the same idempotency key.
- Trigger execution replay should not create duplicate agent runs unless explicitly requested by a human.

## Release Notes Skill

Add a predefined `release_notes_writer` skill.

Naming rule:

- Use the skill registry's existing naming convention.
- Prefer `release_notes_writer` to match the template key unless the registry requires kebab-case.
- If the registry requires kebab-case, document the divergence in both the template seed and skill metadata.

Runtime instructions:

- Call `get_release_context` first.
- Use `release_kind` and warnings to decide whether to proceed.
- Use related task matches to call `get_task_context`.
- Read linked docs only when they likely explain user-facing behavior.
- Prefer task/docs context over commit-message wording.
- Do not invent user-facing impact from code-only changes.
- Separate user-facing changes from internal-only changes.
- Call out breaking changes, migration notes, and known issues.
- Cite task keys and PR numbers in an appendix.
- Create one release notes document in the configured docs destination.
- Link the document back to related tasks when appropriate.

Recommended document sections:

- Title
- Summary
- Highlights
- Improvements
- Fixes
- Breaking Changes
- Migration Notes
- Known Issues
- Internal Changes
- Appendix: Tasks and Pull Requests

## Flow Creation

When creating an agent from the Release Notes Writer template with `create_flow = true`, create an automation rule:

```json
{
  "trigger_type": "github.release_published",
  "trigger_config": {
    "repo_full_name": "acme/api",
    "release_kinds": ["minor"],
    "include_prerelease": false,
    "tag_pattern": "v*"
  },
  "action_type": "start_agent_run",
  "action_config": {
    "agent_id": "agent-id",
    "target_type": "repository",
    "target_id": "repo-id",
    "output": {
      "type": "docs_document",
      "space_id": "docs-space-id",
      "collection_id": "release-notes-collection-id"
    }
  }
}
```

## Frontend

Add template browsing to Automation / Agents.

Surfaces:

- Template gallery.
- Create from template action.
- Release Notes Writer setup wizard.
- Agent provenance badge.

Release Notes Writer wizard fields:

- Agent name.
- GitHub repository.
- Release kinds: major, minor, patch, prerelease, unknown.
- Include prereleases.
- Optional tag pattern.
- Docs space.
- Docs collection.
- Model/provider overrides if advanced settings are open.
- Create flow checkbox.

Copy:

- "Created from Release Notes Writer template."
- "This is now a custom agent. It does not inherit future template changes."

## Backend Wiring Checklist

Template system:

- Add `agent_templates` model.
- Add migration in `server/internal/dbmigrate/sql/`.
- Add repository.
- Add service.
- Wire service in `cmd/api/main.go`.
- Add handler routes.
- Add frontend service types.

Release event context:

- Extend `model.AgentRunEventContext`.
- Extend `model.AutomationEvent`.
- Populate from GitHub release webhook handling.
- Pass through automation rule engine to `startTargetRun`.
- Ensure run input persists typed event context.

Release filters:

- Extend `TriggerConfigGitHubReleasePublished`.
- Update trigger validation.
- Update trigger matching.
- Update automation catalog descriptions and flow UI metadata.

Fact tools:

- Add tool schemas in `server/internal/worker/tools.go`.
- Add handlers in a new `tools_github_release.go` or similar.
- Add tool catalog categories.
- Add allowed tools for relevant runtime/template defaults.
- Expose service bridge functions.

Docs:

- Add or update docs for agent templates.
- Add tool contract docs if exact payloads become model-critical.

## Testing Plan

Template tests:

- System templates list correctly.
- Workspace template write endpoints are gated or unregistered in V1.
- Workspace templates override/list correctly when the feature flag is enabled.
- Template-created agent has no `preset_key`.
- Template-created agent has provenance fields.
- `source_template_key` survives template rename/delete.
- Template-created agent copies skills/tools/targets.
- Invalid tools/skills are rejected.
- Existing custom-agent creation still rejects preset fields.

Release trigger tests:

- GitHub release webhook emits release metadata.
- Release webhook with empty `target_commitish` is handled safely.
- Trigger matching supports `repo_full_name`.
- Trigger matching supports `tag_name`.
- Trigger matching supports `tag_pattern`.
- Tag pattern matching supports tags with slashes such as `release/v1.4.0`.
- Trigger matching supports `release_kinds`.
- Empty or missing `release_kinds` matches all kinds for existing rules.
- Patch release does not launch a minor-only rule.
- Prerelease does not launch when excluded.
- First release with no previous release returns `unknown` and does not match minor-only rules.
- Deleted or yanked releases do not trigger release notes runs.
- Backfilled or replayed release webhooks are idempotent or explicitly skipped according to trigger execution policy.

GitHub service tests:

- Previous release selection.
- Semver classification.
- Compare output normalization.
- Commit and file truncation warnings.
- Workspace repo scoping.
- Inactive integration rejection.

Task matching tests:

- PR number produces high confidence.
- Branch produces high confidence.
- Task key in PR title produces high confidence.
- Task key in commit message produces high confidence.
- Fuzzy fallback is low confidence.
- Cross-workspace links are not returned.

Tool tests:

- `get_release_context` defaults from repository target and event context.
- `get_release_context` validates missing tag/repo.
- `find_tasks_for_git_changes` validates empty evidence.
- `get_task_context` returns linked docs metadata.
- `get_task_context` does not include document content unless requested.
- `get_task_context` does not leak cross-workspace docs.

End-to-end tests:

- Creating from Release Notes Writer template creates a custom agent.
- Optional flow creation creates a `github.release_published` rule.
- A matching minor release starts a repository-targeted run.
- The run input includes typed GitHub event context.
- Re-running the same release does not create duplicate release notes documents.
- Replayed trigger executions do not create duplicate agent runs unless explicitly requested.

## Rollout Plan

Phase 1: Infrastructure

- Add DB-backed templates.
- Add read-only template APIs plus create-agent-from-template.
- Add template-created agent provenance.
- Keep workspace template write APIs feature-flagged or unregistered.
- Keep UI hidden or internal-only.

Phase 2: GitHub release context

- Add typed GitHub event context.
- Add release trigger filters.
- Add semver classification.
- Add GitHub release comparison service.

Phase 3: Reusable fact tools

- Add `get_release_context`.
- Add deterministic task matching.
- Add `find_tasks_for_git_changes`.
- Add `get_task_context`.

Phase 4: Release Notes Writer

- Add predefined skill.
- Seed system template.
- Add frontend template wizard.
- Create flow from template.

Phase 5: Hardening

- Add trigger execution diagnostics for skipped release-kind mismatches.
- Add preview/review mode if customers want human approval before document creation.
- Expose workspace-created templates after system template path is stable.

## Open Decisions

- Should release notes documents be created directly, or should the agent publish a preview first and request approval?
- Should workspace users be able to edit DB-backed templates in v1, or should v1 only list system templates?
- Should `get_task_context` include comments/checklists in v1, or defer those to keep context small?

## Preferred V1 Cut

Build the smallest end-to-end version that still has the right architecture:

- DB-backed system template only.
- Custom agent creation from template.
- Release-kind filtering in flow.
- Typed GitHub event context.
- Structured docs output config in `ActionConfigRunAgent`.
- `get_release_context`.
- `get_task_context` with linked docs metadata.
- Release notes skill.
- Create document through existing docs tools.

Defer:

- Workspace-authored templates.
- Fuzzy task matching.
- Human approval preview mode.
