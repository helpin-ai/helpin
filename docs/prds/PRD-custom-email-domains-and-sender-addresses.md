# PRD: Custom Email Domains and Sender Addresses

> Superseded for implementation by [PRD-custom-support-sender-addresses-mvp.md](PRD-custom-support-sender-addresses-mvp.md).
> This file is retained as the broader parity reference. Use the MVP PRD for engineering scope.

## Summary

Helpin should support branded support email sending at Intercom-level parity, with a path toward Crisp-style full custom reply domains.

Workspace admins should be able to configure support addresses such as `support@company.com`, authenticate the sending domain, verify forwarding, and choose how support replies are sent. Customers should receive emails from addresses they recognize, and replies should route back into Helpin reliably.

The current implementation is a production-safe foundation: custom sender domains can be created through Postmark, DKIM and return-path records can be verified, and outbound fallback emails can send from the active verified custom domain with a `support@helpin.email` fallback. This PRD defines the complete product surface required to close the remaining parity gaps.

## Goals

- Let workspaces send support emails from verified customer-owned addresses.
- Support address-level configuration such as `support@company.com`, `billing@company.com`, and `vip@company.com`.
- Support domain authentication with DKIM, custom return-path, and DMARC guidance.
- Verify inbound forwarding for configured support addresses.
- Let replies come from the inbound address when a customer emailed a configured address.
- Preserve reliable fallback delivery through verified Helpin senders.
- Provide operational diagnostics for sender, domain, forwarding, and delivery health.

## Non-Goals

- Build or operate a custom MTA in this phase.
- Replace Postmark in this phase.
- Ship automatic DNS provider configuration in v1.
- Ship customer SMTP in v1.
- Build full multi-brand support before the sender/address model exists.

## Background

Intercom separates support email setup into a few related concepts:

- Add and verify support email addresses.
- Configure automatic forwarding into the inbox.
- Authenticate the domain with DKIM, a custom return-path, and DMARC.
- Choose the default reply address.
- Optionally reply from the inbound address the customer contacted.

Crisp offers a related but different model:

- By default, Crisp sends from hosted domains such as `company.on.crisp.email`.
- For full custom branding, Crisp recommends a dedicated subdomain such as `support.company.com`.
- That subdomain can handle outbound branding and inbound replies through DNS records, including MX.

Helpin should first support existing support addresses such as `support@company.com`. A dedicated reply subdomain should be offered as an advanced option for full white-label routing.

## Product Positioning

Use two product concepts:

1. **Sender addresses**
   - Examples: `support@company.com`, `billing@company.com`
   - Best for Intercom parity.
   - Requires forwarding and domain authentication.
   - Replies can come from the inbound address once verified.

2. **Branded reply domains**
   - Example: `support.company.com`
   - Best for Crisp-style full white-label routing.
   - Requires a dedicated subdomain and MX records.
   - Supports `conv-{conversation_id}@support.company.com` reply routing.

The dedicated subdomain option is better for full reply-domain ownership because it avoids conflicts with Google Workspace, Outlook, and other existing mail providers on the root domain. It should be recommended for advanced branded routing, not required for basic sender address setup.

## Current State

Implemented foundation:

- `support_email_sender_domains` model.
- Postmark Account API client for domain creation and verification.
- DKIM and return-path DNS records are stored and shown.
- One active verified sender domain can be selected per workspace.
- Outbound fallback emails prefer the active verified custom sender domain.
- If Postmark rejects the sender with a sender-signature `422`, Helpin retries with the verified fallback sender.
- Settings UI includes an `Email Custom Domains` tab.

Current gaps:

- No sender-address model.
- No address ownership verification.
- No forwarding verification.
- No address-to-inbox mapping.
- No workspace-level or mailbox-level default sender pointer (today the closest analog is the workspace-singleton "active verified sender domain", which becomes vestigial once addresses exist — see [Migration From Active Domain](#migration-from-active-domain)).
- No `inbound_recipient_address` capture on conversations.
- No “reply from inbound address” resolution.
- No DMARC record display/status (Postmark Account API does not verify DMARC — see [DMARC Handling](#dmarc-handling)).
- No branded reply-domain MX setup.
- No auto-DNS setup.
- No customer SMTP.

## User Personas

### Workspace Admin

Configures support email addresses, DNS records, forwarding, and default reply behavior.

### Support Manager

Owns team inboxes and wants the right sender address per inbox.

### Teammate

Replies from Helpin and expects the customer to see a trusted, branded From address.

### Customer

Receives support replies from a recognizable address and can reply normally.

## User Stories

- As an admin, I can add `support@company.com` and connect it to the Shared Inbox.
- As an admin, I can add `billing@company.com` and connect it to the Billing Team Inbox.
- As an admin, I can verify DNS authentication for my domain.
- As an admin, I can verify that forwarding is correctly configured.
- As an admin, I can set a default workspace sender address.
- As an admin, I can set a default sender address per team inbox.
- As a teammate, when I reply to a conversation that came through `billing@company.com`, the reply can come from `billing@company.com`.
- As a customer, when I reply to an emailed response, the reply appears in the original Helpin conversation.
- As an operator, I can see why a message used a custom sender or fell back to `support@helpin.email`.

## Functional Requirements

### Sender Domains

- Admin can add a sender domain.
- Helpin creates or links the provider-side domain.
- Helpin displays DNS records for:
  - DKIM
  - Custom return-path
  - DMARC recommendation
- Admin can refresh verification status.
- Helpin shows per-record status.
- Domain cannot be used for outbound sending until required records are verified.
- Domain can be deactivated.
- Multiple sender domains can exist per workspace and may all be authenticated simultaneously. The legacy single-active-domain semantics is preserved only as a fallback domain pointer (see [Migration From Active Domain](#migration-from-active-domain)).
- Manual `Verify` clicks must be rate-limited per domain to avoid hammering Postmark Account API checks. Recommended floor: one verify call per domain per 30s, surfaced as a disabled button with countdown.

### Sender Addresses

- Admin can add sender addresses such as `support@company.com`.
- Address domain must be authenticated before the address can be used.
- Address can be associated with:
  - Shared Inbox
  - Team Inbox
  - Workspace default
- Address has status:
  - `pending_domain`
  - `pending_address_verification`
  - `pending_forwarding`
  - `verified`
  - `disabled`
  - `failed`
- Address can define a display name.
- Address can be disabled without deleting historical conversations.

### Address Verification

The system should support at least one of:

- Email-click verification sent to the address.
- Forwarding-token verification received through inbound forwarding.

Domain authentication alone should not prove that the workspace controls a specific mailbox.

**Decision required before Phase 2 starts** (was an open question): v1 ships **forwarding-token verification only**. Email-click verification is deferred — it adds a parallel verification path that complicates state transitions and has weaker signal (proves SMTP delivery to the address, not that it is forwarded into Helpin). Forwarding verification is the stronger guarantee because it is the same channel that will carry production inbound mail.

### Forwarding Verification

- Helpin generates a forwarding address for each sender address.
- Admin configures forwarding in Google Workspace, Outlook, Zoho, or another provider.
- Helpin verifies forwarding by receiving a token or provider verification message.
- Helpin stores last forwarding verification timestamp.
- Helpin shows forwarding status and last inbound timestamp.

### Reply Behavior

Admin can choose reply behavior:

- Workspace default sender.
- Team inbox sender.
- Inbound address.

Resolution priority:

1. If the conversation came through a verified inbound address and “reply from inbound address” is enabled, use that address.
2. Else use the mailbox default sender address.
3. Else use the workspace default sender address.
4. Else use the active verified sender domain fallback local part.
5. Else use the verified Helpin fallback sender.

**Decision required before Phase 2 starts** (was an open question): mailbox default sender, when set, **always** overrides the workspace default. Setting a workspace default does not silently propagate to mailboxes that already have one. UI must surface "Inheriting workspace default" vs "Custom mailbox default" explicitly.

Phase 4 ("reply from inbound address") does **not** depend on Phase 5 (Branded Reply Domains). The `From: billing@company.com` header works independently of where `Reply-To` points; the existing `replies.helpin.email` Reply-To is preserved until Phase 5 ships.

### Branded Reply Domain

Admin can configure a dedicated reply subdomain such as `support.company.com`.

Requirements:

- The domain must be a subdomain, not the apex/root domain.
- Helpin displays MX records.
- Helpin displays DKIM/SPF/DMARC guidance.
- Helpin verifies inbound routing.
- Once active, `Reply-To` can use `conv-{conversation_id}@support.company.com`.
- Existing `replies.helpin.email` remains fallback.

### Operational Safety

- Always retain verified Helpin fallback sender.
- Retry once with fallback sender on provider sender-signature failures.
- Log selected outbound sender and resolution reason.
- Log fallback usage.
- Track failed domain verification attempts.
- Track forwarding health.
- Admin diagnostics should surface domain/address problems without requiring log access.
- Rate-limit manual verification triggers (domain, address, forwarding) per resource to avoid Postmark Account API throttling and DNS resolver pressure.

### DMARC Handling

DMARC is a published-policy record, not something Postmark verifies. Helpin's responsibility is **display + best-effort polling**, not authoritative verification:

- Render the recommended DMARC record (host + value) with copy buttons.
- On `Verify` click, resolve the `_dmarc.<domain>` TXT record directly via DNS and parse the policy (`p=none|quarantine|reject`).
- Persist `dmarc_record_present` (bool), `dmarc_policy` (string), and `dmarc_last_checked_at`. Do **not** persist a `dmarc_verified` boolean — there is no binary "verified" state for DMARC.
- Surface guidance: "Record found, policy `p=none`" / "No DMARC record found" / "DNS lookup failed".
- DMARC is never a blocker for activating a sender domain or address. It is a deliverability recommendation only.

## UX Requirements

### Settings IA

Support settings should contain:

- Team Inboxes
- Conversation Routing
- **Email** (consolidated page with internal tabs — see below)

The `Email` settings page is the canonical home for everything in this PRD. It has internal tabs:

- Addresses
- Domains
- Forwarding
- Reply Behavior
- Diagnostics

The current `Email Forwarding` and `Email Custom Domains` sibling tabs are migrated into the `Email` page during Phase 2. Verification UX (DNS records table, copy buttons, status badges, last-checked timestamps) is shared across the Domains, Addresses, Forwarding, and Branded Reply Domain surfaces — keeping them under one page enables a single shared `<VerificationRecordTable />` component instead of four near-duplicates.

### Email Custom Domains Tab

Must show:

- Active sender domain.
- Add domain form.
- DNS records table.
- Copy buttons for host/value.
- Per-record status.
- Last checked timestamp.
- Last error.
- Verify DNS action.
- Activate action only when required checks pass.

### Sender Addresses Tab

Must show:

- Email address.
- Associated inbox(es) — derived from `support_inboxes.default_sender_address_id` references, not a column on the address itself.
- Domain authentication status (joined from `support_email_sender_domains`).
- Address verification status.
- Forwarding status with last inbound timestamp.
- Default badges: "Workspace default" / "Default for {Mailbox name}" — derived from FKs, not flags.
- Actions: verify forwarding, set as workspace default, set as mailbox default, disable.

When an address is set as a mailbox default, the mailbox's existing default is replaced (1:1 relationship), and the UI confirms the override.

### Copy and Guidance

The UI should clearly explain:

- DNS propagation can take up to 72 hours.
- Some DNS providers auto-append the domain name.
- SPF is handled by custom return-path alignment for the provider path.
- DMARC is recommended and required for high-volume senders.
- A dedicated subdomain is recommended for fully branded reply routing.

## Data Model

### `support_email_sender_domains`

Existing and expanded fields:

- `id`
- `workspace_id`
- `domain`
- `postmark_domain_id`
- `return_path_domain`
- `return_path_domain_cname_value`
- `return_path_domain_verified`
- `dkim_host`
- `dkim_text_value`
- `dkim_pending_host`
- `dkim_pending_text_value`
- `dkim_verified`
- `dmarc_host`
- `dmarc_text_value`
- `dmarc_record_present` (replaces `dmarc_verified` — see [DMARC Handling](#dmarc-handling))
- `dmarc_policy` (`none` | `quarantine` | `reject` | empty)
- `dmarc_last_checked_at`
- `status`
- `active`
- `last_checked_at`
- `last_error`
- `created_by_id`
- `created_at`
- `updated_at`

### `support_email_sender_addresses`

Proposed fields:

- `id`
- `workspace_id`
- `domain_id` (FK → `support_email_sender_domains.id`)
- `email` (canonical lowercased `local_part@domain`, unique per workspace)
- `local_part`
- `display_name`
- `verification_status`
- `forwarding_status`
- `verification_token_hash`
- `forwarding_address` (Helpin-generated route the customer's mail provider forwards to)
- `reply_from_inbound_enabled`
- `active`
- `last_verified_at`
- `last_forwarded_at`
- `last_error`
- `created_by_id`
- `created_at`
- `updated_at`

Note: `mailbox_id`, `default_for_workspace`, and `default_for_mailbox` from earlier drafts are **removed** — mailbox/workspace ownership is now expressed as FKs **on the owning side** (mailbox/workspace settings) rather than backref booleans on the address. This avoids multi-row consistency bugs (two rows both flagged `default_for_workspace=true`) and lets a single address legitimately serve multiple mailboxes.

### Mailbox and workspace pointers (NEW)

Add to existing tables:

- `support_inboxes` (mailboxes): add `default_sender_address_id` (FK → `support_email_sender_addresses.id`, nullable). Null means inherit from workspace default.
- `workspace_support_settings` (or equivalent workspace-scoped settings table): add `default_sender_address_id` (FK, nullable) and `fallback_sender_domain_id` (FK → `support_email_sender_domains.id`, nullable — replaces the legacy "active verified sender domain" workspace-singleton, see [Migration From Active Domain](#migration-from-active-domain)).

A `support_email_sender_addresses` ↔ `support_inboxes` join table (`support_inbox_sender_addresses`) supports the case where one address routes to multiple mailboxes (rare but valid for shared catch-all addresses). For v1, a single mailbox FK on inbound routing is sufficient — the join table can be deferred until needed.

### Conversation inbound recipient (NEW)

`support_conversations`: add `inbound_recipient_address_id` (FK → `support_email_sender_addresses.id`, nullable) and `inbound_recipient_raw` (string, nullable).

- Set on conversation creation when the inbound originated from a verified sender address.
- `inbound_recipient_raw` captures the raw `To`/`Delivered-To` value even when no matching sender address exists (useful for forensics and for upgrading legacy conversations once an address is added).
- This field is the input to resolution priority #1 ("reply from inbound address") — without it, that priority cannot fire.

### `support_email_reply_domains`

Proposed fields:

- `id`
- `workspace_id`
- `domain`
- `provider_domain_id`
- `mx_host`
- `mx_value`
- `spf_host`
- `spf_value`
- `dkim_host`
- `dkim_value`
- `dmarc_host`
- `dmarc_value`
- `mx_verified`
- `spf_verified`
- `dkim_verified`
- `dmarc_verified`
- `status`
- `active`
- `last_checked_at`
- `last_error`
- `created_by_id`
- `created_at`
- `updated_at`

## API Requirements

Existing:

- `GET /support/inbox/email-sender-domains`
- `POST /support/inbox/email-sender-domains`
- `POST /support/inbox/email-sender-domains/{id}/verify`
- `POST /support/inbox/email-sender-domains/{id}/activate`
- `POST /support/inbox/email-sender-domains/{id}/deactivate`

Add:

- `GET /support/inbox/email-sender-addresses`
- `POST /support/inbox/email-sender-addresses`
- `POST /support/inbox/email-sender-addresses/{id}/verify`
- `POST /support/inbox/email-sender-addresses/{id}/verify-forwarding`
- `POST /support/inbox/email-sender-addresses/{id}/set-default`
- `POST /support/inbox/email-sender-addresses/{id}/disable`

Later:

- `GET /support/inbox/email-reply-domains`
- `POST /support/inbox/email-reply-domains`
- `POST /support/inbox/email-reply-domains/{id}/verify`
- `POST /support/inbox/email-reply-domains/{id}/activate`
- `POST /support/inbox/email-reply-domains/{id}/deactivate`

## Migration From Active Domain

Today, `support_email_sender_domains.active` is a workspace-singleton flag — exactly one verified domain can be active per workspace, and outbound fallback emails use `<sender_local_part>@<active_domain>`. Once sender addresses exist, that concept becomes vestigial.

Migration plan:

1. **Phase 2 ship**: introduce `workspace_support_settings.fallback_sender_domain_id` and `workspace_support_settings.default_sender_address_id`. On migration apply, copy the current `active=true` row's id into `fallback_sender_domain_id` for each workspace.
2. **Phase 2 backfill**: for each workspace with an active domain, auto-create one `support_email_sender_addresses` row (`<from_local_part>@<domain>`, `verification_status='pending_forwarding'`, `forwarding_status='unverified'`) and set `workspace_support_settings.default_sender_address_id` to it. The address starts unverified — the admin must complete forwarding setup to activate the new resolution path.
3. **Phase 2 cutover**: outbound resolution starts using the address-based path (priorities 1–3). The legacy "active domain default" path (priority 4) reads from `fallback_sender_domain_id` instead of the `active` flag.
4. **Phase 4 follow-up**: once forwarding-verified addresses are in production, drop the `active` column from `support_email_sender_domains` in a separate migration. Multiple domains can be authenticated and used simultaneously thereafter.

This avoids a flag-day cutover: the legacy fallback path keeps working through Phases 2–4 while admins migrate.

## Outbound Sender Resolution

Every outbound support email should record:

- `resolved_from_email`
- `resolution_reason`
- `custom_sender_domain_id`
- `sender_address_id`
- `fallback_used`

Resolution reasons:

- `conversation_inbound_address`
- `mailbox_default`
- `workspace_default`
- `active_domain_default`
- `helpin_fallback`

## Inbound Routing

Current routing:

- `Reply-To: conv-{conversation_id}@replies.helpin.email`

Future branded reply domain routing:

- `Reply-To: conv-{conversation_id}@support.company.com`

Forwarded support address routing:

- Customer emails `support@company.com`.
- Customer mail provider forwards to Helpin-generated route.
- Helpin records original inbound recipient.
- Teammate reply can come from `support@company.com` if enabled and verified.

## Phased Delivery

Phases 2 and 3 are sequential — Phase 3 writes to `support_email_sender_addresses` rows that Phase 2 creates. Phase 4 depends on Phase 3 (needs `inbound_recipient_address_id`). Phase 5 is independent of Phase 4 and can ship in either order.

### Phase 1: Domain Authentication Hardening

- Keep current implementation.
- Add DMARC display fields per [DMARC Handling](#dmarc-handling) (`dmarc_record_present`, `dmarc_policy`, `dmarc_last_checked_at`).
- Improve per-record DNS status.
- Add copy and status polish.
- Rate-limit manual verification triggers.

### Phase 2: Sender Addresses (depends on Phase 1)

- Add `support_email_sender_addresses` model.
- Add `default_sender_address_id` to `support_inboxes` and `workspace_support_settings`.
- Add `fallback_sender_domain_id` to `workspace_support_settings`; backfill from current `active=true` domain.
- Add sender address settings UI (consolidated `Email` page, `Addresses` tab).
- Run migration described in [Migration From Active Domain](#migration-from-active-domain).
- Outbound resolution starts using priorities 1–3 (#1 stays inert until Phase 3 lands `inbound_recipient_address_id`).

### Phase 3: Forwarding Verification (blocks on Phase 2)

- Generate verification tokens per address.
- Verify forwarding for each sender address.
- Add `inbound_recipient_address_id` and `inbound_recipient_raw` to `support_conversations`; populate on inbound mail.
- This phase cannot start until Phase 2's address rows exist — forwarding tokens are address-scoped.

### Phase 4: Reply From Inbound Address (depends on Phase 3)

- Add reply behavior setting (`reply_from_inbound_enabled` per address, plus workspace-level toggle).
- Activate resolution priority #1 using `inbound_recipient_address_id`.
- `From: address@customer.com` works with the existing `replies.helpin.email` Reply-To. Branded Reply-To is **not** a prerequisite.
- Add diagnostics surface for sender resolution (resolved sender + reason per outbound message).
- Drop legacy `support_email_sender_domains.active` column in this phase's migration.

### Phase 5: Branded Reply Domains (independent of Phase 4)

- Add dedicated subdomain setup.
- Show MX and authentication records.
- Verify inbound domain routing.
- Use branded conversation reply addresses (`conv-{id}@support.company.com`).

### Phase 6: Auto DNS and Custom SMTP

- Evaluate Entri or Cloudflare API integration for automatic DNS setup.
- Add optional customer SMTP for high-volume/deliverability-sensitive customers.

## Success Metrics

- 95%+ custom sender messages send without fallback.
- Zero recurring Postmark sender-signature `422` errors for active verified domains.
- Forwarding verification completion rate above 80% for started setups.
- Reduced support tickets about missing or unbranded support emails.
- Increased reply rate for workspaces using custom sender addresses.

## Risks

- DNS setup is error-prone.
- Root-domain SPF/DMARC changes can affect customers' existing mail systems.
- Forwarding providers have inconsistent verification flows.
- Incorrect sender selection can break reply threading.
- Custom domains may create deliverability problems if customers publish strict but incomplete DMARC.

## Decisions

Resolved during PRD revision (formerly open questions):

- **Address ownership verification**: forwarding-token verification only in v1. Email-click is deferred. (See [Address Verification](#address-verification).)
- **Mailbox vs workspace defaults**: mailbox default always overrides workspace default when set. UI must show inheritance state explicitly. (See [Reply Behavior](#reply-behavior).)
- **DMARC**: display + best-effort polling, not authoritative verification. Never blocks activation. (See [DMARC Handling](#dmarc-handling).)
- **Phase ordering**: Phase 4 does not require Phase 5. Branded Reply Domains can ship before or after "reply from inbound address." (See [Phased Delivery](#phased-delivery).)
- **Active sender domain migration**: the workspace-singleton `active` flag is replaced by `workspace_support_settings.fallback_sender_domain_id` and dropped in Phase 4. (See [Migration From Active Domain](#migration-from-active-domain).)

## Open Questions

- Should custom sender addresses be gated by plan?
- Should branded reply domains be gated separately from sender addresses?
- How should multiple brands map to sender addresses (one address per brand, or a `brand_id` on the address)?
- Should Helpin offer DMARC aggregate-report (`rua=`) ingestion later?

## References

- Intercom outbound custom addresses: https://www.intercom.com/help/en/articles/182-send-outbound-email-from-your-own-address
- Intercom email channel setup and authentication: https://www.intercom.com/help/en/articles/9744849-connect-your-email-support-channel
- Intercom DKIM and DMARC troubleshooting: https://www.intercom.com/help/en/articles/11183745-troubleshooting-email-authentication-issues-dkim-dmarc-and-dns-setup
- Crisp custom email domain: https://help.crisp.chat/en/article/how-can-i-setup-a-custom-email-domain-ndklh7/
