# Agent Runtime Staging Enablement Runbook

Ops runbook for enabling delegated Helpin agent runs on staging
(stage.helpin.ai) against the shared `agent-runtime` service. Local setup lives
in `docs/AGENT_RUNTIME_LOCAL.md`; this document covers staging only.

> Current architecture: Agent Runtime is Helpin's only agent executor. Setting
> `AGENT_RUNTIME_LAUNCH_ENABLED=false` does not roll back to an in-process
> executor; it makes new agent run starts fail. Before changing the flag,
> verify runtime availability and use the runtime's own rollback controls.

## Topology

| Component | Where | Deployed by | Env source |
| --- | --- | --- | --- |
| Helpin API + temporal-worker | `helpin` namespace (`k8s/stage/server.yaml`, `k8s/stage/temporal-worker.yaml`) | GitHub Actions (`develop` -> stage) | `helpin-secrets` k8s secret (Doppler-managed, not in this repo) |
| agent-runtime API + worker | `agent-runtime` namespace (`agent-runtime` repo, `k8s/stage/`) | ArgoCD sync of `agent-runtime` repo | `agent-runtime-secrets` k8s secret via ExternalSecret from ClusterSecretStore `doppler-agent-runtime-api` (`dataFrom: find` — every Doppler secret becomes an env var) |
| NATS JetStream | shared staging cluster | existing | — |

Call directions:

- Helpin -> runtime: `AGENT_RUNTIME_BASE_URL` +
  `Authorization: Bearer $AGENT_RUNTIME_SERVICE_TOKEN` (launch, cancel, list).
- Runtime -> Helpin: host adapter callbacks at
  `https://stage.helpin.ai/api/internal/agent-runtime/...`, bearer-authed
  against Helpin's `INTERNAL_API_SECRET`
  (`internal/middleware/auth.go: RequireInternalAPISecret`).
- Runtime -> Helpin (async): NATS JetStream stream `AGENT_RUNTIME_EVENTS`,
  consumed by the Helpin temporal-worker projection
  (`internal/service/agent_runtime_projection.go`, subject filter
  `agent-runtime.events.helpin.>`).

IMPORTANT: the production app `usermaven` already runs on this runtime. Every
change to the runtime's Doppler config (especially `AGENT_RUNTIME_APP_CONFIG`)
must preserve the existing usermaven entry byte-for-byte.

## Step 1 — Doppler secrets: agent-runtime side

Doppler project behind ClusterSecretStore `doppler-agent-runtime-api`, staging
config. Set/verify:

| Secret | Value | Notes |
| --- | --- | --- |
| `AGENT_RUNTIME_SERVICE_TOKEN` | generate: `openssl rand -hex 32` | REQUIRED. The runtime now FAILS CLOSED at startup without it (`AGENT_RUNTIME_ALLOW_ANONYMOUS` must never be set on staging/prod). Changing it breaks usermaven's calls — if a token already exists, reuse it for Helpin instead of rotating. |
| `AGENT_RUNTIME_APP_CONFIG` | merged JSON, see below | Append the `helpin` entry to the EXISTING value. Do not touch the usermaven object. |
| `AGENT_RUNTIME_EVENT_SINK` | `log,nats` | Enables NATS publishing alongside logs. |
| `AGENT_RUNTIME_NATS_URL` | same NATS URL as Helpin's `NATS_URL` | See "NATS sharing" below. |
| `EXA_API_KEY` | Exa provider key | Required when Helpin agents enable `web_search_exa`. Restart both the runtime API and durable worker after adding or rotating it. |
| `AGENT_RUNTIME_BROWSER_ENABLED` | `true` | Enables the shared Kernel browser infrastructure; apps still opt in independently in `AGENT_RUNTIME_APP_CONFIG`. |
| `KERNEL_API_KEY` | Kernel API key | Shared runtime infrastructure credential; never exposed to host apps or models. |
| `AGENT_RUNTIME_BROWSER_SESSION_TIMEOUT_SECONDS` | `300` | Safety timeout; the runtime also closes each session when the run ends. |

### `AGENT_RUNTIME_APP_CONFIG` value (canonical)

Canonical copy with endpoint-derivation table:
`agent-runtime` repo -> `docs/staging-app-config.md`. Merged form to paste into
Doppler (ops-pending action — cannot be done from either repo):

```json
{
  "apps": [
    {
      "app_id": "usermaven",
      "_do_not_edit": "PLACEHOLDER — replace this whole object with the existing usermaven entry copied verbatim from the current Doppler value."
    },
    {
      "app_id": "helpin",
      "context_endpoint": "https://stage.helpin.ai/api/internal/agent-runtime/target-context",
      "context_token": "<HELPIN_INTERNAL_API_SECRET>",
      "command_provider": {
        "transport": "http",
        "base_url": "https://stage.helpin.ai/api/internal/agent-runtime/commands",
        "token": "<HELPIN_INTERNAL_API_SECRET>"
      },
      "skill_provider": {
        "transport": "http",
        "base_url": "https://stage.helpin.ai/api/internal/agent-runtime/skills",
        "package_base_url": "https://stage.helpin.ai/api/internal/agent-runtime/skill-packages",
        "token": "<HELPIN_INTERNAL_API_SECRET>"
      },
      "workspace_provider": {
        "transport": "repository",
        "base_url": "https://stage.helpin.ai/api/internal/agent-runtime/workspace",
        "token": "<HELPIN_INTERNAL_API_SECRET>",
        "root_dir": "/var/lib/agent-runtime-workspaces"
      },
      "browser": {
        "enabled": true,
        "allowed_domains": ["*"],
        "artifact_provider": {
          "transport": "http",
          "upload_endpoint": "https://stage.helpin.ai/api/internal/agent-runtime/artifacts",
          "token": "<HELPIN_INTERNAL_API_SECRET>"
        }
      }
    }
  ]
}
```

- `<HELPIN_INTERNAL_API_SECRET>` is a placeholder, never a literal in git.
  Replace it with staging Helpin's `INTERNAL_API_SECRET` value when composing
  the Doppler secret — or add `HELPIN_INTERNAL_API_SECRET` as its own secret in
  the runtime Doppler config and use a Doppler secret reference
  (`${HELPIN_INTERNAL_API_SECRET}`) inside the JSON so the literal exists once.
- The runtime appends fixed path suffixes; the base URLs above resolve to
  exactly the routes Helpin registers in `internal/router/router.go`:
  `/target-context`, `/commands/execute`, `/workspace/repository-spec`,
  `/skills/by-id`, `/skills/active-by-key`, `/skill-packages/objects/*`, and
  `/artifacts`.
- `allowed_domains: ["*"]` permits authenticated navigation to any HTTP(S)
  host for Helpin runs. Other apps retain their own browser enablement, domain
  policy, profile namespace, and artifact provider.

## Step 2 — Deploy runtime config and verify auth

1. ArgoCD: sync the `agent-runtime` staging app (ops-pending action) so pods
   restart and pick up the new `agent-runtime-secrets` values (ExternalSecret
   refresh interval is 1m; pods still need a restart to see changed env).
2. Verify runtime is up and fails closed:

```bash
# From inside the cluster (runtime service is ClusterIP, not exposed publicly):
kubectl -n agent-runtime run curl-check --rm -it --image=curlimages/curl --restart=Never -- \
  sh -c '
    curl -s -o /dev/null -w "healthz: %{http_code}\n" http://agent-runtime:8090/healthz;   # expect 200
    curl -s -o /dev/null -w "no token: %{http_code}\n" http://agent-runtime:8090/v1/agents?app_id=helpin;  # expect 401
    curl -s -o /dev/null -w "with token: %{http_code}\n" -H "Authorization: Bearer $TOKEN" \
      "http://agent-runtime:8090/v1/agents?app_id=helpin"'  # expect 200
```

3. Verify usermaven is unaffected: runtime logs show no app-config decode
   errors, and a `GET /v1/agents?app_id=usermaven` with the service token still
   returns their agents.

## Step 3 — Verify Helpin /internal endpoints reject outsiders

Before pointing the runtime at Helpin, confirm the host adapter surface is
locked down from the public internet:

```bash
curl -s -o /dev/null -w "%{http_code}\n" -X POST \
  https://stage.helpin.ai/api/internal/agent-runtime/target-context   # expect 401
curl -s -o /dev/null -w "%{http_code}\n" -X POST \
  https://stage.helpin.ai/api/internal/agent-runtime/commands/execute # expect 401
curl -s -o /dev/null -w "%{http_code}\n" -X POST \
  https://stage.helpin.ai/api/internal/agent-runtime/workspace/repository-spec # expect 401

# With the shared secret (expect non-401 — 400 for an empty body is fine):
curl -s -o /dev/null -w "%{http_code}\n" -X POST \
  -H "Authorization: Bearer $INTERNAL_API_SECRET" -H "Content-Type: application/json" -d '{}' \
  https://stage.helpin.ai/api/internal/agent-runtime/target-context
```

A `503 internal API not configured` means staging Helpin is missing
`INTERNAL_API_SECRET` — set it in the Helpin Doppler config first.

## Step 4 — Doppler secrets: Helpin side (flag stays OFF)

Staging Helpin env comes entirely from the `helpin-secrets` secret via
`envFrom` in `k8s/stage/server.yaml` and `k8s/stage/temporal-worker.yaml` — the
manifests carry no per-variable env lists for these, so no manifest change is
needed; all four keys are Doppler-only (ops-pending action):

| Secret | Value |
| --- | --- |
| `AGENT_RUNTIME_BASE_URL` | `http://agent-runtime.agent-runtime.svc.cluster.local:8090` |
| `AGENT_RUNTIME_SERVICE_TOKEN` | same value as the runtime's `AGENT_RUNTIME_SERVICE_TOKEN` |
| `AGENT_RUNTIME_APP_ID` | `helpin` |
| `AGENT_RUNTIME_LAUNCH_ENABLED` | `true` (required; false/unset disables agent execution) |
| `INTERNAL_API_SECRET` | must already exist; same value embedded in the runtime app-config tokens |

Redeploy/restart the Helpin server and temporal-worker deployments so they pick
up the environment. The flag must be true before launching agent runs.

## Step 5 — NATS sharing requirement

- The runtime publishes run events to JetStream stream `AGENT_RUNTIME_EVENTS`
  (subjects `agent-runtime.events.>`).
- The Helpin temporal-worker consumes that stream for live run projection
  (`agent_runtime_projection.go`) using Helpin's `NATS_URL`; it creates the
  stream if missing.
- Therefore the runtime's `AGENT_RUNTIME_NATS_URL` and Helpin's `NATS_URL`
  MUST point at the same NATS JetStream cluster. If they differ, delegated runs
  launch but never project status back — rows stay in their launch state until
  the reconciliation sweep backstop.
- Verify from a debug pod:
  `nats -s "$NATS_URL" stream info AGENT_RUNTIME_EVENTS` shows the stream, and
  after Step 6 message counts grow during a run.

## Step 6 — Flag rollout order

1. Runtime config deployed and verified (Steps 1–2).
2. Helpin `/api/internal/...` verified 401 from outside (Step 3).
3. Confirm `AGENT_RUNTIME_LAUNCH_ENABLED=true` in the STAGING Helpin Doppler
   config and restart Helpin server + temporal-worker.
4. Confirm an authenticated runtime capabilities request succeeds before
   launching a run.
5. Smoke test: launch a marketer-preset workspace run
   on staging and confirm in the Helpin DB:
   - `agent_runs.external_runtime = 'agent-runtime'`
   - `agent_runs.external_runtime_id` set, no Helpin `workflow_id`
   - status/messages progress via projection (NATS), not just the sweep
6. Watch runtime logs for callback failures against
   `stage.helpin.ai/api/internal/agent-runtime/...` (401s here mean the
   app-config token and Helpin's `INTERNAL_API_SECRET` have drifted).

## Rollback

- Do not use `AGENT_RUNTIME_LAUNCH_ENABLED=false` as a rollback: it disables
  new run starts because no in-process executor exists. Roll back the runtime
  deployment/configuration while keeping the Helpin flag enabled.
- In-flight delegated runs are NOT orphaned: the runtime keeps executing them
  and the Helpin projection consumer (which runs regardless of the launch flag)
  finishes projecting their terminal state.
- Do not remove the `helpin` entry from `AGENT_RUNTIME_APP_CONFIG` while
  delegated runs are still active — the runtime needs the callbacks to finish
  them. Remove it only after all delegated runs are terminal, and again
  preserve the usermaven entry when editing.

## Ops-pending checklist (actions outside git)

- [ ] Runtime Doppler (project behind `doppler-agent-runtime-api`, staging
      config): set `AGENT_RUNTIME_SERVICE_TOKEN`, merged
      `AGENT_RUNTIME_APP_CONFIG` (usermaven preserved), `AGENT_RUNTIME_EVENT_SINK`,
      `AGENT_RUNTIME_NATS_URL`, browser infrastructure keys, and `EXA_API_KEY`
      when Exa search is enabled.
- [ ] ArgoCD sync of the `agent-runtime` staging app + API and worker pod restart.
- [ ] Helpin Doppler (`helpin-secrets` source, staging config): set the four
      `AGENT_RUNTIME_*` keys and confirm `INTERNAL_API_SECRET`.
- [ ] Restart staging Helpin server + temporal-worker deployments.
- [ ] Flag flip to `true` after verification, staging only.
- [ ] After applying the slimmed Temporal worker manifest, run
      `scripts/agent-runtime-retirement/delete-retired-deployments.sh <staging-kubectl-context> --confirm`
      so the five removed zero-replica Deployment objects do not linger.
