# Combined inbox plan review

Date: 2026-11-02. Status: review; implementation not started.

This review is for engineers and support roadmap owners evaluating the
[combined inbox plan](2026-11-02-combined-inbox-plan.md), including its SMS and
RCS additions. It assesses the proposed design against Helpin at `77226e460`
and selected provider documentation. It does not authorize implementation or
set roadmap priority. Scope: shared Community product architecture; no private
operational or customer data is included.

## Assessment

Combined inbox support is a natural direction for Helpin, but the plan needs
revision before implementation. Commit initially to phases 0–1, then choose
additional channels based on customer demand.

The timing makes sense: reporting, CSAT, SLAs, bulk actions, and support
webhooks are already implemented. The
[support operations outcome](2026-10-25-support-operations-plan.md#outcome)
and current code provide a stronger basis for this decision than the historical
September gap list. Implementation evidence does not establish deployment or
customer adoption.

The strongest decisions are keeping mailboxes separate from connected accounts,
introducing identities scoped to those accounts, using durable delivery,
centralizing channel policy, and shipping incrementally. These extend the
existing inbox without requiring the full cross-channel person model first.

## Findings to address before implementation

### 1. Separate message identity from event identity

The proposed deduplication keys can discard legitimate messages or updates.
Telegram message IDs are unique within a chat, not across all chats served by
a bot. Store a composite external message key such as `chat_id:message_id`,
or include the external chat ID in the uniqueness constraint. Keep `update_id`
as the Telegram webhook-event key.

Delivery events also need separate keys: successive status callbacks for the
same Twilio `MessageSid` must all survive. A message ID alone cannot distinguish
sent, delivered, and read updates. Define provider-specific keys for messages,
delivery/read events, edits, and reactions, including watermark events without
a message ID.

Verification should cover two Telegram chats using the same message ID,
duplicate deliveries of one event, successive statuses for one message, and
out-of-order callbacks.

Sources: [Telegram message and update definitions](https://core.telegram.org/bots/api#message)
and [Twilio status lifecycle](https://www.twilio.com/docs/messaging/guides/outbound-message-status-in-status-callbacks).

### 2. Define Telegram account resolution explicitly

The webhook pipeline expects the connected account ID in the payload. Ordinary
Telegram updates do not provide the receiving bot ID that way. Verification
also needs the correct account's secret before processing the event.

Use an account-specific webhook identifier plus secret verification, or define
a secret-to-account lookup. Do not put the bot API token in the webhook URL.
Test multiple connected bots, wrong secrets, unknown accounts, and reconnects.

Source: [Telegram webhook and update contract](https://core.telegram.org/bots/api#setwebhook).

### 3. Include the separate AI publication path in outbound delivery

The plan attaches delivery to `CreateConversationMessage`, but
[`createSupportAIReply`](../../server/internal/service/support_ai_publish.go)
writes through separate message and processing repositories. Following the
proposed integration literally could show an AI answer in Helpin without
delivering it to the customer.

Specify a shared transactional delivery path covering human replies, AI
replies, and follow-ups. Preserve the existing atomic relationship between
AI publication and processing settlement. Explicitly exclude internal notes
and internal system events from external delivery.

Verification should prove that each supported reply producer creates its
message and delivery intent together, while private content never enters the
channel outbox.

### 4. Define uncertain sends and recheck policy at dispatch

A provider can accept a send before Helpin times out or crashes. Retrying the
same local outbox row can still send the customer a duplicate: a semantic key
prevents duplicate local intent, not duplicate remote execution.

Define how each adapter handles uncertain outcomes, provider idempotency where
available, reconciliation, and callbacks arriving before the send result is
persisted. Distinguish a definite rejection from an unknown delivery outcome.

Recheck reply windows, disconnects, opt-outs, and deletion immediately before
sending, including retries. Queue-time checks alone become stale. Define
cancellation and retry transitions separately from forward-only delivery
receipt progression.

Audit existing
[pending sends](../../server/internal/service/support_pending_send.go),
[delivery and undo behavior](../../server/internal/service/support_delivery.go),
and [merged-conversation delivery](../../server/internal/service/support_merge_delivery.go).
Record which capabilities carry over and any deliberate restrictions. Test a
timeout after provider acceptance, a crash before saving the external ID,
early callbacks, and a window or opt-out change while queued.

### 5. Extend privacy cleanup to the new identities and queues

The plan overstates what existing deletion flows cover.
[Contact privacy cleanup](../../server/internal/repository/crm_contact_privacy.go)
matches conversations and, for unprocessed email, sender email addresses.
The [inbound privacy trigger](../../server/internal/dbmigrate/sql/202609210002_support_inbound_jobs.sql)
depends on an associated conversation. Neither automatically covers unlinked
channel identities or channel events awaiting conversation creation.

Add explicit identity cleanup, pending-send cancellation, payload clearing,
and protection against replay restoring deleted identity. Define behavior for
contacts with several channel identities and identities with no CRM contact.
Test deletion racing with inbound processing and outbound dispatch.

Start with manual CRM linking. Exact phone matching needs a separate decision
about normalization, ambiguous matches, and identity trust; the
[contact model](../../server/internal/model/crm_contact.go) does not establish
phone uniqueness.

### 6. Preserve existing behavior during the channel registry migration

Centralizing channel policy is worthwhile. The source/channel discrepancy is
present in
[`supportConversationChannel`](../../server/internal/service/support_inbox_triage.go).
However, a simple `chat|email` mapping must retain the existing distinctions in
the [AI channel policy](../../server/internal/model/support_ai_channels.go):
portal drafts, portal visibility, incoming email versus the original
conversation channel, mailbox overrides, and machine-mail suppression.

Before backfilling `channel` or narrowing `source` to import provenance, audit
all readers and writers. Preserve imported-history exclusions, per-message
delivery intent, and the ability to distinguish transport from provenance.
Use a behavior matrix for widget, email, portal, API, internal, and imported
conversations, including mixed-channel histories.

## Recommended sequence

1. Implement the channel registry with compatibility checks and explicit
   migration rules. This has value independently of new providers.
2. Deliver one complete Telegram integration, including human and AI replies,
   attachments, retries, disconnect/reconnect, access controls, and deletion.
   Treat it as proof of the shared delivery architecture.
3. Choose the next provider from actual customer demand. Telegram is a useful
   engineering proving ground; it does not establish whether WhatsApp or
   Messenger/Instagram should ship next.
4. Treat WhatsApp templates as part of its usable release, even if implemented
   in a separate phase, so teams have a defined way to resume support outside
   the free-form reply window.
5. Schedule SMS and RCS separately. Their sender setup, fallback behavior, and
   operational costs broaden the scope considerably.

For RCS, the previous delivery being RCS does not guarantee the next delivery
will be RCS. Twilio can fall back to SMS. The composer needs fallback-aware
content and cost behavior, rather than assuming that the last delivery permits
unrestricted rich content for the next send.
[Twilio documents the fallback behavior](https://www.twilio.com/docs/studio/tutorials/use-rcs-with-studio).

Treat the effort estimates as provisional until delivery, lifecycle, and
compatibility work is included. The full channel roadmap is broader than a
single next implementation step.

## Research corrections and validation limits

The plan says Intercom does not offer Telegram, but the cited
[Intercom channel list](https://www.intercom.com/help/en/articles/9955432-channels-explained)
currently lists Telegram. Correct the categorical claim and verify the actual
integration scope before using it as a differentiation argument. Channel
presence alone does not establish feature parity.

This was a source and documentation review. No application tests, provider
integration tests, or live staging checks were run. Meta documentation was
inaccessible through browsing, so detailed permission and policy claims remain
unverified. The referenced Chatwoot revision could not be retrieved during this
review; the plan's detailed Chatwoot comparisons were not independently
confirmed. SMS/RCS fees, country coverage, and registration timelines were not
fully revalidated.
