# Workspace-Aware Lifecycle Analytics Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a workspace-aware activation and billing event system that feeds Usermaven for user/company analytics and Customer.io for behavioral campaigns without confusing users who belong to multiple workspaces.

**Architecture:** Keep one canonical person identity per user. Treat each workspace as the behavioral and billing context on events; represent the parent organization as the Usermaven company and as a Customer.io object. Customer.io object type `1` is the workspace and object type `2` is the organization. The repository has an optional backend Customer.io Track API path, but the deployed integration is currently frontend-only; the plan enables and extends the backend path so all memberships and authoritative lifecycle events are synchronized. Retain frontend tracking for interaction and module behavior, with both providers receiving the same normalized event contract.

**Tech Stack:** Go 1.24, Chi, GORM, PostgreSQL, Stripe billing/webhooks, React 19, TypeScript, TanStack Query, Usermaven SDK/server API, Customer.io Pipelines/Track API, Vitest, Go tests.

---

## Scope and invariants

- A user is identified globally by `user_id` in both providers.
- A workspace is the unit of activation, billing, trial expiry, module access, and campaign context.
- An organization is the stable parent/company context and receives derived aggregate attributes.
- Never put a single workspace’s `plan`, `trial_ends_at`, `billing_status`, or `workspace_role` on the global person profile.
- Every product, billing, and lifecycle event emitted for a workspace includes `workspace_id` and `organization_id` when available.
- Trial and upgrade campaigns target owners/admins through the relationship to the affected workspace.
- Module education targets eligible members based on module access, role, use case, and unfinished activation milestones; viewers are excluded by default.
- Analytics must not include message bodies, support content, tokens, payment card data, or other sensitive payloads.

## Proposed event contract

All events use a shared envelope:

```ts
type LifecycleEvent = {
  name: string;
  occurred_at: string;
  user_id?: string;
  workspace_id?: string;
  organization_id?: string;
  source: 'frontend' | 'backend' | 'stripe';
  properties: Record<string, unknown>;
};
```

Required workspace properties where applicable:

```ts
{
  workspace_id,
  organization_id,
  workspace_role,
  membership_status,
  enabled_modules,
  plan,
  billing_status,
  trial_ends_at,
  trial_days_left
}
```

Initial event catalog:

| Area | Event | Trigger |
|---|---|---|
| Workspace | `workspace_created` | Workspace is created and billing row initialized |
| Onboarding | `workspace_onboarding_use_cases_selected` | User selects one or more use cases |
| Membership | `workspace_member_invited` | Invite is created |
| Membership | `workspace_member_joined` | Invite is accepted or membership becomes active |
| Activation | `module_viewed` | User reaches a module’s meaningful entry surface, deduped per session/day |
| Activation | `module_first_value` | User completes the module’s first meaningful outcome |
| Activation | `module_repeat_value` | User reaches the module’s repeat-use threshold |
| Billing | `trial_started` | Workspace receives its 14-day trial |
| Billing | `subscription_started` | Workspace becomes paid after checkout |
| Billing | `subscription_changed` | Plan or interval changes |
| Billing | `trial_expired` | Workspace trial ends without conversion |
| Billing | `payment_failed` | Stripe reports a payment failure |
| Billing | `subscription_canceled` | Subscription is canceled or ends |

Module first-value milestones must be explicitly defined and tested:

| Module | First value | Repeat value candidate |
|---|---|---|
| PM | Create the first task/story | Complete or update 3 tasks |
| Docs/knowledge | Create or publish the first document/article | Create or edit 3 documents |
| Support | Send the first customer reply | Send 3 replies or resolve one conversation |
| CRM | Create the first contact or deal | Create/update 3 CRM records |
| Automation | Enable or execute the first automation | Execute 3 successful runs |

If a module has multiple valid use cases, the milestone registry should allow alternatives rather than forcing a single product path.

## Organization aggregate contract

Usermaven company attributes should be derived snapshots, recalculated server-side:

- `workspace_count`
- `active_workspace_count_30d`
- `trialing_workspace_count`
- `paid_workspace_count`
- `activated_workspace_count`
- per-module workspace counts such as `support_workspace_count`
- `member_count`
- `active_user_count_30d`
- `activated_user_count_30d`
- `organization_trialing`
- `organization_plan_mix`
- `aggregates_updated_at`

Do not send all workspace IDs or nested per-workspace activation blobs as company attributes. Raw workspace-specific behavior remains in event properties.

## File map

### Frontend analytics and module instrumentation

- Modify: `frontend/src/lib/analytics.ts` — normalize the shared event envelope, add workspace/member context helpers, preserve Usermaven organization company identity, and remove workspace-as-company behavior.
- Modify: `frontend/src/lib/types.ts` — add any missing analytics-safe workspace/membership types.
- Create: `frontend/src/lib/activationMilestones.ts` — central module milestone registry and event names; no page-specific milestone strings.
- Modify: `frontend/src/lib/workspaceOnboardingUseCases.ts` — route use-case events through the shared event helper.
- Modify: module entry/action files under `frontend/src/routes/_authenticated/w/$slug/`, `frontend/src/components/`, and relevant module services — emit only the registered first/repeat value events at successful action boundaries.
- Test: `frontend/src/lib/__tests__/analytics.test.ts` — identity, workspace context, dual-provider fan-out, and no global workspace billing attributes.
- Create: `frontend/src/lib/__tests__/activationMilestones.test.ts` — milestone registry validation and module coverage.

### Backend lifecycle and billing events

- Create: `server/internal/analytics/events.go` — canonical Go event types, property validation, redaction rules, and provider-neutral emitter interface.
- Create: `server/internal/analytics/dispatcher.go` — asynchronous, non-blocking provider dispatch with structured error logging and no business-operation failure on analytics outage.
- Create: `server/internal/analytics/usermaven.go` — server-side Usermaven event/company adapter using the server token.
- Modify: `server/internal/service/customer_io.go` — extend the existing optional Customer.io Track API client and identity service for all workspace/organization relationships and lifecycle event delivery; do not create a second adapter.
- Modify: `server/internal/config/config.go`, `server/cmd/api/main.go`, and environment example files — document and enable the existing backend Customer.io Track API configuration in deployed environments.
- Modify: `server/internal/service/workspace.go` — emit workspace creation and initial trial events after successful transaction completion.
- Modify: `server/internal/service/billing.go` — emit subscription, trial-expiry, payment-failure, cancellation, and plan-change events after durable state changes.
- Modify: Stripe webhook handling files — preserve workspace ID/subscription ID correlation and pass the affected workspace into lifecycle events.
- Modify: membership service/handler files identified by the audit — synchronize all active workspace and organization memberships, including creation, acceptance, revocation, role changes, and module access changes.
- Create: `server/internal/analytics/aggregates.go` — organization aggregate query and snapshot builder.
- Create or modify: background worker/scheduler registration — periodic organization aggregate reconciliation to repair missed events.
- Test: `server/internal/analytics/*_test.go` — contract validation, redaction, provider failure isolation, deduplication, and aggregate calculations.
- Test: billing and membership service tests — assert events are emitted only after successful writes and carry the correct workspace.

### Customer.io configuration and campaign handoff

- Create: `docs/customer-io/workspace-lifecycle-data-contract.md` — object type `1` for workspaces, object type `2` for organizations, relationship attributes, event properties, suppression rules, and Liquid examples.
- Create: `docs/customer-io/trial-lifecycle-campaigns.md` — campaign entry/exit criteria and message schedule.
- Create: `docs/customer-io/module-activation-campaigns.md` — module-specific audiences, milestones, and role rules.
- Create: `scripts/verify-lifecycle-analytics.mjs` — optional non-production verification utility for inspecting event payloads without sending sensitive data.

## Implementation tasks

### Task 1: Freeze the event and identity contract

**Files:** `frontend/src/lib/analytics.ts`, `frontend/src/lib/activationMilestones.ts`, `server/internal/analytics/events.go`, related tests.

- [ ] Write failing TypeScript tests for a workspace-scoped event containing user, workspace, organization, membership, billing, and source context.
- [ ] Write failing Go tests for required identifiers, allowed event names, UTC timestamps, and sensitive-property redaction.
- [ ] Implement provider-neutral event builders in both languages with matching names and property casing.
- [ ] Make `trackAnalyticsEvent` fan out the normalized payload to Usermaven and Customer.io.
- [ ] Ensure `buildAnalyticsUserTraits` contains only global user traits plus stable organization context.
- [ ] Remove the assumption that the active workspace is the Usermaven company.
- [ ] Run `cd frontend && npm test -- --run src/lib/__tests__/analytics.test.ts` and `cd server && go test ./internal/analytics`.
- [ ] Commit: `feat: define workspace-aware lifecycle analytics contract`.

### Task 2: Implement Usermaven organization and event context

**Files:** `frontend/src/lib/analytics.ts`, `server/internal/analytics/usermaven.go`, `server/internal/analytics/aggregates.go`, configuration files, tests.

- [ ] Write failing tests for organization aggregate snapshots and stable Usermaven company identity.
- [ ] Implement organization aggregate queries across workspaces, memberships, module access, activation milestones, and recent activity.
- [ ] Send aggregate company attributes with `aggregates_updated_at` and deterministic defaults for empty organizations.
- [ ] Send workspace context on every event as event properties.
- [ ] Keep Usermaven `group()` out of the workspace switching path unless it is confirmed to represent the stable organization company.
- [ ] Add event/property redaction and bounded list handling for `organization_plan_mix`.
- [ ] Add a periodic reconciliation job and an on-change update path.
- [ ] Run backend aggregate tests and frontend analytics tests.
- [ ] Commit: `feat: add Usermaven organization lifecycle aggregates`.

### Task 3: Synchronize Customer.io workspace objects and memberships

**Files:** `server/internal/service/customer_io.go`, membership/workspace services, config, tests, Customer.io contract doc.

- [ ] Write failing tests for a user related to multiple workspaces with different roles.
- [ ] Implement workspace object type `1` upsert with plan, trial, billing, module, and aggregate-safe attributes.
- [ ] Implement organization object type `2` upsert with organization aggregates.
- [ ] Implement relationship upsert with `workspace_role`, `membership_status`, module access, and membership timestamps for every active relationship.
- [ ] Implement relationship deletion/revocation handling.
- [ ] Perform a complete membership synchronization from the backend so relationships do not depend only on visiting a workspace in the frontend.
- [ ] Ensure workspace A events cannot use workspace B’s role or billing state.
- [ ] Run Customer.io adapter tests using a fake HTTP server; no live credentials in tests.
- [ ] Commit: `feat: sync Customer.io workspace relationships`.

### Task 4: Emit authoritative workspace billing lifecycle events

**Files:** `server/internal/service/billing.go`, Stripe webhook handler/service files, `server/internal/analytics/*`, billing tests.

- [ ] Add failing tests for trial creation, checkout success, subscription change, trial expiry, payment failure, cancellation, and reactivation.
- [ ] Emit events only after the corresponding database transaction/state transition succeeds.
- [ ] Include both `workspace_id` and `organization_id` plus subscription/customer IDs where safe and useful; never include payment method secrets or card data.
- [ ] Preserve the workspace/subscription correlation when Stripe reports only a customer ID.
- [ ] Make lifecycle dispatch idempotent using the Stripe event ID or an event idempotency key.
- [ ] Ensure analytics failure is logged and retried/queued without changing billing success/failure behavior.
- [ ] Run focused billing tests and webhook tests.
- [ ] Commit: `feat: emit workspace billing lifecycle events`.

### Task 5: Instrument module activation milestones

**Files:** `frontend/src/lib/activationMilestones.ts`, module routes/components/services, module tests.

- [ ] Add failing tests for each module’s first-value and repeat-value action boundary.
- [ ] Instrument successful PM, Docs/knowledge, Support, CRM, and Automation actions using the registry.
- [ ] Include `module`, `milestone`, `workspace_id`, `organization_id`, `workspace_role`, enabled modules, and onboarding use case.
- [ ] Deduplicate first-value events so retries, route reloads, and optimistic UI do not inflate activation.
- [ ] Avoid tracking raw customer content, document text, email bodies, ticket bodies, or CRM notes.
- [ ] Run module-specific tests and the full frontend test suite.
- [ ] Commit: `feat: instrument module activation milestones`.

### Task 6: Add lifecycle campaign specifications

**Files:** `docs/customer-io/trial-lifecycle-campaigns.md`, `docs/customer-io/module-activation-campaigns.md`, `docs/customer-io/workspace-lifecycle-data-contract.md`.

- [ ] Define Customer.io object type and relationship attributes for workspaces.
- [ ] Define trial campaigns triggered by workspace trial state or lifecycle events and sent only to related owners/admins.
- [ ] Define module campaigns for eligible members with module access and no first-value milestone.
- [ ] Exclude viewers by default; add manager-specific variants only where a module has team-level responsibilities.
- [ ] Add campaign suppression rules after first value, upgrade, cancellation, unsubscribe, or workspace revocation.
- [ ] Add frequency caps and cross-campaign priority so one user in multiple workspaces is not overwhelmed.
- [ ] Add Liquid examples that render the affected workspace name and workspace-specific trial dates.
- [ ] Document Usermaven segments and organization-level reports that correspond to each campaign.
- [ ] Commit: `docs: specify workspace-aware lifecycle campaigns`.

Recommended initial campaigns:

- Welcome and use-case routing.
- Workspace activation for owners/admins.
- Module first-value education for eligible members.
- Inactive-user reminder after module entry without first value.
- Trial day 7, day 11, day 13, and expiry campaigns for owners/admins.
- Upgrade confirmation and post-upgrade activation continuation.
- Trial-expired win-back.
- Payment failure recovery.

### Task 7: Verify in staging and hand off campaign setup

**Files:** verification script, runbook docs, CI/test configuration as needed.

- [ ] Add a seeded test organization with two workspaces and one user who is owner in one and member in the other.
- [ ] Verify Usermaven receives one stable company identity and events with distinct workspace IDs.
- [ ] Verify Customer.io shows two workspace relationships with different roles.
- [ ] Verify trial expiry for Workspace A targets only Workspace A owners/admins.
- [ ] Verify a member who activates Support is suppressed from Support first-value education but remains eligible for CRM education.
- [ ] Verify organization aggregates match the underlying workspace rows after a workspace upgrade and membership change.
- [ ] Verify provider outages do not break signup, workspace creation, billing, or module actions.
- [ ] Run `git diff --check`, frontend tests/build, `go test ./...`, and migration validation if schema changes were added.
- [ ] Record the staging payload samples and Customer.io campaign activation checklist.
- [ ] Commit: `test: verify workspace-aware lifecycle analytics`.

## Rollout order

1. Deploy contract builders and disabled provider adapters.
2. Enable Usermaven event properties and organization aggregates in staging.
3. Enable Customer.io workspace/relationship synchronization in staging.
4. Backfill current organization aggregates and workspace relationships.
5. Enable billing lifecycle events and validate trial expiry targeting.
6. Enable module first-value events one module at a time.
7. Build campaigns in draft mode and test with an internal organization.
8. Activate owner/admin trial campaigns first.
9. Activate one module education campaign, measure noise and activation lift, then expand.
10. Add reconciliation monitoring and dashboards before broad campaign rollout.

## Success criteria

- A user can belong to multiple workspaces with different roles without analytics attributes overwriting one another.
- Usermaven can report organization aggregates and workspace-scoped module behavior.
- Customer.io can target owners/admins of the affected workspace and members eligible for a specific module campaign.
- Trial expiry and upgrade events are emitted once, with correct workspace context.
- Module first-value events are deduplicated and tied to real successful outcomes.
- Analytics provider failures do not block product or billing operations.
- Campaigns show the correct workspace name, plan, and trial date.
- No sensitive product or customer content is sent to either provider.
