# CRM architecture and data model

This guide explains the CRM data model, backend structure, and connections to
project management, knowledge, support, and agents. Use it to locate a CRM change;
use the linked signals guide for detailed detection and activation behavior.

CRM connects contacts, companies, and deals with support, project work, signals,
and configured automation. Similar object names do not imply HubSpot API
compatibility or automatic pipeline management without setup and authorization.

For CRM-signal architecture, rule definitions, activation safety, and local
operations, use the canonical
[`crm-signals.md`](crm-signals.md) reference. This module overview
does not duplicate that changing rule catalogue.

For the agreed Signals/Review and Playbooks experience, use the
[CRM blueprint](crm-customer-work-blueprint.md). The
[connection plan](crm-playbook-automation-change-proposal.md) documents the implemented
Playbooks → Flows → built-in Beacon + specialized skills integration. Guided setup,
explicit activation, durable checks, exact approvals and result inspection share the
existing Automation runtime and owning-module services. Independently configured
outbound sequences retain their own behavior. This overview is not a production
deployment or activation report.

## Architecture

The CRM follows Helpin's standard Handler → Service → Repository layering with RBAC authorization, GORM models, and TanStack Query on the frontend.

### Permissions

| Permission | Roles | Description |
|-----------|-------|-------------|
| `crm.read` | viewer+ | Read contacts, companies, deals, activities |
| `crm.edit` | member+ | Create/update CRM objects |
| `crm.admin` | admin, owner | Pipeline configuration, signal activation/routing policy, Playbook configuration/publication |

### Database

CRM schema changes belong in versioned SQL under `server/internal/dbmigrate/sql`.
Startup AutoMigrate runs only when enabled; Community Compose disables it and
uses the migration service. Legacy SQL under `server/migrations` is reference-only.

---

## Data Model

### Core objects

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  CRMContact  │────▶│CRMAssociation│◀────│  CRMCompany  │
│              │     │  (polymorphic │     │              │
│ lifecycle    │     │   many-to-many│     │ domain       │
│ lead_status  │     │   any-to-any) │     │ industry     │
│ custom_props │     └──────────────┘     │ revenue      │
└──────┬───────┘            ▲              └──────────────┘
       │                    │
       ▼                    │
┌──────────────┐     ┌──────┴───────┐
│ CRMActivity  │     │   CRMDeal    │
│              │     │              │
│ note/call/   │     │ pipeline_id  │
│ meeting/     │     │ stage_id     │
│ email        │     │ amount       │
└──────────────┘     │ close_date   │
                     └──────────────┘
                            │
                     ┌──────┴───────┐
                     │ CRMPipeline  │
                     │   └─ Stages  │
                     │  (open/won/  │
                     │   lost)      │
                     └──────────────┘
```

**CRMContact** — People with lifecycle stages (subscriber → lead → marketing_qualified → sales_qualified → opportunity → customer → evangelist) and lead statuses (new/open/in_progress/unqualified).

**CRMCompany** — Organizations with domain, industry, employee count, annual revenue.

**CRMDeal** — Opportunities tied to a pipeline and stage. Tracks amount, currency, close date, probability.

**CRMPipeline / CRMPipelineStage** — Configurable sales pipelines with ordered stages typed as open/won/lost.

**CRMAssociation** — Polymorphic links between supported object types, including
contacts, companies, deals, meetings, and cross-module work. Service validation
protects special relationships; changing a deal’s customer uses the deal customer
action rather than an arbitrary association.

**CRMActivity** — Records completed events (note, call, meeting, email) against
contacts, companies, or deals. Work to do lives in PM tasks; the old `task` activity
type has been retired.

Contacts, companies, and deals retain `custom_properties` JSONB fields. The old
property-definition, property-group, and static/smart-list models and routes are
not part of the current CRM implementation. Legacy migration tables are not an
API contract.

**CRMImportJob** tracks imports, processing state, and errors. See the import
handler and service for supported inputs and validation.

### Email and calendar

**CRMEmailAccount** — OAuth-connected Gmail/Microsoft accounts with encrypted tokens and sync state.

**CRMEmailThread / CRMEmailMessage** — Email threads and individual messages with direction (inbound/outbound), auto-matched to contacts by email address.

**CRMCalendarEvent** — Synced calendar events with attendee-to-contact matching and deal association.

OAuth, incremental synchronization, message normalization, and downstream
signal ingestion are documented in [`crm-email-sync.md`](crm-email-sync.md).

### Intelligence

**CRMEnrichmentResult** — Contact/company enrichment data with source and
confidence fields. The model recognizes `apollo`, `ai`, and `manual` source labels;
a label alone does not establish that a provider integration is configured.

**CRMSignal** — Durable evidence from manual entries, verified conversation
extraction and versioned deterministic rules across support, delivery, CRM,
relationship, web, product, and external domains. The stable signal taxonomy is
`buying_intent`, `objection`, `competitor_mention`, `budget_signal`,
`timeline_signal`, `champion_signal`, and `risk_signal`.

**CRMDealHealthScore** — Deterministic 0–100 deal health with a versioned factor
breakdown. The service refreshes it on reads and selected signal ingestion paths;
the API also runs a workspace sweep at startup and every six hours. Refresh errors
can leave older persisted results, so use `calculated_at` when assessing freshness.

**CRMSuggestion** — Recommendations such as `follow_up`, `deal_create`,
`deal_advance`, `enrichment`, and `risk_alert`, plus Playbook actions. Decision status
includes pending, accepted, dismissed, superseded, and expired. A separate
`execution_status` records pending, in-progress, succeeded, failed, or manual work
required. Acceptance alone does not mean an action completed; Playbook actions
use their own revision-checked executor.

### Outreach and writing profiles

**CRMEmailSequence** defines email/task steps, preceding delays, sending windows,
and optional stage-entry enrollment. **CRMSequenceEnrollment** snapshots recipient
content and sequence version and tracks progress. **CRMSequenceDelivery** journals
external operations before execution. The Temporal scheduled-events dispatcher
calls the outreach runner; durable claims, delivery records, and reconciliation
handle retries. This is not a blanket exactly-once guarantee for provider sends.

**CRMWritingProfile** — Per-workspace-member writing style attributes (tone, formality, patterns) for AI-assisted email drafting.

---

## API Endpoints

The tables below list selected endpoints relative to `/api`; the current
[router](../server/internal/router/router.go) is authoritative. CRM routes require
authentication, workspace access, an active workspace, and CRM module access.
Supply workspace context through `X-Workspace-ID` or the `workspace_id` query
parameter. Route permissions below are minimum gates; services can add ownership,
mailbox, or action-specific checks.

### Contacts
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/contacts` | crm.read | List contacts (filters: search, lifecycle_stage, lead_status, owner_member_id, company_id, source) |
| POST | `/crm/contacts` | crm.edit | Create contact |
| GET | `/crm/contacts/{id}` | crm.read | Get contact |
| PUT | `/crm/contacts/{id}` | crm.edit | Update contact |
| DELETE | `/crm/contacts/{id}` | crm.edit | Delete contact |
| GET | `/crm/contacts/{id}/activities` | crm.read | List contact activities |
| GET | `/crm/contacts/{id}/associations` | crm.read | List contact associations |

### Companies
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/companies` | crm.read | List companies (filters: search, industry, owner_member_id) |
| POST | `/crm/companies` | crm.edit | Create company |
| GET | `/crm/companies/{id}` | crm.read | Get company |
| PUT | `/crm/companies/{id}` | crm.edit | Update company |
| DELETE | `/crm/companies/{id}` | crm.edit | Delete company |
| GET | `/crm/companies/{id}/activities` | crm.read | List company activities |
| GET | `/crm/companies/{id}/associations` | crm.read | List company associations |

### Deals
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/deals` | crm.read | List deals (filters: search, pipeline_id, stage_id, owner_member_id, contact_id, company_id) |
| POST | `/crm/deals` | crm.edit | Create deal |
| GET | `/crm/deals/{id}` | crm.read | Get deal |
| PUT | `/crm/deals/{id}` | crm.edit | Update deal |
| DELETE | `/crm/deals/{id}` | crm.edit | Delete deal |
| GET | `/crm/deals/{id}/activities` | crm.read | List deal activities |
| GET | `/crm/deals/{id}/associations` | crm.read | List deal associations |

### Pipelines
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/pipelines` | crm.read | List pipelines with stages |
| POST | `/crm/pipelines` | crm.admin | Create pipeline |
| GET | `/crm/pipelines/{id}` | crm.read | Get pipeline |
| PUT | `/crm/pipelines/{id}` | crm.admin | Update pipeline (including stages) |
| DELETE | `/crm/pipelines/{id}` | crm.admin | Delete pipeline |

### Associations
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| POST | `/crm/associations` | crm.edit | Create association |
| DELETE | `/crm/associations/{id}` | crm.edit | Delete association |

### Activities
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/activities` | crm.read | List activities |
| POST | `/crm/activities` | crm.edit | Create activity |
| GET | `/crm/activities/{id}` | crm.read | Get activity |
| PUT | `/crm/activities/{id}` | crm.edit | Update activity |
| DELETE | `/crm/activities/{id}` | crm.edit | Delete activity |

### Imports
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| POST | `/crm/imports` | crm.edit | Create import job |
| GET | `/crm/imports/{id}` | crm.read | Get import status |
| POST | `/crm/imports/{id}/process` | crm.edit | Start processing |

### Email
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/email/accounts` | crm.read | List email accounts |
| POST | `/crm/email/accounts` | crm.edit | Connect account |
| DELETE | `/crm/email/accounts/{id}` | crm.edit | Disconnect account |
| GET | `/crm/email/threads` | crm.read | List threads |
| GET | `/crm/email/messages` | crm.read | List messages |
| GET | `/crm/contacts/{id}/emails` | crm.read | Contact's emails |
| GET | `/crm/deals/{id}/emails` | crm.read | Deal's emails |

### Calendar
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/calendar/events` | crm.read | List events |
| GET | `/crm/contacts/{id}/calendar` | crm.read | Contact's events |
| GET | `/crm/deals/{id}/calendar` | crm.read | Deal's events |

### Intelligence
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/enrichments` | crm.read | List enrichments |
| POST | `/crm/enrichments` | crm.edit | Create enrichment |
| GET | `/crm/signals` | crm.read | List CRM signals |
| POST | `/crm/signals` | crm.edit | Create signal |
| GET | `/crm/signals/feed` | crm.read | Ranked workspace signal feed |
| GET | `/crm/signals/brief` | crm.read | Current signal brief |
| GET | `/crm/meetings/{id}/signal-brief` | crm.read | Meeting signal brief |
| GET | `/crm/signals/precision` | crm.read | Rule/version feedback report |
| GET | `/crm/signals/routing-policy` | crm.read | Active routing policy |
| POST | `/crm/signals/routing-policy` | crm.admin | Create routing policy version |
| POST | `/crm/signals/rules/{ruleKey}/versions/{version}/activate` | crm.admin | Activate rule version |
| POST | `/crm/signals/external-evidence` | crm.admin | Ingest normalized external evidence |
| POST | `/crm/signals/{id}/review` | crm.edit | Record review feedback |
| POST | `/crm/signals/{id}/dismiss` | crm.edit | Dismiss with a reason |
| POST | `/crm/signals/{id}/acted` | crm.edit | Record action feedback |
| GET | `/crm/contacts/{id}/signals` | crm.read | Contact's signals |
| GET | `/crm/deals/{id}/signals` | crm.read | Deal's signals |
| GET | `/crm/companies/{id}/signals` | crm.read | Company's signals |
| GET | `/crm/health-scores` | crm.read | List health scores |
| GET | `/crm/deals/{id}/health-score` | crm.read | Deal's health score |
| GET | `/crm/suggestions` | crm.read | List suggestions |
| PUT | `/crm/suggestions/{id}` | crm.edit | Update suggestion (accept/dismiss) |

### Outreach sequences

These routes are registered when the outreach handler is configured.

| Method | Path | Permission | Description |
| --- | --- | --- | --- |
| GET | `/crm/outreach/sequences` | crm.read | List sequences |
| POST | `/crm/outreach/sequences` | crm.edit | Create sequence |
| PUT | `/crm/outreach/sequences/{id}` | crm.edit | Update sequence |
| POST | `/crm/outreach/sequences/{id}/enroll` | crm.edit | Preview or confirm recipient enrollment |
| GET | `/crm/outreach/enrollments` | crm.read | List enrollments |
| GET | `/crm/outreach/enrollments/{id}` | crm.read | Enrollment and delivery detail |
| POST | `/crm/outreach/enrollments/{id}` | crm.edit | Control enrollment |

### Writing Profiles
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/writing-profiles` | crm.read | List profiles |
| GET | `/crm/writing-profiles/member/{memberId}` | crm.read | Get member profile |
| POST | `/crm/writing-profiles` | crm.edit | Create profile |
| PUT | `/crm/writing-profiles/{id}` | crm.edit | Update profile |

### Search
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/search?q=term` | crm.read | Search across contacts, companies, deals |

---

## Frontend

### Routes

| Path | Page | Description |
|------|------|-------------|
| `/w/:slug/crm` | — | Redirects to /overview |
| `/w/:slug/crm/contacts` | Contacts | Contact list with search, filters, create |
| `/w/:slug/crm/contacts/:id` | ContactDetail | Full contact view with associations, activities, emails, signals |
| `/w/:slug/crm/companies` | Companies | Company list |
| `/w/:slug/crm/companies/:id` | CompanyDetail | Company detail view |
| `/w/:slug/crm/deals` | Deals | Deal list + Kanban board toggle |
| `/w/:slug/crm/deals/:id` | DealDetail | Deal detail with pipeline stage, health score |
| `/w/:slug/crm/insights` | Signals | Unified customer situations and standalone recommendations, with visible filters and exact action decisions |
| `/w/:slug/crm/insights?view=evidence` | Insights | Preserved raw evidence, deal health, CRM search and setup destinations |
| `/w/:slug/crm/review` | — | Compatibility redirect to Signals: Needs approval; defaults to Everyone when no scope is supplied |
| `/w/:slug/crm/playbooks` | Playbooks | Published/draft policy, manual participation, guided automation setup and explicit activation |
| `/w/:slug/crm/playbooks/:playbookId` | PlaybookDetail | Signals / Setup / Activity for the same canonical customer work |

The former `/crm/lists` and `/crm/sequences` page routes are absent. Current
email and outreach UI lives under `/w/:slug/crm/emails`; overview and meeting
pages live under `/crm/overview` and `/crm/meetings`.

### Sidebar Navigation

Current CRM sub-navigation is defined in the shared sidebar configuration:

- Overview
- Contacts
- Companies
- Deals
- Emails
- Meetings
- Playbooks
- Signals

Review is not a second sidebar queue. Preserved routes are not necessarily primary
navigation items; see the route table and the current CRM reference.

### Code entry points

Use these current sources instead of a fixed file count or a legacy migration
inventory:

- [CRM query hooks](../frontend/src/hooks/queries/useCRM.ts) and
  [service adapter](../frontend/src/lib/services/crmService.ts) define data access.
  Hooks are workspace-scoped: for example `useContact(workspaceId, contactId)`,
  `useCRMActivities(workspaceId, filters)`, and `useCreateContact(workspaceId)`.
- [Contacts table](../frontend/src/components/crm/ContactsTable.tsx),
  [deal board](../frontend/src/components/crm/DealBoard.tsx), and
  [entity signals](../frontend/src/components/crm/EntitySignals.tsx) render core CRM data.
- [Email page](../frontend/src/pages/crm/Emails.tsx) and
  [outreach sequence editor](../frontend/src/components/crm/outreach/SequenceEditor.tsx)
  expose email and sequence work.
- [Association model](../server/internal/model/crm_association.go) and
  [association service](../server/internal/service/crm_association.go) define
  supported relationships and validation, including cross-module links.
- [Outreach models](../server/internal/model/crm_outreach.go),
  [service](../server/internal/service/crm_outreach.go), and
  [runner](../server/internal/service/crm_outreach_runner.go) define current sequences.
- [Suggestion model](../server/internal/model/crm_suggestion.go) and
  [suggestion service](../server/internal/service/crm_suggestion.go) distinguish decisions
  from execution outcomes. [Signal service](../server/internal/service/crm_signal.go)
  calculates health scores; [API startup](../server/cmd/api/main.go) wires the sweep.
- [Role matrix](../server/internal/authorization/rbac.go) and
  [router](../server/internal/router/router.go) define base permissions and endpoints.

## Design boundaries

Custom properties use JSONB on contacts, companies, and deals. Associations use a
polymorphic table with service validation; storage flexibility does not authorize
arbitrary cross-workspace links. Display IDs use `CON-`, `COM-`, and `DEAL-` prefixes.
Email OAuth tokens are encrypted at rest. Signal extraction, deterministic
scoring, Playbook activation, and outreach delivery are separate concerns; consult
the linked feature guides for their gates and failure behavior.

## Further work

This overview intentionally does not maintain a speculative backlog. Use dated
documents under `docs/plans` for proposals and verify implementation status in
the linked canonical feature references before relying on a plan.
