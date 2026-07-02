# Agent Runtime Shadow Validation (Mira Canary Slice)

Runbook for validating the Helpin → agent-runtime migration on staging using
real Mira (marketer-preset) workspace runs before widening the delegated
surface. Companion docs:

- `docs/AGENT_RUNTIME_LOCAL.md` — local E2E smoke and the Marketer Workspace
  Parity Row (capability status table).
- `server/internal/service/agent_runtime_projection.go` — the projection
  worker whose behavior this runbook validates.
- Canary script: `scripts/agent-runtime-canary/canary.sh`.

## Scope

`AGENT_RUNTIME_LAUNCH_ENABLED=true` delegates **only** workspace-target runs
for agents whose `preset_key` is the marketer preset (the predicate is
preset-based, not name-based — see `shouldDelegateRunToAgentRuntime` in
`server/internal/service/agent.go`). All other runs continue through the
in-process Temporal executor. Shadow validation exercises exactly this slice.

A correctly delegated run has:

| Field | Expected |
| --- | --- |
| `external_runtime` | `agent-runtime` (stamped synchronously at launch) |
| `external_runtime_id` | runtime run ID, non-null at launch |
| `workflow_id` | **null** — Helpin creates no local run workflow |
| `status` | projected from NATS: `queued → running (→ paused) → completed/failed/cancelled` |
| `tokens_used` / `input_tokens` / `output_tokens` | projected usage, non-zero for real model runs |
| `output_summary.agent_runtime_usage_consumed` | `true` after the terminal event's billing consumption |
| `output_summary.agent_runtime_usage_consumed_at` | timestamp of consumption |

Transcript (`agent_run_messages`), artifacts (`agent_run_artifacts`), and
interactions are mirrored from the `AGENT_RUNTIME_EVENTS` JetStream stream by
the projection worker running in the Temporal worker process; the
reconciliation sweep is only a backstop.

## Pre-requisites

All of these must hold before the validation window starts. A canary run that
fails a pre-req produces a misleading signal.

1. **Staging config slice deployed.** Helpin API **and** Temporal worker on
   staging both have:
   - `AGENT_RUNTIME_BASE_URL`, `AGENT_RUNTIME_SERVICE_TOKEN`,
     `AGENT_RUNTIME_APP_ID` (must match the runtime app config entry)
   - `AGENT_RUNTIME_LAUNCH_ENABLED=true`
   - `NATS_URL` reachable, JetStream enabled. The worker creates the
     `AGENT_RUNTIME_EVENTS` stream if missing (durable consumer
     `helpin-agent-runtime-projection`).
   - The runtime's app config callbacks (`context_endpoint`,
     `workspace_provider`, `command_provider`, `skill_provider`) point at the
     staging Helpin API and use the same bearer token as
     `AGENT_RUNTIME_SERVICE_TOKEN`. Staging app config is a runtime secret —
     merge the Helpin entry with existing app entries, never overwrite.
2. **Real model API keys on the runtime.** The agent-runtime deployment must
   have production-grade model credentials configured. **Deterministic /
   stub fallback is NOT allowed** during shadow validation — the canary
   enforces this by failing any run with `tokens_used == 0`.
3. **Runtime auth pre-provisioned.** For any Codex-backed tests, the codex
   workspace on the runtime must be pre-authenticated (complete the
   device-code login once, out of band). A run that parks at
   `pause_reason=authentication` validates launch delegation but is a canary
   failure for shadow-validation purposes.
4. **A Mira agent exists in the canary workspace.** Note its `agent_id`
   (`GET /api/pm/agents` with `X-Workspace-ID`). It must use the marketer
   preset. The canary workspace should be a dedicated staging workspace with
   AI usage credits available (preflight rejects launch when the workspace
   meter is exhausted).
5. **Canary user token.** A staging user JWT with `pm.edit` in the canary
   workspace and the Automation module enabled for the workspace.

## Running the canary

```bash
export HELPIN_API_URL=https://stage.helpin.ai/api
export HELPIN_API_TOKEN=<user JWT>
export WORKSPACE_ID=<canary workspace UUID>
export AGENT_ID=<Mira agent UUID>

./scripts/agent-runtime-canary/canary.sh
```

Optional knobs: `CANARY_TIMEOUT_SECS` (default 900), `CANARY_POLL_INTERVAL_SECS`
(default 5), `CANARY_PROMPT` (the `additional_context` for the run),
`CANARY_CANCEL_ON_TIMEOUT` (default `true`, keeps the script re-runnable by
cancelling a stuck run).

The script starts a run via `POST /api/pm/agent-runs` with
`{target_type: "workspace", target_id: <WORKSPACE_ID>, agent_id, additional_context}`,
then polls `GET /api/pm/agent-runs/{id}` and asserts:

1. `external_runtime == "agent-runtime"` and `external_runtime_id` set
   immediately in the launch response;
2. `workflow_id` stays null at launch, mid-run, and terminal;
3. status transitions through `running` (or `paused`) to a terminal state
   within the timeout, and the terminal state is `completed`;
4. at least one `role == "assistant"` row in
   `GET /api/pm/agent-runs/{id}/messages`;
5. `tokens_used > 0` — **the real-model canary**: zero usage means stub
   fallback or broken usage projection;
6. on `completed`, `output_summary.agent_runtime_usage_consumed == true`.

Exit code 0 = pass. Non-zero = at least one assertion failed; the failure
message names the assertion and the run ID for follow-up in the dock UI.
Each invocation creates a fresh run, so the script is safe to re-run at any
cadence.

## Validation window

**Definition of done: 20 canary runs over 5 business days (≥ 3/day), all
passing, with zero projection mismatches.**

- Run the canary at least 3× per business day (morning / midday / evening to
  cover deploys and load variance). CI cron or manual is fine; record each
  run ID and verdict in the tracking issue.
- **Zero projection mismatches** means, for every canary run:
  - the canary exits 0;
  - the dock UI for the run shows the same terminal status, transcript, and
    token totals the canary observed via the API;
  - no `agent runtime terminal usage consumption failed` or projection error
    logs in the Temporal worker for the run ID.
- Any failure **resets the window** after root-cause: fix, then restart the
  N-runs count. A failure caused purely by a staging infra outage (e.g. NATS
  restart) may be excluded with a written note, but two infra exclusions in
  one window should be treated as a real finding about projection resilience.

## Parity checks to eyeball (per run, at least once daily)

Automated assertions don't cover rendering and billing surfaces. For at least
one canary run per day, manually verify:

1. **Transcript matches the dock UI.** Open the run in the Helpin agent dock:
   the assistant messages, ordering, and any tool activity shown must match
   `GET /api/pm/agent-runs/{id}/messages` (`sequence_no` order, no duplicated
   or missing turns).
2. **Billing dashboard shows consumption.** The workspace AI usage dashboard
   must reflect the run's tokens after the terminal event. The projection
   consumes usage with idempotency key
   `workspace/agent-runtime/<run_id>/terminal-usage`, so re-delivered NATS
   events must NOT double-bill — verify the run appears exactly once.
3. **Run row hygiene.** `pause_reason` returned to `none` at terminal,
   `error_message` empty on completed runs, `completed_at` set.
4. **Usage-overage behavior** (once per window, optional): exhaust a throwaway
   workspace's credits and confirm the projection cancels the runtime run and
   stamps `execution_stage=usage_overage_cancel_requested` with the standard
   error message.

## Rollback drill

Perform this drill once during the window (not only if something breaks), so
rollback is proven, not assumed.

1. **Start one delegated run** with the canary and let it reach `running`.
2. **Flip the flag off**: set `AGENT_RUNTIME_LAUNCH_ENABLED=false` on the
   staging Helpin API and Temporal worker and restart/redeploy them. Do NOT
   stop the Temporal worker permanently — it still hosts the projection
   consumer.
3. **Verify new runs route to the Temporal executor.** Start a fresh Mira
   workspace run (the canary will now correctly FAIL its delegation
   assertion — that failure is the verification). Confirm via
   `GET /api/pm/agent-runs/{id}` that the new run has `workflow_id` set and
   `external_runtime` null, and that it progresses normally.
4. **Verify in-flight delegated runs still project to terminal.** The run
   from step 1 must continue receiving status/message/usage updates and reach
   a terminal state with `agent_runtime_usage_consumed=true`. Projection is
   keyed by `external_runtime` + `external_runtime_id` and is independent of
   the launch flag; the reconciliation sweep backstops any missed events.
5. **Re-enable** `AGENT_RUNTIME_LAUNCH_ENABLED=true`, rerun the canary once
   to confirm delegation resumes, and log the drill in the tracking issue.

If rollback is triggered for real (not a drill): leave the flag off, let
in-flight delegated runs drain to terminal via projection, and only then
triage. Never delete the `AGENT_RUNTIME_EVENTS` stream or the durable
consumer while delegated runs are in flight.
