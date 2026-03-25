# Codex + OpenCode Coder Runtime Plan

## Status

Draft plan for introducing Codex as a first-class coder runtime alongside OpenCode for the existing coder and reviewer presets.

This plan assumes:

- `code_builder` remains the coder preset
- `review_agent` remains the reviewer preset
- Codex is introduced as a new `runtime_kind`
- runtime choice stays separate from preset/tool policy

## Goal

Allow coder-style agents to run on either:

- `opencode`
- `codex`

without introducing a separate Codex-specific preset and without duplicating the policy surface for coder tools, approvals, or target types.

## Non-Goals

This phase does not aim to:

- replace OpenCode as the default coder runtime immediately
- add Codex to planner or support presets
- make a single run switch between OpenCode and Codex
- redesign the broader agent preset model
- solve every artifact naming inconsistency in one pass beyond what Codex needs

## Current State

### Preset defaults

- `code_builder` defaults to `opencode`
- `review_agent` defaults to `opencode`
- planner/support presets default to `native_sdk`

Primary files:

- [server/internal/worker/runtime_profiles.go](/root/teampulse/server/internal/worker/runtime_profiles.go)
- [server/internal/service/agent_policy.go](/root/teampulse/server/internal/service/agent_policy.go)
- [frontend/src/pages/pm/Agents.tsx](/root/teampulse/frontend/src/pages/pm/Agents.tsx)

### Runtime registration

The runtime registry currently exposes two adapters:

- `opencode`
- `native_sdk`

Primary file:

- [server/internal/worker/runtime_factory.go](/root/teampulse/server/internal/worker/runtime_factory.go)

### Queue routing

Queue mapping is runtime-based:

- `native_sdk` -> native queues
- `opencode` -> `agent-opencode-autonomous`

Primary files:

- [server/internal/worker/resolve.go](/root/teampulse/server/internal/worker/resolve.go)
- [server/internal/temporalapp/queues.go](/root/teampulse/server/internal/temporalapp/queues.go)

### Runtime-specific cleanup still exists

Some reconciliation and UI logic still assumes OpenCode-specific stages and artifacts:

- stuck post-run reconciliation checks `run.RuntimeKind == "opencode"`
- post-run stages include `opencode_finished`
- frontend artifact labels and transcript handling know about `opencode_stdout`, `opencode_stderr`, and related chunk artifacts

Primary files:

- [server/internal/service/agent.go](/root/teampulse/server/internal/service/agent.go)
- [frontend/src/components/pm/AgentRunDrawer.tsx](/root/teampulse/frontend/src/components/pm/AgentRunDrawer.tsx)
- [frontend/src/components/pm/AgentRunDetail.tsx](/root/teampulse/frontend/src/components/pm/AgentRunDetail.tsx)
- [frontend/src/components/pm/AgentRunArtifactView.tsx](/root/teampulse/frontend/src/components/pm/AgentRunArtifactView.tsx)

## Design Principles

1. Runtime is execution infrastructure, not agent identity.
2. Preset policy defines tools, commands, targets, and approvals.
3. Runtime choice should be per-agent configuration, not a new preset explosion.
4. Codex support should be added without regressing OpenCode behavior.
5. Runtime-specific assumptions should be isolated behind adapters or generic runtime abstractions.
6. Rollout should be incremental and reversible.

## Target Model

### 1. Add a new runtime kind

Introduce:

- `codex`

in the same places that already support:

- `opencode`
- `native_sdk`

This becomes a first-class `Agent.RuntimeKind` and `AgentRun.RuntimeKind` value.

### 2. Keep coder presets stable

Do not add:

- `codex_coder`
- `codex_reviewer`

Instead:

- `code_builder` supports `opencode` and `codex`
- `review_agent` supports `opencode` and `codex`
- planner/support remain constrained to their existing runtimes unless explicitly expanded later

### 3. Keep one policy surface per preset

`code_builder` should keep one canonical policy for:

- allowed tools
- allowed commands
- target types
- approval requirements

The runtime changes only how execution happens, not what the agent is allowed to do.

### 4. Move toward generic runtime artifacts

Longer term, runtime-produced logs and stages should stop hardcoding the runtime name into the contract.

Preferred direction:

- generic artifact families such as `runtime_stdout`, `runtime_stderr`, `runtime_config`
- runtime-specific metadata like `runtime_kind=opencode` or `runtime_kind=codex`

If that is too large for the first Codex rollout, add Codex with parallel runtime-specific artifacts first, then normalize in a follow-up.

## Implementation Workstreams

## Workstream 1: Extend RuntimeKind Across the Model and API

### Objective

Make `codex` a valid runtime end-to-end in backend and frontend types.

### Tasks

1. Add `codex` to backend runtime validation.
2. Add `codex` to frontend `AgentRuntimeKind`.
3. Add a display label for Codex in the UI.
4. Ensure agent create/update payloads accept and round-trip `codex`.

### Primary files

- [server/internal/service/agent.go](/root/teampulse/server/internal/service/agent.go)
- [server/internal/model/agent.go](/root/teampulse/server/internal/model/agent.go)
- [frontend/src/lib/pmTypes.ts](/root/teampulse/frontend/src/lib/pmTypes.ts)
- [frontend/src/lib/agentRuntime.ts](/root/teampulse/frontend/src/lib/agentRuntime.ts)

### Exit criteria

- agents can be created and updated with `runtime_kind="codex"`
- runs can persist `runtime_kind="codex"`
- the frontend renders Codex as a supported runtime value

## Workstream 2: Introduce a Codex Runtime Adapter

### Objective

Implement a `CodexExecutor` that is a peer to `OpenCodeExecutor`, not a special mode inside it.

### Tasks

1. Add a new runtime adapter, likely:
   - `/root/teampulse/server/internal/worker/codex.go`
   - optional helpers/tests alongside it
2. Decide the execution boundary:
   - shell out to a Codex CLI, or
   - wrap an SDK/provider flow with repo-aware execution support
3. Match the runtime adapter interface already used by the registry.
4. Emit runtime artifacts, stage heartbeats, and final responses in the same shape expected by the run engine.

### Primary files

- [server/internal/worker/runtime_factory.go](/root/teampulse/server/internal/worker/runtime_factory.go)
- [server/internal/worker/opencode.go](/root/teampulse/server/internal/worker/opencode.go)
- [server/internal/worker/eino_executor.go](/root/teampulse/server/internal/worker/eino_executor.go)
- new file(s): `/root/teampulse/server/internal/worker/codex*.go`

### Design notes

- Do not overload `OpenCodeExecutor` to branch on provider/model and pretend that is Codex support.
- Codex should have its own adapter identity and runtime kind.
- If Codex needs different config or command assembly, keep that isolated in the Codex adapter.

### Exit criteria

- runtime registry can execute `codex` runs
- Codex execution produces heartbeats, artifacts, and a final result compatible with current run processing

## Workstream 3: Add Queue Routing for Codex

### Objective

Route Codex runs through their own shared queue so operations and capacity can be managed independently from OpenCode.

### Tasks

1. Add a queue constant for Codex, for example:
   - `agent-codex-autonomous`
2. Update queue resolution to map `runtime_kind="codex"` correctly.
3. Ensure the Temporal worker registers that queue.

### Primary files

- [server/internal/worker/resolve.go](/root/teampulse/server/internal/worker/resolve.go)
- [server/internal/temporalapp/queues.go](/root/teampulse/server/internal/temporalapp/queues.go)
- worker bootstrap/wiring under `cmd/temporal-worker`

### Exit criteria

- Codex runs enqueue onto a dedicated queue
- a Temporal worker can poll and execute that queue

## Workstream 4: Allow Codex on Coder/Reviewer Presets

### Objective

Expose Codex as a supported runtime choice for code-oriented presets only.

### Tasks

1. Add preset-to-runtime compatibility rules:
   - `code_builder` -> `opencode`, `codex`
   - `review_agent` -> `opencode`, `codex`
   - `epic_planner` / `story_planner` / `support_agent` remain constrained
2. Keep default runtime for coder preset as `opencode` initially.
3. Update preset fallback metadata in the frontend.
4. Update any validation tests that currently assume only `opencode` or `native_sdk`.

### Primary files

- [server/internal/service/agent_policy.go](/root/teampulse/server/internal/service/agent_policy.go)
- [server/internal/worker/runtime_profiles.go](/root/teampulse/server/internal/worker/runtime_profiles.go)
- [frontend/src/pages/pm/Agents.tsx](/root/teampulse/frontend/src/pages/pm/Agents.tsx)

### Exit criteria

- UI only offers Codex where intended
- backend accepts Codex for coder/reviewer and rejects it where unsupported

## Workstream 5: Decouple Runtime-Specific Stages and Reconciliation

### Objective

Remove OpenCode-only assumptions that would otherwise make Codex brittle or invisible to stale-run reconciliation.

### Tasks

1. Replace or generalize `opencode`-specific post-run stage logic.
2. Decide whether Codex should share the same generic “post-run” states or have its own runtime-specific states.
3. Update stale-run reconciliation to work by runtime capability or generic stage family, not by `run.RuntimeKind == "opencode"` only.

### Primary files

- [server/internal/service/agent.go](/root/teampulse/server/internal/service/agent.go)

### Recommended direction

Prefer generic stages like:

- `runtime_starting`
- `runtime_running`
- `runtime_finished`
- `persisting_changes`
- `pushing_changes`
- `finalizing`

instead of runtime-branded stage names.

### Exit criteria

- Codex runs cannot get stranded because cleanup/reconciliation only knows about OpenCode

## Workstream 6: Normalize Runtime Artifacts in the UI

### Objective

Make the run drawer and run detail surfaces handle Codex output cleanly.

### Tasks

1. Audit all OpenCode-specific artifact assumptions in the frontend.
2. Decide between:
   - introducing parallel `codex_*` artifact types now, or
   - generalizing immediately to runtime-agnostic artifact families
3. Update run detail views, live transcript sections, and artifact labels accordingly.

### Primary files

- [frontend/src/components/pm/AgentRunDrawer.tsx](/root/teampulse/frontend/src/components/pm/AgentRunDrawer.tsx)
- [frontend/src/components/pm/AgentRunDetail.tsx](/root/teampulse/frontend/src/components/pm/AgentRunDetail.tsx)
- [frontend/src/components/pm/AgentRunArtifactView.tsx](/root/teampulse/frontend/src/components/pm/AgentRunArtifactView.tsx)
- [frontend/src/components/pm/agentRunConstants.ts](/root/teampulse/frontend/src/components/pm/agentRunConstants.ts)

### Recommended direction

If scope allows, normalize to generic artifact groups now. That avoids doing the same integration twice.

### Exit criteria

- Codex run output is visible and understandable in the UI
- Codex does not appear as a broken or artifact-less runtime

## Workstream 7: Add Codex Runtime Configuration

### Objective

Make Codex runtime configuration explicit and operationally clear.

### Tasks

1. Add config keys for Codex path/model/base URL as needed.
2. Keep Codex config separate from OpenCode config.
3. Document environment variables and worker requirements.

### Candidate config

- `CODEX_PATH`
- `CODEX_MODEL`
- `CODEX_BASE_URL`
- existing provider keys such as `OPENAI_API_KEY` if Codex depends on them

### Primary files

- [server/internal/config/config.go](/root/teampulse/server/internal/config/config.go)
- relevant bootstrap/docs files

### Exit criteria

- operators can configure Codex without repurposing OpenCode settings

## Workstream 8: Add Tests for Runtime Selection and Execution

### Objective

Protect the runtime expansion with coverage at the preset, queue, and execution boundaries.

### Tests to add or update

1. Runtime validation:
   - `codex` accepted
   - unsupported runtimes still rejected
2. Queue resolution:
   - `codex` maps to Codex queue
3. Policy compatibility:
   - coder/reviewer accept Codex
   - planner/support reject Codex if that remains the policy
4. Frontend runtime typing/rendering:
   - Codex label renders
   - Agents modal supports Codex for coder presets only
5. Runtime registry:
   - Codex adapter is registered and resolvable
6. Stale-run reconciliation:
   - Codex runs get the intended post-run handling

### Primary files

- [server/internal/service/agent_policy_test.go](/root/teampulse/server/internal/service/agent_policy_test.go)
- [server/internal/worker/resolve_test.go](/root/teampulse/server/internal/worker/resolve_test.go)
- new Codex runtime tests
- relevant frontend tests under `frontend/src`

## Rollout Plan

## Phase 1: Backend support, default off

Ship:

- `codex` runtime kind
- runtime registry support
- queue mapping
- backend validation

Do not change preset defaults yet.

## Phase 2: UI support for coder/reviewer

Ship:

- Agents modal runtime selection for Codex
- labels and type support
- backend policy enforcement

Default coder runtime remains `opencode`.

## Phase 3: Internal validation

Create one internal coder agent on Codex and validate:

- story run creation
- repo checkout / file editing
- command execution
- artifact capture
- completion / failure handling
- delivery/push/finalization path

## Phase 4: Broader rollout

After validation:

- optionally allow Codex for reviewer preset in production
- optionally make Codex selectable for more users/workspaces
- decide later whether `code_builder` default should move from OpenCode to Codex

## Risks

### 1. Runtime-specific artifact drift

If Codex is added with its own artifact names and the UI is still OpenCode-specific, the run experience will be incomplete or broken.

### 2. Queue and worker mismatch

If the queue is added but not polled by a worker, Codex runs will sit queued indefinitely.

### 3. Preset/runtime policy drift

If the UI allows Codex but backend policy rejects it, agent updates and runs will fail inconsistently.

### 4. Reconciliation blind spots

If stale-run reconciliation only understands OpenCode stages, failed Codex runs may leak in `running` state.

## Recommended Implementation Order

1. Add `codex` to model/API/frontend runtime enums.
2. Implement and register the Codex adapter.
3. Add queue routing and worker registration.
4. Add backend policy compatibility for coder/reviewer.
5. Add UI selection for coder/reviewer.
6. Generalize or extend runtime artifact/stage handling.
7. Add tests and run an internal validation pass.

## Success Criteria

- a `code_builder` agent can be configured with `runtime_kind="codex"`
- the run resolves to the Codex queue and executes successfully
- coder tool policy is unchanged across OpenCode and Codex
- frontend surfaces Codex cleanly in agent settings and run detail views
- OpenCode behavior remains unchanged for existing agents
