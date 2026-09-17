# Helpin Engineering Documentation

This page is the index for current engineering documentation. Dated files under
`docs/plans`, `docs/specs`, and assessment documents record design history;
they are not authoritative descriptions of current runtime behavior unless a
current document links to them explicitly.

## Start here

- [Product help docs — coming soon](https://helpin.ai/docs)
- [Getting help](../SUPPORT.md)
- [Self-hosting troubleshooting](community/troubleshooting.md)
- [Architecture overview](../ARCHITECTURE.md)
- [Community installation](../community/README.md)
- [Community source builds](community/development.md)
- [Local development](development.md)
- [Contributing](../CONTRIBUTING.md)
- [AI connections and profiles](ai-connections.md)
- [Widget architecture and builds](widget-architecture.md)
- [Email architecture](email-architecture.md)

## Publication boundary

This index includes internal operations, strategy, and historical design material;
it is not a list of pages to import into the public help center. See the
[publication review](publication-review.md) for specific findings and required
disposition decisions before public export.

## Document collections

| Collection | Purpose |
| --- | --- |
| [Product requirements](prds/README.md) | PRDs and product contracts |
| [Implementation plans](plans/README.md) | Dated plans, progress notes, and implementation audits |
| [Design specifications](specs/README.md) | Detailed feature and interaction designs |
| [Operations](ops/README.md) | Migration, deployment, and operational runbooks |
| [Research](research/README.md) | Investigations and comparative assessments |
| [Strategy](strategy/README.md) | Product direction, backlog, and campaign proposals |
| [Mockups](mockups/README.md) | Standalone HTML design references |
| [Manual testing](testing/README.md) | Browser test pages |

PRDs, plans, and specs describe intent at the time of writing. They may contain
superseded decisions; use current engineering guides and implementation for
present behavior. Filenames are retained where useful for history and search.

## Core architecture

- [`AGENTS_AND_AUTOMATION.md`](AGENTS_AND_AUTOMATION.md) — agent ownership,
  execution, and automation contracts.
- [`CRM_MODULE.md`](CRM_MODULE.md) — CRM module overview.
- [`crm-signals.md`](crm-signals.md) — canonical CRM-signal
  architecture, rule catalogue, activation model, and operations.
- [`internal-tools-framework.md`](internal-tools-framework.md) — built-in tool
  contracts and execution.

## CRM references

- [`crm-customer-work-blueprint.md`](crm-customer-work-blueprint.md) — agreed
  complete Signals/Review and Playbooks product scope, customer outcomes and UX.
- [`crm-playbook-automation-change-proposal.md`](crm-playbook-automation-change-proposal.md)
  — implemented Playbooks → Flows → Beacon + skills connection, safeguards and
  verification; the filename is retained, but the older ordered-step proposal is
  superseded. Branch implementation does not imply production activation.
- [`AUTOMATION_PRODUCT_MODEL.md`](AUTOMATION_PRODUCT_MODEL.md) — shared Flow,
  Agent and Activity mental model, including the guarded CRM connection.
- [`crm-signal-ingestion.md`](crm-signal-ingestion.md) —
  conversation and email signal extraction.
- [`crm-email-sync.md`](crm-email-sync.md) — CRM mailbox synchronization.
- [`crm-entity-summaries.md`](crm-entity-summaries.md) — contact, company, and
  deal summaries.

The dated
[`crm-buyer-signals-assessment.md`](crm-buyer-signals-assessment.md) is retained
for historical design rationale. Use `crm-signals.md` for current status.
The earlier Signals contract and design mock are also historical references;
they do not override the reconciled blueprint and connection plan. Current-state
references describe the inspected branch, not proof of production deployment.

## Runtime and integrations

- [`AGENT_RUNTIME_LOCAL.md`](AGENT_RUNTIME_LOCAL.md) — local Agent Runtime.
- [`AGENT_RUNTIME_STAGING.md`](AGENT_RUNTIME_STAGING.md) — staged Agent Runtime.
- [`EXTERNAL_MCP_SERVERS.md`](EXTERNAL_MCP_SERVERS.md) — outbound MCP servers.
- [`HELPIN_PUBLIC_MCP.md`](HELPIN_PUBLIC_MCP.md) — public inbound MCP surface.
- [`../events-pipeline/README.md`](../events-pipeline/README.md) — local event
  ingestion and ClickHouse operations.
- [`../events-pipeline/CAPACITY_BASELINE.md`](../events-pipeline/CAPACITY_BASELINE.md)
  — verified throughput, resource allocation, and JetStream storage sizing.

## Additional references

- [GitLab integration](GITLAB_INTEGRATION.md)
- [Mattermost integration](mattermost-integration.md)
- [Widget messenger security](widget-messenger-security-integration-guide.md)
- [Widget feature parity](widget-feature-parity.md)
- [AI usage metering](ai-usage-metering.md)
- [Customer lifecycle campaigns and data contracts](customer-io/README.md)
- [Notification research](notifications/README.md)
- [Role and permission analysis](rbac/README.md)
