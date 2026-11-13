# Telegram channel (combined inbox phase 1)

Date: 2026-11-03, revised 2026-11-13. Branch: `feat/telegram-channel`. Status:
plan, not started.

This is the implementation plan for phase 1 of the
[combined inbox plan](2026-11-02-combined-inbox-plan.md): the shared provider
foundation (accounts, identities, inbound jobs, outbox, adapter registry) plus
the first provider, Telegram. It is written against `develop` at `5f904d67c`,
after phase 0 (channel registry, #782) and the workspace-delete fix (#783).
Shared Community scope. Nothing here touches billing.

## Outcome

- An admin pastes a bot token in Settings → Channels.
- From then on, anyone who messages the bot privately lands in the support inbox
  as a normal conversation.
- Teammates, the AI agent and AI follow-ups reply from Helpin, and the reply
  arrives in Telegram.
- Delivery state shows on each message: sending, sent, not delivered, or not
  confirmed.
- Disconnecting the bot stops sending and keeps history.
- Deleting the person's data deletes their Telegram identity too.

## Decisions

Settled with the owner on 2026-11-03:

| Question | Decision |
|---|---|
| AI reply setting | Telegram follows the existing "chat" setting (`ai_reply_channels`) |
| Triage toggle | Widget toggle |
| SLA policies | Telegram is selectable |
| Setup evidence | Resolved Telegram conversations count toward activation |
| CRM contact | Linked by hand only; never matched automatically |

Added on 2026-11-13 after comparing with Intercom, whose Telegram channel is
native: bot token, private chats only, one conversation per user, several bots
per workspace.

| Question | Decision |
|---|---|
| AI per bot | The chat setting stays the master switch. Each bot also has an "AI replies" switch, on by default, so a team can run AI on the widget but not on Telegram (Intercom deploys its AI per channel). |
| Routing | A triage rule wins, then the bot's default mailbox, then the workspace default inbox. Intercom has no per-account default (its users have asked for one); Chatwoot makes each account its own inbox. |
| Reopen or new | A message within 5 days of resolving reopens the conversation; after that it starts a new one. Intercom uses 5 days for Facebook, Instagram and SMS. |
| Formatting | Bold, italic, code, quotes and links are sent and received, as Intercom does. Plain text is the fallback. |
| Message types | Customers can send text, photos, videos, documents, audio, voice notes, video notes and animations. Stickers and other unsupported types show as a note instead of being dropped. |

Made in this plan, from reading the code:

1. **Outbound rows are written by a database trigger on `support_messages`,
   not by wiring `EnqueueTx` into each producer.** Every reply already goes
   through `SupportMessageRepository.Create` or `WithTx(tx).Create`, but AI
   follow-ups insert messages directly in three places
   (`support_ai_follow_up_sequence.go:69`,
   `support_ai_follow_up_outcome.go:153` and `:287`), and a future producer
   could bypass a Go hook. An `AFTER INSERT` trigger runs in the producer's own
   transaction, so the message and its delivery row commit together or not at
   all. `webhook_message_inserted` (`202610250008_support_webhooks.sql`) is the
   precedent, and the trigger reuses its definition of a customer-visible
   reply.
2. **Inbound events reuse `support_inbound_jobs`** with a new kind
   `channel_event` and `channel_attachment`. `StartInboundWorker` currently
   sends every non-email kind to the email attachment processor, so dispatch
   becomes an explicit switch.
3. **The token key falls back to the CRM key**, following
   `resolveWebhookEncryptionKey`. `CHANNEL_ENCRYPTION_KEY` is optional; with
   neither set, connecting a channel returns a clear "encryption not
   configured" error.
4. **Channel settings require `support.admin`**, like email routes and support
   webhooks.
5. **Delivery is ordered per Telegram chat.** The outbox claims only the oldest
   undelivered row per identity, the same idea as the pending-send claim
   (`support_pending_send.go:214`). Telegram also limits bots to about one
   message per second per chat.
6. **Formatting uses Telegram's HTML mode**, not MarkdownV2. Only `<`, `>` and
   `&` need escaping, where MarkdownV2 needs about 18 characters escaped
   depending on context, and a bad escape fails the whole send. Markdown is
   converted to the supported HTML subset: `b`, `i`, `u`, `s`, `code`, `pre`,
   `blockquote`, `a`. Anything else becomes plain text. If Telegram still
   answers "can't parse entities", the same message is resent once as plain
   text. Inbound, Telegram's entities (bold, links and so on) are converted to
   markdown, so the inbox renders them.
7. **The AI switch per bot rides on the existing mailbox AI policy.**
   `LoadSupportMailboxAIPolicy` (`repository/support_mailbox_ai.go`) already
   sets `conversation.MailboxAIRepliesDisabled`, and every AI gate reads it:
   `SupportAIReplyAllowed`, `SupportAIConversationBlocked`, follow-up
   eligibility, and the recheck before delayed replies. The loader also sets the
   flag when the conversation's bot has AI replies off. There are no new
   checks.
8. **Out of scope for v1:**
   - group chats and channels (only private chats). Groups are thread-based,
     like Slack and Discord; see the combined plan's notes on thread-based
     channels;
   - edited and deleted customer messages;
   - reactions;
   - CSAT on Telegram;
   - Telegram Business connections;
   - outbound-first messages (Intercom doesn't offer them on Telegram either);
   - quick replies and CSAT as inline keyboards;
   - a per-bot reopen window setting (5 days for everyone in v1);
   - triage-rule conditions on a specific bot, and inbox views filtered by bot.
     The bot's default mailbox covers per-bot routing until a workspace runs
     several bots that need finer rules.

## Slices

Each slice is one PR, merges on its own, and leaves the product working. The
Channels settings page ships in slice 3. Until then nothing is reachable by
users.

### Slice 1: schema, models and registry (2 days)

Migration `2026110300NN_support_channels.sql`. It is idempotent and SQL-only,
with no AutoMigrate registration, following the `customer_io_outbox`
precedent:

- **`support_channel_accounts`:**
  - `id`, `workspace_id`;
  - `provider` (CHECK `telegram`);
  - `external_account_id` (the bot's user id), `display_name`, `handle`;
  - `mailbox_id` (the default mailbox; see Routing);
  - `ai_replies_disabled boolean NOT NULL DEFAULT false`;
  - `token_encrypted`, `webhook_secret_hash`;
  - `status` (`active`, `reauthorization_required`, `disabled`);
  - `auth_error_count`, `last_error_code`, `last_error_message`,
    `last_event_at`;
  - `connected_by`, `disabled_at`, timestamps.

  The unique index on (`provider`, `external_account_id`) only covers rows
  where `disabled_at IS NULL`. That way one bot can't be live in two
  workspaces.
- **`support_channel_identities`:**
  - `id`, `workspace_id`, `channel_account_id`;
  - `external_user_id` (the Telegram user id, also the private chat id);
  - `display_name`, `username`, `avatar_url`;
  - `crm_contact_id`;
  - `last_inbound_at`, timestamps.

  Unique on (`channel_account_id`, `external_user_id`).
- **`support_conversations`:** add `channel_account_id` and
  `channel_identity_id`, both nullable, with an index on
  (`channel_identity_id`, `status`).
- **`support_messages`:**
  - add `channel_account_id` and `external_message_key`, with a partial unique
    index on both where neither is null;
  - add `external_echo boolean NOT NULL DEFAULT false`.
- **`support_inbound_jobs`:** add `channel_account_id` and `subject_key`, plus
  an index on `subject_key`.
- **`support_channel_outbox`:**
  - `id`, `workspace_id`, `channel_account_id`, `channel_identity_id`,
    `conversation_id`, `message_id`;
  - `semantic_key` (unique; `message:<id>`);
  - `dispatch_status` (`pending`, `dispatching`, `accepted`, `rejected`,
    `uncertain`, `cancelled`);
  - `receipt_status` (`none`, `sent`, `delivered`, `read`, `failed`);
  - `attempts`, `next_attempt_at`, `claim_token`, `lease_expires_at`;
  - `external_message_key`, `last_error_code`, `last_error`, timestamps.

  It gets the due-work partial index from `customer_io_outbox`.

Registry, in `model/support_channels.go` and `frontend/src/lib/supportChannels.ts`:

- New transport `provider`.
- New entry `telegram`: `Transport: provider`, `AIMode: chat`,
  `Triage: widget`, `TriageRuleChannel: true`, `SLAChannel: true`.
- Add `telegram` to `TicketSource`, with a Telegram icon and label in the
  frontend entry.
- New registry flag `SetupEvidence`. `repository/setup.go:272` builds its
  channel list from it instead of the literal `('widget','email','api')`. It is
  true for widget, email, api and telegram, so existing results don't change.
- Extend the phase 0 behaviour matrix with a `telegram` row for every consumer,
  plus a test that `setup.go`'s list is unchanged for existing channels.

Models and repositories: `SupportChannelAccount`, `SupportChannelIdentity` and
`SupportChannelOutbox` in `model/support_channel.go`, with repositories in
`repository/support_channel_*.go`.

Tests:
- the migration applies twice cleanly;
- the unique indexes hold;
- the registry matrix covers telegram;
- `go run ./cmd/migrate validate` passes.

### Slice 2: adapter boundary and Telegram client (2 days)

`server/internal/channels`:

- `Adapter` interface:
  - `Provider()`, `Capabilities()`;
  - `VerifyWebhook`, `ParseWebhook` (into normalised `Event`s);
  - `Send`, returning `Accepted(key)`, `NotSent`, `Rejected(code)` or
    `Unknown`;
  - `FetchProfile`, `DownloadFile`.
- `registry.go` maps provider to adapter.

`server/internal/channels/telegram`, a small client with no new dependency
(`net/http` plus JSON):

- **Calls:** `getMe`, `setWebhook` (with `secret_token`, `allowed_updates:
  ["message"]` and `drop_pending_updates: false`), `deleteWebhook`,
  `sendMessage`, `sendPhoto`, `sendDocument`, `getFile`, plus the file
  download.
- **Parse:**
  - only private-chat `message` updates;
  - message key `chat_id:message_id`, event key `update_id`;
  - text and captions, with entities converted to markdown;
  - the largest photo size, documents, videos, audio, voice notes, video notes
    and animations, each as a file to download;
  - stickers, locations, contacts, polls and anything else become an
    `unsupported` event carrying the type name;
  - anything else becomes an ignored event, logged once.
- **Render:** markdown to Telegram HTML (decision 6), with a plain-text
  fallback.
- **Capabilities:** text up to 4096 characters (captions 1024), HTML subset,
  attachments up to 20 MB for downloads (the Bot API limit), image and file
  types, no read receipts.
- **Error classification:**
  - `Rejected`: 400 bad request, and 403 when the bot is blocked or the user is
    deactivated;
  - `Rejected`, and also an auth error: 401;
  - `NotSent` (honouring `parameters.retry_after`): 429, or a connection
    refused before the request was written;
  - `Unknown`: 5xx, a timeout after the request was written, or a dropped
    connection.
- **Outbound HTTP** goes through `netguard.DialContext`. The token appears only
  in the request path and is never logged; client errors are scrubbed of it.

Tests: parse fixtures recorded from real Bot API payloads in
`channels/telegram/testdata`, covering:
- private text;
- photo with caption;
- document;
- a group message (ignored);
- an edited message (ignored);
- two chats sharing a `message_id`;
- error classification for each HTTP status, using an `httptest` server.

### Slice 3: connect, webhook intake and Settings → Channels (2–3 days)

Backend:

- **`SupportChannelService`:**
  - **Connect:**
    1. Validate the token with `getMe`.
    2. Refuse if that bot is already live in any workspace.
    3. Encrypt the token: AES-GCM via `crypto.EncryptStringWithAAD`, with AAD
       `channel-token:<account id>`.
    4. Generate a 32-byte webhook secret, store its SHA-256 hash, and call
       `setWebhook` to `<public API base>/api/webhooks/channels/telegram/<account id>`.
       On failure, nothing is saved.
  - **Disconnect:** `deleteWebhook` on a best-effort basis, then set
    `disabled_at` and cancel pending outbox rows. History stays.
  - **Reconnect:** a new secret, and `setWebhook` again. The old secret stops
    working immediately.
  - **Change mailbox.**
  - The public API origin uses the same fallback chain as external agents'
    upload links (`PUBLIC_API_BASE_URL`, then `CLI_PUBLIC_BASE_URL`, then
    `APP_BASE_URL`); there is no new setting. If the origin is not HTTPS,
    connect fails with a clear message, because Telegram requires HTTPS.
- **Routes:**
  - `GET/POST/PATCH/DELETE /api/support/channels`, behind
    `requirePerm(PermSupportAdmin)`.
  - Public `POST /api/webhooks/channels/telegram/{accountId}`, in the no-JWT
    block next to the Postmark route.
- **Webhook handler**, following `webhook_postmark.go`:
  1. Load the account. Unknown or disabled accounts get a 200 and nothing is
     stored.
  2. Compare `X-Telegram-Bot-Api-Secret-Token` against the stored hash with a
     constant-time compare. A mismatch gets a 401.
  3. Read at most 1 MB of body and parse it.
  4. Insert one `support_inbound_jobs` row per event: kind `channel_event`, id
     `inboundStableID("telegram:<account>:<update_id>")`, with `workspace_id`,
     `channel_account_id` and `subject_key = sha256(<account>:<user id>)`.
  5. Return 200 only after the commit; return 503 if the insert fails, so
     Telegram retries.
  6. Update `last_event_at`.

Frontend:
- `pages/settings/SupportChannelsSettingsPage.tsx`, route
  `settings/support-channels.tsx`, and a section entry in `settingsSections.tsx`
  (group Support, `requiredPermission: 'support.admin'`). It also goes in
  `workspaceSurface.ts` and the settings-sections test list.
- `useSupportChannels` hooks and `supportChannelsService`, following the
  support-webhooks files.
- The page lists connected bots with handle, status, last event, mailbox, and
  failed and not-confirmed counts. Actions: connect (paste token), reconnect,
  change mailbox, turn "AI replies" on or off, disconnect with confirmation.

Tests:
- **Handler:**
  - wrong, missing and old secrets get 401;
  - an unknown account gets 200 and no row;
  - a duplicate `update_id` gives one row;
  - an oversized body is rejected;
  - a database failure gives 503.
- **Service:**
  - connect with an invalid token stores nothing;
  - a bot already live elsewhere is refused;
  - reconnect rotates the secret;
  - disconnect cancels pending rows;
  - a missing encryption key gives a clear error.
- **Frontend:** a Vitest test for the page states.

### Slice 4: inbound processing (2–3 days)

`StartInboundWorker` dispatch becomes an explicit switch:
- `email` and `attachment`, exactly as today;
- `channel_event` → `SupportChannelService.ProcessEvent`;
- `channel_attachment` → `ProcessAttachment`;
- unknown kinds → `Finish` with an error, so they fail visibly instead of
  being treated as email attachments.

`main.go` starts one `channel_event` and one `channel_attachment` poller next
to the existing ones.

`ProcessEvent`, in one transaction:
1. Take an advisory transaction lock on `subject_key`, then
   `LockContactAnonymization`.
2. Find or create the identity. On first sight, fetch the profile (name and
   username) outside the lock; it falls back to "Telegram user". A known
   identity gets its name refreshed if it changed.
3. Find the latest conversation for the identity that isn't `closed` or
   `spam`. A resolved conversation counts only if it was resolved within the
   last 5 days (`telegramReopenWindow`); otherwise a new conversation starts.
   - **A resolved or waiting conversation reopens** through `createWidgetReply`
     (`support_widget_reply.go:14`), which is already channel-agnostic.
   - **Otherwise, create a conversation:**
     - Channel and source are `telegram`; `channel_account_id` and
       `channel_identity_id` are set.
     - `customer_name` comes from the identity.
     - `crm_contact_id` comes from the identity's link, if any.
     - The mailbox is the account's mailbox; if that's null,
       `maybeApplyMailboxRoutingForChannel(ctx, ws, nil, true, "telegram")`.
4. Insert the message:
   - customer sender, `via_channel = telegram`;
   - `external_message_key` set;
   - on conflict, do nothing and treat it as already processed.
5. For each photo or document, create a placeholder `support_attachments` row
   (status processing) and a `channel_attachment` job sharing its stable id,
   as `enqueueInboundAttachments` does.
6. Set `identity.last_inbound_at` and back-fill the job's `conversation_id` and
   `message_id`, so the existing privacy trigger covers it.

An `unsupported` event inserts a customer message with the note "Sent a
<type> on Telegram, which Helpin can't display", so the teammate knows
something arrived. It doesn't trigger AI.

After commit, run the side effects in the widget's order:
1. websocket publish (conversation created, or message);
2. `enrichSupportMessageLinksAsync`;
3. `recordSupportEvent` (`Channel: "telegram"`);
4. `ProcessSupportCustomerReplyNotification`;
5. post-message automation.

`LoadSupportMailboxAIPolicy` also sets `MailboxAIRepliesDisabled` when the
conversation's bot has `ai_replies_disabled` (decision 7).

`runWidgetPostMessageAutomation` hard-codes the "chat" AI mode. It becomes
`runPostMessageAutomation(ctx, conv, msg)`, which reads the AI mode from the
registry. The widget passes the same values as today, and the behaviour matrix
pins it. There is no wider refactor of the email path in this phase.

`ProcessAttachment`:
- `getFile`, then a streamed download, capped at 20 MB and 60 seconds;
- `StoreInboundEmailAttachment` gains a bytes variant
  (`StoreInboundAttachmentBytes`), sharing the type allow-list, sniffing and
  S3 key layout.
- A rejected file finishes as `rejected`, as email attachments do.

Tests, in the service and on the Postgres fixture:
- first message creates identity, conversation and message;
- a message 3 days after resolving reopens the conversation; one 6 days after
  starts a new one;
- a sticker becomes a visible note and requests no AI;
- a bot with AI replies off gets no AI request, even with the chat setting on;
- a message with bold and a link arrives as markdown;
- a second message reuses the open conversation;
- a resolved conversation reopens;
- a duplicate update gives one message;
- two chats with the same `message_id` give two messages;
- concurrent first messages from one user give one conversation;
- a group message is ignored;
- a photo becomes an attachment, and an oversized file is rejected;
- AI is requested only when the chat AI setting allows it;
- triage runs with the widget toggle;
- an unknown job kind fails rather than being treated as an email attachment.

### Slice 5: outbound delivery (3 days)

**Trigger** (migration `2026110300NN_support_channel_outbox_trigger.sql`):
`AFTER INSERT ON support_messages`, for each row. It inserts an outbox row when
all of these hold:
- the conversation has a `channel_identity_id` and its account is active;
- the message is customer-visible: sender `user`, `agent` or `ai`;
  `message_type = 'reply'`; not internal; not deleted; not an echo. This is the
  same predicate as `webhook_message_inserted`, plus the echo check.
- `next_attempt_at` is `cancellable_until` when it is set, so the undo window
  works; otherwise `now()`.

Undo, meaning deleting a message inside its window, cancels the row: an
`AFTER UPDATE OF deleted_at` trigger marks pending rows `cancelled`. A
translated pending send inserts its message only when released, so it needs
nothing extra.

**`CreateConversationMessage`:**
- **Delivery guard (L2260):** a `provider` conversation accepts `chat_only` (or
  an empty delivery mode), which is mapped to delivery through its channel.
  `email_only` and `chat_and_email` are rejected when the conversation has no
  customer email.
- No outbox code is needed here, because the trigger covers it.

**`SupportChannelOutboxWorker`**, started in `main.go` like the Customer.io
worker (SKIP LOCKED, no leader):
- **Claim:** due rows where no earlier row for the same identity is still
  `pending` or `dispatching`; lease 2 minutes, batch 25.
- **Re-check before each send:**
  - the account is active;
  - the message isn't deleted;
  - the conversation isn't anonymized or deleted.

  If any check fails, the row is cancelled with a reason.
- **Render:** markdown to Telegram HTML, with the plain-text fallback on a
  parse error. Text over 4096 characters
  is split at paragraph boundaries into ordered parts with keys
  `message:<id>:<n>`. Attachments go as `sendPhoto` or `sendDocument`, with the
  first 1024 characters as caption and the rest as a following text message.
- **Results:**
  - `Accepted`: store the key on the row and the message; receipt `sent`.
  - `NotSent`: back off from 1 second (doubling) with jitter, capped at 1 hour,
    honouring `retry_after`; at most 8 attempts, then `rejected` with code
    `retries_exhausted`.
  - `Rejected`: store the code. A 401 also counts toward
    `reauthorization_required` after 2.
  - `Unknown`: `uncertain`, with no automatic retry.
- **Every status change** publishes `publishMessageUpdated`, so bubbles update
  live.

**API:**
- Message payloads gain `channel_delivery: {status, error_code, retryable}`,
  read from the outbox row.
- `POST /support/inbox/conversations/{id}/messages/{msgId}/channel-retry` sets
  a `rejected` or `uncertain` row back to `pending` (requires `PermSupportEdit`).

**Frontend:**
- **Sidebar:** the conversation shows which bot it came from (its handle), next
  to the channel.
- **Composer:** a `provider` conversation shows a single fixed delivery option
  ("Telegram") instead of `ReplyDeliverySelector`, and sends `chat_only`. It
  shows no "chat unavailable" message.
- **Delivery state:** `getMessageDelivery` gains a channel delivery kind that
  reads `channel_delivery`:
  - sending, then sent;
  - "Not delivered: <reason> · Retry";
  - "Delivery not confirmed · Send again", with a duplicate warning.

  `MessageDeliveryStatus` shows the Telegram icon.
- `helpers.ts` stays untouched, because support-mobile compiles it.

Tests:
- **Trigger, on the Postgres fixture:**
  - a teammate reply, an AI reply via `CreateReply`, and an AI follow-up insert
    each create exactly one outbox row in the same transaction;
  - an internal note, a system message, an echo, and a message in a widget
    conversation create none;
  - an undo inside the window cancels the row.
- **Worker:**
  - bold, code and a link arrive as Telegram HTML; a parse error resends once
    as plain text;
  - 429 retries after `retry_after`;
  - 403 is rejected;
  - a timeout after the request is written becomes uncertain and is not resent;
  - per-identity order holds when the first send is retried;
  - a disconnect while queued cancels the row;
  - long text splits in order.
- **Frontend:** the composer for a Telegram conversation, and the delivery
  states.

### Slice 6: identity linking and privacy (1–2 days)

- **Linking:** `updateConversationCRMContact` (`support_inbox.go:4577`) also
  sets `support_channel_identities.crm_contact_id` when the conversation has a
  `channel_identity_id`. Later conversations from that person start linked.
  Unlinking clears it.
- **Contact deletion:** `DeleteAnonymizingSupport` finds conversations by
  `channel_identity_id` for identities linked to the contact, then follows its
  existing steps. In the same transaction it also:
  - deletes those identities;
  - clears the payloads of `channel_event` and `channel_attachment` jobs
    matching their `subject_key`;
  - cancels the outbox rows of the affected conversations.
- **Telegram-only person, never linked to a contact:** deleting their
  conversation (`SupportConversationRepository.Delete`) also deletes the
  identity once no other conversation references it, clears its pending jobs
  and cancels its outbox rows.
- **Replay safety:** completed jobs keep their primary key with an empty
  payload, so a retried `update_id` stays a no-op. A new message after deletion
  creates a new identity, which is intended.
- **Workspace delete** is already covered by the #783 sweep. The new tables all
  have `workspace_id`.

Tests:
- link and unlink, and a later conversation inheriting the link;
- contact deletion removes identities, clears jobs and cancels the outbox while
  inbound and outbound work is in flight, with nothing sent afterwards;
- conversation deletion removes an unlinked identity;
- a replayed update after deletion creates nothing.

### Slice 7: docs and end-to-end check (1 day)

- `docs/support-channels.md`: an engineering guide covering tables, the
  inbound and outbound flow, retry and uncertain states, and how to add a
  provider.
- A Community how-to: create a bot with @BotFather, paste the token, and the
  HTTPS `PUBLIC_API_BASE_URL` requirement.
- `CHANNEL_ENCRYPTION_KEY` in `server/.env.example`.
- Update the plan index and the combined inbox plan's phase 1 "as built" notes.
- **Manual run on staging with a real bot:**
  - connect;
  - first message, AI reply, teammate reply, photo both ways;
  - block the bot, then reply (shows not delivered);
  - disconnect and reconnect;
  - delete the contact.

## Effort

| Slice | Estimate |
|---|---|
| 1 Schema, models, registry | 2 days |
| 2 Adapter and Telegram client | 2 days |
| 3 Connect, webhook, settings | 2–3 days |
| 4 Inbound processing | 3 days |
| 5 Outbound delivery | 3–4 days |
| 6 Linking and privacy | 1–2 days |
| 7 Docs and E2E | 1 day |
| **Total** | **14–18 days** |

That is up from 10–14 days in the combined plan. The difference is the
explicit privacy path for unlinked identities, attachments in both directions,
splitting long messages, and the changes from the Intercom comparison
(formatting, more message types, the reopen window, the AI switch per bot).

## Verification for every slice

- `go test ./...` and `go test -tags ee ./...` for touched packages.
- The Postgres fixture tests (`*_postgres_test.go`) on a migrated disposable
  database.
- `scripts/check-community-backend.sh`, and for frontend slices
  `scripts/check-community-frontend.sh`, the support-mobile typecheck, and the
  unused-code check on the touched files.
- The phase 0 behaviour matrix still passes unchanged for existing channels.

## Risks

- **The trigger is business logic in SQL.** Mitigation: it reuses the webhook
  trigger's predicate, and slice 5 tests every producer against it on
  Postgres.
- **Telegram has no idempotency key on send.** A crash after Telegram accepts
  a message but before we save the result produces an `uncertain` row, never
  an automatic resend. Teammates see "not confirmed" and decide.
- **One bot per workspace in practice.** The schema allows several, but a bot
  can only have one webhook URL. Moving a bot to another workspace means
  disconnecting it first.
