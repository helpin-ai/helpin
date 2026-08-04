---
name: task_plan_publishing
description: Operational input contract for publishing an Atlas task-plan preview without malformed or partial tool calls.
metadata:
  title: Task Plan Publishing Tool
  required_tools:
    - publish_task_plan
  supported_runtimes:
    - codex
    - native_sdk
---

# `publish_task_plan` tool contract

Use this skill only when preparing or repairing a `publish_task_plan` call. Agent behavior, planning order, approval rules, and task-decomposition judgment belong to the system prompt, not this skill.

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
