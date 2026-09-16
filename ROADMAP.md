# Community beta scope and known limitations

The first release focuses on support chat, visitor identification, and public
help-center articles. Agents assist that workflow. PM, CRM navigation, automation
builders, and coding are outside the default Community surface.

Known limitations to address:

- Retrieved knowledge is marked as untrusted reference data. Prompt-injection
  defenses remain layered: keep tools scoped and approvals enabled; trust marking
  is not a guarantee of model resistance.
- Contact deletion is not a complete personal-data erasure workflow across all
  messages, imports, logs, and backups.
- Agent profiles do not configure every AI feature. Knowledge embeddings need
  separate server configuration and 1,536-dimensional vectors. Without it,
  retrieval falls back to keyword matching. Triage and help-center AI answers
  retain separate server-level provider settings.
- SMTP supports application mail. Support email replies and inbound delivery
  still require the optional Postmark integration.
- The support-only SDK does not collect analytics events. ClickHouse and an
  analytics collector are not part of the bundle.

Planned for 0.2 (Community packaging maintainers): tested upgrades from 0.1,
automated public-edge/ACME fixtures, cached SDK loader upgrade compatibility,
Postmark support-delivery fixtures, and a signing-secret setup UI. The 0.1 guide
includes explicit backup/restore and external proxy instructions.

These are tracked limitations, not a claim that an untested deployment is ready
for public traffic. Follow the release gate and published patch notes.
