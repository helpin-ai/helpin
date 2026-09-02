# Helpin MCP Tool Registration PRD

**Date:** 2026-09-02  
**Status:** Implemented for compatibility-first Release 1  
**Repositories:** `agent-runtime-go`, `agent-runtime`, `helpin`

## Outcome

Helpin publishes its complete internal command-tool catalog through the same
simple HTTP MCP-provider contract already used by Usermaven. Agent Runtime
discovers those definitions under bare canonical names and exposes them through
the shared app registry to Codex and Native SDK executions when the effective
agent/run `allowed_tools` policy permits them.

Release 1 keeps the frozen static Helpin command table and
`/commands/execute` as a startup fallback. MCP definitions override overlapping
static definitions. Release 2 removes both legacy surfaces only after 14
consecutive healthy days.

## Contracts

- `agent-runtime-go.Tool` carries `risk_level` and compatibility `aliases`.
- `ToolCallResult` carries raw `structured_content` so command JSON is not
  flattened into a text wrapper.
- Provider operations accept `context.Context`; `/tools` has an eight-second
  deadline while `/call` retains the five-minute execution timeout.
- Provider-supplied `supported_target_types` is deliberately not projected into
  runtime definitions in this release. Helpin's values are context hints, and
  propagating Usermaven's existing values would introduce new Codex filtering
  that Native SDK does not enforce.

Helpin serves authenticated internal endpoints:

```text
GET  /api/internal/agent-runtime/mcp/helpin/tools
POST /api/internal/agent-runtime/mcp/helpin/call
```

The call path resolves model aliases to internal command names server-side and
reuses `AgentRuntimeHostService`, preserving app validation, workspace mapping,
actor authorization, and command execution.

## Runtime behavior

- Registry precedence is runtime-global, app-static, then app providers in
  configuration order. Provider refresh replaces one complete app-provider
  layer atomically.
- `tool_namespace` defaults to `provider`; Usermaven therefore keeps its
  `usermaven__...` names. Helpin uses `none` and keeps bare `snake_case` names.
- Provider aliases live in an app-scoped alias-to-canonical map. They are
  accepted by lookup, execution, and allowlist normalization but never appear
  as duplicate model definitions.
- Helpin refreshes every 30 seconds. Failures retain the last-known-good
  snapshot, or the declared static fallback before the first successful load.
  Empty and invalid catalogs are rejected.
- `/healthz` remains process-only. `/readyz` reports provider readiness.
  Workers do not poll Temporal until required provider catalogs or declared
  fallbacks are ready.
- Discovery is not authorization. New commands must still be added to Helpin
  presets or customer-agent `allowed_tools`. Active executions retain their
  cloned snapshot; the next new or resumed execution sees a successful refresh.
- OpenCode's current `mcp__`-only broker behavior is unchanged and is not part
  of the bare Helpin parity guarantee.

## Compatibility rollout

Release 1 configures both `command_provider` and the Helpin MCP provider:

```yaml
mcp_providers:
  - name: helpin
    transport: http
    url: http://helpin-internal/api/internal/agent-runtime/mcp/helpin
    token_env: HELPIN_INTERNAL_API_SECRET
    tool_namespace: none
    refresh_interval: 30s
    startup_policy: allow_fallback
    unknown_refresh_cooldown: 30s
```

The static table is frozen at 54 definitions in CI. Do not add newly discovered
tools to presets during the rolling window until every Agent Runtime pod reports
the MCP source active.

Release 2 is permitted after 14 consecutive days with zero
`/commands/execute` calls, MCP active on every pod, no unresolved fallback or
stale-snapshot incident, and passing Codex, Native SDK, and Usermaven regression
checks. Any violation resets the window. Release 2 deletes the static table and
legacy endpoint and makes the Helpin provider required.

## Acceptance coverage

- SDK JSON compatibility for risk, aliases, and structured content.
- Race-tested concurrent registry listing, lookup, clone, execute, and refresh.
- Complete Helpin catalog validation, including non-exposed non-nil tool blocks,
  duplicate aliases, executable commands, and `create_collection` presence.
- Canonical/legacy alias resolution without duplicate model tools.
- Identical allowed Helpin definitions and structured execution in Codex and
  Native SDK paths.
- Usermaven namespace, startup-only discovery, text-result shape, sensitive
  mutation fallback, app isolation, and intentionally un-enforced target
  metadata remain unchanged.
- Readiness failure cannot fail liveness or allow workers to poll with a missing
  required catalog.
