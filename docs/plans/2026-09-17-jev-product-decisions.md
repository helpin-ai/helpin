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
- Semantic topic matching is connected to both canonical assignment entry points. Workspace/open-topic filtering, explicit uncertainty review, Unicode-preserving identity, transactional source/topic checks and shared manual/dismissal fences are covered by tests. Candidate retrieval remains the existing bounded topic list, as documented.
- Answer-evidence assessment is wired through the actual support knowledge command and preview path, with exact returned excerpts, advisory provenance and preserved full citation evidence. Tool-level and fallback tests pass.
- Flows support an optional semantic condition in trigger_config across API validation, create/edit/detail UI, action gating and Activity outcomes. Scheduled/managed-playbook conditions are rejected. Unavailable/uncertain/shadow checks fail closed; legacy Flows remain unchanged. Pipeline quick editing preserves a saved condition. Activity counts do not treat condition checks as executed actions.
- New-meeting projection tests verify draft preservation, routing provenance and no duplicate projection.
- Disposable PostgreSQL tests apply both migrations twice and verify concurrent admission deduplication/caps, condition history without an agent, and manual topic assignment winning against a concurrent semantic result.
- Focused backend suites pass under the race detector. Full service/repository/handler/model suites pass with TZ=UTC. Two existing SQLite expiry tests fail under local Europe/Berlin time; both failures reproduce unchanged on the earlier PM branch. No unrelated expiry code was changed.
- Community and Enterprise API/worker/migrate builds and backend vet pass; go mod tidy leaves dependencies unchanged. Both frontend edition builds pass. 32 Flow/Activity component tests and two mobile light/dark Chromium harness tests pass, including keyboard use, editing/removal, disabled/error states and outcomes. Screenshots were inspected; the local symlinked dependency setup blocked the bundled font in Vite, so browser checks used its fallback font.
- Rollout/fallback/limitations and test commands are documented in docs/deployments/jev-product-decisions.md. All new classifiers default to shadow; no real customer/provider calls were used for verification.
- No customer-data/provider calls, merge, push or deployment performed for this work.

## Completion audit

| Reviewed recommendation | Implemented behavior | Evidence |
| --- | --- | --- |
| 2: Meetings | New and existing draft classification with exact quote and legacy fallback | meeting_follow_up_jev_test.go; existing meeting routing tests |
| 3: Knowledge gaps | Closed categories constrain grounded narrative generation | support_coverage_jev_test.go; Coverage regressions |
| 4: Topic matching | Bounded semantic comparison; reversible guarded assignments and review | support_coverage_topic_jev_test.go; support_coverage_semantic_assignment_test.go; PostgreSQL concurrency test |
| 5: Flows | Optional editor/API condition, guarded execution, durable Activity and provenance | automation_rule_jev_test.go; Flow/Activity components; Chromium harness |
| 6: Answer evidence | Exact excerpt assessments in knowledge-tool output, unchanged citation validation | support_answer_evidence_jev_test.go; search-knowledge regressions |

Recommendation 1, CRM signal classification, remains excluded. Merging, deployment and primary-mode rollout require a separate instruction.
