# Community Edition distribution and release plan

Status: planned. This plan covers the first supported self-hosted Community Edition release for Helpin and Agent Runtime. It does not change the SaaS/EE deployment path, production secrets, or existing databases until the release gates below pass.

## Problem and outcome

Community code is already separated at compile time, but there is no supported clean-machine installation. Helpin's current Compose file is development infrastructure with the API and frontend commented out, and Agent Runtime's Compose examples assume that the host already provides the application and infrastructure services. Source-level community checks therefore prove that commercial code is absent, but they do not prove that a user can install, configure, upgrade, and run the product.

The release should provide one documented command path:

```text
download a versioned release bundle → create/edit one environment file → start
```

The result is a working community Helpin instance with its own Agent Runtime, durable storage, migrations, health checks, backups, and an optional coding worker. Users supply their own AI provider keys. Community usage recording remains zero-cost and does not require SaaS billing or a BYOK tariff.

## Decisions and boundaries

- Community is the default build in both repositories. EE is selected explicitly by Go build tags and the EE frontend build command.
- Community releases use prebuilt, versioned images. The first release supports Docker Compose on Linux amd64; arm64 is either added with a tested multi-architecture build or explicitly listed as unsupported. Do not advertise an architecture that has not passed the full Compose gate.
- The bundle pins image tags or digests. It never uses `latest` for application images. Infrastructure image updates are reviewed separately.
- The community stack owns Postgres, Redis, NATS, Temporal, Helpin API, Helpin Temporal worker, Helpin migrator, frontend, Runtime API, Runtime normal worker, and the Runtime coding worker behind an opt-in Compose profile. S3-compatible storage uses MinIO by default, with documented external S3-compatible storage support.
- Runtime remains a separate service and remains useful standalone. Helpin supplies tenant context, tools, credentials, and events through its configured app entry.
- Community has no Stripe, subscription entitlement, paid-tool charge, SaaS token tariff, or commercial settings route. Users can configure provider API keys and personal/workspace connections, but the community meter records normalized usage without a Helpin token charge.
- ChatGPT subscription access stays optional. The installer must not require ChatGPT flags, OAuth registration, a model-credential callback, or a BYOK tariff for a normal API-key installation.
- The coding worker is opt-in because it runs repository commands. It uses one retained workspace volume and must not mount the Docker socket or arbitrary host repositories.
- Existing SaaS Kubernetes workflows, app-config entries, database ledgers, and deployment secret names remain unchanged.

## Release layout

Create a Community release artifact in the Helpin repository (or a small release repository if the bundle must be independently versioned) containing:

```text
community/
  compose.yaml                 # complete pinned stack
  .env.example                 # documented variables and safe defaults
  apps.example.json            # generated Runtime app configuration template
  setup.sh                     # install/start/stop/restart/upgrade/logs/backup
  README.md                    # operator guide
  checksums.txt                # checksums for the bundle files
```

The release workflow publishes the application images and attaches this bundle to the matching GitHub release. The bundle version, Helpin image tag, Runtime image tag, and optional coding image tag are recorded together. A user can download a historical release and reproduce that release without relying on mutable branch files.

The setup script must be small and auditable. It may download the matching Compose/configuration files, but it must not execute unreviewed remote code or overwrite the operator's environment file. It should provide these actions:

```text
install     create directories, generate initial secrets, validate prerequisites
start       docker compose up -d
stop        docker compose down (preserve named volumes)
restart     restart the stack after configuration changes
upgrade     back up, download a selected release bundle, reconcile new variables
logs        show selected service logs
backup      dump Postgres and archive configuration metadata/volume instructions
status      show container and health status without printing secret values
```

The first release may keep backup/restore as explicit commands rather than pretending named volumes alone are a backup. The setup script must preserve `.env`, `data/`, and operator-managed app configuration during upgrades. New variables are presented in a diff file for review.

## Compose services and dependencies

The Compose file should use YAML anchors for shared environment and health settings, with explicit `depends_on` health conditions where supported:

| Service | Image/build | Responsibility | Persistent data |
| --- | --- | --- | --- |
| `helpin-db` | Postgres with pgvector | Helpin database | `helpin_db_data` |
| `runtime-db` | Postgres | Runtime database | `runtime_db_data` |
| `redis` | Redis/Valkey | Helpin queues/cache | `redis_data` |
| `nats` | Pinned NATS | Runtime/Helpin events | `nats_data` |
| `temporal` | Pinned Temporal auto-setup | Durable workflows | `temporal_data` |
| `temporal-ui` | Matching Temporal UI | Local operator visibility | none |
| `minio` | Pinned MinIO | Default S3-compatible storage | `minio_data` |
| `helpin-migrate` | Helpin community image | Core migration runner | none |
| `helpin-api` | Helpin community image | HTTP API | logs volume optional |
| `helpin-worker` | Helpin community image | Product/automation Temporal worker | logs volume optional |
| `helpin-frontend` | Helpin community image | Static web app | none |
| `agent-runtime` | Runtime default image | Runtime API | logs volume optional |
| `agent-runtime-worker` | Runtime default image | Normal native worker | logs volume optional |
| `agent-runtime-coding-worker` | Runtime coding image | Optional repository execution | `coding_workspaces` |

The coding service belongs behind `profiles: [coding]`. Its image must match the Runtime API/worker release. Its mounted root is `/tmp/agent-runtime-workspaces`; use one named volume and one replica. The normal worker must never accept the coding queue.

Use service names for internal URLs, not public DNS names. The default community app config should point Helpin to `http://helpin-api:8080/...` and Runtime to `http://agent-runtime:8090` over the Compose network. Runtime's callback URL validator permits HTTP only for localhost/127.0.0.1 today, so the community stack must either:

1. terminate TLS at a local reverse proxy and use HTTPS for the callback, or
2. add an explicit, documented development/community callback allowance for the private Compose hostname, with an authentication and network-boundary review.

Do not silently put an internal callback URL through a public hostname. Choose one path before implementation and cover it with a startup test and a clean-install test. The production SaaS callback remains HTTPS-only.

Temporal address, namespace, task queue prefix, NATS subject/stream names, and database URLs must be shared consistently among the services that use them. Helpin uses `NATS_URL`; Runtime uses `AGENT_RUNTIME_NATS_URL`.

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
| `CORS_ORIGINS`, `VITE_API_URL` | Installation URL; setup derives defaults from the selected public address. |

Runtime variables:

| Variable | Community behavior |
| --- | --- |
| `DATABASE_URL` | Runtime Postgres URL, separate from Helpin's database. |
| `AGENT_RUNTIME_STORE_DRIVER` | `postgres`. |
| `AGENT_RUNTIME_SERVICE_TOKEN` | Same value as Helpin's client token. |
| `AGENT_RUNTIME_MODEL_CREDENTIAL_ENCRYPTION_KEY` | Stable separate 32-byte raw/base64 key, generated once and shared by Runtime API and all workers. |
| `AGENT_RUNTIME_MCP_CREDENTIAL_ENCRYPTION_KEY` | Stable separate MCP credential key, generated once. |
| `AGENT_RUNTIME_APP_CONFIG` | Mounted generated app-config JSON. |
| `HELPIN_INTERNAL_API_SECRET` | Same value as Helpin's `INTERNAL_API_SECRET`. |
| `AGENT_RUNTIME_EVENT_SINK` | `nats` for the v2 event publisher. |
| `AGENT_RUNTIME_NATS_URL` | `nats://nats:4222`. |
| `TEMPORAL_ADDRESS`, `TEMPORAL_NAMESPACE`, `TEMPORAL_TASK_QUEUE_PREFIX` | Same Temporal target/prefix used by Runtime processes. |
| `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `OPENROUTER_API_KEY` | Optional Runtime defaults for standalone/community Runtime use; Helpin app-credential runs should use their accepted credentials. |
| `AGENT_RUNTIME_CHATGPT_ENABLED` | `false` by default. Enable only with a deliberate personal-connection configuration and callback validation. |

The runtime config template should include `usermaven` only for deployments that actually host Usermaven. The default Helpin bundle contains only the Helpin app entry. An operator adding a second app must add its callback/token variables and preserve its provider defaults.

## Generated Helpin app configuration

Setup should render an app config from a template rather than requiring users to hand-edit a long JSON string. The bundled community entry contains the Helpin context, MCP, skill, workspace, and event settings. Use `token_env` for every service token. `model_credential_callback` supports `url` and `token_env` only; it does not support an inline `token`.

For the private Compose network, use the selected callback policy from the dependency section above. The production SaaS form remains:

```json
{
  "app_id": "helpin",
  "event_protocol": "v2",
  "require_run_model_credentials": false,
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

The existing EE workflows continue to pass `GO_BUILD_TAGS=ee` and build `build:ee`. They must not be reused for community artifacts through an environment toggle. The frontend community build must be checked against the actual generated static artifact, not only TypeScript success.

The image build should use reproducible metadata: source commit, release version, build edition, and base-image digest. Avoid embedding secrets or a developer `.env` in any image layer. Add a CI assertion that the community image does not contain the `server/ee` tree or `frontend/src/ee` tree and that its binaries report the community edition where an edition endpoint exists.

## Clean-install acceptance test

Create a CI job that runs on a clean runner with Docker Compose and no repository-local volumes. It should use a temporary project directory and the published release bundle.

The test must:

1. Generate an environment file with deterministic test secrets and no provider API keys.
2. Run `setup.sh install` or the equivalent noninteractive installer mode.
3. Start the full default stack and wait for health/readiness of Postgres, Redis, NATS, Temporal, Helpin, Runtime, and frontend.
4. Confirm Helpin's core schema migrations complete and Runtime's Postgres schema migration completes.
5. Create a workspace and owner through the supported API/bootstrap path.
6. Verify the community settings surface has no billing route, upgrade dialog, or commercial price asset.
7. Add a test provider connection using a deterministic mock provider or a local compatible endpoint; do not make the basic CI gate depend on a paid external API.
8. Start Ask Agent, confirm the request reaches Runtime, and verify normalized usage is recorded without a Helpin credit/tariff requirement.
9. Exercise a product background workflow through Helpin's Temporal worker.
10. Restart Runtime API and normal worker and verify a durable run can resume.
11. Start the optional coding profile, run a disposable repository task, pause/resume it, restart the coding worker, and verify the named workspace volume is reused.
12. Assert no service logs secret values, and assert the default Runtime worker rejects coding queue work.
13. Tear down containers while preserving a test artifact containing sanitized service status, migration heads, image digests, and failure logs.

Use a separate pre-release smoke with real provider API keys for OpenAI, Anthropic, and OpenRouter. The real-provider test verifies the four standard profiles and actual credential routing; it is not the deterministic pull-request gate.

## Upgrade acceptance test

The upgrade job installs the previous Community release into empty volumes, creates representative data, configures an encrypted connection, runs an agent, and records migration heads and profile IDs. It then:

- backs up the database and configuration;
- downloads the new bundle and runs `setup.sh upgrade`;
- reviews and applies new variables without replacing existing secrets;
- waits for the migrator and health checks;
- verifies users, workspaces, connections, profiles, run history, checkpoints, attachments, and Temporal schedules;
- starts a new run and resumes the prior durable run;
- verifies the community artifact still has no EE routes/assets;
- verifies an interrupted upgrade can be retried without duplicating migrations.

Do not rotate `AI_CONNECTION_ENCRYPTION_KEY`, `AGENT_RUNTIME_MODEL_CREDENTIAL_ENCRYPTION_KEY`, or `AGENT_RUNTIME_MCP_CREDENTIAL_ENCRYPTION_KEY` during an ordinary upgrade. A key rotation is a separate documented migration with re-encryption and recovery checks.

## Runtime and Helpin test coverage

Extend CI with focused checks for:

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
- installation and first login;
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

1. Add/fix the community Compose stack and generated `.env.example`.
2. Add the Runtime service to the community stack and settle the private callback transport policy.
3. Add the setup/upgrade script and public operator documentation.
4. Add image builds and a bundle artifact to CI without changing EE deployment workflows.
5. Add the clean-install Compose gate.
6. Add the upgrade gate and backup/restore checks.
7. Run a real-provider pre-release smoke, including all four standard profiles and an optional coding task.
8. Publish a release candidate and test it on a clean host outside the CI runner.
9. Publish the first stable Community release.

Each stage should be independently reviewable. Do not publish a community release from a branch whose EE build or staging deployment merely happens to pass; the acceptance artifact must be the community bundle itself.

## Rollback and support policy

Before every upgrade, retain a database backup, the previous bundle, image digests, and the environment/configuration revision. A failed application rollout can return to the previous bundle if migrations have not changed the schema. After a migration, use a forward fix or a coordinated database restore; reverting containers alone is not a complete rollback. Restore operations must account for external side effects and must not automatically replay old agent runs.

The release notes should identify the exact supported upgrade path and any migrations that require downtime. A community installation should report its Helpin, Runtime, Compose bundle, and schema versions in a safe diagnostic command without printing secrets.

## Completion criteria

The change is ready for publication when:

- a clean host can install with one environment file and no private registry access;
- the exact published bundle passes health, migration, product, AI, restart, and optional coding checks;
- a previous release upgrades while preserving encrypted credentials and durable history;
- community source, binaries, images, frontend assets, and routes contain no EE/billing code;
- external provider keys are optional at startup and configuration errors are actionable at use time;
- the default stack has documented backups, stable key handling, and safe upgrade behavior;
- the EE staging/production workflows remain unchanged and continue to build EE explicitly.

## Reference implementation patterns

Plane's Community distribution is a useful packaging reference: its release provides a setup script, Compose file, environment file, install/start/stop/upgrade/backup actions, and versioned prebuilt images. It preserves the operator environment file while presenting new upgrade variables for review. See the [Docker Compose installation guide](https://developers.plane.so/self-hosting/methods/docker-compose) and [community upgrade guide](https://developers.plane.so/self-hosting/manage/upgrade-plane).
