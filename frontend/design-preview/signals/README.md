# Connected CRM design mockup

Status: **Historical design reference; automation direction reconciled 2026-09-07.**
Use the [CRM blueprint](../../../docs/crm-customer-work-blueprint.md) for agreed
product scope and the [connection plan](../../../docs/crm-playbook-automation-change-proposal.md)
for the Playbooks → Flows → Beacon + skills architecture. The mock's simulated
Flow steps, “CRM Assistant” label, setup tabs and enrollment behavior are not
implementation instructions. The agreed design reuses built-in Beacon, requires
no ordered-step Flow editor, and keeps existing Playbook Signals / Setup / Activity
tabs. This documentation reconciliation does not change the mock code.

An isolated desktop mockup using Helpin’s real global stylesheet, font, icons,
Quiet components, tabs, pickers, dialogs, sheets, and conversation composer.
The shell is a visual stand-in. Production routes, APIs and customer data are
unchanged. All three agreed customer journeys are represented together.

## Review

From `frontend/`:

```sh
node_modules/.bin/vite --config design-preview/signals/vite.config.ts
```

Open `http://localhost:5187/design-preview/signals/`, using the server’s reachable
hostname for remote access. **Explore screens** opens the connected screen guide;
`?guide=1` opens it directly.

Main destinations:

- `?view=signals`: unified daily work and approvals; five fixed table columns.
- `?view=playbooks`: dedicated catalogue, ownership and enrollment controls.
- `?view=playbook&id=renewal-recovery`: overview and operating rules.
- `?view=playbook&id=renewal-recovery&tab=Customers`: participating customers.
- `?view=playbook&id=renewal-recovery&tab=Outcomes`: recorded customer results.
- `?view=edit&id=renewal-recovery`: configuration, preview and publishing.
- `?view=customer&id=Northstar`: customer progress and relevant linked work.
- `?view=flows&id=renewal-recovery`: connected execution configuration.
- `?view=agents`: historical “CRM Assistant” simulation; the real built-in CRM Agent is Beacon.
- `?view=activity`: execution results, separate from customer outcomes.

Direct drawers: `?situation=harbor-buyer`, `lumen-handoff`, `northstar-risk`,
`orbit-delivery`, or `cedar-deal`. Assignment, category and work-state filters
are in the URL. Search and local detail tabs are transient. Browser Back works
between main destinations.

## Connected sample interactions

- Edit and approve a response; distinguish failure from waiting on the customer.
- Accept handoff ownership, commitments and the first-value goal.
- Review standalone deal creation, stage changes and contact enrichment.
- Request changes with feedback, review evidence, assign, pause/resume or reopen.
- Record customer outcomes; blocker resolution alone does not close a renewal.
- Explicitly create and link an optional PM task with an owner and deadline.
- Configure or create a playbook, review a sample policy walkthrough, publish,
  enable/disable new enrollment, and manually enroll a sample customer.
- Inspect the same customer work from Signals, Playbooks, customer or activity.

Existing situations keep their starting configuration snapshot when a new
playbook definition is published. Definition revisions are not partial product
releases. Category is the only tab-style selector on Signals; all filters and
all five columns remain visible. No mobile or extra keyboard work is added.

## Simulation boundaries

Everything stays in tab memory. Reload or Reset restores the samples. No backend,
authentication, real email/OAuth, PM writes, agent execution, scheduling or durable
storage is connected. The publish check illustrates policy behavior; it is not
a runtime validator. Source links show sample evidence; other module links
explain their existing role. Empty/loading/error/read-only controls simulate
queue/catalogue states, not an entire backend. The existing dark theme is available.

This mock is not proof of production capability parity or authorization safety.
Those remain completion requirements in the linked blueprint. The mock itself
does not alter application routing or activate automation. In the implementation
branch, Review already redirects to Signals; see the
[current CRM reference](../../../docs/crm-signals.md#review-consolidation).

## Verification

From `frontend/`, with the preview server running:

```sh
node_modules/.bin/tsc -p design-preview/signals/tsconfig.json
node_modules/.bin/vite build --config design-preview/signals/vite.config.ts
node design-preview/signals/verify-workspace.mjs
```

The browser check covers the connected desktop journeys, approvals, exact changes,
publishing, enrollment, outcomes, permissions, error recovery and navigation.
Screenshots are captured under `/tmp/crm-design-*.png`. Set `CRM_PREVIEW_URL` to
check another host. The test also asserts no browser runtime errors.

`App.tsx`, `SignalDetail.tsx`, `data.ts`, `presentation.ts` and `verify.mjs` preserve
the earlier Signals-only mock for reference. The connected mock entry is `WorkspaceMock`;
its browser simulation check is `verify-workspace.mjs`, not production acceptance.
