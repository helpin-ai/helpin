# Community beta scope and known limitations

This page describes what Community 0.2 includes, what is outside the bundle,
the limitations we track, and what is planned next. It applies to the Community
bundle; the release notes of your bundle record changes.

## What Community 0.2 includes

Community 0.2 includes every product surface: support chat with visitor
identification, the shared inbox, and the public help center; Docs; Projects;
CRM; automation; and agents, including coding agents. All modules are enabled
by default: `support,docs,agents,pm,crm,automation`.

New in 0.2, external A2A agents connect under **Settings → External agents**.
They need `EXTERNAL_A2A_ENCRYPTION_KEY`, which the installer generates; see
[configuration](docs/community/configuration.md).

## Narrow the enabled modules

Operators can narrow the surface with `HELPIN_ENABLED_MODULES`; Support
requires Docs. For only the support surface, set
`HELPIN_ENABLED_MODULES=support,docs,agents`. Shared customer services remain
available to Support either way, and some agent APIs, including coding-session
routes, are classified under Agents.

Subscription billing, payment UI, the analytics collector, ClickHouse, and the
desktop, mobile, admin, and email notice apps are not part of the Community
bundle.

## Known limitations

- Retrieved knowledge is marked as untrusted reference data. Prompt-injection
  defenses remain layered: keep tools scoped and approvals enabled; trust marking
  is not a guarantee of model resistance.
- Contact deletion anonymizes linked support identity and revokes visitor sessions.
  Messages, comments, subjects, and attachments are retained, including personal
  details written in them. Linked conversations become read-only. This is not
  complete erasure: imports, model/runtime transcripts, external systems, and
  backups require separate retention and deletion procedures.
- Agent profiles do not configure every AI feature. Knowledge embeddings need
  separate server configuration and 1,536-dimensional vectors. Without it,
  retrieval falls back to keyword matching. Triage and help-center AI answers
  retain separate server-level provider settings.
- SMTP supports application mail. Support email replies and inbound delivery
  still require the optional Postmark integration.
- The support-only SDK does not collect analytics events. ClickHouse and an
  analytics collector are not part of the bundle.
- One Runtime worker runs chat, coding, and Python execution. Its `community`
  image omits Node, Go, and browser automation. See
  [optional Ask Agent execution](community/README.md#optional-ask-agent-execution)
  for isolation limits and [deployment](docs/community/deployment.md#services-and-data)
  for the image contents.

These are tracked limitations, not a claim that an untested deployment is ready
for public traffic. Follow the release gate and published patch notes.

## Planned work

Community packaging maintainers plan tested upgrade paths between published
releases, automated public-edge/ACME fixtures, cached SDK loader upgrade
compatibility, Postmark support-delivery fixtures, and a signing-secret setup
UI. The installation guides already include backup/restore and external proxy
instructions. The operator CLI implements backup, restore, and
compatibility-gated upgrades; full-stack upgrade evidence and published
compatible releases remain separate requirements. These items are plans, not
evidence of shipped features or completed deployment checks.

## Implementation references

[Community module defaults](https://github.com/helpin-ai/helpin/blob/develop/server/internal/deployment/defaults_community.go),
[API module classification](https://github.com/helpin-ai/helpin/blob/develop/server/internal/deployment/modules.go),
[Compose defaults](community/compose.yaml), and
[provider wiring](https://github.com/helpin-ai/helpin/blob/develop/server/cmd/api/main.go).
