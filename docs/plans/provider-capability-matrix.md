# Provider Capability Matrix

## Purpose

This matrix records what the current `native_sdk` runtime is wired to support today.

Status meanings:

- `wired`: the code path exists
- `partial`: some support exists, but the behavior is limited or provider-specific
- `not wired`: no active implementation path
- `needs characterization`: code exists, but behavior still needs test-backed validation against live providers

This is intentionally code-backed first. It should be updated as characterization tests and live validation work land.

## Current Matrix

| Capability | Anthropic | OpenAI | OpenRouter |
|---|---|---|---|
| runtime path | `wired` chat-model path | `wired` agentic Responses path | `wired` agentic Responses path |
| `stream_text` | `wired` via `Stream` | `partial` assistant completion event exists, but current path uses `Generate` not streaming | `partial` assistant completion event exists, but current path uses `Generate` not streaming |
| `stream_tool_calls` | `partial` tool-call events emitted after assistant message stream completes | `not wired` as incremental provider streaming; tool calls are processed from final agentic message | `not wired` as incremental provider streaming; tool calls are processed from final agentic message |
| `tool_result_roundtrip` | `wired` | `wired` | `wired` |
| `continuation` | `not wired` | `wired` | `partial` Responses-style runtime exists, but `ProviderSupportsResponseContinuation` is false today |
| `pause_resume` | `wired` through app transcript + Temporal | `wired` through app transcript + Temporal, with provider continuation checkpoint support | `wired` through app transcript + Temporal, without continuation checkpoint reuse |
| `json_output` | `wired` at prompt/response contract level | `wired` at prompt/response contract level | `wired` at prompt/response contract level |
| `token_usage_reporting` | `wired` from `ResponseMeta.Usage` | `wired` from `ResponseMeta.TokenUsage` | `wired` from `ResponseMeta.TokenUsage` |
| `prompt_caching` | `not wired` in current runtime layer | `not wired` in current runtime layer | `not wired` in current runtime layer |

## Code References

- provider routing: [server/internal/worker/eino_exec.go](/root/teampulse/server/internal/worker/eino_exec.go)
- runtime adapter wiring: [server/internal/worker/runtime_factory.go](/root/teampulse/server/internal/worker/runtime_factory.go)
- native executor entrypoint: [server/internal/worker/eino_executor.go](/root/teampulse/server/internal/worker/eino_executor.go)
- checkpoint load/save: [server/internal/temporalapp/activities.go](/root/teampulse/server/internal/temporalapp/activities.go)

## Current Notes

- OpenAI is the only provider currently marked continuation-capable by `ProviderSupportsResponseContinuation`.
- OpenRouter uses the same agentic model family as OpenAI, but the runtime currently treats it as non-continuation-capable.
- The native agentic path emits assistant start/completed events, but it does not currently stream incremental text/tool-call deltas from provider responses.
- Pause/resume is primarily app-owned through persisted run messages, artifacts, and Temporal workflow state. Provider continuation is an optimization on top of that, not the source of truth.

## Next Update Criteria

Update this matrix after:

1. characterization tests cover provider routing, message shaping, and continuation handling
2. structured runtime logs confirm actual provider/model/checkpoint behavior in shared runs
3. any live-provider validation reveals behavior that differs from the code-backed assumptions above
