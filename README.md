# Helpin

Helpin brings customer support, CRM, project management, knowledge, and AI-assisted
work into one platform. This monorepo contains the web app, Go API and workers,
embeddable chat SDK, help center, and native support apps.

## License

The community application is **AGPL-3.0-only**. Code in `server/ee/` and
`frontend/src/ee/`, and any other directory named `ee`, is covered by the
[Helpin Enterprise License](ee/LICENSE): internal development, testing, and
evaluation are permitted; production use requires a commercial agreement.
The public SDKs and embedded widget packages are **Apache-2.0**. See
[LICENSE](LICENSE) for exact scopes and third-party exceptions, and
[CONTRIBUTING.md](CONTRIBUTING.md) for contribution terms.

## Community 0.1 beta

The self-hosted support beta includes visitor identification and chat, a staff
inbox, public help-center articles, and agents using your own AI connections.
Start with the [Community installation guide](community/README.md), including
public DNS/HTTPS setup, application SMTP, backups, and [known limitations](ROADMAP.md).
The release candidate is local and unpublished until its
[publication and architecture gates](community/PUBLICATION.md) pass.
There is no telemetry endpoint or Helpin account requirement by default.

## Get started

- [Local development](docs/development.md) — prerequisites, infrastructure, and app setup.
- [Documentation](docs/README.md) — architecture, integrations, operations, and product design.
- [Contributing](CONTRIBUTING.md) — making changes and running checks.

## Repository structure

| Directory | Contents |
| --- | --- |
| `frontend/` | Main React application |
| `server/` | Go API, migrations, and workers |
| `packages/` | Shared libraries, widget components, and JavaScript SDK |
| `widget/` | Standalone widget bundle |
| `help-center/` | Public help center |
| `apps/` | Admin, desktop support, mobile support, and email notice apps |
| `events-pipeline/` | Event capture, delivery, and analytics ingestion |
| `website/` | Marketing website |
| `docs/` | Engineering guides, PRDs, plans, and design references |
| `docker/`, `k8s/`, `ops/` | Infrastructure and deployment configuration |

## Community and SaaS editions

Community builds use workspace AI profiles with customer credentials or configured
local models. SaaS builds explicitly select the EE edition and include commercial
billing. See [AI connections and profiles](docs/ai-connections.md) for configuration,
edition commands, and migration requirements for existing installations.

Agent execution uses the separate Agent Runtime service. Helpin owns workspace
configuration, authorization, triggers, and product workflows; Agent Runtime owns
execution. Start with the [local runtime guide](docs/AGENT_RUNTIME_LOCAL.md) and
[agent architecture](docs/AGENTS_AND_AUTOMATION.md).

## Working on widgets

The embedded SDK, app preview, and standalone widget have different build paths.
See [widget architecture and builds](docs/widget-architecture.md) before changing
shared widget components or deploying SDK assets.
