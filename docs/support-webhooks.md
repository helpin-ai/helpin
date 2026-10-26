# Support webhooks

This page is for developers who receive Helpin support events in their own
systems, and for contributors changing how events are sent. Workspace
admins manage webhooks in **Settings → Webhooks** (the `support.admin`
permission). The implementation lives in
`server/internal/service/webhook.go` and `webhook_delivery.go`; database
triggers are in `202610250007_support_webhooks.sql`.

## Events

| Event | Sent when |
| --- | --- |
| `conversation.created` | A customer or teammate starts a conversation. |
| `message.created` | A customer, teammate or the AI posts a public reply. Internal notes are never sent. |
| `conversation.status_changed` | The status changes: open, waiting on customer, resolved or spam. |
| `conversation.assigned` | The assignee changes or is removed. |
| `conversation.priority_changed` | The priority changes. |
| `conversation.snoozed` | A teammate snoozes the conversation. |
| `sla.breached` | A first response, next response or resolution target is missed. |
| `csat.submitted` | A customer rates a resolved conversation or changes their rating. |
| `ping` | Only for **Send test event**. |

An endpoint receives every event unless it lists the ones it wants. It can
also be limited to some inboxes.

Some things never produce events:
- imported history (anything created more than an hour before it is recorded);
- anonymized conversations;
- replies deleted within their undo window.

## Request

Each event is a `POST` with a JSON body:

```json
{
  "id": "4a0c…",
  "type": "conversation.status_changed",
  "created_at": "2026-10-26T09:12:03Z",
  "workspace_id": "…",
  "data": {
    "conversation": {
      "id": "…", "number": 128, "subject": "Invoice missing VAT",
      "status": "resolved", "priority": "high", "channel": "email",
      "inbox_id": null, "assignee_id": "…",
      "customer": { "name": "Ada", "email": "ada@example.com", "crm_contact_id": null },
      "tags": ["billing"], "snoozed_until": null, "sla": { … },
      "created_at": "…", "updated_at": "…", "resolved_at": "…",
      "url": "https://app.helpin.ai/w/acme/support/…"
    },
    "changes": { "status": { "from": "open", "to": "resolved" } }
  }
}
```

`data.conversation` is the conversation when the event is sent, which may be
moments after it happened. The rest of `data` depends on the event:

| Event | Extra fields |
| --- | --- |
| Status, assignee and priority changes | `changes` holds the values from and to. |
| `message.created` | `message`: `id`, `sender_type` (`customer`, `user`, `ai` or `agent`), `sender_user_id`, `sender_name`, `content` (Markdown) and `created_at`. |
| `conversation.snoozed` | `snoozed_until`. |
| `sla.breached` | `sla.metrics`, `sla.policy_id` and `sla.policy_name`. |
| `csat.submitted` | `csat.rating` (1 to 5), `comment`, `channel`, `resolved_by` (`ai` or `human`) and `updated`. |

Every request carries these headers:

| Header | Value |
| --- | --- |
| `Helpin-Event-Id` | The event ID; the same for redeliveries. |
| `Helpin-Event-Type` | The event type. |
| `Helpin-Delivery-Id` | This attempt's delivery ID. |
| `Helpin-Signature` | `t=<unix seconds>,v1=<hex HMAC-SHA256>` |

## Verify the signature

`v1` is the HMAC-SHA256 of `"<t>.<raw body>"`, keyed with the endpoint's
signing secret (`whsec_…`, shown once when the webhook is created or its
secret is rotated). Reject the request if:
- the signature does not match (compare in constant time);
- or `t` is more than five minutes from now.

```js
const [t, v1] = header.split(',').map((part) => part.split('=')[1])
const expected = crypto.createHmac('sha256', secret).update(`${t}.${rawBody}`).digest('hex')
const valid = v1?.length === expected.length
  && crypto.timingSafeEqual(Buffer.from(v1), Buffer.from(expected))
  && Math.abs(Date.now() / 1000 - Number(t)) < 300
```

Answer with any 2xx status within 15 seconds. Redirects count as failures.
Process events idempotently by `Helpin-Event-Id`: a retry or a manual resend
carries the same ID.

## Retries and failures

- Each event is retried after 1 minute, 5 minutes, 30 minutes, 2 hours,
  6 hours and 15 hours, then marked failed: about a day in total.
- An endpoint that answers `410 Gone` is turned off at once.
- An endpoint is also turned off after 20 failed attempts in a row with no
  success for a day.
- Turning an endpoint off fails its pending deliveries. Turning it back on
  clears its failure count; resend events from the delivery log if needed.
- The delivery log keeps 30 days.

Conversation and message events are written to an outbox by database
triggers, in the same transaction as the change, so every code path produces
them. SLA breaches and ratings are written by the server right after it
records them. A worker turns them into deliveries every few
seconds and sends them with a lease, so a restart loses nothing.

## Network rules

Webhook URLs must use HTTPS and resolve to public addresses. Each connection
checks the resolved address again, which stops DNS rebinding. Self-hosted
installations that deliver to their own network can set
`WEBHOOKS_ALLOW_PRIVATE_NETWORKS=true`, which also allows plain HTTP.

Signing secrets are encrypted with `WEBHOOK_ENCRYPTION_KEY`, or with
`CRM_ENCRYPTION_KEY` when that is unset. Without either key, webhooks cannot
be created.
