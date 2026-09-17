# Billing Integration Audit — Loose Ends & Bugs

Date: 2026-06-22 · Branch: `feature/stripe-billing` (worktree)
Scope: Stripe integration, usage metering, entitlements, paywalls, frontend billing, spec compliance.
Method: 4 parallel code audits. Findings marked **(confirmed ×N)** were independently flagged by N agents.

---

## Founder Plan Decision

Add a hidden, non-Stripe **Founder** plan for all current organizations.

Product behavior:

- Founder is assigned at the workspace billing row level as `plan = founder`.
- All workspaces that belong to organizations existing at the migration cutoff become Founder workspaces.
- New workspaces created later inside those same Founder organizations also default to Founder.
- New organizations created after the cutoff do not get Founder by default.
- Founder has all features enabled: custom agents, automation flows, scheduled agents, all modules, and other Growth-gated capabilities.
- Founder has no entity limits: unlimited documents, unlimited CRM contacts, unlimited teams, and no workspace/member caps enforced by plan.
- Founder includes **100,000 AI credits per month**, resetting monthly with no rollover.
- Founder is not Stripe-managed. Stripe checkout, plan changes, customer portal, payment method management, on-demand billing, and payment recovery links should be disabled or hidden for Founder workspaces.
- Billing Settings should show the plan name as **Founder** and explain that it is managed by Helpin.

Implementation shape:

- Implemented with a first-class `BillingPlanFounder = "founder"` constant, rather than an `is_founder` side flag.
- Implemented with `organization_billing.founder_plan_enabled`, so future workspaces created under Founder organizations inherit Founder.
- Migration `202606230001_founder_plan.sql` marks all organizations existing at the cutoff as Founder-enabled and upserts/updates their workspace billing rows to `founder` with `included_credits = 100000`.
- Founder workspaces are `active`, monthly, non-trial workspaces with synthetic monthly periods because there is no Stripe subscription period to copy.
- Entitlement checks treat Founder as above Growth for features and unlimited for entity limits.
- Backend billing APIs reject Founder checkout, plan changes, portal sessions, and on-demand billing.
- Frontend billing helpers and billing settings display Founder, but do not render upgrade/downgrade/portal/payment actions.

---

## Critical / High — fix before shipping

### 1. Trial workspaces never reset usage — the new default plan bricks itself
After `202606220001_remove_free_billing_plan`, every org with no subscription defaults to a **14-day Growth trial** with a 25k hard credit wall. But `CreditsUsed` is reset to 0 **only** inside `ApplyStripeSubscriptionUpdate` when a Stripe period advances (`service/billing.go:776-786`). Trialing/non-Stripe workspaces have no subscription, so their usage **never rolls over** — they accumulate until 25k, then are permanently blocked ("AI usage exhausted"). And `billingCanUseOnDemand` requires `Status==active` + Stripe IDs, so a trial can't buy its way out. Combined: trial = the default, trials never reset, no on-demand escape → silent AI lockout for new orgs.

### 2. Billing is admin-accessible, violating the owner-only convention **(confirmed ×3)**
Project rule (MEMORY `project_billing.md`): billing is **owner-only, admins excluded**. But:
- Backend: checkout/change-plan/portal/on-demand routes gate on `PermSettingsManage` (`router.go:466-472`), which `rbac.go:21-24` grants to **admin**.
- Frontend: `BillingSettingsPage.tsx:189` gates on `canManageSettings` (= `settings.manage`).
Admins can start/alter paid subscriptions. Needs an owner-only check at both layers.

### 3. Deferred plan changes are disabled — all changes apply immediately **(confirmed ×2)**
`billingPlanChangeIsDeferred()` is hardcoded `return false` (`service/billing.go:997`); the frontend mirror `planChangeIsDeferred` also returns `false` (`BillingSettingsPage.tsx:1269`). So **every** plan change (incl. downgrades, monthly→annual) takes the immediate proration path. The entire `ScheduleSubscriptionPriceChange` / pending-plan-apply machinery + `billingPlanRank` is **dead code**, and the preview dialog hardcodes "applies immediately" (`:1123`) even if the backend returns a deferred date. Downgrades hit the customer with immediate proration instead of switching at renewal.

### 4. On-demand overage charges are silently lost on failure **(confirmed ×2)**
`billOnDemandBlocksIfNeeded` runs **outside** the `ConsumeCredits` transaction (`service/billing.go:1058-1097`). Credits are committed first; if the Stripe `BillCreditBlock` call then fails, `ConsumeCredits` returns an error but `CreditsUsed` is already persisted. The retry re-enters with `AlreadyUsed=true` and **skips** on-demand billing entirely → usage counted, customer never invoiced. No retry/log.

### 5. Non-atomic webhook idempotency drops failed events
`InsertStripeWebhookEvent` marks an event "seen" **before** the billing write succeeds (`service/billing.go:730-742`). If the subsequent apply fails, Stripe retries → but the row exists → `isNew=false` → retry silently dropped (`return nil, nil`). The `processed` flag is recorded but never checked, so half-applied events never reprocess. Insert + apply should be one transaction (or reprocess when `processed=false`).

### 6. Concurrent usage bypasses the cap (TOCTOU)
The `nextUsed > IncludedCredits` check reads `summary` **outside** the DB transaction (`service/billing.go:1023-1036`). The increment locks the row `FOR UPDATE` but does **not** re-validate the limit inside the lock. Two concurrent requests both pass and both increment, overshooting `IncludedCredits`.

### 7. Usage limit enforced post-hoc, after the LLM call already ran
`ConsumeCredits` runs **after** `ChatCompletion` returns / after the agent run completes (`ai_usage_meter.go:173-194`, `:309`). The check blocks the *next* call, never the current one — an exhausted workspace still gets one (or, for long agent runs, an unbounded) over-limit execution. No pre-flight balance check before expensive runs.

### 8. Customer-ID event resolution can hit the wrong workspace
One org Stripe customer backs **multiple** workspace subscriptions, but `GetByStripeCustomerID` returns the most-recently-updated row via `Order("updated_at DESC")` (`repository/billing.go:49`). Invoice/trial events resolved by customer ID (when subscription ID is absent) can apply to the wrong workspace.

---

## Medium

9. **Org billing routes have no permission middleware** (`router.go:427-432`) — rely entirely on `canManageOrg`, which returns true for a billing-owner of *any one* workspace in the org. A delegated single-workspace billing owner can read the whole-org roll-up and add/update/**delete** org-shared cards other workspaces depend on.
10. **`CanReserveWorkspaceSeat` never checks the seat count** (`service/billing.go:340`) — only rejects locked workspaces. `SeatOverLimit` is display-only; members can be invited past the plan seat limit.
11. **`past_due` is not "locked"** — `billingStatusLocked` returns true only for `trial_expired`/`canceled`, so failed-payment workspaces keep consuming credits + on-demand with no cap/expiry on the grace period.
12. **Checkout confirmation must tolerate incomplete Stripe period metadata** — resolved for the client return path by falling back `CurrentPeriodStart` before applying checkout subscription state. Webhook ordering should still be monitored against live Stripe events.
13. **Stripe write then separate DB write, non-transactional** (`service/billing.go:550-632`) — if the DB write fails after Stripe succeeds, state diverges until a webhook reconciles (and that webhook may be dropped — see #5).
14. **Entitlement services fail OPEN when unwired** (`entitlements.go:57-59,86-88`) — `if s == nil || s.billing == nil { return nil }`. Worker paths and any service constructed without `SetEntitlementService` enforce nothing.
15. **Org `PlanChangeModal` is intentionally Checkout-first for pre-paid upgrades** — the dead "Billed to" selector was removed. Remaining work is live Stripe verification and any future paid-subscription plan-change preview parity.
16. **`remove_free_billing_plan` is a hard cutover** — flips all free rows to `growth/trial_expired` (locked) on deploy, no grace window, not reversible.
17. **Org-side mutations don't invalidate per-workspace billing cache** (`useBilling.ts:81-136`) — `useSetOnDemand`/`useLinkPaymentMethod`/`useSetBillingOwner`/`useDeleteCard` only touch `billing.org`+`billing.cards`, leaving `billing.workspace(wsId)` stale.

---

## Spec compliance & loose ends

18. **Free plan still live in code despite removal** — `BillingPlanFree = "free"` (`model/billing.go:6`) and ~9 branches still gate on it (`service/billing.go:385,465,554,712,769,1189,1210`, `repository/workspace.go:241`). Migration rewrote rows but left "free" a reachable legacy state. Spec §3/§4 still describe Free as a v1 plan — never reconciled.
19. **Pricing numbers diverge from the cited source** — spec §4 says Starter $99 / Growth $299; `docs/strategy/pricing-strategy.md` says Starter $299 / Growth $799 (+ a Business tier). Confirm actual Stripe prices.
20. **Trial-nudge emails not implemented** — spec §10 requires owner/billing-owner nudges at T-3, T-1, drop. No code (sidebar banner exists; emails don't).
21. **Standalone in-app card capture is no longer planned for v1** — app-managed trials use **Upgrade** CTAs that route to Stripe Checkout, which creates the subscription and collects/saves the payment method. Stripe Portal remains for existing paid billing management.
22. **Spec is missing from the branch** — the design spec doesn't exist in the worktree; it lives only on `main` with **uncommitted** edits (the "§17 Implementation Notes" reconciliation). Pending-plan-changes, cancellation state, and payment notices (3 migrations + 4 routes) were added ad hoc with no spec coverage.

---

## Lower-priority polish
Brittle duplicate detection via error-string match (`repository/billing.go:70` — use `errors.Is(gorm.ErrDuplicatedKey)`); unclamped usage `pct` and negative `credits_remaining` in text (`UsageDetail.tsx:168`, `BillingSettingsPage.tsx:587`); inconsistent metering floor between LLM-wrapper and agent-run paths (`ai_usage_meter.go:267` vs `worker/usage_metering.go:17`); per-message AI idempotency keyed on attempt counter → retries double-charge (`command_bar.go:565`, `support_ai.go:1794`); inconsistent unknown-plan handling (entitlements fail-open vs credits fail-closed).

---

# Over-Engineering Review

Separate lens from the bug audit above: unnecessary complexity, dead abstractions, premature generalization. Method: 2 parallel audits (backend + frontend). The codebase is mostly proportionate — the on-demand block math, billing-notice columns, and metering interfaces all earn their keep. The over-engineering is concentrated in **one dead feature** plus some duplication.

## Genuinely dead — safe to delete

### O1. The deferred / scheduled plan-change feature is fully built but completely unreachable
Gated behind `billingPlanChangeIsDeferred()` (`service/billing.go:997`) and its frontend mirror `planChangeIsDeferred()` (`BillingSettingsPage.tsx:1269`), **both hardcoded `return false`**. That single flag makes an entire vertical slice dead, spanning DB → Stripe → service → UI:
- **Backend:** the pending-plan branch in `ChangeWorkspacePlan` (`billing.go:600-608`); the `ScheduleSubscriptionPriceChange` gateway interface method (`billing.go:34`) **+ its ~50-line Stripe `subscriptionschedule` implementation** (`internal/billingstripe/gateway.go:300`); `billingPlanRank()` (`billing.go:1001`); the webhook reconcile that clears `pending_plan` (`billing.go:802-805`).
- **Schema:** migration `202606200001_billing_pending_plan_changes` (`pending_plan`, `pending_billing_interval`, `pending_change_at`) + matching model/DTO/upsert fields — written only by the dead branch, never meaningfully read. (`cancel_at_period_end` from the same migration **is** used by the resume flow — keep it.)
- **Frontend:** the `"Downgrade/Switch at renewal"` branches in `planActionState` (`BillingSettingsPage.tsx:1253-1259`); the `PlanChangePreviewDialog` deferred-date copy.

This is one coherent unfired feature. Highest-value cleanup: **either** delete it (plan changes become honestly always-immediate) **or** finish wiring the flag — but don't leave both. (Cross-ref bug audit #3.)

### O2. `ReasoningTokens` plumbed end-to-end, never set
`AIUsageMeterInput.ReasoningTokens` flows into `CalculateAIUsageUnits` with a `×6` weight (`ai_usage_meter.go`), but **no caller ever supplies it** — agent-run and LLM-wrapper paths pass only input/output/cached. Dead parameterization; drop the field and the `reasoning*6` term until a caller exists.

### O3. `BillingPlanFree` lingers after the free plan was removed
The constant (`model/billing.go:6`) + ~10 guard checks (`billing.go:465,554,712,1189,1210`, `repository/workspace.go:241`) now defend against an unreachable state. Most `plan != Free` guards are always true. Collapse once data is migrated. (Cross-ref bug audit #18.)

## More general than needed — but currently used

### O4. Entitlements models an N-plan product that is really 2 plans
Every gated feature in `featureDefinition` requires `BillingPlanGrowth` and the message is hardcoded `"requires the Growth plan"`; `limitDefinition.byPlan` has two entries each (Starter=N, Growth=-1). `billingPlanRank` + `requiredPlan` indirection reduces to `if plan != Growth`. Acceptable as forward-looking config, but heavier than the current need.

### O5. Dual-calling-convention hooks (frontend)
`useBillingCheckout` / `useBillingPortal` sniff argument shape at runtime (`'wsId' in vars`) to serve both the org page and settings page (`useBilling.ts:108-180`); `useSetOnDemand` vs `useSetBillingOnDemand` are near-identical, differing only in cache-invalidation target. Fragile polymorphism — prefer two thin hooks or one hook parameterized by the invalidation key.

## Duplication / source-of-truth drift

### O6. `BillingSettingsPage.tsx` (~1,400 lines) re-implements `billingUtils` locally
Two date formatters coexist (`Intl` vs `dayjs`); local `formatNumber`/`formatMoney`/`PLAN_LABELS` duplicate exported equivalents; and **pricing/credits are hardcoded twice** — local `PLAN_OPTIONS` + `includedCreditsForPlan` (5000/25000, $99/$299) vs `billingUtils.PLAN_OPTIONS`. Pricing needs one source. Cleanest win: extract the ~20 pure derivation/format helpers into a `billingSettingsCopy.ts` module importing from `billingUtils`.

## Speculative stub

### O7. Setup-intent / card-capture scaffolding with no real flow
`useCreateSetupIntent` + `createSetupIntent` service + `SetupIntentResponse` type exist, but `handleAddCard` only fires the mutation and toasts "continue in the Stripe form" — no Stripe Elements mount; `client_secret`/`customer_id` are never consumed. Structure for an unbuilt feature. (Same item as bug audit #21.)

## Verified NOT over-engineered (keep)
On-demand block math (`ceil(overage/blockSize)` minus invoiced — minimal correct idempotent metering); billing-notice columns (written and read); `MeteredLLMProvider`/consumer interface (single impl but 10+ real call sites, reasonable test seam); `billingState.ts` lock helpers (used + unit-tested). `daysUntil` is **used** (`WorkspaceSelector.tsx:13`) — an earlier stale flag; keep it.

**Top recommendation:** O1 (delete the dead deferred-change subtree — backend + frontend + the 3 unused columns, as one PR), then O6 (collapse duplicated pricing constants to one source).
