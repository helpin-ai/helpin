# Community beta scope and known limitations

This page describes what Community 0.1 includes, what is outside its supported
surface, the limitations we track, and what is planned next. It applies to the
published Community bundle; the release notes of your bundle record changes.

## What Community 0.1 includes

The first release focuses on support chat, visitor identification, and public
help-center articles, with agents assisting that workflow. The default enabled
modules are `support,docs,agents`.

## Outside the default surface

PM and CRM navigation and automation builders are outside the default modules.
This is a deployment surface choice, not removal of their code: shared customer
services remain available to Support, and some agent APIs, including
coding-session routes, are classified under Agents. Coding workflows are outside
this release's support-focused scope.

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

These are tracked limitations, not a claim that an untested deployment is ready
for public traffic. Follow the release gate and published patch notes.

## Planned for 0.2

Community packaging maintainers plan tested upgrades from 0.1, automated
public-edge/ACME fixtures, cached SDK loader upgrade compatibility, Postmark
support-delivery fixtures, and a signing-secret setup UI. The 0.1 guide already
includes explicit backup/restore and external proxy instructions. These items
are plans, not evidence of shipped features or completed deployment checks.

## Implementation references

[Community module defaults](https://github.com/helpin-ai/helpin/blob/develop/server/internal/deployment/defaults_community.go),
[API module classification](https://github.com/helpin-ai/helpin/blob/develop/server/internal/deployment/modules.go),
[Compose defaults](community/compose.yaml), and
[provider wiring](https://github.com/helpin-ai/helpin/blob/develop/server/cmd/api/main.go).
