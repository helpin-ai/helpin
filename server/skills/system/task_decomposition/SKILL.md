---
name: coding_task_decomposition
description: Breaks approved product specs into implementation-ready coding tasks with vertical slices, affected files, dependencies, and test strategy.
metadata:
  title: Coding Task Decomposition
  required_tools:
    - publish_task_plan
    - update_plan
    - create_task_batch
  supported_runtimes:
    - native_sdk
---

For the task plan preview, use `publish_task_plan`. The paths below are
illustrative placeholders; replace them with files found in the selected repository:

```json
{
  "title": "Task Plan",
  "content": {
    "summary": "...",
    "proposed_tasks": [
      {
        "ref": "task_1",
        "name": "Add tracking helper",
        "description": "...",
        "task_type": "chore",
        "acceptance_criteria": ["..."],
        "dependency_refs": [],
        "slice_type": "enabler",
        "implementation_brief": {
          "approach": "...",
          "files_to_modify": [
            {
              "path": "path/to/tracking/helper.go",
              "action": "modify",
              "description": "..."
            }
          ],
          "test_strategy": ["..."]
        }
      },
      {
        "ref": "task_2",
        "name": "Wire tracking into capture errors",
        "description": "...",
        "task_type": "feature",
        "acceptance_criteria": ["..."],
        "dependency_refs": ["task_1"],
        "slice_type": "vertical",
        "implementation_brief": {
          "approach": "...",
          "files_to_modify": [
            {
              "path": "path/to/capture/errors.go",
              "action": "modify",
              "description": "..."
            }
          ],
          "test_strategy": ["..."]
        }
      }
    ],
    "open_questions": [],
    "risks": []
  }
}
```

The value of `content` must be a JSON object. Do not stringify the JSON object into a string.
Call `publish_task_plan` only when you have the full structured payload ready in a single tool call. Do not send title-only payloads. Do not send a raw string wrapper. Do not send partial JSON. Do not put the plan in prose and expect the tool to extract it.
The minimum valid payload is:

```json
{
  "content": {
    "summary": "...",
    "proposed_tasks": [
      {
        "name": "Add event classification helper",
        "description": "...",
        "task_type": "feature",
        "acceptance_criteria": ["GIVEN ... WHEN ... THEN ..."],
        "dependency_refs": []
      }
    ]
  }
}
```

Inside `proposed_tasks`, use the canonical field names `name` and `task_type`.
Each entry in `proposed_tasks` must be a JSON object. Never send arrays of strings, refs, placeholders, partial fragments, or key names.
Use `dependency_refs` only for refs that appear elsewhere in the same `proposed_tasks` array. Example: `"dependency_refs": ["task_1"]` means the current task depends on the task whose ref is `task_1`.

Invalid examples:

```json
{
  "content": {
    "summary": "placeholder",
    "proposed_tasks": ["task_1"]
  }
}
```

```json
{
  "title": "Task Plan"
}
```

## Task Planning

Work like a technical product planner interactively decomposing the approved spec into implementation tasks.

- Read the approved spec from linked documents and use tools to understand the current codebase.
- Discuss decomposition with the human when priorities, constraints, or slicing preferences are unclear.
- Tasks must not be created without a team. If the epic has no team, call `list_workspace_teams` and ask the human to choose the team inline before task creation.

### Vertical Slicing (Critical)

Default to vertical slices for user-visible work. A vertical slice cuts through the necessary layers to deliver one independently shippable unit of value.

### Blocker & Enabler Consolidation

When multiple tasks share a common blocker, consolidate all shared setup into a single enabler task rather than scattering setup across tasks.

### Task Separation & Scoping

Each task must have a clearly bounded scope with zero overlap with other tasks.

### Implementation Briefs (Required)

Every task must include a detailed implementation brief. `files_to_modify` must be an array of objects with `path`, `action`, and `description`.

### Acceptance Criteria (Required)

Every task must include concrete acceptance criteria using GIVEN/WHEN/THEN format.

### Task Ordering

Order tasks by dependency graph: enablers first, then independent tasks, then dependent tasks.

When you have enough information, load the `task_plan_publishing` skill with `read_skill` before publishing or requesting approval, then follow that skill's current tool contract. Each `publish_task_plan` call replaces the previous task plan preview.

## After Task Approval

- Treat the approved `publish_task_plan` artifact as the source of truth.
- Call `create_task_batch` with `{"proposed_tasks":[...]}`, using the full approved `proposed_tasks` array and preserving every approved field and `dependency_refs`.
- Finish only after `create_task_batch` succeeds. Do not claim the task plan was applied based on approval alone.
- Do not write the PRD again.
