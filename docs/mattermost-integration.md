# Mattermost integration proposal

> Status: unimplemented proposal, checked against this checkout on 2026-09-17.
> The Mattermost handlers, services, client, and settings components listed below
> are proposed files, not existing integration code. The River queue and phased
> setup instructions are design choices, not supported installation steps.
> Older references to stories also predate the task terminology used by current code.


## Context

Helpin needs a Mattermost integration similar to [Shortcut's Slack integration](https://www.shortcut.com/integrations/slack). Since Helpin uses self-hosted Mattermost, we can leverage Mattermost's Bot Accounts + REST API directly (no app marketplace needed). The integration connects project management activity in Helpin to team communication in Mattermost.

## How It Works (Self-Hosted Mattermost)

**Approach**: Helpin acts as an API client to Mattermost. A Bot Account is created in Mattermost, and its Personal Access Token is stored in Helpin. The integration is **owned by a Helpin team**, not the whole workspace. Helpin then uses the Mattermost REST API (`/api/v4/*`) to:
- Post messages to channels (notifications)
- Read channel lists (for config UI)
- Send DMs to users (personal notifications)
- Respond to slash commands and interactive messages (inbound)

Because this deployment uses a **single Mattermost server**, each Helpin team stores the same Mattermost `server_url` in its own integration settings. That keeps ownership and permissions team-scoped while still supporting one shared Mattermost instance.

**No Mattermost plugins required** -- everything works via REST API + webhooks + slash commands, all available in self-hosted Mattermost (any version 5.x+).

### Message Delivery: River Queue

Instead of making direct HTTP calls to Mattermost (which are fire-and-forget and lost on failure), all outbound messages are routed through [River Queue](https://riverqueue.com/) -- a PostgreSQL-backed background job system for Go.

**Why River Queue:**
- **Same infrastructure** -- uses PostgreSQL + pgx, which Helpin already runs. No new services (Redis, RabbitMQ, etc.) needed.
- **Reliable delivery** -- if Mattermost is down, jobs are retried automatically with exponential backoff.
- **Rate control** -- worker concurrency limits prevent flooding Mattermost.
- **Scheduling** -- Phase 5's daily digest is a scheduled job; River has native support for periodic/cron jobs.
- **Observability** -- pending/failed/completed jobs are rows in PostgreSQL. Delivery status can be surfaced in the settings UI.

**How it flows:**
```
Hub.Broadcast(event)
  -> MattermostNotifier.OnEvent(event)
    -> Resolves entity details, formats message
    -> Enqueues a River job (MattermostDeliveryArgs)

River Worker picks up job
  -> Posts to Mattermost REST API
  -> On failure: automatic retry with backoff (max 5 attempts)
  -> On success: job marked complete
```

This means `Hub.Broadcast()` returns instantly -- no risk of Mattermost HTTP latency affecting WebSocket delivery to browser clients.

---

## Mattermost Bot Setup Steps

### Prerequisites
- Mattermost self-hosted instance (v5.x or later)
- System Admin access to Mattermost

### Step 1: Enable Bot Accounts
1. Go to **System Console > Integrations > Bot Accounts**
2. Set **Enable Bot Account Creation** to `true`
3. Save

### Step 2: Create a Bot Account
1. Go to **Integrations > Bot Accounts** (from the main menu, not system console)
2. Click **Add Bot Account**
3. Fill in:
   - **Username**: `helpin-bot`
   - **Display Name**: `Helpin`
   - **Description**: `Helpin project management notifications`
   - **Role**: `Member` (sufficient for posting to channels)
   - **Icon**: Upload a Helpin logo (optional)
4. Click **Create Bot Account**
5. **Copy the Access Token** -- this is shown only once. Store it securely.

### Step 3: Enable Personal Access Tokens (if not already)
1. Go to **System Console > Integrations > Integration Management**
2. Set **Enable Personal Access Tokens** to `true`
3. Save

### Step 4: Add Bot to Channels
1. In each Mattermost channel where you want notifications:
   - Click the channel name > **Add Members**
   - Search for `helpin-bot` and add it
2. The bot must be a member of any channel it will post to

### Step 5: Enable Slash Commands (for Phase 2)
1. Go to **System Console > Integrations > Integration Management**
2. Set **Enable Custom Slash Commands** to `true`
3. Save

### Step 6: Note Your Server URL
- Your Mattermost server URL (e.g., `https://mattermost.yourcompany.com`)
- Must be reachable from the Helpin server (network/firewall)

### Step 7: Configure in Helpin
1. Navigate to **Settings > Teams > {Team}** in Helpin
2. Open the **Mattermost** section for that team
3. Enter the Mattermost Server URL
4. Paste the Bot Access Token
5. Click **Test Connection** -- should show the bot's display name
6. Enable the integration
7. Add one or more channel links for that team

The Team Settings UI should follow the same interaction model as the reference Slack example:
- A **Connect a Mattermost channel** card at the top of the team's Mattermost section
- A separate **Notifications** section below it with per-event toggles
- Team users with permission can decide exactly which updates this team wants posted to Mattermost

---

## Phase 1: Outgoing Notifications + Channel Linking

**Value**: When stories/epics/comments change in Helpin, formatted messages auto-post to linked Mattermost channels. This is the highest-value, lowest-complexity feature.

### Architecture

Hook into the existing WebSocket `Hub.Broadcast()` in `server/internal/websocket/hub.go`, but do **not** derive Mattermost notifications from generic `updated` events alone. Mattermost routing needs explicit domain intent such as `story_state_changed`, `story_completed`, `comment_created`, `epic_state_changed`, etc. The PM services should emit richer notification events (or enrich `websocket.Event` with change metadata) before the notifier consumes them.

```
Hub.Broadcast(event)
  -> existing: send to WebSocket clients
  -> new: call registered EventListeners (MattermostNotifier)

MattermostNotifier.OnEvent(event)
  -> Resolves the Helpin team(s) for the entity using explicit model fields
  -> Looks up team-owned integration config(s) (skip if none/disabled)
  -> Looks up channel links for that team
  -> Checks notification toggles for the entity type
  -> Fetches full entity details (story with state, owners, etc.)
  -> Formats a rich Mattermost message
  -> Enqueues a River job (MattermostDeliveryArgs)

River Worker (MattermostDeliveryWorker)
  -> Picks up job from PostgreSQL queue
  -> Posts message to Mattermost REST API
  -> On failure: retries with exponential backoff (max 5 attempts, 30s/1m/5m/15m/30m)
  -> Stores Mattermost post ID in job result (used for thread mapping in Phase 4)
```

### Database Models

File: `server/internal/model/mattermost.go`

**MattermostIntegration** (one per Helpin team)

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | `gen_random_uuid()` |
| workspace_id | UUID | index, FK to workspaces |
| team_id | UUID | unique index with workspace, FK to `workspace_teams` |
| server_url | string | e.g. `https://mattermost.example.com` |
| bot_token | string | `json:"-"` -- never in API responses |
| bot_user_id | string | cached after validation |
| webhook_secret | string | `json:"-"` -- for verifying inbound requests (Phase 2) |
| enabled | bool | default false |
| created_at | timestamp | auto |
| updated_at | timestamp | auto |

Unique key: `(workspace_id, team_id)`

**MattermostChannelLink** (many per team integration)

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| workspace_id | UUID | index |
| team_id | UUID | index, FK to `workspace_teams` |
| integration_id | UUID | FK to MattermostIntegration |
| channel_id | string | Mattermost channel ID |
| channel_name | string | cached display name |
| notify_story_created | bool | default true -- a new story is added to the team |
| notify_story_status_changed | bool | default true -- a story moves to a different state |
| notify_story_completed | bool | default true -- a story is marked done |
| notify_story_comment | bool | default true -- a comment is added to a story |
| notify_epic_created | bool | default false -- a new epic is created |
| notify_epic_status_changed | bool | default false -- an epic changes state |
| notify_sprint_started | bool | default false -- a sprint starts |
| notify_sprint_completed | bool | default false -- a sprint is completed |
| notify_objective_update | bool | default false -- an objective or key result is updated |
| created_at | timestamp | |
| updated_at | timestamp | |

Unique key: `(integration_id, channel_id)` to prevent duplicate posts to the same Mattermost channel for a team.

These granular toggles follow the pattern used by Linear's Slack integration, giving teams fine-grained control over which events trigger notifications. For example, a team may only want to see completed stories and comments, but not every status change.

### Backend Files

| File | Purpose |
|------|---------|
| `server/internal/mattermost/client.go` | HTTP client wrapping Mattermost REST API. Nil-safe (like `email/postmark.go`). Methods: `PostMessage`, `PostMessageWithAttachments`, `GetChannels`, `GetChannel`, `ValidateConnection` |
| `server/internal/mattermost/types.go` | Types: `Channel`, `Post`, `Attachment`, `AttachmentField`, `User`, `PostCreateRequest` |
| `server/internal/mattermost/worker.go` | River worker (`MattermostDeliveryWorker`) that processes delivery jobs. Accepts `MattermostDeliveryArgs` (integration_id, team_id, channel_id, message, attachments, workspace_id), resolves the client for the team integration, and calls the Mattermost API |
| `server/internal/mattermost/jobs.go` | River job args types: `MattermostDeliveryArgs`, `MattermostDigestArgs` (Phase 5). Implements `river.JobArgs` interface |
| `server/internal/model/mattermost.go` | GORM models above |
| `server/internal/repository/mattermost.go` | `MattermostIntegrationRepo` (GetByTeam, ListByWorkspace, UpsertByTeam, DeleteByTeam) + `MattermostChannelLinkRepo` (ListByTeam, Create, Update, Delete) |
| `server/internal/service/mattermost.go` | Team-scoped CRUD for config + channel links, TestConnection, ListAvailableChannels, team authorization checks |
| `server/internal/service/mattermost_notifier.go` | Implements `EventListener`, formats messages per entity type, enqueues River jobs (does NOT call Mattermost directly) |
| `server/internal/handler/mattermost.go` | HTTP handlers for CRUD endpoints |

### Modified Files

| File | Change |
|------|--------|
| `server/internal/websocket/hub.go` | Add `EventListener` interface, `listeners []EventListener` field, `AddListener()` method. Extend `Event` to carry notification-relevant change metadata or explicit change kinds. In `Broadcast()`, iterate listeners and call `go l.OnEvent(event)` |
| `server/internal/service/pm_story.go` | Emit explicit notification events for story creation, state changes, completions, and comments instead of relying on generic `updated` semantics |
| `server/internal/service/pm_epic.go` | Emit explicit notification events for epic creation and state changes |
| `server/internal/service/pm_objective.go` | Emit explicit notification events for objective updates and key result updates |
| `server/internal/router/router.go` | Add `Mattermost` handler to `Handlers` struct, register team-scoped routes under `/api/settings/teams/{teamID}/mattermost/` and optional workspace admin list routes |
| `server/cmd/api/main.go` | Wire repos, services, handler. Register notifier as Hub listener. Initialize River client with pgx pool, register `MattermostDeliveryWorker`, start River client. Add models to AutoMigrate |
| `server/go.mod` | Add `github.com/riverqueue/river` and `github.com/riverqueue/river/riverdriver/riverpgxv5` dependencies |

### River Queue Setup (in `main.go`)

```go
import (
    "github.com/riverqueue/river"
    "github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// Initialize River client with the existing pgx pool
riverClient, err := river.NewClient(riverpgxv5.New(pgxPool), &river.Config{
    Queues: map[string]river.QueueConfig{
        "mattermost": {MaxWorkers: 5},  // max 5 concurrent Mattermost API calls
    },
    Workers: workers,
})

// Register workers
workers := river.NewWorkers()
river.AddWorker(workers, mattermost.NewDeliveryWorker(mmIntegrationRepo))

// Start processing
riverClient.Start(ctx)
```

River uses its own PostgreSQL tables (`river_job`, `river_leader`, etc.) created via its migration tool. Run `river migrate-up --database-url $DATABASE_URL` once during setup.

### Team-Scoped Notification Routing

Each Helpin team owns its own Mattermost integration and channel links. Routing is based on the entity's **explicit team fields**, not inferred indirectly from workflow state transitions.

**Example configuration:**

| Helpin Team | Mattermost Channel | Story Created | Status Changed | Completed | Comments | Epic Created |
|---|---|---|---|---|---|---|
| Engineering | `#engineering` | on | on | on | on | on |
| Customer Support | `#cs-updates` | on | off | on | on | off |
| Marketing | `#marketing` | off | off | on | off | off |

This gives teams fine-grained control. For example, Marketing only sees completed stories (not every status change), while Engineering gets the full firehose.

**Routing logic when a story changes state** (e.g., SC-42 moves from "In Progress" to "Done"):

1. `PMStoryService` emits a notification event such as `story_state_changed` or `story_completed` with the old and new state IDs
2. Notifier fetches the story and reads `story.team_id` directly
3. If `story.team_id` is nil, the notifier skips delivery or uses `workflow.team_id` only as a fallback for legacy records
4. Notifier loads that team's `MattermostIntegration`
5. If the integration is enabled, it loads all `MattermostChannelLink`s for that team
6. It filters by the correct toggle (`notify_story_status_changed` or `notify_story_completed`)
7. It enqueues one River delivery job per matching channel

So if SC-42 belongs to Engineering: the update posts only to Engineering's configured Mattermost channels, not to Customer Support or Marketing.

**Entity team resolution rules:**
- Stories: use `pm_stories.team_id`
- Story comments: use the parent story's `team_id`
- Epics: use `pm_epics.team_id`
- Sprints: use `pm_sprints.team_id`
- Objectives: use `pm_objective_teams` and fan out to each linked team's integration
- Objective comments / key result updates: resolve through the objective's linked teams

**Notifier resolution pseudocode:**
```
func (n *MattermostNotifier) resolveTargets(event Event) []DeliveryTarget:
    teamIDs = resolveEntityTeams(event) // explicit team fields only
    targets = []

    for teamID in teamIDs:
        integration = integrationRepo.GetByTeam(event.WorkspaceID, teamID)
        if integration == nil || !integration.enabled:
            continue

        links = channelLinkRepo.ListByTeam(event.WorkspaceID, teamID)
        for link in links:
            if toggleEnabled(link, event.ChangeKind):
                targets = append(targets, DeliveryTarget{
                    TeamID: teamID,
                    IntegrationID: integration.ID,
                    ChannelID: link.ChannelID,
                })

    return targets
```

### Settings Access Points

Mattermost settings are primarily managed **inside each Helpin team**:

1. **Per-team**: Settings > Teams > {Team} -- team owners can configure that team's Mattermost server URL, bot token, enabled flag, and channel links.
   The Team Settings view should include a connection card followed by a notification preferences list, so users can connect Mattermost and then choose which events the team wants to receive.

2. **Workspace overview (optional)**: Settings > Integrations -- workspace admins can see a read-only or admin-editable summary of all team integrations.

The source of truth is the team-owned `MattermostIntegration` record. The workspace overview is only an aggregate view.

### API Endpoints

Primary routes under `/api/settings/teams/{teamID}/mattermost/` (require JWT + workspace header + workspace admin/owner or `team_user_memberships.role = 'owner'` for that team):

| Method | Path | Description |
|--------|------|-------------|
| GET | `/integration` | Get team integration config (token masked) |
| PUT | `/integration` | Create/update team integration config |
| POST | `/integration/test` | Validate bot token, return bot info |
| DELETE | `/integration` | Remove integration |
| GET | `/channels` | List Mattermost channels the bot can access |
| GET | `/channel-links` | List configured channel links for the team |
| POST | `/channel-links` | Create a channel link for the team |
| PUT | `/channel-links/{id}` | Update notification toggles |
| DELETE | `/channel-links/{id}` | Remove a channel link |

Optional admin overview routes under `/api/pm/mattermost/`:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/integrations` | List all team integrations in the workspace |

### Frontend Changes

| File | Change |
|------|--------|
| `frontend/src/lib/pmTypes.ts` | Add `MattermostIntegration`, `MattermostChannelLink`, `MattermostChannel` interfaces |
| `frontend/src/lib/services/mattermostService.ts` | New API service (follows `pmAutomationService.ts` pattern) |
| `frontend/src/pages/Settings.tsx` | Add a Mattermost section inside Team Settings instead of a workspace-global integration owner form |
| `frontend/src/components/settings/TeamMattermostSettings.tsx` | **New**: Team-scoped Mattermost settings UI with a top connection card and a notifications section below it |
| `frontend/src/components/settings/MattermostWorkspaceOverview.tsx` | **Optional**: admin-only summary of team integrations |

### Team Settings UX

Inside **Settings > Teams > {Team} > Mattermost**, the UI should be structured as:

1. **Connect a Mattermost channel** -- card for server URL, bot token, test connection, enable/disable, and channel selection
2. **Notifications** -- list of toggles so the team can decide what should trigger a Mattermost post

Recommended toggles:
- New story is added to the team
- Story changes status
- Story is completed
- Comments on stories
- Epic is created
- Epic changes status
- Sprint starts
- Sprint completes
- Objective or key result is updated

This keeps the integration clearly team-owned and gives users direct control over what their team is notified about, rather than forcing a workspace-wide default.

### Message Formats

**Story created:**
```
**John Doe** created a new story
┌─────────────────────────────────
│ [SC-42] Fix login timeout bug
│ Type: bug | Priority: high | State: Backlog
│ -> View in Helpin
└─────────────────────────────────
```

**Story state changed:**
```
**John Doe** moved SC-42 from **Backlog** -> **In Progress**
│ Fix login timeout bug
│ -> View in Helpin
```

**Comment added:**
```
**Jane Smith** commented on SC-42
│ "We should check the session refresh logic too"
│ -> View in Helpin
```

Color-coded by story type (blue=feature, red=bug, yellow=chore).

### Security

- Bot token: `json:"-"` tag, GET endpoint returns `****...last4`
- Webhook secret: generated server-side via `crypto/rand` (32 bytes, hex-encoded)
- All DB queries scoped by `workspace_id` and, for integration config, by `team_id`
- Authorization: only workspace admins/owners or team owners can mutate a team's integration
- Rate limiting: per-team message throttle (configurable, default 10/sec)
- TLS: client validates certs by default; optional `allow_insecure` for dev with self-signed certs

---

## Phase 2: Slash Commands (Inbound from Mattermost)

**Value**: Users type `/helpin create "Bug title" --type=bug` in Mattermost to create stories without leaving chat.

### Mattermost Setup

1. Go to **Integrations > Slash Commands > Add Slash Command**
2. Configure:
   - **Command Trigger Word**: `helpin`
   - **Request URL**: `https://helpin.yourcompany.com/api/webhooks/mattermost/slash`
   - **Request Method**: POST
   - **Token**: Copy this -- enter it as the webhook secret in that team's Mattermost settings
3. Save

### Architecture

Slash commands POST to a public endpoint (no JWT). Authentication is via the Mattermost webhook token matching the stored `webhook_secret` on the owning team integration.

### New Database Model

**MattermostUserMapping**

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| workspace_id | UUID | index |
| mattermost_user_id | string | unique index (composite with workspace) |
| user_id | UUID | FK to users table |
| created_at | timestamp | |

### Commands

| Command | Action |
|---------|--------|
| `/helpin create "title" [--type=bug] [--priority=high] [--team=Frontend]` | Create a story |
| `/helpin search <query>` | Find stories (returns top 5 with links) |
| `/helpin status SC-123` | Show story details as rich card |
| `/helpin link` | Link MM account to Helpin (sends one-time URL via DM) |
| `/helpin help` | List available commands |

### Interactive Messages

After story creation, the bot posts an interactive message with buttons:
- **View Story** (link to Helpin)
- **Set Priority** (dropdown: low/medium/high/urgent)
- **Assign to Me** (auto-assigns the MM user if mapped)

Button clicks POST to `/api/webhooks/mattermost/interactive`.

### Account Linking Flow

1. User types `/helpin link` in Mattermost
2. Bot sends the user a DM with a one-time URL: `https://helpin.example.com/api/mattermost/link?code=<otp>&mm_user=<mm_id>&team=<team_id>`
3. User clicks link -- browser opens, Helpin validates their JWT session
4. `MattermostUserMapping` record is created
5. Bot confirms linkage via DM

### New Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/webhooks/mattermost/slash` | Webhook secret | Slash command handler |
| POST | `/api/webhooks/mattermost/interactive` | Webhook secret | Interactive action handler |
| GET | `/api/mattermost/link` | JWT | Account linking callback |

---

## Phase 3: Link Unfurling (Rich Previews)

**Value**: Paste a Helpin URL in Mattermost and get a rich preview card showing story details.

### How It Works

**Option A -- Outgoing Webhook (simpler)**:
1. Create an Outgoing Webhook in Mattermost that triggers on URLs matching `helpin.yourcompany.com`
2. Webhook POSTs the message to `POST /api/webhooks/mattermost/unfurl`
3. Handler parses the URL, fetches entity details, returns a rich attachment response

**Option B -- Bot polling (fallback if webhooks are restricted)**:
- Bot periodically checks for messages mentioning Helpin URLs via the Mattermost API
- Less real-time, but works without configuring outgoing webhooks

### URL Patterns Detected

| Pattern | Entity |
|---------|--------|
| `/w/{slug}/pm/stories/{displayID}` | Story card with type, priority, state, owners |
| `/w/{slug}/pm/epics/{id}` | Epic card with progress, health, story count |
| `/w/{slug}/pm/sprints/{id}` | Sprint card with dates, story counts |
| `/w/{slug}/pm/objectives/{id}` | Objective card with KR progress |

### Unfurl Card Example

```
┌─ SC-42 Fix login timeout bug ──────────────
│ Type: Bug       Priority: High
│ State: In Progress (Frontend Workflow)
│ Owners: John Doe, Jane Smith
│ Epic: Q1 Auth Improvements
│ Sprint: Sprint 12 (Mar 3-14)
│ Updated: 2 hours ago
└─────────────────────────────────────────────
```

---

## Phase 4: Bidirectional Comment Thread Sync

**Value**: Comments on a story appear in a Mattermost thread, and replies in the thread create comments in Helpin.

### New Database Model

**MattermostThreadMapping**

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| workspace_id | UUID | index |
| team_id | UUID | index |
| entity_type | string | "story" |
| entity_id | UUID | unique index |
| channel_id | string | Mattermost channel |
| post_id | string | Mattermost root post ID (unique index) |
| created_at | timestamp | |

### Sync Flow

**Helpin -> Mattermost**:
1. Phase 1 notifier already posts story creation messages
2. When posting, store the returned post ID as the thread root in `MattermostThreadMapping`
3. On subsequent comment events for that story, post as a reply to the root post (using `root_id` in Mattermost API)

**Mattermost -> Helpin**:
1. Outgoing webhook triggers on replies in threads where the root post is from the bot
2. Handler looks up `MattermostThreadMapping` by `post_id`
3. Resolves MM user to Helpin user via `MattermostUserMapping`
4. Calls `PMCommentService.Create` with the message body

### Deduplication

- Events from Helpin carry `source: "helpin"` in metadata
- Posts from Mattermost carry `source: "mattermost"` in comment metadata (JSONB)
- Notifier skips events with `source: "mattermost"`
- Webhook handler skips posts from the bot user ID

---

## Phase 5: Personal DMs + Advanced Features

### Personal Notifications via DM

Users with linked accounts receive DMs for:
- @mentions in comments (`@john` in a comment -> DM to John's mapped MM user)
- State changes on stories they **own** (uses `pm_story_owners` table)
- State changes on stories they **follow** (uses `pm_story_followers` table)
- Assignment/unassignment

Requires: `MattermostUserMapping` from Phase 2.

### Daily Digest

Configurable per channel link and executed per team integration. Uses River's built-in **periodic job** support (cron-style scheduling) instead of a custom ticker. The `MattermostDigestArgs` job is registered with a cron schedule (e.g., `0 9 * * 1-5` for weekdays at 9am). River handles scheduling, deduplication, and execution:

```
Daily Summary for Frontend Team -- March 5, 2026

Completed: 3 stories (8 points)
In Progress: 7 stories
New: 2 stories added
Blocked: 1 story (SC-38 needs API review)

Sprint 12 Progress: 64% (ends Mar 14)
-> View Sprint Dashboard
```

### Stand-Up Bot

`/helpin standup` in a channel:
1. Bot DMs each team member with standup questions
2. Collects responses over a configured window (e.g., 15 min)
3. Posts formatted summary to the team channel

---

## Implementation Priority & Dependencies

```
Phase 1 ──────────────────────> Phase 3 (unfurling)
    │                               │
    └──> Phase 2 (slash cmds) ──> Phase 4 (thread sync)
              │                       │
              └───────────────────> Phase 5 (DMs, digest)
```

- **Phase 1** is standalone, no dependencies
- **Phase 2** is standalone but benefits from Phase 1 (interactive messages reference notifications)
- **Phase 3** needs Phase 1 (uses same Mattermost client)
- **Phase 4** needs Phase 1 (thread mapping from notification posts) + Phase 2 (user mapping for inbound comments)
- **Phase 5** needs Phase 2 (user mapping for DMs)

---

## Key Existing Code to Reuse

| What | Where | Why |
|------|-------|-----|
| WebSocket Hub + Publisher | `server/internal/websocket/hub.go`, `publisher.go` | Event system to hook into |
| Postmark email client pattern | `server/internal/email/postmark.go` | Nil-safe external service client pattern |
| PM Automation service pattern | `server/internal/service/pm_automation.go` | Background job + config model pattern |
| Settings page sections | `frontend/src/pages/Settings.tsx:28-91` | Section-based settings UI pattern |
| Automation service (frontend) | `frontend/src/lib/services/pmAutomationService.ts` | API service pattern |
| Activity logging | `server/internal/model/pm_activity.go` | Can add `source` metadata field for dedup |

## External Dependencies

| Package | Purpose | Notes |
|---------|---------|-------|
| `github.com/riverqueue/river` | Background job queue | PostgreSQL-backed, uses pgx -- same driver Helpin already uses |
| `github.com/riverqueue/river/riverdriver/riverpgxv5` | River's pgx v5 driver | Connects River to the existing pgx pool |

River creates its own tables in PostgreSQL (`river_job`, `river_leader`, `river_migration`). One-time migration needed:
```bash
go run github.com/riverqueue/river/cmd/river migrate-up --database-url "$DATABASE_URL"
```
