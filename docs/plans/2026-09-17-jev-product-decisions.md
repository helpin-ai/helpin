# Jev product decisions: implementation contract

Scope: implement the reviewed recommendations 2–6 end to end: meeting follow-up routing, knowledge-gap classification, feedback/knowledge-topic matching, semantic conditions in Flows, and answer-evidence assessment. CRM signal classification (recommendation 1) is excluded. No deployment or merge is part of this implementation request.

## Shared requirements

- Reuse the direct, pinned Jev provider. Authorize/retrieve candidate evidence before classification; never let model output grant permissions, change business priority, or bypass deterministic action guards.
- Closed choices, explicit abstention, independently configurable off/shadow/primary modes, thresholds and per-workspace feature budgets. New features default to shadow; primary behavior must be fully implemented and tested.
- Cache by feature/version/model/evidence/questions/policy; cap failed attempts as well as successes; bound input, latency, and retries. Audit decisions and operator-funded usage without persisting raw evidence.
- Shadow mode leaves existing decisions intact. Missing/unavailable/uncertain classifiers preserve existing product behavior, except explicitly configured semantic Flow conditions fail closed.
- Preserve reviewed edits, evidence provenance, current workflow behavior, metering, and existing deterministic eligibility checks.

## Capability parity and integration

1. Meetings: classify both newly generated and existing follow-ups as internal/customer/uncertain. Preserve draft generation, exact-source evidence validation, linked/customer work protection, revision checks, and pending-draft routing. Bounded source evidence must not silently imply completeness for long transcripts.
2. Knowledge gaps: classify existing supported gap categories while retaining grounded narrative/fix generation, coverage rollout controls, retrieval and durable findings. Do not expose contradictory narratives or invent target IDs.
3. Topics: compare retrieved workspace-scoped topics semantically, distinguish same need from related/distinct needs, retain reversible memberships/manual decisions and abstention. Do not merge/delete canonical topics automatically.
4. Flows: add an optional user-configurable semantic condition to rule creation/editing and execution. Preserve trigger/scope/entitlement/action/loop guards. Expose match/skip/unavailable outcomes through existing Activity surfaces; evaluate once per relevant input/policy and skip when uncertain/unavailable.
5. Answer evidence: assess retrieved, authorized excerpts separately from existing ranking. Surface supported/partial/unsupported/uncertain evidence to answer generation with source IDs; do not claim that snippets prove completeness of an entire document or let this bypass reply/handoff guards.

## Verification and completion gates

- Meaningful unit/service/repository tests for success, abstention, malformed output, permissions/scope, input limits, caching, caps, provider/usage failures, off/shadow behavior, source revisions, and existing behavior.
- Real PostgreSQL migration/admission concurrency checks; no customer/provider calls required.
- Flow configuration UI: type checking and meaningful component/browser coverage including editing, disabled/error states and keyboard/mobile where applicable; follow Helpin design system.
- Community/Enterprise builds, backend vet, module tidy, focused regressions, clean diff and committed implementation.
- Review every item above against source/test evidence before marking complete.

## Progress

- Inspected existing entry points and established an isolated worktree from main `9131f93f0`.
- Shared decision admission/policies/audit implemented with a SQL-owned migration; raw evidence is hashed, not stored. Ready outcomes are cached; failed/abandoned attempts consume independent feature budgets. Shadow/default and primary modes are implemented.
- Meeting routing wired in the Temporal worker for new projections and existing-draft routing, with exact supplied-excerpt selection and legacy fallback for ambiguous, failed or oversized inputs. Existing routing revision guards remain in use.
- Knowledge-gap classification wired in API and worker; accepted classifications constrain the existing narrative generator's schema/prompt. Contradictory generated classifications are rejected before persistence. Generated recommendations and their business priority remain independent of Jev probabilities.
- Focused service/repository/config tests pass under the Go race detector for shared decisions, meeting routing and coverage classification, plus existing meeting/coverage regression tests. API and Temporal worker builds pass.
- Still pending: semantic topic matching, Flow model/API/UI/execution and Activity outcomes, answer-evidence assessment, full new-meeting projection tests, PostgreSQL migration/concurrency verification, final Community/Enterprise validation and completion audit.
- No customer-data/provider calls, merge, push or deployment performed for this work.
