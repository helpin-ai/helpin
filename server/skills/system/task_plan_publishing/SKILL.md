---
name: task_plan_publishing
description: Operational contract for publishing, approving, and applying an Atlas task plan without malformed or partial tool calls.
metadata:
  title: Task Plan Publishing Tool
  required_tools:
    - publish_task_plan
    - request_approval
    - create_task_batch
  supported_runtimes:
    - codex
    - native_sdk
---

# `publish_task_plan` tool contract

Use this skill when preparing or repairing a `publish_task_plan` call, requesting approval for that preview, or applying the approved plan with `create_task_batch`. The system prompt owns planning order and task-decomposition judgment, not this skill.

Send the complete current plan in one call. `content` must be a JSON object, never a JSON-encoded string, prose wrapper, list of task refs, or partial placeholder.

```json
{
  "title": "Task Plan",
  "content": {
    "summary": "Short outcome-oriented summary",
    "proposed_tasks": [
      {
        "ref": "task_1",
        "name": "Implementation-ready task name",
        "description": "Bounded scope and intended outcome",
        "task_type": "feature",
        "acceptance_criteria": ["GIVEN ... WHEN ... THEN ..."],
        "dependency_refs": [],
        "slice_type": "vertical",
        "implementation_brief": {
          "approach": "Concrete implementation approach",
          "files_to_modify": [
            {
              "path": "path/to/file.go",
              "action": "modify",
              "description": "Why and how this file changes"
            }
          ],
          "test_strategy": "Tests that prove the acceptance criteria"
        }
      }
    ],
    "open_questions": [],
    "risks": []
  }
}
```

Before calling the tool, verify:

- `content.summary` is non-empty and `content.proposed_tasks` contains at least one object.
- Every task uses `name` and `task_type`; do not substitute `title` or `type`.
- Every task has at least one non-empty `acceptance_criteria` entry.
- Every explicit `ref` is unique. Every `dependency_refs` value names a ref in the same call, never itself, and the dependency graph has no cycle.
- `files_to_modify` entries are objects with `path`, `action`, and `description`.
- The call contains the full replacement plan. A successful call replaces the prior task-plan preview.

After the preview is published, call `request_approval` with `phase="tasks"` and `preview_panel_key="task_plan"`, then stop that turn.

After approval, call `create_task_batch` with the full approved task array:

```json
{
  "proposed_tasks": [
    {
      "ref": "task_1",
      "name": "Implementation-ready task name",
      "description": "Bounded scope and intended outcome",
      "task_type": "feature",
      "acceptance_criteria": ["GIVEN ... WHEN ... THEN ..."],
      "dependency_refs": []
    }
  ]
}
```

Preserve every approved task field, stable ref, and `dependency_refs`. Do not claim the task plan was applied or finish until `create_task_batch` succeeds.
