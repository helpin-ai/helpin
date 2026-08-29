# Helpin Engineering Documentation

This page is the index for current engineering documentation. Dated files under
`docs/plans`, `docs/superpowers`, and assessment documents record design history;
they are not authoritative descriptions of current runtime behavior unless a
current document links to them explicitly.

## Core architecture

- [`AGENTS_AND_AUTOMATION.md`](AGENTS_AND_AUTOMATION.md) — agent ownership,
  execution, and automation contracts.
- [`CRM_MODULE.md`](CRM_MODULE.md) — CRM module overview.
- [`crm-signals.md`](crm-signals.md) — canonical CRM-signal
  architecture, rule catalogue, activation model, and operations.
- [`internal-tools-framework.md`](internal-tools-framework.md) — built-in tool
  contracts and execution.

## CRM references

- [`crm-signal-ingestion.md`](crm-signal-ingestion.md) —
  conversation and email signal extraction.
- [`crm-email-sync.md`](crm-email-sync.md) — CRM mailbox synchronization.
- [`crm-entity-summaries.md`](crm-entity-summaries.md) — contact, company, and
  deal summaries.

The dated
[`crm-buyer-signals-assessment.md`](crm-buyer-signals-assessment.md) is retained
for historical design rationale. Use `crm-signals.md` for current status.

## Runtime and integrations

- [`AGENT_RUNTIME_LOCAL.md`](AGENT_RUNTIME_LOCAL.md) — local Agent Runtime.
- [`AGENT_RUNTIME_STAGING.md`](AGENT_RUNTIME_STAGING.md) — staged Agent Runtime.
- [`EXTERNAL_MCP_SERVERS.md`](EXTERNAL_MCP_SERVERS.md) — outbound MCP servers.
- [`HELPIN_PUBLIC_MCP.md`](HELPIN_PUBLIC_MCP.md) — public inbound MCP surface.
- [`../events-pipeline/README.md`](../events-pipeline/README.md) — local event
  ingestion and ClickHouse operations.
- [`../events-pipeline/CAPACITY_BASELINE.md`](../events-pipeline/CAPACITY_BASELINE.md)
  — verified throughput, resource allocation, and JetStream storage sizing.
