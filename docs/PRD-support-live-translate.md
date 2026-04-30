# PRD: Support Inbox Live Translate

**Status:** Draft  
**Version:** v1.0  
**Date:** 2026-04-30  
**Owners:** Support, AI Platform, Frontend  
**Primary areas:** `server/internal/model/support_inbox.go`, `server/internal/service/support_inbox_settings.go`, `server/internal/service/support_ai.go`, `frontend/src/components/support/MessageThread.tsx`, `frontend/src/components/support/MessageBubble.tsx`, `frontend/src/components/support/ReplyComposer.tsx`, `frontend/src/lib/services/supportService.ts`, `frontend/src/lib/pm-types/support.ts`

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
4. The feature is embedded in Crisp Inbox and can work across connected channels, including live chat, email, WhatsApp, Instagram, Messenger, SMS, Line, and Telegram.
5. Translation usage needs quotas/rate limits because the provider cost scales with translated characters and request volume.

Sources are listed in Section 15.

---

## 3. Problem Statement

Support agents currently need to handle non-English customer conversations manually. This creates several product gaps:

1. Agents have to copy/paste customer messages into external translators.
2. Agents can lose conversation context while switching tools.
3. Customers may receive inconsistent language quality when different agents handle the same conversation.
4. AI support and human handoff become weaker when the customer language does not match the agent/workspace language.
5. Helpin cannot measure translation usage, quality, or cost because translation happens outside the product.

---

## 4. Goals

1. Detect a customer's conversation language automatically.
2. Let agents view inbound customer messages in their preferred language.
3. Let agents draft in their preferred language and send a translated reply in the customer's language.
4. Preserve original and translated text for audit, debugging, and future improvements.
5. Support widget and email conversations in v1, with an extensible channel model for future channels.
6. Add workspace settings, per-conversation controls, and quota/rate-limit protections.
7. Keep the agent in control: translation should be previewable, editable, and disableable.

---

## 5. Non-Goals

1. Building a new inbox or live chat product.
2. Shipping voice, video, co-browsing, or social channel integrations.
3. Translating every historical conversation automatically.
4. Guaranteeing legal-grade or certified translation.
5. Replacing human review for sensitive messages.
6. Adding customer-side language switching for the full widget UI in this PRD.
7. Auto-translating internal notes for customers. Internal notes remain internal and agent-language only in v1.

---

## 6. Target Users

**Support agent**

Needs to understand customer messages and reply naturally without leaving the inbox.

**Support manager**

Needs consistent multilingual support, visibility into language coverage, and controls over cost/usage.

**Workspace admin**

Needs to enable/disable translation, choose defaults, control quotas, and understand provider usage.

**Customer**

Receives support in their own language without seeing the operational complexity behind the translation.

---

## 7. Product Principles

1. Show original text one click away.
2. Never silently change an agent's meaning without showing a preview before send.
3. Store the customer-visible message exactly as delivered.
4. Avoid duplicate translation cost through caching.
5. Disable or degrade gracefully when quotas, provider errors, or language detection confidence fail.
6. Keep translation separate from AI answering. Translation changes language; AI support changes content.

---

## 8. User Stories

1. As an agent, I can enable translation on a conversation when the customer writes in a language I do not understand.
2. As an agent, I can see the translated version of each customer message and reveal the original when needed.
3. As an agent, I can write a reply in English and preview the Spanish/French/etc. translation before sending.
4. As an agent, I can edit the translated reply before sending if the wording looks wrong.
5. As an agent, I can disable translation for a conversation if it is unnecessary or inaccurate.
6. As a customer, I receive the agent reply in my language in the same conversation thread.
7. As a manager, I can see which conversations used translation and how many characters were translated.
8. As an admin, I can configure whether translation auto-enables when a customer language mismatch is detected.

---

## 9. MVP Requirements

### 9.1 Language Detection

The system must detect and store:

1. `customer_language` for the conversation.
2. `agent_language` for the active agent or workspace default.
3. `language_detection_confidence`.
4. `language_source`: `browser_locale`, `message_detection`, `manual`, or `fallback`.

Detection rules:

1. Prefer explicit customer/browser locale when available from widget metadata.
2. Re-evaluate language from the first meaningful customer message.
3. Do not auto-enable translation when confidence is below threshold.
4. Allow manual override from the conversation header.
5. Treat same-language conversations as translation disabled by default.

### 9.2 Inbound Translation

For customer messages:

1. Store the original customer message in `support_messages.content`.
2. Translate to the agent language when translation is enabled.
3. Show translated text by default with an "Original" toggle.
4. Cache translations by message, source language, target language, and content hash.
5. Support widget and email customer messages in v1.

### 9.3 Outbound Translation

For agent replies:

1. Agent drafts in their preferred language.
2. Composer shows a `Translate to <customer language>` action when translation is enabled.
3. Agent can preview and edit translated text before sending.
4. The customer-visible translated reply is stored in `support_messages.content`.
5. The original agent draft is stored in translation metadata/audit storage.
6. The message bubble shown to agents can display both delivered translated content and original draft.
7. Email fallback/outbound email must send the translated customer-visible content.

### 9.4 Per-Conversation Controls

Add a compact translation control in the conversation header or thread toolbar:

1. Translation status: off, suggested, enabled, provider_error, quota_limited.
2. Customer language selector.
3. Agent language selector.
4. Toggle: translate inbound messages.
5. Toggle: translate outbound replies.
6. Action: re-detect language.
7. Action: disable for this conversation.

### 9.5 Workspace Settings

Add settings under support inbox settings:

1. `live_translate_enabled`
2. `live_translate_auto_enable`
3. `live_translate_default_agent_language`
4. `live_translate_provider`
5. `live_translate_monthly_character_limit`
6. `live_translate_request_limit_window`
7. `live_translate_request_limit_count`
8. `live_translate_excluded_languages`
9. `live_translate_store_originals`

Default proposal:

1. Enabled: false
2. Auto-enable: false
3. Default agent language: workspace locale or `en`
4. Store originals: true

### 9.6 Quotas And Rate Limits

The system must protect provider cost and operational stability:

1. Track translated characters per workspace per month.
2. Track translation requests per workspace per rolling window.
3. Return a typed quota error when limits are hit.
4. Show a clear UI state in the composer and message thread when translation is unavailable.
5. Do not block normal untranslated replies when translation fails.

### 9.7 Provider Abstraction

Add a provider interface instead of coupling product code to one vendor:

```go
type TranslationProvider interface {
    DetectLanguage(ctx context.Context, text string) (*LanguageDetectionResult, error)
    Translate(ctx context.Context, req TranslationRequest) (*TranslationResult, error)
}
```

Provider selection should support:

1. A production external translation provider.
2. A no-op/local fake provider for tests.
3. Optional LLM fallback only when configured.

Provider result fields:

1. Source language
2. Target language
3. Translated text
4. Confidence when available
5. Provider name
6. Provider request ID when available
7. Character count

---

## 10. Data Model

### 10.1 Conversation Translation State

Preferred v1 approach: add nullable columns to `support_conversations` for fast list/header reads.

Proposed fields:

1. `language_code`
2. `language_confidence`
3. `language_source`
4. `translation_enabled`
5. `translation_agent_language`
6. `translation_customer_language`
7. `translation_inbound_enabled`
8. `translation_outbound_enabled`

Alternative: `support_conversation_translation_states` table if we want to avoid widening `support_conversations`.

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

This table powers quota enforcement, analytics, and cost review.

---

## 11. API Contract

### Dashboard APIs

1. `POST /api/support/inbox/conversations/{conversation_id}/translation/detect`
2. `PATCH /api/support/inbox/conversations/{conversation_id}/translation`
3. `POST /api/support/inbox/conversations/{conversation_id}/translation/translate-message`
4. `POST /api/support/inbox/conversations/{conversation_id}/translation/translate-draft`
5. `GET /api/support/inbox/conversations/{conversation_id}/translations`
6. `GET /api/support/inbox/translation/usage`

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
  "target_language": "es"
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
  "cached": false
}
```

---

## 12. Frontend UX

### 12.1 Conversation Header

Show a translation pill when language mismatch is detected:

1. Suggested state: `Spanish detected`
2. Enabled state: `Translating Spanish ↔ English`
3. Error state: `Translation unavailable`
4. Quota state: `Translation quota reached`

The pill opens controls for language selection and enable/disable toggles.

### 12.2 Message Bubble

Inbound customer message:

1. Display translated content when enabled and available.
2. Show a small `Translated from Spanish` label.
3. Provide `Show original` / `Show translation`.
4. Show original by default if translation fails.

Outbound agent message:

1. Display the customer-visible sent content.
2. For agents, include `Original draft` behind a compact reveal.
3. Never expose original draft to the customer unless it was the sent content.

### 12.3 Reply Composer

When translation is enabled:

1. Show target language near the send button.
2. Add a translate preview action.
3. Let the agent send untranslated only through an explicit menu/action.
4. Disable send while translation is in progress if outbound translation is required.
5. Preserve the draft if translation fails.
6. Support shortcuts before translation; the inserted shortcut content should be translated with the rest of the draft.

### 12.4 Settings

Add a `Live translate` section to support settings:

1. Enable live translate
2. Auto-suggest for language mismatch
3. Default agent language
4. Monthly quota
5. Per-window request cap
6. Included/excluded languages
7. Usage summary

---

## 13. Backend Flow

### Inbound Message

1. Customer message is created normally.
2. Service detects language if missing or stale.
3. If translation is enabled and source differs from agent language, enqueue or perform translation.
4. Save translation cache.
5. Broadcast message update over existing support realtime path.

### Draft Translation

1. Agent clicks translate preview or sends with outbound translation enabled.
2. Backend checks quota and cache.
3. Backend translates draft.
4. Agent can edit translated draft.
5. On send, create support message with translated content as the delivered content.
6. Store original draft and translation metadata.

### Provider Failure

1. Return typed error to UI.
2. Preserve agent draft.
3. Allow untranslated send when agent chooses.
4. Log provider error with no sensitive text in structured logs.

---

## 14. Acceptance Criteria

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

---

## 15. Research Sources

1. Crisp LiveTranslate product page: https://crisp.chat/en/livechat/realtime-chat-translation/
2. Crisp Inbox guide: https://help.crisp.chat/en/article/getting-started-with-the-crisp-inbox-opv83/
3. Crisp LiveTranslate quota guide: https://help.crisp.chat/en/article/what-happens-if-i-reach-my-monthly-livetranslate-limit-1hcbmaj/

---

## 16. Open Questions

1. Which provider should be the first production translation backend: Azure Translator, DeepL, Google Cloud Translation, or LLM-based translation?
2. Should translation be available to all paid workspaces or gated by plan/module access?
3. Should auto-enable default to on for customer language mismatch after the feature is proven?
4. Do we need per-agent preferred language settings before v1, or is workspace default enough?
5. Should translation of AI-generated answers use the same translation provider or happen inside the support AI response prompt?
6. How long should translated content be retained if a workspace has strict privacy settings?
7. Should translated content be searchable in conversation search, or only original content?

