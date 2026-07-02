# Local Agent Runtime Setup

This runbook is for testing delegated Helpin runs against a local `agent-runtime`.
It is intentionally local-only; staging and production app config are managed as
runtime secrets and must merge Helpin with any existing app entries.

## Helpin Env

Use `127.0.0.1` for the runtime URL. On some machines Go resolves `localhost`
to IPv6 first while the runtime listens on IPv4.

```bash
AGENT_RUNTIME_BASE_URL=http://127.0.0.1:8090
AGENT_RUNTIME_SERVICE_TOKEN=dev-token
AGENT_RUNTIME_APP_ID=helpin
AGENT_RUNTIME_LAUNCH_ENABLED=true
```

`AGENT_RUNTIME_LAUNCH_ENABLED=true` currently delegates only Mira workspace runs.
Other Helpin runs continue through the in-process Temporal executor.

## Runtime App Config

Create a local app config whose callback URLs point at the running Helpin API
port. If Helpin is on `:8080`, use:

```json
{
  "apps": [
    {
      "app_id": "helpin",
      "context_endpoint": "http://127.0.0.1:8080/api/internal/agent-runtime/target-context",
      "context_token": "dev-token",
      "workspace_provider": {
        "transport": "repository",
        "base_url": "http://127.0.0.1:8080/api/internal/agent-runtime/workspace",
        "token": "dev-token",
        "root_dir": ".local/workspaces"
      },
      "command_provider": {
        "transport": "http",
        "base_url": "http://127.0.0.1:8080/api/internal/agent-runtime/commands",
        "token": "dev-token"
      },
      "skill_provider": {
        "transport": "http",
        "base_url": "http://127.0.0.1:8080/api/internal/agent-runtime/skills",
        "package_base_url": "http://127.0.0.1:8080/api/internal/agent-runtime/skill-packages",
        "token": "dev-token"
      }
    }
  ]
}
```

If Helpin is running on another port, update all three callback URLs. The runtime
must send the same bearer token as `AGENT_RUNTIME_SERVICE_TOKEN`.

## Runtime Env

Example `agent-runtime` local env:

```bash
AGENT_RUNTIME_ADDR=:8090
AGENT_RUNTIME_STORE_DRIVER=sqlite
AGENT_RUNTIME_SQLITE_DSN=.local/helpin-agent-runtime.sqlite3
AGENT_RUNTIME_SERVICE_TOKEN=dev-token
AGENT_RUNTIME_APP_CONFIG=@/tmp/helpin-app-config.json
TEMPORAL_ADDRESS=localhost:7233
TEMPORAL_NAMESPACE=default
TEMPORAL_TASK_QUEUE_PREFIX=helpin-
AGENT_RUNTIME_EVENT_SINK=log,nats
AGENT_RUNTIME_NATS_URL=nats://127.0.0.1:4222
CODEX_APP_SERVER=true
CODEX_PATH=codex
```

`CODEX_APP_SERVER=true` is required when using the normal Codex CLI binary. Raw
command mode starts the interactive TUI and fails in non-TTY worker execution.

Start the runtime from the `agent-runtime` checkout:

```bash
set -a
source .env.helpin-local
set +a
GOCACHE=/private/tmp/agent-runtime-go-cache go run ./cmd/agent-runtime
```

Start a local NATS server with JetStream before running the Helpin worker and
runtime. Live projection uses the `AGENT_RUNTIME_EVENTS` stream; the
reconciliation sweep is only a backstop.

```bash
nats-server -js
```

## Smoke Check

1. Start NATS with JetStream.
2. Start Helpin API and the Temporal worker with the Helpin env above.
3. Start `agent-runtime` with the app config above.
4. Launch a Mira workspace run.
5. Confirm the Helpin `agent_runs` row has:
   - `external_runtime = 'agent-runtime'`
   - `external_runtime_id` set
   - no Helpin `workflow_id`
6. Confirm projection updates the row from runtime events. The worker creates
   the `AGENT_RUNTIME_EVENTS` stream if it is missing.

For local Codex auth flows, a successful unauthenticated smoke can pause with
`pause_reason = authentication`; that still validates launch delegation,
runtime execution, and Helpin projection.
