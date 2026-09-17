# Custom support sender addresses MVP

## Summary

Helpin should let support teams configure usable sender addresses such as `support@company.com` and `billing@company.com`, map them to inboxes, verify that inbound forwarding works, and send replies from those addresses safely.

This is intentionally smaller than a full Intercom/Crisp email parity project. The MVP is two phases:

1. **Phase 1: Sender addresses and domain authentication**
2. **Phase 2: Forwarding verification and inbound-recipient sender resolution**

Branded reply domains with MX records, automatic DNS setup, custom SMTP, and Crisp-style full custom reply domains are out of scope for this PRD.

## Why This Scope

The customer value is not “custom email infrastructure.” The value is:

- Customers see replies from `support@company.com`.
- Support teams can route `support@company.com` and `billing@company.com` into the right inboxes.
- Replies still land in Helpin.
- Unsafe sender states are blocked before they cause silent delivery failures.

This should be a focused support-email capability, not a four-month deliverability platform project.

## Current Behavior and Compatibility

Existing email forwarding inboxes should not have a problem with this change.

Current forwarding routes such as:

```text
inbox@workspace.on.helpin.email
route-xxxx@on.helpin.email
```

continue to work as-is. The current flow remains:

```text
Customer email provider
  -> forwards email to Helpin forwarding address
  -> Helpin creates/updates support conversation
```

The new sender-address work adds outbound identity and verification on top. It does not require current forwarding routes to be recreated, and it does not change the existing `Reply-To: conv-{id}@replies.helpin.email` fallback.

Backward compatibility rules:

- Existing forwarding addresses remain valid.
- Existing conversations keep threading through `replies.helpin.email`.
- If no custom sender address is configured, outbound email uses the current Helpin sender behavior.
- If a custom sender fails provider validation, Helpin retries with the verified fallback sender.
- Existing active sender-domain rows can be treated as an implementation detail or migrated into the new unified sender model without interrupting forwarding.

## Product Decision

Custom sender addresses should be a paid support feature.

Recommended gating:

- Available on paid support/inbox tiers.
- Not available on free/basic plans.
- Future branded reply domains, custom SMTP, and auto-DNS are enterprise-only if built later.

This should be decided before engineering starts because it affects UI visibility, route access, and upgrade prompts.

## Non-Goals

- Crisp-style branded reply domains with customer MX records.
- Automatic DNS provider setup through Entri, Cloudflare, or registrar APIs.
- Customer SMTP.
- Multiple brands.
- Full DMARC reporting ingestion.
- Building or operating our own MTA.

## Provider Parity Target

Target MVP:

- Add support email address.
- Authenticate domain.
- Verify forwarding.
- Set sender as default for workspace or inbox.
- Send replies from the configured address when safe.

Not included in MVP:

- Crisp-style custom reply domain such as `conv-id@support.company.com`.
- Customer-owned MX routing.
- Provider-specific auto DNS.

## Unified Data Model

Use a single product primitive: `support_email_senders`.

One row represents one usable address, for example `support@company.com`.

This is simpler than maintaining parallel `sender_domains` and `sender_addresses` hierarchies. The product is address-centric: admins configure addresses, not abstract domains.

### `support_email_senders`

Fields:

- `id`
- `workspace_id`
- `mailbox_id` nullable
- `email`
- `local_part`
- `domain`
- `display_name`
- `provider_domain_id`
- `forwarding_address`
- `forwarding_token_hash`
- `domain_status`
- `dkim_host`
- `dkim_value`
- `dkim_verified`
- `return_path_host`
- `return_path_value`
- `return_path_verified`
- `dmarc_host`
- `dmarc_policy`
- `dmarc_record_present`
- `dmarc_last_checked_at`
- `forwarding_status`
- `last_forwarded_at`
- `verification_status`
- `default_scope`
- `active`
- `last_checked_at`
- `last_error`
- `created_by_id`
- `created_at`
- `updated_at`

`default_scope`:

- `none`
- `workspace`
- `mailbox`

Constraints:

- Unique `(workspace_id, email)`.
- At most one workspace default per workspace.
- At most one mailbox default per mailbox.

Status model:

```text
draft
  -> pending_dns
  -> pending_forwarding
  -> verified
  -> disabled
  -> failed
```

## DMARC Safety

DMARC cannot be treated as cosmetic.

Activation requirements:

- DKIM must be verified.
- Return-path must be verified.
- If the domain has strict DMARC (`p=quarantine` or `p=reject`), DKIM alignment must be verified before any reply can use that address as visible `From`.

Rules:

- No custom sender activation without DKIM.
- No reply-from-inbound for strict DMARC domains unless DKIM is verified.
- If DMARC lookup fails, show warning but allow activation only if DKIM and return-path are verified.
- If strict DMARC exists and DKIM is incomplete, block activation with a clear reason.

This avoids silent recipient-side rejection when customers publish strict DMARC before completing Helpin authentication.

## Phase 1: Sender Addresses and Domain Authentication

### Goal

Admins can add `support@company.com`, authenticate its domain, and choose where it should be used for outbound replies.

### Requirements

- Add `support_email_senders`.
- Add APIs to create/list/update/disable sender rows.
- Normalize and validate email addresses.
- Use Postmark Account API to create or link domain authentication.
- Store DKIM and return-path DNS records.
- Resolve DMARC TXT record with best-effort DNS lookup.
- Show per-record status.
- Allow setting sender as:
  - workspace default
  - mailbox default
  - not default
- Outbound sender resolution uses verified sender rows.
- Existing Helpin fallback remains.

### Sender Resolution After Phase 1

```text
Agent sends reply
  |
  v
Does mailbox have verified default sender?
  | yes -> From: mailbox sender
  | no
  v
Does workspace have verified default sender?
  | yes -> From: workspace sender
  | no
  v
Use Helpin verified fallback sender
```

### Phase 1 ASCII UI

```text
Support Settings / Email

+---------------------------------------------------------------------+
| Sender Addresses                                      [Add sender]  |
+---------------------------------------------------------------------+
| Address                  Inbox          Status        Default        |
|---------------------------------------------------------------------|
| support@acme.com         Shared Inbox   DNS pending   -             |
| billing@acme.com         Billing        Verified      Billing       |
| hello@acme.com           -              Verified      Workspace     |
+---------------------------------------------------------------------+

Add sender
+---------------------------------------------------------------------+
| Email address                                                      |
| [ support@acme.com                                      ]          |
|                                                                   |
| Display name                                                       |
| [ Acme Support                                         ]           |
|                                                                   |
| Use for                                                           |
| ( ) Workspace default   ( ) Shared Inbox   ( ) Team Inbox          |
|                                                                   |
| Team Inbox                                                        |
| [ Billing                                      v ]                 |
|                                                                   |
|                                             [Cancel] [Add sender]  |
+---------------------------------------------------------------------+

DNS authentication for acme.com
+---------------------------------------------------------------------+
| Type    Host                         Value                  Status  |
|---------------------------------------------------------------------|
| TXT     pm._domainkey.acme.com       k=rsa; p=...           Pending |
| CNAME   pm-bounces.acme.com          pm.mtasv.net           Verified|
| TXT     _dmarc.acme.com              v=DMARC1; p=none       Found   |
+---------------------------------------------------------------------+
| [Verify DNS]                                                       |
+---------------------------------------------------------------------+
```

### Phase 1 API

- `GET /support/inbox/email-senders`
- `POST /support/inbox/email-senders`
- `PATCH /support/inbox/email-senders/{id}`
- `POST /support/inbox/email-senders/{id}/verify-dns`
- `POST /support/inbox/email-senders/{id}/set-default`
- `POST /support/inbox/email-senders/{id}/disable`

### Phase 1 Done Criteria

- Admin can add `support@company.com`.
- DNS records are visible and copyable.
- DKIM/return-path verification updates status.
- Strict DMARC without DKIM blocks activation.
- Workspace and mailbox defaults work.
- Outbound messages use verified default sender.
- Existing forwarding inboxes still work.
- Fallback sender still works.

## Phase 2: Forwarding Verification and Inbound Recipient Resolution

### Goal

Admins can prove that `support@company.com` forwards into Helpin, and Helpin can use the inbound recipient to choose the right reply sender.

This is the hard phase. It is likely larger than Phase 1 because every provider handles forwarding setup differently.

### Requirements

- Generate one forwarding address per sender.
- Generate one forwarding token per sender.
- Accept provider verification messages and Helpin verification tokens.
- Detect forwarded messages from:
  - Google Workspace / Gmail
  - Outlook / Microsoft 365
  - Zoho Mail
  - Fastmail
  - Generic forwarders
- Store original inbound recipient on the conversation.
- Mark forwarding status:
  - `not_started`
  - `pending`
  - `verified`
  - `failing`
- Show provider-specific setup instructions.
- Use inbound recipient sender when safe.

### Forwarding Target Options

Preferred v1 approach:

```text
fwd-{sender_id}-{token}@on.helpin.email
```

Benefits:

- Token is in local part.
- Easy to verify without relying on email body parsing.
- Does not need a per-customer catch-all domain.
- Existing inbound route infrastructure can be extended.

Alternative:

```text
support+verify-{token}@workspace.on.helpin.email
```

This is more user-friendly but creates more parsing and collision risk.

Recommendation: use `fwd-{sender_id}-{token}@on.helpin.email` for MVP.

### Phase 2 ASCII UI

```text
Support Settings / Email / Forwarding

+---------------------------------------------------------------------+
| Forwarding Setup                                                    |
+---------------------------------------------------------------------+
| support@acme.com                                                    |
| Inbox: Shared Inbox                                                  |
| Status: Pending verification                                        |
|                                                                     |
| Forward emails from:                                                |
|   support@acme.com                                                   |
|                                                                     |
| To this Helpin address:                         [Copy]              |
|   fwd-8bf3a9-7c91@on.helpin.email                                   |
|                                                                     |
| Provider instructions                                                |
| [ Google Workspace v ]                                               |
|                                                                     |
| 1. Open Gmail routing settings                                       |
| 2. Add a forwarding address                                          |
| 3. Paste the Helpin forwarding address                               |
| 4. Confirm the verification email if your provider asks              |
| 5. Send a test email to support@acme.com                             |
|                                                                     |
| [I sent a test email] [Refresh status]                               |
+---------------------------------------------------------------------+

Verified state
+---------------------------------------------------------------------+
| support@acme.com                                                    |
| Inbox: Shared Inbox                                                  |
| Status: Verified                                                     |
| Last received: May 1, 2026 18:42 UTC                                |
|                                                                     |
| Replies to conversations received at this address can use:           |
|   From: support@acme.com                                             |
+---------------------------------------------------------------------+
```

### Inbound Recipient Resolution

Store on conversation:

- `inbound_sender_id`
- `inbound_recipient_raw`

Resolution after Phase 2:

```text
Agent sends reply
  |
  v
Was conversation received through a verified sender address?
  | yes
  v
Is reply-from-inbound enabled and DMARC/DKIM safe?
  | yes -> From: inbound sender address
  | no
  v
Does mailbox have verified default sender?
  | yes -> From: mailbox sender
  | no
  v
Does workspace have verified default sender?
  | yes -> From: workspace sender
  | no
  v
Use Helpin verified fallback sender
```

### Phase 2 API

- `POST /support/inbox/email-senders/{id}/forwarding-token`
- `POST /support/inbox/email-senders/{id}/verify-forwarding`
- `POST /support/inbox/email-senders/{id}/refresh-forwarding-status`
- `PATCH /support/inbox/email-senders/{id}/reply-behavior`

### Phase 2 Done Criteria

- Admin can copy a sender-specific forwarding address.
- Helpin can verify forwarded test emails.
- Conversations store matched inbound recipient sender.
- Replies can use inbound recipient address when safe.
- Provider-specific instructions exist for Google Workspace, Outlook, Zoho, Fastmail, and generic providers.
- Admin diagnostics show forwarding status and last matched inbound.

## Migration From Current Implementation

The current implementation has `support_email_sender_domains` with an active domain. We should avoid expanding that into a long-term parallel hierarchy.

Migration approach:

1. Add `support_email_senders`.
2. For each active verified sender domain, create one sender row:
   - `email = from_local_part + '@' + domain`
   - `domain_status = verified` if current DKIM and return-path are verified
   - `forwarding_status = not_started`
   - `default_scope = workspace`
3. Keep `support_email_sender_domains` read-only during transition.
4. Change outbound resolution to prefer `support_email_senders`.
5. Keep old domain path as fallback until all production rows are migrated.
6. Later remove or collapse `support_email_sender_domains`.

No existing forwarding route needs to be changed for this migration.

## Operations and Diagnostics

Admin diagnostics should show:

- Sender address.
- Sender status.
- Domain DNS status.
- DMARC policy.
- Forwarding status.
- Last verification attempt.
- Last forwarded message timestamp.
- Last outbound sender resolution reason.
- Whether fallback sender was used.
- Last provider error.

Logs should include:

- `workspace_id`
- `conversation_id`
- `message_id`
- `sender_id`
- `resolved_from_email`
- `resolution_reason`
- `fallback_used`
- `provider_error`

## Adoption Metrics

Adoption is a first-class metric. Deliverability quality does not matter if no one configures the feature.

Track:

- Workspaces that view Email sender settings.
- Workspaces that start sender setup.
- Workspaces that add at least one sender.
- Workspaces that complete DNS verification.
- Workspaces that complete forwarding verification.
- Active custom senders by workspace.
- Percentage of support outbound emails sent from custom senders.
- Drop-off step in setup flow.
- Setup time from first add to verified.

Quality metrics:

- Fallback rate for custom senders.
- Provider 422 sender-signature errors.
- Bounce rate for custom senders.
- Forwarding verification failure rate.
- Reply-threading failure rate.

## Risks

- Forwarding setup is provider-specific and slow for admins.
- Google Workspace and Outlook routing instructions may change.
- Strict DMARC can silently break deliverability if activation is too permissive.
- Existing forwarding routes must remain untouched.
- Sender resolution can confuse teammates if the UI does not explain defaults.
- A unified model may need normalization later if enterprise multi-brand support grows.

## Future Work

Separate PRDs if usage justifies them:

- Branded reply domains with MX records.
- Auto-DNS through Entri, Cloudflare, or registrar APIs.
- Customer SMTP.
- Multiple brands.
- DMARC aggregate report ingestion.

## References

- Intercom outbound custom addresses: https://www.intercom.com/help/en/articles/182-send-outbound-email-from-your-own-address
- Intercom email channel setup and authentication: https://www.intercom.com/help/en/articles/9744849-connect-your-email-support-channel
- Intercom DKIM and DMARC troubleshooting: https://www.intercom.com/help/en/articles/11183745-troubleshooting-email-authentication-issues-dkim-dmarc-and-dns-setup
- Crisp custom email domain: https://help.crisp.chat/en/article/how-can-i-setup-a-custom-email-domain-ndklh7/
