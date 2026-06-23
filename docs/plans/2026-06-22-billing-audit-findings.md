# Billing Audit Findings And Product Decisions

Status: active implementation notes for the Stripe billing worktree.

## Product Decisions

- Billing management is owner-only for now. Non-owners can see billing status and upgrade prompts, but billing actions use a short disabled CTA such as "Ask owner".
- Trial workspaces get one 14-day trial with a hard 25,000 AI usage unit cap. Trial usage does not reset periodically.
- Starter includes up to 500 documents. Document creation is blocked once the limit is reached.
- Starter includes up to 5,000 CRM contacts. Automated contact creation can continue for support/automation ingestion, but viewing CRM contacts over the limit is blocked with an upgrade prompt.
- Starter does not include custom AI agents, automation flows, scheduled agents, or cron flows.
- Growth upgrade messaging should mention the relevant blocked feature plus broader Growth benefits, including unlimited teams, unlimited CRM contacts, unlimited documents, custom AI agents, automation flows, scheduled agents, 25,000 monthly AI usage units, multilingual help center, round robin assignment, SLA policies, deal automation, and removing Helpin branding.

## Stripe Flow

- Feature-limit `Upgrade` CTAs should route to Helpin Billing Settings first. The Billing page provides context about the blocked feature, current plan, owner-only billing permissions, and available upgrade benefits.
- Billing Settings should launch Stripe Checkout for subscription upgrades.
- Payment recovery actions such as `Update payment` should launch the Stripe-hosted Customer Portal directly because the task is narrow and payment-specific.
- Hosted Stripe surfaces remain the source of truth for payment method collection, invoices, and subscription management.

## Workspace Locking

- Stripe `unpaid` subscriptions lock the workspace outside billing so the owner can still reach Billing Settings and recover payment.
- `past_due` remains grace-state behavior and should not immediately lock the workspace.
- The locked workspace banner should be visible outside Billing Settings to all users, with CTA behavior adjusted by billing permission.

## AI Usage Enforcement

- Backend AI execution must fail closed. Agent/coding/support/automation LLM paths must run with metering context and preflight credit checks before model calls.
- If AI usage is exhausted, the user-facing frontend should show `UpgradeRequiredDialog`; it should not show raw `AI usage exhausted` toasts.
- Any frontend surface that can start an agent or AI rewrite must classify errors through `getUpgradeRequiredReason` before falling back to toast errors.
- When a create flow both creates an entity and starts an agent, handle `agent_run_error` specially:
  - If it is an upgrade-required billing error, keep the create modal open and show the upgrade dialog.
  - If it is a normal agent-start failure, show a "created, but agent did not start" warning/error.

Covered frontend launch surfaces:

- Automation Agents page: manual runs, custom agent creation, agent template creation.
- Automation Flows page: flow create/update/toggle/run-now/template install.
- PM task agent panel: direct task agent runs.
- PM epic planner panel: direct epic planner runs.
- Coding session handoff: next-agent handoff runs from completed coding sessions.
- Global epic panel: assigned epic agent run.
- Create task modal: `Save & run agent` and post-create `agent_run_error`.
- Global create epic modal: `Create & run agent` and post-create `agent_run_error`.
- Support thread: run assigned support agent.
- Support reply composer: AI rewrite actions.

## Upgrade Dialog Behavior

- `UpgradeRequiredDialog` owns the upgrade explanation and Growth benefits.
- Parent surfaces that open the upgrade dialog from inside another dialog must pass `onUpgrade` to close the parent modal before navigating to Billing Settings.
- Current nested-dialog closures:
  - Create document modal.
  - Create task modal.
  - Global create epic modal.
  - Automation agent create/template/run dialogs.
  - Automation flow composer/gallery/template setup dialogs.

## Entity Limits

- Document limit is enforced during document creation, including user-created documents and backend/agent-created documents that use the same service path.
- CRM contact limit is enforced on viewing contacts rather than automated contact creation so support routes can still preserve incoming customer context.
- CRM import should preflight contact count before import starts and route limit failures to upgrade messaging.

## Testing Notes

- Unit coverage exists for upgrade-required error classification, including AI usage exhaustion and document limits.
- Create-document regression coverage verifies that clicking Upgrade closes the create-document modal before routing to Billing Settings.
- Frontend build should be run after changing upgrade-dialog call sites because these surfaces span large page components.
