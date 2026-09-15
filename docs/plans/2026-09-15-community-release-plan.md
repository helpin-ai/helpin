# Community Edition distribution and release plan

Status: planned; revised September 15 to consolidate infrastructure while preserving the existing product experience. This plan covers the first supported self-hosted Community Edition release for Helpin and Agent Runtime. It does not change the SaaS/EE deployment path, production secrets, or existing databases until the release gates below pass.

## Problem and outcome

Community code is already separated at compile time, but there is no supported clean-machine installation. Helpin's current Compose file is development infrastructure with the API and frontend commented out, and Agent Runtime's Compose examples assume that the host already provides the application and infrastructure services. Source-level community checks exercise commercial-code separation, but they do not prove that a user can install, configure, upgrade, and run the product.

The release should provide one documented command path:

```text
download a versioned release bundle → create/edit one environment file → start
```

The result is a working community Helpin instance with its own Agent Runtime, durable storage, migrations, health checks, backups, and an optional coding worker. Users supply their own AI provider keys. Community usage recording remains zero-cost and does not require SaaS billing or a BYOK tariff.

## Decisions and boundaries

- Community is the default build in both repositories. EE is selected explicitly by Go build tags and the EE frontend build command.
- Community releases use prebuilt, versioned images. The first release supports Docker Compose on Linux amd64; arm64 is either added with a tested multi-architecture build or explicitly listed as unsupported. Do not advertise an architecture that has not passed the full Compose gate.
- The bundle pins image tags or digests. It never uses `latest` for application images. Infrastructure image updates are reviewed separately.
- Keep NATS, Temporal, and Redis in the supported bundle. Use one Postgres server with separate databases and database users for Helpin, Runtime, and Temporal (including Temporal visibility). Keep the Helpin worker and Runtime normal worker on the existing durable execution path. Runtime coding is an opt-in worker; Temporal UI and database admin UIs are optional. S3-compatible storage uses MinIO by default, with an external storage option.
- Runtime remains a separate service and remains useful standalone. Helpin supplies tenant context, tools, credentials, and events through its configured app entry.
- Community has no Stripe, subscription entitlement, paid-tool charge, SaaS token tariff, or commercial settings route. Users can configure provider API keys and personal/workspace connections, but the community meter records normalized usage without a Helpin token charge.
- ChatGPT subscription access stays optional. Normal API-key installation requires neither ChatGPT flags nor OAuth registration nor a BYOK tariff. Verify callback requirements for API-key credential refresh/reconnection and provide the authenticated callback wiring where required.
- The coding worker is opt-in because it runs repository commands. It uses one retained workspace volume and must not mount the Docker socket or arbitrary host repositories.
- Existing SaaS Kubernetes workflows, app-config entries, database ledgers, and deployment secret names remain unchanged.

## Minimum supported setup: preserve features, consolidate services

The goal is the smallest supported installation that preserves the existing Helpin experience. Retain the services that current product behavior depends on instead of introducing a new local scheduler or disabling features to reduce container count.

- **NATS stays:** Helpin's event integration and Runtime's v2 event publisher require it. Use JetStream with persistent storage.
- **Temporal stays:** Helpin uses it for product workflows, and its central agent launcher currently requests durable Runtime execution. Retain the existing API/worker split, run recovery, approvals, and coding queue behavior.
- **Redis stays:** Helpin can start without Redis, and single-instance WebSockets and some caches have local fallbacks. However, support email fallback workers require Redis, explicit support email delivery can reject without it, widget/help-center rate limits and help-center answer budgets fail open, and the help-center auto-index backfill trigger skips execution. It is not merely a scaling cache. Keep it for the supported full experience.
- **One Postgres server:** provision independent Helpin, Runtime, Temporal, and Temporal visibility databases/users. Preserve database boundaries; do not combine their schemas or migration ledgers. Initialize and migrate each through its own supported tooling, including on an existing volume.
- **Optional admin UIs and coding:** Temporal UI and pgAdmin are not required for operation. Start the coding worker only when repository execution is enabled.
- **Object storage:** keep MinIO for complete local attachments/artifacts, or use an external S3-compatible service. Do not silently disable uploads merely to remove its container.

Runtime supports SQLite in code, but its current Docker binaries use CGO-disabled builds while the SQLite driver requires CGO. SQLite also does not remove the Postgres server Helpin already needs. Do not add SQLite packaging, lightweight launch configuration, in-process coding admission, or lightweight crash reconciliation to this distribution plan. Those remain possible standalone Runtime work.

Redis-free operation may be documented later as a reduced feature configuration after a complete audit. It is not the default release acceptance target.

Evidence: Helpin `server/cmd/api/main.go` (optional Redis connection and conditional email fallback workers), `server/internal/service/support_delivery.go` (email validation), `server/internal/middleware/widget_rate_limit.go`, `server/internal/middleware/helpcenter_rate_limit.go`, `server/internal/service/helpcenter_ai_search.go`, and `server/internal/service/agent.go` (durable launch mode); Runtime `cmd/agent-runtime/main.go`, `internal/engine/execution_policy.go`, `internal/store/gorm.go`, and `Dockerfile`.

## First-install prerequisites

### Frontend uses the operator's origin

Build the Community frontend with a relative `/api` base. Its nginx routes `/api/` to `http://helpin-api:8080/api/` and preserves WebSocket upgrades and streaming responses. This is the selected solution; do not introduce runtime JavaScript configuration injection.

`VITE_API_URL` is a build-time variable today, not an operator runtime setting. Remove it from the installation environment contract. Audit every absolute-host derivation, including `ChatGeneralTab.tsx`, widget installation snippets, WebSocket URL construction, redirects, and shared support-core clients. Resolve a relative API base against the browser origin when an absolute URL is required. Test access from a different hostname without rebuilding the image.

Use the workspace-aware pnpm Community build in the frontend image; the current isolated `npm ci` Dockerfile is not the release build contract. Browser traffic to Helpin's API uses the same origin. Keep backend origin validation configured for the installation URL, and document separate-origin requirements for widgets and direct object uploads.

### Local signup does not require email verification

Product decision: the local Community bundle allows normal password signup and login without sending or requiring an email verification message. Reuse the ordinary organization/workspace onboarding flow to create the first owner. No separate bootstrap-owner command is needed.

Resolve an explicit server-owned verification policy through the existing edition wiring. EE selects verification required and cannot be weakened by the Community local setting. Community supports an operator setting, defaulting to required when unspecified; the local bundle explicitly disables it. Establish the exact configuration name during implementation and add it to the environment example. Do not infer this policy from a nil email client, a failed send, request headers, or a caller flag.

Keep signup, verification tokens, email delivery, and enforcement implementation in shared auth code. Only the edition's policy selection belongs in EE versus Community wiring; shared code consumes the resolved policy without importing EE. This preserves opt-in verification for public self-hosted Community deployments and avoids duplicating auth behavior. The frontend consumes the effective server policy rather than assuming every Community deployment disables verification.

Apply the policy consistently to signup, login/session handling, API authorization, verification/resend endpoints, and frontend onboarding/banners. Expose the effective requirement through the existing public auth configuration path, or one small configuration field if needed. A local account admitted without verification is not proof of mailbox ownership: do not fabricate an `EmailVerifiedAt` timestamp or weaken OAuth identity/linking rules. Audit all `email_verified` checks, including existing local unverified users.

Test signup through workspace creation and Ask Agent with no Postmark credentials; assert no verification token/email is produced in disabled mode. Test that required mode still enforces verification even when email delivery is unavailable.

Postmark is currently the only outbound email implementation. No-email local operation must clearly identify unavailable invitations, password reset, support email delivery, and sender-domain onboarding before implying an email was sent. Reuse any existing non-email invitation path only after checking its authorization and expiry behavior. Password changes by authenticated users remain separate from email-based password recovery. SMTP and a new general mail abstraction are outside this release.

For deterministic delivery tests, add a narrowly scoped injectable HTTP endpoint/client to the existing Postmark adapter and run a local Postmark-compatible fixture in CI. Keep the production endpoint default; this is a test seam, not another supported mail provider. Test the no-email local setup separately from the opt-in Postmark flows.

### Licensing is a publication prerequisite

Neither repository currently has a root license for its application code. The owner must select the license for Helpin Community and Agent Runtime, and define the EE source/distribution boundary. Add the selected root LICENSE files, applicable EE notices, and third-party notices before public artifacts are published. Do not infer a license from dependency licenses. Packaging work can proceed while this decision is pending; public release cannot.

### Public help-center is included

Add the existing `help-center/` application as `helpin-helpcenter`, using its Node server image and runtime `INTERNAL_API_URL`. Serve its browser API requests through its own origin and document the public help-center host/port and tenant mapping. Verify actual publication, article rendering, search, assets, and embedded widget URLs on an operator hostname.

Audit its built-in SaaS URL fallbacks and build-time widget variables as well as the main frontend. The released Community help-center must not send requests to Helpin SaaS or require rebuilding to configure the operator host. Use existing runtime host/proxy configuration where available; add only the configuration needed by this service.

## Not in this release

- ClickHouse and `events-pipeline`: no containers, ingestion endpoint, or `CLICKHOUSE_DSN` in the default bundle. ClickHouse-backed CRM behavioral signals/event analytics are unavailable; ordinary CRM features and other signal sources are not categorically disabled.
- Google OAuth login: optional configuration, disabled when unconfigured; password signup is sufficient for first use.
- SMTP or provider-neutral outbound mail: deferred. Postmark remains optional for email-dependent features.
- SQLite Runtime packaging, lightweight launch/recovery changes, and merged API/coding execution: deferred as described above.
- arm64 artifacts: unsupported for v1 unless the image build and full acceptance suite are made architecture-aware and pass.
- Automated upgrade/backup orchestration in `setup.sh`: deferred; v1 ships explicit tested operator commands.

## Release layout

Keep the release bundle in the Helpin repository under `community/`. The Helpin release workflow takes an explicitly pinned Runtime image version/digest as an input and validates that pair. Publish:

```text
community/
  compose.yaml                 # complete pinned stack
  .env.example                 # documented variables and safe defaults
  apps.example.json            # generated Runtime app configuration template
  setup.sh                     # install/start/stop/logs/status
  README.md                    # operator guide
  checksums.txt                # checksums for the bundle files
```

The release workflow publishes the application images and attaches this bundle to the matching GitHub release. The bundle version, Helpin image tag, Runtime image tag, and optional coding image tag are recorded together. A user can download a historical release and reproduce that release without relying on mutable branch files.

The v1 script provides only these actions:

```text
install     create directories, generate initial secrets, validate prerequisites
start       docker compose up -d
stop        docker compose down (preserve named volumes)
logs        show selected service logs
status      show container and health status without printing secret values
```

Ship upgrade, backup, restore, and configuration-change restart procedures as explicit Docker Compose and database commands in the operator guide. Do not automate variable diffing or schema rollback in Bash. Publish new-variable notes and an updated example for each release; preserve the operator's environment file and volumes. Test these same documented procedures in CI.

## Compose services and dependencies

The Compose file should use YAML anchors for shared environment and health settings, with explicit `depends_on` health conditions where supported:

| Service | Image/build | Responsibility | Persistent data |
| --- | --- | --- | --- |
| `postgres` | Postgres with required extensions, including pgvector | Separate Helpin, Runtime, Temporal, and visibility databases/users | `postgres_data` |
| `redis` | Pinned Redis | Helpin support delivery, limits, caches, and realtime coordination | `redis_data` |
| `nats` | Pinned NATS | Runtime/Helpin events | `nats_data` |
| `temporal` | Pinned Temporal with validated schema initialization | Durable workflows | Separate databases in `postgres_data` |
| `temporal-ui` (optional) | Matching Temporal UI | Local operator visibility | none |
| `minio` | Pinned MinIO | Default S3-compatible storage | `minio_data` |
| `helpin-migrate` | Helpin community image | Core migration runner | none |
| `helpin-api` | Helpin community image | HTTP API | logs volume optional |
| `helpin-worker` | Helpin community image | Product/automation Temporal worker | logs volume optional |
| `helpin-frontend` | Helpin community frontend image | Static web app and same-origin API proxy | none |
| `helpin-helpcenter` | Helpin public help-center image | Public documentation server and API proxy | none |
| `agent-runtime` | Runtime default image | Runtime API | logs volume optional |
| `agent-runtime-worker` | Runtime default image | Normal native worker | logs volume optional |
| `agent-runtime-coding-worker` | Runtime coding image | Optional repository execution | `coding_workspaces` |

Use a single Temporal server for Helpin and Runtime with explicit namespaces and task queues. Validate its schema/visibility setup against the shared Postgres version. Run Helpin migrations as a one-shot service; retain Runtime's own schema initialization. Backups cover every database. Do not place PostgreSQL, NATS, or Redis on public host ports by default.

The coding service belongs behind `profiles: [coding]`. Its image must match the Runtime API/worker release. Its mounted root is `/tmp/agent-runtime-workspaces`; use one named volume and one replica. The normal worker must never accept the coding queue.

Use Compose service names for internal traffic. App-config context, MCP, skill, workspace, and artifact URLs already accept HTTP through the general URL validator. Their `http://helpin-api:8080/...` URLs do not need a new TLS proxy.

The model-credential callback has a separate HTTPS/localhost-only validator. Commit to a narrow explicit allowance for this one callback: add a Runtime flag defaulting off plus an exact trusted HTTP callback host/port allowlist. Both are operator-owned startup configuration; the bundle enables only `helpin-api:8080`. Establish and document the final variable names in implementation.

Keep token authentication and all existing URL-shape checks, reject arbitrary hosts even when enabled, and prevent redirects from bypassing the destination restriction or forwarding credentials. Test opt-out rejection, opted-in success, wrong host/port rejection, and redirect handling. Public deployment defaults remain HTTPS-only. No extra mandatory TLS container is needed for the internal callback.

Temporal address, namespace, task queue prefix, NATS subject/stream names, and database URLs must be shared consistently among the services that use them. Helpin uses `NATS_URL`; Runtime uses `AGENT_RUNTIME_NATS_URL`.

The Postgres image must supply pgvector. Helpin API and worker already run `CREATE EXTENSION IF NOT EXISTS vector`; the release must ensure required extensions (including vector, pgcrypto, and pg_trgm where used) exist before the one-shot migrator reaches dependent SQL. Add an idempotent prerequisite step rather than editing historical migrations. For external Postgres, document extension installation by the database administrator and the role/permissions needed to enable extensions in Helpin's database. Fail preflight with an actionable error when unavailable.

Pin an actual Temporal version/digest rather than the development Compose `latest`. Validate its schema and visibility tooling against the selected PostgreSQL major version, including PG17 if that is chosen. Record the tested version pair in the bundle.

## Community environment contract

The generated `.env.example` must group variables by owner and explain whether each is generated, required, or optional. It must contain names only and safe placeholders, never working credentials.

Helpin variables:

| Variable | Community behavior |
| --- | --- |
| `DATABASE_URL` | Helpin Postgres URL, generated by Compose defaults or external override. |
| `JWT_SECRET` | Required stable secret; setup generates it once. |
| `INTERNAL_API_SECRET` | Required stable secret shared with Runtime as `HELPIN_INTERNAL_API_SECRET`. |
| `AI_CONNECTION_ENCRYPTION_KEY` | Stable 32-byte raw/base64/hex key for Helpin connections; setup generates once. |
| `OPENAI_API_KEY` | Optional provider key for user/workspace configuration and standard Large profile. |
| `ANTHROPIC_API_KEY` | Optional provider key for standard Flagship profile. |
| `OPENROUTER_API_KEY` | Optional provider key for standard Small/Medium profiles. |
| `AGENT_RUNTIME_BASE_URL` | Defaults to `http://agent-runtime:8090`. |
| `AGENT_RUNTIME_SERVICE_TOKEN` | Stable token matching Runtime. |
| `AGENT_RUNTIME_APP_ID` | `helpin`. |
| `AGENT_RUNTIME_EVENT_PROTOCOL` | `v2` for the bundled versions. |
| `AGENT_RUNTIME_LAUNCH_ENABLED` | `true` for normal operation. |
| `NATS_URL` | `nats://nats:4222`. |
| `TEMPORAL_ADDRESS` | `temporal:7233`. |
| `TEMPORAL_NAMESPACE` | A dedicated default namespace for this installation. |
| `TEMPORAL_TLS_ENABLED` | `false` for the private default Compose network; document TLS for external Temporal. |
| `REDIS_URL` | Compose Redis URL. |
| `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | MinIO defaults or external S3 credentials. |
| `AWS_S3_ENDPOINT_URL`, `AWS_S3_BUCKET_NAME`, `AWS_REGION` | MinIO defaults or external S3 settings. |
| `CORS_ORIGINS` | Backend origin validation uses the operator's public origin; the main UI calls same-origin `/api`. |
| `POSTMARK_APP_SERVER_TOKEN`, `POSTMARK_APP_FROM_EMAIL` | Optional product email configuration; not required for local signup/login. |
| `POSTMARK_REPLY_SERVER_TOKEN`, `POSTMARK_REPLY_FROM_EMAIL` | Optional support outbound email configuration; document any additional sender/domain prerequisites. |
| `GOOGLE_AUTH_CLIENT_ID`, `GOOGLE_AUTH_CLIENT_SECRET` | Optional Google login; document redirect configuration and hide the login option when unconfigured. |

Add the new explicit local verification policy variable after implementation; disabled in the local bundle and required under existing SaaS defaults. `VITE_API_URL=/api` belongs to the Community image build, not this runtime environment file.

The help-center service receives `INTERNAL_API_URL=http://helpin-api:8080/api` at runtime. Document its public host/tenant configuration and widget origin separately from internal API addressing.

Runtime variables:

| Variable | Community behavior |
| --- | --- |
| `DATABASE_URL` | Runtime database/user in the shared Postgres server, separate from Helpin's database. |
| `AGENT_RUNTIME_STORE_DRIVER` | `postgres`. |
| `AGENT_RUNTIME_SERVICE_TOKEN` | Same value as Helpin's client token. |
| `AGENT_RUNTIME_MODEL_CREDENTIAL_ENCRYPTION_KEY` | Stable separate 32-byte raw/base64 key, generated once and shared by Runtime API and all workers. |
| `AGENT_RUNTIME_MCP_CREDENTIAL_ENCRYPTION_KEY` | Stable separate MCP credential key, generated once. |
| `AGENT_RUNTIME_APP_CONFIG` | Generated JSON through the supported loader; do not pass a file path as though it were JSON. |
| `HELPIN_INTERNAL_API_SECRET` | Same value as Helpin's `INTERNAL_API_SECRET`. |
| `AGENT_RUNTIME_EVENT_SINK` | `nats` for the v2 event publisher. |
| `AGENT_RUNTIME_NATS_URL` | `nats://nats:4222`. |
| `TEMPORAL_ADDRESS`, `TEMPORAL_NAMESPACE`, `TEMPORAL_TASK_QUEUE_PREFIX` | Same Temporal target/prefix used by Runtime processes. |
| `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `OPENROUTER_API_KEY` | Optional Runtime defaults for standalone/community Runtime use; Helpin app-credential runs should use their accepted credentials. |
| `AGENT_RUNTIME_CHATGPT_ENABLED` | `false` by default. Enable only with a deliberate personal-connection configuration and callback validation. |
| `AGENT_RUNTIME_MCP_ALLOW_HTTP` | `false` by default for user-added MCP servers. |
| `AGENT_RUNTIME_MCP_ALLOW_PRIVATE_NETWORKS` | `false` by default for user-added MCP servers. |
| `AGENT_RUNTIME_MCP_ALLOWED_HOSTS` | Empty means no extra hostname allowlist restriction; scheme and private-network rules still apply. Operators may restrict this to explicit approved hosts. |

These MCP variables govern run-scoped/user-added MCP attachments, not app-config providers or model-credential callbacks. Document LAN access: allow private networks and set approved hosts; also allow HTTP only when the selected server needs it. Do not switch them all on just to connect Helpin's built-in MCP provider. Add the separate proposed callback opt-in/allowlist variables once implemented.

The runtime config template should include `usermaven` only for deployments that actually host Usermaven. The default Helpin bundle contains only the Helpin app entry. An operator adding a second app must add its callback/token variables and preserve its provider defaults.

## Generated Helpin app configuration

Setup should render an app config from a template rather than requiring users to hand-edit a long JSON string. The bundled community entry contains the Helpin context, MCP, skill, workspace, and event settings. Use `token_env` for every service token. `model_credential_callback` supports `url` and `token_env` only; it does not support an inline `token`.

For the private Compose network, generate service-name HTTP URLs with the narrow callback exception above. The following HTTPS example documents the equivalent public-host form:

```json
{
  "app_id": "helpin",
  "event_protocol": "v2",
  "require_run_model_credentials": true,
  "context_endpoint": "https://<helpin-host>/api/internal/agent-runtime/target-context",
  "context_token_env": "HELPIN_INTERNAL_API_SECRET",
  "model_credential_callback": {
    "url": "https://<helpin-host>/api/internal/agent-runtime/model-credentials/refresh",
    "token_env": "HELPIN_INTERNAL_API_SECRET"
  },
  "mcp_providers": [
    {
      "name": "helpin",
      "transport": "http",
      "url": "https://<helpin-host>/api/internal/agent-runtime/mcp/helpin",
      "token_env": "HELPIN_INTERNAL_API_SECRET",
      "tool_namespace": "none",
      "refresh_interval": "30s",
      "startup_policy": "required",
      "unknown_refresh_cooldown": "30s"
    }
  ],
  "skill_provider": {
    "transport": "http",
    "base_url": "https://<helpin-host>/api/internal/agent-runtime/skills",
    "package_base_url": "https://<helpin-host>/api/internal/agent-runtime/skill-packages",
    "token_env": "HELPIN_INTERNAL_API_SECRET",
    "package_token_env": "HELPIN_INTERNAL_API_SECRET"
  },
  "workspace_provider": {
    "transport": "repository",
    "base_url": "https://<helpin-host>/api/internal/agent-runtime/workspace",
    "token_env": "HELPIN_INTERNAL_API_SECRET",
    "root_dir": "/tmp/agent-runtime-workspaces"
  },
  "browser": {
    "enabled": true,
    "allowed_domains": [],
    "artifact_provider": {
      "transport": "http",
      "upload_endpoint": "https://<helpin-host>/api/internal/agent-runtime/artifacts",
      "token_env": "HELPIN_INTERNAL_API_SECRET"
    }
  }
}
```

Community must not set `allowed_domains: ["*"]` by default. Browser access should be disabled or explicitly configured by the operator. The generated template must not replace other app entries when an operator supplies an existing config.

## Build and image pipeline

Add a Community release workflow alongside the existing EE staging/production workflows. It should:

1. Determine a release version from the repository tag or an explicit workflow input.
2. Run the canonical source checks:
   - `scripts/check-community-backend.sh`
   - `scripts/check-community-frontend.sh`
   - community frontend artifact scan
   - Go vet/build for community binaries
   - manifest/config validation
3. Build and publish Helpin API, Helpin worker/migrator, and frontend images from the community build with no `GO_BUILD_TAGS=ee`.
4. Build and publish Runtime default and coding images from the matching Runtime release.
5. Produce the Compose bundle with immutable image tags/digests and the matching Runtime version.
6. Run the disposable Compose acceptance job against the exact published image references.
7. Attach the bundle, checksums, release notes, and a software bill of materials to the GitHub release only after acceptance passes.

The server Dockerfile currently hardcodes `GOARCH=amd64`; v1 builds/publishes amd64 explicitly. Before publishing arm64, wire Docker `TARGETARCH` through every relevant Go binary build and test all runtime dependencies and final images on that architecture. Do not label amd64 binaries as multi-architecture images.

The existing EE workflows continue to pass `GO_BUILD_TAGS=ee` and build `build:ee`. They must not be reused for community artifacts through an environment toggle. The frontend community build must be checked against the actual generated static artifact, not only TypeScript success.

The image build should use reproducible metadata: source commit, release version, build edition, and base-image digest. Avoid embedding secrets or a developer `.env` in any image layer. Add a CI assertion that the community image does not contain the `server/ee` tree or `frontend/src/ee` tree and that the deployed community router exposes no EE routes and uses community service wiring. `router/edition.go` defines a route-registration interface, not itself a public edition-reporting endpoint; verify the actual response contract before adding any edition-reporting assertion. The migrate image intentionally includes `clickhouse-migrate` and `reindex-helpcenter-search`; their presence does not enable those services or indicate an EE leak.

## Clean-install acceptance test

Create a CI job that runs on a clean runner with Docker Compose and no repository-local volumes. It should use a temporary project directory and the published release bundle.

The test must:

1. Generate an environment file with deterministic test secrets and no provider API keys.
2. Run `setup.sh install` or the equivalent noninteractive installer mode.
3. Start the supported default stack with one Postgres server and without admin UIs or the coding worker; wait for health/readiness of Postgres, Redis, NATS, Temporal, Helpin, Runtime, and frontend.
4. Confirm Helpin's core schema migrations complete and Runtime's Postgres schema migration completes.
5. Sign up through the normal local password flow, log in, and complete organization/workspace onboarding without email verification or Postmark. Verify the UI and API honor the configured policy.
6. Verify the community settings surface has no billing route, upgrade dialog, or commercial price asset.
7. Add a test provider connection using a deterministic mock provider or a local compatible endpoint; do not make the basic CI gate depend on a paid external API.
8. Start Ask Agent, confirm the request reaches Runtime, and verify normalized usage is recorded without a Helpin credit/tariff requirement.
9. Exercise a product background workflow through Helpin's Temporal worker.
10. Exercise Redis-backed support email delivery against the Postmark-compatible CI fixture after the adapter test seam exists; separately assert actionable behavior with email unconfigured.
11. Verify widget/help-center rate limiting and help-center answer budgets.
12. Verify an attachment/artifact round trip through the bundled object store, including a browser-accessible presigned URL.
13. Publish a help-center article and load/search it from the operator-facing help-center origin.
14. Test the same prebuilt frontend on a different hostname, including API calls, WebSockets, widget preview/snippets, and redirects.
15. Restart Runtime API and normal worker and verify a durable run can resume.
16. Start the optional coding profile, run a disposable repository task, pause/resume it, restart the coding worker, and verify the named workspace volume is reused.
17. Assert no service logs secret values, and assert the default Runtime worker rejects coding queue work.
18. Tear down containers while retaining sanitized status, Helpin ledger head, Runtime image digest, Temporal schema versions, and failure logs.

Use a separate pre-release smoke with real provider API keys for OpenAI, Anthropic, and OpenRouter. The real-provider test verifies the four standard profiles and actual credential routing; it is not the deterministic pull-request gate.

## Upgrade acceptance test

The upgrade job installs the previous Community release into empty volumes, creates representative data, configures an encrypted connection, runs an agent, and records Helpin's migration ledger head, Runtime image version/digest, Temporal schema versions, and profile IDs. Runtime's Postgres startup uses explicit `MigratePostgres` SQL; GORM AutoMigrate applies to its SQLite path, which this bundle does not use. Runtime has no equivalent migration head. Retrying a partial Runtime schema upgrade must still be tested, not assumed trivial. It then:

- backs up the database and configuration;
- downloads the new pinned bundle and runs the documented Compose upgrade commands;
- reviews and applies new variables without replacing existing secrets;
- waits for the migrator and health checks;
- verifies users, workspaces, connections, profiles, run history, checkpoints, attachments, and Temporal schedules;
- starts a new run and resumes the prior durable run;
- verifies the community artifact still has no EE routes/assets;
- verifies Helpin ledger/checksum behavior and Runtime SQL schema convergence after an interrupted upgrade, using a real Postgres database.

For the first public release, use a populated release-candidate fixture and backup/restore checks until a previous public version exists. State the supported starting schema explicitly.

Do not rotate `AI_CONNECTION_ENCRYPTION_KEY`, `AGENT_RUNTIME_MODEL_CREDENTIAL_ENCRYPTION_KEY`, or `AGENT_RUNTIME_MCP_CREDENTIAL_ENCRYPTION_KEY` during an ordinary upgrade. A key rotation is a separate documented migration with re-encryption and recovery checks.

## Runtime and Helpin test coverage

Extend CI with focused checks for:

- same-origin API/WebSocket routing, help-center host mapping, and widget URL generation;
- local signup with verification disabled and SaaS verification enforcement under email failure;
- app-config parsing and generated token-env resolution;
- private Compose callback transport and startup validation;
- community edition service wiring with no EE imports;
- community provider placeholders and standard profile provisioning;
- no-billing frontend route registration and artifact content;
- migration idempotency on empty and populated databases;
- migration upgrade from the previous released schema;
- API/worker/Runtime health and event protocol compatibility;
- normal versus coding task queue admission;
- run pause/resume after API and worker restarts;
- backup/restore documentation commands;
- installer behavior when Docker is missing, ports are occupied, or an existing `.env` is present.

Keep the existing Runtime Go tests, Helpin community backend check, community frontend check, EE tests, and mobile/desktop checks. The new Compose jobs validate integration boundaries that unit tests cannot cover.

## Documentation and operator experience

Add a public Community Edition section to Helpin's README and a dedicated `docs/community/` guide covering:

- supported architecture and resource requirements;
- installation and first login without email verification in the local bundle;
- optional Postmark/Google login configuration and email-dependent feature limits;
- public help-center host mapping and publication;
- provider key configuration;
- changing provider keys and why encryption keys must remain stable;
- external Postgres/Redis/NATS/Temporal/S3 configuration;
- optional coding worker and its trust boundary;
- backups and restore limits;
- upgrades and new-variable review;
- logs, health checks, and troubleshooting;
- how to build from source with community defaults;
- how to opt into EE separately, without implying that setting an environment variable unlocks it.

The operator guide should use the same Compose bundle tested by CI. Do not copy a development-only Compose file into the public instructions.

## Security and licensing checks

- Scan the final images and bundle for credentials, local paths, `.env` files, and commercial source.
- Keep generated secrets out of logs and shell history where practical; setup should write them to a protected environment file with restrictive permissions.
- Use non-root application users in all application containers.
- Do not mount the Docker socket, host repositories, or the host filesystem.
- Make browser domains explicit and empty by default; document the implications of enabling browser access.
- Pin image and infrastructure versions and publish checksums.
- Verify the community distribution contains only the intended open-source license and notices. Commercial code and pricing assets must be absent from the source archive and images.
- Run the existing dependency/license/security scanners on the final release bundle.

## Delivery sequence

0. Record the owner's license choices and add root licenses/EE boundaries in both repositories before public publication; local packaging work need not wait.
1. Implement the Community relative API base/proxy and audit absolute URL consumers. Add explicit local email-verification policy and normal first-owner onboarding with no mail service. These are first-install blockers.
2. Assemble the shared-Postgres Compose stack, including public help-center, NATS, Temporal, Redis, and storage. Ensure extensions and all schemas initialize in order on empty and existing volumes. Add the narrowly gated model-callback HTTP allowance.
3. Add the five-command installer and operator documentation for manual upgrade/backup/restore, optional email/OAuth, LAN MCP servers, and excluded services. Add the small Postmark test seam.
4. Add Community image builds and a bundle workflow in Helpin with a pinned Runtime version input, explicit amd64 builds, and final-image content checks. Multi-architecture publication requires TARGETARCH plumbing and matching acceptance.
5. Add clean-install acceptance, including browser host portability, email-free signup, help-center publication, separate workflow/delivery/limits/storage checks, and coding.
6. Add upgrade and backup/restore gates using Helpin ledger state, Runtime image/schema verification, and Temporal schema versions.
7. Run real-provider and optional real-Postmark pre-release smoke tests; the default installation still needs no email provider.
8. Publish a release candidate after the licensing gate and test it on a clean host outside CI.
9. Publish the first stable Community release only after the exact bundle passes.

Each stage should be independently reviewable. Do not publish a community release from a branch whose EE build or staging deployment merely happens to pass; the acceptance artifact must be the community bundle itself.

## Rollback and support policy

Before every upgrade, retain a database backup, the previous bundle, image digests, and the environment/configuration revision. A failed application rollout can return to the previous bundle if migrations have not changed the schema. After a migration, use a forward fix or a coordinated database restore; reverting containers alone is not a complete rollback. Restore operations must account for external side effects and must not automatically replay old agent runs.

The release notes should identify the exact supported upgrade path and any migrations that require downtime. A community installation should report its Helpin, Runtime, Compose bundle, and schema versions in a safe diagnostic command without printing secrets.

## Completion criteria

The change is ready for publication when:

- a clean host can install with one environment file and no private registry access, rebuild, or email provider;
- local signup has no verification gate; SaaS verification remains enforced;
- the main frontend and included public help-center work on the operator hostname;
- both repositories have owner-selected distribution licenses before publication;
- the exact published bundle passes health, migration, product, AI, restart, and optional coding checks;
- a previous release upgrades while preserving encrypted credentials and durable history;
- community source, binaries, images, frontend assets, and routes contain no EE/billing code;
- external provider keys are optional at startup and configuration errors are actionable at use time;
- the default stack shares one Postgres server, retains NATS/Temporal/Redis, makes admin UIs and coding optional, and has documented backups, stable key handling, and safe upgrade behavior;
- the EE staging/production workflows remain unchanged and continue to build EE explicitly.

## Reference implementation patterns

Plane's Community distribution is a useful packaging reference: its release provides a setup script, Compose file, environment file, install/start/stop/upgrade/backup actions, and versioned prebuilt images. It preserves the operator environment file while presenting new upgrade variables for review. See the [Docker Compose installation guide](https://developers.plane.so/self-hosting/methods/docker-compose) and [community upgrade guide](https://developers.plane.so/self-hosting/manage/upgrade-plane).
