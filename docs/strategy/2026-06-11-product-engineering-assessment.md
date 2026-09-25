# Product and engineering assessment: June 2026

> Historical assessment, source-compared on 2026-09-17. This page preserves
> June product priorities for maintainers. Its verdict, scorecard, estimates,
> counts, and competitor comparisons are not a current readiness or security audit.

## Material changes since June

Several absence claims are contradicted by current source:

- **Billing and usage:** Enterprise [billing handlers](../../server/ee/handler/billing.go)
  and [AI usage handlers](../../server/ee/handler/ai_usage.go) exist. Consult the
  [usage guide](../ai-usage-metering.md) for current accounting boundaries.
  Their presence does not imply Community includes commercial billing or that a
  particular deployment has completed live acceptance.
- **Product surfaces:** [Support](../../frontend/src/pages/pm/Support.tsx) and
  [Support search](../../frontend/src/pages/pm/SupportSearch.tsx) exist, and
  [Deals](../../frontend/src/pages/crm/Deals.tsx) renders board and list views.
  “One page/no inbox/no search” and “no kanban” are not current descriptions.
- **Validation:** the [CI workflow](../../.github/workflows/ci.yml) runs backend,
  frontend, widget, and mobile tests without the cited `continue-on-error: true`
  setting. Workflow configuration does not establish that branch protection
  requires every job or that the latest run passed.
- **Rate limiting:** [router wiring](../../server/internal/router/router.go)
  includes injected authenticated, widget, and help-center answer limiters.
  This disproves “none anywhere”; it is not proof that every endpoint or websocket
  path has equivalent limits in every deployment.
- **Distribution:** [production npm publishing](../../.github/workflows/publish-npm-prod.yml)
  and a [Support mobile app](../../apps/support-mobile/README.md) exist.
  Source and workflow presence do not establish successful publication or mobile
  store availability.
- **Onboarding:** the [Setup page](../../frontend/src/pages/SetupSuccessPage.tsx)
  now provides a goal-driven guide. The historical empty onboarding assessment
  should not be reused as a present feature inventory.

The original claims of uniform tenant isolation, production readiness, universal
layering, minimal TODOs, fixed agent counts, and superiority to named competitors
were not re-established by this documentation review. In particular, “no isolation
holes found” is a historical observation, not a security guarantee. The full
module analytics, integrations, importer parity, and go-to-market scorecard need
separate acceptance evidence; the old week estimates are not current commitments.
See the [documentation audit](../research/2026-09-17-documentation-code-audit.md)
for concrete current findings rather than treating this strategic verdict as a
release checklist.

## Original June assessment

Scope: full-repo audit across product module maturity, engineering quality, and go-to-market readiness.

## Verdict

Helpin is a **sophisticated, AI-first backend wearing an unfinished product**. The differentiator (autonomous agents + signal intelligence) is real and ahead of every named competitor. The blockers are not features — they are **commercialization (billing), visibility (reporting/UI), and trust (test gates, API docs)**.

---

## What we have done well

1. **Agents & automation architecture is the moat.** Event → trigger → automation_rule → agent_run lifecycle, 7 system agents, Ask Agents orchestration, multiple runtimes (native_sdk/opencode/codex), durable `agent_run` primitive. This is genuinely ahead of Zapier/Make/Intercom Fin-class products.
2. **Support backend is enterprise-grade.** 77 services: AI confidence scoring, coverage/gap analysis vs. knowledge base, SLA logic, escalation detection, email routing. Inbound email → ticket via Postmark is production-ready (routing, threading, audit logs, recovery worker).
3. **Security & multi-tenancy discipline.** RBAC (2.3k LOC, well-tested), consistent `workspace_id` scoping verified across repositories, standard JWT with sane TTLs, Sentry wired. No isolation holes found.
4. **Docs/help center is strong.** Multilingual (1.4k LOC), embeddings search, change proposals, SEO/ETags, public help-center app.
5. **Clean codebase.** Handler→service→repo layering held everywhere, 6.5k `if err != nil` checks, only 3 TODOs in the entire Go codebase, no god files, idempotent migrations, Temporal for durability.
6. **CRM AI layer** (signal detection, enrichment, suggestions) exceeds Pipedrive/Attio on autonomous intelligence.

## What we must do better — ranked

### Tier 1: Revenue blockers (do first, ~3-4 weeks total)
1. **Billing is completely absent.** No Stripe, no plans, no seats, no trials, no entitlements. Zero ability to convert a customer. Build: tier pricing + seat enforcement (~2 wks), then feature gating (~3 days).
2. **AI usage metering.** LLM calls have real COGS; nothing tracks tokens per workspace. Unmetered AI + no billing = negative margin at scale. (~1 wk, after billing.)

### Tier 2: The backend/frontend gap (biggest product debt)
3. **Support UI: 77 backend services, 1 page.** The Intercom-killer surface has no inbox UX, no SLA dashboard, no assignment, no search, no canned responses, no CSAT reports. Highest-ROI build in the company — the backend already supports it.
4. **Reporting/analytics: zero across all modules.** No PM velocity/burndown, no CRM pipeline/forecast views, no support metrics. Data is collected but never surfaced. Enterprise buyers will not purchase without dashboards.
5. **CRM pipeline visualization** — deals exist, no kanban/stage UI or forecasting.

### Tier 3: Trust & distribution
6. **Tests don't block PRs** (`continue-on-error: true`). Stabilize and flip to blocking.
7. **No rate limiting** anywhere (API or websocket hub) — abuse/OOM risk.
8. **No public API docs/OpenAPI, no outbound webhooks for customers** — competitors live on integrations.
9. **Slack integration missing** — enterprise table-stakes (~1 wk).
10. **Widget at ~70% Intercom parity** but no npm publish pipeline, no i18n, no identity verification.
11. **No competitor import** (Zendesk/Intercom/Jira/Pipedrive CSV) — switching cost kills sales without it.
12. **Onboarding**: auth/invites work, but no sample data, empty states, tours, or trial funnel.

### Watch items
- Frontend build needs 4-8GB heap — dependency tree bloat (tiptap, excalidraw, etc.).
- No metrics/tracing (Sentry only); Sentry traces at 100% sample rate will cost at scale.
- Marketing module is 0 handlers/0 models — skills-only; decide deliberately whether it's a module or an agent capability.
- Mobile: web-only; acceptable for v1, plan for it.

## Module scorecard

| Module | Backend | Frontend | vs. competitor |
|---|---|---|---|
| Agents/Automation | Strong | Good | **Ahead** |
| Support | Strong | Nascent (1 page) | Backend ahead, product behind |
| Docs/Help center | Strong | Functional | Competitive |
| PM | Functional | Functional | Behind Jira on estimates/velocity/reporting |
| CRM | Functional + strong AI | Thin | Behind on pipeline viz/forecasting |
| Marketing | Absent | Absent | N/A |

## Recommended sequence (next 2 quarters)

1. **Weeks 1–4:** Stripe billing + seats + feature gates + AI metering. Open paid GTM.
2. **Weeks 3–10 (parallel):** Support inbox UI build-out — make the strongest backend visible.
3. **Weeks 8–14:** Reporting layer (support metrics → CRM pipeline → PM velocity), CRM kanban.
4. **Continuous:** flip tests to blocking, add rate limiting, OpenAPI spec, Slack integration, competitor importers.

**Strategic framing:** stop building backend features. The backend can support ~5x more product than is currently exposed. The next two quarters are about *exposing and selling* what exists: billing, UI, dashboards, and trust signals.
