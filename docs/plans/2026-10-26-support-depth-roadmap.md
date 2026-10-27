# Support depth roadmap: company portal first

Date: 2026-10-26. Follows `2026-10-25-support-operations-plan.md`.

An external review (2026-10-26) rated Helpin credible for a small B2B team on
chat, email and portal, and behind Intercom and Zendesk in five areas: SLA
follow-through, B2B portal collaboration, workload routing, measured AI quality,
and migration fidelity. This roadmap sequences those gaps. We ship one phase at
a time. Each phase gets its own detailed plan section before work starts, and
each leaves the product working if later phases never land.

The 2026-10-25 plan deferred "company-wide request visibility in the portal"
until a B2B customer needed it. This roadmap reverses that deferral on purpose:
company accounts are the main reason a B2B buyer picks a portal over email.

## Phases

| # | Phase | Why this position | Status |
|---|---|---|---|
| 1 | Company portal and basic customer roles | Other B2B work depends on it: participants, company-tier SLAs, per-company reporting | **Built** on `feat/company-portal`; see [Customer portal access](../support-portal-access.md) |
| 1b | Human-owned portal tickets and article suggestions at intake | Puts AI before the ticket and people after it; measures deflection | **Built** on `feat/company-portal`; see [the plan](2026-10-27-portal-intake-deflection-plan.md) |
| 1c | Private help spaces for signed-in portal customers; search by meaning in the help center search dialog | Lets teams share guides that must not be public | **Built** on `feat/company-portal`; see [the plan](2026-10-27-portal-private-help-spaces-plan.md) |
| 1d | Help assistant (Ask AI) for the help center and portal | Conversational answers from the docs before a ticket | **Built** on `feat/company-portal`; see [the plan](2026-10-27-help-assistant-plan.md) and [the doc](../help-assistant.md) |
| 2 | Actionable SLAs | The reviewer's top priority. It also supplies the "next update by" commitment the portal shows customers | Outline |
| 3 | Portal collaboration | Builds on phase 1 company membership | Outline |
| 4 | Workload-aware assignment | Matters once a team has more than 3–4 agents | Outline |
| 5 | AI evaluation | Lets us show evidence of AI quality instead of claiming it | Outline |
| 6 | Migration fidelity | Unblocks "switch from Intercom" positioning | Outline |

## Phase 1: Company portal and basic customer roles

### How the competitors do it

- **Zendesk** has a setting at both the organization level and the user level. An organization can be set to "can view own tickets only" or "can view all org tickets", optionally "and add comments". Each user can be set to "own tickets only", "can view tickets from user's org", or "can view and edit tickets from user's org". When the two settings conflict, the more permissive one wins. Agents manage all of this; end users cannot manage each other. Users can belong to several organizations. Sources: [Creating organizations](https://support.zendesk.com/hc/en-us/articles/4408882246298-Creating-organizations), [org ticket sharing](https://www.eesel.ai/blog/zendesk-organization-ticket-sharing).
- **Intercom** splits portal rules into two parts:
  - An *access* audience: attribute rules decide who may use the portal at all.
  - A *visibility* toggle: if it is off, everyone sees all of their company's tickets. If it is on, attribute conditions restrict the matching users to their own tickets.

  There are no end-user roles. Users must belong to a company. Source: [Customer portal setup](https://www.intercom.com/help/en/articles/8489114-customer-portal-setup).

Neither product has a self-serve customer admin. Both use two levels of visibility, "own" and "company", set by agents. We copy that model. A customer-side admin who manages colleagues is a possible later addition, not part of phase 1.

### What exists today

- **Sign-in and identities.** Customers sign in with a magic link. Each sign-in creates a portal identity, keyed per email (`support_portal_identities`). Listing is scoped by identity (`repository/customer_portal.go:359`). Each conversation has one request reference (`UNIQUE (workspace_id, conversation_id)`). A colleague can therefore open the same `/requests/{reference}` URL once authorized, and no new reference scheme is needed.
- **Access control.**
  - Workspace level: portal on/off, and an access mode (`approved_contacts` or `any_verified_email`).
  - Contact level: `crm_contacts.portal_access` (allowed / blocked / null).
  - The rule is in `evaluatePortalEligibility` (`service/customer_portal_access.go:36`). A session stores its `CRMContactID` and is re-checked on every request.
- **Contacts and companies** are many-to-many through `crm_associations`. These associations are also created **automatically**: by the support inbox (`service/support_inbox.go:2891`), meetings, and email sync.
- **Conversations** already store `crm_company_id`. The company may have been inferred from the email domain (`CompanyMatchMethod`).
- **Admin screens.**
  - The company detail page has tabs (`frontend/src/lib/companyDetailTabs.ts`).
  - The contact detail page has a per-contact portal control (`ContactPortalAccessControl.tsx`).
  - The admin access API lives under `/inbox/portal/...` and requires `support.admin`.
- **Customer roles.** None exist yet.

### Decisions (as built)

1. **Company switch.** One nullable column, `crm_companies.portal_default_role`:
   - `NULL`: no company features. This is the default, so today's behavior is unchanged.
   - `requester`: contacts see only their own requests.
   - `member`: contacts see all of the company's requests.

   This mirrors Zendesk's organization setting and Intercom's toggle.
2. **Roles live on the contact, as in Zendesk.** `crm_contacts.portal_role` (`requester`, `member` or `NULL`) overrides the company default in either direction. `NULL` follows the company.
   - Unlike Zendesk, an override can also restrict a contact below the company default. Intercom allows this, and it fits contractors.
   - A per-company role table was considered and dropped. Keeping the role on the contact needs no new table and no cleanup when associations change.
   - When the company setting is `NULL`, no company features apply, whatever the overrides.
3. **Company membership is explicit (`support_portal_company_members`), never inferred from CRM links.** A support admin gives each contact company access on the Portal tab. Sign-in also needs an explicit **Allowed** decision, even in any-verified-email mode.

   Reason: associations are created automatically by email sync, the inbox, meetings and the Messenger. The Messenger accepts unverified identity claims by default. The first build trusted associations, and a branch review showed that a visitor could claim a company in the Messenger and read its requests. Membership became an explicit table.

   Bulk adding covers only linked contacts whose email is on the company's domain. It skips blocked contacts, contacts without an email, and shared emails.
4. **Which requests count as company requests.** A conversation is visible to member M of company C only if all of the following hold:
   - `conv.crm_company_id = C`;
   - the requester (`conv.crm_contact_id`) is a member of C and not blocked;
   - the conversation is published to the portal and not anonymized.

   Colleagues' email and chat conversations are published when a member lists company requests, under the same continuity rule as the colleague's own sign-in. New portal requests take the requester's portal company when there is exactly one, otherwise their single CRM company, as email does. A CRM company shares nothing unless the requester is a member there.
5. **Who wrote each message.** Portal replies store `portal_identity_id` in message metadata. Inbound email already stores `email_sender`. The portal marks each message `from_you`, and a colleague's reply carries their contact name in the inbox.
   - Read state and ratings stay the requester's.
   - Each time a member opens a colleague's request, it is audited, at most once per hour per member and request.
6. **Explicitly out of scope for phase 1:**
   - a customer-side admin role (planned later);
   - per-conversation "hide from company", beyond the existing hide-from-portal control;
   - automatic access when a contact is added to a company;
   - email notifications to colleagues (phase 3 participants).

### What was built

- **Server:**
  - migration `202610260004_support_portal_company_access.sql`;
  - repository `customer_portal_company.go`;
  - service `customer_portal_company.go`, with company-aware lookup in request detail, reply, rating and attachments;
  - session and exchange responses list companies;
  - `scope=company` on request lists;
  - admin routes for company access, contact role and bulk allow;
  - the live-update grant includes company requests;
  - a conversation portal-context route for teammates (originally portal-audience).
- **App:**
  - a **Portal** tab on the company page, for support admins;
  - a role selector on the contact rail;
  - a sidebar note on conversations that colleagues can read.
- **Portal:**
  - **My requests** and per-company tabs, remembered per browser tab;
  - "Asked by" on company rows;
  - colleague attribution on messages;
  - "Asked by" on a colleague's request page.

### Verification
- Go unit tests (SQLite) and a Postgres test for the migration.
- Vitest for the Portal tab and for the portal scope tabs.
- Manual check:
  1. Sign in as two contacts of the same company with different roles.
  2. Confirm what each one can see.
  3. Remove one from the company.
  4. Confirm their next request is denied and their live updates stop.
- Docs: [Customer portal access](../support-portal-access.md) describes the company model and its trust rules.

### Risks
- **Leaking a ticket to a colleague is the main risk.** Mitigations:
  - visibility is off by default;
  - access is granted only by explicit admin action;
  - a request must have a known contact as its requester;
  - authorization is re-checked on every request and every websocket subscription;
  - views are recorded in the audit log.
- **Customer expectations about privacy.** Customers who wrote before visibility was turned on did not expect colleagues to read their requests. The Portal tab states how many of the company's requests members would see, including while company access is off.

## Phase 2: Actionable SLAs (outline)
- Show metric definitions in reports, explaining SLA clock vs. first-human-response (`support_reports.go:180`, `support-reports.md:72`).
- Drill down from missed-SLA counts to the affected conversations.
- Pre-breach actions with a lead time: notify, escalate, or reassign.
- Match SLAs on company attributes (tier, custom fields), which is useful now that phase 1 gives companies a portal role.
- Show "next update by" to the customer in the portal request detail.
- Cut over imported open conversations: start new obligations from the cutover time and never judge history retroactively (`support_sla_evaluate.go:24`).

## Phase 3: Portal collaboration (outline)
- Participants and CC, managed from the portal; colleagues receive emails.
- Request types with configurable intake fields.
- Full-history search, across all messages rather than only the latest preview.
- Possibly a customer-side company admin, if a design partner asks for one.

## Phase 4: Workload-aware assignment (outline)
- Per-agent open-conversation limits and balanced assignment that respects them.
- Overflow to a fallback inbox. Skills-based routing later.

## Phase 5: AI evaluation (outline)
- A reusable evaluation set: correct answer, appropriate handoff, action success, latency.
- Track "assumed resolved" conversations that reopen.

## Phase 6: Migration fidelity (outline)
- Import attachments, assignee mapping and ratings.
- A cutover reconciliation report.
