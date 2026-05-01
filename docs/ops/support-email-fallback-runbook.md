# Support Email Fallback Operations Runbook

This runbook covers day-1 operations for support inbox email fallback: when a
visitor is offline, an agent reply should be delivered by email and customer
replies should thread back into the support conversation.

## Normal Flow

1. Agent sends a public reply in the support inbox.
2. If the visitor is offline and the conversation is eligible, the message is
   queued in Redis with a short cancellation delay.
3. The email fallback poller sends the message through Postmark.
4. Outbound email uses:
   - Preferred `From`: workspace mailbox sender, for example
     `inbox@contentpen.on.helpin.email`
   - Fallback `From`: verified Postmark sender, for example
     `support@helpin.email`
   - `Reply-To`: conversation route, for example
     `conv-{conversation_id}@replies.helpin.email`
5. Postmark delivery/open/bounce webhooks update `support_email_logs` and
   message receipt fields.

## Sender Verification Behavior

Postmark requires the actual `From` address/domain to be verified as a Sender
Signature or Domain Signature. A configured server token is not enough.

Current production-safe behavior:

1. Helpin tries the preferred workspace sender.
2. If Postmark returns a sender-signature `422`, Helpin retries once with the
   verified fallback sender.
3. The `Reply-To` remains conversation-specific, so inbound replies still route
   into the original conversation.

This keeps emails moving while we build the longer-term branded sender flow.

Preferred future sender pattern:

- Verify `inbox.helpin.email` in Postmark.
- Send workspace-branded emails as `<workspace-slug>@inbox.helpin.email`.
- Keep `POSTMARK_REPLY_FROM_EMAIL` as a verified fallback such as
  `support@helpin.email`.
- Keep `Reply-To` as `conv-{conversation_id}@replies.helpin.email`.

Avoid using per-workspace subdomains like
`inbox@contentpen.on.helpin.email` as the long-term outbound `From` unless each
subdomain is verified for outbound sending.

## First Checks

Use the current pod name from:

```bash
k get pods -n helpin | grep helpin-server
```

Check fallback worker startup:

```bash
k logs <server-pod> -n helpin --since=30m | grep -E "email fallback workers|email fallback poller|email fallback reconciler|Postmark support reply"
```

Check send/fallback errors:

```bash
k logs <server-pod> -n helpin --since=30m | grep -E "email fallback|postmark|sender signature|status 422"
```

Follow useful logs without request noise:

```bash
k logs -f <server-pod> -n helpin | grep -Ev "http request|ws|token refreshed"
```

## Expected Logs

Healthy startup:

```text
Postmark support reply email configured
email fallback workers starting
email fallback poller started
email fallback reconciler started
```

Branded sender rejected, then recovered:

```text
email fallback branded sender rejected, retrying with verified sender
email fallback sent with verified sender fallback
```

Unhealthy sender verification:

```text
email fallback send failed ... postmark API returned status 422 ... is not a Sender Signature
```

If the retry fallback is deployed, the 422 should be followed by a verified
sender retry. If 422s continue without retry success, confirm
`POSTMARK_REPLY_FROM_EMAIL` is verified in Postmark.

## Admin Diagnostics

Open the platform admin email diagnostics page and check:

- Reply Postmark is configured.
- Redis and poller are configured.
- Verified fallback From is populated.
- Queue is draining.
- Recent email logs show outbound `sent`/`delivered` rows.
- Conversation lookup explains why a specific message was queued, sent, or
  blocked.

## Recover Missed Emails

Use the dedicated recovery binary. Do not run `/app/server` for recovery; that
starts a second API server and will usually fail with `listen tcp :8080: bind:
address already in use`.

Dry-run first:

```bash
k exec -it <server-pod> -n helpin -- /app/recover-missed-support-emails \
  --from 2026-05-01T15:37:00Z \
  --to 2026-05-01T16:05:00Z \
  --limit 1000
```

Execute only after the dry-run counts look correct:

```bash
k exec -it <server-pod> -n helpin -- /app/recover-missed-support-emails \
  --from 2026-05-01T15:37:00Z \
  --to 2026-05-01T16:05:00Z \
  --limit 1000 \
  --execute
```

Adjust `--from` and `--to` to the confirmed outage window. Keep the window
narrow to avoid sending old replies unexpectedly.

Watch recovery:

```bash
k logs -f <server-pod> -n helpin | grep -E "email fallback backfill|email fallback sent|verified sender fallback|send failed|status 422"
```

## When To Run Backfill

Run backfill when:

- Postmark rejected outbound fallback emails.
- Redis/poller was down and messages missed the normal queue.
- A code bug prevented emails from being queued or sent.
- The periodic reconciler skipped older messages as stale.

Do not run broad backfills across multiple days without product approval. Old
support replies arriving late can confuse customers.

## Recovery Scope

Active Redis outbox entries should retry automatically after the fix deploys.

The periodic reconciler covers recent messages only. Older missed messages need
the explicit recovery command above, which bypasses the stale delivery window
for the operator-supplied time range.

## Post-Incident Checklist

1. Confirm new agent replies are producing outbound email logs.
2. Confirm no new `status 422` sender-signature errors.
3. Confirm `support_email_logs` has `sent` rows for recovered messages.
4. Confirm the admin queue is not stuck.
5. Send a reply to a recovered email and confirm it threads into the original
   conversation.
6. Record the outage window, command used, dry-run counts, execute counts, and
   any skipped messages in the incident notes.

## Useful SQL

Recent failed outbound logs:

```sql
select created_at, workspace_id, conversation_id, from_email, to_email, status, error_message
from support_email_logs
where direction = 'outbound'
  and status in ('failed', 'bounced', 'spam_complaint')
order by created_at desc
limit 50;
```

Recent outbound sends:

```sql
select created_at, workspace_id, conversation_id, from_email, to_email, status, postmark_message_id
from support_email_logs
where direction = 'outbound'
order by created_at desc
limit 50;
```

## Longer-Term Product Work

Build a managed sender-domain flow:

1. Verify `inbox.helpin.email` in Postmark.
2. Change preferred outbound sender to `<workspace-slug>@inbox.helpin.email`.
3. Keep verified fallback sender.
4. Later add customer-owned domain verification with DKIM/TXT/CNAME records,
   Postmark account/domain API integration, and verification status in settings.
