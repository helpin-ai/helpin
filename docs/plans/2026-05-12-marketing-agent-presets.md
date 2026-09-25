# Marketing agent presets composition plan

> Historical composition proposal, source-compared on 2026-09-17. This page
> explains the original six-agent marketing concept for contributors. Its
> readiness labels and build order are not the current preset catalog.

## Current implementation

The [preset catalog](../../server/internal/service/agent_presets.go) implements
one consolidated `marketer` preset named **Mira**, rather than the six preset keys
below. Mira uses `native_sdk`, allows manual triggers, defaults to interactive
invocation, and sets approval mode to `always`. These configuration values do not
prove every external action is available or automatically approved.

Its tools cover research, document authoring, task creation/comments, CRM discovery,
and selected CRM writes. Current names include `request_approval`, `web_search`,
and `list_crm_signals`; the older names below are not copyable tool contracts.
[CRM operational commands](../../server/internal/service/internal_command_crm_operational.go)
now include deal creation and contact/deal updates, so the blanket claim that CRM
writes still need to ship is obsolete. Tool availability remains subject to the
preset, runtime policy, permissions, and configured services.

Marketing skills use current catalog keys such as `marketing_context_setup` and
`marketing_copywriting`, with skill discovery/read tools. The proposed hyphenated
skill composition is not proof that each skill is automatically loaded for every
run. [Preset test source](../../server/internal/service/agent_presets_marketer_test.go)
checks Mira's configuration; it is not an end-to-end campaign execution test.

Browser tools including `browser_snapshot` now occur in
[Runtime profiles](../../server/internal/agentcontract/runtime_profiles.go), so
“once browser_snapshot lands” is historical. That does not establish browser
availability for every Mira run. No `schedule_email_sequence` or `draft_social_post`
implementation was found in current backend source. Gmail/Postmark integrations
alone do not establish a marketing preset's ability to send campaigns.

The [automation event constants](../../server/internal/model/automation_rule.go)
use `task.state_entered` and provider-specific release events such as
`github.release_published`; the old `story.state_entered` and generic release
examples below are not current event identifiers. Mira's preset remains manual;
these event examples are proposed compositions, not installed triggers.

See the [companion skill evaluation](2026-05-12-marketing-skills-catalog-eval.md)
for the historical rationale. No campaigns were sent or runtime tests run during
this documentation review.

## Original six-agent proposal


Companion to `2026-05-12-marketing-skills-catalog-eval.md`. Names each proposed agent, the skills it composes, the Helpin tools it relies on, its trigger surfaces, and readiness today.

All agents run on the existing `native_sdk` runtime — same pattern as `support_agent`, `code_builder`, `crm_operator`. No new orchestration primitives.

Every agent silently includes `product-marketing-context` (read-first) and `general_agent_behavior` (the existing baseline skill). Those are omitted from the per-agent lists below to reduce noise.

---

## 1. `content_marketer`

**Purpose:** Drafts, edits, and packages written marketing content — blog posts, landing-page copy, social posts, newsletters.

**Skills:**
- `copywriting`
- `copy-editing`
- `content-strategy`
- `social-content`
- `marketing-psychology`

**Helpin tools used today:** `search_documents`, `read_document`, `create_document`, `publish_document_change_proposal`, `add_task_comment`, `web_search_*`, `fetch_url`, `request_human_approval`.

**Trigger surfaces:**
- `manual` (user kicks off from a PM task or Docs page)
- `story.state_entered` on marketing-typed PM tasks (e.g. when a content task moves to "In Progress")

**Readiness:** Full. No new tools needed.

**Why it's coherent:** All skills produce text artifacts; outputs land in Docs or PM tasks; reviewer = human via `request_human_approval`. Mirrors existing `prd_authorship` flow.

---

## 2. `seo_specialist`

**Purpose:** Audits and optimizes site/help-center SEO — technical audits, schema, internal linking, AI-search visibility, programmatic-page templates.

**Skills:**
- `seo-audit`
- `ai-seo`
- `schema-markup`
- `site-architecture`
- `programmatic-seo`

**Helpin tools used today:** `fetch_url`, `crawl_url`, `web_search_*`, `read_document`, `search_documents`, `create_document`, `publish_document_change_proposal`.

**Trigger surfaces:**
- `manual`
- `cron` (weekly site audit)
- `article.published` event (auto-polish new help-center articles)

**Readiness:** Works today in audit/authoring mode. Gets materially stronger once **`browser_snapshot`** lands (catches JS-injected schema, accurate render diffs).

**Why it's coherent:** Single output medium (markdown reports + doc PRs to help center). Help center is the obvious first deployment surface since article PublicIDs already exist.

---

## 3. `email_marketer`

**Purpose:** Builds and runs lifecycle/cold-email campaigns. Pulls CRM signals to personalize, drafts copy, queues sequences.

**Skills:**
- `email-sequence`
- `cold-email`
- `copywriting`
- `customer-research`

**Helpin tools used today:** `list_deals`, `list_contacts`, `list_buyer_signals`, Gmail send (via existing `internal/sync`), Postmark, `request_human_approval`, `preview_md`.

**Trigger surfaces:**
- `manual`
- CRM signal events: `buying_intent`, `timeline_signal`, `need_signal` above the review threshold
- `cron` (weekly nurture)

**Readiness:** Partial today — can draft and queue one-off emails. Becomes the highest-leverage agent in the catalog once **`schedule_email_sequence`** ships (multi-step sequences with conditions). Approval-gated for confidence below `auto_execute_threshold`.

**Why it's coherent:** Reuses Gmail OAuth + CRM autonomy thresholds Helpin already has. No new product surface — the agent *is* the marketing-automation layer.

---

## 4. `cro_analyst`

**Purpose:** Audits conversion surfaces (pages, signup, onboarding, forms, popups, paywalls) and proposes experiments.

**Skills:**
- `page-cro`
- `signup-flow-cro`
- `onboarding-cro`
- `form-cro`
- `popup-cro`
- `paywall-upgrade-cro`
- `ab-test-setup`

**Helpin tools used today:** `fetch_url`, `crawl_url`, `web_search_*`, `read_document`, `create_document`, `request_human_approval`, `preview_md`.

**Trigger surfaces:**
- `manual`
- `cron` (biweekly review)

**Readiness:** Works today as a heuristic/authoring agent (outputs audit reports + experiment plans). Becomes much sharper with **`browser_snapshot`** (rendered DOM, form analysis) and any analytics integration the user provides via tool calls.

**Why it's coherent:** All seven skills follow the same pattern — audit a surface, score against a checklist, output prioritized experiment list. Output schema can be shared across them.

---

## 5. `launch_coordinator`

**Purpose:** Owns the multi-channel choreography of a product launch — release notes, sales collateral, social posts, email announcement, distribution.

**Skills:**
- `launch-strategy`
- `sales-enablement`
- `social-content`
- `release_notes_writer` (existing Helpin skill — composed, not duplicated)
- `copywriting`

**Helpin tools used today:** `get_release_context`, `find_tasks_for_git_changes`, `list_epic_tasks`, `create_document`, `add_task_comment`, `publish_prd_draft`, Gmail/Postmark send, `request_human_approval`.

**Trigger surfaces:**
- `manual`
- `release.published` event (or whichever release-cut event Helpin emits)
- `agent_run.approved` (when a launch checklist is approved, fan out subtasks)

**Readiness:** Works today end-to-end as an authoring agent. Email blast step gets a glow-up from `schedule_email_sequence`; social distribution is human copy-paste until `draft_social_post` lands.

**Why it's coherent:** Already overlaps with the existing `release_notes_writer` skill — wraps it with adjacent skills so a single approval covers the full launch fanout. Pairs with the **Release → Marketing fanout** flow template.

---

## 6. `revops_specialist`

**Purpose:** Owns CRM hygiene and the marketing-to-sales handoff — lead lifecycle stages, deal progression rules, sales collateral matched to deal stage.

**Skills:**
- `revops`
- `sales-enablement`
- `customer-research`
- `competitive_intelligence_digest` (existing Helpin skill, enriched with `competitor-profiling` + `competitor-alternatives` content)

**Helpin tools used today:** `list_deals`, `list_contacts`, `list_buyer_signals`, `crm_operator` skill surfaces.

**Trigger surfaces:**
- `manual`
- CRM signal events (any type, low confidence → suggestions)
- `cron` (daily CRM hygiene sweep)

**Readiness:** Partial today — read-only over CRM. Becomes fully autonomous once **CRM write tools** (`update_deal_stage`, `create_deal`, `update_contact`) ship. Until then, outputs suggestions for human approval.

**Why it's coherent:** This is the marketing-side counterpart to `crm_operator` — `crm_operator` runs the deal pipeline, `revops_specialist` runs the rules and reporting around it.

---

## Coverage map: every adopted skill → an agent

| Skill | Primary agent | Also used by |
|---|---|---|
| `product-marketing-context` | (foundation — all agents) | — |
| `copywriting` | `content_marketer` | `email_marketer`, `launch_coordinator` |
| `copy-editing` | `content_marketer` | — |
| `content-strategy` | `content_marketer` | — |
| `social-content` | `content_marketer` | `launch_coordinator` |
| `marketing-psychology` | `content_marketer` | (cross-cutting reference) |
| `marketing-ideas` | (workspace-level brainstorm — no dedicated agent; surfaced via `manual` skill call) | — |
| `seo-audit` | `seo_specialist` | — |
| `ai-seo` | `seo_specialist` | — |
| `schema-markup` | `seo_specialist` | — |
| `site-architecture` | `seo_specialist` | — |
| `programmatic-seo` | `seo_specialist` | — |
| `email-sequence` | `email_marketer` | — |
| `cold-email` | `email_marketer` | — |
| `customer-research` | `email_marketer` | `revops_specialist`, `content_marketer` |
| `page-cro` | `cro_analyst` | — |
| `signup-flow-cro` | `cro_analyst` | — |
| `onboarding-cro` | `cro_analyst` | — |
| `form-cro` | `cro_analyst` | — |
| `popup-cro` | `cro_analyst` | — |
| `paywall-upgrade-cro` | `cro_analyst` | — |
| `ab-test-setup` | `cro_analyst` | — |
| `analytics-tracking` | (manual skill — no dedicated agent; called inside `cro_analyst` runs) | — |
| `launch-strategy` | `launch_coordinator` | — |
| `pricing-strategy` | (manual skill — owner-level; no agent automation) | — |
| `sales-enablement` | `launch_coordinator` | `revops_specialist` |
| `revops` | `revops_specialist` | — |
| `competitor-profiling` / `competitor-alternatives` | merged into existing `competitive_intelligence_digest` | `revops_specialist` |

Three skills (`marketing-ideas`, `analytics-tracking`, `pricing-strategy`) stay as manually-invoked skills with no dedicated agent — they're one-shot reasoning tasks where the agent layer adds nothing.

---

## Build order recommendation

1. **`content_marketer`** — fully ready today, broadest immediate value, lowest risk.
2. **`seo_specialist`** — fully ready today; obvious win for help center.
3. **`launch_coordinator`** — composes an existing skill (`release_notes_writer`), so demonstrates value of the agent-over-skills pattern without new infra.
4. **`cro_analyst`** — ready in authoring mode; ship and let `browser_snapshot` upgrade it later.
5. **`email_marketer`** — ship in single-send mode now; full power after `schedule_email_sequence` lands.
6. **`revops_specialist`** — ship in read-only/suggestion mode now; goes autonomous after CRM write tools.

Each agent is a preset row plus a markdown template — implementation is tiny next to the skills themselves. The expensive part already happened in the previous doc (curating which skills to bring in).
