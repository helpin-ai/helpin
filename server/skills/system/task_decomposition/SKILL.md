---
name: task_decomposition
description: Implementation task planning guidance including vertical slicing and task-plan tool contracts.
metadata:
  title: Task Decomposition
  required_tools:
    - publish_task_plan
    - update_plan
  supported_runtimes:
    - native_sdk
---

For the task plan preview, use `publish_task_plan`:

```json
{
  "title": "Task Plan",
  "content": {
    "summary": "...",
    "proposed_tasks": [
      {
        "ref": "story_1",
        "name": "Add tracking helper",
        "description": "...",
        "story_type": "chore",
        "acceptance_criteria": ["..."],
        "dependency_refs": [],
        "slice_type": "enabler",
        "implementation_brief": {
          "approach": "...",
          "files_to_modify": [
            {
              "path": "server/internal/worker/tools.go",
              "action": "modify",
              "description": "..."
            }
          ],
          "test_strategy": ["..."]
        }
      },
      {
        "ref": "story_2",
        "name": "Wire tracking into capture errors",
        "description": "...",
        "story_type": "feature",
        "acceptance_criteria": ["..."],
        "dependency_refs": ["story_1"],
        "slice_type": "vertical",
        "implementation_brief": {
          "approach": "...",
          "files_to_modify": [
            {
              "path": "server/internal/capture/errors.go",
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

Inside `proposed_tasks`, use the canonical field names `name` and `task_type`. Legacy `proposed_stories` and `story_type` are still accepted for compatibility.
Use `dependency_refs` only for refs that appear elsewhere in the same `proposed_tasks` array. Example: `"dependency_refs": ["story_1"]` means the current task depends on the task whose ref is `story_1`.

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

When you have enough information, call `publish_task_plan` with the full current plan. Each `publish_task_plan` call replaces the previous task plan preview.

## After Task Approval

- Treat the approved `publish_task_plan` artifact as the source of truth.
- The platform will apply that approved artifact and create the tasks.
- After approval, do not replay the same plan through story-creation or document-mutation tools.
- Do not write the PRD again.
