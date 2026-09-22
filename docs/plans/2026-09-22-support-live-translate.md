# Support Live Translate implementation plan

Status: implemented and verified on `waqar-fixes`. Updated 2026-09-22.

This is the single execution plan for contributors implementing the revised support Live Translate experience. Follow it for backend, inbox, settings, and widget work; update its checkboxes and execution notes rather than creating additional planning documents. This plan supersedes conflicting behavior in the [earlier translation plan](2026-09-18-support-translation.md) and [original requirements](../prds/support-live-translate.md) for this revision. Those documents describe earlier scope, not the final decisions below.

> Execute sequentially with `superpowers:executing-plans`. Do not use subagents or create a new worktree. Work in `/root/teampulse/.worktrees/waqar-fixes` on `waqar-fixes`. Preserve unrelated changes and mockups. This document authorizes no deployment or branch promotion by itself.

**Goal:** Translate new support messages reliably when Live Translate is enabled, provide explicit translation of older messages, and make progress and recovery clear without slowing inbox or widget message loading.

**Architecture:** Persist conversation policy and message-level language/translation eligibility on the backend. Process incoming translation asynchronously after durable message acceptance; keep customer text authoritative. Outgoing sends use durable, idempotent operations with sender-private progress, then publish only the final customer-facing reply. Reuse cached results and existing delivery infrastructure.

**Stack:** Go, PostgreSQL/GORM, existing background-job/event infrastructure, React/TanStack Query, SDK TypeScript and Preact widget components, OpenRouter.

## Final product contract

### Settings and conversation controls

| Surface | Contract |
| --- | --- |
| Settings → Support → Live Translate | Exactly two controls: **Reading language** (default English) and **Live Translate on by default** (default on). |
| Default switch | Initializes new conversations only. Changing it does not reset existing conversations or process their history. Off does not disable manual Translate. |
| Removed settings | Separate incoming/outgoing switches and workspace-wide customer-language picker. Live Translate handles both directions. |
| Conversation bar | Compact, persistent below the header; remains visible while scrolling. Example: **Live Translate · 🇪🇸 Spanish ↔ 🇬🇧 English [On/off]**. |
| Conversation scope | On/off and manually selected customer language persist and are shared by teammates with access. They never modify other conversations. |
| Bar visibility | Show for differing languages, including when Live Translate is off. For uncertain language show **Customer language unknown · Choose language**. Hide for confidently same-language conversations; controls remain in the conversation menu. |
| Bar tooltip | **Automatically translates new messages and replies in this conversation.** |
| Default-switch tooltip | **Enable Live Translate for new conversations. Existing conversations keep their setting.** |
| Flags | Small decorative flags next to language names in bar and pickers. Stable shared mapping; regional flag for explicit locale such as Portuguese (Brazil). Neutral language icon for unknown/mixed/unmapped languages. Names stay visible; never infer nationality or language from a flag. No repeated flags in message footers. |
| Permissions | Settings remain support-admin editable. Conversation controls require the existing permission to modify that accessible conversation; read-only users cannot change shared policy. Enforce server-side. |

### Message eligibility and language

- Automatically translate only new public customer messages and new teammate replies while Live Translate is on, and only when source and target differ. Internal notes remain unchanged.
- Store eligibility when a message is accepted, not when a browser renders it. The first message of a newly created conversation follows its initialized policy.
- Turning Live Translate on does not translate earlier messages. Turning it off prevents new automatic work. Re-enabling does not backfill the off interval.
- Scrolling, reopening, and loading history never launch automatic translation or historical language detection. Existing saved translations may still be displayed; Show original remains available. Off does not erase previous translations.
- A historical message can be translated only through explicit **Translate**. Reuse cached results. Never use this action to update the current customer/reply language.
- Detect each new meaningful customer message once; save source language, evidence source, uncertainty, content hash and detector version. Reuse across teammates and reopenings. Content edits invalidate affected results; edits to historical/ineligible messages do not silently opt them into live translation.
- Customer reply-language priority: manual conversation selection → recent confident customer-message evidence → browser locale as a hint for live chat. No IP/country inference.
- Browser locale does not override meaningful message evidence. A selected language governs reply destination; each incoming message still uses its own actual source language for display translation.
- Short acknowledgements, emojis, names, order IDs, code-only and attachment-only messages must not overwrite an established language. Preserve nonlinguistic content. Handle mixed-language messages without inventing a confident conversation language.
- Update inferred conversation language in message chronology order, never completion order. Late processing of an older message cannot override newer evidence. Account for customer/CC sender identity; do not blindly change the primary customer's reply language from another participant's message.
- Normalize supported language/locale codes consistently across backend, prompt and picker (including Chinese variants, Portuguese variants, and Persian, which was tested but is currently absent from the application list).

### Message UI and send recovery

| Location/state | Required behavior and copy |
| --- | --- |
| Inside bubble, bottom | Small secondary text/actions: **Translate**, **Translated from Spanish · Show original**, or **Show translation**. |
| Translate tooltip | **Translate to English**, using the current reading language. The visible label remains just **Translate**. |
| Same-language message | Hide Translate only when reliably known to match reading language. Unknown-language historical messages may offer it without proactive detection. |
| Incoming/manual translation pending | Original stays readable; **Translating…** inside the bubble only once translation is known to be required. Do not mislabel cache retrieval or language detection as translation. |
| Incoming/manual translation failure | Keep original, secondary **Translation unavailable · Retry** inside bubble; no blocking banner. |
| Outgoing Send | Immediately show submitted original text/attachments as a subtly dimmed pending bubble, sender-only, and clear composer while retaining a recoverable snapshot. |
| Existing outgoing delivery-status area | **Preparing… → Translating… → Sending…**, then actual existing delivery status. Same-language sends skip Translating. Progress must reflect backend events, not timers or guesses. |
| Provider fallback | Keep Translating; no provider names or technical errors in the customer/support message flow. |
| Translation failure | Keep bubble with **Not sent · Retry · Send original** in the existing delivery-status area. Never send original automatically. |
| Ordinary delivery failure | Existing failure/retry behavior; do not offer Send original as a delivery repair action. |
| Edit failed send | Restore submitted text and attachments without overwriting a newer draft. Disable restoration until the newer draft is cleared/sent, with the reason in a tooltip; preserve the failed snapshot meanwhile. |
| Success | Reconcile pending and confirmed messages in place using one stable client/send identity. No duplicate bubble, jump, or extra notification. |
| Conversation change/reload | Preserve pending/failed operations and isolate drafts by conversation and sender. Reconnect recovers authoritative progress; no phantom indefinitely pending bubble. |

Do not repeat failure messages in the composer and thread. Keep supplementary detail in tooltips. Failures must remain legible despite secondary styling. No preview/approval step before each translated send.

### Engine, performance and boundaries

- Primary for all messages requiring translation: OpenRouter `openai/gpt-oss-120b`, pinned to Cerebras (`cerebras/fp16` endpoint observed in testing), low reasoning as tested.
- Secondary: OpenRouter `deepseek/deepseek-v4.1-flash`, pinned to CoreWeave (`coreweave/fp8` observed), reasoning disabled as tested. Confirm current provider selectors before implementation; do not silently autoroute elsewhere.
- Fallback on bounded primary timeout, retryable rate/service failure, or invalid/incomplete output. Give secondary its own usable deadline within a bounded total. Do not re-use an already-cancelled primary context. Respect cancellation of the send operation itself.
- Remove Jev from support language detection and blocking translation review, without changing unrelated Jev features. Replace the old production prompt with a validated detect/translate contract; benchmark prompts are not identical to the existing protected-token/chunk pipeline.
- Detect and translate together when source language is unknown and translation is eligible; use detection-only when required for reply-language evidence without display translation. Reuse saved detection and same-language outcomes.
- Preserve numbers, URLs, code, identifiers, formatting, meaning, negation and commitments. Validate JSON schema, source-language normalization, completion, placeholder integrity and nonempty required output. Same-language output must preserve original exactly.
- Chunk long text only for actual input/output limits, preferably at paragraph boundaries. Remove Jev-driven chunk restrictions. Handle partial failure without publishing a partial reply or mixing old/new attempts.
- Incoming receipt/acknowledgment and inbox history queries never wait for model calls. Bound concurrency and deduplicate work across readers. Persist job recovery and cooldowns; no unbounded polling or retry storms.
- Return saved language/eligibility/cache summaries with message pages in batches; avoid per-bubble database/API fetches. Lazy-load translated bodies where necessary without triggering generation.
- Separate history/manual/detection budgets from outgoing send capacity. The current shared 1,000-record/day quota must not let reading activity block replies. Keep bounded usage controls and existing metering; send-original remains available when translation cannot proceed.
- Cache by tenant, message/source hash, purpose, target and applicable pipeline version. Language evidence cache is independent of UI mounting. Persist “not needed,” unknown and failed states distinctly with appropriate retry semantics.
- Sample quality asynchronously with an independent reviewer, not per-message blocking review. Record sampling, provider/model, fallback, latency, cost, failures and queue backlog without raw message text in logs. Remove Jev-only review blocking states from this pipeline.
- No production credentials or benchmark API keys in source, docs, logs or fixtures.

### Widget scope

- Keep customer sends and original text unchanged; acceptance cannot depend on translation success.
- Use saved browser locale from SDK session creation. Refresh locale on session restore, retaining it as a hint only; old clients omitting locale remain supported.
- Deliver only the final submitted teammate reply; never stream staff originals, private pending states, failure actions or provider details to visitors. Do not add a customer Live Translate bar or staff Translate controls.
- Reconnect/history must return exactly the delivered content, without retranslation.
- Add automatic text direction to message prose and composer input for Arabic, Urdu, Hebrew, etc.; isolate code, links and identifiers, and avoid flipping the entire widget layout accidentally.
- Check SDK/shared widget and standalone widget entry points against the widget architecture guide. Apply parity changes where still supported; do not assume the standalone bundle is unused.
- Full localization of widget interface labels is explicitly out of scope. AI-generated support replies have a separate path: audit for consistency but do not add a second translation pass or runtime changes without documenting the need in this plan.

## Code map and verified starting behavior

Paths below are repository-relative. The focused live-translation modules now implement the revised behavior.

| Area | Existing entry points / proposed focused modules |
| --- | --- |
| Settings | `frontend/src/components/settings/SupportTranslationSettings.tsx`, `TranslationLanguagePicker.tsx`, `translationLanguages.ts`; `frontend/src/pages/settings/SupportTranslationSettingsPage.tsx`; `frontend/src/lib/settingsSections.tsx` |
| Inbox | `frontend/src/components/support/TranslatedMessageBubble.tsx`, `MessageBubble.tsx`, `MessageThread.tsx`, `ReplyComposer.tsx`, `SupportInboxPanelHeader.tsx`, `ConversationActionsMenu.tsx` |
| Queries and state | `frontend/src/hooks/queries/useSupportTranslation.ts`, `useOutgoingSupportTranslation.ts`, `useSupport.ts`; associated support store and realtime sync handlers |
| Proposed UI units | `frontend/src/components/support/LiveTranslateBar.tsx`; focused pending-send state module and tests alongside current support state owner; shared language presentation mapping |
| Translation backend | `server/internal/service/support_translation.go`, `support_language_detection.go`, `support_translation_generation.go`, `support_translation_text.go`; `server/internal/handler/support_translation.go` |
| Persistence/API | `server/internal/model/support_translation.go`, `support_inbox.go`; `server/internal/repository/support_translation.go`; additive migration under `server/internal/dbmigrate/sql/` using the next available migration ID |
| Message acceptance/delivery | `server/internal/service/support_inbox.go`; repository message creation, email inbound and widget entry points; existing authorized support websocket/event path |
| Routing | `server/cmd/api/main.go`; `server/internal/service/ai_completion_routes.go` and completion executors; model catalog/policy allowlists and billing integration |
| Observability | `server/internal/observability/translation.go`, existing translation dashboards and alert definitions |
| Widget | `packages/sdk-js/src/core/widget.ts`; `packages/widget-core/src/components/MessageBubble.tsx`, `ComposeBar.tsx`, widget styles; shared types; `widget/src/helpin-widget.js` |
| Widget server boundary | `server/internal/service/support_inbox_widget.go`; `server/internal/websocket/widget_handler.go`, `widget_public.go`; `server/internal/model/support_widget_public.go` |

Starting problems verified in code: viewport entry triggers incoming translation; all fetches are labelled Translating; conversation overrides are deliberately ignored; outgoing auto-translation skips optimistic insertion; backend defaults to older DeepSeek with no declared translation fallback; Jev may gate sending; browser locale is stored but unused by translation; widget restore omits locale.

## Execution checklist

Use behavior-focused tests for each backend/state change: write the regression, observe failure, implement, verify. Do not add tests that simply mirror markup. Commit coherent verified changes, maintaining this single checklist.

### 1. Baseline and contracts

- [x] Read applicable nested AGENTS files and current widget architecture/build guide. Confirm branch/status; preserve unrelated files.
- [x] Audit settings, conversation controls, manual translation API, all incoming channels, outgoing text/attachment-only sends, email delivery, realtime events, privacy deletion and AI reply paths for capability parity.
- [x] Record exact owner modules for background jobs and sender-private events in execution notes before adding new infrastructure.
- [x] Define DTOs for policy, message evidence/eligibility, translation state and pending-send progress; version compatibility for old clients. Separate no-op/manual/live request intent server-side.

### 2. Persistence and migration

- [x] Add tests for default initialization, toggles, off intervals, first-message eligibility, edit invalidation and out-of-order detection completion.
- [x] Implement persisted conversation enabled state/manual language and monotonic policy revision; snapshot eligibility atomically with message acceptance. A single latest-enabled timestamp alone is insufficient to preserve off intervals.
- [x] Store message evidence/hash and durable work/send identity; reuse existing tables/queue where practical. Add indexes for batched history metadata and recovery.
- [x] Specify migration precedence: explicit legacy conversation off wins; preserve workspace-disabled behavior and existing explicit choices. Never promote previously disabled functionality silently. For asymmetric legacy incoming/outgoing settings, conservatively initialize affected existing conversations off and record the migration outcome.
- [x] Preserve explicit legacy language choices as conversation overrides when needed; new conversations default to detection. No message backfill. Retain valid caches and original content.
- [x] Test migration twice on disposable PostgreSQL, old/new client compatibility, concurrent acceptance/toggle, tenant isolation, edit/deletion/anonymization cleanup and rollback behavior.

### 3. Model routing and language processing

- [x] Add route tests proving Cerebras first and CoreWeave second, required provider pinning, reasoning per route, validation fallback and independent timeout budgets.
- [x] Implement routes in both API wiring and completion policy so preferred-route overrides cannot bypass the agreed chain. Verify billing/metering and both supported edition paths.
- [x] Replace Jev detection/review calls only within support translation. Implement normalized detect/translate/no-op contract, protected-text validation and necessary-only chunking.
- [x] Add cases for English no-op, language switches, mixed text, short/unknown messages, browser hints, manual override, Persian/Chinese variants, long emails and invalid output. Recheck the production protected-token prompt with synthetic samples; previous benchmark success alone is insufficient.
- [x] Add bounded fallback and recovery tests, including failure of both models, quota exhaustion, cancellation and safe explicit send-original.

### 4. New incoming messages and on-demand history

- [x] Test that durable incoming acceptance returns before translation and enqueues only eligible new public messages across chat and email; attachment-only/notes are skipped.
- [x] Implement queued processing and shared cached results; recheck policy revision/authorization/source before starting and publishing. Cancel automatic work not yet started on disable; stale completions cannot reactivate the feature. Preserve already delivered content.
- [x] Add manual Translate endpoint intent/authorization independent of Live Translate on/off. Known-language no-op returns immediately; manual historical requests never update conversation inference.
- [x] Remove viewport-triggered generation and misleading fetching indicator. Include batched metadata/cache summaries in history; completed updates arrive through authorized realtime events.
- [x] Separate budgets, cap concurrency, and test duplicate readers, reconnects, lost notifications, edited sources, cooldowns and terminal pending timeout.

### 5. Outgoing send lifecycle

- [x] Write regressions for immediate pending insertion, composer clearing, failure retention, new drafts, navigation, reload, retry and realtime-before-HTTP-response races.
- [x] Use a durable idempotent send operation with a frozen submitted intent: text, attachments, recipients/channel, target/policy revision and client identity. Reuse the existing send identity contract; do not hold database locks across model calls.
- [x] Emit authoritative preparing/translating/sending/failed progress privately to the initiating user. Define ordering/versioning and recovery retrieval so stale events cannot regress state.
- [x] On policy/target changes during an in-flight send, stop before delivery with recoverable state rather than silently switching language or sending original. Retry uses an explicit refreshed intent.
- [x] Only commit/publish the final public reply after success. Reconcile client ID to persisted ID atomically; replay/retry cannot create duplicate messages or emails.
- [x] Implement thread recovery actions in existing delivery-status area. Preserve failed snapshots and attachments until success, explicit dismissal, or normal retention cleanup; do not overwrite newer drafts on Edit.

### 6. Settings and inbox presentation

- [x] Simplify settings to the two controls; update settings-home description/search metadata and preserve route compatibility.
- [x] Implement conversation bar/menu controls, shared realtime updates and language correction. Include applicable permission and admin-disabled/provider-unavailable states without pretending translation is active.
- [x] Centralize language names/flags; use text labels and accessible neutral fallback icons. Keep supplementary guidance in tooltips.
- [x] Put manual translation actions inside bubble bottom with secondary styling; outgoing lifecycle and failures stay in existing delivery-status area. Avoid duplicated composer/thread errors.
- [x] Verify readable pending/failure styles, keyboard access and compact layouts with component/interaction checks. Do not add a required screenshot or visual-verification workflow.

### 7. Widget integration

- [x] Refresh locale on restore for supported SDK/widget paths; validate and normalize server-side, tolerate omission, and preserve manual/detected priorities.
- [x] Add direction-aware message prose/input and isolation tests for mixed Arabic/Urdu/Hebrew, English URLs, code and numbers.
- [x] Test customer sending under translation outage and final translated reply delivery across websocket, history and reconnect. Assert staff progress/originals/private notes never enter public projections.
- [x] Audit standalone support and implement applicable parity. Confirm app preview uses shared widget components. Keep full interface localization and separate AI runtime work out of this change.

### 8. Observability, verification and release readiness

- [x] Add sampled nonblocking quality review and metrics for route/fallback, no-op, invalid output, queue latency, total latency, usage/cost, duplicate suppression and terminal failures. Ensure review budget cannot consume send capacity.
- [x] Run focused tests below, then relevant integration/build checks. Fix failures caused by this work and record pre-existing blockers precisely.
- [x] Verify no history-triggered requests, no per-bubble query pattern, and responsive history loading in a long-conversation fixture; verify bounded concurrency during incoming bursts.
- [x] Review final diff for complete acceptance coverage, permission/data boundaries, migration compatibility and unrelated changes.
- [x] Record commits, tests and remaining rollout prerequisites below. Commit on waqar-fixes; do not push/promote/deploy without a current instruction authorizing that step.

## Verification commands and scenarios

Run from repository root unless a directory is specified. Verify current test names before execution; extend matching suites rather than copying implementation into tests.

```sh
# Backend: server/ working directory; include relevant existing and new suites.
go test ./internal/service ./internal/repository ./internal/handler ./internal/model ./internal/websocket ./internal/observability -run 'Translation|Translate|Language|Widget|SupportMessage' -count=1
go test -tags ee ./internal/service -run 'AICompletion|Translation|Translate' -count=1

# Repository root: focused UI, SDK and shared-widget suites.
pnpm --dir frontend test src/components/support/__tests__ src/components/settings --run
pnpm --dir packages/widget-core test
pnpm --dir packages/sdk-js test
pnpm --dir frontend exec tsc -p tsconfig.app.json --noEmit
pnpm --dir packages/sdk-js build

# Documentation (also run for this plan-only change).
python3 scripts/docs/check_names.py
python3 scripts/docs/check_links.py
python3 -m unittest discover -s scripts/docs -p '*_test.py'
git diff --check
```

Expected: focused regressions fail before implementation and pass afterward; final checks pass or unrelated pre-existing failures are documented. Use disposable PostgreSQL for migration/concurrency tests and the established harness for send/email integration tests. Verify frontend CE/EE build variants as appropriate to touched modules. Do not run paid benchmarks or production message sends as part of ordinary tests.

Release acceptance must cover: same-language; manual history; first message; live on/off/re-on; history reopening; no history requests; uncertain/mixed language; manual correction; browser hint refresh; multiple teammates; read-only permission; concurrent messages; primary/secondary outage; invalid/truncated output; large text; notes; attachment-only sends; email CC sender evidence; new draft while previous send fails; send-original; edit recovery; double click/retry; websocket-before-response; reload; workspace switch; widget reconnect; RTL; privacy deletion during model call; limit exhaustion without losing a reply.

## Execution notes

- 2026-09-22: Plan created from the agreed conversation and code inspection. No application implementation started. Model comparisons were synthetic, small-sample and model-reviewed; they do not certify translation accuracy or production reliability.
- Repository baseline: branch `waqar-fixes`; unrelated untracked `docs/mockups/my-work.html` and `docs/mockups/my-work.png` must remain untouched.
- Progress, concrete schema/event decisions, command results, commits and any product deviations belong here as execution proceeds.

### Implementation record — 2026-09-22

- Implemented in the existing `waqar-fixes` worktree without subagents. The unrelated My Work mockups remain untouched.
- Persistence owner: additive `202609220001_support_live_translate.sql`, `repository/support_live_translate.go`, and the existing translation repository. Database triggers snapshot policy revision, enabled state, reading language and source hash when each incoming message is accepted. Existing caches remain; no historical jobs are created. Legacy `inherit` resolves conservatively against all three old switches; explicit conversation choices win.
- Execution owner: `service/support_live_translate.go` (two bounded incoming workers), `service/support_pending_send.go` (two durable sender-private send workers), and `service/support_translation_review.go` (one independent sample/maintenance worker). They use the API shutdown context and PostgreSQL row locks/skip-locked claims. Attempts fence stale workers; no database lock spans a model call. Incoming failures get at most three attempts with cooldown; crashed claims can recover after two minutes.
- Send recovery freezes the submitted request, recipient, target hint, policy revision and client identity. Commit rechecks policy, lease, privacy state, recipient and attachment ownership. Permissions are resolved again after generation. Attachments link atomically with the final message. Confirmed messages retain the submission time and stable client identity; recovery includes recent confirmations to close HTTP/realtime races. Sent draft text is cleared; failed/dismissed operations have a 30-day retention window and privacy deletion removes them immediately.
- Events: `support_translation` is scoped to its conversation; `support_pending_send` also has `TargetUserID`. Visitors never receive pending operations; the public widget projection explicitly rejects pending replies in addition to its existing allowlist. Separate AI-generated replies retain their existing path.
- UI: two settings controls, per-conversation bar/menu controls, manual bubble actions, sender-private pending/failure actions, attachment-aware draft protection, and batched cached translations. History rendering makes no generation request. Realtime updates are primary; translation-cache polling is limited to fresh/pending work, and outgoing recovery polls quickly only while an operation is active.
- Browser locale refresh works on SDK websocket restore and standalone HTTP history restoration. Message prose/input use automatic direction; code and links are isolated. Widget UI labels remain outside scope.
- Provider selectors rechecked against public OpenRouter endpoint metadata on 2026-09-22: `cerebras/fp16` and `coreweave/fp8`. No production content, API keys or paid translation calls were used in implementation tests. Provider attempts have independent 12-second deadlines within bounded workflows. Jev is removed from support detection and blocking review.
- Pricing: GPT-OSS/Cerebras is registered at $0.35/M input and $0.75/M output, with no discounted prompt-cache capability for this pinned route. It belongs to the existing medium pricing tier because its input price exceeds the small tier ceiling; existing customer tier prices were not changed. The existing DeepSeek price entry bounds the CoreWeave fallback. Both edition paths have regression coverage.
- Quality review samples IDs beginning `00` (approximately 0.4%), uses a different model, caps review at 50 items per workspace/day, and records shadow verdicts without blocking delivery. Provider/fallback/cost metrics remain, with workflow duration, queue-state gauges and sampled-review outcomes added. Review and retention processing never logs message content.
- Verification recorded so far: backend translation/widget regression packages pass; enterprise completion/pricing tests pass; PostgreSQL arrival/off-interval/privacy/stale-lease/policy-change/concurrent-send and migration reapplication tests pass; inbox suite 285 tests passes; final touched UI subset 50 tests passes; shared widget 254 tests passes; SDK 239 tests passes; SDK and standalone builds pass; frontend production build passes (existing chunk-size warning); documentation naming/links and 11 documentation tests pass. Frontend type checking and enterprise frontend/API production builds also passed. The final UI subset was rerun after the last formatting change.
- Release prerequisite: deploy the migration/backend worker/API changes before or together with the frontend and rebuilt SDK/widget artifacts. This session does not authorize a push or deployment. Provider accuracy and real production traffic have not been re-benchmarked; monitor the existing translation dashboard plus queue and sampled-review metrics after rollout.

- Final review: checked migration precedence, source-edit invalidation, sender-only recovery, delivery-time revision/lease checks, public widget allowlists, stable client reconciliation and automatic-work boundaries directly (no subagents). Tests use synthetic fixtures and disposable local databases; actual customer delivery, production burst capacity and provider accuracy remain rollout monitoring concerns, not claims established by these tests.
- Plan clarifications: the composer retains its draft only until the fast enqueue request is durably acknowledged, then clears while translation continues in the thread. Cached translation summaries are fetched in batched requests independently of history rendering. Quality-review sampling is approximately 0.4%, not a per-message gate. A provider-free greeting/attachment-only reply does not require invented language evidence.

- Commit scope: `feat(support): implement conversation-scoped live translation` on `waqar-fixes`. No push or deployment is included.
