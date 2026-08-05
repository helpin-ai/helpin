# Support Conversation Company Context Design

## Context

Helpin already accepts a `company` object through widget identity calls, upserts a CRM company, and associates that company with the CRM contact. However, a support conversation stores only `crm_contact_id`. The inbox therefore infers a company from the contact's current primary company and can show only a generic company association, not the account and subscription context that was active when the conversation began.

This is incorrect for the uncommon but important case where one person belongs to multiple customer accounts. Changing the contact's primary company can also change the inferred context of older conversations.

The SDK has a related gap. `group(company)` persists company data for analytics events, but it does not synchronize that company through the support identity path. `id(user)` sends a company only when it is nested directly in the user payload; it does not merge the separately persisted group into the backend identity payload.

## Product Decisions

- A support conversation has one explicit company context.
- The conversation's company identity remains fixed unless an agent explicitly changes it.
- The inbox reads the linked CRM company's current fields and subscription attributes. It does not store or display a frozen subscription snapshot.
- Most contacts belong to one company, but the data model and selection flow must safely support multiple memberships.
- Widget-provided active company context wins over contact-level inference.
- Helpin must not guess when a contact has multiple companies and the widget did not provide an active company.
- The existing sidebar order remains: routing, tags, email recipients, contact details, company details, other conversations, tasks, CRM, and docs.

## Goals

- Capture the company/account that is active when a widget conversation is created.
- Preserve that company identity on the conversation while showing current CRM company data.
- Make `group(company)` a reliable active-company synchronization path.
- Keep nested `company` payloads on `id(...)` and `lead(...)` backward compatible.
- Preserve many-to-many contact-company memberships without changing the primary company on every account switch.
- Present company and subscription context in the support inbox using the same compact visual language as contact details.
- Consolidate the current `User details` and `Contact Details` sidebar sections into one `Contact Details` section.

## Non-goals

- Capturing a historical snapshot of every company attribute on each conversation or message.
- Reintroducing CRM property-definition and property-group tables.
- Adding workspace-configurable sidebar layouts or draggable sections.
- Adding support-inbox company filters or reporting in this iteration.
- Automatically changing an existing conversation's company when a visitor switches accounts.
- Using email domain as the authoritative company identity when a stable external company ID is supplied.

## Industry Findings

Intercom distinguishes a user's current `company` from the complete `companies` membership list. It stores an explicit company on a conversation, automatically fills it only for single-company users, supports manual selection for multi-company users, and preserves old conversation links after membership changes. Intercom also displays live user and company data in the inbox sidebar. Its documented Messenger limitation is that the current company is not automatically captured for multi-company users; Helpin avoids that gap by turning widget-supplied active company data into the explicit conversation link.

References:

- https://developers.intercom.com/installing-intercom/web/attributes-objects
- https://www.intercom.com/help/en/articles/8838326-conversations-faqs
- https://www.intercom.com/help/en/articles/6433002-start-a-conversation-from-the-inbox
- https://www.intercom.com/help/en/articles/6988783-get-context-fast-with-user-and-company-profiles

Chatwoot's current company implementation provides structured company fields, JSONB custom attributes, and useful compact attribute presentation. However, it stores one `company_id` on the contact, infers companies from email domains, does not store a company on the conversation, and sends only a flattened company name through its widget identity path. Helpin should reuse the compact presentation ideas while retaining its stronger many-to-many CRM association model.

Reference commit: https://github.com/chatwoot/chatwoot/tree/ce0612158769e641b9e28c9a522aa597ef9d3584

## Chosen Architecture

Add a nullable, indexed `crm_company_id` to both `support_conversations` and `support_widget_sessions`.

- `support_widget_sessions.crm_company_id` is mutable active browsing context.
- `support_conversations.crm_company_id` is the stable company identity for that conversation.
- Company details are loaded from `crm_companies` when visitor context is requested, so plan, status, seats, renewal dates, and revenue remain current.
- No company JSON snapshot is added to the conversation.

The direct field is the canonical support-company relationship. Generic CRM associations may continue to expose other related records, but the active company must not be duplicated as a generic row in the sidebar.

### Existing-data migration

The schema change is additive and idempotent. Existing conversations may be backfilled only when their CRM contact has exactly one company association. Conversations for contacts with zero or multiple company associations remain unset. This avoids inventing historical context.

Existing sessions begin with no active company and acquire one through a later identity/group call.

If a linked company is deleted, conversation and session references are cleared rather than redirected to another company.

## Company Identity and Membership Semantics

The customer-provided `company.id` is the authoritative external account/tenant identifier. Matching remains workspace-scoped and uses this order:

1. external company ID;
2. normalized domain when no matching external ID exists;
3. exact normalized name as a final compatibility fallback.

Domain and name matching must never replace a company that was already resolved by external ID.

Sending an active company ensures a contact-company membership exists. It does not automatically make that company primary when another primary membership already exists. The first company membership may be marked primary; subsequent active companies are ordinary memberships unless an agent changes the primary relationship explicitly in CRM.

This requires splitting the current widget behavior that always calls `ensurePrimaryContactCompanyAssociationTx` into membership creation and primary-selection rules.

## SDK Contract

Customers may continue to send the company inline:

```ts
await helpin.id({
  id: user.id,
  email: user.email,
  first_name: user.firstName,
  last_name: user.lastName,
  company: {
    id: activeAccount.id,
    name: activeAccount.name,
    created_at: activeAccount.createdAt,
    domain: activeAccount.domain,
    plan: subscription.plan,
    subscription_status: subscription.status,
    support_tier: subscription.supportTier,
    seats_used: subscription.seatsUsed,
    seats_total: subscription.seatsTotal,
    renewal_date: subscription.renewalDate,
    mrr: subscription.mrr,
    currency: subscription.currency,
  },
});
```

The preferred account-switch flow is:

```ts
await helpin.group(activeCompany);
```

SDK behavior changes:

- `group(company)` persists the active company as it does today.
- When a stored identified user has an email, `group(company)` also sends the combined user and company identity to the existing support backend path. Calling `group` before `id` stores the company and defers support synchronization until identity is known.
- `id(user)` merges the persisted group when `user.company` is absent. An inline company takes precedence for that call and becomes the persisted active company.
- `lead(payload)` keeps accepting an inline company. It does not inherit an unrelated persisted customer group unless the lead payload explicitly includes it.
- Widget boot/automatic session upgrade includes the persisted active company so a page refresh does not lose account context.
- The `doNotSendEvent` argument continues to suppress the analytics event only; identity synchronization remains independent, matching current `id(...)` behavior.
- `reset()` continues clearing both user and company persistence.

Company custom attributes retain JSON scalar types. Omitted keys preserve existing CRM values. An explicit `null` removes a custom property. Nested `custom`, `custom_properties`, and `properties` remain accepted, and unknown top-level company keys continue to map into `custom_properties` for backward compatibility.

Recommended subscription keys are:

- `plan`
- `subscription_status`
- `support_tier`
- `seats_used`
- `seats_total`
- `trial_ends_at`
- `renewal_date`
- `mrr`
- `arr`
- `currency`

Customers should send the active company after sign-in, on account switch, and whenever subscription data changes. Payment credentials, card details, secrets, and other sensitive billing data must not be sent.

## Conversation Capture Rules

When a conversation is created from a widget session, it copies the session's current `crm_company_id`.

When identity/company data arrives after conversation creation:

- update the current session's active company;
- fill the current session's conversation company only when it is unset;
- never overwrite a different company already stored on the conversation;
- update current active sessions in the HTTP anonymous-ID fallback, but update only conversations referenced by those sessions and only when their company is unset;
- do not backfill unrelated historical conversations merely because they share an anonymous ID.

When a user switches accounts during an existing conversation, the session changes to the new active company but the conversation remains linked to the original company. The next newly created conversation uses the new session company.

## Agent Editing

Add a support-edit endpoint parallel to the existing CRM-contact endpoint:

```text
PUT /api/support/inbox/conversations/{id}/crm-company
{ "crm_company_id": "uuid-or-null" }
```

The service validates that the company belongs to the same workspace, updates the direct field, records an activity entry, and publishes the existing support-conversation updated event. Clearing the field is allowed.

The sidebar selector lists the contact's associated companies first. `Link another company` may use the existing CRM search flow; choosing it creates the contact-company membership and sets the conversation company. A manual conversation correction does not change the contact's primary company.

The direct company must participate in existing association consumers such as task-copying and support context, but the active company is rendered only once in the dedicated Company Details section.

## Visitor Context API

Extend the existing visitor-context response rather than adding another request:

```ts
interface VisitorCompanyData {
  id: string;
  display_id: string;
  external_id?: string;
  name: string;
  domain?: string;
  industry?: string;
  employee_count?: number;
  annual_revenue?: number;
  description?: string;
  logo_url?: string;
  custom_properties: Record<string, unknown>;
  updated_at: string;
}

interface VisitorCompanyOption {
  id: string;
  display_id: string;
  name: string;
  domain?: string;
  logo_url?: string;
}
```

`VisitorContextResponse` gains optional `company` and `company_options` fields. `company` is loaded strictly from the conversation's explicit `crm_company_id`. `company_options` contains the contact's current company memberships and always includes the selected company if it is still available.

Unlike the existing contact DTO, company custom properties remain `Record<string, unknown>` so numbers and booleans are not flattened into strings. Contact custom properties should also retain scalar types while the two current sidebar sections are consolidated.

Failure to load optional contact or company context must not prevent the conversation from rendering. Repository errors are logged with workspace, conversation, and entity IDs but never attribute values that may contain customer data.

## Sidebar Design

Preserve the existing order:

1. customer identity header;
2. Conversation Routing;
3. Tags;
4. Email recipients, when present;
5. Contact Details;
6. Company Details;
7. Other Conversations;
8. Tasks;
9. CRM;
10. Docs.

### Contact Details

Merge the current `User details` and `Contact Details` sections into one open-by-default `Contact Details` collapsible section. Retain the existing compact label/value rows.

CRM contact fields appear first: job title, lifecycle, lead status when useful, phone, and contact custom properties. A subtle `Current visit` subsection label then groups channel, current page, location, local time, language, device, and browser/OS. The customer identity header remains the primary display for name and email, so those values are not repeated unnecessarily.

### Company Details

Add an open-by-default `Company Details` collapsible section immediately after Contact Details. It uses the same spacing, typography, row component, badges, and background behavior rather than a separate card treatment.

The first rows show company name and domain. The company name links to the CRM company profile. For contacts with multiple memberships, a compact selector affordance appears on the company row. Read-only agents see the linked name without edit controls.

High-signal non-empty attributes are displayed in this order:

1. plan;
2. subscription status;
3. support tier;
4. seats used / seats total;
5. trial end or renewal date;
6. MRR or ARR.

Formatting rules:

- known subscription states use small semantic badges;
- seats render as `used / total` when both values exist;
- currency values use `Intl.NumberFormat` and the supplied `currency` code;
- dates use the workspace/browser locale, with relative timing available in a tooltip;
- booleans render as Yes or No;
- empty values are omitted;
- keys are humanized for display;
- `sdk_company_id`, `sdk_created_at`, and fields already represented above are hidden from the extra-attribute list.

Remaining custom values are collapsed behind `Show N more`. The footer shows company freshness using the CRM company's `updated_at` value.

States:

- exactly one membership and no explicit legacy link: the safe migration/backfill supplies it;
- several memberships with widget context: show the explicit widget-selected company;
- several memberships without context: show `Select company` and do not guess;
- no membership: show `Link a company`;
- missing/deleted company: show an unavailable state and allow an editor to select another company;
- loading: use a compact inline skeleton confined to the section;
- optional context error: show a retry action without hiding the conversation.

The active company is omitted from the generic CRM list to prevent duplication. Other companies, the CRM contact, deals, and manually linked CRM records remain there.

## Realtime and Cache Behavior

Company selection publishes a `support_conversation` update. The existing realtime invalidation path must refresh the conversation, visitor context, and associations queries. CRM company updates must invalidate or refresh visitor context for open conversations so subscription changes appear without a full page reload.

The UI resets section-specific expanded state and company selector state when the conversation ID changes.

## Compatibility

- Both new model fields are nullable.
- Existing widget identity payloads remain valid.
- Inline company payloads remain valid.
- Existing conversations with ambiguous company membership remain usable with no company selected.
- Visitor-context additions are optional and additive.
- Non-widget channels may acquire a company through the single-membership backfill/inference or manual selection; this work does not invent company context for ambiguous email/API conversations.
- Existing CRM `custom_properties` JSONB remains the extensibility mechanism; no property-definition subsystem is added.

## Testing

Backend coverage will include:

- additive schema and safe single-membership historical backfill;
- conversation creation copies the session company;
- session upgrade fills only an unset current conversation company;
- account switching updates session context without rewriting the conversation;
- anonymous-ID HTTP fallback updates only active session-linked, unset conversations;
- external ID matching wins over domain and name;
- a second active company creates a membership without replacing the existing primary;
- explicit null removes a company custom property while omission preserves it;
- visitor context returns the current linked company with typed custom properties and membership options;
- manual set, replace, clear, workspace validation, activity logging, permissions, and realtime publication;
- safe behavior when the linked company was deleted or is unavailable;
- association consumers prefer the direct conversation company and avoid duplicate summaries.

SDK coverage will include:

- `id` inherits a persisted group when inline company is absent;
- inline `id.company` takes precedence and becomes active;
- `group` after identification synchronizes company context;
- `group` before identification defers backend synchronization and is included by the later `id`;
- widget boot/session restoration includes persisted company context;
- `lead` does not accidentally inherit an unrelated group;
- `reset` clears active company data;
- WebSocket and HTTP identity paths serialize the same company payload.

Frontend coverage will include:

- preserved section ordering;
- merged Contact Details content and Current visit subsection;
- Company Details row ordering and formatting;
- one-company, multi-company, no-company, deleted-company, loading, and error states;
- read-only versus editable selector behavior;
- company selection invalidates conversation, visitor-context, and association queries;
- the active company is not duplicated in the CRM section;
- navigation to the CRM company profile;
- typed custom values and `Show N more` behavior.

Verification will run targeted Go service/repository/handler tests, SDK unit tests, frontend support component tests, TypeScript compilation, Go tests for affected packages, frontend production build, and `git diff --check`.

