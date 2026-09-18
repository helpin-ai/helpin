# Email architecture

This guide explains how Helpin sends and receives email: application mail,
support replies, and support route ingestion. Use it when configuring mail or
changing email code. The hostnames and addresses below are Helpin Cloud's;
self-hosted installations substitute their own verified domains.

## Overview

Helpin now treats email as three separate concerns:

1. App / product email
2. Support reply email
3. Support inbound route ingestion

There is also a fourth email-related surface that is intentionally separate:

4. CRM-connected Gmail / calendar accounts

CRM-connected mailboxes are not sent through Postmark. They use the Gmail OAuth
and sync stack under `server/internal/oauth` and `server/internal/sync`.

## Domains

- App / product email domain: `helpin.ai`
- Support reply domain: `replies.helpin.email`
- Support route base domain: `on.helpin.email`

Examples:

- Product invite: `notifications@helpin.ai`
- Support reply sender: `support@helpin.email`
- Support reply alias: `conv-<conversation-id>@replies.helpin.email`
- Support unsubscribe alias: `unsubscribe-<conversation-id>@replies.helpin.email`
- Shared support route alias: `inbox@<workspace-slug>.on.helpin.email`
- Mailbox route alias: `<mailbox-handle>@<workspace-slug>.on.helpin.email`

Notes:

- The branded route namespace currently comes from `workspaces.slug`.
- The shared inbox local part is reserved as `inbox`.
- Existing legacy `route-...@...` aliases can still resolve if they already
  exist in the database.

## Postmark Servers

Use three Postmark servers. Community installations may deliver application
mail through `SMTP_HOST`/`SMTP_FROM` instead of the Postmark app server; support
reply and route ingestion still require Postmark inbound webhooks.

### 1. App Mail

Purpose:

- invites
- password reset / auth mail
- notifications
- other product/system email

Recommended sender examples:

- `notifications@helpin.ai`
- `support@helpin.ai`

### 2. Support Replies

Purpose:

- outbound support reply emails
- support transcript emails
- inbound conversation reply processing
- open tracking for outbound support reply email

Recommended sender examples:

- `support@helpin.email`
- `messages@helpin.email`

Inbound domain:

- `replies.helpin.email`

### 3. Support Routes

Purpose:

- receives forwarded email that should create or route support conversations

Inbound domain:

- `*.on.helpin.email`

Notes:

- The route server token is stored in config for ops clarity and future use.
- The current backend does not send mail through the route server.
- Route ingestion currently relies on the shared inbound webhook endpoint.

## Environment Variables

These are the canonical variables going forward.

```env
# App / product email
POSTMARK_APP_SERVER_TOKEN=
POSTMARK_APP_FROM_EMAIL=notifications@helpin.ai

# Support replies
POSTMARK_REPLY_SERVER_TOKEN=
POSTMARK_REPLY_FROM_EMAIL=support@helpin.email
POSTMARK_REPLY_INBOUND_WEBHOOK_SECRET=
SUPPORT_EMAIL_REPLY_DOMAIN=replies.helpin.email

# Support forwarding routes
POSTMARK_ROUTE_SERVER_TOKEN=
POSTMARK_ROUTE_INBOUND_WEBHOOK_SECRET=
SUPPORT_EMAIL_ROUTE_DOMAIN=on.helpin.email
```

Legacy compatibility is still supported:

```env
POSTMARK_SERVER_TOKEN=
POSTMARK_FROM_EMAIL=
POSTMARK_INBOUND_WEBHOOK_SECRET=
```

Fallback behavior:

- `POSTMARK_APP_SERVER_TOKEN` falls back to `POSTMARK_SERVER_TOKEN`
- `POSTMARK_APP_FROM_EMAIL` falls back to `POSTMARK_FROM_EMAIL`
- `POSTMARK_REPLY_SERVER_TOKEN` falls back to `POSTMARK_SERVER_TOKEN`
- `POSTMARK_REPLY_FROM_EMAIL` falls back to `POSTMARK_FROM_EMAIL`
- `POSTMARK_REPLY_INBOUND_WEBHOOK_SECRET` falls back to `POSTMARK_INBOUND_WEBHOOK_SECRET`
- `POSTMARK_ROUTE_INBOUND_WEBHOOK_SECRET` falls back to `POSTMARK_INBOUND_WEBHOOK_SECRET`
- `SUPPORT_EMAIL_ROUTE_DOMAIN` falls back to `SUPPORT_EMAIL_REPLY_DOMAIN`

This keeps older environments working during rollout.

## Wiring In The API

Main bootstrap:

- config load: `server/internal/config/config.go`
- email client wiring: `server/cmd/api/main.go`

Runtime client split:

- app email client uses:
  - `POSTMARK_APP_SERVER_TOKEN`
  - `POSTMARK_APP_FROM_EMAIL`
- support reply email client uses:
  - `POSTMARK_REPLY_SERVER_TOKEN`
  - `POSTMARK_REPLY_FROM_EMAIL`

The support route server token is currently configuration-only and not used for
outbound mail sends.

## Product Email Flows

These use the app email client:

- auth service
  - password reset / auth email flows
  - `server/internal/service/auth.go`
- invite service
  - workspace invitations
  - `server/internal/service/invite.go`
- notification service
  - notification email delivery
  - `server/internal/service/notification.go`
The former support coverage digest sender is no longer present or wired into the
API/worker. Its delivery model and repository records remain, but those records
alone do not implement scheduled weekly coverage emails. Notification digests are
a separate flow in the notification service.

## Support Email Flows

These use the support reply email client:

- email fallback service
  - delayed outbound support emails
  - inbound replies
  - open tracking
  - `server/internal/service/email_fallback.go`
- support transcript sending
  - currently sent through the fallback service's configured email client
  - `server/internal/service/support_inbox_widget.go`

### Reply aliases

Generated by `EmailFallbackService`:

- `conv-<conversation-id>@<reply-domain>`
- `unsubscribe-<conversation-id>@<reply-domain>`

Relevant code:

- `server/internal/service/email_fallback.go`

### Route aliases

Generated by `SupportInboxService.CreateEmailRoute`:

- `inbox@<workspace-slug>.<route-domain>` for the shared inbox
- `<mailbox-handle>@<workspace-slug>.<route-domain>` for mailbox-specific routes

Relevant code:

- `server/internal/service/support_email_route.go`

## Inbound Webhooks

Endpoint:

- `POST /api/webhooks/postmark/inbound`

Open tracking endpoint:

- `POST /api/webhooks/postmark/open`

Handler:

- `server/internal/handler/webhook_postmark.go`

The handler now accepts more than one valid webhook secret so the reply server
and route server can both call the same endpoint.

## Current Resolution Rules

Inbound reply / route processing happens in:

- `server/internal/service/email_fallback.go`

Current alias recognition:

- `conv-...`
- `unsubscribe-...`
- `route-...`

Current route resolution:

- reply-thread email
- unsubscribe email
- legacy `route-...` support email
- branded support email routes by full recipient address, for example
  `billing@acme.on.helpin.email`

Current limitation:

- workspace namespace is derived from `workspaces.slug`; there is no separate
  admin-managed email namespace yet
- renaming a mailbox updates its active branded route address
- existing forwarding rules outside Helpin still need to be updated manually if
  the destination address changes

## Database Records

Support email route metadata:

- model: `server/internal/model/support_inbox.go`
- table: `support_email_routes`

Support email logs:

- model: `server/internal/model/support_email_log.go`
- table: `support_email_logs`

Webhook audit events:

- model: `server/internal/model/support_email_webhook_event.go`
- table: `support_email_webhook_events`

## Operational Setup Checklist

### App Mail Server

- verify `helpin.ai` sending domain
- configure `POSTMARK_APP_SERVER_TOKEN`
- configure `POSTMARK_APP_FROM_EMAIL`

### Support Reply Server

- verify `helpin.email` sending domain
- set inbound domain to `replies.helpin.email`
- configure inbound webhook to `/api/webhooks/postmark/inbound`
- configure open webhook to `/api/webhooks/postmark/open`
- configure `POSTMARK_REPLY_SERVER_TOKEN`
- configure `POSTMARK_REPLY_FROM_EMAIL`
- configure `POSTMARK_REPLY_INBOUND_WEBHOOK_SECRET`
- configure `SUPPORT_EMAIL_REPLY_DOMAIN`

### Support Route Server

- set inbound domain to `*.on.helpin.email`
- configure inbound webhook to `/api/webhooks/postmark/inbound`
- configure `POSTMARK_ROUTE_SERVER_TOKEN`
- configure `POSTMARK_ROUTE_INBOUND_WEBHOOK_SECRET`
- configure `SUPPORT_EMAIL_ROUTE_DOMAIN`

## Known Follow-Ups

These are not part of the current implementation and remain planned work:

- custom workspace namespace editing instead of using workspace slug
- migration tooling for backfilling or regenerating existing legacy route
  addresses
- separate inbound endpoints per Postmark server if we want cleaner observability
