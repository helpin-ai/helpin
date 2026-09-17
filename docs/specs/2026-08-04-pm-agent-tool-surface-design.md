# PM Agent Tool Surface Design

## Summary

Helpin agents need a coherent, safe PM tool surface that lets them discover project context, inspect core PM entities, and create or update operational work. The current catalog covers a narrow task- and epic-planning path. It does not expose sprints, objectives, members, or labels for discovery; task creation cannot place work in a sprint; sprint and objective runs are not valid agent targets; and some selectable checklist tools do not have matching internal-command executors.

This design adds explicit, typed tools backed by the existing `InternalCommandService` and PM services. It deliberately excludes deletion, archiving, workflow administration, automation administration, broad reordering, and file attachment upload.

## Goals

- Let an agent turn a sprint description into correctly scoped tasks assigned to that sprint.
- Let agents discover the IDs and current state needed for valid PM mutations.
- Support safe operational work across tasks, epics, sprints, objectives, comments, checklists, labels, dependencies, and key results.
- Allow agents to run directly against sprints and objectives with useful target context.
- Preserve workspace, role, and team-access boundaries for every read and write.
- Prevent catalog entries from drifting away from executable runtime commands.

## Non-goals

- Deleting or archiving PM entities.
- Creating, updating, deleting, or reordering workflows and workflow states.
- Creating or changing PM automation rules.
- Managing saved views, recurring templates, or task templates.
- Uploading attachments or manipulating binary content.
- Following entities, managing reactions, or managing external links.
- Replacing the public MCP catalog or exposing every PM HTTP endpoint.

## Chosen approach

Add explicit resource-oriented tools through the internal-command boundary. Each tool has a narrow JSON schema, a read/write classification, supported target types, a compact result contract, and an implementation that calls an existing service.

This is preferable to a generic `query_pm` / `mutate_pm` pair because explicit tools give the model better schemas and make policy, approvals, auditing, and testing easier. It is preferable to proxying REST endpoints because the internal-command boundary already carries the run actor, workspace, target, and centralized authorization checks.

## Tool surface

### Discovery

| Tool | Mode | Purpose |
| --- | --- | --- |
| `list_workspace_teams` | Read | Existing tool; discover team IDs and defaults. |
| `list_workspace_members` | Read | Return active assignable members with member ID, user ID, display name, and team IDs. |
| `list_team_workflows_with_stages` | Read | Existing tool; discover valid workflow and state IDs. |
| `list_pm_labels` | Read | Return non-archived workspace and team labels, optionally filtered by team or name. |
| `ensure_task_label` | Write | Existing create-or-return tool; retained for idempotent label use. |

Member results must not expose email addresses or other unnecessary profile data. List results are capped at 100 records.

### Tasks

| Tool | Mode | Purpose |
| --- | --- | --- |
| `list_tasks` | Read | Existing tool, expanded with `epic_id`, `sprint_id`, `workflow_id`, `state_id`, `task_type`, `priority`, `severity`, `completed`, `archived`, `updated_after`, page, and per-page filters. |
| `get_task` | Read | Return one task with state, team, owners, labels, epic/sprint references, description, deadline, blocking fields, and compact relationship metadata. |
| `get_task_context` | Read | Existing richer multi-task context tool; retained for linked docs, comments, and Git context. |
| `create_task` | Write | Existing tool, extended with optional `sprint_id`, `severity`, `blocked`, `blocker`, and checklist items. A sprint target supplies `sprint_id` by default. |
| `create_task_batch` | Write | Existing epic-planning batch tool; remains epic-specific to preserve its review and dependency semantics. Sprint agents create tasks through bounded `create_task` calls. |
| `update_task` | Write | Update bounded editable fields: name, description, type, epic, sprint, owners, estimate, priority, severity, deadline, blocked state, blocker, and labels. Team moves, archive, and deletion are excluded. |
| `update_task_state` | Write | Existing state-transition tool; retained as the only workflow-state mutation. |
| `set_task_dependencies` | Write | Existing dependency tool; retained with cycle and workspace validation. |

`create_task` and `update_task` validate that team, sprint, epic, owners, labels, workflow, and state belong to the run workspace and are compatible. When both a sprint target and an explicit `sprint_id` are supplied, they must match. Empty strings clear optional epic, sprint, deadline, and blocker fields where the underlying PM update contract supports clearing.

### Epics

| Tool | Mode | Purpose |
| --- | --- | --- |
| `list_epics` | Read | List compact epics with team, state, owner, dates, health, labels, and progress stats. Filters: team, state, label, and archived. |
| `get_epic` | Read | Return one epic with description, labels, objectives, stats, planning repository, and current health. |
| `list_epic_tasks` | Read | Existing target-aware task list; retained. |
| `create_epic` | Write | Create an epic with team, state, owner, dates, color, health, labels, and optional planning repository. Agent auto-run fields are excluded. |
| `update_epic` | Write | Update the same operational fields except archive and assigned-agent configuration. |

The existing spec approval and epic planner tools remain unchanged.

### Sprints

| Tool | Mode | Purpose |
| --- | --- | --- |
| `list_sprints` | Read | List compact sprints with dates, status, team, labels, and progress stats. Filters: team, status, and archived. |
| `get_sprint` | Read | Return one sprint with full description, dates, status, team, labels, and stats. Defaults to the current sprint target. |
| `list_sprint_tasks` | Read | Return compact non-archived tasks in a sprint. Defaults to the current sprint target. |
| `create_sprint` | Write | Create a sprint with name, description, dates, team, and labels. |
| `update_sprint` | Write | Update name, description, dates, team, and labels. Archive and deletion are excluded. |

Sprint dates use `YYYY-MM-DD`. Existing sprint overlap, duration, and team-management rules remain enforced by `PMSprintService`.

### Objectives and key results

| Tool | Mode | Purpose |
| --- | --- | --- |
| `list_objectives` | Read | List compact objectives with type, state, dates, health, teams, owners, labels, linked epics, and stats. |
| `get_objective` | Read | Return one objective with description, associations, key results, and progress stats. Defaults to the current objective target. |
| `create_objective` | Write | Create an objective with type, state, dates, health, teams, owners, labels, and linked epics. |
| `update_objective` | Write | Update the same operational fields except archive. |
| `create_key_result` | Write | Create a key result on an objective with name, result type, initial/current/target values, and note. |
| `update_key_result` | Write | Update bounded key-result fields. Deletion is excluded. |

### Collaboration

| Tool | Mode | Purpose |
| --- | --- | --- |
| `add_task_comment` | Write | Existing task-specific compatibility alias. |
| `add_pm_comment` | Write | Add a comment to a task, epic, sprint, or objective. The current target supplies entity type and ID by default. |
| `list_task_checklist` | Read | Existing catalog entry; add the missing internal-command executor. |
| `create_task_checklist_item` | Write | Create one checklist item on a task. |
| `update_task_checklist_item` | Write | Update item text, completion, assignee, due date, or position. |

Comment and checklist writes use the run actor for attribution. Comment content and checklist text keep the existing service limits and sanitization.

## Agent targets

Add `sprint` and `objective` to the canonical agent target type set and the custom-agent configuration UI. Both targets are supported by generic direct runs and manual “run now” launches.

Target context resolution returns:

- Sprint: ID, name, description, dates, derived status, team, labels, and stats.
- Objective: ID, name, description, type, state, dates, health, teams, owners, labels, linked epics, key results, and stats.

Target-aware tools default their entity ID from the run target. Explicit IDs remain available for workspace-targeted agents. A supplied explicit ID that conflicts with the current target is rejected for mutating operations.

Parent targets may operate on validated child entities. In particular, an epic-targeted agent may mutate a task only when that task belongs to the target epic, and a sprint-targeted agent may mutate a task only when that task belongs to the target sprint. This child-entity rule applies to task updates, state changes, comments, checklists, and dependencies. It does not permit access to an unrelated task merely because the task is in the same workspace.

### Supported-target and defaulting matrix

| Tool group | Supported run targets | Required or defaulted identity |
| --- | --- | --- |
| Workspace/member/team/workflow/label discovery | `workspace`, `task`, `epic`, `sprint`, `objective` | Workspace always comes from the run; explicit workspace IDs are not accepted. |
| `list_tasks` | `workspace`, `task`, `epic`, `sprint`, `objective` | Task target defaults `task_id`; epic target defaults `epic_id`; sprint target defaults `sprint_id`; objective target applies no implicit task filter. |
| `get_task` / `get_task_context` | `workspace`, `task`, `epic`, `sprint`, `objective` | Task target defaults `task_id`; otherwise explicit task ID is required. Parent-target reads must pass normal workspace/team access checks. |
| `create_task` | `workspace`, `epic`, `sprint` | `team_id` is always required; epic/sprint targets default their association ID. An explicit association must match the target. |
| `update_task`, task state, task comments/checklists | `workspace`, `task`, `epic`, `sprint` | Task target defaults task ID; other targets require an explicit task ID. Epic/sprint parent targets validate the child association before writing. |
| `set_task_dependencies` | `workspace`, `epic`, `sprint` | All task IDs are explicit. Under epic/sprint targets, every changed task must belong to that parent. |
| Epic list/get/create/update | `workspace`, `epic` | Epic target defaults epic ID for get/update; list/create use the run workspace. |
| Sprint list/get/tasks/create/update | `workspace`, `sprint` | Sprint target defaults sprint ID for get/tasks/update; list/create use the run workspace. |
| Objective list/get/create/update | `workspace`, `objective` | Objective target defaults objective ID for get/update; list/create use the run workspace. |
| Key-result create/update | `objective` | Objective target supplies `objective_id`; update also requires `key_result_id` belonging to that objective. |
| `add_pm_comment` | `workspace`, `task`, `epic`, `sprint`, `objective` | Entity targets default type and ID; workspace runs require both explicitly. |

All dates use `YYYY-MM-DD`. List results return stable IDs, human-readable names, relevant state/status fields, and pagination metadata where the backing service is paginated. Create/update results return the updated compact entity plus canonical IDs. On association-bearing updates, omitted arrays preserve current associations and supplied arrays replace the complete set; an empty supplied array clears the set. Empty strings clear optional scalar task associations where supported.

## Runtime and data flow

1. The agent configuration UI loads the workspace tool catalog and stores canonical aliases in `allowed_tools`.
2. Agent launch sends the allowed aliases and a target to Agent Runtime.
3. Agent Runtime invokes the Helpin host command associated with an alias.
4. `AgentRuntimeHostService` resolves the run workspace and actor and builds `InternalCommandContext`.
5. The runtime host places the resolved `authorization.Actor` into the Go context, preserving workspace role and team-role information for service-level checks. `InternalCommandService` verifies the command supports the run target and applies central module permission checks.
6. The command parses and validates bounded input, resolves target defaults, and calls an existing PM or workspace service.
7. The PM service and repository enforce workspace and accessible-team scope and emit existing activity/realtime side effects.
8. The command returns compact JSON suitable for another model step.

No command calls an HTTP handler, concatenates SQL, or bypasses the service layer.

## Authorization and safety

- Discovery and PM read tools require `pm.read`, except `list_workspace_members`, which requires workspace read plus PM read in the command policy.
- PM mutations require `pm.edit`.
- Task reads and writes retain existing accessible-team filtering. Task creation requires membership in the destination team for non-privileged actors.
- Epic reads retain accessible-team filtering, and epic writes retain the existing rule that regular members may edit epics only in their own teams.
- Sprint reads retain accessible-team filtering. Sprint create/update requires an admin/owner or a manager of the sprint team.
- Objective reads remain intentionally workspace-wide for every actor with `pm.read`; team filtering is not added to objective reads. Objective and key-result create/update requires an admin/owner or a manager of at least one linked objective team. Objectives with no teams remain admin/owner-managed.
- `AgentRuntimeHostService` must propagate the fully resolved actor into the service call context. Stamping only `ActorRole` and flat team IDs onto `InternalCommandContext` is insufficient because sprint/objective management depends on team-owner roles.
- Entity IDs are loaded and checked against `InternalCommandContext.WorkspaceID` before use.
- Team-scoped actors only receive or mutate entities visible to their accessible team set, except for the intentionally workspace-wide objective read model described above.
- Read results are compact by default and capped at 100 items per call.
- Mutating tools reject empty required names, malformed dates, cross-workspace IDs, incompatible team/workflow/sprint combinations, and writes with no editable fields.
- No new tool deletes, archives, sends external communication, starts another agent implicitly, or edits workflow/automation configuration.

## Catalog and executor consistency

The frozen agent tool catalog remains the UI/runtime contract. PM command metadata lives in `commandtools`, and each exposed PM alias must map to exactly one executable internal command.

Add a parity test that:

- collects all `PM / Tasks`, `PM / Epics`, `PM / Sprints`, and `PM / Objectives` catalog entries intended for Helpin host execution;
- verifies every entry resolves to a command definition;
- verifies every exposed PM command appears in the catalog;
- verifies aliases are unique; and
- rejects deprecated `assign_task_agent` from new preset defaults while retaining its compatibility entry until a separate removal decision.

The parity test also covers PM-supporting discovery aliases in the `Workspace` category, including `list_workspace_members` and `list_workspace_teams`.

This closes the current gap where a tool can be selectable without an executor.

## Catalog organization and presets

Split the current broad `PM / Tasks` presentation into four ordered categories:

- `PM / Tasks`
- `PM / Epics`
- `PM / Sprints`
- `PM / Objectives`

Existing tools keep their names. Tools move only between display categories when that improves discovery.

Custom agents can select every new tool. Built-in preset expansion is explicit:

- `ask_agent`: add every new read alias (`list_workspace_members`, `list_pm_labels`, `get_task`, `list_epics`, `get_epic`, `list_sprints`, `get_sprint`, `list_sprint_tasks`, `list_objectives`, `get_objective`). Add no PM write aliases.
- `command_agent`: add all new read and write aliases. This preset is the bounded sub-agent executor; the orchestrator still narrows `allowed_tools` per run and obtains its existing launch approval.
- `epic_planner`: add `list_workspace_members`, `list_pm_labels`, `get_task`, `list_epics`, and `get_epic`. Its existing task-batch and approval contract remains unchanged; do not add unrelated sprint/objective writes.
- `task_planner`: add `list_workspace_members`, `list_pm_labels`, and `get_task`. Add no new PM writes.
- `marketer`: add the new read aliases only. Its existing `create_task` gains the compatible new fields, but the preset receives no new mutation aliases.
- `crm_operator`, `support_agent`, `documentation_agent`, `code_builder`, and `review_agent`: no automatic tool expansion.

Workspace-defined custom presets and custom agents are not silently modified; administrators can select the new aliases through the catalog.

System-agent prompt text changes only where a prompt enumerates the old tool set.

## Error behavior

Commands return clear, model-actionable errors without sensitive data. Examples include:

- `sprint not found in this workspace`
- `sprint team does not match task team`
- `workflow state does not belong to the resolved workflow`
- `actor is not authorized to manage any team linked to this objective`
- `at least one editable field is required`
- `command does not support target type objective`

Repository and service errors are wrapped with operation context. Sensitive descriptions, comments, tokens, or user contact data are not logged.

## Testing strategy

Implementation follows red-green-refactor for each tool group.

1. Metadata/schema tests verify names, categories, required fields, enums, optional clearing behavior, and bounded list inputs.
2. Internal-command tests execute real command definitions against the existing test database and verify compact output plus persisted mutations.
3. Target tests verify sprint/objective defaults, explicit-ID behavior, and conflict rejection.
4. Authorization tests verify read versus write permission mapping, runtime actor propagation, team-scoped task/epic/sprint access, workspace-wide objective reads, and manager-only objective/key-result writes.
5. Isolation tests reject cross-workspace entities and related IDs.
6. Service behavior tests cover sprint date/team validation, task-sprint compatibility, objective/key-result validation, and checklist/comment attribution.
7. Catalog parity tests prove selectable PM tools have executors and executable PM aliases are selectable.
8. Agent launch tests cover sprint and objective targets, target-context projection, and a team-restricted actor launching against a multi-team objective.
9. Frontend tests cover target options and tool-category rendering.
10. Verification runs focused Go packages, `go test ./...`, relevant frontend tests, and the production frontend build.

## Rollout and compatibility

No database migration is required. All changes use existing PM models and tables.

Existing agent configurations and tool aliases continue to work. New tools are opt-in for custom agents. Built-in preset updates flow through the existing preset reconciliation/version mechanism. `create_task` gains optional fields without changing existing required fields or default workflow resolution.

The change is complete when a sprint-targeted agent can read sprint context, discover members/labels/workflow stages, inspect existing sprint tasks, create tasks directly in the sprint, update those tasks, add comments/checklists/dependencies, and finish without using an HTTP endpoint or an unregistered runtime tool.
