# Support Triage at Intercom-Fin Level — Gap Analysis & Plan

**Date:** 2026-06-12
**Current code:** `server/internal/service/support_inbox_triage.go`

## Where we are today

| Capability | Current state |
|---|---|
| Rule conditions | `phrase_contains` (substring, case-insensitive) + `email_domain_equals`, AND between the two types |
| Classification | Single Haiku call choosing a mailbox handle, with lexical fallback |
| When it runs | Once per conversation, on first customer reply only |
| Actions | Move to mailbox (suggest or auto-move) — nothing else |
| Spam | Skips already-spam conversations; cannot *detect* spam |
| Feedback | Accepted/dismissed/corrected events recorded but never used |
| Rule lifecycle | No preview, no backtest, no retroactive apply |
| Observability | slog lines only — no dashboard, no per-rule hit counts |

## What "Fin level" means (target behaviors)

1. **Intent-first, not phrase-first.** Conversations are classified into a workspace-defined topic/intent taxonomy (e.g. `spam/link_building_outreach`, `billing`, `bug_report`) semantically — "link-building", "guest post opportunity", "SEO collab" all land in one intent without enumerating phrasings. Rules then reference intents, with phrases/regex as a precision layer.
2. **Continuous triage.** Every inbound customer message can re-classify the conversation (meaning-change detection via existing `input_hash`), not just the first one.
3. **Rich rule engine.** Condition groups `{logic, rules[]}` using the shared query-builder conventions (see CLAUDE.md): fields = subject, body, sender email/domain, detected intent, detected language, sentiment, channel, business hours, CRM lifecycle stage; operators = `contains`, `not_contains`, `matches_regex`, `semantic_match`, `is`, `is_empty`, …
4. **Multi-action rules.** Beyond "move to mailbox": assign team/agent, set priority, add tags, **mark as spam**, close, send saved reply, hand to AI agent, escalate to human.
5. **Built-in spam/cold-outreach detection.** A dedicated cheap classifier stage (link building, SEO outreach, partnership pitches, recruiting) that runs before routing — this is exactly the user-reported pain.
6. **Learning loop.** Use the existing triage feedback events: inject recent corrections as few-shot examples into the prompt per workspace; surface "create a rule from this pattern?" suggestions after N similar manual moves.
7. **Rule preview + backtest + retroactive apply.** "Test this rule" runs conditions against the last N conversations and shows would-match list before saving; option to apply to open conversations on save.
8. **Triage analytics.** Per-rule hit counts, auto-move rate, override rate, AI confidence distribution, deflection — the trust mechanism that lets admins turn on auto-move.

## Phased plan

### Phase 1 — Fix trust gaps (small, immediate)
- Re-evaluate open conversations when a rule is created/updated (bounded backfill: open + unassigned + last 30 days).
- Normalize text before matching: collapse whitespace, strip punctuation/hyphens so `link building` matches `link-building`/`Link  Building`. (`triageConditionsMatch`)
- Per-rule hit counter + `last_matched_at` on `support_triage_rules`; show in rules UI.
- Surface skip reasons in UI (triage disabled, channel off, already evaluated) instead of silent no-ops.
- Default `TriageEnabled=true` for new installs; settings UI nudge when rules exist but triage is off.

### Phase 2 — Intent taxonomy + spam stage
- New `support_intents` table (workspace-scoped, seeded defaults incl. `cold_outreach_spam`).
- Two-stage pipeline: (a) cheap classifier → intent + language + spam score; (b) routing decision. Persist intent on conversation.
- Rule condition type `intent_is`; action types `mark_spam`, `add_tags`, `set_priority`, `assign_team`, `close`.
- Auto-spam: spam score ≥ threshold → mark spam + skip from inbox counts (setting-gated, default suggest-only).

### Phase 3 — Rule engine on shared query builder
- Replace bespoke `SupportTriageRuleConditions` with the structured filter group payload `{logic, rules[]}` used elsewhere (canonical operators), keeping a migration shim for existing phrase/domain rules.
- Frontend: rule builder component reusing the query-builder UI; preview/backtest endpoint (`POST /support/triage-rules/preview`).

### Phase 4 — Learning + analytics
- Few-shot prompt enrichment from `support_conversation_triage_events` corrections (last 20 per workspace, cached).
- "Suggested rule" generator: cluster manual moves by intent/domain, propose rule drafts.
- Triage dashboard (hit rates, overrides, confidence calibration) under Support settings.

## Notes
- Keep deterministic rules as tier 0 (free, fast, auditable); LLM classification as tier 1 — same as today's ordering, just richer.
- Budget control already exists (`TriageDailyBudget`); Phase 2's classifier stays on Haiku.
- All evaluation already flows through `EvaluateAndRoute`; the pipeline refactor stays inside `SupportInboxTriageService`.
