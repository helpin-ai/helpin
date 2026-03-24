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

### `publish_story_plan`

```json
{
  "title": "Story Plan",
  "content": {
    "summary": "...",
    "proposed_stories": [
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

### `publish_story_plan_doc`

```json
{
  "title": "Story Planning Document",
  "content": "# Outcome\n..."
}
```

### `request_human_approval`

```json
{
  "phase": "prd|stories|story_doc",
  "title": "...",
  "summary": "..."
}
```

## Canonical Story Plan Fields

Inside `proposed_stories`, use:

- `ref`
- `name`
- `description`
- `story_type`
- `acceptance_criteria`
- `dependency_refs`
- `slice_type`
- `implementation_brief`

Do not use:

- `title` instead of `name`
- `type` instead of `story_type`

## Compatibility Aliases

These are accepted during decode for backward compatibility:

- `title` -> `name`
- `type` -> `story_type`
- `test_strategy` as either:
  - string
  - array of strings
- preview `content` as:
  - structured JSON object
  - stringified JSON

No other planner payload aliases should be added casually.

## `dependency_refs`

`dependency_refs` must point to refs that exist elsewhere in the same `proposed_stories` array.

Example:

```json
{
  "ref": "story_2",
  "dependency_refs": ["story_1"]
}
```

That means `story_2` depends on the story whose ref is `story_1`.

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
