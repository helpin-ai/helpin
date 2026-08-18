# Local Agent Runtime Setup

Use this runbook to execute Helpin agents against a local `agent-runtime`.
Agent Runtime is the only agent executor: system agents, custom agents, and
one-shot command agents all use this connection for `native_sdk`, `codex`, and
`opencode` runs.

For the ownership model, see `docs/AGENTS_AND_AUTOMATION.md`. For staging, see
`docs/AGENT_RUNTIME_STAGING.md`.

## Helpin environment

Use `127.0.0.1`; some machines resolve `localhost` to IPv6 before IPv4.

```bash
AGENT_RUNTIME_BASE_URL=http://127.0.0.1:8090
AGENT_RUNTIME_SERVICE_TOKEN=dev-token
AGENT_RUNTIME_APP_ID=helpin
AGENT_RUNTIME_EVENT_PROTOCOL=v2
AGENT_RUNTIME_LAUNCH_ENABLED=true
```

`AGENT_RUNTIME_LAUNCH_ENABLED=false` disables new agent execution. It does not
restore an in-process executor.

Helpin's API and Temporal worker must also share a NATS JetStream connection
with Agent Runtime so runtime events can be projected into Helpin run records.

## Runtime app configuration

Configure the `helpin` app entry with these host providers:

- target context: `/api/internal/agent-runtime/target-context`
- commands: `/api/internal/agent-runtime/commands`
- skills and packages: `/api/internal/agent-runtime/skills` and
  `/api/internal/agent-runtime/skill-packages`
- repository workspaces: `/api/internal/agent-runtime/workspace`
- browser artifacts: `/api/internal/agent-runtime/artifacts`

All callbacks must use Helpin's `INTERNAL_API_SECRET`. The canonical app shape
and endpoint derivation are maintained in the Agent Runtime repository at
`docs/app-configuration.md` and `docs/staging-app-config.md`.

Example local runtime environment:

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

Add the provider key required by the selected agent. Browser runs additionally
need `AGENT_RUNTIME_BROWSER_ENABLED=true`, `KERNEL_API_KEY`, and a browser block
in the Helpin app configuration.

## Start order

1. Start NATS with JetStream.
2. Start the Helpin API and Temporal worker with the Helpin environment above.
3. Start the Agent Runtime API and durable worker with the same NATS URL.
4. Launch any system or custom agent against one of its allowed targets.

The Helpin service upserts the selected executable agent definition into Agent
Runtime immediately before starting the run. Agent Runtime does not seed
Helpin's agents itself.

## Smoke verification

For the Helpin `agent_runs` row, confirm:

- `external_runtime = 'agent-runtime'`;
- `external_runtime_id` is populated;
- `workflow_id` is null because Agent Runtime owns the run workflow;
- status, messages, interactions, artifacts, and usage are projected back from
  runtime events;
- the agent returns to `idle` after a terminal run.

For a repository-backed run, also confirm that Agent Runtime prepared the
expected repository and branch, and that Helpin's delivery finalizer recorded
the pushed branch or pull request.

For a custom agent, use the same checks. `is_system` and preset ownership do not
change launch routing.

## Common failures

| Symptom | Likely cause |
| --- | --- |
| Run start says Agent Runtime is disabled | `AGENT_RUNTIME_LAUNCH_ENABLED` is not `true` in Helpin |
| Run start says client is not configured | Base URL, service token, or app ID is missing |
| Runtime returns agent not found | Agent registration/upsert failed before run start |
| Runtime cannot resolve a target | Helpin app config points to the wrong context endpoint or token |
| Product tools are missing | Tool is not registered for the Helpin app, not allowed by the agent, or narrowed out at run start |
| Repository workspace is missing | The agent does not request repository workspace mode or the target has no authorized repository spec |
| Run status changes in Runtime but not Helpin | NATS URLs, stream/consumer configuration, or the Helpin projection worker differ |
| Codex pauses for authentication | Complete the runtime-issued device-code flow and resume the run |

There is no local execution fallback. Diagnose the runtime connection or
configuration instead of looking for a Helpin worker executor.
