# agent-runtime audit

Scope: `/root/agent-runtime` (engine), `/root/agent-runtime-go` and `/root/agent-runtime-python` (SDKs), and how `/root/helpin` consumes them.

---

## 1. Purpose, scope, executors, adapters, model providers

### Purpose

`/root/agent-runtime/README.md:1-3` — a **host-neutral execution engine for host-owned AI agents**. It intentionally owns no product concepts. Host applications plug in by registering app-scoped agents, targets, context providers, MCP tools, workspace providers, and optional tool packs. It never creates its own agents: the host registers them via `PUT /v1/agents`, and a missing registration surfaces to the host as `502 {"detail":"agent not found"}`.

It exposes an internal HTTP API on `:8090` (`/root/agent-runtime/internal/api/server.go`) covering agents, runs, messages, artifacts, interactions, and run controls. Versioned contract at `/root/agent-runtime/docs/openapi.yaml`; integration contracts at `/root/agent-runtime/docs/interfaces.md`.

### Executors — two

| Executor | Location | Enablement |
| --- | --- | --- |
| Lightweight in-process | `/root/agent-runtime/internal/engine/engine.go` | Default |
| Durable Temporal | `/root/agent-runtime/internal/durable/{workflow,activities,engine,worker,queues}.go` + `/root/agent-runtime/cmd/agent-runtime-worker` | Only when `TEMPORAL_ADDRESS` is set |

`openDurableExecutor` at `/root/agent-runtime/cmd/agent-runtime/main.go:307-320` returns a nil executor when `TEMPORAL_ADDRESS` is empty. **Temporal is optional, not required.**

Supporting event machinery: `/root/agent-runtime/internal/engine/event_broker.go`, `persisted_event_sink.go`, `nats_event_bridge.go`, `v2_events.go`.

### Runtime adapters — three

`/capabilities` reports `RuntimeKinds: []string{"native_sdk", "codex", "opencode"}` at `/root/agent-runtime/cmd/agent-runtime/main.go:266`.

1. **Native SDK (Eino)** — `/root/agent-runtime/internal/runtime/native*.go` (~25 files). Model layer in `native_eino_provider.go`, `native_eino_model.go`, `native_eino_agentic.go`, `native_eino_schema.go`. Includes built-in human input/approval interaction tools (`native_interaction.go`), context management/compaction (`native_context.go`, `native_checkpoint.go`), and recovery/resume (`native_recovery.go`, `native_resume.go`, `native_reconcile.go`).
2. **Codex** — `/root/agent-runtime/internal/runtime/codex*.go` (~18 files). Two paths: a command-wrapper path and an app-server protocol path (`codex_appserver_client.go`, `codex_appserver_protocol.go`). Has its own auth manager (`codex_auth_manager.go`) and dynamic tool bridge (`codex_dynamic_tools.go`).
3. **OpenCode** — `/root/agent-runtime/internal/runtime/opencode.go`, `opencode_mcp.go`.

Fallback behavior across adapters: `/root/agent-runtime/internal/runtime/fallback.go`.

### Model providers — exactly three

All in `/root/agent-runtime/internal/runtime/native_eino_provider.go`.

| Provider | API key env | Base URL override env | Default model | Const line |
| --- | --- | --- | --- | --- |
| Anthropic | `ANTHROPIC_API_KEY` | `ANTHROPIC_BASE_URL` | `claude-opus-4-8` | `:17` |
| OpenAI (Responses API) | `OPENAI_API_KEY` | `OPENAI_BASE_URL` | `gpt-5.6-terra` | `:18` |
| OpenRouter | `OPENROUTER_API_KEY` | `OPENROUTER_BASE_URL` | `openai/gpt-5.6-terra` | `:19` |

Default base URLs: OpenAI `https://api.openai.com/v1` (`:20`), OpenRouter `https://openrouter.ai/api/v1` (`:21`). Default max tokens 16384 (`:22`).

Underlying libraries (`/root/agent-runtime/go.mod:6-9`):
- `github.com/cloudwego/eino v0.9.12`
- `github.com/cloudwego/eino-ext/components/model/claude v0.1.19` — Anthropic path
- `github.com/cloudwego/eino-ext/components/model/agenticopenai v0.2.2` — OpenAI + OpenRouter path (Responses API)

**Not supported: Gemini, Bedrock, Vertex, Azure, Ollama.** A grep for all of those names across every non-test Go file in the repo returns zero hits. There is no adapter, no stub, and no config surface for them.

**Ollama / OpenAI-compatible base URLs — partial and constrained.** You can point `OPENAI_BASE_URL` at another host, and `resolveOpenAIResponsesBaseURL` (`native_eino_provider.go:233-238`) will honor it. But the OpenAI path constructs `agenticopenai.NewResponsesModel`, i.e. the **Responses API**, not Chat Completions. Ollama and most OpenAI-compatible servers implement Chat Completions only. So this is not a drop-in for local models. OpenRouter is the realistic escape hatch for model breadth, since it fans out to many vendors behind one Responses-compatible endpoint.

### Where model provider config lives

- **Process-global environment**, read once in `DefaultNativeConfigFromEnv` at `/root/agent-runtime/internal/runtime/native_eino_provider.go:70-96`, populating `EinoProviderFactory` (`:61-69`).
- **Per-agent override** in `resolveProviderAndModel` at `/root/agent-runtime/internal/runtime/native_eino_provider.go:202-220`: reads `execCtx.Agent.Provider` and `execCtx.Agent.Model`, falling back to `AGENT_RUNTIME_NATIVE_PROVIDER` / `AGENT_RUNTIME_NATIVE_MODEL`, then to Anthropic.
- **Per-agent execution tuning** via `execCtx.Agent.ExecutionConfig` JSON, currently used only for OpenRouter provider quantization hints (`openRouterExtraFields`, `:161-181`).
- **Capability reporting** to hosts via `NativeProviderCapabilities()` at `:36-59`, surfaced through `GET /capabilities` (`/root/agent-runtime/cmd/agent-runtime/main.go:264-276`). It reports per-provider `configured`, `default_model`, and `base_url_overridden`.
- Anthropic/OpenAI/OpenRouter keys are also allowlisted as process passthrough for subprocess adapters at `/root/agent-runtime/internal/procenv/procenv.go:68-70`.

### BYOK — not supported

There is **no per-app or per-run model API key**. Keys come only from process environment. Searches across `/root/agent-runtime/internal/appconfig/`, `internal/agentcore/`, and `internal/store/` for `byok`, `api_key`, `APIKey` return only one unrelated hit (`internal/appconfig/config.go:338`, Kernel browser). The app config schema (`/root/agent-runtime/internal/appconfig/config.go`) carries host callback endpoints, tokens for those callbacks, browser config, skill/workspace providers — but no model credentials.

Run-scoped credentials **do** exist, but only for **MCP servers**, not model providers: `/root/agent-runtime/internal/mcp/run_config.go`, encrypted with `AGENT_RUNTIME_MCP_CREDENTIAL_ENCRYPTION_KEY`, documented at `/root/agent-runtime/docs/run-scoped-mcp.md`.

Consequence for a multi-tenant self-hosted product: every tenant shares the operator's model keys. Per-workspace BYOK would require new work in the provider factory and the agent/run models.

### Other capability env (from `/root/agent-runtime/README.md`)

Web search providers `TINYFISH_API_KEY` → `EXA_API_KEY` → `BRAVE_SEARCH_API_KEY` with graceful downgrade; `WEB_FETCH_PROXY_URLS`; Kernel browser infrastructure (`AGENT_RUNTIME_BROWSER_ENABLED`, `KERNEL_API_KEY`, ~14 recording/trim knobs).

---

## 2. Is agent-runtime a hard dependency of Helpin's customer-support AI?

### Answer: yes — hard, with no fallback path

This contradicts the framing of the question. The customer-facing support agent is **not** driven by `helpin/server/internal/llm`. It is an agent-runtime **chat-mode run**.

The file header states it outright — `/root/helpin/server/internal/service/support_chat.go:3-7`:

```go
// SupportChatService runs a support conversation as a long-lived agent-runtime
// chat-mode run (Echo). It replaces the in-app LLM pipeline: deterministic
// pre-gates stay here (state machine, idempotency, locks, hard-phrase
// escalation, caps, budget), the reasoning happens in the runtime, and
// support.send_reply / escalate_to_human are the only output paths.
```

### Full call chain, visitor message → delivered reply

**(a) HTTP ingress (widget)**
- Handler `/root/helpin/server/internal/handler/support_inbox_widget.go`
- Service `/root/helpin/server/internal/service/support_inbox_widget.go:734` → `go s.runWidgetPostMessageAutomation(...)`
- `:738-790` gates on `shouldAutomaticallyProcessSupportAI(settings)`, then `:762` → `s.supportAIService.PublishAIRequest(ctx, workspaceID, conversationID, messageID, content)`
- Email channel does the same at `/root/helpin/server/internal/service/email_fallback.go:1372`

**(b) NATS JetStream hop (durability boundary)**
- Publish `AIRequestEvent` to `support.ai.request.<workspaceID>`, stream `SUPPORT_AI` — `/root/helpin/server/internal/service/support_ai_escalate.go:19-40`, stream def `/root/helpin/server/internal/websocket/jetstream.go:22`
- Durable pull consumer `ai-responder`, `MaxDeliver: 3` — `/root/helpin/server/internal/service/support_ai_consumer.go:25-84`; poison messages → `EscalateToHumanForMessage(..., "ai_pipeline_error")` at `:96-118`
- Wiring `/root/helpin/server/cmd/api/main.go:1716-1722` → `supportAIService.StartNATSConsumer(projectionCtx, supportChatService.HandleVisitorMessage)`

**(c) Deterministic Go pre-gates — no LLM**
`/root/helpin/server/internal/service/support_chat.go:82-194` (`HandleVisitorMessage`): settings/agent-id gate `:86-95`; human-takeover / escalated / resolved-reopen state machine `:104-125`; idempotency `processingRepo.BeginAttempt` `:127-131`; per-conversation Redis lock `:134-141`; `checkHardEscalation` bypass `:144-151`; turn caps (`supportChatMaxAITurns = 30`, `AIMaxFollowups`) `:154-165`; token/daily budget `:167-189`; typing indicator `:191`.

**(d) Run lifecycle**
`startOrResumeChatRun` `/root/helpin/server/internal/service/support_chat.go:198-231`:
- active run paused on `AgentRunPauseReasonUserMessage` → `agentService.SendRunMessage(...)` resumes the existing runtime run
- paused on an *interaction* (approval) → `processingRepo.MarkDeferred`
- mid-turn → deferred (queue-and-coalesce)
- no active run → `startSupportChatRun` `:235-296`, building carry-forward transcript (`buildCarryForward` `:301-320`), trigger `supportChatTriggerType = "support_chat"`, `targetType: "support_conversation"`, `invocationMode: InvocationModeInteractive`

**(e) The LLM call happens inside agent-runtime — hard dependency proven here**
`/root/helpin/server/internal/service/agent.go:6496-6510`:

```go
// Agent Runtime is the only execution path — a run that cannot delegate
// fails loudly instead of falling back to a local executor.
if !s.delegatesRunToAgentRuntime(params.agent, params.targetType) {
    err := fmt.Errorf("agent runtime launch is disabled; no execution path available for this run")
    s.failRunStart(ctx, run, params.agent, params.workspaceID, err)
    return nil, err
}
runtimeLauncher, ok := s.agentRuntimeClient.(agentRuntimeLaunchClient)
if !ok || runtimeLauncher == nil {
    err := fmt.Errorf("agent runtime launch is enabled but client is not configured")
    ...
}
```

`delegatesRunToAgentRuntime` at `/root/helpin/server/internal/service/agent.go:223-228` returns true whenever `agentRuntimeLaunchEnabled` is set and a target type exists. Then `:6520` `runtimeLauncher.UpsertAgent(...)` and `:6565` `runtimeLauncher.StartRun(...)`.

Client: `/root/helpin/server/internal/service/agent_runtime_client.go:55-112`, wrapping SDK `github.com/helpin-ai/agent-runtime-go`; constructed at `/root/helpin/server/cmd/api/main.go:1087-1088`.

Chat-mode config: `/root/helpin/server/internal/service/agent.go:607-660` (`defaultSupportChatIdleTimeoutSeconds = 24*60*60`).

`/root/helpin/docs/AGENT_RUNTIME_LOCAL.md:19-21` confirms the absence of any fallback:

> `AGENT_RUNTIME_LAUNCH_ENABLED=false` disables new agent execution. It does not restore an in-process executor.

**(f) Reply delivery — a runtime tool call, never model text**
The runtime calls back into Helpin at `POST /api/internal/agent-runtime/mcp/helpin/call`:
- Route `/root/helpin/server/internal/router/router.go:435-451`, guarded by `middleware.RequireInternalAPISecret`
- Handler `/root/helpin/server/internal/handler/agent_runtime_host.go:148-160` (`CallProviderTool`)
- Host service `/root/helpin/server/internal/service/agent_runtime_mcp.go:52-73` (`ExecuteCommand`)
- Tools `/root/helpin/server/internal/service/internal_command_support_reply.go`: `support.send_reply` / `send_support_reply` registered `:100-140`; `support.escalate_to_human` `:141-160`
- Server-side grounding gate `evaluateSupportReplyGate` `:57-84` — evidence-grounded claim validation, numeric checks, confidence vs workspace threshold, declining-satisfaction trend; failure → escalation `:271-290`. Model label recorded as `supportReplyModelLabel = "agent-runtime"` `:26`
- Evidence snapshot re-validated against `/root/helpin/server/internal/model/support_run_evidence.go` and `/root/helpin/server/internal/service/internal_command_support_knowledge.go`
- Silent-run nudge `/root/helpin/server/internal/service/support_chat_results.go:19-63` (`OnSupportChatRunPaused`), wired via `/root/helpin/server/internal/service/agent_runtime_projection.go:197-205,943-955` and `/root/helpin/server/cmd/api/main.go:1707-1715`

Per `/root/helpin/server/internal/agentcontract/skill_catalog.go:208`, plain assistant text is never delivered to the visitor. Streaming (`/root/helpin/server/internal/service/support_ai_stream.go`) publishes only canned progress labels.

### What still uses `internal/llm` directly

Everything adjacent to the support agent but **not** the customer-facing reply:

| Surface | Path |
| --- | --- |
| Admin agent "support preview" test-drive | `/root/helpin/server/internal/router/router.go:1219` → `handler/support_ai.go:421-445` → `service/support_ai_admin.go:378-509` |
| Teammate composer draft rewrite | `service/support_ai_admin.go:81-215` |
| Conversation → PM task draft | `service/support_ai_admin.go:219-343` |
| Inbox triage / routing | `service/support_inbox_triage.go:749` |
| Knowledge retrieval + embeddings (inside `search_knowledge`) | `service/support_knowledge_search.go:42` + `SUPPORT_RERANKER_*` |
| Support coverage pipeline | `service/support_coverage_*.go` |

### `/root/helpin/server/internal/llm` contents

| File | Contents |
| --- | --- |
| `provider.go` | `Provider` / `EmbeddingProvider` interfaces, `ChatRequest`/`ChatResponse`/`Message`/`ContentPart`/`ReasoningConfig`, `ProviderError` `:26-58`, `Retryable()` `:60-71`, `ErrInsufficientCredits` `:16` |
| `claude.go` | `ClaudeProvider` over `agentcontract.ClaudeClient` `:19-26`; JSON mode via forced tool `emit_json_response` `:11` |
| `openai.go` | `OpenAIProvider`, OpenAI-compatible, default base `https://api.openai.com/v1`, default model `gpt-4o` `:52-59`; also the embeddings provider; `gpt-5*` reasoning handling `:285` |
| `router.go` | `Router` — named chat/embedding provider maps and defaults |
| `support_router.go` | `NewSupportRouter(anthropicKey, openAIKey, openAIBaseURL, openRouterKey, openRouterBaseURL)`; preference anthropic → openai → openrouter `:43-49`; embeddings OpenAI-only `:51-56` |
| `jsonparse.go` | Tolerant JSON extraction from model output |

Three providers: Anthropic, OpenAI-compatible, OpenRouter. No Gemini/Bedrock/Vertex/Ollama — the same gap as agent-runtime. Config at `/root/helpin/server/internal/config/config.go:270-343`. Constructed `/root/helpin/server/cmd/api/main.go:1001,1036`.

**Note:** `internal/llm` *does* accept arbitrary OpenAI-compatible base URLs via Chat Completions, so it is more Ollama-friendly than agent-runtime's Responses-API path. That asymmetry matters if local-model self-hosting is ever a goal.

---

## 3. What agent-runtime requires to run, and self-hosting weight

### Minimum requirements

| Dependency | Required? | Detail |
| --- | --- | --- |
| Store | Yes, but trivial | `memory` \| `sqlite` \| `postgres` — `/root/agent-runtime/internal/store/env.go:20-40` |
| Temporal | **No** | Only when `TEMPORAL_ADDRESS` set — `/root/agent-runtime/cmd/agent-runtime/main.go:307-320` |
| Postgres | **No** | SQLite fully supported — `/root/agent-runtime/internal/store/gorm.go:32-40` |
| NATS | No (optional event sink) | `AGENT_RUNTIME_EVENT_SINK=log,nats` |
| Service token | Yes | `AGENT_RUNTIME_SERVICE_TOKEN`, or `AGENT_RUNTIME_ALLOW_ANONYMOUS=true` for isolated local runs |
| App config | Yes | `AGENT_RUNTIME_APP_CONFIG` JSON or `@/path/file.json` |
| Model key | Yes | At least one of `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / `OPENROUTER_API_KEY` |

Store resolution: `openStore` at `/root/agent-runtime/cmd/agent-runtime/main.go:279-306` — memory store, or GORM SQL with `MigratePostgres` for Postgres and `AutoMigrate` otherwise. Migrations at `/root/agent-runtime/internal/store/migrations.go`; JSON/null-byte sanitization at `/root/agent-runtime/internal/store/sanitize.go`.

Helpin's own local runbook uses SQLite — `/root/helpin/docs/AGENT_RUNTIME_LOCAL.md:49-50`:

```bash
AGENT_RUNTIME_STORE_DRIVER=sqlite
AGENT_RUNTIME_SQLITE_DSN=.local/helpin-agent-runtime.sqlite3
```

### Does agent-runtime have its own docker-compose and k8s? Yes to both.

**Compose** — `/root/agent-runtime/docker-compose.yml`, a single service, memory store, no Postgres, no Temporal:

```yaml
services:
  agent-runtime:
    build: .
    environment:
      AGENT_RUNTIME_ADDR: ":8090"
      AGENT_RUNTIME_STORE_DRIVER: memory
      AGENT_RUNTIME_SERVICE_TOKEN: dev-token
      AGENT_RUNTIME_APP_CONFIG: '{"apps":[]}'
      AGENT_RUNTIME_MCP_CREDENTIAL_ENCRYPTION_KEY: "${AGENT_RUNTIME_MCP_CREDENTIAL_ENCRYPTION_KEY:?set a shared 32-byte base64 key}"
      AGENT_RUNTIME_MCP_ALLOWED_HOSTS: "mcp.customer.io,mcp-eu.customer.io"
      AGENT_RUNTIME_MCP_ALLOW_PRIVATE_NETWORKS: "false"
      AGENT_RUNTIME_MCP_ALLOW_HTTP: "false"
    ports:
      - "8090:8090"
```

Also `/root/agent-runtime/run-local.sh` and `/root/agent-runtime/justfile`.

**Kubernetes** — 18 manifests across two overlays:

```
k8s/stage/{namespace,deployment,worker-deployment,service,secrets,kustomization,console-deployment,console-service,console-ingress}.yaml
k8s/prod/{same nine}
```

Production posture is heavy: ArgoCD GitOps from `develop`→stage and `main`→prod; CloudNativePG Helm chart for Postgres (deliberately **not** in these manifests); Doppler + External Secrets Operator for secrets; a separate worker Deployment for the Temporal worker; an operator console (`packages/console`) behind TLS/basic-auth ingress.

The production runtime image is also large by design — `/root/agent-runtime/Dockerfile` bundles Git, curl, ripgrep, Make, Go 1.24.3, Node/npm, pnpm, Yarn, Python/pip, pytest, uv, Poetry, Rust/Cargo, Codex, OpenCode, and ffmpeg. **Almost none of that is needed for a support-only deployment**; it exists for coding agents.

### Self-hoster weight: verdict

Two very different stories. The *floor* is one container plus a SQLite file plus one model key — genuinely light. The *documented production path* is Kubernetes + ArgoCD + CloudNativePG + Doppler + ESO + a Temporal cluster + a console ingress, which is far beyond a typical self-hoster.

### Does Helpin's docker-compose include it? No.

There are exactly two compose files, `/root/helpin/docker-compose.yaml` and `/root/helpin/docker-compose-infra.yaml`. **Neither defines an `agent-runtime` service.** It is also absent from `/root/helpin/k8s/stage/` and `/root/helpin/k8s/prod/`, which contain only `server.yaml`, `temporal-worker.yaml`, `helpcenter.yaml`, ingresses, migrations, and events-pipeline.

Helpin reaches it purely by URL — `/root/helpin/server/.env.example:117` `AGENT_RUNTIME_BASE_URL`, defaulting to `http://127.0.0.1:8090`.

Helpin's compose **does** run Temporal (`temporal-postgres:19-32`, `temporal:34-52` on `7233`, `temporal-ui:54-65` on `8233`, `temporal-worker:112-139`), but that is Helpin's own worker (`/root/helpin/server/cmd/temporal-worker/main.go`), unrelated to agent-runtime's durable executor. `docker-compose-infra.yaml` has no Temporal, only postgres, pgadmin, nats, redis.

**This is the material self-hosting gap.** Someone who brings up Helpin's compose gets no customer-support AI at all, because the sole executor for it is an out-of-repo service compose never starts, and there is no in-process fallback.

---

## 4. Fin-relevant capabilities, and what Helpin's support module actually consumes

| Capability | agent-runtime location | Consumed by Helpin support today? |
| --- | --- | --- |
| Tool registry | `/root/agent-runtime/internal/tools/registry.go` | **Yes — it is the entire output path** |
| MCP gateway | `/root/agent-runtime/internal/mcp/gateway.go`, `register.go`, `http_provider.go`, `rpc_provider.go` | **Yes**, via Helpin's own provider endpoint |
| Human approval interactions | `/root/agent-runtime/internal/runtime/native_interaction.go:18-27` | **Yes**, for teammates (not visitors) |
| Run-scoped MCP / OAuth credentials | `/root/agent-runtime/internal/mcp/run_config.go`, `/root/agent-runtime/docs/run-scoped-mcp.md` | Plumbed on the shared path, **but unused by the support preset** |
| Workspace providers | `/root/agent-runtime/internal/workspace/git_provider.go`, `http_provider.go` | **No** |

### Tool registry + MCP gateway — consumed, and load-bearing

The support agent's entire output path is runtime tool calls routed back into Helpin:
- Routes `GET/POST /api/internal/agent-runtime/mcp/helpin/{tools,call}` — `/root/helpin/server/internal/router/router.go:443-444`
- Catalog from `InternalCommandService.ToolDefinitions()` — `/root/helpin/server/internal/service/agent_runtime_mcp.go:21-50`
- Support tools: `send_support_reply`, `escalate_to_human`, `search_knowledge`, `draft_support_reply`, `update_conversation_status`, `finish_support_follow_up`, plus ~15 inbox operational tools — `/root/helpin/server/internal/agentcontract/runtime_profiles.go:44`

`/root/helpin/server/internal/service/agent.go:6531-6533` records why the runtime owns this: *"Agent Runtime owns the executable tool registry. Build the run contract from the agent it accepted, rather than the local projection we sent, so allowed_tools cannot race or drift."*

Runtime-side built-in tools that support does **not** use: workspace filesystem/search/command/patch/git (`/root/agent-runtime/internal/tools/workspace_*.go`), repository checkout (`repository_checkout_tools.go`), browser (`browser_tools.go`, `kernel_browser_provider.go`), web (`web_tools.go`), artifact preview (`artifact_preview_tools.go`).

### Human approval interactions — consumed, teammate-facing

Three interaction kinds at `/root/agent-runtime/internal/runtime/native_interaction.go:18-27`: `request_user_input` → `request_user_input_v1`, `request_approval` → `approval_request` (`:343-404`), `request_review_checkpoint` → `review_checkpoint` (`:516-538`). Persisted via `/root/agent-runtime/internal/agentcore/store.go` and surfaced through `/root/agent-runtime/internal/api/server.go`.

Helpin consumption:
- Support defers when pause reason is not `user_message` — `/root/helpin/server/internal/service/support_chat.go:216-219`
- Teammate endpoints `/root/helpin/server/internal/handler/support_inbox_ai_run.go:33-60` (`ListAIRunInteractions`, `ResolveAIRunInteraction`), for `support_plan_confirm` approvals on non-read-only child launches
- Auto-approval boundary `/root/helpin/server/internal/service/support_child_tools.go:15-46`: server-owned read-only allowlist; anything broader pauses for teammate approval. `supportChildMaxConcurrent = 2`, `supportChildMaxPerConversation = 5`
- The support profile lists `request_user_input` / `request_approval` / `request_review_checkpoint` in `AllowedTools` with `ApprovalRequired: false` — `/root/helpin/server/internal/agentcontract/runtime_profiles.go:44-47`

The visitor never sees an approval. This is the closest existing primitive to a Fin-style "hand off with confirmation" flow, but it currently targets the teammate side only.

### Run-scoped MCP / OAuth — available, unused by support

Mechanics live on the shared launch path, applying to any agent whose `AllowedTools` contain non-`mcp__helpin__` `mcp__*` entries:
- `/root/helpin/server/internal/service/agent.go:6541-6584` → `externalMCPService.ResolveRunAttachments` → `startReq.MCPServers`, then `PersistRunBindings`
- Credential rotation `/root/helpin/server/internal/service/agent_runtime_client.go:115-121` (`UpdateRunMCPCredential`), driven from `agent.go:1064`
- OAuth `/root/helpin/server/internal/service/external_mcp_oauth.go`, `/root/helpin/server/internal/externalmcp/{oauth,security}.go`
- Runtime-side guards: `AGENT_RUNTIME_MCP_ALLOWED_HOSTS`, `AGENT_RUNTIME_MCP_ALLOW_PRIVATE_NETWORKS` (deny by default), `AGENT_RUNTIME_MCP_ALLOW_HTTP` (HTTPS required by default), encryption via `AGENT_RUNTIME_MCP_CREDENTIAL_ENCRYPTION_KEY` (32-byte base64, shared API+worker)

But `support_agent` ships **zero** `mcp__*` tools (`/root/helpin/server/internal/agentcontract/runtime_profiles.go:44`) — every tool is a Helpin internal command through the `mcp__helpin__` gateway. A workspace would have to attach external MCP tools to the support agent to light this up. **This is the single most Fin-relevant unused capability**: it is exactly the mechanism for letting a support agent hit Stripe, Shopify, or an order system on behalf of a workspace.

### Workspace providers — not consumed

`support_agent` has `AllowedCommands: []string{}` and `RequiresRepo: false` (`/root/helpin/server/internal/agentcontract/runtime_profiles.go:45-48`). The route `POST /api/internal/agent-runtime/workspace/repository-spec` (`/root/helpin/server/internal/router/router.go:442`) serves coding and docs agents. Support can only reach repo reads indirectly, by launching a read-only child run (`checkout_repositories`, `read_files`, `repository_search` in `supportChildReadOnlyTools`, `/root/helpin/server/internal/service/support_child_tools.go:15-23`).

### Also consumed by support

- **Skills / skill packages** — `find_skills` / `read_skill`; support agent prompt lives in `/root/helpin/server/internal/agentcontract/skill_catalog.go:208-258` (the "every visitor turn MUST end with `send_support_reply` or `escalate_to_human`" contract). Runtime side: `/root/agent-runtime/internal/skills/` (`registry.go`, `policy.go`, `archive.go`, `http_lookup.go`, `http_package_store.go`).
- **v2 durable ordered event projection** — `POST /api/internal/agent-runtime/events` (`router.go:440`) → `/root/helpin/server/internal/service/agent_runtime_projection.go`.
- **Sub-agent / child-run delegation** — `start_agent_run`, `start_agent_plan`, `get_agent_run`, `cancel_agent_run`, gated by `support_child_tools.go`.

### Not consumed at all by support

Browser tools and artifacts (documentation profile only, `runtime_profiles.go:53`); shell execution (`run_command` absent, `AllowedCommands` empty); Helpin's Temporal worker (support durability comes from NATS JetStream plus the runtime chat run's own persistence); streaming assistant text to the visitor.

---

## 5. TODOs, stubs, and "not implemented" markers

A grep for `TODO`, `FIXME`, `not implemented`, `unimplemented`, `XXX:` across every non-test Go file under `/root/agent-runtime/internal/` and `cmd/` returns **exactly one hit**:

```
/root/agent-runtime/internal/workspace/git_provider.go:203
    return nil, fmt.Errorf("repository finalize policy %q is not implemented", spec.FinalizePolicy)
```

That is a repository-workspace concern with no bearing on support use cases.

**Nothing is stubbed in the model provider layer.** Every unconfigured-provider path returns a clean, explicit error rather than a placeholder — `native_eino_provider.go:112` `"anthropic API key is not configured"`, `:126` `"openai API key is not configured"`, `:140` `"openrouter API key is not configured"`, `:157` `"unsupported native Eino provider %q"`.

The conclusion is that the provider gap is **architectural, not unfinished work**. Gemini, Bedrock, and Ollama were never built and nothing in the code anticipates them. Adding one means a new `case` in `ResolveNativeModel`, a new Eino component dependency, new entries in `NativeProviderCapabilities()`, and new keys in `procenv.go`.

Related design docs that read as known-open areas rather than code TODOs, under `/root/agent-runtime/docs/prd/`:
- `2026-07-08-codex-auth-shared-store.md`
- `2026-07-08-codex-resume-approval-hang.md`
- `2026-07-08-native-approval-consumption.md`
- `2026-09-07-native-context-management.md`

---

## 6. Size

| Metric | Value |
| --- | --- |
| Go files | 212 |
| Go lines of code | 65,655 |
| Test files | 85 |
| Test functions (`func Test*`) | 602 |

Roughly 40% of Go files are tests, with ~2.8 test functions per test file. That is a healthy ratio for a service of this kind. Postgres migration tests run in CI and locally via `AGENT_RUNTIME_TEST_POSTGRES_DSN` (`/root/agent-runtime/README.md`).

Non-Go surface: `packages/react` (embeddable run transcript/artifact/interaction UI, `npm test`) and `packages/console` (operator console with server-side BFF).

---

## Appendix: the two SDK repos

**`/root/agent-runtime-go`** — 13 Go files, ~2,972 lines. Public wire contracts and a `/v1` HTTP client, explicitly scoped: *"This package contains only wire contracts, event helpers, host callback DTOs, and a small `/v1` HTTP client. Product lifecycle policy such as billing, projection, finalizers, automations, and UI persistence belongs in the host application."* Files: `client.go`, `types.go`, `events.go`, `sse.go`, `host.go`, `nats.go`, plus an `mcpauth` package. Helpin wraps it at `/root/helpin/server/internal/service/agent_runtime_client.go:55`.

**`/root/agent-runtime-python`** — thin Python client (`agent_runtime/`, `pyproject.toml`, `setup.py`, `tests/`). Same contract surface plus an optional headless MCP OAuth helper (`MCPOAuthClient`, `hash_mcp_oauth_state`) that handles MCP discovery, PKCE, dynamic registration, exchange, and refresh, while the host keeps workspace/user authorization, callback routes, encrypted state and refresh-token storage, tool policy, and notifications.

Neither SDK contains product logic or an execution path. Neither is used by Helpin's support module beyond the Go client's `StartRun` / `UpsertAgent` / `SendRunMessage` / `UpdateRunMCPCredential` calls.

---

## Two conclusions worth surfacing

**1. The support agent is not separable from agent-runtime.** Any plan treating agent-runtime as optional infrastructure serving only coding/PM/CRM agents is working from a false premise. `/root/helpin/server/internal/service/agent.go:6496-6498` removes the local executor deliberately and fails loudly instead. Removing or not deploying agent-runtime removes customer-facing autoreply. Given the standing scope-discipline principle of defaulting to subtracting scope, the honest framing here is inverted: agent-runtime is already load-bearing for the most customer-visible feature, so the question is what to trim *around* it, not whether to keep it.

**2. Self-hosting is broken by omission, and cheaply fixable.** No compose or k8s manifest in `/root/helpin` starts agent-runtime, yet the support AI cannot function without it. The floor configuration is one container with `AGENT_RUNTIME_STORE_DRIVER=sqlite` and no Temporal and no Postgres. Adding that single service to `/root/helpin/docker-compose.yaml` would close the gap. The heavy production topology (ArgoCD, CloudNativePG, Doppler, ESO, Temporal, console ingress) is a deployment choice, not a requirement of the software.
