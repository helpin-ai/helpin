# Native Provider Runtime Migration Plan

## Status

Draft plan for the `native_sdk` runtime only.

This plan replaces the broader rewrite posture from [docs/plans/2026-03-23-generic-interactive-runtime-and-responses-plan.md](/root/teampulse/docs/plans/2026-03-23-generic-interactive-runtime-and-responses-plan.md) with a narrower sequence:

1. characterize the current provider/runtime behavior precisely
2. harden the tool and persistence layer around it
3. only then decide whether any Eino transport replacement is still necessary

## Goal

Strengthen the in-process `native_sdk` runtime without replacing Eino in this phase.

The target outcome is a runtime that is:

- safer when reading and editing repositories
- cheaper in token usage during long planner/support runs
- easier to resume and debug
- explicit about per-provider behavior and limitations

## Scope

In scope:

- `native_sdk` execution for planner, story-planner, CRM operator, and support-style runs
- provider behavior across Anthropic, OpenAI, and OpenRouter
- repo tool safety and prompt discipline
- transcript, artifact, and continuation durability

Out of scope for this phase:

- replacing `ExecuteWithEino`
- removing Eino from the backend
- changing `opencode` story execution
- making provider-side state the source of truth

## Current Code-Backed Findings

### 1. The provider split already exists

The current runtime is not one generic path:

- Anthropic uses `einoclaude.NewChatModel` through the chat-model path in [server/internal/worker/eino_exec.go](/root/teampulse/server/internal/worker/eino_exec.go)
- OpenAI and OpenRouter use `agenticopenai.New` through the agentic Responses-style path in [server/internal/worker/eino_exec.go](/root/teampulse/server/internal/worker/eino_exec.go)

This means the most important near-term problem is not introducing a second transport abstraction. It is verifying and hardening the behavior of the split that already exists.

### 2. `native_sdk` is the interactive runtime

The runtime registry wires `native_sdk` as the only in-process SDK-backed adapter in [server/internal/worker/runtime_factory.go](/root/teampulse/server/internal/worker/runtime_factory.go).

Today it is used by:

- epic planner
- story planner
- CRM operator
- support agent

That mapping comes from [server/internal/worker/runtime_profiles.go](/root/teampulse/server/internal/worker/runtime_profiles.go) and [server/internal/service/agent_presets.go](/root/teampulse/server/internal/service/agent_presets.go).

### 3. Continuation support is already partial and provider-specific

`ProviderSupportsResponseContinuation` currently returns true only for OpenAI in [server/internal/worker/eino_exec.go](/root/teampulse/server/internal/worker/eino_exec.go).

That means:

- OpenAI continuation is an implemented path that needs characterization and tests
- OpenRouter is currently Responses-style but not treated as continuation-capable
- Anthropic remains on the chat path and should be treated separately

### 4. The repo tool layer is still too permissive

Current filesystem tools in [server/internal/worker/tools_fs.go](/root/teampulse/server/internal/worker/tools_fs.go):

- allow blind `write_file`
- do not require a prior read before mutation
- do not track last-read mod-times
- do not provide structured search/replace edit semantics
- do not provide an atomic patch tool

This is the highest-risk gap if repo-aware planner or support flows expand further.

### 5. Prompt discipline is still broad, not search-first

The prompt layer in [server/internal/worker/prompt.go](/root/teampulse/server/internal/worker/prompt.go) tells the model to use tools, but it does not strongly enforce:

- search before broad reads
- ranged reads before whole-file reads
- transcript summarization once interactive history grows
- artifact-context trimming when the same payload is repeatedly replayed

### 6. Persistence is useful but still lossy

The current Temporal activity layer already does important work:

- loads prior conversation history into `ExecutionContext`
- loads artifact context into the prompt
- persists assistant turns and provider checkpoints

Relevant files:

- [server/internal/temporalapp/activities.go](/root/teampulse/server/internal/temporalapp/activities.go)
- [server/internal/model/agent_run_message.go](/root/teampulse/server/internal/model/agent_run_message.go)
- [server/internal/worker/context.go](/root/teampulse/server/internal/worker/context.go)

But the persisted shape still has weaknesses:

- only the final round of tool-result messages is persisted as standalone `tool_result` run messages
- preview side effects and repo-scan summaries are not normalized as first-class execution artifacts
- `content_blocks` is present but not yet treated as a complete debugging surface for streamed execution

## Migration Principles

1. Temporal and Teampulse-owned tables remain the source of truth.
2. Provider continuation is transport state, not business state.
3. We should not replace Eino until we can point to measured failures that survive tool and persistence hardening.
4. Planner and support agents should get safer tools and cheaper context before they get a new transport layer.

## Workstreams

## Workstream 1: Characterize Current Provider Behavior

### Objective

Build an explicit, test-backed understanding of what the current runtime already does per provider.

### Why first

Without this, any transport rewrite is guesswork. The code already contains provider-specific branches; we need to know which of those branches are actually failing in practice.

### Tasks

1. Document the current execution path for:
   - Anthropic chat path
   - OpenAI agentic Responses path
   - OpenRouter agentic Responses path
2. Add focused tests for:
   - assistant text streaming
   - tool-call emission and tool-result loops
   - continuation checkpoint extraction
   - pause/resume behavior with prior conversation history
3. Record which token-usage fields are reliably populated per provider.
4. Add structured logs around provider selection, model selection, tool rounds, and continuation loading/saving.

### Primary files

- [server/internal/worker/eino_exec.go](/root/teampulse/server/internal/worker/eino_exec.go)
- [server/internal/worker/eino_executor.go](/root/teampulse/server/internal/worker/eino_executor.go)
- [server/internal/temporalapp/activities.go](/root/teampulse/server/internal/temporalapp/activities.go)
- new tests under `/root/teampulse/server/internal/worker`

### Exit criteria

- a provider matrix exists with observed behavior, not assumptions
- tests fail if continuation or tool-call behavior regresses
- log output is sufficient to explain a failed native run without re-running blindly

## Workstream 2: Harden Repo Mutation Semantics

### Objective

Bring the native runtime’s generic repo tools closer to the defensive behavior already proven in OpenCode-style tooling.

### Tasks

1. Add per-run file read tracking.
2. Record last-read mod-time for files read through runtime tools.
3. Reject writes when a file changed after the last observed read.
4. Add a search/replace edit tool with:
   - explicit unique-match behavior
   - structured mismatch errors
   - line-aware repair guidance in the error text
5. Add an `apply_patch`-style tool for atomic multi-hunk edits.
6. Return diff-oriented success and failure payloads so the model can repair its own edit attempts.

### Primary files

- [server/internal/worker/tools.go](/root/teampulse/server/internal/worker/tools.go)
- [server/internal/worker/tools_fs.go](/root/teampulse/server/internal/worker/tools_fs.go)
- new file `/root/teampulse/server/internal/worker/tool_file_state.go`
- new file `/root/teampulse/server/internal/worker/tools_edit_patch.go`

### Exit criteria

- repo writes require a coherent read-then-write flow
- conflicting edits are rejected with actionable errors
- the native runtime can perform safe document/repo mutations without relying on blind overwrite semantics

## Workstream 3: Reduce Token Cost for Repo Comprehension

### Objective

Make planner and support runs more disciplined about how they inspect code and reuse context.

### Tasks

1. Tighten system-prompt rules so the model prefers:
   - `list_directory`
   - `ripgrep`
   - `search_files`
   - `list_symbols`
   - `read_file_range`
   before `read_file`
2. Improve `read_file_range` ergonomics and response formatting so it becomes the default read path for large files.
3. Add truncation markers and optional lightweight file summaries for large file reads.
4. Trim duplicated artifact context before each execution round.
5. Add transcript summarization checkpoints for long interactive sessions.

### Primary files

- [server/internal/worker/prompt.go](/root/teampulse/server/internal/worker/prompt.go)
- [server/internal/worker/tools_fs.go](/root/teampulse/server/internal/worker/tools_fs.go)
- [server/internal/worker/context.go](/root/teampulse/server/internal/worker/context.go)
- [server/internal/temporalapp/activities.go](/root/teampulse/server/internal/temporalapp/activities.go)
- new file `/root/teampulse/server/internal/worker/transcript_summarizer.go`

### Exit criteria

- large full-file reads drop materially in planner/support traces
- long interactive runs stop growing linearly in prompt size
- summarized runs still resume correctly from durable state

## Workstream 4: Normalize Message, Artifact, and Checkpoint Persistence

### Objective

Make stored native run history easier to inspect, replay, and debug.

### Tasks

1. Review how `ExecutionBlock`, `ExecutionMessage`, and `AgentRunMessage.content_blocks` are populated today.
2. Normalize persistence for:
   - assistant text blocks
   - tool calls
   - tool results
   - preview side effects
   - transcript summary artifacts
3. Keep provider checkpoints attached to the assistant turn that created them.
4. Add explicit artifact types for:
   - repo scan summaries
   - prompt summaries
   - transcript summaries
5. Ensure resumed runs can reconstruct a compact, trustworthy context from durable state without relying on raw assistant prose alone.

### Primary files

- [server/internal/worker/eino_exec.go](/root/teampulse/server/internal/worker/eino_exec.go)
- [server/internal/temporalapp/activities.go](/root/teampulse/server/internal/temporalapp/activities.go)
- [server/internal/model/agent_run_message.go](/root/teampulse/server/internal/model/agent_run_message.go)
- [server/internal/model/agent_runtime_checkpoint.go](/root/teampulse/server/internal/model/agent_runtime_checkpoint.go)

### Exit criteria

- native run history is understandable from persisted data alone
- checkpoint replay is tied to the correct assistant sequence number
- support and planner resume logic becomes less fragile

## Workstream 5: Publish a Provider Capability Matrix

### Objective

Turn current runtime behavior into an explicit support contract.

### Tasks

1. Define capabilities to track:
   - `stream_text`
   - `stream_tool_calls`
   - `tool_result_roundtrip`
   - `continuation`
   - `pause_resume`
   - `json_output`
   - `token_usage_reporting`
   - `prompt_caching`
2. Record actual observed status for Anthropic, OpenAI, and OpenRouter.
3. Gate runtime assumptions and prompts based on measured capabilities.
4. Log when the runtime is forced onto a degraded path because a capability is partial or missing.

### Primary files

- new file `/root/teampulse/docs/plans/provider-capability-matrix.md`
- [server/internal/worker/eino_exec.go](/root/teampulse/server/internal/worker/eino_exec.go)
- [server/internal/worker/runtime_factory.go](/root/teampulse/server/internal/worker/runtime_factory.go)

### Exit criteria

- supported behavior is explicit per provider
- product and engineering decisions can reference a stable matrix instead of anecdotal failures

## Workstream 6: Re-evaluate the Need for Transport Replacement

### Objective

Decide whether Eino should actually be replaced, based on measured residual problems.

### Only start this after Workstreams 1-5

Questions to answer:

- Are OpenAI or OpenRouter tool-call loops still materially broken?
- Is continuation still unreliable after checkpoint and persistence cleanup?
- Is Eino still preventing the event fidelity or control flow the app needs?
- Are any remaining gaps transport-level, or are they prompt/tool/persistence-level?

### If replacement is still justified

The follow-up should be a narrow transport change, not another broad runtime rewrite. Expected target shape:

- keep `ExecutionContext`, artifacts, and Temporal orchestration app-owned
- replace only the provider execution adapter that is still failing
- keep the provider capability matrix and tests as the acceptance contract for the replacement

## Recommended Order

1. Characterize current provider behavior and add logs/tests.
2. Harden repo mutation semantics.
3. Tighten prompt guidance and ranged-read behavior.
4. Add transcript summarization and artifact-context trimming.
5. Normalize message and checkpoint persistence.
6. Publish the provider capability matrix.
7. Decide whether transport replacement is still warranted.

## Immediate Execution Plan

### PR 1: characterization and observability

- add tests around `ExecuteWithEino`
- add structured logs for provider/model/tool-round/checkpoint events
- document first-pass capability findings

### PR 2: safe repo edits

- add read tracking and mod-time validation
- add structured search/replace edit support
- add `apply_patch`-style edits

### PR 3: prompt and transcript cost control

- tighten search-first instructions
- improve ranged reads and truncation behavior
- add transcript summarization checkpoints

### PR 4: persistence normalization

- improve `content_blocks` and tool-result persistence
- add explicit artifacts for summaries and repo scans
- verify resume reconstruction from durable state

## Re-evaluation Gate

Do not replace Eino unless at least one of the following is still true after the earlier workstreams land:

- OpenAI or OpenRouter still fails core tool-call or continuation scenarios in a reproducible way
- native run persistence is still too lossy to support reliable resume/debugging
- provider-specific behavior still forces unstable, ad hoc logic in the activity layer

If none of those remain true, the right outcome is to keep Eino and ship the hardened runtime.
