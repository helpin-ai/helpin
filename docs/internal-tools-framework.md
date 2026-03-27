# Internal Tool Framework

## Purpose

This is the single source of truth for internal tools in Helpin.

Use it when adding or changing:

- runtime tools in `server/internal/worker/tools.go`
- tool implementations in `server/internal/worker/tools_*.go`
- tool catalog metadata in `server/internal/worker/tool_catalog.go`
- runtime allowlists in `server/internal/worker/runtime_profiles.go`
- prompt examples or docs that depend on exact tool JSON

Current model:

1. tools are the main model-facing contract
2. some business-mutation tools are backed by internal commands internally
3. runtime-local tools stay tool-only

In practice, optimize for the tool contract first. Internal command backing is an implementation detail except for shared business mutations.

## What Exists Today

The main tool families in the app are:

- filesystem and code-search tools
- `run_command`
- git tools
- web search
- human-input and approval tools
- preview and publishing tools
- PM, Docs, CRM, Workspace, and Support tools

Some mutation tools use shared internal-command backing for consistency. That should continue for new reusable business mutations, but it does not change the tool contract itself.

## Tool Types And Relationships

The current shape of the system is:

```text
                          +----------------------+
                          |   Runtime Profiles   |
                          | allowed tool subsets |
                          +----------+-----------+
                                     |
                                     v
+---------+      +-------------------+-------------------+      +------------------+
|  Model  +----->+ Runtime Tool Contract (name/schema/ui) +----->+ Tool Handler     |
+---------+      +-------------------+-------------------+      | tools_*.go        |
                                     |                          +---------+--------+
                                     |                                    |
                                     |                                    |
                                     |                          runtime-only|command-backed
                                     |                                    |
                                     v                                    v
                          +--------------------+                +----------------------+
                          | Tool Catalog / UI  |                | Internal Command     |
                          | name/category/docs |                | shared backend action |
                          +--------------------+                +----------------------+
```

Tool families look like this:

```text
Runtime Tools
|
+-- Local runtime tools
|   +-- filesystem
|   +-- code search
|   +-- run_command
|   +-- git
|   +-- web search
|
+-- Interaction tools
|   +-- request_human_input
|   +-- request_human_approval
|   +-- preview / publish tools
|
+-- Product tools
    +-- reads
    |   +-- list_documents
    |   +-- list_deals
    |   +-- list_epic_stories
    |
    +-- mutations
        +-- update_story_state
        +-- write_document_content
        +-- update_deal_stage
```

For a command-backed mutation, the relationship should be:

```text
tool alias                  internal command
-----------                 ----------------
update_story_state   -----> pm.update_story_state
write_document_content ---> docs.write_document_content
update_deal_stage    -----> crm.update_deal_stage
```

Rule:

- the tool is still the primary model-facing contract
- the internal command should reuse the same semantics, not invent a second unrelated payload

## JSON Contract Framework

Every JSON tool contract has four layers:

```text
1. Tool Definition Layer
   name + description + input_schema

2. Decode Layer
   typed Go struct with json tags

3. Validation Layer
   required fields + defaults + context checks

4. Output Layer
   compact JSON or intentional plain text
```

That means a good tool contract is not just the schema in `tools.go`.
It is the full agreement between:

- the schema the model sees
- the Go struct we decode into
- the validation rules we enforce
- the output shape we return

For JSON tools, these four layers should stay aligned.

## Contract Anatomy

Think of each tool contract as this shape:

```text
Tool Name
  -> snake_case action

Input Schema
  -> JSON object
  -> explicit fields
  -> explicit required list

Decode Struct
  -> typed Go struct
  -> same field names as schema

Validation
  -> trim
  -> default
  -> cross-field checks
  -> run-context checks

Output
  -> compact JSON if machine-relevant
  -> plain text only when intentionally human-facing
```

## Default Tool Shape

Every tool should use a top-level JSON object.

Preferred schema shape:

```json
{
  "type": "object",
  "properties": {
    "field_name": {
      "type": "string",
      "description": "What this field means"
    }
  },
  "required": ["field_name"],
  "additionalProperties": false
}
```

Rules:

- use a top-level object, not arrays or positional inputs
- use `snake_case`
- define `properties` explicitly
- define `required` explicitly
- prefer `additionalProperties: false`
- give every field a short `description`
- use `enum` when the set of values is intentionally small
- use `oneOf` or `anyOf` only when the product really supports multiple shapes

### Canonical field categories

Most JSON tool fields fall into a small set of categories:

- identifiers: `story_id`, `document_id`, `deal_id`
- collections: `stories`, `dependencies`, `questions`
- free text: `content`, `summary`, `description`, `note`
- filters: `query`, `limit`, `status`, `space_id`, `stage_id`
- booleans: `replace`, `is_internal`
- enums: `story_type`, `priority`, `status`

If a field does not fit one of those patterns, document why it needs to exist.

## Naming Rules

### Tool names

Use `snake_case` action names:

- `read_file`
- `list_documents`
- `update_story_state`
- `request_human_input`

### Field names

Use consistent names across PM, Docs, CRM, and Support:

- IDs end in `_id`: `story_id`, `document_id`, `deal_id`
- lists use plural nouns: `stories`, `dependencies`, `questions`
- booleans read like booleans: `replace`, `is_internal`
- free-form text is usually `content`, `summary`, `description`, or `note`
- filters usually use `limit`, `query`, `status`, `space_id`, `stage_id`

Avoid mixed naming like `storyId`, `docId`, or vague fields like `data` unless there is a strong reason.

## Implementation Pattern

Each tool should follow the same handler pattern:

1. register it in `server/internal/worker/tools.go`
2. implement it in the matching domain file
3. unmarshal into a typed Go struct
4. validate and normalize early
5. check run-context constraints before doing work
6. return output in a shape the model can use

The contract should line up across files like this:

```text
server/internal/worker/tools.go
  -> tool name
  -> description
  -> input schema

server/internal/worker/tools_*.go
  -> decode struct
  -> validation
  -> execution
  -> output

server/internal/worker/tool_catalog.go
  -> category

server/internal/worker/runtime_profiles.go
  -> exposure policy
```

Example:

```go
func toolExample(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		StoryID string `json:"story_id"`
		Limit   int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(params.StoryID) == "" {
		return "", fmt.Errorf("story_id is required")
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 20
	}

	result := map[string]any{
		"story_id": params.StoryID,
		"limit":    params.Limit,
	}
	return toCompactJSONString(result), nil
}
```

## Validation Rules

Validation should be strict, short, and easy to repair.

Rules:

- wrap invalid JSON as a parse error
- trim strings before validating them
- apply defaults in one place
- validate cross-field relationships explicitly
- fail early on missing run context
- keep one canonical payload shape whenever possible

Good errors:

- `story_id is required`
- `dependencies is required`
- `list_directory requires a checked-out repository workspace for this run`

Avoid:

- `invalid payload`
- raw low-level errors with no field guidance
- silent coercions that hide malformed input

### Validation order

Use this order for JSON tools:

1. parse JSON
2. trim and normalize fields
3. apply defaults
4. validate required fields
5. validate field relationships
6. validate run context
7. execute

That keeps errors deterministic and easier for the model to recover from.

## Output Rules

Prefer structured JSON when later reasoning depends on the result.

Use JSON for:

- mutations
- multi-field reads
- anything the model may need to inspect in a later step
- anything UI or orchestration may reuse

Plain text is fine for:

- simple acknowledgements
- search/read empty states
- file and shell output where preserving raw text matters

Default rule: if the next step may parse it, return JSON.

### Output shape guidance

Prefer outputs like:

```json
{"status":"updated","story_id":"story_123","state_id":"done"}
```

```json
{"document_id":"doc_123","title":"Spec","has_draft_content":true}
```

Avoid mixing human prose with machine fields in the same payload.

If plain text is returned, make it explicit and useful:

- `No matches found.`
- `Directory is empty.`
- `list_directory requires a checked-out repository workspace for this run`

## Compatibility Rules

Prefer one canonical payload shape.

If a compatibility alias is needed:

- keep the canonical field obvious in prompts and examples
- document the alias near the decode logic
- keep the alias temporary

Current example:

- canonical: `stories`
- compatibility alias: `proposed_stories`

## Checklist For New Tools

When adding a tool:

1. add the tool definition in `server/internal/worker/tools.go`
2. add the implementation in the matching `tools_*.go` file
3. add or update the tool catalog category in `server/internal/worker/tool_catalog.go`
4. add or update runtime allowlists in `server/internal/worker/runtime_profiles.go`
5. add tests under `server/internal/worker/`
6. update prompt examples or docs if field names matter to the model

If the tool is a business mutation that also needs backend reuse, add or reuse the matching internal command too. The tool contract should still stay the canonical model-facing shape.

For planner-specific payload details, also use `docs/plans/planner-tool-contract-reference.md`.

## Quick Decision Rules

Use these defaults:

- if the tool acts on local repo state, keep it runtime-only
- if the tool is human-approval or preview UX, keep it runtime-only
- if the tool is a reusable product mutation, keep the tool contract stable and add or reuse internal-command backing
- if the output will be reused by later reasoning, return JSON
- if a second payload shape is not necessary, do not add one
