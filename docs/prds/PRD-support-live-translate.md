# PRD: Support Inbox Live Translate

**Status:** Draft, ready for design review  
**Version:** v1.1  
**Date:** 2026-04-30  
**Owners:** Support, AI Platform, Frontend  
**Primary areas:** `server/internal/model/support_inbox.go`, `server/internal/service/support_inbox_settings.go`, `server/internal/service/support_translation.go`, `server/internal/repository/support_translation.go`, `server/internal/handler/support_translation.go`, `server/internal/router/router.go`, `server/internal/translate/`, `server/internal/dbmigrate/sql/`, `frontend/src/components/support/MessageThread.tsx`, `frontend/src/components/support/MessageBubble.tsx`, `frontend/src/components/support/ReplyComposer.tsx`, `frontend/src/hooks/queries/useSupport.ts`, `frontend/src/lib/services/supportService.ts`, `frontend/src/lib/supportTypes.ts`

---

## 1. Context

Helpin already has the shared support inbox, conversation threads, reply composer, AI support pipeline, widget/email channels, message shortcuts, message metadata, mailbox routing, and support settings. The missing capability is live translation inside existing support conversations.

Crisp positions LiveTranslate as an inbox feature: customer messages can be translated for the agent, and agent replies can be translated back into the customer's language while the agent is composing. Crisp also exposes per-conversation language controls, automatic language detection, and quota/rate-limit behavior.

This PRD defines the Helpin version of that feature. It does not introduce a new live chat product, inbox, or widget. It adds translation to the current support conversation surfaces.

---

## 2. Research Summary

Crisp's public material describes these LiveTranslate behaviors:

1. Agents can translate customer messages in real time and reply in the customer's language.
2. Translation is controlled per conversation, not only globally.
3. Customer language can be detected automatically from browser settings and conversation context.
4. The feature is embedded in Crisp Inbox and can work across connected channels.
5. Translation usage needs quotas/rate limits because provider cost scales with translated characters and request volume.

Research sources are listed in Section 22.

---

## 3. Problem Statement

Support agents currently need to handle non-English customer conversations manually.

Current gaps:

1. Agents copy/paste customer messages into external translators.
2. Agents lose context while switching tools.
3. Customers may receive inconsistent language quality across agents.
4. AI support and human handoff are weaker when the customer language does not match the agent/workspace language.
5. Helpin cannot measure translation usage, quality, or cost because translation happens outside the product.

---

## 4. Goals

1. Detect a customer's conversation language automatically.
2. Let agents view inbound customer messages in their preferred language.
3. Let agents draft in their preferred language and send a translated reply in the customer's language.
4. Preserve original and translated text for audit, debugging, and future improvements.
5. Support widget and email conversations in v1.
6. Add workspace settings, per-conversation controls, quota/rate-limit protection, and usage reporting.
7. Keep the agent in control: translation should be previewable, editable, and disableable.

---

## 5. Non-Goals

1. Building a new inbox or live chat product.
2. Shipping voice, video, co-browsing, or social channel integrations.
3. Translating every historical conversation automatically.
4. Guaranteeing legal-grade or certified translation.
5. Replacing human review for sensitive messages.
6. Localizing the full customer widget UI in this PRD.
7. Translating internal notes. Internal notes remain internal and agent-language only in v1.

---

## 6. Target Users

**Support agent**

Needs to understand customer messages and reply naturally without leaving the inbox.

**Support lead**

Needs consistent multilingual support, visibility into language coverage, and controls over cost/usage.

**Workspace admin**

Needs to enable/disable translation, choose defaults, control quotas, and understand provider usage.

**Customer, as passive recipient**

Receives support in their own language without seeing the operational complexity behind translation.

---

## 7. Product Principles

1. Show original text one click away.
2. Never silently change an agent's meaning without showing a preview before send.
3. Store the customer-visible message exactly as delivered.
4. Avoid duplicate translation cost through caching.
5. Disable or degrade gracefully when quotas, provider errors, or language detection confidence fail.
6. Keep translation separate from AI answering. Translation changes language; AI support changes content. In v1, AI support generates the normal answer first, then Live Translate translates the final customer-visible answer when needed. AI-generated answers translated this way count against the same translation quota as human replies.

---

## 8. User Stories

1. As an agent, I can enable translation on a conversation when the customer writes in a language I do not understand.
2. As an agent, I can see translated customer messages and reveal the original when needed.
3. As an agent, I can write a reply in English and preview the Spanish/French/etc. translation before sending.
4. As an agent, I can edit the translated reply before sending if the wording looks wrong.
5. As an agent, I can disable translation for a conversation if it is unnecessary or inaccurate.
6. As a customer, I receive the agent reply in my language in the same conversation thread.
7. As a support lead, I can see which conversations used translation and how many characters were translated.
8. As an admin, I can configure whether translation auto-suggests for language mismatch.

---

## 9. MVP Requirements

### 9.1 Language Detection

The system must detect and store:

1. `customer_language` for the conversation.
2. `agent_language` from workspace default in v1.
3. `language_detection_confidence`.
4. `language_source`: `browser_locale`, `message_detection`, `manual`, or `fallback`.

Detection rules:

1. Prefer explicit customer/browser locale when available from widget metadata.
2. Re-evaluate language from the first meaningful customer message.
3. Do not auto-suggest translation when confidence is below threshold.
4. Allow manual override from the conversation header.
5. Treat same-language conversations as translation disabled by default.

### 9.2 Inbound Translation

For customer messages:

1. Store the original customer message in `support_messages.content`.
2. Translate to the agent language when translation is enabled.
3. Show translated text by default with an `Original` toggle.
4. Cache translations by message, source language, target language, and content hash.
5. Support widget and email customer messages in v1.

### 9.3 Outbound Translation

For agent replies:

1. Agent drafts in the workspace/agent language.
2. Composer shows `Translate to <customer language>` when outbound translation is enabled.
3. Agent can preview and edit translated text before sending.
4. The customer-visible translated reply is stored in `support_messages.content`.
5. The original agent draft is stored in translation metadata/audit storage.
6. The agent-side message bubble can reveal the original draft after send.
7. Email fallback/outbound email sends the translated customer-visible content.

### 9.4 Per-Conversation Controls

Add a compact translation control in the conversation header or thread toolbar:

1. Status: `off`, `suggested`, `enabled`, `provider_error`, `quota_limited`.
2. Customer language selector.
3. Agent language selector.
4. Toggle: translate inbound messages.
5. Toggle: translate outbound replies.
6. Action: re-detect language.
7. Action: disable for this conversation.

### 9.5 Workspace Settings

Add settings under `SupportInboxSettings`:

| Setting | Type | Unit | Default |
|---------|------|------|---------|
| `live_translate_enabled` | bool | n/a | false |
| `live_translate_auto_enable` | bool | n/a | false |
| `live_translate_default_agent_language` | string | BCP 47 language tag | workspace locale or `en` |
| `live_translate_monthly_character_limit` | int | translated characters per calendar month | 500000 |
| `live_translate_request_limit_window_secs` | int | seconds | 60 |
| `live_translate_request_limit_count` | int | requests per window | 120 |
| `live_translate_excluded_languages` | []string | BCP 47 language tags | [] |
| `live_translate_store_originals` | bool | n/a | true |
| `live_translate_detect_min_confidence` | float64 | 0.0-1.0 | 0.75 |

### 9.6 Quotas And Rate Limits

The system must protect provider cost and operational stability:

1. Track translated characters per workspace per month.
2. Track translation requests per workspace per rolling window.
3. Reserve quota atomically before calling the provider.
4. Return a typed quota error when limits are hit.
5. Show clear UI state when translation is unavailable.
6. Do not block normal untranslated replies when translation fails.

### 9.7 Provider Abstraction

Add `server/internal/translate/` with this interface. `cmd/api/main.go` wires the concrete provider into `SupportTranslationService`.

```go
type TranslationProvider interface {
    DetectLanguage(ctx context.Context, text string) (*LanguageDetectionResult, error)
    Translate(ctx context.Context, req TranslationRequest) (*TranslationResult, error)
}
```

Provider selection:

1. Azure Translator is the v1 production provider.
2. A fake/no-op provider is required for tests.

Provider selection is environment-driven in v1 through Doppler-managed config. It is not configurable per workspace. Per-workspace settings control enablement, quotas, and behavior, not which provider backend is used.

Provider operations must enforce:

1. 5 second timeout for detect and translate calls.
2. 1 retry for transient 429/5xx errors with jittered backoff.
3. Circuit breaker per provider when error rate exceeds threshold.
4. No provider call when quota cannot be atomically reserved.
5. PII minimization/redaction before provider calls where safe. Redact obvious secrets such as API keys, bearer tokens, passwords, and credit card-like numbers.
6. Region/data-residency setting for providers that support it. V1 uses the commercial cloud region covered by Helpin's vendor DPA and subprocessor list.

Provider result fields:

1. Source language
2. Target language
3. Translated text
4. Confidence when available
5. Provider name
6. Provider request ID when available
7. Character count

---

## 10. Data Model And Migration Plan

All schema changes for v1 use `dbmigrate` SQL in `server/internal/dbmigrate/sql/YYYYMMDDNNNN_support_live_translate.sql`, not only GORM AutoMigrate. AutoMigrate may include struct additions after the migration lands, but the idempotent dbmigrate file is the rollout source of truth.

### 10.1 Conversation Translation State

Preferred v1 approach: add nullable/defaulted columns to `support_conversations` for fast list/header reads.

Fields:

1. `detected_customer_language`
2. `detected_customer_language_confidence`
3. `detected_customer_language_source`
4. `translation_enabled`
5. `translation_agent_language`
6. `translation_customer_language`
7. `translation_inbound_enabled`
8. `translation_outbound_enabled`
9. `translation_version` integer, default 1, for optimistic concurrency

`detected_customer_language` is the latest language detected by browser locale or message detection. `translation_customer_language` is the confirmed routing language used for translation. When the agent manually overrides language, only `translation_customer_language` must change; the detected value remains useful context.

Widening `support_conversations` is acceptable for v1 because these fields are read with the conversation header/list and avoid an extra join on every inbox thread load. The migration requires no historical backfill beyond default values.

For existing conversations, all language fields stay `NULL`, `translation_enabled` defaults false, and the UI must render "No language detected" with manual language selection available.

Indexes:

1. `(workspace_id, translation_enabled)`
2. `(workspace_id, detected_customer_language)`

### 10.2 Message Translations

Add `support_message_translations`:

1. `id`
2. `workspace_id`
3. `conversation_id`
4. `message_id`
5. `direction`: `inbound` or `outbound`
6. `source_language`
7. `target_language`
8. `source_text_hash`
9. `translated_content`
10. `original_content`
11. `provider`
12. `provider_model`
13. `provider_request_id`
14. `confidence`
15. `character_count`
16. `created_by_user_id`
17. `created_at`
18. `updated_at`

Unique constraint:

```sql
(message_id, source_language, target_language, source_text_hash)
```

Indexes:

1. `(workspace_id, conversation_id, created_at)`
2. `(conversation_id, message_id)`
3. `(workspace_id, source_language, target_language)`

Hash definition:

1. Normalize content to UTF-8 NFC.
2. Trim leading/trailing whitespace.
3. Collapse CRLF/CR to LF.
4. Preserve case and internal whitespace.
5. Hash with SHA-256.
6. Store lowercase hex string.

### 10.3 Usage Accounting

Add `support_translation_usage_events`:

1. `id`
2. `workspace_id`
3. `conversation_id`
4. `message_id`
5. `provider`
6. `operation`: `detect` or `translate`
7. `source_language`
8. `target_language`
9. `character_count`
10. `request_count`
11. `status`: `success`, `provider_error`, `quota_limited`, `rate_limited`
12. `created_at`

Indexes:

1. `(workspace_id, created_at)`
2. `(workspace_id, status, created_at)`
3. `(workspace_id, provider, created_at)`

### 10.4 Monthly Quota Counters

Add `support_translation_monthly_usage`:

1. `workspace_id`
2. `period_start` date
3. `character_count`
4. `request_count`
5. `updated_at`

Primary key:

```sql
(workspace_id, period_start)
```

Quota reservation must run in a database transaction using an atomic upsert/update with a `WHERE character_count + reserved_chars <= limit` guard. Do not implement quota as SELECT then UPDATE.

Canonical SQL pattern:

```sql
INSERT INTO support_translation_monthly_usage (
  workspace_id,
  period_start,
  character_count,
  request_count,
  updated_at
) VALUES (
  $1,
  $2,
  $3,
  1,
  now()
)
ON CONFLICT (workspace_id, period_start)
DO UPDATE SET
  character_count = support_translation_monthly_usage.character_count + EXCLUDED.character_count,
  request_count = support_translation_monthly_usage.request_count + EXCLUDED.request_count,
  updated_at = now()
WHERE support_translation_monthly_usage.character_count + EXCLUDED.character_count <= $4
RETURNING character_count;
```

Reservation succeeds iff a row is returned. If no row is returned, the service records a `quota_limited` usage event and returns a typed quota error without calling the provider.

### 10.5 Audit Events

Audit/system-event metadata must record:

1. Translation enabled/disabled.
2. Customer/agent language changed.
3. Translated draft manually edited before send.
4. Provider quota/error state shown to an agent.

---

## 11. API Contract And RBAC

All endpoints are workspace-scoped with `?workspace_id=` and use the existing `RequireAuth`, `RequireWorkspaceAccess`, and `RequirePermission` middleware chain.

| Endpoint | Permission | Role floor |
|----------|------------|------------|
| `POST /api/support/inbox/conversations/{conversation_id}/translation/detect` | `support.translate` | member |
| `PATCH /api/support/inbox/conversations/{conversation_id}/translation` | `support.translate` | member |
| `POST /api/support/inbox/conversations/{conversation_id}/translation/translate-message` | `support.translate` | member |
| `POST /api/support/inbox/conversations/{conversation_id}/translation/translate-draft` | `support.translate` | member |
| `GET /api/support/inbox/conversations/{conversation_id}/translations` | `support.read` | viewer |
| `GET /api/support/inbox/translation/usage` | `support.translate.manage` | admin |

Add permission constants in `server/internal/authorization`:

1. `PermSupportTranslate = "support.translate"`
2. `PermSupportTranslateManage = "support.translate.manage"`

`support.translate` allows per-conversation use. `support.translate.manage` allows settings and quota visibility. The current RBAC role set is `viewer`, `member`, `admin`, `owner`; there is no workspace `manager` role in `server/internal/authorization/rbac.go`, so manage access starts at `admin`.

### Request Examples

```json
{
  "message_id": "uuid",
  "target_language": "en"
}
```

```json
{
  "content": "Thanks for reaching out. I can help you with that.",
  "source_language": "en",
  "target_language": "es",
  "draft_revision": 7,
  "content_hash": "sha256hex"
}
```

### Response Example

```json
{
  "source_language": "en",
  "target_language": "es",
  "translated_content": "Gracias por contactarnos. Puedo ayudarte con eso.",
  "confidence": 0.94,
  "provider": "azure_translator",
  "character_count": 51,
  "cached": false,
  "draft_revision": 7
}
```

---

## 12. Realtime Contract

Translation uses the existing support WebSocket channel and emits explicit events so all open inbox sessions reconcile consistently.

### Server-to-dashboard Events

`support.translation_state_updated`

```json
{
  "type": "support.translation_state_updated",
  "workspace_id": "uuid",
  "conversation_id": "uuid",
  "translation_enabled": true,
  "inbound_enabled": true,
  "outbound_enabled": true,
  "customer_language": "es",
  "agent_language": "en",
  "status": "enabled",
  "version": 4,
  "updated_by_user_id": "uuid",
  "updated_at": "2026-04-30T12:00:00Z"
}
```

`support.message_translation_updated`

```json
{
  "type": "support.message_translation_updated",
  "workspace_id": "uuid",
  "conversation_id": "uuid",
  "message_id": "uuid",
  "source_language": "es",
  "target_language": "en",
  "status": "ready",
  "cached": false,
  "updated_at": "2026-04-30T12:00:00Z"
}
```

`support.translation_usage_limited`

```json
{
  "type": "support.translation_usage_limited",
  "workspace_id": "uuid",
  "conversation_id": "uuid",
  "reason": "monthly_quota",
  "reset_at": "2026-05-01T00:00:00Z"
}
```

Frontend reconciliation:

1. `useRealtimeSync.ts` invalidates conversation, messages, translations, and settings query keys for the affected workspace/conversation.
2. If another agent changes translation state while the composer is open, show a non-blocking inline notice and update the target language label.
3. Draft translation responses include `draft_revision`. The composer discards stale responses when the active draft revision differs.

---

## 13. Concurrency And Idempotency

1. Per-conversation translation settings use `translation_version`. `PATCH` requests may include `expected_version`; stale updates return `409 conflict` with current state.
2. Translation cache writes use `INSERT ... ON CONFLICT DO UPDATE/NOTHING` on `(message_id, source_language, target_language, source_text_hash)`.
3. Draft translation requests include `draft_revision` and `content_hash`. The frontend only applies the result if both still match the active draft.
4. Customer messages arriving while a previous translation is running are processed independently by message ID and sequence.
5. Quota reservation is atomic through `support_translation_monthly_usage` inside the same transaction that records the usage event.
6. The primary `Send` action is disabled only while a required outbound translation request is actively pending. If translation fails or is unavailable, agents can use explicit `Send without translation`.

---

## 14. Frontend UX

### 14.1 Conversation Header

Show a translation pill when language mismatch is detected:

1. Suggested state: `Spanish detected`
2. Enabled state: `Translating Spanish to English`
3. Error state: `Translation unavailable`
4. Quota state: `Translation quota reached`

The pill opens controls for language selection and enable/disable toggles.

### 14.2 Message Bubble

Inbound customer message:

1. Display translated content when enabled and available.
2. Show a small `Translated from Spanish` label.
3. Provide `Show original` / `Show translation`.
4. Show original by default if translation fails.

Outbound agent message:

1. Display the customer-visible sent content.
2. For agents, include `Original draft` behind a compact reveal.
3. Never expose original draft to the customer unless it was the sent content.

### 14.3 Reply Composer

When translation is enabled:

1. Show target language near the send button.
2. Add a translate preview action.
3. Let the agent send untranslated only through an explicit menu/action.
4. Disable the primary send action only while a required outbound translation request is actively pending.
5. Preserve the draft if translation fails.
6. Support shortcuts before translation; the inserted shortcut content is translated with the rest of the draft.
7. Hide translation controls when composing an internal note.

### 14.4 Settings

Add a `Live translate` section to support settings:

1. Enable live translate
2. Auto-suggest for language mismatch
3. Default agent language
4. Monthly quota, editable by users with `support.translate.manage`
5. Per-window request cap, editable by users with `support.translate.manage`
6. Included/excluded languages
7. Usage summary

---

## 15. Backend Flow

### 15.1 Inbound Message

1. Customer message is created normally.
2. Service detects language if missing or stale.
3. If translation is enabled and source differs from agent language, reserve quota and translate.
4. Save translation cache.
5. Broadcast `support.message_translation_updated`.

### 15.2 Draft Translation

1. Agent clicks translate preview or sends with outbound translation enabled.
2. Backend checks cache and atomically reserves quota if provider call is needed.
3. Backend translates draft.
4. Agent can edit translated draft.
5. On send, create support message with translated content as delivered content.
6. Store original draft and translation metadata.

### 15.3 Provider Failure

1. Return typed error to UI.
2. Preserve agent draft.
3. Allow untranslated send when agent chooses.
4. Log provider error with no sensitive text in structured logs.

---

## 16. Security And Privacy

1. Provider API keys live in Doppler-managed environment variables, not database rows. Suggested env vars: `TRANSLATION_PROVIDER`, `AZURE_TRANSLATOR_KEY`, `AZURE_TRANSLATOR_ENDPOINT`, `AZURE_TRANSLATOR_REGION`.
2. Do not log raw message text, translated text, or provider payloads.
3. Use `slog.InfoContext`, `slog.WarnContext`, and `slog.ErrorContext` with structured fields: `workspace_id`, `conversation_id`, `message_id`, `provider`, `operation`, `source_language`, `target_language`, `character_count`, `cached`, `latency_ms`, `status`.
4. Sending customer text to the translation provider must be covered by Helpin's vendor DPA and subprocessors list before GA.
5. Redact obvious secrets before provider calls where redaction does not destroy translation quality.
6. `support_message_translations.original_content` is only stored when `live_translate_store_originals` is true. If false, the lookup path must tolerate missing `original_content`. Inbound translation cache rows still keep `translated_content` because it is the cached display value. Outbound sent replies must not duplicate the customer-visible delivered content already stored in `support_messages.content`; they keep hash, language pair, provider metadata, and original draft only when retention settings allow it.
7. Retention follows support conversation retention in v1. Future stricter retention can delete rows from `support_message_translations` by workspace/conversation.
8. Audit who enabled/disabled translation and who edited translated drafts before send.

---

## 17. Observability

Metrics:

1. `support_translation_requests_total{workspace_id,provider,operation,status}`
2. `support_translation_characters_total{workspace_id,provider,operation,status}`
3. `support_translation_latency_ms{provider,operation}`
4. `support_translation_cache_hits_total{workspace_id,provider}`
5. `support_translation_quota_limited_total{workspace_id}`
6. `support_translation_provider_errors_total{provider,error_class}`

Alerts:

1. Provider error rate above 5% for 10 minutes.
2. P95 translate latency above 4 seconds for 15 minutes.
3. Circuit breaker open for any production provider.
4. Workspace reaches 90% and 100% monthly quota.

Dashboards:

1. Per-workspace monthly usage.
2. Provider latency and error rate.
3. Cache hit rate.
4. Top language pairs.

---

## 18. Rollout Plan

1. Add server-side feature flag `SUPPORT_LIVE_TRANSLATE_ENABLED=false` as global kill switch.
2. Add workspace beta gate/module flag before exposing settings UI.
3. Ship fake provider tests and hidden UI behind the flag.
4. Enable internal workspace with fake provider, then Azure provider.
5. Enable 3-5 beta workspaces with low monthly quota.
6. Review usage, latency, customer-visible quality issues, and provider cost.
7. Expand to paid workspaces behind plan/module access.
8. Keep global kill switch and per-workspace disable for GA.

Plan gating decision for v1: Live Translate is beta-gated by workspace/module flag, then later tied to paid plan/module access.

---

## 19. Testing Strategy

Backend unit tests:

1. Language detection threshold behavior.
2. Settings merge/validation.
3. Cache hash normalization and uniqueness.
4. Quota atomic reservation under concurrent goroutines.
5. Provider timeout/retry/circuit-breaker behavior with fake provider.
6. Permission enforcement for every endpoint.
7. Draft translation stale revision rejection.

Backend integration tests:

1. `support_message_translations` cache hit path.
2. Translation usage event creation.
3. Conversation translation state optimistic concurrency.
4. Inbound message flow emits realtime translation events.
5. Outbound translated send stores delivered content plus original draft metadata.

Frontend tests:

1. Message bubble translated/original toggle.
2. Composer applies only current `draft_revision`.
3. Composer hides translation for internal notes.
4. Conversation header handles suggested/enabled/error/quota states.
5. Keyboard navigation and ARIA labels for translation controls.

E2E tests:

1. Agent enables translation on Spanish conversation.
2. Customer message appears translated.
3. Agent translates reply and sends.
4. Another open agent session receives translation state/message update.
5. Quota-limited workspace can still send untranslated replies.

Load/concurrency tests:

1. Concurrent quota reservations must not exceed configured monthly character limit.
2. 100 simultaneous translation cache requests for the same message produce one persisted cache row.

---

## 20. Acceptance Criteria

### Functional

1. A Spanish inbound customer message can be displayed in English to an English-speaking agent.
2. The same message can reveal the original Spanish text.
3. An English agent draft can be translated to Spanish before send.
4. The customer receives only the Spanish reply in widget/email.
5. The agent can reveal the original English draft after send.
6. Translation can be enabled/disabled per conversation.
7. Workspace admins can turn Live Translate on/off.
8. Quotas prevent unlimited translation calls and show a user-facing quota state.
9. Cached translations are reused for repeated display and do not create duplicate provider calls.
10. Existing inbox messaging works when translation is disabled or unavailable.
11. Internal notes never show translation controls and are never sent to the translation provider.
12. AI-generated support replies are translated through the same Live Translate provider before customer delivery when the conversation target language differs.

### Non-Functional

1. P95 draft translation latency is under 4 seconds for messages up to 2000 characters.
2. P95 cached translation read latency is under 150ms.
3. Provider error state appears in UI within 2 seconds of a failed request.
4. For repeated displays of the same already-translated message within a workspace after the first provider call, cache hit rate should exceed 95%.
5. Monthly quota enforcement must not overspend under concurrent requests.
6. Translation controls are keyboard accessible and have ARIA labels for `Show original`, `Show translation`, language selectors, and enable/disable actions.
7. No raw message or translated content appears in application logs.

---

## 21. Resolved Product Decisions

1. V1 production provider: Azure Translator, wired through `server/internal/translate/`.
2. V1 availability: beta-gated per workspace/module flag, then paid-plan/module gated after beta.
3. AI answers: generate content through the existing support AI pipeline, then translate the final customer-visible response using Live Translate when required.
4. Agent language: workspace default language for v1; per-agent preferred language is post-MVP.
5. Search: original support message content remains the searchable source of truth in v1. Translated content search is post-MVP.

---

## 22. Research Sources

1. Crisp LiveTranslate product page: https://crisp.chat/en/livechat/realtime-chat-translation/
2. Crisp Inbox guide: https://help.crisp.chat/en/article/getting-started-with-the-crisp-inbox-opv83/
3. Crisp LiveTranslate quota guide: https://help.crisp.chat/en/article/what-happens-if-i-reach-my-monthly-livetranslate-limit-1hcbmaj/
4. Azure AI Translator product overview: https://azure.microsoft.com/en-us/products/ai-services/ai-translator
5. Azure AI Translator pricing: https://azure.microsoft.com/en-us/pricing/details/cognitive-services/translator/
6. Azure AI Translator language support: https://learn.microsoft.com/en-us/azure/ai-services/translator/text-translation/reference/v3/languages
7. Azure AI Translator data, privacy, and security: https://learn.microsoft.com/en-us/legal/cognitive-services/translator/data-privacy-security

---

## 23. Remaining Open Questions

1. Should auto-enable default to on after beta, or stay manual with suggestions?
2. How long should translated content be retained if a workspace has stricter privacy settings than default support retention?
3. Should visitor-facing widget UI labels be localized in the same release or handled by the existing widget localization roadmap?
