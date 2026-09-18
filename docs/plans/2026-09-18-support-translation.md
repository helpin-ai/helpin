# Support translation implementation

Status: implemented on `feat/support-translation`; not merged or deployed.

Deliver automatic translation of visible incoming public messages and automatic outgoing translation on Send, with no preview step. Use the existing economical DeepSeek route (and a configured Luna route when OpenRouter is unavailable); optional Jev reviews outgoing text. Add production metrics and Grafana panels for outcomes, latency, token usage, cost, cache and review results.

## Invariants

- Received/sent message content remains canonical. Translations and original outgoing drafts are separate, private artifacts.
- Source content hashes identify revisions (including changes made outside the application); a stale artifact cannot render or send as current.
- Workspace/member language preferences and conversation overrides are persisted. Missing LLM configuration disables translation clearly; missing Jev permits automatic translation with deterministic validation and records an unchecked-review outcome.
- Translation uses the governed AI completion boundary and its own economical route/usage feature. Jev decisions use the shared bounded, metered infrastructure.
- Incoming receipt never waits for translation. Visible-message requests share cached artifacts and bounded admission.
- Outgoing artifact ownership, source hash, language, expiry and review are checked server-side. Persisting the sent message consumes the artifact atomically; repeat send cannot create another reply. Existing channel/recipient/attachment/permission guards remain.
- No internal note, draft or assessment is exposed through the public widget. Deleted/anonymized conversations and messages invalidate/remove artifacts.
- Preserve links, code and identifiers; failures retain original text/drafts. No live customer/provider calls for verification.

## Required verification

Backend and browser checks cover source/target languages, original toggle, errors/unavailable providers, missing Jev, stale inputs, foreign workspace/actor, duplicate send, chat/email delivery, deletion, keyboard, narrow layout and light/dark themes. Verify dashboard provisioning and real recorded telemetry (including failed attempts), bounded labels, usage accounting and alert expressions. Run migrations on disposable PostgreSQL, targeted race tests, production builds for both editions, vet and module tidy. Commit the completed implementation; merge/deploy only on request.

## Verification results

Community and Enterprise Go suites, targeted race tests, and both editions’ Go
vet/build checks passed. Production frontend builds passed for both editions,
using the existing widget build artifact. The standard prebuild wrapper could
not rebuild widget-core because its linked dependencies lack Vite; the frontend
build script itself completed successfully.

The 73 focused frontend tests and five deterministic Chromium scenarios passed,
including normal/keyboard Send, language changes, incoming original toggle,
failed-send draft preservation and messaging with no provider. Desktop/mobile
light/dark screenshots were captured and the mobile light screenshot inspected;
the linked local dependency layout blocked the Inter font, so screenshots used
the browser fallback font.

Disposable PostgreSQL checks applied the migration twice and verified concurrent
send consumption, changed-intent rejection, tenant foreign keys, edited-message
cleanup and anonymization during generation. Dashboard JSON, alert YAML and all
12 metric references validated locally. Documentation checks passed. No live
model calls, deployment, dashboard provisioning or linguistic-quality evaluation
was performed by this change.
