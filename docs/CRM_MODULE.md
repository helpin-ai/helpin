# CRM Module

AI-driven CRM integrated with Helpin's PM, Docs, Support, and Agent modules. HubSpot-compatible data model with zero-touch pipeline management, CRM signal detection, and automated outbound sequences.

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
| `crm.admin` | admin, owner | Pipeline configuration, property management, Playbook configuration/publication |

### Database

CRM schema is maintained by startup AutoMigrate, focused idempotent repository
migrations, and versioned SQL under `server/internal/dbmigrate/sql`. Legacy SQL
under `server/migrations` is reference-only.

---

## Data Model

### Core Objects (Phase 1)

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
│ email/task   │     │ amount       │
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

**CRMAssociation** — Polymorphic many-to-many linking any CRM object to any other (contact↔company, contact↔deal, company↔deal). Supports cross-module associations.

**CRMActivity** — Timeline entries (note, call, meeting, email, task) attached to contacts, companies, or deals.

All core objects support `custom_properties` (JSONB) for user-defined fields with GIN indexes for filtering.

### Properties & Lists (Phase 2)

**CRMPropertyDefinition** — Schema for custom fields per object type. Supports 10 field types: text, number, date, select, multiselect, boolean, url, email, phone, currency. Grouped by `CRMPropertyGroup` for UI organization. System properties are protected from deletion.

**CRMList** — Static lists (manual member management) and smart lists (filter_criteria JSONB evaluated dynamically). Tracks member_count.

**CRMImportJob** — CSV import with column mapping, row-by-row processing, and error tracking. Supports contacts, companies, and deals.

### Email & Calendar (Phase 3)

**CRMEmailAccount** — OAuth-connected Gmail/Microsoft accounts with encrypted tokens and sync state.

**CRMEmailThread / CRMEmailMessage** — Email threads and individual messages with direction (inbound/outbound), auto-matched to contacts by email address.

**CRMCalendarEvent** — Synced calendar events with attendee-to-contact matching and deal association.

OAuth, incremental synchronization, message normalization, and downstream
signal ingestion are documented in [`crm-email-sync.md`](crm-email-sync.md).

### Intelligence

**CRMEnrichmentResult** — Contact/company enrichment data from Apollo, AI, or manual sources with confidence scores.

**CRMSignal** — Durable, explainable evidence from verified conversation
extraction and versioned deterministic rules across support, delivery, CRM,
relationship, web, product, and external domains. The stable signal taxonomy is
`buying_intent`, `objection`, `competitor_mention`, `budget_signal`,
`timeline_signal`, `champion_signal`, and `risk_signal`.

**CRMDealHealthScore** — Deterministic 0–100 deal health with a versioned factor
breakdown. Refreshed after signal changes and by a periodic workspace sweep.

**CRMSuggestion** — AI-generated action items: follow_up, deal_create, deal_advance, enrichment, risk_alert. Statused as pending/accepted/dismissed.

### Sequences (Phase 5)

**CRMSequence** — Multi-step outbound sequences with steps defined as JSONB array (email/delay/task steps with templates and conditions). Status: draft/active/paused.

**CRMSequenceEnrollment** — Per-contact enrollment tracking with status (active/completed/paused/bounced/unsubscribed/exited) and step progress.

**CRMWritingProfile** — Per-user writing style attributes (tone, formality, patterns) for AI-assisted email drafting.

---

## API Endpoints

All endpoints are under `/api/crm/` and require workspace context (`X-Workspace-ID` header or middleware).

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

### Properties
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/properties` | crm.read | List property definitions (filter: object_type) |
| POST | `/crm/properties` | crm.admin | Create property definition |
| PUT | `/crm/properties/{id}` | crm.admin | Update property definition |
| DELETE | `/crm/properties/{id}` | crm.admin | Delete property definition |
| GET | `/crm/property-groups` | crm.read | List property groups |
| POST | `/crm/property-groups` | crm.admin | Create property group |
| PUT | `/crm/property-groups/{id}` | crm.admin | Update property group |
| DELETE | `/crm/property-groups/{id}` | crm.admin | Delete property group |

### Lists
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/lists` | crm.read | List all lists |
| POST | `/crm/lists` | crm.edit | Create list |
| GET | `/crm/lists/{id}` | crm.read | Get list |
| PUT | `/crm/lists/{id}` | crm.edit | Update list |
| DELETE | `/crm/lists/{id}` | crm.edit | Delete list |
| GET | `/crm/lists/{id}/members` | crm.read | List members |
| POST | `/crm/lists/{id}/members` | crm.edit | Add member |
| DELETE | `/crm/lists/{id}/members/{objectId}` | crm.edit | Remove member |

### Imports
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| POST | `/crm/imports` | crm.edit | Create import job |
| GET | `/crm/imports/{id}` | crm.read | Get import status |
| POST | `/crm/imports/{id}/process` | crm.edit | Start processing |

### Email
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/email-accounts` | crm.read | List email accounts |
| POST | `/crm/email-accounts` | crm.edit | Connect account |
| DELETE | `/crm/email-accounts/{id}` | crm.edit | Disconnect account |
| GET | `/crm/email-threads` | crm.read | List threads |
| GET | `/crm/email-messages` | crm.read | List messages |
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
| POST | `/crm/signals/routing-policy` | crm.edit | Create routing policy version |
| POST | `/crm/signals/rules/{ruleKey}/versions/{version}/activate` | crm.edit | Activate rule version |
| POST | `/crm/signals/external-evidence` | crm.edit | Ingest normalized external evidence |
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

### Sequences
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/sequences` | crm.read | List sequences |
| POST | `/crm/sequences` | crm.edit | Create sequence |
| GET | `/crm/sequences/{id}` | crm.read | Get sequence |
| PUT | `/crm/sequences/{id}` | crm.edit | Update sequence |
| DELETE | `/crm/sequences/{id}` | crm.edit | Delete sequence |
| GET | `/crm/sequences/{id}/enrollments` | crm.read | List enrollments |
| POST | `/crm/sequences/{id}/enrollments` | crm.edit | Enroll contact |
| PUT | `/crm/enrollments/{id}` | crm.edit | Update enrollment |

### Writing Profiles
| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/crm/writing-profiles` | crm.read | List profiles |
| GET | `/crm/writing-profiles/{id}` | crm.read | Get profile |
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
| `/w/:slug/crm` | — | Redirects to /contacts |
| `/w/:slug/crm/contacts` | Contacts | Contact list with search, filters, create |
| `/w/:slug/crm/contacts/:id` | ContactDetail | Full contact view with associations, activities, emails, signals |
| `/w/:slug/crm/companies` | Companies | Company list |
| `/w/:slug/crm/companies/:id` | CompanyDetail | Company detail view |
| `/w/:slug/crm/deals` | Deals | Deal list + Kanban board toggle |
| `/w/:slug/crm/deals/:id` | DealDetail | Deal detail with pipeline stage, health score |
| `/w/:slug/crm/lists` | Lists | Static/smart list management + import wizard |
| `/w/:slug/crm/sequences` | Sequences | Sequence list |
| `/w/:slug/crm/sequences/:id` | SequenceDetail | Sequence builder + enrollments |
| `/w/:slug/crm/insights` | Signals | Unified customer situations and standalone recommendations, with visible filters and exact action decisions |
| `/w/:slug/crm/insights?view=evidence` | Insights | Preserved raw evidence, deal health, CRM search and setup destinations |
| `/w/:slug/crm/review` | — | Compatibility redirect to Signals: Everyone + Needs approval |
| `/w/:slug/crm/playbooks` | Playbooks | Published/draft policy, manual participation, guided automation setup and explicit activation |
| `/w/:slug/crm/playbooks/:playbookId` | PlaybookDetail | Signals / Setup / Activity for the same canonical customer work |

### Sidebar Navigation

Current CRM sub-navigation is defined in the shared sidebar configuration:

- Overview
- Contacts
- Companies
- Deals
- Meetings
- Signals
- Playbooks

Review is not a second sidebar queue. Preserved routes are not necessarily primary
navigation items; see the route table and the current CRM reference.

### Key Components

| Component | Description |
|-----------|-------------|
| `ContactsTable` | TanStack Table with lifecycle stage badges and filters |
| `CompaniesTable` | Company list with industry and revenue columns |
| `DealsTable` | Deal list with pipeline stage indicators |
| `DealBoard` | Kanban board grouped by pipeline stages (dnd-kit) |
| `ActivityTimeline` | Shared activity log with type-specific icons |
| `EmailTimeline` | Email thread viewer for contact/deal detail |
| `CalendarEvents` | Calendar event list |
| `EntitySignals` | Signal badges and detail cards |
| `DealHealthScore` | Visual health score indicator |
| `SuggestionsPanel` | AI suggestion cards with accept/dismiss |
| `EnrichmentCard` | Enrichment data display |
| `PropertyEditor` | Dynamic field renderer (10 field types) |
| `PropertySettings` | Property definition management |
| `PipelineSettings` | Pipeline/stage configuration |
| `ListManager` | Smart list filter builder + static list members |
| `CRMImportWizard` | Multi-step CSV import (upload → map → preview → import) |
| `SequenceBuilder` | Visual sequence step editor |
| `SequenceEnrollments` | Enrollment tracking table |
| `CreateContactDialog` | Contact creation modal |
| `CreateCompanyDialog` | Company creation modal |
| `CreateDealDialog` | Deal creation modal |

### Query Hooks

All CRM data fetching uses TanStack Query hooks exported from `hooks/queries/useCRM.ts`:

```typescript
// Read hooks
useContacts(workspaceId, filters)
useContact(contactId)
useCompanies(workspaceId, filters)
useDeals(workspaceId, filters)
usePipelines(workspaceId)
useCRMActivities(filters)
useSequences(workspaceId)
// ... 30+ hooks total

// Mutation hooks
useCreateContact()
useUpdateContact()
useDeleteContact()
useCreateDeal()
// ... matching mutations for all entities
```

---

## File Inventory

### Backend (62 files)

```
server/internal/model/
  crm_contact.go, crm_company.go, crm_deal.go, crm_association.go,
  crm_activity.go, crm_property.go, crm_list.go, crm_import.go,
  crm_email.go, crm_calendar.go, crm_enrichment.go, crm_signal.go,
  crm_suggestion.go, crm_sequence.go, crm_writing_profile.go

server/internal/repository/
  crm_contact.go, crm_company.go, crm_deal.go, crm_association.go,
  crm_activity.go, crm_property.go, crm_list.go, crm_import.go,
  crm_email.go, crm_calendar.go, crm_enrichment.go, crm_signal.go,
  crm_suggestion.go, crm_sequence.go, crm_writing_profile.go

server/internal/service/
  crm_contact.go, crm_company.go, crm_deal.go, crm_association.go,
  crm_activity.go, crm_property.go, crm_list.go, crm_import.go,
  crm_email.go, crm_calendar.go, crm_enrichment.go, crm_signal.go,
  crm_suggestion.go, crm_sequence.go, crm_writing_profile.go,
  crm_search.go

server/internal/handler/
  crm_contact.go, crm_company.go, crm_deal.go, crm_association.go,
  crm_activity.go, crm_property.go, crm_list.go, crm_import.go,
  crm_email.go, crm_calendar.go, crm_enrichment.go, crm_signal.go,
  crm_suggestion.go, crm_sequence.go, crm_writing_profile.go,
  crm_search.go

server/migrations/
  025_crm_module.sql
  026_crm_properties_lists.sql
  027_crm_email_calendar.sql
  028_crm_intelligence.sql
  029_crm_sequences.sql
```

### Frontend (40+ files)

```
frontend/src/lib/
  crmTypes.ts
  services/crmService.ts

frontend/src/hooks/queries/
  useCRM.ts

frontend/src/components/crm/
  ContactsTable.tsx, CompaniesTable.tsx, DealsTable.tsx,
  DealBoard.tsx, ActivityTimeline.tsx, EmailTimeline.tsx,
  CalendarEvents.tsx, EmailAccountConnect.tsx,
  EntitySignals.tsx, DealHealthScore.tsx, SuggestionsPanel.tsx,
  EnrichmentCard.tsx, PropertyEditor.tsx, PropertySettings.tsx,
  PipelineSettings.tsx, ListManager.tsx, CRMImportWizard.tsx,
  SequenceBuilder.tsx, SequenceDetail.tsx, SequenceEnrollments.tsx,
  CreateContactDialog.tsx, CreateCompanyDialog.tsx, CreateDealDialog.tsx,
  CRMSearchResults.tsx

frontend/src/pages/crm/
  Contacts.tsx, ContactDetail.tsx, Companies.tsx, CompanyDetail.tsx,
  Deals.tsx, DealDetail.tsx, Lists.tsx, Sequences.tsx,
  SequenceDetail.tsx, Insights.tsx

frontend/src/routes/_authenticated/w/$slug/crm/
  index.tsx
  contacts/index.tsx, contacts/$contactId.tsx
  companies/index.tsx, companies/$companyId.tsx
  deals/index.tsx, deals/$dealId.tsx
  lists/index.tsx
  sequences/index.tsx, sequences/$sequenceId.tsx
  insights.tsx
```

### Modified Existing Files

| File | Changes |
|------|---------|
| `server/internal/authorization/permissions.go` | Added PermCRMRead, PermCRMEdit, PermCRMAdmin |
| `server/internal/authorization/rbac.go` | Added CRM perms to role matrix |
| `server/internal/router/router.go` | Added 16 handler fields + ~50 CRM routes |
| `server/cmd/api/main.go` | AutoMigrate + DI wiring for all CRM layers |
| `frontend/src/lib/queryKeys.ts` | Added CRM query key factory |
| `frontend/src/hooks/queries/index.ts` | Export useCRM hooks |
| `frontend/src/components/layout/Sidebar.tsx` | CRM rail + 6 nav items |
| `frontend/src/stores/globalCreateStore.ts` | Added crm_contact, crm_company, crm_deal modal types |
| `frontend/src/routeTree.gen.ts` | Auto-generated CRM routes |

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Custom properties | Hybrid JSONB + definitions table | No EAV JOIN overhead, GIN index for filtering, HubSpot-compatible |
| Associations | Polymorphic table | Flexible any-to-any linking, supports cross-module associations |
| Display IDs | Sequential per workspace (CON-1, COM-1, DEAL-1) | Human-readable, follows PM pattern |
| Email tokens | Encrypted at rest | OAuth tokens are sensitive credentials |
| AI analysis | Durable Temporal extraction plus deterministic evaluators | Keeps ingestion asynchronous and evidence auditable |
| Sequences | Designed for Temporal workflows | Exactly-once guarantees, timer support |
| CRM permissions | Flat RBAC (crm.read/edit/admin) | Follows existing pattern, no per-object ACL needed |

## Further work

This overview intentionally does not maintain a speculative backlog. Use dated
documents under `docs/plans` for proposals and verify implementation status in
the linked canonical feature references before relying on a plan.
