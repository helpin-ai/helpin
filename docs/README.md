# Helpin documentation

Start here to run Helpin, understand the code, or contribute a change. Current
technical guides are versioned in this repository. A hosted product help center
for tutorials is planned; until it is live, use the installation and support
guides below.

## Choose your starting point

| What you want to do | Start here |
| --- | --- |
| Try the self-hosted Community edition | [Install Community](../community/README.md) |
| Diagnose an installation problem | [Troubleshoot Community](community/troubleshooting.md) |
| Set up a development environment | [Local development](development.md) or [build the complete Community stack](community/development.md) |
| Understand the services and data flow | [Architecture overview](../ARCHITECTURE.md) |
| Configure AI connections | [AI connections and profiles](ai-connections.md) |
| Submit a fix or improve a guide | [Contributing](../CONTRIBUTING.md) and [documentation conventions](documentation-guide.md) |
| Ask a question or report an issue | [Getting help](../SUPPORT.md) |

- [Community CLI](community/cli.md): guided installation, configuration, and service management.

## Agents and automation

- [Agents and automation](agents-and-automation.md): ownership, triggers, runs, and execution boundaries.
- [How automation works](automation-product-model.md): the relationship between flows, agents, and activity.
- [How the agent dock works](agent-dock.md): chat sessions and their underlying agent runs.
- [How coding agents execute work](coding-agent-execution.md): repository workspaces and product delivery.
- [Support execution through Agent Runtime](support-agent-runtime.md): support conversation processing and retained backend responsibilities.
- [Support AI human control](support-ai-human-control.md): human control over support automation.
- [Internal tools](internal-tools-framework.md): tool contracts and execution.
- [Set up Agent Runtime locally](agent-runtime-local-setup.md): connect Helpin to the separate execution service.

## Customer relationships and signals

CRM and PM are included in the Community default modules; operators can disable
them with `HELPIN_ENABLED_MODULES`.

- [CRM architecture and data model](crm-overview.md): the CRM module and its connections to other Helpin modules.
- [CRM signals](crm-signals.md): implemented detection, scoring, activation, and operations.
- [Customer-work blueprint](crm-customer-work-blueprint.md): Signals/Review and Playbooks product scope.
- [Playbook automation](crm-playbook-automation-change-proposal.md): the implemented connection between Playbooks, Flows, and Beacon skills; deployment is a separate concern.
- [Signal ingestion](crm-signal-ingestion.md): conversation and email extraction.
- [Email synchronization](crm-email-sync.md): CRM mailbox synchronization.
- [Entity summaries](crm-entity-summaries.md): contact, company, and deal summaries.

The [buyer-signals assessment](crm-buyer-signals-assessment.md) preserves historical
design rationale. Use the current signals guide for behavior and operating instructions.

## Integrations and shared components

- [Connect external MCP servers](external-mcp-servers.md): make selected external tools available to Helpin agents.
- [Connect external clients to Helpin MCP](public-mcp-server.md): expose authorized Helpin capabilities to outside clients.
- [Public MCP engineering notes](public-mcp-engineering-notes.md): design decisions and implementation lessons.
- [GitLab integration](gitlab-integration.md): organization access-token connections and workspace repositories.
- [Widget architecture and builds](widget-architecture.md): embedded SDK, app preview, and standalone bundles.
- [Widget messenger security](widget-messenger-security-integration-guide.md): visitor identity and integration boundaries.
- [Widget feature parity](widget-feature-parity.md): historical March 2026 comparison, not the current capability reference.
- [Email architecture](email-architecture.md): shared message processing and delivery.
- [AI usage metering](ai-usage-metering.md): usage accounting.
- [Event pipeline](../events-pipeline/README.md) and [capacity baseline](../events-pipeline/CAPACITY_BASELINE.md): capture, delivery, and analytics ingestion.

## Operations and design references

These collections have different purposes. Plans and specifications describe intent
at the time of writing; they are not proof that a feature has shipped. Check a
page's status and the corresponding current guide before using its instructions.

| Collection | What you will find |
| --- | --- |
| [Product requirements](prds/README.md) | Problems, desired behavior, and product contracts |
| [Implementation plans](plans/README.md) | Delivery steps, progress notes, and implementation audits |
| [Design specifications](specs/README.md) | Feature and interaction designs |
| [Operations](ops/README.md) | Migration, deployment, and infrastructure procedures |
| [Research](research/README.md) | Investigations and comparative assessments |
| [Strategy](strategy/README.md) | Product direction and business planning |
| [Mockups](mockups/README.md) | HTML design references |
| [Manual testing](testing/README.md) | Browser test pages |
| [Notifications](notifications/README.md) | Notification design research |
| [Roles and permissions](rbac/README.md) | Access-control analysis |
| [Customer lifecycle campaigns](customer-io/README.md) | Campaign configuration and data contracts |


## Publication boundary

This index includes internal operations, strategy, and historical design material.
It is not a list of pages to import into the public help center. The
[publication review](publication-review.md) records material requiring a disposition
decision before public export. A passing documentation check confirms names and
links, not publication readiness.
