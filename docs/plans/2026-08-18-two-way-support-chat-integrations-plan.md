# Two-way Slack and Mattermost support bridge proposal

This unimplemented August 2026 proposal describes a customer-support bridge for contributors evaluating Slack and Mattermost integration work. It is a design and estimate, not installation instructions or a statement that either provider is connected.

## Source review — 2026-09-18

- Repository searches found none of the proposed `support_chat_integrations` tables/models, `chatbridge`/`slackapi`/`mattermostapi` packages, `chat-integrations` routes, `CreateExternalConversationMessage`, or `SUPPORT_CHAT_BRIDGE_ENABLED` configuration. The named migration and environment variables below are proposed additions.
- The existing [SupportChatService](../../server/internal/service/support_chat.go) runs the Echo Agent Runtime conversation lifecycle. Its name does not indicate an implemented Slack/Mattermost bridge. The [support message service](../../server/internal/service/support_inbox.go) remains the integration point to assess for customer delivery semantics.
- [Support attachments](../../server/internal/service/support_attachment.go) currently allow 100 MiB (displayed as 100 MB), replacing the 10 MB assumption below. A future provider adapter must reconcile provider limits and the current Helpin limit; the proposal does not establish an existing provider-download path.
- [Customer.io outbox](../../server/internal/service/customer_io_outbox.go) and [PostgreSQL leadership](../../server/internal/coordination/postgres_leader.go) implementations exist as reuse candidates. Their existence does not implement bridge atomicity, delivery reconciliation, listener recovery, or the exactly-once acceptance criteria.
- The freely replying channel-member policy and customer-visible identity rules remain proposed product decisions. Provider scopes, API details, effort estimates, external references and live-provider behavior were not revalidated in this source review. Validate them before implementation; no provider messages or connections were created.

## Original proposal

**Date:** 2026-08-18  
**Primary area:** Support inbox  
**Initial providers:** Slack and Mattermost  
**Estimated production-v1 effort:** 42-55 engineering days  
**Estimated elapsed time:** 8-11 weeks for one engineer, or 5-7 weeks for two engineers working in parallel

## 1. Objective

Build a provider-neutral support bridge that posts Helpin customer conversations into linked Slack or Mattermost channels and lets authorized channel members reply from the provider thread. Replies must be persisted in Helpin and delivered to the customer through the conversation's existing customer channel.

The first release follows the Crisp operator-bridge model:

1. A customer starts or continues a conversation through the Helpin widget or email.
2. Helpin creates one root post in the linked Slack or Mattermost channel.
3. Later public conversation messages are mirrored as replies under that root.
4. A human reply in the provider thread becomes a public Helpin support message.
5. Helpin delivers that reply through the original customer channel and retains the complete conversation history.

The schema and provider interfaces reserve two other modes without implementing them in v1:

- `notification_only`: one-way event posts whose thread replies do not affect Helpin.
- `customer_channel`: conversations initiated by customers inside Slack, Slack Connect, Mattermost, or another provider.

## 2. Scope

### 2.1 Production v1

- Connect one or more Slack workspaces to a Helpin workspace through Slack OAuth.
- Connect one or more Mattermost servers through an administrator-provided server URL and bot access token.
- Map a Helpin support mailbox, including the shared inbox, to a provider channel.
- Allow one mailbox to post to multiple provider channels.
- Allow each external channel to have only one active Helpin mailbox mapping in v1.
- Create one provider thread per `(channel link, Helpin conversation)`.
- Mirror public customer, human, and AI replies after the thread exists.
- Never mirror internal notes.
- Convert provider thread replies into public Helpin messages.
- Resolve the customer delivery channel from the Helpin conversation:
  - widget conversation: publish to the widget and retain the existing email-fallback behavior;
  - email conversation: send through the existing support email delivery path;
  - API/internal conversation: persist and publish in Helpin, but do not invent an external customer destination.
- Preserve the provider responder's external ID, display name, username, and avatar snapshot.
- Support common attachments up to Helpin's existing 10 MB limit and MIME allowlist.
- Support resolve and reopen actions after the external user is linked to an active Helpin workspace member with `support.edit`.
- Provide retries, idempotency, loop prevention, connection health, structured logs, and admin-visible delivery failures.
- Support public and private Slack channels when the Helpin Slack app has been added to the channel.
- Support public and private Mattermost channels through a bot-authenticated WebSocket connection rather than Mattermost outgoing webhooks.

### 2.2 Explicitly deferred

- Creating Helpin conversations from arbitrary top-level Slack or Mattermost posts.
- Slack Connect customer identity and CRM contact creation.
- Direct-message support.
- Broadcasts and campaigns.
- Ticket forms or provider-native modals beyond account linking and resolve/reopen confirmation.
- Editing or deleting a provider reply after it has been imported.
- Propagating Helpin message edits/deletes to provider history.
- Emoji reaction synchronization.
- Typing indicators and read receipts.
- Mirroring internal notes.
- Provider-side assignment, priority, tags, or mailbox changes.
- Historic backfill of conversations that existed before a channel was linked.
- Microsoft Teams, Discord, Google Chat, or generic webhooks.

## 3. Product decisions required before implementation

### 3.1 Thread creation trigger

Recommended default: create the provider thread on the first public customer message after the conversation has been routed to a mailbox.

Alternative triggers that the schema can support later:

- conversation creation;
- AI-to-human handoff;
- priority or tag workflow;
- manual `Send to Slack/Mattermost` action.

Creating on the first message avoids empty threads and ensures mailbox routing is known.

### 3.2 Who may reply

Recommended v1 policy:

- Free-text replies from any non-bot member of an explicitly linked channel may be sent to the customer.
- The settings UI must warn that linking a channel grants its members customer-reply capability.
- Resolve/reopen and future administrative actions require a linked Helpin user with the matching Helpin permission.
- Every reply records an immutable external identity snapshot even when there is no Helpin account mapping.

The channel link stores a `reply_policy` so stricter workspaces can choose `linked_helpin_users` instead of `channel_members`.

### 3.3 Customer-visible sender identity

Recommended behavior:

- Persist the Slack/Mattermost name and avatar snapshot on the Helpin message.
- Show that identity to the customer using the existing teammate-message rendering.
- Do not silently attribute an unlinked external user to the integration installer or the bot.

### 3.4 AI messages

Recommended behavior: mirror public AI replies because they are part of the customer-visible history. Ignore AI typing/progress events and internal agent-run messages.

### 3.5 Multiple linked channels

Recommended behavior:

- A public Helpin message fans out to every active channel link for the conversation's mailbox.
- An external reply is not echoed back into its source thread.
- The reply is mirrored to other linked provider threads for the same Helpin conversation.

## 4. Existing capabilities to reuse

- `SupportInboxService.CreateConversationMessage` remains the only business path for public support replies.
- `SupportInboxService.UpdateConversationStatus` remains the canonical status-transition path.
- Widget WebSocket delivery, unread updates, AI/human takeover state, notification handling, email fallback, CRM association, and analytics continue to run from those service paths.
- `SupportAttachmentService` provides validated S3 storage, a 10 MB limit, MIME allowlisting, attachment hydration, and provider-download storage helpers.
- `crypto.EncryptStringWithAAD` and its matching decrypt helper protect provider credentials with workspace/integration-bound AAD.
- The Customer.io PostgreSQL outbox supplies the claim-token, lease, `SKIP LOCKED`, retry, and graceful-worker pattern.
- `coordination.PostgresLeader` supplies single-leader execution for Mattermost WebSocket listeners across multiple API pods.
- Existing support permissions are used:
  - `support.read` for listing connection health;
  - `support.admin` for connect, disconnect, channel linking, and retries;
  - `support.edit` for linked-user resolve/reopen actions.

Do not use `notification_deliveries` as the primary channel-post queue. That table belongs to per-user notification events, while a support thread is a shared conversation projection with its own lifecycle and inbound mapping.

## 5. Architecture

```text
Customer widget/email
        |
        v
SupportInboxService.CreateConversationMessage
        |
        +-- existing Helpin/WebSocket/email/notification side effects
        |
        +-- support_chat_outbox rows (one per active channel link)
                    |
                    v
          SupportChatOutboxWorker
                    |
          +---------+----------+
          |                    |
          v                    v
      Slack API          Mattermost REST API
          |                    |
          v                    v
      thread reply        thread reply

Slack Events API / Mattermost WebSocket `posted`
        |
        v
support_chat_inbound_events (deduplicated and acknowledged)
        |
        v
SupportChatInboundWorker
        |
        v
SupportInboxService.CreateExternalConversationMessage
        |
        +-- public Helpin message and normal customer delivery
        +-- outbox fanout to other linked threads, excluding the source
```

### 5.1 Provider boundary

Create a small provider interface in `server/internal/chatbridge`:

```go
type Provider interface {
    ValidateConnection(ctx context.Context, credential Credential) (*RemoteWorkspace, error)
    ListChannels(ctx context.Context, credential Credential, cursor string) (*ChannelPage, error)
    PostRoot(ctx context.Context, credential Credential, req PostRequest) (*PostedMessage, error)
    PostReply(ctx context.Context, credential Credential, req ReplyRequest) (*PostedMessage, error)
    UpdateRoot(ctx context.Context, credential Credential, req UpdateRequest) error
    DownloadFile(ctx context.Context, credential Credential, file RemoteFile) (*DownloadedFile, error)
}
```

Provider implementations normalize provider-specific IDs and errors. Service and repository packages must not branch on Slack or Mattermost response payloads.

Recommended packages:

- `server/internal/chatbridge`: provider-neutral types, formatting, retry classification, and credential boundary.
- `server/internal/slackapi`: Slack OAuth and Web API client.
- `server/internal/mattermostapi`: Mattermost REST and WebSocket client.

### 5.2 Delivery guarantees

- At-least-once delivery with provider-level idempotency enforced by Helpin semantic keys and external message mappings.
- The same inbound provider event must create at most one Helpin message.
- The same Helpin message must create at most one provider post per channel link.
- A worker crash after provider success but before database completion can cause an uncertain delivery. Before retrying, the worker should use the saved provider result when available; where the provider cannot accept an idempotency key, the operation records the response before marking the row delivered and provides an admin reconciliation action.
- Failed operations retry transient errors (`408`, `429`, `5xx`, network timeout) with exponential backoff and jitter. Authentication, permission, invalid-channel, and validation errors become terminal failures and mark the connection or channel link unhealthy when appropriate.

## 6. Database design

Add the models to startup AutoMigrate for development. Also add an idempotent versioned migration named `server/internal/dbmigrate/sql/202608180001_support_chat_bridge.sql` so environments with `RUN_AUTO_MIGRATE=false` receive the tables, check constraints, foreign keys, and partial indexes before application deployment.

### 6.1 `support_chat_integrations`

One installed provider workspace/server connection.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | `gen_random_uuid()` |
| `workspace_id` | UUID NOT NULL | FK `workspaces(id)`, indexed |
| `provider` | VARCHAR(20) NOT NULL | `slack` or `mattermost` |
| `external_tenant_id` | VARCHAR(255) NOT NULL | Slack team/enterprise identity or canonical Mattermost server identity |
| `external_tenant_name` | VARCHAR(255) | Display name |
| `server_url` | TEXT | Mattermost only; canonical origin without credentials/path |
| `connection_mode` | VARCHAR(30) NOT NULL | `oauth_events` or `bot_websocket` |
| `bot_user_id` | VARCHAR(255) | Used for loop prevention |
| `access_token_encrypted` | TEXT NOT NULL | AES-256-GCM; never serialized |
| `refresh_token_encrypted` | TEXT | Slack token rotation when enabled |
| `token_expires_at` | TIMESTAMPTZ | Nullable for non-expiring tokens |
| `scopes` | JSONB NOT NULL | Granted provider scopes |
| `configuration` | JSONB NOT NULL | Non-secret provider options |
| `status` | VARCHAR(30) NOT NULL | `active`, `degraded`, `reauthorization_required`, `disabled` |
| `last_connected_at` | TIMESTAMPTZ | Last successful validation/socket connection |
| `last_event_at` | TIMESTAMPTZ | Last accepted provider event |
| `last_error_code` | VARCHAR(80) | Sanitized stable code |
| `last_error_message` | TEXT | Sanitized; never token/body content |
| `connected_by` | UUID | FK `users(id)` |
| `disabled_at` | TIMESTAMPTZ | Soft disconnect; rows remain for audit/mappings |
| `created_at` / `updated_at` | TIMESTAMPTZ | Standard timestamps |

Constraints and indexes:

- check `provider IN ('slack', 'mattermost')`;
- check known connection/status values;
- unique active connection on `(workspace_id, provider, external_tenant_id)`;
- index `(workspace_id, status)`;
- never physically delete an integration referenced by message provenance.

### 6.2 `support_chat_oauth_states`

Short-lived Slack authorization state.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `state_hash` | VARCHAR(128) UNIQUE NOT NULL | Store only SHA-256 hash |
| `workspace_id` | UUID NOT NULL | OAuth target |
| `actor_user_id` | UUID NOT NULL | Initiating administrator |
| `provider` | VARCHAR(20) NOT NULL | `slack` in v1 |
| `return_path` | TEXT NOT NULL | Must be a validated local settings path |
| `expires_at` | TIMESTAMPTZ NOT NULL | Ten-minute TTL |
| `consumed_at` | TIMESTAMPTZ | Atomic one-time use |
| `created_at` | TIMESTAMPTZ | |

Add an index on `expires_at` and clean expired states daily. Follow the atomic consume pattern in `ExternalMCPOAuthState`.

### 6.3 `support_chat_channel_links`

Maps a provider channel to a Helpin support mailbox.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `workspace_id` | UUID NOT NULL | Tenant scoping |
| `integration_id` | UUID NOT NULL | FK integration |
| `mailbox_id` | UUID | NULL means shared inbox |
| `external_team_id` | VARCHAR(255) | Mattermost team ID when applicable |
| `external_channel_id` | VARCHAR(255) NOT NULL | Stable provider ID, never channel name |
| `external_channel_name` | VARCHAR(255) NOT NULL | Cached display name |
| `mode` | VARCHAR(30) NOT NULL | `support_bridge`; reserve other modes |
| `root_trigger` | VARCHAR(40) NOT NULL | Default `first_customer_message` |
| `reply_policy` | VARCHAR(40) NOT NULL | `channel_members` or `linked_helpin_users` |
| `sync_attachments` | BOOLEAN NOT NULL | Default true |
| `sync_status` | BOOLEAN NOT NULL | Default true |
| `active` | BOOLEAN NOT NULL | Kill switch |
| `last_error_code` / `last_error_message` | TEXT | Channel-specific failure |
| `created_by` | UUID | Administrator |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

Constraints and indexes:

- unique `(integration_id, external_channel_id)` in v1;
- index `(workspace_id, mailbox_id, active)` for delivery target resolution;
- check known modes, triggers, and reply policies;
- validate that `mailbox_id`, when set, belongs to the same workspace.

### 6.4 `support_chat_thread_mappings`

Durable mapping between a Helpin conversation and a provider root post.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `workspace_id` | UUID NOT NULL | Tenant scoping |
| `integration_id` | UUID NOT NULL | Provider connection |
| `channel_link_id` | UUID NOT NULL | Routing configuration |
| `conversation_id` | UUID NOT NULL | FK support conversation |
| `external_channel_id` | VARCHAR(255) NOT NULL | Denormalized stable lookup key |
| `external_root_message_id` | VARCHAR(255) NOT NULL | Slack root `ts` or Mattermost root post ID |
| `status` | VARCHAR(20) NOT NULL | `active`, `archived`, `failed` |
| `last_synced_at` | TIMESTAMPTZ | |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

Constraints and indexes:

- unique `(channel_link_id, conversation_id)`;
- unique `(integration_id, external_channel_id, external_root_message_id)`;
- index `(workspace_id, conversation_id, status)`.

### 6.5 `support_chat_external_identities`

Auditable provider user identity with an optional Helpin account mapping.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `workspace_id` | UUID NOT NULL | |
| `integration_id` | UUID NOT NULL | |
| `external_user_id` | VARCHAR(255) NOT NULL | Stable provider user ID |
| `username` | VARCHAR(255) | Provider username/handle |
| `display_name` | VARCHAR(255) NOT NULL | Latest snapshot |
| `avatar_url` | TEXT | Provider image URL snapshot |
| `user_kind` | VARCHAR(20) NOT NULL | `human`, `bot`, `app`, `unknown` |
| `helpin_user_id` | UUID | Explicit account link; never infer solely by display name |
| `last_seen_at` | TIMESTAMPTZ | |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

Constraints and indexes:

- unique `(integration_id, external_user_id)`;
- index `(workspace_id, helpin_user_id)`;
- reject bot/app replies before support-message creation.

### 6.6 `support_chat_inbound_events`

Durable, deduplicated inbound queue and audit record.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `workspace_id` | UUID NOT NULL | |
| `integration_id` | UUID NOT NULL | |
| `provider_event_id` | VARCHAR(255) NOT NULL | Slack `event_id`; deterministic Mattermost event key |
| `event_type` | VARCHAR(80) NOT NULL | Normalized/provider event type |
| `external_channel_id` | VARCHAR(255) | |
| `external_thread_id` | VARCHAR(255) | |
| `external_message_id` | VARCHAR(255) | |
| `external_user_id` | VARCHAR(255) | |
| `payload` | JSONB NOT NULL | Required processing fields; avoid unnecessary secrets/profile data |
| `status` | VARCHAR(20) NOT NULL | `pending`, `processing`, `processed`, `ignored`, `failed` |
| `attempts` | INTEGER NOT NULL | |
| `next_attempt_at` | TIMESTAMPTZ NOT NULL | |
| `claim_token` | UUID | Worker lease token |
| `claimed_at` / `lease_expires_at` | TIMESTAMPTZ | |
| `last_error` | TEXT | Truncated and sanitized |
| `received_at` / `processed_at` | TIMESTAMPTZ | |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

Constraints and indexes:

- unique `(integration_id, provider_event_id)`;
- due-work index `(status, next_attempt_at, lease_expires_at)`;
- channel/thread lookup index;
- check `attempts >= 0` and known statuses;
- purge or minimize payloads after the configured audit-retention period.

### 6.7 `support_chat_outbox`

Durable outbound provider operations.

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `semantic_key` | VARCHAR(500) UNIQUE NOT NULL | Prevents duplicate logical posts |
| `workspace_id` | UUID NOT NULL | |
| `integration_id` | UUID NOT NULL | |
| `channel_link_id` | UUID NOT NULL | |
| `thread_mapping_id` | UUID | Nullable until root creation succeeds |
| `conversation_id` | UUID NOT NULL | |
| `support_message_id` | UUID | Nullable for status/root operations |
| `operation` | VARCHAR(40) NOT NULL | `post_root`, `post_reply`, `update_root`, `status_reply` |
| `payload` | JSONB NOT NULL | Immutable event-time render input, no credentials |
| `status` | VARCHAR(20) NOT NULL | `pending`, `processing`, `delivered`, `failed`, `cancelled` |
| `attempts` | INTEGER NOT NULL | |
| `next_attempt_at` | TIMESTAMPTZ NOT NULL | |
| `claim_token` | UUID | |
| `claimed_at` / `lease_expires_at` | TIMESTAMPTZ | |
| `external_message_id` | VARCHAR(255) | Result for reconciliation |
| `result` | JSONB NOT NULL | Provider result/file IDs, no tokens |
| `last_error` | TEXT | Truncated/sanitized |
| `delivered_at` | TIMESTAMPTZ | |
| `created_at` / `updated_at` | TIMESTAMPTZ | |

Semantic key examples:

- `root:{channel_link_id}:{conversation_id}`
- `message:{channel_link_id}:{support_message_id}`
- `status:{channel_link_id}:{conversation_id}:{status}:{status_version}`

Add the same lease and due-work indexes used by the existing Customer.io outbox.

### 6.8 Changes to `support_messages`

Add nullable provenance columns:

| Column | Type | Purpose |
|---|---|---|
| `source_integration_id` | UUID | Integration from which the message originated |
| `source_external_identity_id` | UUID | External responder snapshot |
| `external_message_id` | VARCHAR(255) | Provider message/post ID |

Add a partial unique index on `(source_integration_id, external_message_id)` where both values are non-null.

Keep `SupportConversation.Channel` unchanged for the Crisp-style bridge. It remains the customer's channel (`widget`, `email`, `api`, or `internal`); Slack/Mattermost is an operator transport recorded on the message as `ViaChannel`.

## 7. Backend implementation

### 7.1 Models and repositories

Add:

- `server/internal/model/support_chat_integration.go`
- `server/internal/repository/support_chat_integration.go`
- `server/internal/repository/support_chat_channel_link.go`
- `server/internal/repository/support_chat_thread.go`
- `server/internal/repository/support_chat_identity.go`
- `server/internal/repository/support_chat_inbound_event.go`
- `server/internal/repository/support_chat_outbox.go`

Repository requirements:

- every query scoped by `workspace_id` where a workspace is known;
- transaction helpers for atomic message/outbox insertion;
- `FOR UPDATE SKIP LOCKED` for PostgreSQL worker claims;
- compare-and-set claim token for deliver/retry/fail operations;
- SQLite-compatible locking for unit tests, following the Customer.io repository pattern;
- no raw dynamic SQL derived from provider input.

### 7.2 Credential service

Add `SupportChatCredentialService`:

- require `SUPPORT_CHAT_INTEGRATION_ENCRYPTION_KEY` to be a 32-byte key when chat integrations are enabled;
- encrypt tokens with AAD containing workspace ID, integration ID, provider, and token kind;
- decrypt only immediately before a bounded provider call;
- never log, serialize, or place credentials in outbox/event payloads;
- support Slack refresh-token rotation without changing integration identity;
- clear encrypted credentials when disconnected, while retaining the integration audit row.

### 7.3 Bridge service

Add `SupportChatBridgeService` with these responsibilities:

- resolve active channel links from the conversation mailbox;
- decide whether a message qualifies for mirroring;
- create root or reply outbox rows;
- exclude the inbound source channel/thread to prevent loops;
- format immutable event-time payload snapshots;
- create/update thread mappings after provider success;
- enqueue status updates when configured;
- provide an idempotent reconciler that finds recent eligible Helpin messages missing expected semantic keys after a process crash.

Modify `SupportInboxService` through explicit dependency injection:

- add `SetSupportChatBridgeService`;
- enqueue bridge delivery after a public message has been persisted and attachments have been linked;
- call a repository method that persists the message and initial outbox rows transactionally where targets are already known;
- use the reconciler as the recovery mechanism for the widget first-message path and any legacy/non-transactional message insertion path;
- call the bridge for status transitions after the canonical status update succeeds.

Do not subscribe to generic WebSocket broadcasts. They are a presentation/fanout transport and can be duplicated across pods; they are not a transactional domain event source.

### 7.4 External reply service path

Add an internal method:

```go
func (s *SupportInboxService) CreateExternalConversationMessage(
    ctx context.Context,
    source ExternalSupportMessageSource,
    req model.CreateMessageRequest,
) (*model.SupportMessage, error)
```

It must:

- load the thread mapping and accessible conversation by workspace;
- verify the channel link is active and conversational;
- upsert the external identity;
- reject the provider bot, app messages, unsupported subtypes, and edited/deleted events;
- select customer delivery channels from the conversation instead of trusting provider input;
- call the normal message-creation logic with `sender_type=user`, external display/avatar snapshots, and provenance fields;
- set `via_channel` to `slack` or `mattermost`;
- mark the inbound event processed only after the Helpin message commit succeeds;
- let the bridge fan out to other linked threads while excluding the source.

Refactor common message creation into a private method rather than duplicating notification, ownership, flow-state, email, WebSocket, and analytics side effects.

### 7.5 Outbound worker

Add `SupportChatOutboxWorker`, started and stopped in `server/cmd/api/main.go` like the Customer.io outbox worker.

- Poll every 1-2 seconds for a bounded batch.
- Lease rows for five minutes.
- Use provider-specific rate-limit headers and `Retry-After` where present.
- Limit attempts to ten, with a one-hour maximum backoff.
- Mark integrations `reauthorization_required` on token revocation/401.
- Mark channel links unhealthy on channel-not-found, bot-not-in-channel, or permission errors.
- Record external message IDs before final completion.
- Publish an existing workspace WebSocket update when delivery health changes so settings can refresh.

### 7.6 Inbound worker

Add `SupportChatInboundWorker` using the same claim/lease pattern.

- Normalize event payloads before business processing.
- Ignore bot messages, root posts unrelated to known mappings, message-change/delete subtypes, broadcast copies, and events outside linked channels.
- Resolve the mapping from `(integration, channel, root message ID)`.
- Download and validate attachments before message creation.
- Retry transient provider/download/storage failures.
- Treat missing/deactivated links as ignored, not failed.

### 7.7 Slack adapter

Configuration:

- `SUPPORT_CHAT_SLACK_CLIENT_ID`
- `SUPPORT_CHAT_SLACK_CLIENT_SECRET`
- `SUPPORT_CHAT_SLACK_SIGNING_SECRET`
- `SUPPORT_CHAT_SLACK_REDIRECT_URL`

Minimum bot scopes for the planned behavior:

- `chat:write`
- `channels:read`
- `channels:history`
- `groups:read`
- `groups:history`
- `users:read`
- `files:read`
- `files:write`

Implementation:

- OAuth v2 start and callback with hashed, single-use, expiring state.
- Verify every Events API and interaction request using Slack timestamp/signature validation and reject stale timestamps.
- Handle Slack URL verification without enqueueing a domain event.
- Resolve the integration from Slack `team_id`/enterprise context after signature validation.
- Insert the inbound event and return HTTP 200 within Slack's three-second deadline.
- Subscribe to `message.channels` and `message.groups`; only process threaded human replies under known root posts.
- Use `chat.postMessage` with `thread_ts` for thread replies.
- Use Slack's current external-file upload flow for attachments rather than the retired legacy upload endpoint.
- Cache user snapshots in `support_chat_external_identities` and refresh them when stale.
- App/bot messages are ignored by default to prevent automation loops.

Public endpoints:

- `GET /api/integrations/support-chat/slack/oauth/callback`
- `POST /api/webhooks/support-chat/slack/events`
- `POST /api/webhooks/support-chat/slack/interactions`

### 7.8 Mattermost adapter

Configuration:

- `SUPPORT_CHAT_MATTERMOST_ALLOWED_HOSTS`
- `SUPPORT_CHAT_MATTERMOST_ALLOW_PRIVATE_NETWORKS` default false
- optional development-only insecure-localhost flag

Implementation:

- Administrator enters canonical HTTPS server URL and bot access token.
- Validate with `/api/v4/users/me` and store the bot user ID.
- List teams/channels available to the bot through REST.
- Post roots/replies through `/api/v4/posts`, using `root_id` for thread replies.
- Download/upload files through Mattermost REST endpoints.
- Use `/api/v4/websocket` and the authenticated `posted` event for two-way private/public channel support.
- Run one global `MattermostListenerManager` under `coordination.PostgresLeader`; the leader maintains and reconnects all active Mattermost integrations.
- On leadership loss, close every provider socket before another pod takes ownership.
- Persist each `posted` event before processing; derive a deterministic provider event key from integration ID, post ID, and create/update timestamp.
- Ignore posts authored by the configured bot ID.

Security:

- Reuse the DNS-resolution and dial-time pinning ideas from `externalmcp` to prevent DNS rebinding.
- Private/link-local/loopback addresses are blocked unless the deployment explicitly enables and allowlists them.
- An unrestricted `*` allowlist must not enable private IP ranges.
- TLS certificate validation remains mandatory outside an explicit localhost development mode.
- Mattermost outgoing webhooks are not the v1 inbound mechanism because they do not provide full private-channel parity.

### 7.9 Formatting

Root post contents:

- customer name/email only when allowed by workspace privacy configuration;
- conversation identifier and subject;
- mailbox, priority, and current status;
- first customer message excerpt;
- attachment summary;
- direct link to the Helpin conversation;
- clear statement that replies in this thread are customer-visible.

Thread reply contents:

- sender display name and role (`Customer`, `Helpin teammate`, or `AI`);
- message body with provider-safe formatting;
- uploaded attachments or signed links when native upload fails;
- no internal metadata, email recipients, CRM private fields, or AI source traces.

Status updates:

- update the root card status where supported;
- optionally add a short thread reply for resolved/reopened transitions;
- never expose internal flow-state or coverage-analysis data.

## 8. API design

Authenticated routes under the existing support router:

| Method | Path | Permission | Purpose |
|---|---|---|---|
| GET | `/support/inbox/chat-integrations` | `support.read` | List connections and health |
| POST | `/support/inbox/chat-integrations/slack/oauth/start` | `support.admin` | Return Slack authorization URL |
| POST | `/support/inbox/chat-integrations/mattermost` | `support.admin` | Create/update Mattermost connection |
| POST | `/support/inbox/chat-integrations/{id}/test` | `support.admin` | Validate credentials/connectivity |
| POST | `/support/inbox/chat-integrations/{id}/disconnect` | `support.admin` | Disable/revoke connection |
| GET | `/support/inbox/chat-integrations/{id}/channels` | `support.admin` | Paginated channel picker |
| GET | `/support/inbox/chat-channel-links` | `support.read` | List mailbox mappings |
| POST | `/support/inbox/chat-channel-links` | `support.admin` | Link provider channel |
| PATCH | `/support/inbox/chat-channel-links/{id}` | `support.admin` | Update mode/policy/toggles |
| DELETE | `/support/inbox/chat-channel-links/{id}` | `support.admin` | Disable channel link |
| GET | `/support/inbox/chat-deliveries` | `support.admin` | Inspect pending/failed operations |
| POST | `/support/inbox/chat-deliveries/{id}/retry` | `support.admin` | Retry terminal delivery after correction |
| POST | `/support/inbox/chat-identities/{id}/link` | `support.admin` or self-link flow | Link provider identity to Helpin user |

Handler rules:

- never return access/refresh tokens or shared secrets;
- return stable error codes for missing scope, bot-not-in-channel, unreachable server, TLS failure, host blocked, token revoked, and rate limited;
- paginate provider channel lists;
- validate every integration, channel link, mailbox, and identity against the requested workspace.

## 9. Frontend implementation

Add a `Chat integrations` tab to `InboxesRoutingSettingsPage`, because channel links are mailbox routing configuration rather than personal notification preferences.

New files:

- `frontend/src/lib/supportChatTypes.ts`
- `frontend/src/lib/services/supportChatService.ts`
- `frontend/src/hooks/queries/useSupportChatIntegrations.ts`
- `frontend/src/components/settings/SupportChatIntegrationsTab.tsx`
- `frontend/src/components/settings/SupportChatConnectionDialog.tsx`
- `frontend/src/components/settings/SupportChatChannelLinkDialog.tsx`
- component tests under `frontend/src/components/settings/__tests__/`

Modify:

- `frontend/src/pages/settings/InboxesRoutingSettingsPage.tsx`
- `frontend/src/lib/queryKeys.ts`
- `frontend/src/components/settings/index.ts`
- generated route tree only through the normal TanStack Router generation process if a route change is introduced.

UX sections:

1. **Connections**
   - Slack `Connect workspace` OAuth button.
   - Mattermost server URL/token form.
   - Provider workspace/server name, status, last connected/event times, and sanitized error.
   - Test, reconnect, and disconnect actions.

2. **Mailbox channel mappings**
   - Mailbox selector, including Shared inbox.
   - Provider connection and searchable channel selector.
   - Thread trigger, reply policy, attachment sync, and status sync.
   - Warning that channel replies can be customer-visible.

3. **Delivery health**
   - Pending/failed counts.
   - Latest failure reason and retry action.
   - Link to the affected Helpin conversation where applicable.

4. **External identities**
   - Show identities that have replied.
   - Optional Helpin user mapping for permissioned actions.

Use TanStack Query for all server state and invalidate integration, channel-link, and health keys after mutations.

## 10. Security and privacy requirements

- Encrypt every provider token at rest with integration-bound AAD.
- Never store plaintext Slack client secrets/signing secrets in the database.
- Validate Slack signatures in constant time and enforce timestamp freshness.
- Accept Mattermost events only from authenticated bot WebSockets or a future separately authenticated plugin endpoint.
- Treat linked channels as a customer-data boundary and show an explicit admin warning.
- Default to the least provider scopes needed for configured public/private channel behavior.
- Exclude internal notes, AI traces, CRM metadata, email headers, and private customer attributes from posts.
- Apply the existing Helpin attachment MIME and size limits to inbound provider files.
- Re-scan or rely on configured object-storage scanning before making inbound files available to customers.
- Sanitize provider Markdown/mentions before customer delivery; provider mentions must not become Helpin mentions accidentally.
- Prevent SSRF and DNS rebinding for Mattermost URLs and redirects.
- Do not follow provider-controlled redirects to a different origin while authenticated.
- Rate-limit OAuth start, Mattermost connect/test, manual retry, and public webhook endpoints.
- Retain sanitized inbound/outbound audit data according to workspace retention rules.
- Record actor, workspace, integration, channel, conversation, message, and event IDs in structured logs; never log bodies or tokens.

## 11. Failure behavior

| Failure | Expected behavior |
|---|---|
| Slack/Mattermost unavailable | Keep outbox pending and retry; Helpin customer conversation continues normally |
| Token revoked | Mark integration `reauthorization_required`; stop repeated terminal attempts |
| Bot removed from channel | Mark channel link unhealthy; preserve mapping/history |
| Duplicate Slack retry | Unique inbound event returns success without creating another message |
| Duplicate Mattermost socket event | Deterministic event key ignores the duplicate |
| Unknown provider thread | Ignore and retain a sanitized inbound audit row |
| Provider reply after link disabled | Ignore; do not send to customer |
| Attachment rejected/too large | Create the text reply only if non-empty and post a visible provider error; otherwise mark event failed |
| Helpin message succeeds but other provider mirror fails | Customer delivery succeeds; failed mirror remains retryable |
| Provider post succeeds but DB acknowledgement fails | Reconcile using stored external result/provider lookup; surface uncertain delivery to admin |
| Mattermost listener leader dies | Advisory lock is released; another pod reconnects and resumes events |
| Conversation resolved then customer replies | Existing Helpin reopen logic runs; status update is mirrored |

## 12. Testing plan

### 12.1 Backend unit tests

- credential encryption/AAD and token redaction;
- Slack signature freshness and constant-time verification;
- OAuth state single use, expiry, actor/workspace binding, and return-path validation;
- Mattermost URL normalization, allowlist, private-IP rejection, DNS rebinding defense, and TLS rules;
- provider error classification and retry delays;
- root/reply rendering and privacy exclusions;
- reply-channel resolution for widget, email, API, and internal conversations;
- bot/app/edited/deleted event filtering;
- external identity upsert and link behavior;
- loop prevention for source thread plus fanout to other links;
- internal notes never enqueue;
- AI and public human/customer messages do enqueue;
- resolve/reopen permission enforcement.

### 12.2 Repository tests

- tenant isolation on every table;
- unique integration/channel/thread/event/semantic-key constraints;
- concurrent `ClaimDue` calls never claim the same row;
- stale claim token cannot mark delivered/retry/failed;
- expired lease becomes claimable;
- message provenance uniqueness;
- disabled links are excluded from target resolution;
- migration validation and required indexes/check constraints.

### 12.3 Handler tests

- support permissions for all authenticated routes;
- secrets never appear in JSON;
- Slack challenge/signature/retry behavior;
- malformed events receive safe responses without unbounded retries;
- workspace/channel/integration cross-tenant IDs are rejected;
- Mattermost connection errors are sanitized.

### 12.4 Worker/service integration tests

- widget customer message -> root outbox -> provider root -> mapping;
- second customer message -> provider thread reply;
- Slack/Mattermost human reply -> one Helpin message -> widget delivery;
- provider reply on email conversation -> outbound support email;
- duplicate provider event -> exactly one Helpin message/email;
- Helpin public reply -> provider thread;
- provider-origin reply does not echo to source but reaches a second linked channel;
- attachment transfer in both directions;
- retry after 429/5xx and terminal 401/403 behavior;
- status resolve/reopen synchronization;
- worker crash/lease recovery;
- Mattermost listener leadership handoff.

### 12.5 Frontend tests

- connection and channel-link loading/error/empty states;
- OAuth return success/failure presentation;
- token fields never repopulated from the API;
- private-channel/bot membership guidance;
- mailbox mapping validation;
- customer-visible reply warning;
- health state and retry actions;
- permission-disabled controls.

### 12.6 End-to-end provider tests

Maintain provider sandbox workspaces/servers and run a non-blocking scheduled suite initially:

- install/connect;
- channel discovery;
- root and threaded post;
- inbound thread reply;
- attachment round trip;
- token revocation and reconnect;
- bot removal from channel;
- private channel behavior.

Promote deterministic mocked bridge tests to required CI before rollout. Provider-live tests remain scheduled because external availability and rate limits are not deterministic.

## 13. Rollout plan

### Phase 0: Product contract and threat model — 2-3 days

- Confirm scope decisions in Section 3.
- Confirm Slack app ownership/distribution strategy.
- Decide which Mattermost hosts/private networks the deployment may reach.
- Confirm data retention and whether customer email may appear in provider roots.
- Create provider sandbox environments.

Exit criteria: approved thread/reply/identity/privacy contracts.

### Phase 1: Schema and credential foundation — 4-5 days

- Add models, migration, repositories, encryption service, OAuth state, and tenant-isolation tests.
- Add configuration validation and disabled-by-default feature flag.

Exit criteria: migration validates; credentials round-trip encrypted; no provider call yet.

### Phase 2: Provider-neutral queues and bridge — 5-6 days

- Add provider interface, inbound/outbound repositories and workers.
- Add bridge target resolution, semantic keys, thread mappings, reconciliation, and health states.
- Wire graceful worker startup/shutdown.

Exit criteria: fake provider passes posting, retry, deduplication, and recovery tests.

### Phase 3: Support service integration — 5-7 days

- Refactor message creation to share external and Helpin reply behavior.
- Add provenance fields, reply-channel resolution, source exclusion, status hooks, and external identity snapshots.
- Verify existing widget/email/AI/notification tests remain green.

Exit criteria: fake external reply travels through the normal Helpin customer delivery path exactly once.

### Phase 4: Slack adapter — 6-8 days

- OAuth, channel listing, posting, event handler, signature verification, user lookup, token health, file transfer.
- Slack sandbox integration tests.

Exit criteria: private/public Slack thread round trip works with duplicate-event protection.

### Phase 5: Mattermost adapter — 7-9 days

- Secure server validation, REST client, channel listing, posts/files, WebSocket listener manager, leadership, reconnection.
- Mattermost sandbox integration tests.

Exit criteria: private/public Mattermost thread round trip survives listener restart/leadership handoff.

### Phase 6: Settings UI — 4-5 days, parallel with Phases 4-5

- Connections, mailbox-channel mappings, policies, health, failures, retry, identity mapping.
- Frontend unit/component tests.

Exit criteria: an admin can configure and diagnose both providers without database or server access.

### Phase 7: Attachments and status actions — 4-5 days

- Native file upload/download and Helpin storage validation.
- Linked-user resolve/reopen action and provider root status update.
- Privacy and unsupported-file behavior.

Exit criteria: common allowed files and permissioned status actions work end to end.

### Phase 8: Hardening and staged rollout — 5-7 days

- Load/rate-limit tests, lease recovery, security review, failure dashboards/logs, cleanup jobs, operator runbook.
- Enable on an internal workspace, then selected design partners, then general availability.

Exit criteria: no duplicate customer sends, no plaintext secrets, documented rollback/kill switch, and acceptable delivery/error rates.

## 14. Effort summary

| Deliverable | Engineering days |
|---|---:|
| Product contract and threat model | 2-3 |
| Schema, repositories, credentials | 4-5 |
| Provider-neutral bridge and workers | 5-6 |
| Support service integration | 5-7 |
| Slack | 6-8 |
| Mattermost | 7-9 |
| Settings UI | 4-5 |
| Attachments and status actions | 4-5 |
| Hardening, CI, rollout | 5-7 |
| **Total** | **42-55** |

Parallel execution recommendation:

- Engineer A: schema, queues, bridge, support-service integration.
- Engineer B: Slack/Mattermost clients, settings UI, provider sandbox tests.
- Shared: attachments, security review, end-to-end tests, rollout.

Expected elapsed time with two engineers is 5-7 weeks, not `42-55 / 2` exactly, because schema/contracts, support-service integration, and final hardening are dependency-bound.

Smaller alternatives:

- One-way posting only, both providers, text/links: 10-15 engineering days.
- Slack-only two-way, text-only pilot: 20-28 engineering days.
- Slack + Mattermost text-only pilot without attachments/actions/admin diagnostics: 30-40 engineering days.
- provider-originated customer channels after this v1: add approximately 15-25 engineering days for channel ownership, customer/CRM identity, conversation creation, Slack Connect/DM semantics, and new routing behavior.

## 15. Deployment and rollback

New environment variables:

- `SUPPORT_CHAT_BRIDGE_ENABLED=false`
- `SUPPORT_CHAT_INTEGRATION_ENCRYPTION_KEY`
- `SUPPORT_CHAT_SLACK_CLIENT_ID`
- `SUPPORT_CHAT_SLACK_CLIENT_SECRET`
- `SUPPORT_CHAT_SLACK_SIGNING_SECRET`
- `SUPPORT_CHAT_SLACK_REDIRECT_URL`
- `SUPPORT_CHAT_MATTERMOST_ALLOWED_HOSTS`
- `SUPPORT_CHAT_MATTERMOST_ALLOW_PRIVATE_NETWORKS=false`
- optional development-only Mattermost localhost/TLS override

Deployment sequence:

1. Deploy the idempotent database migration.
2. Deploy code with the global feature flag off and workers idle.
3. Configure encryption and Slack app secrets.
4. Enable the feature for an internal workspace only.
5. Connect sandbox providers and verify posting, inbound reply, attachments, duplicate retries, and disconnect.
6. Enable selected customer workspaces through an allowlist/feature gate.
7. Remove the allowlist only after delivery failure and duplicate-send metrics remain acceptable.

Rollback:

- Disable a channel link to stop one route.
- Disable an integration to stop one provider connection while retaining audit mappings.
- Disable `SUPPORT_CHAT_BRIDGE_ENABLED` to stop all inbound listeners and outbound workers without removing tables or Helpin messages.
- Pending rows remain available for later inspection/retry; do not automatically replay them after a long global outage without an operator review window.
- No destructive migration is required for rollback.

## 16. Acceptance criteria

- A customer widget message creates exactly one root post in every configured channel.
- Subsequent public messages appear in the correct provider thread in order.
- A human provider reply creates exactly one public Helpin message and reaches the customer once.
- Internal notes and private Helpin metadata never appear in provider channels.
- Provider bot/app messages never create customer replies.
- Duplicate Slack retries and Mattermost socket redelivery are idempotent.
- A provider outage does not block Helpin message creation or customer support operations.
- Tokens are encrypted at rest and absent from logs/API responses/outbox payloads.
- Cross-workspace integration, channel, thread, identity, and message IDs cannot be used to access or mutate another workspace.
- Private Slack and Mattermost channels work when the bot has explicitly been added.
- Attachments obey Helpin's file rules in both directions.
- Resolve/reopen actions require linked Helpin identity and `support.edit`.
- Administrators can see connection health and retry failed outbound operations.
- Feature and per-link kill switches stop new delivery without data deletion.

## 17. References

- Existing Helpin Mattermost PM proposal: `docs/mattermost-integration.md`
- Existing support message path: `server/internal/service/support_inbox.go`
- Existing attachment path: `server/internal/service/support_attachment.go`
- Existing PostgreSQL outbox: `server/internal/service/customer_io_outbox.go`
- Existing leadership primitive: `server/internal/coordination/postgres_leader.go`
- Crisp Slack behavior: https://help.crisp.chat/en/article/how-to-connect-slack-with-crisp-ufmigu/
- Intercom Slack channel behavior: https://www.intercom.com/help/en/articles/11534476-connect-your-slack-channel
- Intercom notification-only distinction: https://www.intercom.com/help/en/articles/12556780-set-up-slack-notifications
- Slack OAuth: https://docs.slack.dev/authentication/installing-with-oauth/
- Slack Events API: https://docs.slack.dev/apis/events-api/
- Slack message events: https://docs.slack.dev/reference/events/message/
- Mattermost API/WebSocket: https://developers.mattermost.com/api-documentation/
- Mattermost webhook limitation: https://developers.mattermost.com/integrate/webhooks/
