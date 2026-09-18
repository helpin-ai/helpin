# Internal tools framework

## Purpose

Use this guide when adding a Helpin product tool or changing its JSON contract.
Helpin owns product authorization and commands; the separate Agent Runtime owns
model loops and runtime-local tools. Start with [agents and automation](agents-and-automation.md)
for the execution boundary.

Current implementation map:

| Responsibility | Source |
| --- | --- |
| Command aliases, descriptions, schemas, and risk metadata | [Command metadata](../server/internal/commandtools/metadata.go) |
| Product command registration and dispatch | [Internal command service](../server/internal/service/internal_command_service.go) and its domain-specific `internal_command_*.go` files |
| Runtime provider discovery and exact tool-name dispatch | [Provider bridge](../server/internal/service/agent_runtime_mcp.go) |
| Runtime callback scope and actor resolution | [Host service](../server/internal/service/agent_runtime_host.go) |
| Configuration/UI catalog | [Host catalog](../server/internal/agentcontract/tool_catalog.go) |
| Presets and per-agent tool overrides | [Runtime profiles](../server/internal/agentcontract/runtime_profiles.go) and [profile resolution](../server/internal/agentcontract/resolve.go) |
| Canonical name normalization | [Tool constants](../server/internal/agentcontract/tool_constants.go) |

The former `worker/tools.go` registry and in-process executor are removed. New
capabilities must use the owning service or the separate Runtime repository.

Current model:

1. tools are the main model-facing contract
2. some business-mutation tools are backed by internal commands internally
3. runtime-local tools stay tool-only
4. Helpin product and interaction tools are exposed to model backends through MCP under bare canonical names such as `create_task` and `request_user_input`

In practice, optimize for the tool contract first. Internal command backing is an implementation detail except for shared business mutations.

## System Tools vs Agent-Facing Tools

There are three distinct layers. Keep them separate:

| Layer | Model-visible? | Primary code | Purpose |
| --- | --- | --- | --- |
| System/internal command | No, unless wrapped | `server/internal/commandtools/` plus service/repository code | Reusable backend action with product invariants |
| Agent-facing runtime tool | Yes | Separate Agent Runtime; Helpin provider bridge for product tools | Model-visible contract and execution routing |
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
- `web_search`: external search capability, so runtime-tool-only is appropriate

### Agent-Facing Runtime Tools

Agent-facing tools are the contracts the model sees and calls. Helpin exposes
command-backed product tools through `ListProviderTools` and `CallProviderTool`.
Agent Runtime selects and executes tools using the run contract. The host catalog
is configuration/UI metadata, not an executable registry.

Use an agent-facing runtime tool when:

- the model must decide when to call the capability
- the tool result should influence the model's next step
- the action is runtime-local, such as filesystem, shell, web search, preview, approval, or interaction
- a backend mutation needs a model-facing wrapper

The model-facing contract is the source of truth for prompts and skills. The internal command, if any, is backend reuse.

### Canonical Names and Legacy MCP Names

Helpin product and interaction tools use bare canonical names. Authored
configuration and stored history can also contain legacy names:

- canonical name: the model-facing and backend name, such as `create_task`, `update_plan`, or `request_user_input`
- legacy MCP name: a stored compatibility form such as `mcp__helpin__create_task`

Use canonical names for:

- runtime profile allowlists
- backend policy and authorization
- tool catalog categories
- artifact extraction and approved-preview application
- test fixtures that are not specifically about provider tool names
- model-facing prompt snippets
- staged skill markdown
- provider tool definitions
- docs that tell an agent which tool to call

Write-side normalization strips the legacy `mcp__helpin__` prefix and maps known
legacy aliases. Provider discovery and calls require exact bare canonical names;
`CallProviderTool` does not accept prefixed aliases. Frontend transcript rendering
strips the prefix while preserving the historical tool identity. Do not create
new prefixed data.

Repo-local backend tools such as filesystem reads, patching, and shell execution may still be provided directly by a runtime. Helpin product and interaction tools should be available through the Helpin MCP bridge for `native_sdk` runs.

### Runtime Exposure

A tool needs an implementation, executable registration, and run exposure:

- `agentcontract/runtime_profiles.go` supplies preset defaults; `ResolveAgentProfile`
  applies per-agent overrides.
- The launch contract carries the selected allowed tools to Agent Runtime. Runtime
  owns execution-time tool filtering and approval policy.
- Helpin's provider catalog includes executable commands with tool metadata.
  The provider validates unique canonical aliases, schemas, and command mappings.
- Host callbacks resolve workspace, agent, run, and actor scope before dispatch.
  `InternalCommandService.Execute` checks actor permissions and mutation guards;
  domain handlers enforce entity membership and product invariants.
- The configuration/UI catalog overlays command metadata onto the embedded
  catalog snapshot. A catalog entry alone does not make a tool executable.

A tool can therefore appear in configuration metadata without being available
for a particular run. Verify both the resolved run contract and provider coverage.

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

### Tool risk and Dock execution

Every mutating tool has a runtime risk classification: `routine_mutation`,
`sensitive_mutation`, or `destructive_mutation`; reads resolve to `read`.
Unclassified mutations default to sensitive. Risk belongs to canonical tool
metadata, not an individual agent prompt or a list of Ask-Agent exceptions.

The `ask_agent` preset uses `approval_mode=risk_based`. Reads and routine,
reversible product mutations execute directly. Sensitive and destructive calls
are intercepted by Agent Runtime before execution, persisted with their exact
input, and executed once after approval. The Helpin command bridge continues to
enforce authentication, actor permissions, workspace/target scope, and domain
invariants, but does not ask for a second approval when the runtime owns the
configured policy.

`prepare_dock_execution` remains as a backward-compatible and explicit grouped
approval mechanism. Agents using `approval_mode=never` retain the legacy host
proposal guard. Repository writes, delivery/release actions, outbound support,
publishing, deletion, and force or bulk destructive operations must remain
sensitive or destructive. Read-only repository checkout, file/search/symbol,
and commit-history tools can be exposed to the Dock directly.

The Dock's delegation decision is capability-driven. `get_my_capabilities`
reads the current run's actual immutable `allowed_tools` input and groups it
into repository reads, product reads/mutations, skills, interactions/web, and
agent orchestration. Prompts should tell the Dock to complete all covered
steps itself and delegate only the smallest step needing an unavailable or
intentionally isolated capability. Paused Dock runs are rotated when their
stored tool set differs from the current scoped preset, so this introspection
does not remain stale across tool or permission changes.

Legacy Dock runs without runtime-owned approval still receive structured
`dock_execution_approval_required` recovery errors. New risk-based runs must
not use that failure-first flow: runtime approval happens before tool execution.

Long-lived Dock runs keep `target_type=workspace` even when the user selects a
document, task, or CRM record as page context. Product tools that expose an
explicit `*_id` field must accept the workspace run target when safe, treat the
explicit ID as canonical, and verify that entity belongs to the command
workspace. Page selection must not require recreating or retargeting the run.

### Optional skill access

Each agent version owns one complete `system_prompt`. Product prompt modules
may be used internally to generate a default version, but they are not exposed
as attached skills and are not runtime dependencies. Approval and completion
requirements are carried separately as structured runtime policy. Optional
skills use these runtime-owned tools on `native_sdk` runs:

- `find_skills {"query"?: string, "limit"?: integer}` lists or searches metadata for the current agent's optional skills.
- `read_skill {"key"?: string, "skill_id"?: string, "path"?: string, "max_bytes"?: integer}` reads one selected package. Exactly one of `key` or `skill_id` is required; `path` defaults to `SKILL.md` and must remain inside the package.

Runtime-local staged skill package tools remain namespaced and separate from
the canonical optional-skill catalog tools above.

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
+---------+      +-------------------+-------------------+      | Runtime / service |
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
|   +-- preview / publish tools, for example publish_task_plan
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

That means a good tool contract is not just the schema in command metadata.
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
  -> canonical snake_case action
  -> runtime MCP name when a Helpin product tool is model-facing

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

Use `snake_case` action names for canonical aliases:

- `read_files`
- `list_documents`
- `update_task_state`
- `request_user_input`

For model-facing Helpin product or interaction tools, use the same bare
canonical name:

- `list_documents`
- `update_task_state`
- `request_user_input`

Do not save a prefixed name in backend allowlists, policies, prompts, or skill
contracts unless the code is specifically testing legacy compatibility.

### Field names

Use consistent names across PM, Docs, CRM, and Support:

- IDs end in `_id`: `task_id`, `document_id`, `deal_id`
- lists use plural nouns: `tasks`, `dependencies`, `questions`
- booleans read like booleans: `replace`, `is_internal`
- free-form text is usually `content`, `summary`, `description`, or `note`
- filters usually use `limit`, `query`, `status`, `space_id`, `stage_id`

Avoid mixed naming like `taskId`, `docId`, or vague fields like `data` unless there is a strong reason.

## Implementation Pattern

For a Helpin product tool:

1. Add or update the canonical alias and schema in command metadata.
2. Register an `InternalCommandDefinition` with its permissions, mutation flag,
   tool metadata, and execution function.
3. Implement the domain action in the corresponding service file. Decode typed
   input, apply defaults, check workspace/entity scope, and enforce invariants.
4. Return structured output and add tests for successful execution and rejected
   inputs or unauthorized scope.
5. Verify provider discovery and the relevant agent's resolved allowlist.

The execution function uses the current service signature:

```go
Execute func(ctx context.Context, meta model.InternalCommandContext,
    input json.RawMessage) (json.RawMessage, error)
```

Use [existing command definitions](../server/internal/service/internal_command_service.go)
and [domain handlers](../server/internal/service/internal_command_docs_metadata.go)
as implementation examples. Authorization checks and context binding must come
from the execution boundary and service, not from model-supplied IDs alone.

Runtime-local tools are implemented and tested in Agent Runtime. Updating Helpin's
metadata does not install an executor implementation.

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
- legacy names such as `request_human_input`, `request_human_approval`, and
  `mcp__helpin__request_user_input` are normalized on the write side; provider
  calls require the canonical name, not the historical alias

## Adding New Capabilities

Start by choosing the right shape. Do not add both a command and a runtime tool unless both contracts are needed.

### Add a System/Internal Command

Use this path for backend-owned product actions that may be reused outside a model runtime.

1. Register the domain-scoped command in `InternalCommandService`, for example
   `pm.create_task_batch` or `docs.write_document_content`.
2. Put validation, permission requirements, and product invariants in the service.
3. Add service/repository tests for the action and authorization boundaries.
4. Add runtime tool metadata only if the model needs to call the command.

### Add a Runtime-Only Agent Tool

Use this path for model-facing capabilities that are local to the agent runtime and do not need a reusable backend command.

Examples:

- filesystem and code-search tools
- `run_command`
- web search
- previews and approval/interaction tools
- read-only context tools

Implement and test this capability in the separate Agent Runtime repository.
Update Helpin's catalog/profile metadata only when the host needs to configure
or expose it. Verify the selected Runtime release supports the capability, then
exercise the integration with Helpin. Do not recreate a worker-side executor.

### Add a Command-Backed Agent Tool

Use this path when the model should call a business mutation, but the mutation must remain reusable and invariant-safe outside the model runtime.

1. Add the canonical alias, schema, category, and risk classification in
   `server/internal/commandtools/metadata.go`.
2. Register the backing command and implement its domain action in service code.
3. Verify provider discovery lists an executable command for that alias. Missing
   handlers, duplicate aliases, and noncanonical names fail provider validation.
4. Add tests for the schema, actor permissions, workspace/target scope, successful
   execution, and domain validation. Use the existing `internal_command_*_test.go`
   and `agent_runtime_mcp_test.go` suites as references.
5. Update the appropriate profile or per-agent allowlist and model-facing guidance.
   Keep the canonical tool alias distinct from the internal command name.

### Add Planner-Specific Tool Behavior

Planner tools can change approval, preview, and completion behavior. Runtime
approval may occur before execution; product validation still runs in Helpin.

Steps:

1. Update the tool schema and decode/validation logic.
2. Update this tool contract reference with any model-facing schema or sequence changes.
3. Update active skill instructions if the model must use a new sequence.
4. Update completion or approval-preview policy tests if the tool affects handoff requirements.
5. Verify the projected run contains the intended structured approval/completion
   policy and allowed tools; optional skills must not replace those policies.

Keep planner-specific payload details in this document unless a separate
runtime-owned reference is introduced outside `docs/plans`.

## Quick Decision Rules

Use these defaults:

- if the tool acts on local repo state, keep it runtime-only
- if the tool is human-approval or preview UX, keep it runtime-only
- if the tool is a reusable product mutation, keep the tool contract stable and add or reuse internal-command backing
- if the output will be reused by later reasoning, return JSON
- if a second payload shape is not necessary, do not add one
