# Historical Helpin support surface audit

This is the September 10 product/source audit, preserved as historical research. Its counts, line numbers, competitive comparisons, security conclusions, and implementation verdicts describe that review and must not be used as a current capability or release checklist.

## Source review — 2026-09-18

Several headline findings have changed in this checkout:

- Licensing and contributor policies now exist: [LICENSE](../../LICENSE), [contributing](../../CONTRIBUTING.md), [security](../../SECURITY.md), and [code of conduct](../../CODE_OF_CONDUCT.md). Read the license boundaries rather than the old “no license” conclusion below.
- [Community deployment](../community/deployment.md), configuration, identity, backup, and troubleshooting guides now accompany the Community packaging. [Publication status](../../community/PUBLICATION.md) separately records what remains unverified; source packaging does not prove public image availability or production readiness.
- [Widget origin authorization](../../server/internal/service/support_widget_origin.go) and the WebSocket handler now enforce installation origin policy. The old claim that allowed origins are decorative is obsolete. [Widget identity setup](../community/widget-identity.md) also documents the server-side signing integration.
- [SMTP transport](../../server/internal/email/smtp.go) exists. This does not establish SMTP parity for every Postmark support-email feature; configure the documented transport and inbound capabilities for the chosen deployment.
- [AI connections](../../server/internal/service/ai_connection.go), credential/access policy, and route selection implement customer-provided provider connections. The old “BYOK is missing” and fixed-tier-only descriptions no longer describe the current AI configuration surface.
- [Deployment module policy](../../server/internal/deployment/modules.go) filters enabled modules before actor grants. The original conclusion that PM and Docs can never be hidden is obsolete. The router also accepts authenticated API rate-limit middleware; the blanket absence claim below is stale.
- Several original source symbols and paths have moved or been removed, including the named escalation helpers. Old line references are provenance for this audit, not navigable evidence of today's call graph. See the maintained [architecture overview](../../ARCHITECTURE.md) for current boundaries.

The remaining feature-gap inventory is historical and needs a feature-specific source review before prioritization. This review did not retest the product, perform a security audit, measure bundle sizes, assess legal compliance, or verify external vendor features. The original legal and competitive judgments below are the author's dated opinions, not current conclusions.

## Original audit — 2026-09-10

## 0. Scale

| Metric | Count |
|---|---|
| Go files in `server/` | 1,624 |
| Go LOC (all) | 476,050 |
| Go LOC (excluding `_test.go`) | 309,665 |
| SQL migrations, `server/internal/dbmigrate/sql/` | 227 |
| SQL migrations, `server/migrations/` (legacy dir) | 85 |
| ClickHouse migrations, `server/internal/chmigrate/sql/` | 4 |
| Distinct `CREATE TABLE` names across both SQL dirs | 241 |
| `TableName()` implementations in `server/internal/model/` | 276 |
| Frontend routes under `/support` | 6 |
| Support-related Go service files | 76 (non-test) |

The 6 support routes are `frontend/src/routes/_authenticated/w/$slug/support.tsx` (module gate), `support/_inbox.tsx`, `support/_inbox/index.tsx`, `support/_inbox/$conversationId.tsx`, `support/search.tsx`, `support/coverage.tsx`. Two adjacent routes sit outside that tree: `settings/support-ai-assistant.tsx` and `pm/support.tsx`.

## 1. The one-line summary

Helpin is a **deep, narrow AI-first support product**. The AI answering engine, the knowledge retrieval pipeline, the email channel, and the help center are genuinely strong, in several places stronger than the PRDs describing them. Everything around that core is thin: two channels only, no reporting, no outbound messaging, no CSAT, and no license.

---

## 2. Widget / Messenger

Architecture: a 2 KB loader stub at `packages/sdk-js/src/loader.ts` async-loads a hashed bundle. The runtime is `packages/sdk-js/src/core/widget.ts` (2,319 lines) mounting Preact components from `packages/widget-core/src/` into an open shadow root. Backend at `server/internal/handler/support_inbox_widget.go` plus `server/internal/websocket/widget_handler.go`.

| Feature | Status | Evidence |
|---|---|---|
| Launcher color / icon | DONE | `packages/widget-core/src/WidgetLauncher.tsx:17-53` |
| Launcher position `bottom-left` | **Broken** | `ChatWindow.tsx:238` sets `--left/--right` on the panel, but `.helpin-launcher` is hardcoded `right: 20px` in `widget.css:324-327` |
| Home / Messages / Help tabs | DONE | `BottomNav.tsx`, `HomeView.tsx`, `MessagesView.tsx`, `HelpView.tsx` |
| Conversation list and history | DONE | WS `conversations:list` at `widget.ts:2089` |
| File attachments | DONE | Presigned S3, `useAttachmentUploads.ts`, `router.go:203-205`. Limit 100 MB (`service/support_attachment.go:17`), but admin copy says 10 MB (`ChatGeneralTab.tsx:1686`) |
| Emoji picker | DONE | `EmojiPicker.tsx`, lazy-loaded catalog |
| CSAT in widget | PARTIAL | UI exists (`CsatRating.tsx`), submission fires an analytics event only (`widget.ts:1509-1527`). No table, no API. Admin toggle hard-disabled: `ChatGeneralTab.tsx:1699-1701` says "will be available soon" |
| Article search in widget | DONE | `HelpView.tsx:32-70` -> `GET /widget/support/help/search` |
| Identity verification | PARTIAL | HMAC-SHA256 v1 verifier at `service/support_widget_identity.go`. **No JWT.** No admin UI for the mode or the secret key. New installs default to `enforced` (`support_inbox_settings.go:601`) with no documented way to satisfy it |
| Visitor attributes / custom data | PARTIAL | Fixed field set in `WidgetIdentityPayload` (`model/support_inbox.go:1128-1139`). `WidgetUser.metadata` is declared at `widget.ts:34` and **never serialized**, so custom attributes are silently dropped |
| Visibility / targeting rules | MISSING | Only imperative `show()`/`hide()` |
| Office hours, expected reply time | DONE | `ChatGeneralTab.tsx:1518-1590`, shared presets in `packages/shared/src/reply-time.ts` with a Go/TS parity test |
| Multilingual / i18n | MISSING | Every widget string is a hardcoded English literal. `navigator.language` is reported but unused |
| Native mobile SDK (customer) | MISSING | See below |
| Tours, checklists, surveys, banners, news | MISSING | Only an admin outage banner (`SpecialNoticeBanner.tsx`) |
| Push notifications | PARTIAL, agent-side only | FCM/APNs for the agent app (`service/push_sender.go`, Tauri plugin). No web push for visitors |
| Typing indicators | DONE, bidirectional | `widget.ts:1421-1483`, `websocket/hub.go:277-279` |
| Read receipts | PARTIAL | Visitor read cursor is sent and agents see delivery state. The visitor never sees whether the agent read their message |
| Offline email fallback | DONE | `service/email_fallback.go`, 4,232 lines, with MailSlurp e2e tests |
| Pre-chat form | PARTIAL | Fixed email then phone only (`PreChatForm.tsx`). No custom fields |
| Unread badge, sound | DONE / PARTIAL | Sound is always on, no setting |
| Allowed domains enforcement | **Decorative** | `allowed_origins` is validated and stored (`support_widget_identity.go:121-145`) but never checked. All widget routes use `AllowedOrigins: []string{"*"}` (`router.go:185, 228, 373, 405, 418`) and the WS upgrader sets `InsecureSkipVerify: true` (`websocket/widget_handler.go:76`) |

**`apps/support-mobile` and `apps/support-desktop` are agent-facing, not customer-facing.** Both are Tauri v2 apps wrapping React. `apps/support-mobile/src-tauri/tauri.conf.json` declares `ai.helpin.mobile`; the screens are `inbox-screen`, `conversation-screen`, `search-screen`. Native code exists only for push. There is **no customer-facing native SDK on any platform**, and no React Native or webview wrapper.

Published npm packages: `@helpin-ai/sdk-js`, `@helpin-ai/react`, `@helpin-ai/nextjs`, `@helpin-ai/vue`. Versions are rewritten at publish time from git tags, so the `0.0.0` in the repo is expected. `packages/widget-embed` is abandoned (19-line `main.tsx`, empty `dist/`, no importers). `widget/` is a dead 2,132-line vanilla JS predecessor built by no workflow.

Also worth filing: the SDK ships **unminified** at 426 KB because `packages/sdk-js/vite.config.ts:50` sets `minify: false`.

---

## 3. Inbox / helpdesk

The conversation model is `server/internal/model/support_inbox.go:14`. It carries two orthogonal state axes, which is a good design. `Status` is `open | waiting_on_customer | resolved | spam`. `FlowState` is `ai_handling | waiting_for_human | queued_for_human | after_hours_queue | assigned_to_human | resolved_by_ai | resolved_by_human`.

| Feature | Status | Notes |
|---|---|---|
| Conversation model, statuses, flow states | DONE | `model/support_inbox.go:108-133` |
| Priority as an editable field | PARTIAL | Settable on create (`CreateConversationDialog.tsx:33`), filterable in search, but **no update route exists**. An agent can never re-prioritize |
| Manual assignment (user and AI agent) | DONE | `router.go:933-934` |
| Round-robin | PARTIAL | `repository/support_mailbox.go:581-621`. Entitlement-gated, fires only on mailbox move or AI handoff, and does **not** consult agent presence |
| Load-balanced assignment | MISSING | |
| Assignment rules targeting a user | MISSING | Triage rules target mailboxes only, deliberately (`support-conversation-triage-routing.md:238`) |
| Teams / mailboxes | DONE | `migrations/064_support_team_inboxes.sql`, `frontend/src/pages/settings/InboxesRoutingSettingsPage.tsx` |
| SLAs | PARTIAL | Reply-time *expectation* only. `EntitlementFeatureSLAPolicies` is billed as a Growth feature at `service/entitlements.go:134` and shown on the pricing page, and **nothing consumes it** |
| Snooze | MISSING | Zero hits on support conversations |
| Tags | DONE | `model/support_tag.go` |
| Internal notes | DONE | `SupportMessage.IsInternal` |
| @mentions | DONE | With a nice touch: typing `@` in reply mode auto-switches to note mode (`ReplyComposer.tsx:1317-1320`) |
| Canned responses with variables | DONE | `!`-triggered, `{{customer.first_name | fallback: "there"}}` (`shortcutVariables.ts`) |
| Macros (text plus side effects) | MISSING | Canned responses insert text only |
| Conversation custom attributes | MISSING | No table, no field definitions |
| Views / saved filters | DONE | `model/support_inbox_view.go`, with per-view unread and needs-reply counts |
| Search | DONE, Postgres FTS | `websearch_to_tsquery` plus `ts_rank_cd`, hand-weighted composite at `repository/support_inbox.go:1556-1580`. No ClickHouse, no vector search for conversations |
| Keyboard shortcuts | PARTIAL | Composer and modals only. No `j`/`k`, no assign, no resolve, no cheatsheet |
| Collision detection / presence | DONE, advisory | WS `support:viewing:start`, typing with live draft preview, presence snapshot on connect. No blocking "X is also replying" warning |
| Separate ticket object | N/A by design | `SupportTicket` was renamed to `SupportConversation` in `migrations/039`. Legacy `/api/support/tickets/*` routes remain as aliases |
| Side conversations | MISSING | |
| Merge / split | MISSING | |
| Spam | PARTIAL | Status plus manual toggle. No blocklist, no classifier, no auto-spam |
| Bulk actions | MISSING | No multi-select, no bulk endpoint |
| Unread tracking | DONE | `SupportConversationUserState` with a relevance mask (`Assignee`, `Opener`, `Mention`) so unread only accrues for relevant agents. The plan doc is marked Superseded because it shipped and evolved |
| Live translate | PRD-ONLY | `docs/prds/support-live-translate.md`, 753 lines. No table, no route, no provider |
| AI rewrite for agents | DONE | Five operations: expand, rephrase, fix_grammar, more_friendly, more_formal |
| AI summarize thread | MISSING | Notable given handoff is the core flow |
| Triage and routing | DONE | All four PRD phases. `service/support_inbox_triage.go`, 1,405 lines. Rules layer, AI classifier with JSON schema, auto-move, feedback capture, input hashing for cost control |
| Agent availability / away mode | PARTIAL | Status model and UI shipped (`SidebarAccountMenu.tsx:96-133`). Team overrides and availability-aware routing still missing |
| Reporting on conversations | MISSING | See section 7 |
| Realtime WebSocket | DONE | |

**Correction to note.** Two of my sub-audits disagreed on AI stuck detection, so I checked. `docs/prds/support-ai-stuck-detection-and-handoff.md` describes the feature, and `server/internal/service/support_ai_escalation.go` implements it thoroughly with 665 lines of unit tests. But `evaluatePreLLMEscalation` (line 62) and `detectStuckOnSameIssue` (line 238) have **no production callers**. A repo-wide grep returns only their definitions and their tests. The live pipeline at `service/support_chat.go:144` calls only `checkHardEscalation`, plus turn caps and budget gates. **The headline feature of that PRD is written, tested, and dormant.**

---

## 4. Channels

Helpin ships **two customer channels**: web widget and email. That is the single biggest gap versus Intercom.

| Channel | Status |
|---|---|
| Web chat / widget | DONE |
| Email | DONE, Postmark only |
| Help center self-serve | DONE |
| WhatsApp, SMS, Instagram, Facebook Messenger, Telegram, Discord, voice, X | MISSING. Every grep hit is an icon name, a user-agent string, or a CRM social-profile URL field |
| Slack, Mattermost | PRD-ONLY. `docs/plans/2026-08-18-two-way-support-chat-integrations-plan.md` estimates 42 to 55 engineering days. Zero lines written. `docs/mattermost-integration.md` is an older, divergent design that proposes River Queue, which is not in `go.mod`. It should be retired |
| API channel | Reserved enum, unreachable. `api` appears in the channel comment and in triage logic, but nothing writes it and neither create request exposes a `channel` field |
| Gmail sync | DONE, but CRM-scoped. Mail lands on contacts and deals, not support conversations (`docs/email-architecture.md:14-19`) |
| Outlook / Microsoft 365 / IMAP | MISSING |

**Email is the most mature subsystem in the repo.** Inbound webhook at `handler/webhook_postmark.go`, threading with full `In-Reply-To` and `References` chains (`email_fallback.go:2628`), inbound and outbound attachments with CID inline image rewriting, HTML sanitization via bluemonday with remote images neutered behind a backend proxy (`email/inboundhtml/convert.go`, `EmailImageProxyHandler`), reply-above-the-line stripping with three fallback layers, custom domains with real DKIM and return-path verification through Postmark's Account API, custom sender addresses with a closed-loop forwarding probe, and a delayed-send queue with Redis leases, a poller, a reconciler, and a recovery CLI.

**There is no SMTP.** `grep -rn "net/smtp" server/` returns nothing. `server/internal/email/` contains only `postmark.go`, `postmark_domains.go`, `template.go` and the inbound HTML converter. `go.mod` has no email library at all. Inbound additionally depends on Helpin-owned domains `replies.helpin.email` and `*.on.helpin.email`, so even a self-hoster with their own Postmark account cannot receive support email. This is deliberate: `docs/prds/custom-support-sender-addresses-mvp.md:13` puts custom SMTP out of scope.

`docs/prds/support-live-chat.md` claims Phases 3 to 5 are "Not Started". That is wrong. The functionality shipped. Anyone assessing the product from that doc will materially undercount it.

---

## 5. AI support agent

This is the strongest part of the product, and it is genuinely customer-facing. The path is verifiable end to end: `service/support_inbox_widget.go:762` publishes to JetStream on every visitor message, `email_fallback.go:1372` does the same for inbound email, `cmd/api/main.go:1719` starts the consumer, `service/support_chat.go:83` runs the turn as a long-lived agent-runtime chat run, and `internal_command_support_reply.go:305` publishes the reply as a real message.

**Separation from the internal coding agent is explicit and defensible.** `server/internal/agentcontract/runtime_profiles.go:41` gives the `support_agent` preset (persona "Echo") `AllowedTargetTypes: ["support_conversation"]` only, an empty `AllowedCommands` list so shell is disabled, and `RequiresRepo: false`. The coding presets get full shell and require a repo. Output is server-mediated: plain assistant text is never delivered to a visitor.

**Providers: three.** `anthropic` (`llm/claude.go`), `openai` (`llm/openai.go`, a generic OpenAI-compatible client), and `openrouter` (the same struct pointed at OpenRouter). Ollama, Vertex, Bedrock, Azure, Groq, Mistral and Gemini have **no provider files**. Gemini and DeepSeek are reachable only as OpenRouter routes. Base URLs are overridable by env (`ANTHROPIC_BASE_URL`, `OPENAI_BASE_URL`), so an operator could point at a local model, but there is no product surface for it and the model must exist in the pricing catalog or execution fails closed.

**BYOK is MISSING.** No model, no migration, no handler, no UI. `aiusage/types.go:28` defines `FundingCustomer = "customer_funded"` and nothing ever emits or consumes it.

**Model selection is tier-based.** Users pick small, medium, large or flagship; `service/agent_model_tiers.go:13` maps each to a route. All four tiers are pinned to OpenRouter, so a workspace cannot choose Anthropic direct from the UI.

**Embeddings: OpenAI `text-embedding-3-small`, 1536 dims, pgvector.** Eight tables carry `vector(1536)`. If `OPENAI_API_KEY` is unset, `embeddingProvider` is nil and all semantic retrieval silently degrades to English-only lexical search with a per-query WARN and no alarm.

**Retrieval is sophisticated.** LLM query planning emits route, decision, intent, language, risk and up to N search queries. Chunking is structure-aware and preserves heading paths, with a separate `search_content` field so lexical matching sees the heading while customer-facing evidence stays clean. Hybrid search fuses pgvector cosine with weighted Postgres FTS across three pools (curated guidance, docs chunks, crawled chunks). A cross-encoder reranker speaks the HuggingFace TEI contract with a 250 ms hard timeout that fails open. Neighbor expansion pulls adjacent chunks. Evidence rows get stable IDs that `send_reply` **re-validates server-side**, so fabricated citations simply drop out.

**Knowledge sources.** Docs spaces DONE. URL crawler DONE (`server/internal/crawler`, three backends, sitemap and sitemap-index discovery). File upload DONE for PDF, DOCX, MD, TXT, CSV, JSON. Curated guidance DONE. HelpScout Docs import DONE and complete (paginated, rate-limit aware, image rehosting, resumable, provenance-tracked). Nextra/MDX import DONE. **Notion, Confluence, Zendesk and Intercom import are all MISSING**, which matters because `docs/strategy/pricing-strategy.md:128` sells "Import from Jira, Notion, Intercom, HubSpot" as a Growth-plan feature.

**Confidence is computed server-side, not trusted from the model.** `support_ai_confidence.go:16` weights retrieval quality 0.40, source coverage 0.25, LLM confidence 0.20, and can-answer 0.15, with authority floors of 0.90 for curated guidance and 0.85 for canonical pricing pages. Only chunks the model actually cited contribute. The prompt explicitly tells the model its confidence is a proposal.

**Escalation is layered:** hard phrase match pre-LLM, state gates, turn caps (30 absolute, `AIMaxFollowups` per issue), budget and provider failures, a post-answer server gate that re-runs numeric and grounding validation, and model-initiated `escalate_to_human`. A human taking over mid-turn suppresses an in-flight reply rather than publishing it.

**Resolution detection DONE, both modes:** confirmed via an affirmative classification, and assumed via a 24-hour idle timer, with the distinction preserved on the record. A proactive follow-up sequence nudges twice then closes.

**AI usage metering is the most complete subsystem in the audit.** An immutable embedded pricing catalog in integer micro-USD, a documented weighting formula, per-feature floors, preflight plus settlement, idempotency keys, enforcement inside the transaction that locks the billing row, and rollback of the ledger, counter and invoiced blocks together if a Stripe charge fails. Every AI action is a code-versioned policy object with modality, category, autonomy and data class. Even reranking is policy-gated. Embeddings are the documented exception: they are not metered.

Gaps in the AI layer, ranked:

1. **No prompt-injection defense.** Grep for `prompt.?injection` and `jailbreak` returns zero. Crawled third-party pages and uploaded PDFs enter the prompt as evidence with no instruction stripping. The evidence-revalidation gate limits the blast radius but does not prevent behavior or tone manipulation.
2. **The crawler ignores `robots.txt` Disallow.** `crawler/sitemap.go:88` fetches robots.txt only to harvest `Sitemap:` lines. `crawler/sitemap_test.go:46` uses a fixture containing `Disallow: /` purely to assert no sitemap is found. Customers point this at third-party domains.
3. **No automated eval harness.** No golden set, no scored offline runs, no CI quality gate. `docs/prds/knowledge-retrieval-platform.md:17` calls evaluation launch-critical.
4. **PII redaction is name-based.** `stripConversationPII` redacts the known customer email and phone plus one generic regex, outbound only. Inbound visitor content reaches the model unredacted.
5. **Preview diverges from production.** `PreviewSupportReply` exercises the legacy in-process pipeline; production runs the agent-runtime path. Admins tune against a system that is not the one answering customers.
6. **No explicit per-conversation AI on/off.** Takeover is implicit: an agent must reply to silence the AI. They cannot observe while muting it.
7. **No workspace-authored custom actions.** No HTTP-webhook action builder equivalent to Fin Custom Actions. External MCP is the closest analog and is not in the support preset's default allowlist.
8. **No structured tone or brand-voice settings.** A workspace must edit raw prompt text.

---

## 6. Help center

DONE, and the second most mature module. `help-center/` is a **TanStack Start** app with real SSR, served by a hand-rolled Node server (`help-center/serve.mjs`) doing brotli negotiation, basepath asset rewriting and a shared render cache. `docs/prds/help-center-ssr-migration.md` is still marked Draft and describes the migration as future work. It shipped. The doc is stale.

DONE: collections and spaces, TipTap authoring, multilingual with structure-preserving LLM translation (`server/internal/docsi18n`, with extract, reinsert, protected terms and validation, plus bulk auto-translate), custom domain in three modes including reverse-proxy with on-demand TLS, SEO and sitemap and robots, article feedback that feeds the coverage product, Postgres tsvector search plus pgvector hybrid plus cited AI answers, versioning with snapshots and revert and draft preview, redirects with slug aliases and SSR resolution, and full theming.

MISSING: **access restriction**. `DocsHelpcenterConfig` (`model/docs.go:560-596`) has no audience or authentication field. There is no private or logged-in-only help center. The public routes are gated only by `IsPublished`.

MISSING: Zendesk, Intercom, Notion and Confluence import.

---

## 7. Outbound / proactive messaging, and reporting

**Outbound messaging: MISSING, entirely.** No in-app posts, no banners, no tours, no checklists, no surveys, no chat campaigns, no email campaigns, no series or journeys. Confirmed by keyword sweeps across `server/internal`, `frontend/src` and `widget/src`. Every hit is unrelated: websocket broadcast, PM task checklists, UI banners. The only proactive thing is a static greeting bubble in the dead legacy widget.

"Automation" in this repo means **internal workflow automation that launches AI agent runs**. `docs/automation-product-model.md` states the model as "When X happens, run Y agent on Z". The full trigger catalog is `server/internal/automationcatalog/triggers.go`: manual runs, PM task state changes, agent lifecycle, doc publish, GitHub and GitLab events, `support.widget_message` (inbound, routes a message to an AI agent), and cron. It never sends a message to a customer.

`docs/customer-io/` is **Helpin's own growth stack, not a product feature**. `service/customer_io.go` ships Helpin's SaaS lifecycle events to Customer.io. The docs are campaign specs to be built in Customer.io's UI. There is an irony visible in `docs/strategy/early-access-email-campaign.md`: the pitch names Intercom as a tool Helpin replaces, while the outbound half of Intercom is not built.

**Support reporting: MISSING across the board.**

| Metric | Status |
|---|---|
| CSAT reports | MISSING. Capture itself is a stub with a disabled toggle |
| First response time | MISSING |
| Resolution time | MISSING. `resolved_at` exists, nothing aggregates it |
| Conversation volume | MISSING |
| Agent performance | MISSING |
| AI resolution rate | MISSING as a metric. `resolved_by_ai` exists only as an inbox filter |
| Data export / CSV | MISSING. No `text/csv` response anywhere |
| Workspace dashboard | MISSING. `frontend/src/pages/Dashboard.tsx` is 18 lines and redirects |

The only real report in the product is PM sprint closeouts. Velocity and cycle time are hardcoded `available: false` cards at `Reports.tsx:20-33`.

`events-pipeline/` is a Rust plus NATS JetStream plus ClickHouse product-analytics ingester forked from Usermaven. It handles pageviews, UTM, identity and sessions. ClickHouse holds two tables and one materialized view. **There are no support or CSAT fact tables in ClickHouse.** Its only Go consumer is CRM usage baselines.

What does exist is `support_coverage`, roughly 30 service files plus a full UI at `/w/$slug/support/coverage`. It measures **knowledge gaps, not team performance**: seven signal sources feed clustering, semantic dedup, knowledge matching, gap emission, nightly LLM enrichment, AI-drafted doc fixes, human review, and publish. It is end to end except for post-fix impact measurement, which the PRD calls for and which does not exist.

---

## 8. Data / CRM

**There is no unified person record.** Three separate identities exist, stitched loosely by email and `anonymous_id`: `CRMContact` (`model/crm_contact.go`), `SupportWidgetSession` (`model/support_inbox.go:518`), and denormalized customer fields on `SupportConversation` itself. There is no visitor or end-user table. Durable cross-visit identity is a cookie value, not a row.

**Custom attributes are an untyped JSONB blob.** `CustomProperties map[string]interface{}` on contacts and companies, with no type and no validation. `server/migrations/026_crm_properties_lists.sql` defines `crm_property_definitions` with `field_type`, `options` and `is_required`, plus `crm_lists` and `crm_list_members` with static and dynamic list types. **All four tables have zero code references** in Go, TS or TSX. Typed attributes and segments were schema'd and never built.

**GDPR: MISSING.** No export endpoint. Deletion is a plain row delete (`repository/crm_contact.go:243`) with no cascade or purge. Conversations, messages, activities, support events, widget sessions and ClickHouse events all retain the person's email and name. The only GDPR mention in all of `docs/` is a Medium-priority cookie-consent row in the widget parity doc.

**Audit logs: PARTIAL.** Admin requests produce structured log lines (`middleware/admin_audit.go`), not a queryable table. MCP has its own activity trail. There is no workspace-level audit of who changed a setting or who read a conversation.

**Visitor to lead conversion: DONE**, and beyond its PRD. `matchOrCreateCRMContactIdentityTx` (`service/support_inbox.go:3381`) matches by email, promotes lifecycle without downgrading, backfills prior anonymous conversations by `anonymous_id`, tracks identity provenance and trust, and infers company from email domain with a free-provider blocklist. `docs/prds/widget-identify-crm-leads.md` is still marked Draft.

---

## 9. Platform

| Capability | Status |
|---|---|
| Versioned public REST API | MISSING. No `/api/v1`, no version prefix |
| OpenAPI spec for Helpin's own API | MISSING. All openapi code is a *Docs feature* that renders customers' uploaded specs |
| API keys / personal access tokens | MISSING. MCP service tokens only |
| Inbound webhooks | DONE. Postmark, Stripe, meeting capture, git |
| Outbound webhooks for customers | MISSING. No subscription, no signing secret, no retry |
| Public MCP server | DONE, beta, unpublished. ~48 tools, OAuth 2.1 with PKCE and dynamic client registration, per-tool RBAC and module checks. Support tools are **read-only**, with internal notes explicitly excluded (`mcp_catalog.go:178`) |
| Password, Google OAuth, TOTP 2FA, passkeys | DONE |
| SAML / OIDC / SCIM / enforced SSO | MISSING. Zero non-test hits |
| RBAC | DONE, fixed. Four roles, 39 permissions, code-defined in memory. Support has exactly three: `support.read`, `support.edit`, `support.admin` |
| Custom roles, record-level scoping | MISSING |
| Workspaces | DONE. Organizations are a billing-only shim |
| Billing | DONE. Stripe, three plans in code (`starter`, `growth`, `founder`), seat limits, AI usage metering, and a `RequireUnlockedWorkspace` middleware that locks unpaid workspaces at the router |
| Rate limiting | PARTIAL. Redis limiters on public widget and help-center routes only. **None on the authenticated `/api` surface and none on `/mcp`** |
| Observability | PARTIAL. Sentry only. Every OTel entry in `go.mod` is marked indirect. No Prometheus, no `/metrics` |
| Module access control | DONE for its V1 scope |

**Can support run standalone? No.** `server/internal/authorization/authz.go:134-137` unconditionally grants PM and Docs to every actor, and lines 141-146 give owners and admins all five modules regardless of grants. You can hide CRM, Support and Automation from a member. You cannot hide PM or Docs from anyone, and there is no workspace-level "this tenant bought Support only" entitlement. `docs/plans/module-access-progress.md` confirms this is deliberate V1 scope. Packaging Support standalone requires workspace-level module entitlements tied to the plan, which do not exist.

---

## 10. Self-hosting

**There is no LICENSE file.** `find . -maxdepth 2 -iname 'LICENSE*'` returns nothing first-party. No CONTRIBUTING, no SECURITY, no CODE_OF_CONDUCT. Legally nobody may self-host this, which makes everything below moot until it changes.

Blockers after that, in order:

1. **`agent-runtime` is not in this repo.** `docs/agent-runtime-local-setup.md:4` states it is "the only agent executor" and `:41` points to a separate repository. `AGENT_RUNTIME_LAUNCH_ENABLED=false` disables agent execution and explicitly does not restore an in-process fallback. **Every AI feature, including the AI support agent, is unavailable to a self-hoster.**
2. **Postmark is the only email transport.** No SMTP, no provider interface for support email. Inbound also needs Helpin-owned domains.
3. **`docker compose up` gives you no application.** `server` and `frontend` are commented out at `docker-compose.yaml:142-190`. You get Postgres, a second Postgres for Temporal, Temporal, its UI, pgAdmin, NATS and Redis, plus a temporal-worker.
4. **Roughly 180 hardcoded `helpin.ai` and `helpin.email` references**, including `email/template.go` (logo and two brand links in every outbound email) and the widget install snippet handed to customers in `ChatGeneralTab.tsx`.
5. **The k8s manifests are Helpin's own**, pinned to private GHCR images with Doppler and sealed secrets. No chart, no Kustomize base.
6. **Six to nine moving pieces** with no documented topology. There are 90-plus files in `docs/` and not one installation guide.

Only two env vars hard-fail: `DATABASE_URL` and `JWT_SECRET` (`config.go:220-231`). Credential-gated graceful degradation is genuinely good: Stripe, S3, LLM keys, Customer.io and Usermaven all no-op when unset. But `server/.env.example:50` ships a **live-looking Sentry DSN pointing at Helpin's own Sentry**, so a self-hoster who copies the example phones home.

Realistic verdict: months of work, and the remaining work is architectural rather than cosmetic.

---

## 11. What the team has already written down

Useful because it tells you what they know versus what they have not noticed.

- **`docs/widget-feature-parity.md`** (2026-03-22, vs Crisp, stale). Ultra-high: online status, identity verification, push. High: emoji picker, GIF picker, audio messages, article search. Medium: attachments, read receipts, lightbox, message editing, quick replies, carousels, link previews, **GDPR/cookie consent**, conversation search, citation display. Its verdict: "feels like a 2015-era live chat."
- **`docs/prds/widget-sdk-feature-parity.md`** scores Helpin **17 of 68** against Crisp's 66. Four categories are flagged CRITICAL: Identity and Session, Connection and Transport, Loading and Performance (0 of 7), CSS and DOM Isolation. Eight MUST-have items. The session row is now stale, superseded by a 2026-07-14 amendment.
- **`docs/plans/support-availability-and-notifications.md`** self-reports three items not complete: the human live status model, team-specific availability overrides, and availability-aware routing. The status model has since shipped. The other two have not.
- **`docs/strategy/2026-06-11-coverage-gaps-first-class.md`**: "the detection half is excellent; the resolution half is manual and the loop never closes." Names five gaps including no impact scoring, an unused `SupportCoverageDigestDelivery` model, and an unmetered daily LLM sweep.
- **`docs/strategy/2026-06-11-product-engineering-assessment.md`**: "a sophisticated, AI-first backend wearing an unfinished product." Its Tier-1 claim that billing is absent is now stale. Still accurate: zero reporting across all modules, no rate limiting, no public API docs, no outbound webhooks, no competitor import ("switching cost kills sales"), no onboarding sample data.
- **`docs/strategy/backlog-and-ideas.md`** (2026-03-03) is badly stale. It is a PM-module backlog written before Support and CRM existed, and several "Open" items have shipped.
- **`docs/prds/2026-03-05-product-review.md`** is very stale. It describes a codebase with zero Go tests, which is no longer true.
- **33 docs mention Intercom.** The most substantive: `docs/prds/ai-support-agent.md` borrows Fin's model wholesale (AI state separate from human status, confirmed versus assumed resolution, reopen deducts from metrics). `docs/prds/widget-messenger-security-jwt.md` recommends Intercom's JWT Messenger Security as the fix for the identity-verification gap. `docs/plans/2026-06-12-support-triage-fin-level-plan.md` is an explicit Fin gap analysis.

**Gaps the docs never name**, which this audit surfaced: no unified person record, typed custom attributes shipped as dead tables, no segments, no GDPR export or deletion, no general audit log, and no license.

---

## 12. The ten findings I would act on first

1. **No LICENSE file.** Blocks any open-source or self-host story.
2. **`allowed_origins` is enforced nowhere.** Validated, stored, and ignored. Widget CORS is `*` and the WS upgrader skips origin verification. Anyone with a widget key can boot the widget from any domain.
3. **AI stuck detection is dead code.** `support_ai_escalation.go:62` and `:238` are implemented and tested with no production caller.
4. **`EntitlementFeatureSLAPolicies` is sold and unbuilt.** Billed as a Growth feature, shown on the pricing page, consumed by nothing.
5. **The crawler ignores `robots.txt` Disallow.** Customer-configured crawls against third-party domains are a legal exposure.
6. **No prompt-injection defense** on crawled pages and uploaded PDFs entering the agent's context.
7. **Zero support reporting.** No CSAT, no first response time, no resolution time, no volume, no agent performance, no AI resolution rate, no export. CSAT capture itself dead-ends in an analytics event behind a disabled toggle.
8. **New widget installs default to `identity_verification_mode = enforced`** with no UI to see the secret key, no UI to change the mode, and no public documentation of the HMAC construction.
9. **Priority is write-once.** Settable on create, filterable in search, with no update route.
10. **Three stale docs will mislead anyone assessing the product**: `support-live-chat.md` claims Phase 3+ not started, `help-center-ssr-migration.md` is marked Draft for a shipped migration, and `docs/mattermost-integration.md` proposes a queue library that is not a dependency.

Versus Intercom specifically, the shape is clear. Helpin's AI answering engine, retrieval pipeline and coverage loop are competitive or ahead. It is missing Intercom's breadth: channels beyond widget and email, the entire outbound messaging product, all support reporting, CSAT, ticket types, side conversations, merge, bulk actions, snooze, macros with side effects, conversation custom attributes, SSO, and a public API with outbound webhooks.
