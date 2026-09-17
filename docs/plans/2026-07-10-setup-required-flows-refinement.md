# Setup required flows refinement plan

> Historical implementation plan, source-compared on 2026-09-17. The required-flow
> catalog, evidence checks, destinations, and goal editor exist in this checkout.
> The unchecked tasks and old agent workflow below are the original work record,
> not current instructions. Use [go.mod](../../server/go.mod) for the current Go
> requirement rather than the historical Go 1.24 label.

## Current behavior

The [setup catalog](../../server/internal/service/setup_catalog.go) includes
required-flow tasks for Product, Help Center, Internal Docs, and CRM, with
prerequisites and module/action permission checks. It appends Automation as a
featured journey when not selected. The original frozen-Support requirement is
scope for that change, not a promise that the Support catalog can never evolve.

[Evidence queries](../../server/internal/repository/setup.go) count enabled
rules with these template keys: Product accepts `release_notes_writer`,
`stale_task_escalation`, or `advance_on_approval`; the other journeys require
`public_help_freshness_sweep`, `docs_freshness_sweep`, or `buying_signal_to_task`.
Enabled installation is not proof of successful execution. Separate success
checks exclude system/template-derived agents from custom-agent achievements.
The actionable-deal predicate requires an owner member, positive amount, close
date, and a contact association; simply having a seeded pipeline does not satisfy it.

[Action destinations](../../frontend/src/lib/setupActions.ts) include template
query parameters and the Knowledge `company-context` anchor. The
[onboarding mapping](../../frontend/src/lib/workspaceOnboardingUseCases.ts) exposes
one “Plan and ship team projects” option mapped to `product_delivery`.
The [Setup page](../../frontend/src/pages/SetupSuccessPage.tsx) limits the goal
editor to one through three goals and shows it to users with `workspace.update`.
Its [query hooks](../../frontend/src/hooks/queries/useSetup.ts) refetch on mount
and window focus. These are setup guidance and evidence projections, not a
substitute for authorization at the destination API.

Original build/test steps below were not rerun during this documentation review.

## Original implementation record

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Setup focused and actionable while requiring an enabled, journey-appropriate automation flow in every applicable non-Support journey.

**Architecture:** Extend the product-owned setup catalog and evidence projection with one enabled-template requirement per applicable journey. Keep Support immutable, centralize action metadata, and use the existing query/service layers for goal updates and progress refresh.

**Tech Stack:** Go 1.24, GORM, React 19, TypeScript, TanStack Query/Router, Vitest.

---

### Task 1: Catalog scope, ordering, and required flows

**Files:**
- Modify: `server/internal/service/setup_catalog.go`
- Modify: `server/internal/model/setup.go`
- Test: `server/internal/service/setup_test.go`

- [ ] Add failing tests for selected journeys plus featured Automation only.
- [ ] Add failing tests for the four required flow tasks, their core membership, prerequisites, and exact template alternatives.
- [ ] Add a frozen Support regression assertion for task keys, order, copy, prerequisites, core membership, and actions.
- [ ] Run focused service tests and confirm the new assertions fail.
- [ ] Implement the minimal catalog, prerequisite, ordering, and progress changes.
- [ ] Run focused service tests and confirm they pass.

### Task 2: Evidence for enabled template installations

**Files:**
- Modify: `server/internal/model/setup.go`
- Modify: `server/internal/repository/setup.go`
- Test: `server/internal/repository/setup_test.go`

- [ ] Add failing repository tests for enabled eligible template rules, disabled rules, and unrelated templates.
- [ ] Add a failing test proving template-created agents do not qualify as custom agents.
- [ ] Run the focused repository tests and confirm failure.
- [ ] Add focused evidence counts/predicates for required flows and custom agents.
- [ ] Run the focused repository tests and confirm success.

### Task 3: Access matrix and action destinations

**Files:**
- Modify: `server/internal/service/setup_catalog.go`
- Modify: `frontend/src/lib/setupActions.ts`
- Modify: `frontend/src/components/settings/KnowledgeTab.tsx`
- Test: `server/internal/service/setup_test.go`
- Test: `frontend/src/lib/setupActions.test.ts`

- [ ] Add failing access tests for Help Center, CRM admin, Automation module, Docs/CRM agent editors, and no scheduling requirement for event flows.
- [ ] Add failing frontend route tests for Knowledge context and template-aware flow routes.
- [ ] Run the tests and confirm failure.
- [ ] Implement the exact action/access matrix and stable company-context anchor.
- [ ] Run the focused tests and confirm success.

### Task 4: Onboarding canonical goal choice

**Files:**
- Modify: `frontend/src/lib/workspaceOnboardingUseCases.ts`
- Modify: `frontend/src/pages/Workspaces.tsx`
- Test: `frontend/src/lib/__tests__/workspaceOnboardingUseCases.test.ts`

- [ ] Add a failing test asserting one Plan and ship team projects option and canonical goal mapping.
- [ ] Run the test and confirm failure.
- [ ] Remove the duplicate option/type path and update copy/mapping.
- [ ] Run the test and confirm success.

### Task 5: Focused Setup UI and goal editor

**Files:**
- Modify: `frontend/src/pages/SetupSuccessPage.tsx`
- Modify: `frontend/src/lib/setupTypes.ts`
- Modify: `frontend/src/hooks/queries/useSetup.ts`
- Test: `frontend/src/pages/__tests__/SetupSuccessPage.test.tsx`

- [ ] Add failing tests for core-progress wording, next-value wording, and editing up to three goals.
- [ ] Run the tests and confirm failure.
- [ ] Implement a compact goal editor using `useUpdateSetupGoals`; preserve the cardless journey layout.
- [ ] Enable prompt refetch on focus/mount where appropriate.
- [ ] Run focused frontend tests and confirm success.

### Task 6: CRM setup evidence correction

**Files:**
- Modify: `server/internal/service/setup_catalog.go`
- Modify: `server/internal/repository/setup.go`
- Test: `server/internal/service/setup_test.go`
- Test: `server/internal/repository/setup_test.go`

- [ ] Add failing tests removing the seeded pipeline task from CRM and requiring contact-associated actionable deals.
- [ ] Run focused tests and confirm failure.
- [ ] Remove the visible pipeline task/prerequisite and tighten the deal predicate.
- [ ] Run focused tests and confirm success.

### Task 7: Verification and review

**Files:**
- Review all modified files.

- [ ] Run focused Setup Go tests.
- [ ] Run focused frontend Setup tests.
- [ ] Run `go test ./internal/service ./internal/repository` and record any unrelated baseline failures separately.
- [ ] Run `go build ./...`.
- [ ] Run `npm run build` in `frontend`.
- [ ] Run `git diff --check` and inspect the complete diff.
- [ ] Verify the Support catalog contract is byte-for-byte unchanged in the diff.
