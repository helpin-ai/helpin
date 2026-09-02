# Single-Release Plan — Helpin MCP Tool Registration

**Date:** 2026-09-02
**Status:** Implemented
**Repositories:** `agent-runtime-go`, `agent-runtime`, `helpin`

## Outcome

Helpin is not live and has no persisted allowlists or draining runs to preserve.
It therefore ships one registration and execution path: Helpin publishes its
canonical internal command-tool catalog through authenticated `/tools` and
`/call` endpoints, and Agent Runtime projects that catalog to Codex, Native SDK,
and OpenCode subject to the normal agent/run allowlist.

The static Agent Runtime command catalog, HTTP command executor,
`command_provider`, provider aliases, `allow_fallback`, and Helpin's legacy
`/commands/execute` route are removed in this release.

## Contracts and runtime behavior

- `agent-runtime-go.Tool` carries `risk_level` but no provider alias list.
  `ToolCallResult.structured_content` preserves raw command JSON.
- Helpin publishes bare `snake_case` names. `supported_target_types` remains
  deliberately unprojected because neither Helpin nor Usermaven metadata is an
  authorization boundary shared by every runtime.
- Agent Runtime keeps immutable atomic app snapshots, live refresh with
  last-known-good retention, provider health, `/readyz`, and the worker idle
  gate. Helpin uses a 30-second refresh; Usermaven remains startup-only.
- `startup_policy` is omitted or `required`. The API retries discovery with
  jittered exponential backoff for up to 90 seconds and then exits. Workers stay
  alive and unready, retry in the background, and do not poll Temporal until the
  required catalog is available.
- Provider aliases are not registered or resolved. Agent Runtime retains its
  small built-in `CanonicalName` normalization. Helpin retains
  `CanonicalToolName` and `NormalizeToolNames` only for authored/persisted agent
  configuration; `/call` accepts only exact published names.
- Discovery is not authorization. A tool must also be present in the effective
  agent/run `allowed_tools` before a model can list or call it.

Authenticated Helpin endpoints:

```text
GET  /api/internal/agent-runtime/mcp/helpin/tools
POST /api/internal/agent-runtime/mcp/helpin/call
```

## Configuration

```yaml
mcp_providers:
  - name: helpin
    transport: http
    url: http://helpin-internal/api/internal/agent-runtime/mcp/helpin
    token_env: HELPIN_INTERNAL_API_SECRET
    tool_namespace: none
    refresh_interval: 30s
    startup_policy: required
    unknown_refresh_cooldown: 30s
```

Future tool renames update presets, templates, skills, and stored agent data in
one explicit migration. Permanent provider aliases are not a rename mechanism.

## Verification

- Every Helpin-owned tool in every built-in preset resolves to one canonical
  provider entry, including `create_collection`.
- Helpin catalog validation covers all non-nil tool blocks, executable commands,
  canonical names, unique names, and valid input schemas.
- Codex, Native SDK, and OpenCode list and execute the same allowed canonical
  Helpin tools; disallowed tools remain hidden.
- Usermaven retains its `usermaven__` namespace, startup-only discovery,
  existing risk/result behavior, and non-enforcing target metadata.
- Required startup, live refresh, last-known-good, readiness, and worker
  idle-gating tests pass.
- `go test -race ./...`, `go vet ./...`, and clean builds pass in all three Go
  modules.

## Release and rollback

Publish the SDK contract first and pin both downstream modules to it. Deploy
Helpin first and verify `/mcp/helpin/tools`, then deploy Agent Runtime. This
ordering protects the live Usermaven workload from Agent Runtime pods waiting
on a Helpin version that does not expose the required catalog. Roll back Agent
Runtime first, then Helpin.

There are no database migrations or persisted Helpin agent-state changes.
Agent Runtime's API intentionally depends on Helpin discovery: it exits after
the bounded retry when Helpin is unavailable, while workers remain idle and
recover automatically.
