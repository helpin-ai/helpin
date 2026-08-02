# Planner Tool Contract Reference

## Purpose

This is the compact source of truth for planner tool payloads.

Use it when changing:

- planner system prompts
- planner tool schemas
- planner payload decode/normalize logic
- approved planner artifact application

## Canonical Planner Tools

### `publish_prd_draft`

```json
{
  "title": "PRD Draft",
  "content": "# Problem\n..."
}
```

### `publish_task_plan`

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
              "path": "server/internal/worker/tools.go",
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

### `publish_task_plan_doc`

```json
{
  "title": "Task Planning Document",
  "content": "# Outcome\n..."
}
```

Rules:

- `content` is required
- `content` must be the full markdown draft being reviewed
- `title` is optional metadata only and must not be sent by itself

### Approved task planning document application

After a human approves the `task_doc` preview, the task planner applies it through two bounded product tools:

1. Call `ensure_task_plan_doc` with an empty object:

```json
{}
```

This creates or loads the canonical task planning document, attaches it to the current task, and returns:

```json
{
  "document_id": "...",
  "title": "..."
}
```

2. Call `write_document_content` with the returned ID and the full approved markdown:

```json
{
  "document_id": "...",
  "content": "# Outcome\n..."
}
```

Rules:

- use `ensure_task_plan_doc`; do not create and link a generic document manually
- write the exact full markdown draft that the human approved
- do not claim the document was persisted or attached until both calls succeed
- repository access remains read-only throughout the task-planning run

### `request_approval`

```json
{
  "phase": "prd|tasks|task_doc",
  "preview_panel_key": "prd_draft|task_plan|task_plan_doc",
  "title": "...",
  "summary": "..."
}
```

For planner approval phases:

- `phase="prd"` requires a same-turn `publish_prd_draft` and `preview_panel_key="prd_draft"`
- `phase="tasks"` requires a same-turn `publish_task_plan` and `preview_panel_key="task_plan"`
- `phase="task_doc"` requires a same-turn `publish_task_plan_doc` and `preview_panel_key="task_plan_doc"`

Treat `request_approval` as the final action in that turn. Do not call additional tools or append approval-choice prose after it.

Do not emit `request_approval` for a planner phase before publishing the corresponding preview in that same turn.

### `request_review_checkpoint`

`request_review_checkpoint` is for review-agent checkpoints, not epic/task planner approval. Planner approval flows should use `request_approval`.

## Canonical Task Plan Fields

Inside `proposed_tasks`, use:

- `ref`
- `name`
- `description`
- `task_type`
- `acceptance_criteria`
- `dependency_refs`
- `slice_type`
- `implementation_brief`

Do not use:

- `title` instead of `name`
- `type` instead of `task_type`

## Compatibility Aliases

These are accepted during decode for backward compatibility only. Do not use them in new prompt examples or skill instructions.

- `title` -> `name`
- `type` -> `task_type`
- `test_strategy` as either:
  - string
  - array of strings
- preview `content` as:
  - structured JSON object
  - stringified JSON

Legacy story-era aliases should not be added to prompts. If one still exists in decoder code, treat it as temporary compatibility, not as part of the model-facing contract.

No other planner payload aliases should be added casually.

## `dependency_refs`

`dependency_refs` must point to refs that exist elsewhere in the same `proposed_tasks` array.

Example:

```json
{
  "ref": "task_2",
  "dependency_refs": ["task_1"]
}
```

That means `task_2` depends on the task whose ref is `task_1`.

Rules:

- no self-dependency
- no unknown refs
- no cycles

## `implementation_brief`

Canonical shape:

```json
{
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
```

Notes:

- `files_to_modify` must be structured objects, not prose
- `test_strategy` should be emitted as an array of strings in prompts/examples
- backend currently normalizes `test_strategy` into an internal string form

## Error Style

Planner validation errors returned to the model should be:

- short
- field-specific
- repair-oriented

Good:

- `task 1 is missing name; use field "name" for the task title`
- `task 2 references unknown dependency ref "task_7" in dependency_refs`
- `implementation_brief.test_strategy must be a string or array of strings`

Avoid:

- raw JSON unmarshal errors with no field guidance
- vague messages like `invalid payload`

## Test Coverage Expectations

Changes to planner contracts should update:

- `server/internal/worker/tools_test.go`
- `server/internal/worker/tools_interaction_test.go`
- `server/internal/worker/tools_planner_test.go`
- `server/internal/model/agent_planning_test.go`
- `server/internal/temporalapp/activities_test.go`

At minimum, tests should cover:

- canonical payload shape
- accepted compatibility aliases
- approved artifact decode
- dependency validation
- prompt snippets that models rely on
