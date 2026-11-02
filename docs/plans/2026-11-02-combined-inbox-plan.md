# Combined inbox: social and messaging channels

Date: 2026-11-02. Branch: `docs/combined-inbox-plan`. Status: proposal, revised after
[review](2026-11-02-combined-inbox-review.md). Commitment is phases 0–1 only;
later providers are chosen from customer demand.

Audience: engineers and whoever decides the support roadmap. This plan covers
how Helpin would take customer conversations from Telegram, Facebook Messenger,
Instagram DMs, WhatsApp, SMS and RCS into the existing support inbox. It is based on
a read of Chatwoot (shallow clone of `chatwoot/chatwoot` `master`, commit
`b08081b`, 2026-10-30; `git clone --depth 1` reproduces it while `master` stays
there) and of Helpin
`develop` at `77226e460`.

Roadmap context: the
[open-source gap list](../strategy/2026-09-10-open-source-intercom-alternative-gap-list.md)
§6 #3 lists these channels as deferred, "not a launch target". This document
plans the work so it is ready when it gets scheduled; it does not set priority.

Intercom parity: Intercom's
[channel list](https://www.intercom.com/help/en/articles/9955432-channels-explained)
names Messenger, email, phone, WhatsApp, Switch, Slack, SMS, Facebook,
Instagram and Telegram. Telegram is listed without detail, so its depth is
unverified. X and RCS are not listed. Phases 1–5 aim at that set. Being on a
list is not feature parity, and only RCS is a candidate differentiator. Check
it again before using it in positioning.

## Problem

Helpin supports widget, email, portal, api and internal channels. Nothing else
exists: the only places Facebook or Instagram appear are CRM profile URL
columns and icons. Three things in the current code block adding a channel:

1. **Channel handling is spread across about twelve hard-coded lists.**
   `support_conversations.channel` is free text with no CHECK. A new value has to
   be added by hand in each of these places:
   - AI gate: `model/support_ai_channels.go`. Unknown channels mean AI is off.
   - Triage: `validSupportTriageChannel` and `triageChannelEnabled` in
     `service/support_inbox_triage.go:1093,1294`. Unknown channels fall through
     to the widget setting.
   - SLA allow-list: `service/support_sla.go:27`.
   - Reports: `service/support_reports.go`.
   - CSAT CHECK: `202610250004`.
   - Default mailbox: `service/support_mailbox.go:681`.
   - Frontend: `TicketSource`, `CHANNEL_META` and `SOURCE_LABELS`.

   The code also disagrees on which field to read. Triage reads `source` first,
   while SLA reads `channel`, so Intercom-imported conversations resolve as
   `intercom` in triage and as `widget` in SLA.
2. **Identity is email-only.** `matchOrCreateCRMContactIdentityTx`
   (`service/support_inbox.go:3684`) returns nil when there is no email. Social
   channels give a platform user id (PSID, IGSID, `wa_id`, Telegram user id,
   E.164 phone) and usually no email.
3. **Messages have no external id.** `support_messages` has no
   `external_message_id`. Email dedupe relies on the inbound job primary key
   plus `support_email_logs`. Social channels need dedupe per platform message
   id, for webhook retries and for echoes.

## What Chatwoot does and what we take from it

Chatwoot is the closest open-source reference. Its shape:

- **Inbox → polymorphic channel.** There is one typed table per channel
  (`channel_facebook_pages`, `channel_whatsapp`, `channel_telegram`, …) holding
  credentials, with a unique index on the external account id. The webhook
  resolves the channel from page id, IG id, phone number or bot token.
- **ContactInbox (inbox_id, source_id) is unique.** This is the identity key: a
  platform user id scoped to one connected account. Contacts are merged across
  channels by email, phone or identifier.
- **Inbound** goes webhook controller → verify signature → enqueue job →
  builder, with a Redis mutex per sender so two quick messages don't open two
  conversations.
- **Outbound** goes `Message after_create_commit` → `SendReplyJob` →
  `Base::SendOnChannelService` subclass. The external id is written back after
  sending. Any message that already has a `source_id` is never sent, which is
  the loop guard.
- **`MessageWindowService`** is the single place for reply rules: WhatsApp 24h,
  Messenger/IG 24h (7 days with HUMAN_AGENT), TikTok 48h. Its result reaches the
  UI as `can_reply?`, which locks the composer.
- **`Reauthorizable`** counts auth errors. Past a threshold it marks the channel
  disconnected, emails admins and shows a UI banner.
- **Echoes** are replies sent from the native app (Meta Business Suite, the
  WhatsApp Business app). They are stored as outgoing messages with
  `external_echo: true` and excluded from human response metrics.

**Copy:**
- External account id unique per channel.
- Identity keyed by (connected account, platform user id).
- Return 200 fast and process async.
- Write the external id back after send.
- One window service feeding the composer.
- Skip echoes sent by our own app (Meta `app_id`).
- Show reauthorization state in the UI.
- Forward-only delivery status (sent → delivered → read, or failed).

**Avoid** (each of these is a real problem in Chatwoot today):

- **No channel registry.** `SendReplyJob::CHANNEL_SERVICES`, the window service,
  source-id generation, reauth handlers, the jbuilder branches and frontend
  `INBOX_TYPES` all need editing per channel. Helpin already has this problem
  with its twelve lists, so phase 0 fixes it first.
- **Instagram implemented twice.** It exists both via FB Page and via Instagram
  Business Login, with duplicated builders, a `type` discriminator in jsonb, and
  duplicate webhooks when both are connected. Pick one path.
- **Dedupe varies by channel.** `messages.source_id` has no unique constraint
  and no inbox scope. Each channel invents its own guard: find-then-create, a
  Redis NX lock, a mutex, or a 2-second delay on echoes. We use one partial
  unique index instead.
- **Thread data in untyped jsonb** (`additional_attributes.type`, `tweet_id`).
- **Reauth state kept only in Redis**, and inbound messages are dropped while
  reauth is required.
- **No outbound retries or 429 handling.** A failed send stays failed until an
  agent resends it.
- **Uneven webhook security.** Telegram relies only on the token in the URL.
- **Unscoped delivery-status lookup.** Messenger's lookup isn't scoped to the
  inbox (`ContactInbox.find_by(source_id:)`).
- **Reactions ignored** on WhatsApp and Instagram.
- **Lazy Instagram token refresh with no background job**, so an idle inbox
  expires silently.
- **X is dead code.** It uses the v1.1 Account Activity API and OAuth 1.0a,
  is hidden from the "add inbox" list, and is not removed.
- **Twilio webhooks are not signature-checked.** `Twilio::CallbackController`
  enqueues the raw params without validating `X-Twilio-Signature`, so anyone who
  knows the URL can inject inbound SMS. SMS opt-out (STOP) is left entirely to
  the provider; Chatwoot has no RCS channel.

The [Slack/Mattermost bridge plan](2026-08-18-two-way-support-chat-integrations-plan.md)
already designed a similar Helpin-native shape: a provider interface, a
deduplicated inbound queue, a semantic-key outbox, and external identities. It
was never built. This plan uses the same conventions so the two can share code
if both ship, but it does not depend on the bridge.

## Decisions

1. **The connected account is the channel; the mailbox stays the queue.** A
   connected Facebook Page, IG account, WhatsApp number, Telegram bot or Twilio
   Messaging Service becomes one `support_channel_accounts` row attached to a mailbox.
   Mailboxes stay channel-agnostic, the same way `support_email_routes` attaches
   email today.
2. **One table for all providers, not one per provider.** Use one row with
   `provider`, `external_account_id`, encrypted tokens and a non-secret
   `config jsonb`. Chatwoot's per-channel tables suit Rails STI; in Go with GORM,
   one table plus a typed config struct per provider is less code and follows
   the shape of `crm_email_accounts`/`git_credentials`.
3. **A provider adapter registry is the only place that branches on
   provider.** Services, repositories and the frontend read capabilities; they
   never switch on the provider name.
4. **Use one Meta path per product.** Use Instagram Business Login (graph.instagram.com)
   for Instagram, not IG through a Facebook Page. Use the Messenger Platform for
   Facebook Pages. Use the WhatsApp Cloud API with embedded signup, with no BSPs
   (360dialog, Twilio) in v1.
5. **DMs only in v1.** Out of scope: public comments, tweets and mentions,
   Instagram comment threads, group chats, business-initiated campaigns, voice.
6. **Reauth state lives in the DB** (`status = reauthorization_required`).
   Inbound webhooks are still accepted and processed while reauth is required;
   only outbound sends pause.
7. **Customer-visible sender.** Messages go out as the Page, account, number or
   bot. Agent names are not prefixed in v1.
8. **Contacts are linked by hand in v1.** A teammate links an identity to a
   CRM contact. Phone-number auto-linking needs its own decision first:
   normalisation, ambiguous matches, trust. `crm_contacts.phone` is not unique.
9. **Every customer-visible reply goes through one delivery path.** That covers
   human replies, AI replies and AI follow-ups. Internal notes and system events
   never reach a channel.

## How channels fit together

Every channel shares the policy layer and the conversation model. Only the
transport layer differs.

| Layer | What it holds | Shared by |
|---|---|---|
| Policy (phase 0 registry) | label, icon, AI eligibility, triage, SLA, reports, CSAT, composer limits, `transport` | every channel |
| Conversation model | `support_conversations.channel`, `support_messages.via_channel`, mailboxes, assignment, SLA, AI | every channel (true today) |
| Transport | how messages enter and leave | per `transport` kind, below |

Each registry entry declares one `transport`:

| `transport` | Channels | Inbound | Outbound |
|---|---|---|---|
| `chat` | widget | widget session APIs | websocket, after the message is saved |
| `portal` | portal | portal request APIs | portal page, with email notices |
| `email` | email | Postmark → `support_inbound_jobs` (`kind=email`) | email fallback and explicit-email paths |
| `provider` | Telegram, Messenger, Instagram, WhatsApp, SMS/RCS, later Slack-as-customer | webhook → `support_inbound_jobs` (`kind=channel_event`) | `EnqueueTx` → `support_channel_outbox` → adapter |
| `none` | internal, api | n/a | no customer delivery beyond what exists today |

`chat` and `portal` are separate because a portal conversation has no live chat
session; existing code treats them differently. Phase 0 ships `chat`, `portal`,
`email` and `none`; phase 1 adds `provider` with its first channel.

Widget, portal and email keep their own transports:
- Widget and portal are first-party surfaces with sessions, identity
  verification and presence. There is no external provider to call.
- Email already has a durable pipeline with threading, fallback, bounces and
  reconciliation. Moving it behind the adapter gains users nothing and risks a
  working channel.

`CreateConversationMessage` remains the single decision point. It reads the
conversation's `transport` from the registry, replacing today's inference from
`source`, `anonymous_id` and `merge_has_widget` in the composer and service.

Slack appears in two roles. They must not be confused:
- **Slack bridge** ([bridge plan](2026-08-18-two-way-support-chat-integrations-plan.md)):
  teammates read and reply from Slack. Slack is a teammate transport, so the
  conversation's `channel` stays widget, email and so on. It reuses this plan's
  patterns, but it is not a registry channel.
- **Slack as a customer channel** (customers in a shared Slack Connect
  channel): one more `provider` adapter. It is the bridge plan's reserved
  `customer_channel` mode, and it is not scheduled.

## Data model

All migrations go in `server/internal/dbmigrate/sql/`, are idempotent, and are
also registered in AutoMigrate.

### `support_channel_accounts`

| Column | Notes |
|---|---|
| `id`, `workspace_id` | |
| `provider` | CHECK in (`telegram`, `messenger`, `instagram`, `whatsapp`, `sms`) |
| `external_account_id` | Page id, IG user id, WA `phone_number_id`, Telegram bot id, Twilio Messaging Service SID |
| `display_name`, `handle`, `avatar_url` | Cached for settings and the inbox |
| `mailbox_id` | Nullable: NULL means the shared inbox |
| `access_token_encrypted`, `refresh_token_encrypted`, `token_expires_at` | AES-GCM via `internal/crypto`, new env key `CHANNEL_ENCRYPTION_KEY` |
| `webhook_secret_hash` | Telegram `secret_token`; unused for Meta, which signs with the app secret |
| `config jsonb` | Non-secret: WABA id, display phone, scopes, whether HUMAN_AGENT is approved |
| `status` | `active`, `reauthorization_required`, `disabled` |
| `auth_error_count`, `last_error_code`, `last_error_message`, `last_event_at` | |
| `connected_by`, `disabled_at`, timestamps | Soft disconnect only; message provenance refers to the row |

Unique on (`provider`, `external_account_id`) where `disabled_at IS NULL`. This
is global, not per workspace, because a webhook arrives without a workspace and
must resolve to exactly one.

### `support_channel_identities`

This is the ContactInbox equivalent.

| Column | Notes |
|---|---|
| `id`, `workspace_id`, `channel_account_id` | |
| `external_user_id` | PSID, IGSID, `wa_id`, Telegram user id, E.164 phone (SMS and RCS share it) |
| `display_name`, `username`, `avatar_url`, `phone` | Latest profile snapshot |
| `crm_contact_id` | Nullable. Set only by an explicit teammate link in v1 |
| `last_inbound_at` | Drives the messaging window |
| `opted_out_at` | SMS/RCS STOP state; blocks sends until START |

Unique on (`channel_account_id`, `external_user_id`).

The CRM contact link stays optional. We do not create email-less CRM contacts
automatically. The unified person record is gap-list §6 #9 and is decided
separately.

### Changes to `support_conversations`

- `channel` gains the provider values, validated in Go through the registry.
  There is no CHECK constraint, because the ticket-create API accepts free-form
  sources (see phase 0).
- `channel_account_id uuid`, `channel_identity_id uuid`.
- The conversation list shows the customer from `customer_name`, which is
  filled from the identity snapshot. That way list queries need no join.
- Conversation reuse: an inbound message goes to the latest non-closed
  conversation for (`channel_identity_id`), otherwise a new one is created. This
  matches the widget, and DM platforms have no thread id to key on.

### Changes to `support_messages`

These follow bridge plan §6.8:

- `channel_account_id uuid`, `external_message_key text`.
- Partial unique index on (`channel_account_id`, `external_message_key`) where
  both are non-null. This dedupes messages, not events.
- The key is built by the adapter and must be unique within the connected
  account:

  | Provider | `external_message_key` |
  |---|---|
  | Telegram | `<chat_id>:<message_id>` (`message_id` is only unique per chat) |
  | Messenger, Instagram | `mid` |
  | WhatsApp | `wamid` |
  | SMS/RCS | Twilio `MessageSid` |

- `external_echo boolean NOT NULL DEFAULT false`. These are replies sent from
  the native app; they are excluded from first-response and SLA metrics.
- `via_channel` (`varchar(20)`) takes the provider value, or `sms`/`rcs` per
  message. No type change is needed.
- Delivery status comes from the outbox row (below), the same way email status
  comes from `support_email_logs`.

### Inbound: reuse `support_inbound_jobs`

The table is already generic: `kind`, lease, attempts, privacy triggers,
tombstones. Add `kind = 'channel_event'` with
`id = uuid.NewSHA1("helpin:inbound:<provider>:<channel_account_id>:<event key>")`.
A provider retry of the same event becomes a primary-key no-op.

Event keys are separate from message keys, so successive updates to one
message all survive:

| Event | Event key |
|---|---|
| Telegram, any update | `update_id` (per bot, so scoped by account) |
| Meta / WhatsApp message, echo | `msg:<external_message_key>` |
| Meta read or delivery watermark (no message id) | `wm:<kind>:<psid>:<watermark>` |
| WhatsApp status | `st:<wamid>:<status>` |
| Twilio status callback | `st:<MessageSid>:<MessageStatus>` |
| Reaction | `rx:<external_message_key>:<user>:<emoji or "removed">:<timestamp>` |
| Edit / unsend | `ed:<external_message_key>:<edit timestamp>` / `del:<external_message_key>` |

Add two nullable columns:
- `channel_account_id`, so pre-conversation events can be found;
- `subject_key`, a SHA-256 of `<channel_account_id>:<external_user_id>`, so
  privacy cleanup can find events whose conversation doesn't exist yet.

Status events can arrive out of order. The outbox applies them forward-only
(below), so a late `sent` after `delivered` is a no-op.

### Outbound: `support_channel_outbox`

The shape follows `customer_io_outbox` and bridge plan §6.7:

- `semantic_key` (unique) is `message:<support_message_id>`. It holds the
  message id, the channel account id, the identity id and an immutable
  `payload`.
- `external_message_key`, `attempts`, `next_attempt_at`, the lease columns,
  `last_error_code` and `last_error`.
- **Dispatch state** (`pending → dispatching → accepted | rejected | uncertain |
  cancelled`) is kept apart from **receipt state** (`none → sent → delivered →
  read`, or `failed`). Dispatch moves only by the rules below. Receipts only
  move forward.

## Provider adapter

Package `server/internal/channels`, one sub-package per provider:

```go
type Adapter interface {
    Provider() string
    Capabilities() Capabilities
    VerifyWebhook(r *http.Request, body []byte, acct *Account) error
    ParseWebhook(body []byte) ([]Event, error)          // normalised: message, echo, delivery, read, deleted, reaction
    Send(ctx context.Context, acct *Account, to Identity, msg Outgoing) (externalID string, err error)
    FetchProfile(ctx context.Context, acct *Account, externalUserID string) (*Profile, error)
    RefreshToken(ctx context.Context, acct *Account) error // no-op where tokens don't expire
}

type Capabilities struct {
    ReplyWindow       time.Duration // 0 = unlimited
    HumanAgentWindow  time.Duration // Meta HUMAN_AGENT tag, humans only
    MaxTextLength     int
    Markdown          bool
    Attachments       []string      // image, video, audio, file
    MaxAttachmentSize int64
    ReadReceipts      bool
    Echoes            bool
}
```

`Send` returns one of four outcomes:

- `Accepted(externalKey)`.
- `NotSent`: the request provably did not execute. Examples: a 429 with
  retry-after, a connection refused before the request was written, or a
  provider error that guarantees no execution. These are safe to retry.
- `Rejected(code)`: a definite refusal. Examples: outside the window, blocked,
  opted out (Twilio `21610`), an invalid recipient, or an auth error. Auth
  errors (Meta 190 and equivalents) also increment `auth_error_count`, and at 2
  set `reauthorization_required`.
- `Unknown`: a timeout after the request was written, a 5xx, or a dropped
  connection. The provider may have sent the message.

None of the v1 providers (Telegram, Meta, WhatsApp, Twilio Messages) accepts a
client idempotency key on send. A blind retry after `Unknown` can therefore
send the customer the message twice.

`internal/channels/registry.go` holds the one map. Phase 0's channel registry
(below) reads `Capabilities` from it for these channels.

## Pipelines

**Inbound**

1. A webhook hits the public block in `router/router.go`:
   - `POST /api/webhooks/channels/meta` for Messenger, Instagram and WhatsApp,
     which share one app.
   - `POST /api/webhooks/channels/{provider}/{channel_account_id}` for Telegram
     and Twilio. Their payloads don't name the receiving account. The account
     id in the path is not a secret, and the bot token never goes in a URL.

   Verification, before any database write:
   - Meta: `X-Hub-Signature-256` HMAC with the app secret, using a
     constant-time compare.
   - Telegram: the `X-Telegram-Bot-Api-Secret-Token` header, compared against
     that account's `webhook_secret_hash`. The secret is a random value set
     through `setWebhook secret_token`. On reconnect, generate a new secret
     and call `setWebhook` again, so the old one stops working.
   - Twilio: `X-Twilio-Signature`, HMAC-SHA1 of the full URL plus sorted form
     params with that account's Auth Token. Chatwoot skips this.
2. Meta: resolve the account from the page id, IG id or `phone_number_id` in
   the payload. Path-routed providers use the path id, and the account must be
   active and match the provider. Unknown or disabled accounts get a 200 and a
   log line; never retry-loop the provider.
3. Insert one `support_inbound_jobs` row per event and return 200.
4. `StartInboundWorker` claims `channel_event` jobs and runs
   `ProcessChannelEvent`:
   - take a transaction advisory lock on `subject_key`. That covers a first
     message where no identity exists yet, and replaces Chatwoot's Redis mutex;
   - find or create the identity (fetch the profile on first sight, falling back
     to the handle or "Instagram user");
   - find or create the conversation;
   - insert the message. On conflict with
     (`channel_account_id`, `external_message_key`), do nothing.
   - Echoes whose `app_id` is ours are skipped, because our sends are recorded
     by the outbox. Other echoes are stored as outgoing with
     `external_echo = true`.
   - Status, read and watermark events update the outbox receipt state, scoped
     by `channel_account_id`.
     - If the outbox row is found by correlation id (below), apply the update.
     - If the target is not yet known, the job retries with backoff for up to
       ~10 minutes. This is the callback-before-send-result race. It is then
       dropped with a log line.
5. Side effects run in the same order as `createInboundConversationFromRoute`:
   notification, support event, websocket publish, triage, then the AI request.
   Pull that sequence into one shared helper rather than copying it a third
   time.

**Outbound**

1. **One enqueue function.** `channeldelivery.EnqueueTx(tx, msg, conv)` inserts
   the outbox row in the same transaction that creates the message. It returns
   without enqueuing when any of these hold:
   - the conversation has no channel identity;
   - the message is internal;
   - `message_type` is not a customer-visible reply.
2. **Reply producers that call `EnqueueTx`:**
   - `CreateConversationMessage`, which covers teammate and API replies.
     Translated pending sends (`support_pending_send.go`) also go through it.
   - `createSupportAIReply` (`support_ai_publish.go`). It writes through
     `messageRepo.Create`, or through `processingRepo.CreateReply` /
     `CreateProgressReply`, which settle AI processing in the same
     transaction. `EnqueueTx` must run inside both. Otherwise an AI answer
     shows in Helpin but never reaches the customer. AI progress notes are not
     enqueued.
   - AI follow-ups (`support_ai_follow_up.go`), which use the same AI publish
     path.

   Test each producer: the message and its outbox row exist together, or
   neither exists. Also test that an internal note never creates an outbox row.
3. **Undo window.** `next_attempt_at` is set to the message's existing
   `cancellable_until` when it has one. Undo cancels the outbox row while it
   is still `pending`.
4. **Re-check policy at dispatch.** `StartChannelOutboxWorker` claims a row
   with a lease and re-checks everything immediately before calling the
   provider, on every attempt:
   - the account is active and not reauth-required;
   - the identity is not opted out;
   - the message and conversation are not deleted or anonymised;
   - the reply window is still open.

   A failed check sets the row to `cancelled` with the reason. A check at queue
   time alone goes stale.
5. **Results:**
   - `Accepted`: store `external_message_key` on the row and the message.
   - `NotSent`: retry with exponential backoff and jitter, at most 8 attempts,
     honouring `retry-after`.
   - `Rejected`: set the row to `rejected` with the code. The composer shows
     "Not delivered: <reason> · Retry".
   - `Unknown`: set the row to `uncertain` and do **not** retry automatically.
     The bubble shows "Delivery not confirmed". If a receipt later arrives via
     correlation, the row moves to `accepted`. Otherwise the teammate can
     choose "Send again", which warns that the customer may get it twice.
     Rows left `uncertain` for 24h without a receipt stay that way. They are
     visible in a per-account failure count on the Channels settings page.
6. **Correlation ids**, so an early receipt finds its row before the send
   result is saved:
   - Twilio: the outbox id goes in the `StatusCallback` URL query.
   - WhatsApp: `biz_opaque_callback_data`, which status webhooks echo back.
   - Meta and Telegram: no per-send correlation. Meta delivery and read are
     time watermarks, so they need none. Telegram has no receipts.
7. **Retry** re-queues the same row from `rejected` or `uncertain` back to
   `pending`. That is a dispatch transition, separate from receipts.
8. **Merged conversations.** `mergedDeliveryConversation` only projects email
   recipients and widget presence today. In v1, a reply in a merged group goes
   to the channel identity of the most recent customer message in the group.
   The composer names that target. Sending to several channel identities at
   once is out of scope.

**Messaging window** (`service/support_channel_window.go`)

- `CanReply(conv, senderKind)` returns `{allowed, reason, expiresAt}` from
  `identity.last_inbound_at` and the adapter capabilities:
  - WhatsApp, Messenger and IG: 24h.
  - Meta: 7 days if the HUMAN_AGENT tag permission is approved and the sender is
    a human. The adapter adds `messaging_type: MESSAGE_TAG, tag: HUMAN_AGENT`
    only in that case.
  - Telegram and SMS: unlimited, but SMS is blocked while the identity is
    opted out.
  - RCS: unlimited for customer care replies. Replies within 24h of the
    customer's message are billed as one user-initiated conversation outside
    the US; this affects cost, not whether we may send.
  - Ad-originated Meta chats (7-day window since 2026-09-28) are out of v1
    scope; they are treated as 24h.
- AI replies are only allowed in the standard window, never under HUMAN_AGENT.
- The result is exposed on the conversation payload. `ReplyComposer` disables
  sending and shows "Reply window closed <time ago>". For WhatsApp the
  composer offers "Send template" once templates exist (phase 4).

## Phases

Each phase ships on its own and leaves the product working if later phases
never land.

### Phase 0: channel registry, with no new channels (built)

Built on `feat/support-channel-registry`. Behaviour is unchanged by
construction: a behaviour-matrix test was written and passed against the old
code first, then kept passing through the refactor.

**Audit findings that shaped the scope:**

- `POST /api/support/tickets` copies a client-supplied `source` into both
  `channel` and `source` (`CreateConversation`). Production rows can hold
  arbitrary values, and API callers can still send them. **There is no CHECK
  constraint and no backfill.** Either would risk failed deploys or broken
  callers. Validation stays in Go, and unknown values keep each consumer's
  existing fallback.
- `source` is provenance and is relied on as such:
  - Intercom imports keep `source = 'intercom'`.
  - Portal SQL requires `channel = 'portal' AND source = 'portal'`.
  - Portal email eligibility requires both fields to be `email`.

  Rewriting either field would change those queries. Both fields stay as they
  are.
- **Triage still reads `source` before `channel`** (`supportConversationChannel`).
  For an imported email conversation that resolves to `intercom`, which uses the
  widget triage toggle and matches no rule channel. Switching to `channel`
  would change routing for imported conversations that get new activity. That
  is a product decision, so phase 0 pins today's result and leaves it open.
- Single-channel checks (`conv.Channel == "portal"`, the widget email-collection
  prompt, the delayed widget team reply) are channel behaviour, not channel
  lists. They stay as they are.
- **Setup evidence** (`repository/setup.go`) counts resolved conversations on
  `widget`, `email` and `api` only. Portal is already left out. Whether a new
  channel counts toward activation is a product choice, so phase 1 decides it
  there.

**What changed:**

- `server/internal/model/support_channels.go` holds one registry entry per
  channel, with exact-key lookup. Each entry records `Transport` (`chat`,
  `portal`, `email`, `none`), `AIMode`, triage toggle, whether triage rules may
  target the channel, whether SLA policies may, and whether the channel skips
  the default inbox.
- These consumers now read the registry and keep all their other inputs:
  - `SupportAIChannelEnabled`;
  - `triageChannelEnabled` and `validSupportTriageChannel`;
  - SLA policy channel validation;
  - `shouldUseDefaultMailboxForChannel`;
  - the three "has a chat session" checks: the reply guard in
    `CreateConversationMessage`, merged-group detail, and
    `mergedDeliveryConversation`;
  - AI support checks for conversations and messages
    (`supportAIConversationSupported`, `supportAIMessageEligible`);
  - AI follow-up candidate channels (`repository.SupportFollowUpChannels`).

  Portal visibility, incoming-email precedence, mailbox AI overrides and
  machine-mail suppression in `support_ai_channels.go` are untouched.
- `frontend/src/lib/supportChannels.ts` mirrors the registry, typed as
  `Record<TicketSource, …>`, so adding a `TicketSource` without an entry fails
  to compile. It replaces:
  - the conversation-row `CHANNEL_META`;
  - message-bubble `SOURCE_LABELS`;
  - report channel labels;
  - SLA channel labels and options;
  - the `source === 'widget'` checks in the composer, receipts and delivery
    state;
  - the AI-control channel check.

  The per-surface labels ("Live chat" in the list, "Chat" in reports,
  "Internal" on message badges) are kept exactly.

**Tests:**

- `service/support_channel_behavior_test.go`: AI eligibility across
  selections and portal drafts, triage toggle and rule channels, SLA
  acceptance, default inbox and chat session. Each covers known, unknown,
  imported and unnormalised values.
- `model/support_channels_test.go` and `frontend/src/lib/__tests__/supportChannels.test.ts`:
  registry integrity and exact matching.

**Adding a channel later** means one Go registry entry, one frontend entry and
the `TicketSource` union. Behaviour specific to that channel lives with its
transport.

### Phase 1: foundation plus Telegram (10–14 days)

Telegram goes first because it needs no app review, no business verification
and no OAuth. Connecting it means pasting a bot token. That lets one provider
prove the shared delivery architecture end to end, and it works for self-hosters
with no setup on our side. It says nothing about which provider customers want
next.

- Migrations: accounts, identities, conversation and message columns, inbound
  job columns, outbox.
- `internal/channels` interface, registry and Telegram adapter:
  - `getMe` validates the token;
  - `setWebhook` to the per-account URL with a random `secret_token`;
    `deleteWebhook` on disconnect;
  - text, photos and documents, with downloads into `support_attachments`.
- Inbound worker kind, `EnqueueTx` wired into all three reply producers, outbox
  worker with dispatch-time re-checks and the uncertain state, and the window
  service (unlimited for Telegram).
- Privacy cleanup for identities and queues (see Security and privacy).
- Settings → Channels page: list the connected accounts, show status, the last
  event and uncertain/failed counts, and offer connect, disconnect, reconnect
  and change mailbox. Copy the `CRMEmailAccountsSettingsPage` pattern. Only
  settings managers can connect or disconnect.
- Inbox: channel icon and label from the registry, an identity card with
  "Link to contact", delivery state on outgoing bubbles, and Retry or Send
  again.
- Tests:
  - Adapter parse fixtures from real payloads.
  - Two chats sharing a `message_id` → two messages.
  - Duplicate webhook delivery → one message.
  - Two bots connected; a wrong secret, an unknown account and a reconnect
    with the old secret are all rejected.
  - Concurrent first messages → one conversation.
  - Human reply, AI reply and AI follow-up each create message and outbox
    together. An internal note creates no outbox row.
  - 429 is retried; 403 ("bot blocked") is rejected.
  - A timeout after the request is written → uncertain, with no automatic
    resend.
  - The account is disconnected while a row is queued → cancelled at
    dispatch.
  - Contact deletion while inbound and outbound work is in flight → no
    identity resurrected and nothing sent.
  - Disconnect stops sends but keeps history.

### Phase 2: Messenger plus Instagram (8–10 days, plus Meta app review lead time)

These share one Meta app, one webhook endpoint and one Graph client.

- Configuration: Meta app id and secret in env/config. Self-hosters must create
  their own Meta app; document this as a how-to.
- Messenger: Facebook Login for Business, then list the user's Pages, then
  store the page token (long-lived) and subscribe `subscribed_apps` to
  `messages, message_echoes, message_deliveries, message_reads,
  messaging_postbacks`.
- Instagram: Instagram Business Login OAuth with a 60-day token. A daily
  Temporal or leader-elected job refreshes tokens that expire within 10 days,
  so idle accounts don't expire silently.
- Events handled:
  - text and attachments;
  - echoes;
  - delivery and read watermarks;
  - IG unsend, as a soft delete with placeholder text;
  - story replies and mentions, shown as a link card with "Story reply";
  - reactions, shown as an annotation on the target message rather than a new
    message (Chatwoot drops them).
- Messaging window plus HUMAN_AGENT. The tag needs a separate Meta permission
  review, so without it the window stays 24h.
- Request app review for `pages_messaging`, `instagram_business_manage_messages`
  and `human_agent` **as soon as Meta is picked as the next provider** (ideally
  during phase 1). Review takes weeks
  and needs a screencast of the working flow.

### Phase 3: WhatsApp Cloud API (8–10 days)

- Embedded signup, then exchange the token, register the number, subscribe the
  WABA webhook and store `phone_number_id` and the WABA id.
- Events: inbound messages (text, media, location, contacts), status webhooks
  (sent, delivered, read, failed with error code), and coexistence echoes
  (`smb_message_echoes`).
- Identity: `wa_id` plus phone. CRM linking is manual, as for every channel.
- Status correlation via `biz_opaque_callback_data` set to the outbox id.
- Per-number media lock: albums arrive as N webhooks, and the identity advisory
  lock covers this.
- Outside the 24h window the composer is locked. **Phase 4 is part of the
  usable WhatsApp release.** Ship phases 3 and 4 together, so teams have a
  defined way to resume a conversation after 24h.

### Phase 4: WhatsApp templates (4–5 days)

- Sync approved templates daily, and on demand from settings, into
  `support_channel_accounts.config` or a small `support_channel_templates`
  table if parameters need querying.
- Composer "Send template" picker with variable fill and a preview. Validate
  parameters against the template definition before queueing.
- No template authoring in Helpin; authors use WhatsApp Manager.

### Phase 5: SMS through Twilio (7–9 days, plus carrier registration lead time)

Intercom offers SMS: US toll-free numbers only, plus local numbers in Canada,
the UK and Australia, with numbers provisioned by Intercom
([Intercom SMS FAQ](https://www.intercom.com/help/en/articles/6311206-sms-faqs)).
Chatwoot offers it through Twilio and Bandwidth. SMS is the channel sales
conversations ask about after WhatsApp.

**Decision: bring your own Twilio account.** The customer connects their Twilio
account (Account SID, Auth Token and a Messaging Service). We do not provision
numbers or act as the sender of record. Reasons:

- US carrier registration (A2P 10DLC brand and campaign, or toll-free
  verification) is tied to the business that sends. Doing it on customers'
  behalf makes Helpin an ISV reseller in The Campaign Registry. That means
  paperwork, liability and billing we don't want.
- Self-hosters need the same path anyway.
- A customer with a 10DLC number can use it, which Intercom does not allow.
- Telnyx and Plivo are 40–60% cheaper per message
  ([comparison](https://startuptalky.com/7-best-twilio-alternatives-real-sms-pricing-compared/)).
  The adapter boundary allows a second SMS provider later, but v1 is Twilio
  only, because RCS (phase 6) also arrives through Twilio.

**Carrier rules the product has to respect** (US; other countries vary):

- Business SMS from a local number needs A2P 10DLC registration. That means a
  brand registration (about $4 one-time) and a campaign (about $10/month),
  1–3 weeks to approve
  ([Telnyx](https://telnyx.com/resources/what-is-10dlc),
  [TextBolt](https://textbolt.com/blog/10dlc-compliance/)). Toll-free numbers
  need toll-free verification per number, about 2–3 weeks. Unregistered
  traffic is filtered or blocked. Use case for Helpin customers: "Customer
  Care".
- STOP, START and HELP must work. Twilio Advanced Opt-Out handles the keywords
  at the provider and rejects later sends with error `21610`. Helpin still
  records `opted_out_at` from the inbound webhook (`OptOutType`) and from
  `21610`. The composer shows "Customer opted out of SMS" instead of letting
  agents send into a failure.
- No outbound-first messaging in v1. Replies only, which keeps us inside
  conversational customer care and away from TCPA marketing consent.

**Build:**

- Connect flow: validate credentials by fetching the Messaging Service. Set its
  inbound URL and status callback to Helpin. Show the numbers in the service
  and their registration status (`A2P` / toll-free verification) read from the
  Twilio API, so admins see why sends would be filtered.
- Identity: E.164 phone from `From`. CRM linking is manual.
- Status callbacks carry the outbox id in the `StatusCallback` URL.
- Inbound: `Body`, MMS media (`MediaUrlN`, US/Canada only; fetched with Basic
  auth into `support_attachments`), opt-out keywords.
- Outbound: plain text only. Markdown is stripped, links kept. The composer
  shows a segment counter: GSM-7 is 160 characters (153 per part when
  concatenated) and UCS-2 is 70 (67), because one emoji switches the whole
  message to UCS-2 and multiplies cost. Attachments go as MMS where supported.
- Delivery: status callbacks give `queued → sent → delivered | undelivered |
  failed` with error codes. There are no read receipts.
- AI replies: allowed. The prompt gets the plain-text and length constraint
  from `Capabilities.MaxTextLength`.

### Phase 6: RCS through Twilio (5–7 days, plus agent approval lead time)

RCS is the rich successor to SMS: branded, verified sender, read receipts,
typing, media, suggested replies. Its reach changed in the last two years:

- iOS 18 (September 2024) added RCS. Since iOS 18.1, business (RBM) messages
  reach iPhones on supported carriers. Business RCS to iPhones is live in the
  US, UK, Germany, France, Spain and South Korea as of September 2026
  ([Infobip](https://www.infobip.com/blog/apple-rcs),
  [Messente](https://messente.com/blog/state-of-rcs-business-messaging)).
  AT&T, Verizon and T-Mobile support it for Android and iOS.
- Senders need an RCS agent: brand name, logo and use case, verified by Google
  and approved per carrier. Vetting goes through the same TCR ecosystem as
  10DLC (about $75) and takes weeks
  ([Messente](https://messente.com/blog/state-of-rcs-business-messaging)).
- Billing: Google merged "Basic" and "Single" into "Non-conversational" from
  2026-02-18. Outside the US, a reply within 24h of a customer message is one
  user-initiated conversation. US traffic is billed per message by content
  (rich, rich media, suggested-action click)
  ([Google US billing model](https://developers.google.com/business-communications/rcs-business-messaging/carriers/billing/us-billing-model)).

**Decision: RCS is an upgrade of the SMS channel, not a separate channel.** The
customer links an approved RCS sender to the same Twilio Messaging Service.
Twilio then sends RCS when the handset supports it and falls back to SMS
automatically
([Twilio](https://www.twilio.com/docs/studio/tutorials/use-rcs-with-studio)).
Consequences:

- The same phone identity and the same conversation, whether a given message
  went as RCS or SMS. `support_messages.via_channel` records `rcs` or `sms`
  per message, from the webhook and status callback.
- The inbound sender may arrive as `rcs:+1…`. Normalise it to E.164 so it
  threads with SMS.
- Each send is decided by Twilio at send time. A customer whose last message
  went as RCS can get the next one as SMS (handset change, carrier, RCS
  offline). So the composer is fallback-aware:
  - text is always shown with the SMS segment count and cost hint;
  - media goes out as RCS media, or as MMS/link when Twilio falls back;
  - the bubble shows `rcs` or `sms` once the status callback reports which
    was used.

  Rich-only features stay deferred until we can attach SMS fallback content
  to them.
- No direct Google RBM API integration. Twilio already handles the RCS/SMS
  decision, and a direct RBM agent would duplicate carrier onboarding for each
  customer.

**Build:**

- Settings: show whether the Messaging Service has an RCS sender and its
  approval state.
- Inbound and outbound media through RCS. Read receipts move the outbox to
  `read`. Typing indicators are optional.
- Opt-out handling is shared with SMS.
- Deferred within RCS: rich cards, carousels and suggested replies. These
  become interesting once the AI agent can offer quick replies.

**Verify before starting phase 5 or 6** (things that change often):

- Twilio's current RCS countries, the webhook field that marks RCS versus SMS,
  and whether RCS needs a separate status callback.
- Current 10DLC and toll-free verification fees and timelines.
- Whether Intercom has added RCS. As of this writing their channel list does
  not include it, so shipping RCS would be ahead of Intercom.

### Deferred

- Public comments and mentions.
- X (Twitter) DMs. Intercom does not offer them and Chatwoot's X channel is
  dead. X's API is now pay per use (about $0.015 per DM sent, $0.01 per DM
  read; [pricing](https://postproxy.dev/blog/x-api-pricing-2026/)). The cost
  lands on whoever owns the developer app. Reconsider only if a customer asks.
- Instagram via FB Page. LINE, TikTok, Viber, Discord, voice.
- SMS providers other than Twilio; direct Google RBM integration.
- Outbound-first SMS/RCS and broadcasts (needs marketing consent handling).
- Business-initiated outbound and broadcasts.
- Ad-originated 7-day windows.
- Unified person record and contact merge across channels.
- Showing agent names to customers.
- Slack/Mattermost as a customer channel; that is the bridge plan's reserved
  `customer_channel` mode.

## Security and privacy

- **Tokens:** encrypted at rest, never serialised in API responses, and never
  logged. Error messages are sanitised before being stored in `last_error_*`.
- **Webhooks:** signature checked before any database write. Unknown accounts
  get a 200 and are dropped. Bodies are size-capped.
- **Privacy cleanup must be extended explicitly.** Existing flows do not cover
  the new tables:
  - `DeleteAnonymizingSupport` (`repository/crm_contact_privacy.go`) finds
    conversations by contact link or exact email.
  - The inbound privacy trigger only fires for jobs with a conversation.

  On contact deletion, in the same transaction:
  - delete the channel identities linked to the contact, or to its
    conversations;
  - clear the payloads of `channel_event` jobs matching those conversations or
    identities' `subject_key`;
  - cancel any `pending`, `dispatching` or `uncertain` outbox rows for them.

  Conversation anonymisation does the same for its identity, when no other
  conversation uses it.
- **No resurrection.** Completed inbound jobs keep their primary key with an
  empty payload (the existing tombstone behaviour), so a provider replay of an
  old event is a no-op and cannot recreate a deleted identity. A **new**
  message from the same person after deletion creates a fresh identity. That
  is intended.
- **Several identities per contact** are each deleted. An identity with no CRM
  contact is deleted through conversation deletion or anonymisation, or by the
  Meta data-deletion callback.
- **Outbound worker:** it re-checks deletion at dispatch, so a row that slipped
  past cleanup is cancelled, not sent.
- **Meta data deletion:** a Meta data-deletion callback endpoint is required
  for app review. It deletes the identities for that app-scoped user id.
- **Tenancy:** every lookup after account resolution is scoped by
  `workspace_id` and `channel_account_id`. This avoids Chatwoot's unscoped
  `ContactInbox.find_by(source_id:)`.

## Verification for every phase

- `go test ./...` for the touched packages, plus repository tests against
  Postgres for the unique indexes and the advisory-lock conversation creation.
- Recorded-payload fixtures per provider in `internal/channels/<p>/testdata`.
- A manual end-to-end run on staging with a real bot, Page, IG account or test
  WABA number: send, receive, echo, read receipt, window expiry, disconnect and
  reconnect.
- Frontend: Vitest for the composer window and delivery-state logic.
- Update `docs/` with an engineering guide (`docs/support-channels.md`) and a
  self-host setup how-to per provider.

## Effort

These estimates are provisional. They now include the delivery lifecycle,
privacy and compatibility work the review found missing, but they will move
once phase 0's audit is done.

| Phase | Estimate |
|---|---|
| 0 Registry and behaviour matrix | built |
| 1 Foundation and Telegram | 10–14 days |
| 2 Messenger and Instagram | 8–10 days, plus Meta review (weeks, in parallel) |
| 3+4 WhatsApp with templates | 12–15 days |
| 5 SMS (Twilio) | 7–9 days, plus each customer's 10DLC or toll-free registration |
| 6 RCS (Twilio) | 5–7 days, plus RCS agent approval |

## Sequence

1. Phase 0, the registry, with compatibility checks. It has value on its own.
2. Phase 1, one complete Telegram integration, as proof of the shared delivery
   architecture.
3. The next provider is chosen from customer demand, with Messenger/Instagram
   and WhatsApp as the candidates. If Meta is likely, start Meta app review
   and business verification during phase 1, because review takes weeks.
4. WhatsApp ships only with templates.
5. SMS and RCS are scheduled separately. Their sender registration, fallback
   behaviour and running costs make them a project of their own.

## Review response

[Review](2026-11-02-combined-inbox-review.md) findings and where this plan
addresses them:

| Finding | Change |
|---|---|
| 1. Message vs event identity | `external_message_key` per provider (Telegram `chat_id:message_id`); separate event-key table; forward-only receipts |
| 2. Telegram account resolution | Per-account webhook path plus per-account secret; tests for multiple bots, wrong secret and reconnect |
| 3. AI publication path | Decision 9; `EnqueueTx` inside `createSupportAIReply`'s processing transactions; per-producer tests |
| 4. Uncertain sends | `NotSent`/`Rejected`/`Unknown` outcomes; `uncertain` state with no automatic resend; dispatch-time re-checks; correlation ids; undo, pending-send and merge rules |
| 5. Privacy cleanup | Explicit identity, job and outbox cleanup; tombstones; manual CRM linking only |
| 6. Registry compatibility | Behaviour matrix before and after; registry keeps today's AI distinctions; transport kept apart from provenance |
| Intercom Telegram claim | Corrected: Telegram is listed, depth unverified |
| RCS fallback | Composer stays SMS-safe; RCS or SMS shown per message after the fact |

## Open questions

1. When is this scheduled relative to other support work?
2. For hosted deployments, who owns the Meta developer app?
3. Phone-based CRM auto-linking: decide normalisation, ambiguity and trust
   rules before any channel links automatically.
