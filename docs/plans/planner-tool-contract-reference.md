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

### `request_review_checkpoint`

```json
{
  "phase": "prd|tasks|task_doc",
  "title": "...",
  "summary": "..."
}
```

For planner approval phases:

- `phase="prd"` requires a same-turn `publish_prd_draft`
- `phase="tasks"` requires a same-turn `publish_task_plan`
- `phase="task_doc"` requires a same-turn `publish_task_plan_doc`

Do not emit `request_review_checkpoint` for a planner phase before publishing the corresponding preview in that same turn.

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

These are accepted during decode for backward compatibility:

- `title` -> `name`
- `type` -> `task_type`
- `story_type` -> `task_type`
- `test_strategy` as either:
  - string
  - array of strings
- preview `content` as:
  - structured JSON object
  - stringified JSON

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

- `story 1 is missing name; use field "name" for the story title`
- `story 2 references unknown dependency ref "story_7" in dependency_refs`
- `implementation_brief.test_strategy must be a string or array of strings`

Avoid:

- raw JSON unmarshal errors with no field guidance
- vague messages like `invalid payload`

## Test Coverage Expectations

Changes to planner contracts should update:

- `server/internal/service/agent_system_prompts_test.go`
- `server/internal/worker/tools_test.go`
- `server/internal/model/agent_planning_test.go`
- `server/internal/temporalapp/activities_test.go`

At minimum, tests should cover:

- canonical payload shape
- accepted compatibility aliases
- approved artifact decode
- dependency validation
- prompt snippets that models rely on
