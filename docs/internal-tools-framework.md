# Internal Tool Framework

## Purpose

This is the single source of truth for internal tools in Helpin.

Use it when adding or changing:

- backend-only command tools in `server/internal/commandtools/`
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

## System Tools vs Agent-Facing Tools

There are three distinct layers. Keep them separate:

| Layer | Model-visible? | Primary code | Purpose |
| --- | --- | --- | --- |
| System/internal command | No, unless wrapped | `server/internal/commandtools/` plus service/repository code | Reusable backend action with product invariants |
| Agent-facing runtime tool | Yes | `server/internal/worker/tools.go` and `tools_*.go` | Model-visible tool contract and execution handler |
| Command-backed runtime tool | Yes | Both of the above | Model-visible alias that delegates to a reusable internal command |

### System/Internal Commands

System/internal commands are backend action contracts. They are useful when the operation is a durable product mutation or a reusable backend action that should not be tied to one model runtime.

Use a system/internal command when:

- the action changes product data
- the action is reused by automation rules, services, or multiple runtime tools
- domain invariants must be enforced outside the model prompt
- the same mutation should behave consistently whether triggered by an agent, automation rule, or backend workflow

Do not expose an internal command directly to the model. If the model needs it, wrap it as an agent-facing runtime tool with a stable tool name, schema, validation, allowlist entry, and tests.

Boundary:

- reusable product mutations should usually be command-backed
- product reads and queries should usually stay runtime-tool-only unless multiple backend surfaces need the exact same shared contract
- runtime-local or external capabilities should stay runtime-tool-only

Decision table:

| Capability type | Default shape | Why |
| --- | --- | --- |
| Product mutation | Command-backed runtime tool | Shared invariants and consistent behavior matter across agents, automations, and backend workflows |
| Product read/query | Agent-facing runtime tool | The model needs a contract, but backend command reuse is often unnecessary for read paths |
| Runtime-local or external capability | Agent-facing runtime tool | These are execution-time capabilities, not durable product commands |

Examples:

- `create_document`: product mutation, so command-backed is appropriate
- `list_collections`: product read/query, so runtime-tool-only is appropriate unless a broader shared backend contract emerges
- `web_search_exa`: external search capability, so runtime-tool-only is appropriate

### Agent-Facing Runtime Tools

Agent-facing runtime tools are the contracts the model sees and calls. These are registered in `ToolRegistry`, filtered by the active allowed-tool set, serialized into provider tool definitions, executed through `ExecuteAllowed`, and shown in tool catalog surfaces.

Use an agent-facing runtime tool when:

- the model must decide when to call the capability
- the tool result should influence the model's next step
- the action is runtime-local, such as filesystem, shell, web search, preview, approval, or interaction
- a backend mutation needs a model-facing wrapper

The model-facing contract is the source of truth for prompts and skills. The internal command, if any, is backend reuse.

### Runtime Exposure

Adding a tool implementation is not enough to make it usable. Exposure is controlled by:

- `server/internal/worker/runtime_profiles.go` for preset/runtime defaults
- `agent.allowed_tools` overrides, resolved by `ResolveAgentProfile`
- `ExecutionContext.AllowedTools`, which is enforced by `ToolRegistry.ExecuteAllowed`
- selective native skill activation, which can make the model see only the skills and policies active for the current turn
- tool catalog metadata in `server/internal/worker/tool_catalog.go` and `server/internal/commandtools/metadata.go`

This means a tool can exist in the registry but still be unavailable to a specific agent run.

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

Do not interpret "business tool" to mean "must be command-backed". The key question is whether the tool is a reusable mutation with product invariants, not whether it merely touches product data or reads from product tables.

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
|   +-- request_user_input
|   +-- request_approval
|   +-- request_review_checkpoint
|   +-- preview / publish tools
|
+-- Product tools
    +-- reads
    |   +-- list_documents
    |   +-- list_deals
    |   +-- list_epic_tasks
    |
    +-- mutations
        +-- create_task_batch
        +-- update_task_state
        +-- write_document_content
        +-- update_deal_stage
```

For a command-backed mutation, the relationship should be:

```text
tool alias                  internal command
-----------                 ----------------
create_task_batch    -----> pm.create_task_batch
update_task_state    -----> pm.update_task_state
set_task_dependencies ----> pm.set_task_dependencies
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

- identifiers: `task_id`, `document_id`, `deal_id`
- collections: `tasks`, `dependencies`, `questions`
- free text: `content`, `summary`, `description`, `note`
- filters: `query`, `limit`, `status`, `space_id`, `stage_id`
- booleans: `replace`, `is_internal`
- enums: `task_type`, `priority`, `status`

If a field does not fit one of those patterns, document why it needs to exist.

## Naming Rules

### Tool names

Use `snake_case` action names:

- `read_file`
- `list_documents`
- `update_task_state`
- `request_user_input`

### Field names

Use consistent names across PM, Docs, CRM, and Support:

- IDs end in `_id`: `task_id`, `document_id`, `deal_id`
- lists use plural nouns: `tasks`, `dependencies`, `questions`
- booleans read like booleans: `replace`, `is_internal`
- free-form text is usually `content`, `summary`, `description`, or `note`
- filters usually use `limit`, `query`, `status`, `space_id`, `stage_id`

Avoid mixed naming like `taskId`, `docId`, or vague fields like `data` unless there is a strong reason.

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
		TaskID string `json:"task_id"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(params.TaskID) == "" {
		return "", fmt.Errorf("task_id is required")
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 20
	}

	result := map[string]any{
		"task_id": params.TaskID,
		"limit":   params.Limit,
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

- `task_id is required`
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
{"status":"updated","task_id":"task_123","state_id":"done"}
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

Current examples:

- canonical task creation input: `tasks`
- accepted task-plan input alias: `proposed_tasks`
- legacy aliases such as `request_human_input` and `request_human_approval` are decode/runtime compatibility only; do not use them in new prompt examples

## Adding New Capabilities

Start by choosing the right shape. Do not add both a command and a runtime tool unless both contracts are needed.

### Add a System/Internal Command

Use this path for backend-owned product actions that may be reused outside a model runtime.

Steps:

1. Define the command name and metadata in `server/internal/commandtools/metadata.go`.
2. Keep the command name domain-scoped, such as `pm.create_task_batch` or `docs.write_document_content`.
3. Put product validation and invariants in service/repository code, not in prompts.
4. Add or reuse a service method that can be called by workers, automation rules, or workflows.
5. Add repository/service tests for the mutation and edge cases.
6. Do not add the command to runtime profiles unless the model needs to call it through a runtime tool.

If this command should be model-callable, add an agent-facing runtime tool wrapper as a separate step.

### Add a Runtime-Only Agent Tool

Use this path for model-facing capabilities that are local to the agent runtime and do not need a reusable backend command.

Examples:

- filesystem and code-search tools
- `run_command`
- web search
- previews and approval/interaction tools
- read-only context tools

Steps:

1. Add the tool definition in `server/internal/worker/tools.go`.
2. Implement the handler in the matching `server/internal/worker/tools_*.go` file.
3. Decode into a typed Go struct and validate before doing work.
4. Add or update the category in `server/internal/worker/tool_catalog.go`.
5. Add the tool to the right runtime profile in `server/internal/worker/runtime_profiles.go`, or require explicit `agent.allowed_tools`.
6. Add worker tests for schema presence, allowlist enforcement, successful execution, and validation errors.
7. Update skills, prompt examples, and docs if the model needs exact field names or sequencing rules.

### Add a Command-Backed Agent Tool

Use this path when the model should call a business mutation, but the mutation must remain reusable and invariant-safe outside the model runtime.

Steps:

1. Add or update the internal command metadata in `server/internal/commandtools/metadata.go`.
2. Add the tool implementation in the matching `server/internal/worker/tools_*.go` file.
3. Register the alias through `registerSharedCommandTools` in `server/internal/worker/tools.go`.
4. In the tool handler, validate model-facing input before delegating to the internal command.
5. Delegate through `executeInternalCommand` when command execution is available.
6. Keep a service fallback only when existing tests or runtime paths still require it.
7. Add or update `server/internal/worker/tools_command_backed_test.go` or a domain-specific worker test.
8. Add or update runtime profile allowlists and catalog metadata.
9. Update skill instructions or prompt docs only with the agent-facing alias, not the internal command name.

### Add Planner-Specific Tool Behavior

Planner tools need extra care because approval, preview, and completion policies are enforced after execution.

Steps:

1. Update the tool schema and decode/validation logic.
2. Update `docs/plans/planner-tool-contract-reference.md`.
3. Update active skill instructions if the model must use a new sequence.
4. Update completion or approval-preview policy tests if the tool affects handoff requirements.
5. Verify native selective planner runs still derive active policy from the same active skill subset that provides the prompt contract.

For planner-specific payload details, use `docs/plans/planner-tool-contract-reference.md`.

## Quick Decision Rules

Use these defaults:

- if the tool acts on local repo state, keep it runtime-only
- if the tool is human-approval or preview UX, keep it runtime-only
- if the tool is a reusable product mutation, keep the tool contract stable and add or reuse internal-command backing
- if the output will be reused by later reasoning, return JSON
- if a second payload shape is not necessary, do not add one
