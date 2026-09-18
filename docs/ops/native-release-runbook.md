# Native runtime and AI profiles release runbook

> Historical release-preparation snapshot. Commit IDs, branch publication status,
> and validation results below describe that preparation session, not current
> branch or deployment state. Recheck the selected release before using this
> checklist. [AI connections](../ai-connections.md) describes the host contract.


Status: preparation only. Staging and production configuration, inventories, and live canaries remain to be verified. This document does not authorize deployment or disposal of existing runs.

SDK `v0.6.0` is already published at `37fb03755c8108444730953f100b7a6783285173`. Runtime release preparation is at `a1e1e38`; Helpin code preparation is at `531572a46`. Both consume the stable SDK. Runtime and Helpin have not been pushed to `develop`. Keep the SDK tag unchanged; deployment images have their own release versions.

At the final working-tree check, Runtime also contained `f3f1be6` (raising the native tool-step default to 200). The earlier release validation predates that commit. Include it in the release review and run the appropriate checks before merging; do not treat the earlier test results as validation of a later head.

## 1. Before merging either repository

1. Hold deployment synchronization for both applications in the target environment. Pushes to `develop` start staging release workflows; pushes to `main` start production release workflows. CI and release are separate workflows, so publishing an image does not establish that CI passed. Arrange an Argo sync hold or an equivalent controlled maintenance window before pushing. Include the separately published frontend in the release coordination.
2. Record the current API, normal worker, coding worker, Helpin worker, frontend, and migrator versions; current secret/config revisions; database recovery points; and existing coding-volume recovery arrangements. Take recoverable backups of both databases before cutover writes.
3. Populate the environment-specific secret stores described below. Confirm synchronization to `helpin-secrets` in namespace `helpin` and `agent-runtime-secrets` in namespace `agent-runtime`. Check presence and secret version without displaying values. New env values take effect only when processes restart.
4. Prepare the Helpin app-config change without replacing other app entries. Keep strict credentials and ChatGPT off during the first deployment stage. Preserve the deployed event protocol until both host and runtime can be switched together with launches paused.
5. Run the read-only inventories against the actual staging databases, then repeat independently for production. Runtime inventory covers every app, including Usermaven. Review stored legacy OAuth connections and imported runtime-specific skills as well as active runs.
6. Confirm storage capacity and scheduling for the coding worker. Both manifests now include one coding replica, a 20Gi ReadWriteOnce PVC, and a Recreate update strategy. Its repository root must be inside `/tmp/agent-runtime-workspaces`; UID/GID 1000 must be able to write there. The PVC preserves checkouts for continuation, not a database snapshot or a universal cache of installed tools. Choose placement appropriate for repository command execution; the worker is not an OS sandbox.
7. Check current branch divergence and CI again before merging. If more code is merged, validate the resulting commits. Record the exact release commits and images rather than assuming the preparation hashes remain the deployment heads.

Local verification already covered the SDK, Runtime Go suite/build/vet, Helpin EE Go suite/build/vet, PostgreSQL migration cases, community frontend build/artifact check, and focused frontend fixes. The full EE frontend run had two stale assertions; both were fixed and their affected suites passed. These checks do not establish staging secret readiness, live credential refresh, or production migration state.

## 2. Helpin environment

Apply the same application configuration to the API and every Helpin worker. The migration job uses the same target database and the EE migration binary. Keep existing database, Temporal, Redis, NATS, storage, email, Stripe, ClickHouse, and other service configuration unless separately changing it.

| Variable | Required setting and timing |
| --- | --- |
| `AI_CONNECTION_ENCRYPTION_KEY` | Stable key for encrypted connections, required for EE API/worker startup even with ChatGPT disabled. Accepts 32 raw bytes, 64 hex characters, or base64 encoding of 32 bytes. Prefer base64. Reuse the existing correct key if encrypted connections already exist. |
| `OPENROUTER_API_KEY` | Helpin-owned managed connection for Small and Medium. Must exist in Helpin, even if Runtime already has this key. |
| `OPENAI_API_KEY` | Helpin-owned managed connection for Large. |
| `ANTHROPIC_API_KEY` | Helpin-owned managed connection for Flagship. |
| `AGENT_RUNTIME_BASE_URL` | API base URL reachable from Helpin API and workers; use the deployed Runtime API root. |
| `AGENT_RUNTIME_SERVICE_TOKEN` | Must equal Runtime's service token. |
| `AGENT_RUNTIME_APP_ID` | `helpin`, unless this environment deliberately uses another ID. Must exactly match the Runtime app entry. |
| `AGENT_RUNTIME_EVENT_PROTOCOL` | Final target `v2`, matching the app's `event_protocol`. Preserve an existing matching protocol during the staggered deployment; coordinate any switch before reopening launches. |
| `AGENT_RUNTIME_LAUNCH_ENABLED` | `true` for normal operation. `false` disables new execution; it does not select a fallback executor. Maintenance still needs ingress/schedule controls so requests do not just create failures. |
| `INTERNAL_API_SECRET` | Existing Helpin internal authentication secret. Runtime's callback/tool token environment variable must hold this same secret. |
| `CHATGPT_CONNECTIONS_ENABLED` | Start `false`. Set `true` when enabling and validating personal ChatGPT access after the base deployment. |
| `CHATGPT_OAUTH_CLIENT_ID` | Optional. Leave unset to use the SDK public client ID, unless intentionally using a separate registered client. |
| `NATS_URL` | Existing Helpin NATS connection, connected to the event bus used by Runtime. Verify JetStream/event delivery permissions and connectivity. |

The API, worker, and migration images must be built with `GO_BUILD_TAGS=ee`; the frontend with `pnpm --dir frontend run build:ee`. The staging and production workflows now do this. EE is a build choice, not an environment variable that turns a community binary into SaaS.

Use package entrypoints for manual EE builds/runs:

```bash
# From the Helpin repository root
cd server
GOWORK=off go run -tags ee ./cmd/api
# In a separate process, only when manually operating the worker:
GOWORK=off go run -tags ee ./cmd/temporal-worker
```

Do not use `go run cmd/api/main.go`: it excludes the sibling edition wiring files. Do not start these manual processes alongside the Kubernetes deployment. `GOWORK=off` ensures Go uses the module's published dependency versions rather than a local SDK checkout.

There is no additional AI-profiles feature flag or `ai-bootstrap` step. Migrations and application startup provision the four standard profiles and managed connections. All three Helpin provider keys are needed for the full default profile set; existing EE provider validation may also prevent startup when a configured completion route has no provider.

## 3. Agent Runtime environment

Apply these consistently to the API, normal worker, and coding worker. Preserve Usermaven's app configuration and global provider defaults.

| Variable | Required setting and timing |
| --- | --- |
| `AGENT_RUNTIME_MODEL_CREDENTIAL_ENCRYPTION_KEY` | Stable key, identical across API and every worker. Accepts 32 raw bytes or base64 encoding of 32 bytes; 64-character hex is not supported here. Separate from Helpin's encryption key. |
| `AGENT_RUNTIME_APP_CONFIG` | Reviewed app configuration, inline JSON/YAML or `@/absolute/path/apps.yaml`. If using a file, mount it in all three deployments; setting an `@` path does not create the mount. |
| `HELPIN_INTERNAL_API_SECRET` | Suggested token-env alias containing Helpin's `INTERNAL_API_SECRET`. An existing alias is fine if the app config references it. This is distinct from the Runtime service token. |
| `AGENT_RUNTIME_SERVICE_TOKEN` | Same value as Helpin's `AGENT_RUNTIME_SERVICE_TOKEN`; preserve other authorized host clients. |
| `AGENT_RUNTIME_CHATGPT_ENABLED` | Start `false`; enable on API and every worker for the ChatGPT validation phase. |
| `AGENT_RUNTIME_STORE_DRIVER` | `postgres` in staging/production; already explicit in the manifests. |
| `DATABASE_URL` | Runtime database, not Helpin database. All Runtime processes must share it. |
| `AGENT_RUNTIME_EVENT_SINK` | Must include `nats` or `jetstream` for v2 publication, e.g. `nats`. Preserve other intentional sinks. |
| `AGENT_RUNTIME_NATS_URL` | Runtime's NATS connection. The Runtime variable is **not** `NATS_URL`. Preserve existing stream, subject, and permission configuration. |
| `TEMPORAL_ADDRESS`, `TEMPORAL_NAMESPACE` | Existing Runtime Temporal target, shared by API and both worker types. Keep the production Temporal architecture for this release. |
| `TEMPORAL_API_KEY`, `TEMPORAL_TLS_ENABLED`, `TEMPORAL_TLS_SERVER_NAME` | Preserve the existing authentication/TLS settings where applicable. |
| `TEMPORAL_TASK_QUEUE_PREFIX` | Preserve the same existing prefix across Runtime API and both workers. An inconsistent prefix makes the coding poller invisible to admission. |
| `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `OPENROUTER_API_KEY` | Keep existing Runtime provider keys for Usermaven and standalone/default-key integrations. Helpin will use credentials attached to its runs. |
| `AGENT_RUNTIME_MCP_CREDENTIAL_ENCRYPTION_KEY` | Preserve this separate existing key for generic MCP credentials. It does not replace the model-credential key. |

If a new encryption key is needed, generate 32 random bytes once and save them directly in the target secret manager, using base64 encoding. Do not regenerate a key on each restart or replace a key that already protects stored connections. Use separate keys per service and environment.

Provider readiness based on environment keys is a startup snapshot. Restart after key changes; capabilities alone do not prove a provider credential can make a successful inference request.

## 4. Runtime app configuration for Helpin

Merge the following fields into the existing Helpin entry. This fragment is **not** a replacement for the complete multi-app configuration:

```yaml
apps:
  - app_id: helpin
    # Keep false during transition; make true after the new Helpin canaries.
    require_run_model_credentials: false
    model_credential_callback:
      url: https://<helpin-api-host>/api/internal/agent-runtime/model-credentials/refresh
      token_env: HELPIN_INTERNAL_API_SECRET
```

Use the actual API hostname for the environment, reachable by Runtime workers. The callback requires HTTPS except for literal localhost/127.0.0.1 development URLs. A plain HTTP Kubernetes service URL is rejected for this callback. Preserve service access through any ingress or maintenance restrictions; do not put an interactive login in front of internal callbacks.

The complete final Helpin integration should retain these existing fields. Fill missing ones using the deployed HTTPS host and existing reviewed tool/browser policy:

```yaml
apps:
  - app_id: helpin
    event_protocol: v2
    require_run_model_credentials: true
    context_endpoint: https://<helpin-api-host>/api/internal/agent-runtime/target-context
    context_token_env: HELPIN_INTERNAL_API_SECRET
    model_credential_callback:
      url: https://<helpin-api-host>/api/internal/agent-runtime/model-credentials/refresh
      token_env: HELPIN_INTERNAL_API_SECRET
    mcp_providers:
      - name: helpin
        transport: http
        url: https://<helpin-api-host>/api/internal/agent-runtime/mcp/helpin
        token_env: HELPIN_INTERNAL_API_SECRET
        tool_namespace: none
        refresh_interval: 30s
        startup_policy: required
        unknown_refresh_cooldown: 30s
    skill_provider:
      transport: http
      base_url: https://<helpin-api-host>/api/internal/agent-runtime/skills
      package_base_url: https://<helpin-api-host>/api/internal/agent-runtime/skill-packages
      token_env: HELPIN_INTERNAL_API_SECRET
    workspace_provider:
      transport: repository
      base_url: https://<helpin-api-host>/api/internal/agent-runtime/workspace
      token_env: HELPIN_INTERNAL_API_SECRET
      root_dir: /tmp/agent-runtime-workspaces
```

This is a reference entry, not permission to discard current `allowed_tools`, browser/artifact settings, event callbacks, additional MCP providers, or other apps. `transport: http` denotes the HTTP transport and supports HTTPS URLs. Keep Usermaven's strict policy unset/false and its current event protocol. Model endpoint entries are only needed for approved compatible/local endpoints; the standard profiles and ChatGPT do not require them.

## 5. Maintenance, legacy inventory, and migration gates

Pause new launches, retries, and automation/scheduled starts before retiring engines. Apply this to every host still launching Codex/OpenCode, including Usermaven if applicable. Keep old workers available for a finite drain and keep Helpin callbacks reachable. Paused/approval-waiting legacy runs also count as nonterminal.

Run these read-only inventory files through an operator connection to the named database:

| Database | Inventory file |
| --- | --- |
| Runtime | `agent-runtime/ops/native-cutover-runtime.sql` |
| Helpin | `helpin/server/scripts/native-cutover-inventory.sql` |

Use the pre-cutover schema inventories before the old auth tables are dropped. Record connection metadata without token payloads. Stored legacy OAuth rows require an explicit disconnect/migration decision. Legacy Codex login is not automatically converted to a native ChatGPT connection. Review imported skills for native compatibility; changing a runtime label does not port instructions inside a package.

Build the disposition tool from the release source:

```bash
cd /root/agent-runtime
GOWORK=off go build -o /tmp/agent-runtime-cutover ./cmd/agent-runtime-cutover
```

Export remaining runs in separate operator sessions with the appropriate database secret profile:

```bash
# Session with Runtime DATABASE_URL:
/tmp/agent-runtime-cutover --database runtime > runtime-disposition.json

# Separate session with Helpin DATABASE_URL:
/tmp/agent-runtime-cutover --database helpin > helpin-disposition.json
```

`--database` selects the schema/query; it does **not** switch the connection. For explicitly reviewed runs that cannot drain, use the reviewed manifest with `--apply runtime-disposition.json` or `--apply helpin-disposition.json`, respectively. Supply the matching Temporal address, namespace, and authentication profile. Apply Runtime disposition first, then Helpin projections. This terminates workflows as well as recording failure; do not substitute a manual status update. Do not run disposal without approving the concrete affected-run list.

Re-export and require zero nonterminal Codex/OpenCode runs in both databases. Also finish or explicitly cancel older Helpin runs that rely on Runtime default credentials before enabling Helpin's strict policy. Drain Helpin runs before changing its event protocol. Stop retired workers and let activities exit before schema changes; prevent older binaries from restarting and recreating removed schema.

Apply `agent-runtime/ops/native-cutover-migrate.sql` against Runtime Postgres through the approved operator SQL runner, with stop-on-error enabled. This is a separate retirement script, **not** part of Helpin's migration command. It updates legacy agent defaults and drops the old Runtime Codex auth table only after its active-run gate passes.

Helpin's forward migrations run through its normal EE runner during the deployment below. Do not manually apply individual Helpin SQL files.

## 6. Deploy Runtime, then Helpin

1. Merge/push Runtime to `develop` only once the staging preparations and cutover gates above are satisfied. Wait for CI and the release image build; release workflow success alone is insufficient. Synchronize the Runtime release in a controlled way. Deploy API, normal worker, and coding worker from the same Runtime release. Keep Helpin strict credentials false and ChatGPT disabled at this point.
2. Check Runtime `/healthz` and `/readyz`, normal worker readiness, coding worker readiness, PVC binding, and a live coding Temporal poller. Inspect authenticated `/v1/capabilities?app_id=helpin` and `/v1/app-health?app_id=helpin` using the service token without logging it. The host MCP/skill/context services must be reachable. An old Helpin API can serve these during the transition while launches are paused; if the host is temporarily stopped, restore it before treating dependent Runtime readiness failures as deployment defects.
3. Merge/push Helpin to `develop`. Wait for CI and all required release artifacts. Confirm API, Helpin worker, and migrator are EE builds, and the frontend is the EE build. Continue holding normal launches and scheduled triggers.
4. Before allowing the Helpin Argo sync, quiesce old Helpin writers/activities for affected tables and prevent old pods from restarting during schema changes. Its `helpin-server-migrate` **PreSync** job runs before new Helpin pods replace old ones; the deployment does not itself establish a drain. Keep any broader maintenance necessary to enforce this ordering.
5. Let the EE migration job run `./migrate up`. Capture its logs while it exists; successful hooks are deleted. Require successful completion before proceeding. Restore the new Helpin API and worker, then the matching frontend. Keep the Helpin Temporal worker: it still handles automation/product workflows, while Runtime workers execute agents.
6. With launches still controlled, ensure Helpin `AGENT_RUNTIME_EVENT_PROTOCOL=v2` matches Runtime Helpin `event_protocol: v2`, if switching from v1. Restart affected processes. Do not change Usermaven's protocol as part of this switch.

For an explicitly manual migration instead of the deployment job, use the release source and injected target Helpin database configuration:

```bash
# From the Helpin repository root
cd server
GOWORK=off go run -tags ee ./cmd/migrate up
GOWORK=off go run -tags ee ./cmd/migrate validate
GOWORK=off go run -tags ee ./cmd/migrate status
```

The commands may load a local dotenv file; use a controlled operator environment with the target configuration already injected and no unrelated development dotenv file. A release migrator image is preferable for production. Do not run a second migration concurrently with PreSync.

Normal migration handles the known branch version collisions and the CRM pipeline correction. Released historical checksums are preserved; the corrected pipeline has its own version. Staging and production already at the previous `develop` migration head should use normal `up`, not manual checksum repair. The local empty legacy CRM-table rename was a developer-database repair; do not copy it into production. An unexpected migration failure requires inspecting that actual schema, not bypassing the ledger.

Expected data changes include the one-time reset of system agents and preset copies to their preset family's standard size, other custom agents to Small, and corresponding saved versions. Accepted run inputs/checkpoints remain unchanged. Subsequent profile edits remain supported. Startup fills managed connection placeholders and reuses existing managed provider connections; it is not a recurring agent-default reset.

## 7. Validate managed profiles, then enforce Helpin credentials

Use one staging workspace first. Keep general launch traffic paused while allowing deliberate canaries.

| Profile | Provider | Default model |
| --- | --- | --- |
| Small | `openrouter` | `deepseek/deepseek-v4.1-flash:nitro` |
| Medium | `openrouter` | `google/gemini-3.8-flash` |
| Large | `openai` | `gpt-5.6-terra` |
| Flagship | `anthropic` | `claude-sonnet-5` |

Small carries the configured OpenRouter quantizations `fp8`, `fp16`, `bf16`, and `fp32`. Verify actual model availability in the target deployment through canaries; this table describes repository defaults, not a provider availability guarantee.

- Settings AI shows profiles, with connected managed providers and no new duplicate Standard connections. Agent defaults are configured in agent settings. Ask Agent and new custom agents default to Small.
- Run Ask Agent and manually select each standard profile. Confirm actual provider/model and credential source using safe metadata; do not log keys or bearer tokens.
- Exercise PM/manual agent launch, an automation, and the CRM reviewed-launch/retry path. Check UI event progression and terminal status against Runtime, including ordered v2 events.
- Run a coding task in a disposable repository on the coding worker. Check read/write/tool execution, approval pause/resume, and checkout preservation across worker restart. Verify no normal worker executes coding tasks. Check terminal cleanup and preview retention according to the selected delivery policy.
- Check managed usage reservations and settlement, including retries, for missing reservations or duplicate charging. Check old histories remain readable and retired versions fail with the intended error.

Once these pass and no older Helpin credentialless runs remain, set `require_run_model_credentials: true` **only on the Helpin app** and restart Runtime API and both worker types. Run Ask Agent again. Confirm a Helpin request without required model credentials is rejected at admission before queueing, and an app-credential run does not consult global keys. Run a Usermaven default-key canary to verify its existing behavior remains available.

If these checks pass, reopen managed-profile launches and automation. OAuth/BYOK enablement can remain off without blocking the managed-profile release.

## 8. Enable SaaS BYOK and personal ChatGPT deliberately

SaaS BYOK needs a workspace policy and an explicit immutable flat token tariff in the database. It is not enabled merely by setting the ChatGPT environment flags. Decide the rate before enabling a workspace: `1000000` micro-USD per million tokens means USD 1 per million; `0` explicitly means no Helpin token fee. An unset tariff is invalid. Paid tools retain separate charges. Do not use the model's published token price as the BYOK fee.

Build the EE operator command:

```bash
# From the Helpin repository root
cd server
GOWORK=off go build -tags ee -o /tmp/helpin-ai-byok-policy ./cmd/ai-byok-policy
```

Run it from a controlled directory without a development `.env`, with target Helpin `DATABASE_URL` injected. Set `WORKSPACE_ID`, `TARIFF_VERSION`, and `RATE_MICROUSD` to the reviewed workspace UUID, immutable version, and chosen nonnegative integer rate:

```bash
/tmp/helpin-ai-byok-policy \
  -workspace "$WORKSPACE_ID" -mode enable \
  -tariff-version "$TARIFF_VERSION" \
  -microusd-per-million-tokens "$RATE_MICROUSD"
```

This previews and rolls back by default. Review its output, then repeat with `-apply` to commit. Reusing a tariff version requires identical values; a rate change needs a new version. Start with a staging canary workspace rather than enabling every workspace at once.

For ChatGPT, set Runtime `AGENT_RUNTIME_CHATGPT_ENABLED=true` and restart all Runtime processes; set Helpin `CHATGPT_CONNECTIONS_ENABLED=true` and restart API and workers. Keep the app callback, token alias, encryption keys, and strict policy configured. Create a personal ChatGPT connection, complete consent, create/select a personal profile, and launch manually through Ask Agent and another manual surface.

Verify inference with `openai_chatgpt` and app-owned OAuth credentials, expired-token refresh, reconnect, disconnect/revocation, and pause/resume. The September 13 test-system run already demonstrated inference/tool calls; it did not establish live expired-token refresh or successful completion of every delivery step. ChatGPT has ordinary transcript continuation but does not preserve the previous-response identifier for lossless provider-state replay.

Verify personal profiles appear for their owner in manual pickers, shared profiles support unattended launches, and personal ChatGPT is not silently selected for unattended execution. Test the configured fallback only for connection-unavailable/reconnect-required admission states. Authorization, policy, billing, and transport errors must not silently switch providers. Routes do not switch mid-run.

Compare normalized token usage with the frozen flat BYOK tariff and verify paid tools are accounted separately. Confirm disconnect revokes subsequent use, active connections remain decryptable after restart, and terminal Runtime credentials are cleaned up.

To disable new BYOK admissions for a workspace, preview and then apply:

```bash
/tmp/helpin-ai-byok-policy -workspace "$WORKSPACE_ID" -mode disable
# After reviewing the preview:
/tmp/helpin-ai-byok-policy -workspace "$WORKSPACE_ID" -mode disable -apply
```

This preserves existing accepted snapshots. Do not remove encryption keys or callback support to disable new admissions. Coordinate active ChatGPT runs before turning off Runtime's ChatGPT flag.

## 9. Production promotion and recovery

Promote only after staging canaries and monitoring are satisfactory. Repeat the entire environment, backup, inventory, drain, and secret/config checks against production. Staging success does not verify production state. Keep production keys separate and preserve existing ciphertext keys.

Merge/promote Runtime to `main` first under the production sync hold, verify the release, then Helpin to `main` and its EE migration/deployment. Main pushes trigger production workflows, so do not merge both blindly and rely on timing. Use the same validated source changes and matching image families; do not retag the SDK. Repeat strict-policy enablement and managed canaries before reopening launches. Enable production BYOK/ChatGPT only for the intended workspaces with the chosen tariff and completed optional gates.

Monitor launch/admission errors, credential refresh/reconnection, failed reservations, event lag, coding queue/poller health, and worker restart behavior. Save sanitized run IDs, migration results, image digests, and config revisions in the release record.

Before schema changes, reverting to the prior images/configuration is a normal release recovery option. After the native cutover drops legacy auth tables and migrates defaults, an image rollback alone is not a complete rollback. Prefer a forward fix; if restoring backups, stop writers and restore a consistent Helpin/Runtime/configuration set with an explicitly reviewed recovery window. Database restoration does not undo external tool side effects. Never automatically resume or replay retired failed runs.

## Source references

- [AI connections contract](../ai-connections.md)
- [Implementation plan](../plans/2026-09-14-ai-profiles-and-ee-billing-plan.md)
- Helpin: `server/internal/config/config.go`, `server/internal/service/ai_standard_profiles.go`, `server/cmd/ai-byok-policy/main.go`, `k8s/{stage,prod}/server-migrate.yaml`, `.github/workflows/deploy-{staging,prod}.yml`.
- Runtime: `internal/appconfig/model_credentials.go`, `internal/appconfig/config.go`, `internal/engine/event_sink.go`, `internal/temporalclient/options.go`, `ops/examples/helpin-apps.yaml`, `docs/native-cutover.md`, `ops/native-cutover-*.sql`, `k8s/components/coding-worker/`, `.github/workflows/{staging,production}-release.yml`.
