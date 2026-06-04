# Marketing Skills Catalog — Evaluation

**Source:** https://github.com/coreyhaines31/marketingskills (40 skills, ~28K stars)
**Date:** 2026-05-12
**Status:** Proposal for review

## Context

Helpin already ships 23 system skills in `server/skills/system/` — all engineering / docs / support / CRM-internal flavored. Zero marketing-side skills today, despite "marketing task management" being a first-class pillar in `CLAUDE.md`. The marketingskills repo is the closest off-the-shelf catalog. Skills are plain markdown — adoption is mostly curation, not engineering.

## Recommendation summary

- **Adopt 22 skills as system skills** (strong fit, mostly drop-in)
- **Adapt 6 skills** (overlap with existing — merge or extend)
- **Skip 12 skills** (out-of-scope or low ROI for Helpin's audience)
- **Add 1 foundational doc per workspace**: `product-marketing-context` (the repo's keystone)
- **Build 6 system agent presets** that compose these skills
- **Wire 5 flow templates** (automation rules) into existing CRM/PM/Docs trigger surfaces

---

## Skills — adopt as-is (22)

Drop into `server/skills/system/` with light Helpin-specific framing (workspace, task_key, CRM signal types).

### Conversion / Growth Engineering (7)
| Skill | Why it fits |
|---|---|
| `page-cro` | Marketing pages — generic, broadly useful |
| `signup-flow-cro` | Helpin's own onboarding has a signup flow; users will too |
| `onboarding-cro` | Pairs with existing `support_gap_to_docs` for activation loops |
| `form-cro` | Lead capture / contact forms in CRM module |
| `popup-cro` | In-app modal optimization |
| `paywall-upgrade-cro` | Strong overlap with `project_billing` (owner-only billing) |
| `ab-test-setup` | Currently no experimentation skill anywhere in catalog |

### SEO (5)
| Skill | Why it fits |
|---|---|
| `seo-audit` | Help center is a public surface — direct value |
| `ai-seo` | AI-search era table stakes for help-center articles |
| `schema-markup` | Article PublicID + structured data, pairs with existing docs IA |
| `site-architecture` | Help-center collections/articles already have IA structure |
| `programmatic-seo` | Helpin CRM has structured data → can drive programmatic pages |

### Content / Copy (4)
| Skill | Why it fits |
|---|---|
| `copywriting` | Foundational; touches release notes, docs, emails |
| `copy-editing` | Pairs with every doc-writing skill we already have |
| `content-strategy` | Editorial planning for marketing module |
| `social-content` | Direct marketing module use case |

### Email / Outreach (2)
| Skill | Why it fits |
|---|---|
| `email-sequence` | Gmail integration already exists; CRM has lifecycle signals |
| `cold-email` | CRM signal `buying_intent` → cold/warm follow-ups |

### Strategy / GTM (4)
| Skill | Why it fits |
|---|---|
| `customer-research` | Feeds product-marketing-context + PRD authorship |
| `launch-strategy` | Pairs with `release_notes_writer` for full launch flow |
| `pricing-strategy` | Owner-level decisions, ties to billing module |
| `sales-enablement` | Direct CRM module overlap (deal stage → collateral) |

---

## Skills — adapt / merge (6)

Helpin already has overlapping coverage. Treat the upstream skill as input, not a clean drop.

| Upstream skill | Existing Helpin skill | Action |
|---|---|---|
| `competitor-profiling` | `competitive_intelligence_digest` | Merge — pull frameworks/checklists from upstream into the existing skill |
| `competitor-alternatives` | `competitive_intelligence_digest` | Merge — add "X alternatives" page playbook section |
| `revops` | `crm_operator` + `epic_state_routing` | Adapt — keep upstream taxonomy, route to existing CRM autonomy thresholds |
| `marketing-ideas` | `prd_authorship` (partial) | Adapt as separate "ideation" skill, distinct from PRD authoring |
| `marketing-psychology` | none directly | Adapt — strip generic content, anchor to Helpin's CRM signal types |
| `analytics-tracking` | none | Adapt — wire to Helpin event model rather than generic GA/Mixpanel |

---

## Skills — skip (12)

- `aso-audit` — no mobile app focus in Helpin
- `image`, `video` — content generation; lower ROI, large surface area
- `free-tool-strategy` — niche, founder-stage advice
- `directory-submissions` — one-shot task, doesn't need a skill
- `co-marketing`, `community-marketing` — squishy, low automation value
- `referral-program` — defer until Helpin has a referral primitive
- `lead-magnets` — covered loosely by `content-strategy` + `copywriting`
- `paid-ads`, `ad-creative` — large surface, low fit for Helpin's ICP today
- `churn-prevention` — already implicitly covered by CRM `churn_risk` signal handling

(Revisit `churn-prevention` and `referral-program` once marketing module is more mature.)

---

## Foundational addition: `product-marketing-context`

The upstream catalog's keystone: a per-project markdown file every other skill reads first. Helpin equivalent: a per-**workspace** "Marketing Context" document — product, ICP, positioning, voice, key URLs. Two implementation paths:

- **Lightweight:** new workspace setting `marketing_context` (long-form text), surfaced as a system skill that loads it
- **Structured:** new model `WorkspaceMarketingContext` with versioning (mirrors how `prd_authorship` works)

Recommend lightweight first; promote to structured once 2+ skills depend on field-level access.

---

## System agents to build (6)

Each is a thin preset over existing `native_sdk` agent runtime — same pattern as `support_agent` / `code_builder` / `crm_operator`. Composition is the value.

| Agent preset | Composed skills | Trigger surfaces |
|---|---|---|
| `content_marketer` | copywriting + copy-editing + content-strategy + social-content + product-marketing-context | manual, `story.state_entered` (PM marketing tasks) |
| `growth_engineer` | page-cro + signup-flow-cro + ab-test-setup + analytics-tracking | manual, `cron` (weekly) |
| `seo_specialist` | seo-audit + ai-seo + schema-markup + site-architecture + programmatic-seo | manual, `cron` (weekly), `article.published` |
| `email_marketer` | email-sequence + cold-email + product-marketing-context | manual, CRM `buying_intent` / `timeline_signal` events |
| `launch_coordinator` | launch-strategy + sales-enablement + social-content + release_notes_writer (existing) | manual, `release.published` |
| `cro_analyst` | page-cro + form-cro + popup-cro + onboarding-cro + paywall-upgrade-cro | manual, `cron` (biweekly) |

All run through the existing generic agent runtime — no special launch paths needed per the canonical `AGENTS_AND_AUTOMATION.md` direction.

---

## Flow templates (automation rules) to ship (5)

Wire to existing trigger surfaces (`story.state_entered`, `agent_run.approved`, `cron`, plus CRM signal events).

| Template | Trigger | Action chain |
|---|---|---|
| **Release → Marketing fanout** | `release.published` | release_notes_writer → social-content draft → launch-strategy checklist → email-sequence (announcement) |
| **Help article → SEO polish** | `article.published` | schema-markup → ai-seo audit → social-content teaser |
| **New buying-intent signal** | CRM signal `buying_intent` confidence ≥ review threshold | sales-enablement collateral pick → cold-email draft (approval-gated) |
| **Trial signup activation watch** | `user.trial_started` + 24h delay | onboarding-cro audit of user's path → email-sequence (activation) |
| **Weekly growth pulse** | `cron` weekly | seo-audit + page-cro audit + ab-test-setup readouts → digest task in PM |

All match the trigger surfaces already in the codebase (`agent.trigger_mode`, `agent.schedule`, automation rules with `event` / `cron`). No new orchestration primitives needed.

---

## Implementation order (suggested)

1. Drop in `product-marketing-context` skill + workspace setting
2. Add the 22 adopt-as-is skills (mostly markdown — fast)
3. Adapt the 6 overlap skills (more thought, slower)
4. Ship `content_marketer` and `seo_specialist` agents first — broadest immediate value
5. Wire `Release → Marketing fanout` template — clearest end-to-end demo
6. Iterate on the rest based on usage

## Open questions for you

- Marketing module UI surfacing — is there a marketing module workspace surface yet, or should these skills launch from PM/CRM/Docs for now?
- License: upstream is presumably permissive (skills are markdown). Confirm before bulk import.
- Do we want a "marketing context" doc per workspace, or per organization (since brand voice is usually org-level)?

---

# External Tool Dependencies — Gap Analysis

Audited the actual SKILL.md bodies for the 28 recommended skills against Helpin's agent tool catalog (`server/internal/worker/tool_catalog.go`).

## What Helpin already has

`read_file*`, `write_file`, `ripgrep`/`grep`, `run_command`, `web_search_brave`, `web_search_exa`, `fetch_url`, `crawl_url`, full Git/PR set, full PM/CRM/Docs/Support tool sets, `request_human_approval`, `preview_*`, plus **Gmail + Postmark** for outbound email. LLM access via the `Provider` interface (Claude + OpenAI).

## Skills that work today with NO new tools (15)

Pure-knowledge skills — they reason over user input and produce markdown/code. No external API needed beyond what we already have.

`copywriting`, `copy-editing`, `content-strategy`, `customer-research`, `marketing-ideas`, `marketing-psychology`, `pricing-strategy`, `sales-enablement`, `launch-strategy`, `product-marketing-context`, `programmatic-seo` (templates only), `page-cro`, `signup-flow-cro`, `onboarding-cro`, `paywall-upgrade-cro`.

These are safe to import as-is. They'll use `fetch_url` and `web_search_*` where the upstream skill says "audit the live page."

## Skills that work but degrade without external tools (7)

Will run, but quality drops because they expect data from a platform we don't talk to.

| Skill | Missing tool / data | Degraded behavior | Workaround |
|---|---|---|---|
| `seo-audit` | Google Search Console, Ahrefs, Screaming Frog | Can't see actual rankings, crawl errors, backlinks | Use `crawl_url` + `fetch_url`; require user to paste GSC CSV exports |
| `ai-seo` | Same — also wants JS-rendered DOM | Misses JS-injected schema and content | Add a browser tool (see below) |
| `schema-markup` | schema.org validator API | Can author markup, can't validate live | `fetch_url` validator.schema.org/?url=... works as a fallback |
| `site-architecture` | Sitemap crawler | Limited beyond what `crawl_url` returns | Often fine; user can paste sitemap XML |
| `analytics-tracking` | GA4 / GTM / Mixpanel / PostHog APIs | Can only produce *plans* and tracking-plan docs — can't read live event data | Acceptable; plan-authoring is the main use |
| `ab-test-setup` | Optimizely / Statsig / GrowthBook | Can author test specs + power calcs; can't read variant data | Acceptable; design is the value |
| `form-cro`, `popup-cro` | Live event/funnel data | Falls back to heuristic audits via `fetch_url` | Acceptable |

**Net:** all of these still deliver value as planning/authoring skills. They get materially better with real data, but they don't *fail* without it.

## Skills that need a new capability to be useful (6)

These are the real gaps. Each row is a recommendation, not a must-do.

| Skill | Missing capability | Recommendation |
|---|---|---|
| `social-content` | Post/schedule to LinkedIn/Twitter/Instagram | **Defer post-step.** Skill drafts content + cadence; user copy-pastes. Add a `draft_social_post` PM-task tool so output lands in Helpin PM with platform metadata. Native posting integrations come later. |
| `email-sequence` | Multi-message scheduler with conditions | **Reuse Helpin email infra.** Helpin already has Gmail send (`internal/sync`) and Postmark. Add a thin `schedule_email_sequence` tool that creates a sequence record + Temporal schedule. This is the highest-leverage build because Gmail + CRM signals are already wired. |
| `cold-email` | Mail-merge / personalization at scale | Same as above — reuse `schedule_email_sequence` once it exists; CRM contacts are already in DB. |
| `ai-seo`, `schema-markup` (deep mode) | JS-rendered page snapshot | **Add a `browser_snapshot` tool** backed by Playwright (we already use it for frontend tests). Returns rendered DOM + JSON-LD scripts. Pays off for SEO and any CRO audit. Stub it behind a config flag if Playwright infra isn't worker-ready. |
| `competitor-profiling` / `competitor-alternatives` | Already partial in `competitive_intelligence_digest` | No new tool — fold upstream content into existing skill. |
| `revops` | CRM write actions (deal stage updates, etc.) | Helpin CRM tools today are read-only (`list_deals`, `list_contacts`, `list_buyer_signals`). **Add write counterparts** (`update_deal_stage`, `create_deal`, `update_contact`) — these are useful beyond marketing skills. |

## Skills with hard external dependency we should NOT build first (skipped from earlier list, restated)

`paid-ads`, `ad-creative` — need Meta/Google/LinkedIn Ads APIs (OAuth + per-platform clients). Real engineering. Skip for now. `referral-program`, `aso-audit`, `churn-prevention` — also need primitives Helpin doesn't have yet.

## Suggested tool builds, prioritized

1. **`schedule_email_sequence`** (highest leverage — unlocks 2 skills + amplifies CRM module). Backed by Temporal + Gmail/Postmark. ~1 week.
2. **CRM write tools** (`update_deal_stage`, `create_deal`, `update_contact`). Unlocks `revops` and broader CRM automation. Pairs with existing autonomy thresholds. ~3 days.
3. **`browser_snapshot`** (Playwright in worker). Unlocks SEO/CRO depth. Has reuse beyond marketing (support agents auditing forms, etc.). ~3–5 days.
4. **`draft_social_post`** (PM-task tool with social metadata). Cheap. ~1 day. Native scheduling integrations come later, behind feature flags.
5. **Analytics/experimentation integrations** — defer. Big surface, niche per customer, better as user-installed connectors than core tools.

## Bottom line

- **22 of the 28 recommended skills work today** — 15 fully, 7 in degraded-but-useful mode.
- **6 skills need new tools.** None require a brand-new product surface; all map to additions that strengthen Helpin's existing CRM/PM/email/agents.
- Biggest leverage moves are **`schedule_email_sequence`** (turns Gmail+CRM into a real marketing-automation engine) and **CRM write tools** (long overdue regardless of marketing skills).

