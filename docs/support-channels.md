# Support messaging channels

This guide is for engineers changing how Helpin receives and answers support
conversations from messaging apps. It explains the channel registry, the
provider tables, the inbound and outbound flows, and the steps for adding a
provider. Telegram is the only provider today. To connect a bot on a Community
installation, see [Telegram in the Community configuration](community/configuration.md#telegram).

## Channels and transports

Every conversation has a `channel` (`widget`, `email`, `portal`, `telegram`,
and others). The registry in `server/internal/model/support_channels.go`, with
its frontend copy in `frontend/src/lib/supportChannels.ts`, says how each
channel behaves: its transport, whether AI may answer it, which triage toggle
applies, and whether it counts as setup evidence. Add behavior to the registry,
not to scattered `channel == "..."` checks.

Telegram uses the `provider` transport: replies go out through a provider
adapter rather than the chat widget, the portal, or email. The composer offers
only "Telegram only" for these conversations, and AI follows the workspace's
chat AI setting plus the bot's own **AI replies** switch.

## Tables

All tables are created by versioned SQL (`202611130001_support_channels.sql`
and `202611130002_support_channel_outbox_trigger.sql`).

| Table | Holds |
| --- | --- |
| `support_channel_accounts` | A connected bot: provider, bot id and handle, encrypted token, webhook secret hash, default inbox, AI switch, status (`active`, `reauthorization_required`, `disabled`). |
| `support_channel_identities` | One customer of one account: provider user id, display name, username, and an optional `crm_contact_id`. |
| `support_channel_outbox` | One row per reply to deliver: dispatch status, attempts, parts sent, provider message key, last error. |

Conversations carry `channel_account_id` and `channel_identity_id`. Messages
carry `channel_account_id` and `external_message_key` (`chat_id:message_id`
for Telegram), which a unique index uses to drop provider retries.

Bot tokens are encrypted with AES-GCM using `CHANNEL_ENCRYPTION_KEY`, or
`CRM_ENCRYPTION_KEY` when it is unset, bound to the account id.

## Receiving messages

1. Telegram posts each update to
   `POST /api/webhooks/channels/telegram/{accountId}`. The handler checks the
   `X-Telegram-Bot-Api-Secret-Token` header against the stored hash, parses the
   update, stores a `channel_event` job with an id derived from the update id,
   and answers 200. A repeated update maps to the same job id and is ignored.
2. A worker claims the job and, under an advisory lock for that customer,
   upserts the identity and finds their conversation: an open one, or one
   resolved within the last five days, which reopens. Otherwise it creates a
   conversation in the triage rule's inbox, the bot's default inbox, or the
   workspace default inbox, in that order.
3. The message is saved with its provider key. Files become attachment
   placeholders with `channel_attachment` download jobs; files over 20 MB are
   refused and shown as such.
4. After commit, the usual customer-message work runs: realtime events,
   notifications, triage, and the AI agent.

Only new messages in private chats from people are stored. Group messages,
edits, bot messages, and the `/start` sent by Telegram's Start button are
ignored. Stickers, locations, polls and similar
types become a visible note saying Helpin cannot display them.

## Sending replies

A database trigger on `support_messages` queues an outbox row for every
customer-visible reply (teammate, AI, or agent) in a Telegram conversation, in
the same transaction that saves it. Internal notes, imported messages,
email-only replies, and messages that came from Telegram are not queued. In a
merged conversation, the reply goes to the person behind the latest Telegram
message in it. A reply inside its undo window waits until the window ends, and
undoing it cancels the row.

A worker in the API process claims due rows every two seconds. It sends at
most one reply per customer at a time, in order, and checks the account,
message, conversation and identity again right before sending. Text is
rendered to Telegram's HTML subset and split to fit the 4,096-character limit
(1,024 for a caption). If Telegram rejects the formatting, the same part is
sent as plain text. Parts already sent are recorded, so a retry resumes after
them.

| Outcome | Row status | Inbox shows |
| --- | --- | --- |
| Accepted | `accepted` | Sent to Telegram |
| Rate limited, or a network error before the request was written | `pending` with backoff (1 s doubling, up to 1 hour, honoring Telegram's `retry_after`; after 8 attempts it becomes `rejected`) | Sending |
| Rejected (blocked bot, bad request, revoked token) | `rejected` | Not delivered, with **Retry** |
| Unknown (Telegram 5xx, or a timeout or error after the request was sent) | `uncertain` | Telegram did not confirm delivery, with **Send again** after a warning about duplicates |
| Bot disconnected, reply deleted, or conversation erased | `cancelled` | Not sent |

Unconfirmed replies are never resent automatically. Two rejected tokens in a
row mark the bot `reauthorization_required`; an admin replaces the token in
**Settings → Channels**.

## Contacts and privacy

Telegram provides no email or phone, so Helpin never matches a Telegram
customer to a CRM contact automatically. Linking a Telegram conversation to a
contact by hand links the identity, and that person's later conversations start
linked and appear on the contact. Unlinking clears it.

Deleting a contact erases all conversations of its Telegram identities, deletes
the identities and their queued replies, clears the payloads of their inbound
jobs, and rewrites stored message keys so they no longer contain the Telegram
user id. Deleting the only conversation of a person who is not linked to a
contact does the same for that person. A provider retry of an old update stays
a no-op; a new message after deletion starts a new identity.

## Add a provider

1. Implement `channels.Adapter` (`server/internal/channels/channels.go`) in a
   package under `server/internal/channels/`: token validation, webhook
   registration, webhook parsing into `channels.Event`, rendering into parts,
   sending one part with an outcome, and file downloads.
2. Register the adapter in `cmd/api/main.go`.
3. Add the channel to both registries with the `provider` transport and the
   matching AI and triage behavior, and add its rows to
   `TestSupportChannelBehaviorMatrix`.
4. Extend the Settings → Channels connect form for the provider.
5. Cover parsing and rendering with recorded fixtures, and the flows with the
   PostgreSQL integration tests.

## Test locally

- Unit tests: `go test -tags ee ./internal/channels/... ./internal/service/ -run Channel`.
- PostgreSQL integration tests need a migrated disposable database:
  `SUPPORT_OPS_TEST_DATABASE_URL=... go test -tags 'ee integration' ./internal/service/ -run 'Channel|Outbox'`.
- `TELEGRAM_API_BASE_URL` points the adapter at a fake Bot API and lifts the
  HTTPS requirement for the webhook URL. Leave it unset in production.
