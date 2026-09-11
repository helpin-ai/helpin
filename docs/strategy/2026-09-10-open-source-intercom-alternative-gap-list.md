# Helpin as an open-source Intercom alternative (local / BYOK) — verified gap list

Date: 2026-09-10
Scope: what is missing in `helpin/` (and its hard dependency `agent-runtime/`) to launch as an open-source, self-hostable Intercom + Fin alternative with local or bring-your-own-key LLMs.

Method: four independent passes, each claim cited.
- Intercom platform inventory from intercom.com/help, developers.intercom.com, intercom.com/pricing, intercom.com/changes (September 2026).
- Fin AI Agent inventory from fin.ai, intercom.com/help, Intercom blog and changelog.
- Code audit of `helpin/` (1,624 Go files, 309k non-test LOC, 316 SQL migrations, 6 frontend support routes).
- Code audit of `agent-runtime/` (212 Go files, 65k LOC) and the Helpin ↔ runtime call chain.
- Ten of the highest-impact code claims were re-verified by hand (grep/read) before this list was written.

Nothing below is proposed because Intercom has it. Items are included only if (a) they block shipping as open source, (b) they are required for the local/BYOK claim to be true, or (c) they are day-one expectations of anyone leaving Intercom for a support inbox plus AI agent. Breadth Intercom has that does not meet that bar is listed once, at the end, as explicitly deferred.

---

## 0. Context that changes the pitch

- **Intercom renamed itself Fin on 2026-05-12, and Salesforce signed to acquire Fin for ~$3.6B on 2026-06-15** (close expected Salesforce FQ4 2027). Intercom's product marketing pages now redirect to fin.ai or 404. This is a real switching moment for Intercom customers who do not want a Salesforce-owned helpdesk.
- **Fin's pricing**: $0.99 per outcome (resolution, procedure handoff, disqualification), 50-outcome monthly minimum, seats $29/$85/$132 per seat/month annual, Copilot $29–35/agent, Pro insights add-on from $99/month. A BYOK self-hosted agent competes on cost with no per-resolution fee.
- **Open-source landscape**: Chatwoot's AI agent (Captain) is Enterprise-Edition-only for self-hosters. Zammad and FreeScout ship agent-assist only. Only Libredesk (AGPL, Go, beta AI) and Tiledesk ship a customer-facing autonomous agent with first-class Ollama. There is no mature open-source "Fin-class" agent. That is the gap Helpin's support core already fills technically.

## 1. What Helpin already has that is competitive or ahead (do not rebuild)

Verified in code; these are the assets the launch should lead with.

| Area | Evidence |
|---|---|
| Customer-facing AI agent on widget and email | `service/support_chat.go`, JetStream `SUPPORT_AI` stream, `internal_command_support_reply.go` |
| Retrieval: LLM query planning, structure-aware chunking, pgvector + Postgres FTS hybrid, TEI-compatible cross-encoder rerank, neighbor expansion | `service/support_knowledge_search.go` and neighbours |
| Server-side confidence (weights 0.40/0.25/0.20/0.15) and citation re-validation that drops fabricated evidence | `service/support_ai_confidence.go:16`, `internal_command_support_reply.go:57-84` |
| Resolution accounting identical to Fin's model: confirmed vs assumed after 24h idle, distinction stored | `docs/PRD-ai-support-agent.md`, service layer |
| Layered escalation: hard phrases, turn caps, budget, post-answer grounding gate, model-initiated handoff, mid-turn human takeover suppresses in-flight reply | `support_chat.go:82-194` |
| AI usage metering with immutable pricing catalog, preflight + settlement, idempotency, Stripe rollback | `internal/aiusage`, `docs/ai-usage-metering.md` |
| Coverage gaps loop (unanswered questions → clustered gaps → AI-drafted article fixes → review UI). Fin sells this as the Pro add-on. | `service/support_coverage_*`, `/support/coverage` |
| Triage and routing (rules + AI classifier, feedback capture) | `service/support_inbox_triage.go` |
| Email threading, CID inline images, HTML sanitisation with proxied remote images, custom sender domains with DKIM verification, delayed-send queue with recovery | `service/email_fallback.go`, `email/inboundhtml` |
| Help center: TanStack Start SSR, multilingual with structure-preserving LLM translation, custom domain with on-demand TLS, versioning/revert, redirects, hybrid search with cited AI answers | `help-center/`, `internal/docsi18n` |
| Knowledge sources: docs spaces, URL crawler with sitemap discovery, PDF/DOCX/MD/TXT/CSV/JSON upload, curated guidance, HelpScout Docs import, Nextra/MDX import | `internal/crawler`, `internal/docsimport`, `internal/helpscout` |
| Widget: shadow-DOM Preact SDK, WebSocket realtime, bidirectional typing, conversation list, attachments via presigned S3, emoji, article search, office hours / reply-time expectations, offline email fallback | `packages/sdk-js`, `packages/widget-core` |
| Inbox: two-axis state (status + flow state), team mailboxes, views with unread/needs-reply counts, tags, notes with @mentions, canned responses with variables, Postgres FTS search, advisory collision presence, AI rewrite (5 ops) | `model/support_inbox.go`, `frontend/src/pages/support` |
| Billing (Stripe, plans, seats, workspace lock), TOTP + passkeys, RBAC, module access control | `internal/billingstripe`, `internal/totp`, `internal/webauthn`, `internal/authorization` |

Correction to the June 2026 assessment: billing is no longer absent and the support UI is no longer one page. Those findings are stale.

---

## 2. Tier 0 — Cannot ship as open source without these

Ordered. Each is verified missing or broken today.

1. **No LICENSE file** in `helpin/`, `agent-runtime/`, `agent-runtime-go/`, or `agent-runtime-python/`. Also no CONTRIBUTING, SECURITY, or CODE_OF_CONDUCT. Decision needed: AGPL-3.0 (Libredesk, Chatwoot-core style) vs Apache-2.0/MIT; the same choice must cover the runtime and both SDKs because the support agent cannot run without them.
2. **agent-runtime is a hard dependency of the customer-facing support AI and is not in the monorepo or its compose.** `service/agent.go:6496-6498` removes any local executor and fails loudly. `AGENT_RUNTIME_LAUNCH_ENABLED=false` disables AI entirely (`docs/AGENT_RUNTIME_LOCAL.md:19-21`). Neither `docker-compose.yaml` nor `k8s/` starts it. Fix is small: the runtime's floor config is one container, `AGENT_RUNTIME_STORE_DRIVER=sqlite`, no Temporal, no Postgres (`agent-runtime/cmd/agent-runtime/main.go:279-320`). Either vendor it into the monorepo or add it as a compose service and pin the image. A support-only runtime image should drop the coding toolchain (Go, Node, Rust, Codex, OpenCode, ffmpeg) that `agent-runtime/Dockerfile` bundles.
3. **`docker compose up` starts no application.** `server` and `frontend` are commented out at `docker-compose.yaml:142` and `:170`. You get Postgres ×2, Temporal, Temporal UI, pgAdmin, NATS, Redis, temporal-worker. Need a single-command path: app + runtime + Postgres + NATS + Redis (+ Temporal only if required; audit whether the support path needs Temporal at all, since support durability comes from JetStream).
4. **No SMTP.** `grep -rn net/smtp server/` is empty; `internal/email/` is Postmark-only and `go.mod` has no mail library. Inbound additionally requires Helpin-owned domains (`replies.helpin.email`, `*.on.helpin.email`). Minimum: an outbound transport interface with SMTP + Postmark implementations so auth emails, notifications, and support replies work on any host. Inbound via generic webhook/IMAP can be phase two; keep Postmark inbound as an optional provider.
5. **Widget origin allow-list is decorative.** `allowed_origins` is validated and stored (`service/support_widget_identity.go:121`) and only echoed back in `handler/support_inbox.go:1344,1377,1404`; all widget routes use `AllowedOrigins: "*"` (`router.go:168,185,228,373,405,418`) and the WS upgrader sets `InsecureSkipVerify: true` (`websocket/widget_handler.go:76,130`). Any site can boot any workspace's widget. Must be enforced before strangers self-host.
6. **New installs default to `identity_verification_mode = enforced`** (`support_inbox_settings.go:601`) with no UI to view the secret, no UI to change the mode, and no public doc of the HMAC construction. A fresh self-hoster's widget will reject users. Ship the settings UI and docs, and default new workspaces to `off` or `optional`.
7. **Phone-home and branding hardcodes.** `server/.env.example:50` ships a live Sentry DSN pointing at Helpin's Sentry. ~180 hardcoded `helpin.ai` / `helpin.email` references including `email/template.go` (logo and links in every outbound email) and the install snippet in `ChatGeneralTab.tsx`. Make base URL, brand name, logo, and asset host configuration.
8. **No installation guide.** 90+ files in `docs/`, none describe topology or a first-run path. Needs: required services, env reference (the `.env.example` is close), migration commands, runtime pairing, first-workspace bootstrap.
9. **Support cannot be packaged standalone.** `authorization/authz.go:134-146` unconditionally grants PM and Docs to every actor and all modules to owners/admins. An OSS "support-only" install will expose PM/CRM/Automation UI. Add a deployment-level module toggle (env or workspace entitlement) so the OSS build can present Support + Help Center + Agents only. Docs module is needed by the help center, so keep it.

---

## 3. Tier 1 — Required for the "local / BYOK" claim to be true

Today both `helpin/server/internal/llm` and `agent-runtime` support exactly three providers: Anthropic, OpenAI-compatible, OpenRouter. Keys are process-global env only. No Ollama, Gemini, Bedrock, Vertex, Azure, Groq, Mistral files exist in either repo.

1. **Per-workspace provider credentials (BYOK).** Missing in Helpin (no model, migration, handler, or UI; `aiusage/types.go:28` defines `customer_funded` and nothing uses it) and in agent-runtime (keys only from env, `native_eino_provider.go:70-96`; run-scoped credentials exist only for MCP servers, `internal/mcp/run_config.go`). Needed: encrypted per-workspace provider config (provider, base URL, key, default model, embedding model) in Helpin, passed per-run to the runtime the same way MCP credentials already are, and honoured in `resolveProviderAndModel` (`native_eino_provider.go:202-220`). Existing `internal/crypto` AES-GCM helper covers storage.
2. **Ollama / OpenAI-compatible Chat Completions path in agent-runtime.** The runtime's OpenAI path uses `agenticopenai.NewResponsesModel`, i.e. the Responses API (`native_eino_provider.go`), which Ollama, vLLM, LM Studio and most gateways do not implement. Helpin's own `internal/llm/openai.go` speaks Chat Completions and would work, but the customer-facing agent does not run through it. Add a Chat Completions provider (Eino has one) and an explicit `ollama` provider entry so it appears in `/capabilities`.
3. **Embedding provider and dimension are hardcoded.** Embeddings are OpenAI `text-embedding-3-small` only (`llm/support_router.go:51-56`); nine `vector(1536)` columns across migrations. Local models (nomic-embed-text 768, bge 1024, etc.) cannot be used. Needed: embedding provider config per workspace, dimension stored per index with re-embed job, and a loud degradation alarm (today a missing key silently drops to lexical search with a WARN).
4. **Model selection UI is pinned to OpenRouter.** All four tiers in `service/agent_model_tiers.go:15-24` are `Provider: "openrouter"`. A workspace cannot pick Anthropic direct, OpenAI direct, or a local model from the product. Replace tier→OpenRouter mapping with tier→(workspace provider, model).
5. **Metering fails closed on unknown models.** Execution fails if the model is not in the embedded pricing catalog (audit of `internal/aiusage`). Local and BYOK models must be allowed at zero cost, with metering recorded as `customer_funded`.
6. **Reranker is already local-friendly** (HuggingFace TEI contract, fails open at 250 ms). Document how to run TEI locally; no code needed.
7. **Preview must run the production path.** `PreviewSupportReply` (`service/support_ai_admin.go:378-509`) exercises the legacy in-process pipeline while customers are answered by the runtime path. Admins tuning a local model would be testing the wrong system.
8. **Runtime web search and browser keys** (Exa/Brave/TinyFish, Kernel) are optional and already degrade gracefully. Document as optional.

---

## 4. Tier 2 — Day-one expectations for a team leaving Intercom's inbox + Fin

Each verified missing, partial, or dormant. Grouped, ordered by leverage.

### 4a. Measure the thing you are selling
1. **CSAT is a stub.** Widget UI exists (`CsatRating.tsx`) but submission only fires an analytics event (`widget.ts:1509-1527`); no table, no route (`grep -i csat router.go` is empty); admin toggle hard-disabled with "will be available soon" (`ChatGeneralTab.tsx:1699`). Intercom: 5-emoji rating + comment, triggerable after resolution, reported per human/AI.
2. **Zero support reporting.** No volume, first response time, resolution time, agent performance, AI resolution rate, AI involvement rate, or CSV export (`Dashboard.tsx` is an 18-line redirect; no `text/csv` anywhere). Fin's headline metrics are **resolution rate** and **involvement rate**; Helpin already stores `resolved_by_ai`, `resolved_at`, confirmed vs assumed. The data exists; the aggregation and one page do not. Minimum: one support overview page with those six metrics, date range, team/agent breakdown, CSV export. ClickHouse is not needed for this at launch volume.
3. **AI stuck detection is dead code.** `support_ai_escalation.go:62` (`evaluatePreLLMEscalation`) and `:238` (`detectStuckOnSameIssue`) are implemented with 665 lines of tests and have no production caller; the live path calls only `checkHardEscalation`. Wire them in or delete them.

### 4b. Inbox basics that Intercom users will hit in the first hour
4. **Snooze**: zero hits in support models/handlers.
5. **Priority is write-once**: settable on create, filterable, no update route.
6. **Bulk actions**: no multi-select, no bulk endpoint (Intercom: 1,000 at a time).
7. **Macros with side effects**: canned responses insert text only; Intercom macros assign, tag, close, set priority.
8. **Conversation custom attributes**: no table or definitions (Intercom "conversation data" is used by routing and reporting).
9. **AI summary of a thread on handoff**: missing, despite handoff being the core loop. `draft_support_reply` and rewrite exist; summarise does not.
10. **Per-conversation AI mute**: takeover is implicit (an agent must reply to silence the AI). Add an explicit "pause AI on this conversation" toggle.
11. **Keyboard shortcuts** beyond the composer (`j/k`, assign, resolve, cheatsheet). Intercom's inbox is Command-K driven.
12. **Availability-aware routing**: round-robin (`repository/support_mailbox.go:581-621`) ignores presence and only fires on mailbox move or AI handoff. The status model shipped; routing does not consult it (`docs/TODO-support-availability-and-agent-notifications.md`).
13. **SLA policies are sold and unbuilt.** `EntitlementFeatureSLAPolicies` is gated as a Growth feature (`service/entitlements.go:134`) and shown on the pricing page; nothing consumes it. Either build first-response/next-response/time-to-close targets with breach events, or remove it from pricing before an OSS audience reads the code.

### 4c. Widget correctness and parity
14. **Custom visitor attributes are dropped.** `WidgetUser.metadata` is declared (`widget.ts:34`) and never serialised; the identity payload is a fixed field set (`model/support_inbox.go:1128-1139`). Intercom custom data attributes are the basis of targeting, routing, and Fin context.
15. **JWT messenger security.** Only HMAC v1 exists; Intercom's current recommendation is JWT with expiry and trusted domains, and `docs/PRD-widget-messenger-security-jwt.md` already specifies it. Pairs with Tier 0 item 6.
16. **Widget i18n**: every string is an English literal; `navigator.language` is captured and unused. Intercom ships 45 languages, and Helpin's help center is already multilingual.
17. **Launcher `bottom-left` is broken** (`widget.css:324-327` hardcodes `right: 20px`).
18. **SDK ships unminified at 426 KB** (`packages/sdk-js/vite.config.ts:50` `minify: false`).
19. **Visibility rules** (show launcher by URL/attribute/audience): only imperative `show()/hide()` today.
20. **Pre-chat form custom fields**: fixed email then phone only.
21. **Read receipts to the visitor** and a **sound toggle**: partial.
22. Attachment limit mismatch: backend 100 MB (`support_attachment.go:17`) vs admin copy 10 MB (`ChatGeneralTab.tsx:1686`).

### 4d. Safety and legal hygiene an open-source audience will find
23. **Crawler ignores `robots.txt` Disallow** (`crawler/sitemap.go:88` reads robots only for `Sitemap:` lines). Customers point it at third-party domains.
24. **No prompt-injection hygiene** on crawled pages and uploaded PDFs entering the agent context (`grep prompt.?injection|jailbreak` returns nothing). The evidence re-validation gate limits fabricated citations but not tone/behaviour manipulation. Minimum: mark retrieved content as untrusted data in the prompt contract and strip instruction-like spans; add a red-team set to the eval harness (Tier 3).
25. **Inbound PII reaches the model unredacted**; `stripConversationPII` is outbound-only and name-based. Intercom offers PAN redaction plus 10 custom regex rules.
26. **No rate limiting on authenticated `/api` or `/mcp`** (Redis limiters exist only on widget and help-center routes).
27. **GDPR delete/export**: deletion is a plain row delete (`repository/crm_contact.go:243`) with no cascade across conversations, messages, widget sessions, events; no export endpoint. Intercom offers per-user permanent delete and CSV/JSON export. An OSS project hosted in the EU will be asked for this in week one.

---

## 5. Tier 3 — Fin-class agent features that make it a Fin alternative, not just an inbox

Post-launch, but these are what Fin buyers compare on. Ordered by cost-to-value given what already exists.

1. **Workspace-attached external tools for the support agent (Fin Data Connectors / MCP connectors).** The runtime's run-scoped MCP with OAuth and encrypted credentials is fully plumbed (`agent-runtime/internal/mcp/run_config.go`, `helpin service/agent.go:6541-6584`, `externalmcp/oauth.go`) but the `support_agent` preset ships zero `mcp__*` tools (`agentcontract/runtime_profiles.go:44`). Exposing "attach MCP servers to the support agent" in settings is the cheapest Fin-parity win in the codebase. A simple HTTP action builder (URL, auth, when-to-use description) can follow.
2. **Guidance as structured settings.** Fin: natural-language guardrails in categories with limits (100 items, 2,500 chars), tone (5 presets), answer length (3), per-guidance usage metrics, audience targeting. Helpin has curated guidance for retrieval but no structured behaviour/tone settings; workspaces edit raw prompt text.
3. **Evaluation harness.** No golden set, no scored offline runs, no CI gate; `docs/PRD-knowledge-retrieval-platform.md:17` calls it launch-critical. Fin ships Previews, Batch tests (50 questions, CSV), Simulations, and since Aug 2026 Evals + Releases (staged rollout, A/B, rollback). Start with batch tests over a CSV and a retrieval-quality score; it also gives BYOK users a way to compare local models.
4. **Knowledge imports from Intercom, Zendesk, Notion, Confluence.** All missing, while `docs/pricing-strategy.md:128` sells "Import from Notion, Intercom". An **Intercom articles + conversations importer** is the single highest-leverage switching tool given the Salesforce acquisition. The HelpScout importer is a complete template to copy.
5. **Procedures-lite.** Fin 3 replaced Tasks with Procedures (NL steps, conditions, connector calls, handoff, wait-for-webhook). Helpin's agent is already tool-driven; a workspace-authored "procedure" is a skill package plus allowed tools. Do after 1 and 2.
6. **Fin-style outcome attributes and escalation reporting**: Fin classifies issue type, sentiment, urgency, escalation reason per conversation for reporting. Helpin's triage classifier already emits structured JSON; persist and surface it.
7. **Fin Memory** (cross-conversation recall for returning customers): Helpin has carry-forward transcript within a run; cross-conversation memory is missing.
8. **Copilot for agents answering from knowledge** with citations in the composer (Helpin has rewrite and a draft tool; verify the draft tool is exposed in the composer UI).
9. **Coverage loop closure** per `docs/strategy/2026-06-11-coverage-gaps-first-class.md` P0/P1 (auto-verify deflection, `coverage.gap_detected` trigger). This is the "knowledge base fixes itself" story Fin's Operator now sells as Pro.

---

## 6. Explicitly deferred — Intercom breadth that should not gate the launch

Listed so nobody re-derives it. Ranked within the group by how soon it will be asked for.

1. **Public REST API with versioning, API keys, outbound signed webhooks.** All missing (`no /api/v1`, no PAT, no webhook subscriptions). Highest of the deferred set; the public MCP server (48 tools, OAuth 2.1) partly covers reads. Intercom: REST v2.16, 10k calls/min, 14 webhook topic families.
2. **Slack as a channel** (Intercom's is native; Helpin's plan at `docs/plans/2026-08-18-two-way-support-chat-integrations-plan.md` estimates 42–55 days). Retire `docs/mattermost-integration.md` (proposes River Queue, not in `go.mod`).
3. **WhatsApp, SMS, Instagram, Facebook Messenger, Telegram, Discord, phone/voice.** Zero code. Fin Voice is sales-only and custom-priced; not a launch target.
4. **Outbound / proactive suite**: chats, posts, banners, tooltips, tours, checklists, surveys, news, Series, email campaigns. Zero code (Helpin "automation" launches agent runs; it never messages a customer). Intercom gates most of this behind Proactive Support Plus at $99/month. Not part of "inbox + AI agent".
5. **Ticket types, ticket states, customer portal, side conversations, merge/split.** Helpin deliberately merged tickets into conversations (`migrations/039`).
6. **SAML/OIDC/SCIM, custom roles.** Zero code. Intercom gates SSO to Expert; OSS users will ask for OIDC eventually.
7. **Help center access restriction / private help center**: `DocsHelpcenterConfig` has no audience field.
8. **Typed custom attributes, segments, custom objects.** `crm_property_definitions`, `crm_lists`, `crm_list_members` exist as tables with zero code references.
9. **Unified person record.** Three identities (`CRMContact`, `SupportWidgetSession`, denormalised conversation fields) stitched by email and `anonymous_id`; no visitor table.
10. **Queryable audit log** (admin actions are log lines only), **Prometheus metrics** (Sentry only), **load-balanced assignment**, **live translate** (PRD only, 753 lines), **native mobile customer SDKs** (the Tauri apps are agent-facing), **multi-brand messenger/help center**, **Outlook/IMAP sync**.

---

## 7. Stale documents to fix before the repo is public

Anyone reading the repo will be misled by these.

- `docs/strategy/2026-06-11-product-engineering-assessment.md`: billing is present; support UI is more than one page.
- `docs/PRD-support-live-chat.md`: claims Phases 3–5 not started; they shipped.
- `docs/PRD-help-center-ssr-migration.md`: marked Draft for a shipped migration.
- `docs/PRD-widget-identify-crm-leads.md`: marked Draft; shipped beyond spec.
- `docs/PRD-support-ai-stuck-detection-and-handoff.md`: describes a feature that is implemented but not wired.
- `docs/mattermost-integration.md`: divergent design, proposes a dependency not in `go.mod`.
- `docs/BACKLOG-and-ideas.md` (March 2026) and `docs/PRD-current-state-review-2026-03-05.md`: pre-support-module, several "Open" items shipped, "zero Go tests" no longer true.
- `docs/widget-feature-parity.md` and `docs/PRD_WIDGET_SDK_FEATURE_PARITY.md`: benchmarked against Crisp; emoji picker and article search have since shipped. Reframe against Intercom if kept.
- `docs/pricing-strategy.md:128`: sells importers that do not exist.
- `docs/PRD-custom-support-sender-addresses-mvp.md:13`: puts SMTP out of scope; must be reversed for self-hosting.
- Dead code to remove before publishing: `widget/` (2,132-line vanilla predecessor, built by nothing), `packages/widget-embed` (abandoned).

---

## 8. Suggested launch sequence

1. Tier 0 in full. It is mostly configuration, packaging, licensing, and two security fixes.
2. Tier 1 items 1–5 (BYOK model, Chat Completions provider, embedding abstraction, model picker, zero-cost metering). This is what makes "local / BYOK" a true statement.
3. Tier 2 4a (CSAT + one reporting page + wire stuck detection) and 4d (robots, injection hygiene, rate limits, GDPR delete). These are what an open-source audience audits first.
4. Tier 2 4b/4c as a rolling inbox/widget backlog.
5. Tier 3 items 1, 3, 4 (attach MCP tools to the support agent, batch-test eval, Intercom importer) as the first post-launch releases; they are the Fin-comparison features and the Intercom-switcher hook.

Sources: `docs/research/2026-09-10-intercom-platform-inventory.md`, `docs/research/2026-09-10-fin-ai-agent-inventory.md`, `docs/research/2026-09-10-helpin-support-surface-audit.md`, `docs/research/2026-09-10-agent-runtime-audit.md`; every code claim above cites a path in this repository or in `agent-runtime/`.
