# Support setup through the UI and external assistants

Status: implemented and verified locally. Shared Community and hosted product behavior.

## Goal and accepted design

Let support administrators continue the existing setup journey in Helpin or with an external assistant using MCP and browser handoffs. Both paths inspect current product evidence. Browser capability belongs to the external client, and MCP credentials never authorize a different browser identity. Keep existing setup actions, permissions, defaults, billing checks, and live activation behavior.

## Capability audit

The setup catalog already defines email, verified widget installation, help docs, synced knowledge, AI activation, team inboxes, and routing. Configuration screens and validation exist. Public MCP supports conversation operations and document creation but has no support configuration/readiness operation. SetupService.Get also initializes goals and achievements; a read-only MCP inspection must use evidence reads without those writes. Existing routes remain available. No schema migration or new CLI executable is needed; expose the new read operation through the existing REST adapter for scripts.

## Implementation

- [x] Add tested, read-only support setup contract using existing evidence and catalog, permission-filtered direct links, per-step verification guidance, and explicit distinction between configuration and end-to-end testing. Do not store a second progress model or claim customer-facing readiness from counts.
- [x] Expose the contract through authenticated UI and public MCP/REST, with workspace binding, support-admin authorization, ordinary MCP policy/scope/module enforcement, strict argument validation, and deployment wiring.
- [x] Add a concise support setup assistant entry point with connection URL, copyable workspace-specific prompt, current readiness, reconnect/permission guidance, manual browser fallback, and refresh/resume behavior. Existing UI flow remains immediately usable.
- [x] Add a portable support-onboarding skill and document actual scope, browser identity/authorization, secrets handoff, and test/activation sequence.
- [x] Test readiness transitions, permissions, routes/schema, UI connection states, copying, error/retry and refresh. Verify backend/frontend in Community and hosted editions, documentation checks, and diff review. Commit on waqar-fixes; no push requested.

## Verification

Targeted Go service and publicapi tests; frontend setup component/page tests; both TypeScript edition configurations; Go vet/build as practical. No live mailbox connections, messages, or customer-facing AI activation during development. Review the final diff for unintended capability changes and secrets.

## Results

Shared support evidence now powers the authenticated setup dialog and the
workspace-bound `get_support_setup` operation / public REST adapter. The workflow
package registers a portable skill and `setup_customer_support` MCP prompt.
Permission checks, current-state transitions, workspace binding, grant filtering,
read-only database access, UI errors/copying/refresh, and prompt discovery are covered.

Validation: Community frontend 20 passed; hosted frontend 19 passed and one
edition-specific skip. Both TypeScript configurations and both Go edition builds
passed. Targeted setup/repository/public API tests passed in both editions; the
broader MCP regression suite passed after allowing local httptest sockets. Go vet,
documentation naming/link checks, documentation unit tests (11), skill validation,
and the secret scan passed. Self-review preserved all existing configuration
entry points and defaults. No production mailbox, website installation, OAuth
connection, publishing, test send, or live AI activation was performed.
