# Customer portal access and company accounts

This page explains who can sign in to the customer portal and which requests each customer sees, including the company accounts added in October 2026. Support admins manage these settings; the rules are enforced by the API on every request.

## Who can sign in

A customer signs in with an emailed link. Access is decided per CRM contact, matched by email (`lower(trim(email))`):

- **Workspace setting.** **Settings → Customer portal** turns the portal on and chooses the access mode:
  - **Approved contacts** (the default) needs exactly one contact with that email whose portal access is **Allowed**.
  - **Any verified email** admits anyone who proves their address.
- **Contact decision.** A contact's portal access is **Allowed**, **Blocked**, or not set. A blocked contact denies the email in either mode.

A session is bound to the contact it signed in as. Eligibility is checked again on every request, so blocking a contact, changing its email, or deleting it ends its sessions on their next request.

## Personal and company requests

By default, the portal is personal: customers see the requests they asked for. A company can also share its requests with its people.

| Company setting | What its contacts see |
|---|---|
| Off (default) | Their own requests only |
| Own requests only | Their own requests. Contacts given the **Member** role also see the company's requests. |
| All company requests | Every request from the company. Contacts given the **Requester** role see only their own. |

A contact's role overrides its companies' default in either direction. Members can open, follow, and reply to colleagues' requests. Only the requester can rate a request.

Admins manage this on the company's **Portal** tab:

- turn company access on and choose the default;
- allow or block each contact, and set each contact's role;
- allow every eligible contact at once.

The contact page shows the same access and role.

### What is shared

Company visibility is deliberately conservative:

- **Sign-in is never inferred.** A member needs an explicit **Allowed** decision, even in any-verified-email mode. CRM associations are created automatically (email sync, the support inbox, meetings), so being associated with a company never grants access by itself. **Allow all contacts** skips contacts that are blocked, that have no email, or whose email another contact also uses.
- **A request belongs to a company only through a known requester.** The request's conversation must name the company, and its requester must be a contact associated with that company who is not blocked. A conversation linked to a company by email domain alone is not shared.
- **Hidden and erased conversations stay private.** Conversations hidden from the portal by a teammate, marked as spam, or anonymized for privacy are never shared.
- **Colleagues' conversations are published on demand.** When a member opens their company's requests, published email and chat conversations from colleagues who have never signed in also appear. They must pass the same continuity checks as the colleague's own first sign-in.
- **New portal requests take the requester's company** when the requester belongs to exactly one company, as new email conversations do. Otherwise a teammate sets the company in the conversation's details.

### Attribution and audit

- Portal replies record the portal identity that wrote them.
  - The portal shows "You" or the colleague's name on each message.
  - In the inbox, a colleague's reply carries their contact name, or their email if they have no name.
- Read state stays the requester's. A colleague opening a request does not mark it read, and company lists show no unread marks.
- Each time a member opens a colleague's request, the portal audit log records a `company_request_viewed` event. It records at most one per member and request each hour.
- The conversation sidebar tells teammates how many colleagues besides the requester can read the conversation in the portal.

## Limitations

- Colleagues receive no email notifications for each other's requests. They follow them in the portal, where live updates cover company requests. Participants and CC managed from the portal are planned separately.
- Customers cannot manage their colleagues' access. Support admins do.
- A request has one company. Contacts of several companies see each company's requests on its own tab.

## API

All routes require the `support.admin` permission unless noted otherwise:

- `GET /api/support/inbox/portal/companies/{companyId}`: the company's default role, its contacts with access, role, shared-email count and last sign-in, and how many of its requests members could see.
- `PUT /api/support/inbox/portal/companies/{companyId}` with `{"default_role": "requester" | "member" | null}`.
- `POST /api/support/inbox/portal/companies/{companyId}/allow-contacts`: allows eligible contacts and returns how many changed.
- `PUT /api/support/inbox/portal/contacts/{contactId}/role` with `{"portal_role": "requester" | "member" | null}`.
- `GET /api/support/inbox/conversations/{id}/portal-audience` (requires `support.read` and mailbox access): the company and the number of colleagues who can read the conversation.

The portal API adds `companies` to the session response. It also adds `scope=company` (with an optional `company_id`) to `GET /api/public/portal/{slug}/requests`.

The schema is in `server/internal/dbmigrate/sql/202610260004_support_portal_company_access.sql`. It adds `crm_companies.portal_default_role` and `crm_contacts.portal_role`.
