# Support Email Forwarding Design

## Summary

Add first-class inbound support email for Shared Inbox and Team Inboxes using forwarding-based email intake.

This feature should not route everything through one generic shared intake first. Instead, each inbox can have its own generated Helpin forwarding address, and inbound email should land directly in the mapped inbox.

The right v1 model is:

- forwarding-based intake, because it is provider-agnostic and fastest to ship
- inbox-specific forwarding addresses, because mailbox routing should happen before the conversation is created
- reuse the existing Postmark inbound webhook and email reply infrastructure where possible
- keep room for future direct account connections without forcing them into v1

This is intentionally closer to the operator experience of Crisp / Intercom forwarding, but adapted to our Team Inbox architecture so Billing, VIP, and similar inboxes can receive email directly without a shared-ingress bottleneck.

## Why This Approach

We already have:

- Team Inboxes as the support visibility and routing model
- Postmark inbound webhook infrastructure
- reply-by-email support for existing conversations
- CRM email connectivity patterns for Gmail / Microsoft

We do not yet have:

- a true support email channel
- inbox-level inbound email routing
- a setup flow for support mailboxes to receive new email conversations

Building direct connected support accounts first would be heavier:

- provider-specific OAuth
- different shared mailbox semantics by provider
- Gmail / Microsoft only on day one
- weaker coverage for Zoho, Fastmail, cPanel, and other providers

Forwarding-based intake is the better v1 because it:

- works with nearly every provider that supports forwarding
- avoids blocking on provider-specific account connection work
- can map directly to Team Inboxes
- ships faster while still fitting our long-term architecture

## Problem

Teams want to receive support email directly inside Helpin, not just widget conversations.

The current support system can already send email replies and process replies to existing Helpin conversations, but it is not yet a true inbound email inbox product. There is no proper admin flow for:

- enabling email on Shared Inbox or Team Inboxes
- assigning forwarding addresses to inboxes
- routing new inbound email into the correct inbox
- exposing email-channel state in the support UI

If we only rely on a single shared forwarding address, we lose the value of Team Inboxes at the point of ingress. That creates unnecessary manual re-triage and weakens ownership boundaries.

## Goals

- Let Shared Inbox optionally receive inbound email.
- Let each Team Inbox optionally receive inbound email through its own forwarding address.
- Create new support conversations directly in the mapped inbox.
- Preserve email replies for existing conversations with correct threading.
- Make setup simple for admins and provider-agnostic.
- Surface the feature with clear, modern UX in Support settings and inbox management.
- Reuse existing support concepts: unread, assignment, notifications, AI handoff, permissions, and mailbox routing.

## Non-Goals

- Direct Gmail / Microsoft / IMAP support for Support in v1.
- Per-inbox custom sending domains in v1.
- Full shared mailbox provider sync in v1.
- Exchange / Office shared mailbox delegation support in v1.
- Full-blown deliverability tooling beyond what is necessary for safe rollout.
- Advanced spam scoring dashboards.

## Primary Users

- Workspace owners and support admins who configure email intake.
- Support leads who want email addresses mapped to the correct Team Inbox.
- Agents who need email-origin conversations to behave like normal support conversations.

## Current Fit

### What we already have

Support:

- workspace-level support conversations
- Shared Inbox plus Team Inbox model
- mailbox routing and ACLs
- outbound fallback email and inbound reply handling for existing conversations
- Postmark inbound webhook processing

CRM:

- Gmail / Microsoft account connectivity patterns
- encrypted token storage
- sync workflow patterns
- email message and thread handling patterns

### What we do not have

- support inbox email routes
- a concept of "this inbox receives email at this generated address"
- new-conversation creation from forwarded inbound email
- inbox-aware email setup UX
- email routing state visible in Team Inbox management

### Product implication

The CRM email stack is useful as future infrastructure, but it is not the right product model for support v1. CRM email accounts are member-centric. Support email intake needs to be workspace / inbox-centric.

## Product Principles

### 1. Inbox first, not shared-ingress first

An inbox should be the routing unit for inbound email.

That means:

- Shared Inbox can have a forwarding address
- Billing inbox can have its own forwarding address
- VIP inbox can have its own forwarding address

An admin should be able to forward `billing@company.com` directly into the Billing inbox address without first landing in Shared Inbox.

### 2. One forwarding address per route

Forwarding is the integration method, not the support abstraction.

The support abstraction is an Email Route:

- generated Helpin address
- mapped target inbox
- active / inactive
- optional display label

### 3. Support UX should feel native

Email-origin conversations should behave like any other support conversation:

- appear in the mapped inbox
- respect Team Inbox ACLs
- support assignment
- drive unread counts
- appear in notifications
- support moving between inboxes

### 4. Keep a clean upgrade path

The data model should leave room for direct connected support accounts later, but forwarding-based routes should be first-class rather than temporary hacks.

## Core Concepts

### Widget

- customer-facing chat entry point
- unrelated to support email route setup
- remains workspace-level

### Shared Inbox

- implicit workspace-wide inbox
- may optionally receive inbound email through one or more routes

### Team Inbox

- private internal support inbox
- may optionally receive inbound email through one or more routes

### Email Route

- generated forwarding address owned by Helpin
- maps inbound email directly to one inbox
- can target:
  - Shared Inbox
  - one Team Inbox

### Conversation Reply Address

- per-conversation reply address used for ongoing email threading
- distinct from inbox-level Email Routes
- already partially exists in current reply-by-email infrastructure

## Product Decisions

### Routing model

- new inbound email to an Email Route creates or matches a support conversation in that route's target inbox
- per-conversation reply aliases remain the mechanism for ongoing Helpin-thread replies
- inbox-level routes are for new inbound conversations
- conversation-level aliases are for continuing a known Helpin thread

### Shared Inbox handling

- Shared Inbox may have its own email route
- email forwarding is not forced through Shared Inbox
- Team Inboxes can have their own routes directly

### Multiple routes

Support should allow multiple routes per workspace and, if needed, multiple routes per inbox later.

V1 should support:

- one active route per inbox
- one route for Shared Inbox
- one route per Team Inbox

This keeps UI simple while leaving room for multiple aliases later.

### Outbound sending

V1 outbound email should be sent from Helpin-managed reply addresses, with:

- route or workspace display name
- correct reply threading headers
- consistent conversation linking

Custom sending domains and "send as support@company.com" are future work.

### Permissions

- owner and support admin can configure all email routes
- support members cannot manage routes unless they already have settings/admin permissions
- conversation visibility remains governed by inbox ACLs, not by who configured the route

## User Stories

- As a support admin, I can enable inbound email for Billing and get a unique forwarding address.
- As a support admin, I can forward `billing@company.com` to the Billing forwarding address.
- As an agent in Billing, I can see email-origin conversations directly inside Billing inbox.
- As an owner, I can see all inboxes and all email routes.
- As an admin, I can disable a route without deleting its history.
- As a customer, when I reply to a Helpin email, my reply continues the same conversation thread.

## Functional Scope

### V1

- Enable inbound email for Shared Inbox
- Enable inbound email for Team Inboxes
- Generate forwarding address per inbox route
- Copy forwarding address from UI
- Create conversations from new inbound email
- Append inbound email replies to existing conversations when threadable
- Show email-origin conversations in the mapped inbox
- Support attachments for inbound email where supported by Postmark inbound
- Show source channel / recipient metadata in thread detail
- Maintain mailbox permissions
- Maintain unread counts, notifications, assignment, and AI compatibility
- Add route status management:
  - active
  - disabled

### Not in V1

- OAuth-connected support inbox accounts
- custom domain sending
- sending from the customer's own mailbox provider
- inbox-specific email signatures
- multiple simultaneous routes per inbox
- advanced spam quarantine UI

## User Experience

### Navigation and entry point

Do not create a new separate product area for email. Keep setup inside Support settings and Team Inbox management.

Recommended information architecture:

- Support page remains the operational area
- Support settings gains an `Email Inboxes` section
- Team Inbox edit / create modal can surface email enablement after inbox creation

### Settings page overview

Admins should see one clear screen for support email routing.

ASCII:

```text
+----------------------------------------------------------------------------------+
| Support Settings / Email Inboxes                                                 |
+----------------------------------------------------------------------------------+
| Email lets customers write to your normal support addresses and have those       |
| conversations appear directly in Helpin inboxes.                                 |
|                                                                                  |
| [ Enable Shared Inbox Email ]                                                    |
|                                                                                  |
| Shared Inbox                                                                     |
| Status: Enabled                                                                  |
| Forward this address from your provider:                                         |
| [ ws-acme-shared@inbound.helpin.ai__________________________ ] [ Copy ]          |
| Last inbound: 2 minutes ago                                                      |
| [ Disable ]                                                                      |
|                                                                                  |
| Team Inbox Routes                                                                |
| -------------------------------------------------------------------------------- |
| Billing                           Enabled                                        |
| billing@company.com -> billing@inbound.helpin.ai                                |
| [ Copy address ] [ Disable ]                                                     |
|                                                                                  |
| VIP                               Not enabled                                    |
| [ Enable Email ]                                                                 |
|                                                                                  |
| Technical Support                 Enabled                                        |
| support-engineering@company.com -> techsupport@inbound.helpin.ai                |
| [ Copy address ] [ Disable ]                                                     |
+----------------------------------------------------------------------------------+
```

Behavior:

- Shared Inbox route is optional
- Team Inbox routes are managed row-by-row
- status and last inbound activity should be visible
- route actions should stay inline and low-friction

### Enable email for an inbox

The setup flow should be fast and concrete.

ASCII:

```text
+------------------------------------------------------------------------+
| Enable Email for Billing Inbox                                    [x]  |
+------------------------------------------------------------------------+
| Customers can email this inbox by forwarding your real mailbox to the  |
| Helpin address below.                                                   |
|                                                                         |
| Inbox                                                                   |
| Billing                                                                 |
|                                                                         |
| Helpin forwarding address                                               |
| [ billing.ws-84f3@inbound.helpin.ai________________________ ] [ Copy ]  |
|                                                                         |
| Your public-facing address                                              |
| [ billing@company.com________________________________________ ]         |
| Optional. Shown internally as setup context.                            |
|                                                                         |
| Setup                                                                   |
| 1. In your email provider, open forwarding settings                     |
| 2. Forward billing@company.com to the Helpin address above              |
| 3. Send a test email                                                    |
|                                                                         |
| Test status                                                             |
| [ Waiting for first email ]                                             |
|                                                                         |
|                                              [ Cancel ] [ Enable ]      |
+------------------------------------------------------------------------+
```

Behavior:

- generated Helpin address is shown immediately
- admin can store the source address as a label for reference
- no need for a multi-step setup wizard
- status should remain understandable if no email has arrived yet

### Team Inbox management integration

The Team Inbox settings page should surface email status as part of inbox configuration.

ASCII:

```text
+----------------------------------------------------------------------------------+
| Team Inbox: Billing                                                              |
+----------------------------------------------------------------------------------+
| Members | Routing | Email                                                        |
|                                                                                  |
| Email                                                                          |
| Status: Enabled                                                                  |
| Forwarding address: [ billing.ws-84f3@inbound.helpin.ai________ ] [ Copy ]      |
| Source address: billing@company.com                                              |
| Last inbound: Today, 09:42                                                       |
|                                                                                  |
| [ Disable Email ]                                                                |
+----------------------------------------------------------------------------------+
```

This should feel like an extension of the inbox, not a separate system.

### Thread UI

Email-origin conversations need visible source metadata without clutter.

ASCII:

```text
+--------------------------------------------------------------------------+
| #1048  Refund request                                                    |
| Customer: Sarah Chen <sarah@acme.com>                                    |
| Source: Email                                                            |
| Inbox: Billing                                                           |
| Sent to: billing@company.com                                             |
|                                                                          |
| ------------------------------------------------------------------------ |
| Sarah Chen                                                               |
| Hello, I was charged twice for March.                                    |
|                                                                          |
| [ Reply ] [ Add note ] [ Move ]                                          |
+--------------------------------------------------------------------------+
```

Behavior:

- clearly show `Source: Email`
- show the original recipient / mapped address
- reply action should continue email threading automatically where applicable

### Empty states

Support email should have explicit empty states instead of hidden capability.

ASCII:

```text
+------------------------------------------------------------------+
| Email Inboxes                                                    |
+------------------------------------------------------------------+
| No inboxes have email enabled yet.                               |
|                                                                  |
| Enable email on Shared Inbox or a Team Inbox to receive support  |
| emails directly in Helpin.                                       |
|                                                                  |
| [ Enable Shared Inbox Email ]                                    |
| [ Open Team Inboxes ]                                            |
+------------------------------------------------------------------+
```

## Behavioral Rules

### New inbound email

When inbound email arrives to an inbox Email Route:

- resolve the route by recipient alias
- identify the target inbox
- attempt conversation match via threading headers
- if matched, append to the existing conversation
- if not matched, create a new conversation in the route's target inbox

### Thread matching

V1 matching order:

1. explicit Helpin conversation alias match
2. `In-Reply-To` or `References` match to known outbound / inbound support email logs
3. fallback route-based new conversation creation

Do not rely on fuzzy subject-only matching in v1.

### Reply handling

When an agent replies to an email-origin conversation:

- send email from Helpin-managed reply address
- include proper `Message-ID`, `In-Reply-To`, and `References`
- log outbound email metadata for future matching
- keep support thread and email thread aligned

### Mailbox permissions

- route assignment determines initial inbox placement
- inbox ACLs determine who can see the resulting conversation
- moving a conversation later should not change the original email log history

### Unread and notifications

- email-origin messages from customers count as unread exactly like widget customer replies
- notifications should only go to users eligible for that inbox
- Team Inbox unread badges should update for email-origin activity

### AI compatibility

- email-origin conversations can still enter AI flows where enabled
- AI handoff should respect the current inbox / mailbox routing rules
- v1 does not need separate AI settings by email route

## Data Model

### New table: support_email_routes

Suggested fields:

- `id`
- `workspace_id`
- `mailbox_id nullable`
  - `NULL` means Shared Inbox route
- `route_key`
  - stable internal generated key used in inbound alias
- `inbound_address`
  - generated Helpin address shown to admins
- `source_address nullable`
  - optional admin-entered reference like `billing@company.com`
- `display_name nullable`
  - human-facing label for outbound display if needed
- `provider_type`
  - `forwarding` in v1
- `active`
- `last_inbound_at nullable`
- `created_by_id`
- timestamps

Constraints:

- max one active route per inbox in v1
- max one active route for Shared Inbox in v1

### Extend support_email_logs

The existing log model should be extended where needed to support true inbox email routing:

- `email_route_id nullable`
- `recipient_address nullable`
- `in_reply_to nullable`
- `references_header nullable`
- `direction`
- `provider_message_id`
- `rfc_message_id`

This gives us better thread matching and diagnostics.

### Support conversation metadata

Consider adding the following to `support_conversations` or derived virtual fields:

- `source_channel = email`
- `email_route_id nullable`
- `email_recipient_address nullable`

If we want to avoid schema spread in v1, these can initially live in message metadata plus query joins, but the product will be cleaner if conversation-level email origin fields are explicit.

## Backend Design

### Route management service

Add support email route CRUD scoped to support settings:

- list routes by workspace
- create route for Shared Inbox
- create route for Team Inbox
- disable route
- regenerate route if compromised

### Inbound resolver

Update inbound email processing to distinguish:

- conversation reply aliases
- inbox Email Routes

Flow:

1. parse inbound webhook
2. inspect recipient / mailbox hash
3. if recipient is `conv-*`, process as existing conversation reply
4. else if recipient is `route-*`, resolve `support_email_routes`
5. match thread if possible
6. else create new support conversation in route target inbox

### Conversation creation

For new inbound email:

- `channel = email`
- `source = email`
- set `mailbox_id` from route
- set customer identity from sender
- create first support message from stripped text / html
- store attachments

### Attachments

Inbound attachments should be stored as normal support attachments when Postmark provides them.

### Spam / abuse controls

Minimum v1 controls:

- max message size
- max attachment count / size
- blocked senders list later if needed
- ignore duplicate provider message ids
- log malformed inbound payloads

### Diagnostics

Add admin visibility for:

- route status
- last inbound timestamp
- recent failures
- recent rejected / malformed inbound events

## API Design

Suggested endpoints:

- `GET /support/inbox/email-routes`
- `POST /support/inbox/email-routes`
- `PUT /support/inbox/email-routes/:id`
- `POST /support/inbox/email-routes/:id/disable`
- `POST /support/inbox/email-routes/:id/regenerate`

Webhook intake continues through Postmark inbound endpoints, but the service behind them becomes route-aware.

## Frontend Design

### New settings section

Add `Email Inboxes` under Support settings.

The screen should:

- explain the feature in one sentence
- show Shared Inbox email state
- show Team Inbox email state
- provide copyable forwarding addresses
- avoid burying route status in deep modal flows

### Team Inbox integration

Team Inbox row actions should include:

- `Enable Email`
- `Copy Forwarding Address`
- `Disable Email`

### Visual design guidance

- treat email routes as operational infrastructure, not as a form-heavy setup wizard
- show system-generated values clearly with copy actions
- use compact cards / rows with clear state badges
- prioritize "what do I copy?" and "where does this go?" in the UI

## Rollout

### Phase 1

- add route model
- enable Shared Inbox route
- enable Team Inbox route
- create new conversations from inbound route emails
- keep existing reply-by-email flow working

### Phase 2

- improve diagnostics
- add attachment polish
- add route regeneration
- add better admin activity logs

### Phase 3

- add custom sending domains
- add multiple routes per inbox
- add direct support account connections

## Risks

- forwarded emails may arrive in odd formats from some providers
- header quality may vary by forwarding source
- some providers may rewrite the envelope in ways that reduce thread quality
- outbound "from" identity will be less ideal until custom domains or direct accounts exist

These are acceptable for v1 given the speed and provider coverage benefits.

## Success Metrics

- number of workspaces enabling at least one support email route
- number of Team Inboxes with direct email routing enabled
- share of inbound support conversations created via email
- low rate of route misconfiguration or missing-email complaints
- low rate of duplicate-thread incidents

## Recommendation

Build support email as forwarding-based, inbox-specific routing now.

Do not:

- route all forwarded email through one shared intake
- overload CRM email accounts into support v1
- block launch on direct Gmail / Microsoft / IMAP support

Do:

- give each inbox its own forwarding address
- create conversations directly in the target inbox
- reuse current inbound / reply plumbing where possible
- design the route model so direct connected support accounts can arrive later without replacing the inbox abstraction
