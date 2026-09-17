# PRD: Notifications System

**Date:** 2026-03-08
**Status:** Draft
**Author:** System Architect
**Inspiration:** Linear, Shortcut

---

## 1. Executive Summary

Build a comprehensive notification system for Helpin's PM tool that keeps users informed about relevant activity in their workspace. The system delivers notifications through multiple channels (in-app inbox, email, WebSocket real-time, and future push/Slack) with granular per-user preference controls.

**Core design principles:**
- **Entity-centric inbox** (Linear-inspired): One notification row per entity (story, epic, objective), not per event. Multiple events on the same entity update the existing notification row.
- **Follower/subscriber model** (Shortcut-inspired): Users receive notifications only for entities they follow. Auto-follow on assignment, creation, mention, and comment.
- **Per-channel, per-event-type preferences**: Users control exactly what they receive and where.
- **Denormalized render snapshots**: The aggregated `notifications` row stores `actor_snapshot`, `entity_snapshot`, and `parent_entity_snapshot` JSONB payloads for inbox rendering. Relational IDs remain canonical for lookups, integrity, and backfills.
- **Transactional consistency**: Notification jobs are enqueued in the same PostgreSQL transaction as the triggering business operation — no ghost notifications.

**Tech choice:** [River](https://riverqueue.com) (PostgreSQL-native job queue) for async notification delivery. See [research/notification-queue-comparison.md](../research/notification-queue-comparison.md) for full evaluation.

---

## 2. Goals & Non-Goals

### Goals
- Deliver timely, relevant notifications across in-app inbox, email digest, and real-time WebSocket channels
- Give users granular control over what notifications they receive and through which channels
- Support entity-centric inbox with read/unread, archive, and snooze
- Enable @-mention notifications in comments and descriptions
- Support follower/subscriber model for per-entity notification opt-in/opt-out
- Batch email notifications into configurable digests (daily/weekly) with immediate delivery for urgent events
- Integrate with existing WebSocket infrastructure for real-time delivery
- Scale to workspaces with 500+ members

### Non-Goals (v1)
- Mobile push notifications (future)
- Slack integration (future — separate PRD)
- Triage/shared team inbox (future)
- Notification reminders with natural language date parsing (future)
- Browser push notifications (future)
- Notification search/filter within inbox (future)

---

## 3. User Stories

### 3.1 As a team member, I want to...
- See all my notifications in a centralized inbox so I can stay on top of relevant activity
- Mark notifications as read/unread so I can track what I've reviewed
- Archive notifications I've dealt with to keep my inbox clean
- Snooze a notification to a future time so I can deal with it later
- Click a notification to navigate directly to the relevant entity
- Know when someone @-mentions me in a comment or description
- Automatically receive notifications for stories I'm assigned to, create, or comment on
- Follow/unfollow specific stories, epics, or objectives to control my notification flow

### 3.2 As a workspace admin, I want to...
- Ensure all members receive critical workspace-level notifications (e.g., sprint started, cycle changes)
- See notification delivery health metrics (future)

### 3.3 As any user, I want to...
- Configure which event types I receive notifications for
- Choose my preferred notification channels (in-app, email)
- Set email digest frequency (immediate for mentions, daily digest for activity)
- Mute all notifications temporarily (Do Not Disturb)

---

## 4. Notification Events

### 4.1 Event Taxonomy

Events are organized into categories. Each category can be independently toggled per notification channel in user preferences.

#### Story Events
| Event | Description | Default Recipients |
|-------|-------------|-------------------|
| `story.assigned` | Story assigned to a user | Assignee + followers |
| `story.unassigned` | Story unassigned from a user | Previous assignee + followers |
| `story.status_changed` | Story moved to a different workflow state | Followers |
| `story.priority_changed` | Story priority updated | Followers |
| `story.comment_added` | New comment on a story | Followers (commenter auto-followed) |
| `story.mentioned` | @-mentioned in story description or comment. Metadata `mention_type`: `direct` or `team`. Direct mentions are `urgent` priority; team mentions are `high`. | Mentioned user or all team members (auto-followed) |
| `story.completed` | Story moved to a "done" state | Followers |
| `story.estimate_changed` | Story estimate/points updated | Followers |
| `story.due_date_changed` | Story due date set or changed | Followers |
| `story.label_changed` | Labels added/removed | Followers |
| `story.moved_to_sprint` | Story added to a sprint | Followers |
| `story.blocked` | Story marked as blocked | Followers + assignee |
| `story.unblocked` | Story blocker resolved | Followers + assignee |

#### Epic Events
| Event | Description | Default Recipients |
|-------|-------------|-------------------|
| `epic.created` | New epic created in a followed project | Project followers |
| `epic.status_changed` | Epic status updated | Followers |
| `epic.comment_added` | New comment on an epic | Followers |
| `epic.mentioned` | @-mentioned in epic description or comment. Metadata `mention_type`: `direct` or `team`. | Mentioned user or all team members (auto-followed) |
| `epic.completed` | Epic marked complete | Followers |

#### Sprint Events
| Event | Description | Default Recipients |
|-------|-------------|-------------------|
| `sprint.started` | Sprint activated | All team members |
| `sprint.completed` | Sprint completed | All team members |
| `sprint.ending_soon` | Sprint ending within 24h | All team members |

#### Objective Events
| Event | Description | Default Recipients |
|-------|-------------|-------------------|
| `objective.status_changed` | Objective status/health updated | Followers |
| `objective.comment_added` | Comment on objective | Followers |
| `objective.progress_updated` | Key result progress changed | Followers |

#### Workspace Events
| Event | Description | Default Recipients |
|-------|-------------|-------------------|
| `workspace.member_invited` | New member invited | Admins |
| `workspace.member_joined` | New member accepted invite | Admins |
| `workspace.settings_changed` | Workspace settings modified | Admins + owners |

#### Mentions (Design Note)

Mentions are **not** a separate event type. They are a sub-case of `{entity}.mentioned` events (e.g., `story.mentioned`, `epic.mentioned`). The `metadata.mention_type` field distinguishes `direct` (single user) vs `team` (team @-mention, fan-out to all members). This avoids dual event types for the same action and keeps the preference UI simple — one toggle for "mentioned" per entity type.

### 4.2 Event Priority Levels

| Priority | Behavior | Examples |
|----------|----------|---------|
| `urgent` | Immediate in-app + immediate email (bypasses digest) | `story.blocked`, `*.mentioned` (direct), `sprint.ending_soon` |
| `high` | Immediate in-app + immediate email | `story.assigned`, `*.mentioned` (team), `story.comment_added` |
| `normal` | Immediate in-app + daily digest | `story.status_changed`, `story.priority_changed`, `story.completed` |
| `low` | In-app only (no email unless user opts in) | `story.label_changed`, `story.due_date_changed`, `story.estimate_changed` |

---

## 5. Architecture

### 5.1 System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Event Sources                           │
│  (StoryService, CommentService, SprintService, etc.)        │
└─────────────┬───────────────────────────────────────────────┘
              │ Emit NotificationEvent within GORM transaction
              ▼
┌─────────────────────────────────────────────────────────────┐
│                  Notification Service                        │
│                                                             │
│  1. Resolve recipients (followers + mentioned + role-based) │
│  2. Check per-user notification preferences                 │
│  3. Deduplicate (don't notify actor of own action)          │
│  4. Enqueue River jobs per recipient per channel            │
└─────────────┬───────────────────────────────────────────────┘
              │ River job insert (same PostgreSQL transaction)
              ▼
┌─────────────────────────────────────────────────────────────┐
│                   River Job Queue                           │
│              (PostgreSQL-backed, river_job table)            │
│                                                             │
│  Queues:                                                    │
│  ├── notifications_critical  (priority: urgent mentions)    │
│  ├── notifications_default   (priority: standard delivery)  │
│  └── notifications_digest    (priority: batch/digest)       │
└─────────────┬──────────┬──────────┬─────────────────────────┘
              │          │          │
              ▼          ▼          ▼
┌──────────┐ ┌────────┐ ┌─────────────────┐
│ In-App   │ │ Email  │ │ Digest Worker   │
│ Worker   │ │ Worker │ │ (cron periodic) │
│          │ │        │ │                 │
│ - Write  │ │ - Send │ │ - Aggregate     │
│   to DB  │ │   via  │ │   unread notifs │
│ - Push   │ │   Post │ │ - Render digest │
│   via WS │ │   mark │ │ - Send via      │
│          │ │        │ │   Postmark      │
└──────────┘ └────────┘ └─────────────────┘
```

### 5.2 Component Design

#### 5.2.1 Notification Service (`internal/service/notification_service.go`)

Central orchestrator. All other services call into this when an event occurs.

```go
type NotificationService struct {
    notificationRepo  *NotificationRepository
    preferenceRepo    *NotificationPreferenceRepository
    followerRepo      *FollowerRepository
    riverClient       *river.Client[pgx.Tx]  // or database/sql
    wsPublisher       *websocket.Publisher
    emailClient       *email.Client
}

// Called by other services within their existing GORM transactions
func (s *NotificationService) Emit(ctx context.Context, tx *gorm.DB, event NotificationEvent) error {
    // 1. Resolve recipients from followers + explicit targets
    // 2. Filter out actor (don't self-notify)
    // 3. Check each recipient's preferences
    // 4. Upsert notifications row with latest title/priority/category/snapshots
    // 5. Insert notification_event row + notification_deliveries rows
    // 6. Enqueue River delivery jobs (within same tx)
}
```

#### 5.2.2 Follower System (`internal/service/follower_service.go`)

Manages entity subscriptions.

```go
type FollowerService struct {
    followerRepo *FollowerRepository
}

// Auto-follow triggers (called by other services):
// - Story created → creator follows
// - Story assigned → assignee follows
// - Comment added → commenter follows
// - @mention → mentioned user follows
// - Manual follow/unfollow via API

func (s *FollowerService) Follow(ctx context.Context, userID, entityType, entityID, workspaceID string) error
func (s *FollowerService) Unfollow(ctx context.Context, userID, entityType, entityID string) error
func (s *FollowerService) GetFollowers(ctx context.Context, entityType, entityID string) ([]string, error)
func (s *FollowerService) IsFollowing(ctx context.Context, userID, entityType, entityID string) (bool, error)
```

#### 5.2.3 Notification Preference Service (`internal/service/notification_preference_service.go`)

Manages per-user, per-channel, per-event-type preferences.

```go
type NotificationPreferenceService struct {
    preferenceRepo *NotificationPreferenceRepository
}

func (s *NotificationPreferenceService) GetPreferences(ctx context.Context, userID, workspaceID string) (*UserNotificationPreferences, error)
func (s *NotificationPreferenceService) UpdatePreferences(ctx context.Context, userID, workspaceID string, req UpdatePreferencesRequest) error
func (s *NotificationPreferenceService) ShouldNotify(ctx context.Context, userID, workspaceID string, eventType string, channel string) (bool, error)
```

#### 5.2.4 River Workers

**InAppNotificationWorker**: Persists notification to DB + pushes via WebSocket.

```go
type InAppNotificationArgs struct {
    NotificationID string `json:"notification_id"`
    RecipientID    string `json:"recipient_id"`
    WorkspaceID    string `json:"workspace_id"`
}

func (w *InAppNotificationWorker) Work(ctx context.Context, job *river.Job[InAppNotificationArgs]) error {
    // 1. Fetch notification row (snapshots included, no inbox-time joins)
    // 2. Push to recipient via WebSocket hub
    // 3. Update corresponding in_app delivery status
}
```

**EmailNotificationWorker**: Sends immediate email notifications.

```go
type EmailNotificationArgs struct {
    RecipientID    string `json:"recipient_id"`
    NotificationID string `json:"notification_id"`
    EventType      string `json:"event_type"`
}

func (w *EmailNotificationWorker) Work(ctx context.Context, job *river.Job[EmailNotificationArgs]) error {
    // 1. Check if user already read the in-app notification (smart dedup)
    // 2. If unread, render email template and send via Postmark
    // 3. Update delivery status
}
```

**DigestWorker**: Cron periodic job (runs daily at user's preferred time).

```go
type DigestArgs struct {
    WorkspaceID string `json:"workspace_id"`
}

func (w *DigestWorker) Work(ctx context.Context, job *river.Job[DigestArgs]) error {
    // 1. Query all users with digest enabled in this workspace
    // 2. For each user, aggregate unread notifications since last digest
    // 3. Group by entity for readability
    // 4. Render digest email template
    // 5. Send via Postmark
    // 6. Mark notifications as "digest_sent"
}
```

### 5.3 Real-Time Delivery Flow

```
Service emits event
    → NotificationService.Emit() (within GORM tx)
        → UPSERT notifications (latest title/category/snapshots)
        → INSERT INTO notification_events + notification_deliveries
        → River.Insert(InAppNotificationArgs) (same tx)
    → Transaction commits
    → River picks up job (~50ms)
        → InAppNotificationWorker
            → WebSocket Hub.Broadcast(notification event to recipient)
            → Frontend receives via useWebSocket
            → useNotifications hook invalidates query cache
            → NotificationCenter UI updates
```

---

## 6. Data Model

### 6.1 Database Tables

#### `notifications`
Primary notification storage. One row per recipient per entity (entity-centric, not event-centric).

```sql
CREATE TABLE notifications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    recipient_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id        UUID REFERENCES users(id) ON DELETE SET NULL,

    -- Entity reference (entity-centric: one row per entity per recipient)
    entity_type     VARCHAR(50) NOT NULL,  -- 'story', 'epic', 'sprint', 'objective', 'comment'
    entity_id       UUID NOT NULL,

    -- Latest event info (updated on each new event for same entity)
    event_type      VARCHAR(100) NOT NULL, -- 'story.assigned', 'story.comment_added', etc.
    title           TEXT NOT NULL,          -- Human-readable notification title
    body            TEXT,                   -- Optional detail text
    metadata        JSONB DEFAULT '{}',    -- Flexible payload (old/new values, comment preview, etc.)
    latest_event_category VARCHAR(50) NOT NULL, -- Coarse grouping for the latest event shown in the inbox

    -- Denormalized render snapshots (latest render state for this aggregated row)
    actor_snapshot  JSONB NOT NULL DEFAULT '{}'::jsonb,
                    -- { "id", "name", "avatar_url", "type" }
    entity_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
                    -- { "title", "identifier", "url", "state", "priority" }
    parent_entity_snapshot JSONB,
                    -- Optional parent context: { "type", "id", "title", "identifier" }

    -- Event aggregation
    event_count     INTEGER DEFAULT 1,     -- Number of events rolled into this notification
    last_event_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- State
    status          VARCHAR(20) NOT NULL DEFAULT 'unread', -- 'unread', 'read', 'archived'
    snoozed_until   TIMESTAMPTZ,
    read_at         TIMESTAMPTZ,
    archived_at     TIMESTAMPTZ,

    -- Priority (highest priority among aggregated events)
    priority        VARCHAR(10) NOT NULL DEFAULT 'normal', -- 'urgent', 'high', 'normal', 'low'

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT uq_notification_recipient_entity
        UNIQUE (recipient_id, entity_type, entity_id, workspace_id)
);

-- Indexes
CREATE INDEX idx_notifications_recipient_status ON notifications(recipient_id, status, last_event_at DESC);
CREATE INDEX idx_notifications_recipient_workspace ON notifications(recipient_id, workspace_id, status);
CREATE INDEX idx_notifications_snoozed ON notifications(snoozed_until) WHERE snoozed_until IS NOT NULL;
CREATE INDEX idx_notifications_entity ON notifications(entity_type, entity_id);
```

> **Design Note: Why snapshots live on the aggregated `notifications` row.**
>
> `actor_snapshot`, `entity_snapshot`, and `parent_entity_snapshot` are display-oriented payloads written by the application so the inbox can render without joining `users`, `pm_stories`, `pm_epics`, etc. The canonical relational keys remain `recipient_id`, `actor_id`, `entity_type`, `entity_id`, and `workspace_id`. Because the row is entity-centric and updated on every new event, these snapshots represent the latest render state of the aggregated row; historical event-level context remains in `notification_events`.

#### `notification_events`
Individual events that contribute to a notification. Each event tracks its own per-channel delivery state, solving the aggregation-vs-delivery conflict (see Design Note below).

```sql
CREATE TABLE notification_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    actor_id        UUID REFERENCES users(id) ON DELETE SET NULL,
    event_type      VARCHAR(100) NOT NULL,
    title           TEXT NOT NULL,
    metadata        JSONB DEFAULT '{}',
    category        VARCHAR(50) NOT NULL,  -- Coarse grouping for UI/analytics/future filters
    actor_snapshot  JSONB NOT NULL DEFAULT '{}'::jsonb,
                    -- Actor snapshot preserved at event time for timeline/email rendering
    priority        VARCHAR(10) NOT NULL DEFAULT 'normal',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notification_events_notification ON notification_events(notification_id, created_at DESC);
```

> `notification_events.category` is a coarse grouping field. `event_type` remains the canonical routing key for preference evaluation and delivery behavior.

#### `notification_deliveries`
Per-event, per-channel delivery tracking. Decoupled from the aggregated `notifications` row so that new events on the same entity can independently track whether they still need email/digest/Mattermost delivery.

```sql
CREATE TABLE notification_deliveries (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_event_id UUID NOT NULL REFERENCES notification_events(id) ON DELETE CASCADE,
    channel             VARCHAR(30) NOT NULL,  -- 'in_app', 'email', 'digest', 'mattermost'
    status              VARCHAR(20) NOT NULL DEFAULT 'pending', -- 'pending', 'delivered', 'skipped', 'failed'
    delivered_at        TIMESTAMPTZ,
    error               TEXT,
    external_message_id VARCHAR(255),          -- Postmark message ID, Mattermost post ID, etc.
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notification_deliveries_event ON notification_deliveries(notification_event_id);
CREATE INDEX idx_notification_deliveries_pending ON notification_deliveries(channel, status) WHERE status = 'pending';
```

> **Design Note: Why delivery lives on events, not notifications.**
>
> The `notifications` table is entity-centric: one row per (recipient, entity). Multiple events on the same story upsert this row. If delivery flags (`email_delivered`, `digest_included`) lived on the notifications row, a later event would inherit stale delivery state — e.g., a second comment would appear "already emailed" because the first comment set `email_delivered = true`. By tracking delivery per event per channel, each new event independently determines whether it needs email/digest/Mattermost delivery, regardless of prior events on the same entity.

#### `notification_preferences`
Per-user, per-workspace notification settings.

```sql
CREATE TABLE notification_preferences (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,

    -- Global toggles
    do_not_disturb      BOOLEAN DEFAULT FALSE,
    dnd_until           TIMESTAMPTZ,

    -- Email settings
    email_enabled       BOOLEAN DEFAULT TRUE,
    email_digest_frequency VARCHAR(20) DEFAULT 'daily', -- 'immediate', 'daily', 'weekly', 'none'
    email_digest_time   TIME DEFAULT '09:00',           -- User's preferred digest delivery time
    email_digest_day    INTEGER DEFAULT 1,              -- Day of week for weekly digest (1=Mon)
    timezone            VARCHAR(50) DEFAULT 'UTC',      -- IANA timezone (e.g., 'America/New_York') for digest scheduling

    -- Per-event-type channel preferences (JSONB for flexibility)
    -- Structure: { "story.assigned": { "in_app": true, "email": true }, ... }
    channel_preferences JSONB DEFAULT '{}',

    -- Activity badge sensitivity
    badge_mode          VARCHAR(20) DEFAULT 'all', -- 'all', 'mentions_only', 'none'

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_notification_pref_user_workspace
        UNIQUE (user_id, workspace_id)
);
```

#### `entity_followers`
Tracks who follows which entity.

```sql
CREATE TABLE entity_followers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entity_type     VARCHAR(50) NOT NULL,  -- 'story', 'epic', 'objective', 'sprint'
    entity_id       UUID NOT NULL,
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    reason          VARCHAR(30) NOT NULL DEFAULT 'manual', -- 'manual', 'assigned', 'created', 'commented', 'mentioned'
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_entity_follower
        UNIQUE (user_id, entity_type, entity_id)
);

CREATE INDEX idx_entity_followers_entity ON entity_followers(entity_type, entity_id);
CREATE INDEX idx_entity_followers_user ON entity_followers(user_id, workspace_id);
```

### 6.2 GORM Models

```go
// Notification represents a single notification for a recipient about an entity.
// Entity-centric: one notification per (recipient, entity) pair, updated on new events.
type Notification struct {
    ID            string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    WorkspaceID   string     `json:"workspace_id" gorm:"type:uuid;not null"`
    RecipientID   string     `json:"recipient_id" gorm:"type:uuid;not null"`
    ActorID       *string    `json:"actor_id" gorm:"type:uuid"`

    EntityType    string     `json:"entity_type" gorm:"type:varchar(50);not null"`
    EntityID      string     `json:"entity_id" gorm:"type:uuid;not null"`

    EventType     string     `json:"event_type" gorm:"type:varchar(100);not null"`
    Title         string     `json:"title" gorm:"not null"`
    Body          *string    `json:"body"`
    Metadata      JSON       `json:"metadata" gorm:"type:jsonb;default:'{}'"`
    LatestEventCategory string `json:"latest_event_category" gorm:"type:varchar(50);not null"`

    // Snapshots power inbox rendering without joining users or PM entities.
    ActorSnapshot JSON       `json:"actor_snapshot" gorm:"type:jsonb;default:'{}'"`
    EntitySnapshot JSON      `json:"entity_snapshot" gorm:"type:jsonb;default:'{}'"`
    ParentEntitySnapshot JSON `json:"parent_entity_snapshot" gorm:"type:jsonb"`

    EventCount    int        `json:"event_count" gorm:"default:1"`
    LastEventAt   time.Time  `json:"last_event_at" gorm:"autoCreateTime"`

    Status        string     `json:"status" gorm:"type:varchar(20);default:'unread'"`
    SnoozedUntil  *time.Time `json:"snoozed_until"`
    ReadAt        *time.Time `json:"read_at"`
    ArchivedAt    *time.Time `json:"archived_at"`

    Priority      string     `json:"priority" gorm:"type:varchar(10);default:'normal'"`

    // Optional relations for repair/backfill/admin paths. Inbox render should use snapshots.
    Actor         *User      `json:"actor,omitempty" gorm:"foreignKey:ActorID"`
    Events        []NotificationEvent `json:"events,omitempty" gorm:"foreignKey:NotificationID"`

    CreatedAt     time.Time  `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt     time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Notification) TableName() string { return "notifications" }

type NotificationEvent struct {
    ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    NotificationID string    `json:"notification_id" gorm:"type:uuid;not null"`
    ActorID        *string   `json:"actor_id" gorm:"type:uuid"`
    EventType      string    `json:"event_type" gorm:"type:varchar(100);not null"`
    Title          string    `json:"title" gorm:"not null"`
    Metadata       JSON      `json:"metadata" gorm:"type:jsonb;default:'{}'"`
    Category       string    `json:"category" gorm:"type:varchar(50);not null"`
    ActorSnapshot  JSON      `json:"actor_snapshot" gorm:"type:jsonb;default:'{}'"`
    Priority       string    `json:"priority" gorm:"type:varchar(10);default:'normal'"`
    Actor          *User     `json:"actor,omitempty" gorm:"foreignKey:ActorID"`
    Deliveries     []NotificationDelivery `json:"deliveries,omitempty" gorm:"foreignKey:NotificationEventID"`
    CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (NotificationEvent) TableName() string { return "notification_events" }

// NotificationDelivery tracks per-event, per-channel delivery state.
type NotificationDelivery struct {
    ID                  string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    NotificationEventID string     `json:"notification_event_id" gorm:"type:uuid;not null"`
    Channel             string     `json:"channel" gorm:"type:varchar(30);not null"`   // 'in_app', 'email', 'digest', 'mattermost'
    Status              string     `json:"status" gorm:"type:varchar(20);default:'pending'"` // 'pending', 'delivered', 'skipped', 'failed'
    DeliveredAt         *time.Time `json:"delivered_at"`
    Error               *string    `json:"error"`
    ExternalMessageID   *string    `json:"external_message_id" gorm:"type:varchar(255)"`
    CreatedAt           time.Time  `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt           time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (NotificationDelivery) TableName() string { return "notification_deliveries" }

type NotificationPreference struct {
    ID                  string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    UserID              string     `json:"user_id" gorm:"type:uuid;not null"`
    WorkspaceID         string     `json:"workspace_id" gorm:"type:uuid;not null"`
    DoNotDisturb        bool       `json:"do_not_disturb" gorm:"default:false"`
    DNDUntil            *time.Time `json:"dnd_until"`
    EmailEnabled        bool       `json:"email_enabled" gorm:"default:true"`
    EmailDigestFrequency string   `json:"email_digest_frequency" gorm:"type:varchar(20);default:'daily'"`
    EmailDigestTime     string     `json:"email_digest_time" gorm:"type:time;default:'09:00'"`
    EmailDigestDay      int        `json:"email_digest_day" gorm:"default:1"`
    Timezone            string     `json:"timezone" gorm:"type:varchar(50);default:'UTC'"` // IANA timezone for digest scheduling
    ChannelPreferences  JSON       `json:"channel_preferences" gorm:"type:jsonb;default:'{}'"`
    BadgeMode           string     `json:"badge_mode" gorm:"type:varchar(20);default:'all'"`
    CreatedAt           time.Time  `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt           time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (NotificationPreference) TableName() string { return "notification_preferences" }

type EntityFollower struct {
    ID          string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    UserID      string    `json:"user_id" gorm:"type:uuid;not null"`
    EntityType  string    `json:"entity_type" gorm:"type:varchar(50);not null"`
    EntityID    string    `json:"entity_id" gorm:"type:uuid;not null"`
    WorkspaceID string    `json:"workspace_id" gorm:"type:uuid;not null"`
    Reason      string    `json:"reason" gorm:"type:varchar(30);default:'manual'"`
    CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (EntityFollower) TableName() string { return "entity_followers" }
```

---

## 7. API Design

### 7.1 Notification Endpoints

```
# Inbox
GET    /api/workspaces/{id}/notifications                    # List notifications (paginated)
GET    /api/workspaces/{id}/notifications/unread-count        # Get unread count
PATCH  /api/workspaces/{id}/notifications/{notifId}           # Update notification (read/archive/snooze)
POST   /api/workspaces/{id}/notifications/mark-all-read       # Mark all as read
POST   /api/workspaces/{id}/notifications/archive-all-read    # Archive all read notifications
DELETE /api/workspaces/{id}/notifications/{notifId}            # Delete a notification

# Preferences
GET    /api/workspaces/{id}/notifications/preferences         # Get user's notification preferences
PUT    /api/workspaces/{id}/notifications/preferences         # Update preferences

# Followers
GET    /api/workspaces/{id}/pm/{entityType}/{entityId}/followers      # List followers of an entity
POST   /api/workspaces/{id}/pm/{entityType}/{entityId}/followers      # Follow an entity
DELETE /api/workspaces/{id}/pm/{entityType}/{entityId}/followers       # Unfollow an entity
GET    /api/workspaces/{id}/notifications/following                    # List all entities user follows
```

### 7.2 Request/Response Examples

#### List Notifications
```
GET /api/workspaces/{id}/notifications?status=unread&limit=20&cursor=xxx

Response:
{
  "data": [
    {
      "id": "uuid",
      "entity_type": "story",
      "entity_id": "uuid",
      "event_type": "story.comment_added",
      "latest_event_category": "comment",
      "title": "Sarah commented on \"Fix login bug\"",
      "body": "I think the issue is in the auth middleware...",
      "entity": {
        "title": "Fix login bug",
        "identifier": "ENG-142",
        "url": "/stories/ENG-142",
        "state": "In Review"
      },
      "parent_entity": null,
      "metadata": {
        "comment_preview": "I think the issue is in the auth middleware..."
      },
      "actor": {
        "id": "uuid",
        "name": "Sarah Chen",
        "avatar_url": "..."
      },
      "event_count": 3,
      "last_event_at": "2026-03-08T14:32:00Z",
      "status": "unread",
      "priority": "high",
      "created_at": "2026-03-08T14:00:00Z"
    }
  ],
  "next_cursor": "xxx",
  "unread_count": 12
}
```

`actor`, `entity`, and `parent_entity` in the response are served directly from snapshot columns on the notification row. The inbox API should not need joins to render the primary list view.

#### Update Notification
```
PATCH /api/workspaces/{id}/notifications/{notifId}

Request:
{ "status": "read" }
// or
{ "status": "archived" }
// or
{ "snoozed_until": "2026-03-09T09:00:00Z" }
```

#### Get/Update Preferences
```
GET /api/workspaces/{id}/notifications/preferences

Response:
{
  "do_not_disturb": false,
  "dnd_until": null,
  "email_enabled": true,
  "email_digest_frequency": "daily",
  "email_digest_time": "09:00",
  "badge_mode": "all",
  "channel_preferences": {
    "story.assigned":        { "in_app": true, "email": true },
    "story.comment_added":   { "in_app": true, "email": true },
    "story.status_changed":  { "in_app": true, "email": false },
    "story.mentioned":       { "in_app": true, "email": true },
    "story.completed":       { "in_app": true, "email": false },
    "story.blocked":         { "in_app": true, "email": true },
    "epic.comment_added":    { "in_app": true, "email": true },
    "sprint.started":        { "in_app": true, "email": true },
    "sprint.ending_soon":    { "in_app": true, "email": true }
  }
}
```

### 7.3 WebSocket Events

Extend the existing WebSocket event structure:

```go
// New WebSocket event types for notifications
Event{
    Action:      "created",           // or "updated"
    Entity:      "notification",
    EntityID:    notificationID,
    WorkspaceID: workspaceID,
    ActorID:     actorID,
    // Additional data sent in the WebSocket message payload
}
```

Frontend receives via existing `useWebSocket` → `useRealtimeSync` invalidates notification queries.

### 7.4 Permissions

| Endpoint | Permission |
|----------|-----------|
| `GET /notifications` | `PermWorkspaceRead` (any member) |
| `PATCH /notifications/{id}` | Owner of notification (recipient_id == current user) |
| `GET/PUT /notifications/preferences` | Own preferences only |
| `POST/DELETE /followers` | `PermPMRead` (any PM user) |

---

## 8. Frontend Design

### 8.1 Notification Center Component

Located in the top navigation bar (header). Inspired by Linear's sidebar inbox + Shortcut's activity button.

```
┌─────────────────────────────────────────────┐
│  🔔 (3)  ← Bell icon with unread badge      │
└─────┬───────────────────────────────────────┘
      │ Click
      ▼
┌─────────────────────────────────────────────┐
│ Notifications                    Mark all ✓ │
│─────────────────────────────────────────────│
│ [All] [Mentions] [Assigned]   ← Tab filters │
│─────────────────────────────────────────────│
│ ● Sarah commented on ENG-142     2m ago     │
│   "I think the issue is in..."              │
│─────────────────────────────────────────────│
│ ● You were assigned ENG-155      15m ago    │
│   "Implement user avatar upload"            │
│─────────────────────────────────────────────│
│ ○ Sprint 12 started              1h ago     │
│   "Backend Sprint - March 2026"             │
│─────────────────────────────────────────────│
│ ○ 3 updates on ENG-130           3h ago     │
│   "Fix payment processing"                  │
│─────────────────────────────────────────────│
│         Load more...                        │
└─────────────────────────────────────────────┘

● = unread    ○ = read
```

### 8.2 Tab Filters

| Tab | Description |
|-----|-------------|
| **All** | All notifications (default) |
| **Mentions** | Only notifications where user was @-mentioned |
| **Assigned** | Only assignment-related notifications |

### 8.3 Notification Row Actions

Available via right-click context menu or hover action buttons:

| Action | Description |
|--------|-------------|
| Mark as read/unread | Toggle read status |
| Archive | Remove from inbox (retrievable) |
| Snooze | Hide until selected time (1h, tomorrow, next week, custom) |
| Unfollow entity | Stop following the story/epic/objective |
| Go to entity | Navigate to the full entity page |

### 8.4 Notification Preferences Page

Located at: `/w/{slug}/settings/notifications`

```
┌─────────────────────────────────────────────────────────────┐
│ Notification Settings                                        │
│─────────────────────────────────────────────────────────────│
│                                                             │
│ General                                                     │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ Do Not Disturb          [Toggle]                        │ │
│ │ Mute all notifications until...    [Date picker]        │ │
│ │                                                         │ │
│ │ Activity Badge           ◉ All activity                 │ │
│ │                          ○ Mentions only                 │ │
│ │                          ○ None                          │ │
│ └─────────────────────────────────────────────────────────┘ │
│                                                             │
│ Email                                                       │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ Email notifications      [Toggle: On]                   │ │
│ │ Digest frequency         [Dropdown: Daily ▾]            │ │
│ │ Delivery time            [Time picker: 9:00 AM]         │ │
│ └─────────────────────────────────────────────────────────┘ │
│                                                             │
│ Event Preferences                                           │
│ ┌────────────────────────────────┬────────┬────────┐       │
│ │ Event                          │ In-App │ Email  │       │
│ ├────────────────────────────────┼────────┼────────┤       │
│ │ Assigned to a story            │  [✓]   │  [✓]   │       │
│ │ @mentioned                     │  [✓]   │  [✓]   │       │
│ │ New comment on followed story  │  [✓]   │  [✓]   │       │
│ │ Story status changed           │  [✓]   │  [ ]   │       │
│ │ Story completed                │  [✓]   │  [ ]   │       │
│ │ Story blocked                  │  [✓]   │  [✓]   │       │
│ │ Sprint started/ended           │  [✓]   │  [✓]   │       │
│ │ Sprint ending soon             │  [✓]   │  [✓]   │       │
│ │ Epic updates                   │  [✓]   │  [ ]   │       │
│ │ Objective progress             │  [✓]   │  [ ]   │       │
│ └────────────────────────────────┴────────┴────────┘       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 8.5 Frontend Architecture

```
src/
  components/
    notifications/
      NotificationCenter.tsx       # Popover component in header
      NotificationList.tsx         # Virtualized notification list
      NotificationRow.tsx          # Individual notification row
      NotificationActions.tsx      # Context menu actions
      NotificationPreferences.tsx  # Settings page component
      NotificationBadge.tsx        # Unread count badge
      FollowButton.tsx             # Follow/unfollow entity button
  hooks/
    queries/
      useNotifications.ts          # TanStack Query hooks for notifications
  lib/
    services/
      notificationsService.ts      # API service adapter
    notificationTypes.ts           # TypeScript interfaces
    queryKeys.ts                   # Extended with notification keys
  stores/
    notificationStore.ts           # Zustand store for UI state (popover open, etc.)
```

### 8.6 TanStack Query Hooks

```typescript
// queryKeys.ts additions
notifications: {
  all: (wsId: string) => ['notifications', { wsId }],
  list: (wsId: string, filters: NotificationFilters) => ['notifications', { wsId, ...filters }],
  unreadCount: (wsId: string) => ['notifications', 'unread-count', { wsId }],
  preferences: (wsId: string) => ['notifications', 'preferences', { wsId }],
  following: (wsId: string) => ['notifications', 'following', { wsId }],
}

// useNotifications.ts
export function useNotifications(wsId: string, filters: NotificationFilters) {
  return useInfiniteQuery({
    queryKey: queryKeys.notifications.list(wsId, filters),
    queryFn: ({ pageParam }) => unwrap(
      await notificationsService.list(wsId, { ...filters, cursor: pageParam })
    ),
    getNextPageParam: (lastPage) => lastPage.next_cursor,
  });
}

export function useUnreadCount(wsId: string) {
  return useQuery({
    queryKey: queryKeys.notifications.unreadCount(wsId),
    queryFn: () => unwrap(await notificationsService.getUnreadCount(wsId)),
    refetchInterval: 30_000, // Fallback polling (WebSocket is primary)
  });
}

export function useMarkAsRead(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (notifId: string) => notificationsService.update(wsId, notifId, { status: 'read' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all(wsId) });
    },
  });
}
```

### 8.7 Real-Time Integration

Extend `useRealtimeSync` to handle notification events:

```typescript
// In useRealtimeSync.ts
case 'notification':
  queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all(workspaceId) });
  queryClient.invalidateQueries({ queryKey: queryKeys.notifications.unreadCount(workspaceId) });
  break;
```

---

## 9. Notification Preference Defaults

New users get sensible defaults. All preferences are overridable.

| Event Type | In-App | Email |
|------------|--------|-------|
| `story.assigned` | On | On |
| `story.unassigned` | On | Off |
| `story.status_changed` | On | Off |
| `story.priority_changed` | On | Off |
| `story.comment_added` | On | On |
| `story.mentioned` | On | On (immediate for direct, immediate for team) |
| `story.completed` | On | Off |
| `story.estimate_changed` | Off | Off |
| `story.due_date_changed` | On | Off |
| `story.label_changed` | Off | Off |
| `story.moved_to_sprint` | On | Off |
| `story.blocked` | On | On |
| `story.unblocked` | On | Off |
| `epic.status_changed` | On | Off |
| `epic.comment_added` | On | On |
| `epic.mentioned` | On | On (immediate for direct, immediate for team) |
| `epic.completed` | On | Off |
| `sprint.started` | On | On |
| `sprint.completed` | On | On |
| `sprint.ending_soon` | On | On |
| `objective.status_changed` | On | Off |
| `objective.comment_added` | On | Off |
| `objective.mentioned` | On | On (immediate) |

---

## 10. Smart Behaviors

### 10.1 Smart Email Deduplication (Linear-inspired)
If a user reads the in-app notification before the email worker processes it, skip the email. This prevents redundant alerts for active users.

### 10.2 Entity-Centric Consolidation
Multiple events on the same entity (e.g., 3 status changes on the same story) update the existing notification row rather than creating 3 separate notifications. The `event_count` increments and `last_event_at` updates, keeping the notification at the top of the inbox.

### 10.2.1 State Transition Rules

When a new event arrives for an entity that already has a notification row, the behavior depends on the current state of that row:

| Current Status | New Event Priority | Resulting State | Behavior |
|---------------|-------------------|----------------|----------|
| `unread` | any | `unread` | Update title, increment event_count, update last_event_at. Escalate priority if new event is higher. |
| `read` | `normal` or `low` | `unread` | Reset to unread, clear read_at, update title/event_count/last_event_at. Notification re-appears at top of inbox. |
| `read` | `urgent` or `high` | `unread` | Same as above. New delivery records created for the new event (email/Mattermost as applicable). |
| `archived` | `normal` or `low` | `archived` | Insert notification_event row for audit, but do NOT un-archive. User explicitly dismissed this entity. |
| `archived` | `urgent` | `unread` | Un-archive: clear archived_at, set status=unread. Urgent events override user's archive. |
| `snoozed` | `normal` or `low` | `snoozed` | Insert notification_event, increment event_count, but notification stays hidden until snooze expires. |
| `snoozed` | `urgent` | `unread` | Break snooze: clear snoozed_until, set status=unread. Urgent events override snooze. |

**Priority escalation rule:** The notification row's `priority` is always set to `max(current_priority, new_event_priority)`. Priority ordering: `urgent > high > normal > low`.

**Key principle:** `archived` means "I'm done with this entity" — only urgent events can override it. `snoozed` means "remind me later" — only urgent events can break through. `read` means "I saw it" — any new activity makes it unread again.

### 10.3 Self-Notification Suppression
Never notify the actor of their own action. If user A assigns a story to themselves, they don't receive an "assigned" notification.

### 10.4 Snooze Handling
Snoozed notifications are hidden from the inbox. A River periodic job runs every minute, checks for notifications where `snoozed_until <= NOW()`, clears the snooze, and re-delivers via WebSocket so they appear as fresh.

### 10.5 Auto-Follow Rules
| Trigger | Entity Type | Reason |
|---------|------------|--------|
| User creates a story | Story | `created` |
| User is assigned to a story | Story | `assigned` |
| User comments on a story | Story | `commented` |
| User is @-mentioned | Story/Epic/Objective | `mentioned` |
| User manually follows | Any | `manual` |

### 10.6 Unfollow Behavior
When a user unfollows an entity, they stop receiving new notifications but existing notifications remain in their inbox.

### 10.7 Migration from Existing `pm_story_followers`

The codebase already has a `pm_story_followers` join table (`PMStoryFollower` model in `server/internal/model/pm_story.go`) that tracks story followers. The new `entity_followers` table is a superset (supports stories, epics, objectives, sprints) with additional metadata (`reason`, `workspace_id`).

**Migration strategy:**
1. Create the new `entity_followers` table in migration 023.
2. In the same migration, backfill from existing data:
   ```sql
   INSERT INTO entity_followers (user_id, entity_type, entity_id, workspace_id, reason, created_at)
   SELECT sf.user_id, 'story', sf.story_id, s.workspace_id, 'manual', sf.created_at
   FROM pm_story_followers sf
   JOIN pm_stories s ON s.id = sf.story_id
   ON CONFLICT (user_id, entity_type, entity_id) DO NOTHING;
   ```
3. Update `FollowerService` to write to `entity_followers` and read from `entity_followers`.
4. Update `StoryService` to use `FollowerService.Follow/Unfollow` instead of directly inserting `pm_story_followers`.
5. Keep `pm_story_followers` as a read-only legacy table during transition. Drop in a future migration once all story follow/unfollow paths are migrated.
6. The `PMStoryFollower` GORM model and its `TableName()` remain until the legacy table is dropped.

### 10.8 Relationship to `notifications_enabled` Workspace Setting

The existing `WorkspaceSettings.NotificationsEnabled` field (`server/internal/model/settings.go:11`) currently controls a narrow scope: the frontend toggle is labeled **"Send email notifications for sprint events"** (`frontend/src/pages/Settings.tsx:2521`). Under the new system, this legacy setting is repurposed:

- **Rename the setting** in the UI from "Send email notifications for sprint events" to "Enable notifications" to reflect its expanded scope.
- **`notifications_enabled = false`**: Disables ALL notification delivery for the workspace (no in-app, no email, no Mattermost). Notification events are still recorded in the database for audit, but no River delivery jobs are enqueued. This is the workspace admin's master switch.
- **`notifications_enabled = true`** (default): Per-user preferences and per-event-type channel settings apply as described in this PRD.
- **Migration note**: Workspaces that currently have `notifications_enabled = false` (opted out of sprint emails) will start with all notifications disabled. This is intentional — admins who disabled the old system should explicitly opt into the new one.
- The per-user `do_not_disturb` flag operates independently: it mutes notifications for that user only, even when the workspace has notifications enabled.

---

## 11. Notification Flow Examples

Concrete scenarios showing the full flow from trigger to what the user sees in each channel.

### 11.1 Direct @-mention in a comment

**Context:** Sarah comments on story ENG-142 and writes `@John can you review the auth logic?`

**Flow:**
```
1. Sarah submits comment (CommentService.Create)
   │
   2. Comment parser extracts mentioned user IDs from @-mention markup
   │
   3. NotificationService.Emit() within same DB transaction:
   │   ├── Resolves recipients:
   │   │   ├── John (explicitly mentioned → mention.direct)
   │   │   ├── All existing followers of ENG-142 (story.comment_added)
   │   │   └── Exclude Sarah (self-notification suppression)
   │   │
   │   ├── Auto-follows John to ENG-142 (reason: "mentioned")
   │   │
   │   ├── For John specifically → ONE notification event:
   │   │   ├── story.mentioned (mention_type: direct, priority: urgent → immediate email)
   │   │
   │   └── For other followers → ONE notification event:
   │       └── story.comment_added (priority: high → digest email)
   │
   4. River jobs enqueued (same transaction):
       ├── InAppNotificationWorker × N recipients
       └── EmailNotificationWorker × 1 (John only, urgent)
```

**What John sees in his inbox:**

```
┌─────────────────────────────────────────────────┐
│ ● Sarah mentioned you on "Fix login bug"  2m ago│
│   @John can you review the auth logic?          │
│   ENG-142                                       │
└─────────────────────────────────────────────────┘
```

- Priority badge: `urgent` (direct mention)
- Email: sent **immediately** (bypasses digest)
- WebSocket: pushed in real-time, badge count increments

**What other followers see:**

```
┌─────────────────────────────────────────────────┐
│ ● Sarah commented on "Fix login bug"      2m ago│
│   @John can you review the auth logic?          │
│   ENG-142                                       │
└─────────────────────────────────────────────────┘
```

- Priority: `high`
- Email: included in next **daily digest** (unless user opted into immediate)

---

### 11.2 @-mention a team

**Context:** Sarah comments on epic EP-5 and writes `@Backend Team this needs your input on the API design`

**Flow:**
```
1. Comment parser detects team mention → resolves team membership
   │   Backend Team has 6 members: John, Alex, Maria, Dev, Priya, Raj
   │
   2. NotificationService.Emit():
   │   ├── Recipients:
   │   │   ├── 6 team members (epic.mentioned, mention_type: team, priority: high)
   │   │   ├── Existing EP-5 followers (epic.comment_added)
   │   │   └── Deduplicated: if John already follows EP-5, he gets ONE notification
   │   │       with the highest priority event (epic.mentioned > epic.comment_added)
   │   │   └── Exclude Sarah (even if she's on Backend Team)
   │   │
   │   ├── Auto-follows all 6 team members to EP-5
   │   │
   │   └── River fan-out: batch insert jobs for all recipients
   │
   3. River picks up jobs (~50ms):
       ├── InAppNotificationWorker × (6 team + other followers - Sarah)
       └── EmailNotificationWorker × 6 (team mentions get immediate email)
```

**What each Backend Team member sees:**

```
┌──────────────────────────────────────────────────────┐
│ ● Sarah mentioned @Backend Team on "API Redesign"    │
│   this needs your input on the API design      1m ago│
│   EP-5                                               │
└──────────────────────────────────────────────────────┘
```

**What non-team followers of EP-5 see:**

```
┌──────────────────────────────────────────────────────┐
│ ● Sarah commented on "API Redesign"            1m ago│
│   @Backend Team this needs your input...             │
│   EP-5                                               │
└──────────────────────────────────────────────────────┘
```

---

### 11.3 Entity-centric consolidation

**Context:** Over 2 hours, three things happen on ENG-142:
1. Sarah changes status: In Progress → In Review
2. Alex adds a comment: "LGTM"
3. Sarah changes status: In Review → Done

**What John (follower) sees — NOT 3 separate notifications, but ONE consolidated row:**

```
┌──────────────────────────────────────────────────────┐
│ ● "Fix login bug" was completed             just now │
│   3 updates · Sarah, Alex                            │
│   ENG-142                                            │
└──────────────────────────────────────────────────────┘
```

Click to expand shows the individual events from the `notification_events` table:

```
┌──────────────────────────────────────────────────────┐
│  Sarah marked as Done                        2:15 PM │
│  Alex commented: "LGTM"                      2:10 PM │
│  Sarah moved to In Review                    12:30 PM│
└──────────────────────────────────────────────────────┘
```

This is the entity-centric model in action — one `notifications` row (latest event title/category plus actor/entity snapshots, `event_count: 3`), three `notification_events` rows for the detailed timeline and per-channel delivery tracking.

---

### 11.4 Preference filtering in action

**John's preferences:**
```json
{
  "story.status_changed":  { "in_app": true,  "email": false },
  "story.comment_added":   { "in_app": true,  "email": true  },
  "story.mentioned":       { "in_app": true,  "email": true  },
  "story.label_changed":   { "in_app": false, "email": false }
}
```

| Event | In-App | Email | Result |
|-------|--------|-------|--------|
| Assigned to a story | Shows in inbox | Included in digest | Both channels |
| Someone changes labels on followed story | **Suppressed** | **Suppressed** | Nothing — John opted out |
| @-mentioned in comment (`story.mentioned`) | Shows in inbox | **Immediate** email | Both, urgent |
| Story status changes | Shows in inbox | **Skipped** | In-app only |

---

### 11.5 Smart email deduplication

**Context:** Sarah assigns ENG-155 to John at 2:00 PM. Email digest is scheduled for 9:00 AM next day.

```
Timeline:
  2:00 PM  → In-app notification delivered via WebSocket
  2:05 PM  → John opens Helpin, reads the notification
  2:05 PM  → Notification status updated to "read"
  ...
  9:00 AM  → DigestWorker runs, checks John's unread notifications
           → ENG-155 assignment already read → email_skipped = true
           → Email NOT sent (prevents redundant alert)
```

If John had NOT read the in-app notification by 9:00 AM, the digest would include it.

---

### 11.6 Notification appearance summary by trigger

| Trigger | Event Type | Notification Title | Priority | Email Behavior |
|---------|-----------|-------------------|----------|---------------|
| `@John` in comment | `story.mentioned` (direct) | "Sarah mentioned you on ..." | `urgent` | Immediate |
| `@Backend Team` in comment | `epic.mentioned` (team) | "Sarah mentioned @Backend Team on ..." | `high` | Immediate |
| Comment on followed story | `story.comment_added` | "Sarah commented on ..." | `high` | Immediate |
| Assigned to story | `story.assigned` | "Sarah assigned you ..." | `high` | Immediate |
| Status change | `story.status_changed` | "\"Fix login bug\" moved to Done" | `normal` | Digest (if opted in) |
| Story blocked | `story.blocked` | "\"Fix login bug\" is blocked" | `urgent` | Immediate |
| Sprint ending | `sprint.ending_soon` | "Sprint 12 ends tomorrow" | `urgent` | Immediate |
| Estimate changed | `story.estimate_changed` | "\"Fix login bug\" estimate updated" | `low` | In-app only |
| Label changed | `story.label_changed` | "Labels updated on \"Fix login bug\"" | `low` | In-app only |

---

## 12. River Integration Details

### 12.1 Setup in `cmd/api/main.go`

```go
import (
    "github.com/riverqueue/river"
    "github.com/riverqueue/river/riverdriver/riverdatabasesql"
)

// After GORM DB setup
sqlDB, _ := db.DB() // Get *sql.DB from GORM

riverDriver := riverdatabasesql.New(sqlDB)
riverClient, err := river.NewClient(riverDriver, &river.Config{
    Queues: map[string]river.QueueConfig{
        "notifications_critical": {MaxWorkers: 10},
        "notifications_default":  {MaxWorkers: 5},
        "notifications_digest":   {MaxWorkers: 2},
    },
    PeriodicJobs: []*river.PeriodicJob{
        // Snooze checker: every minute
        river.NewPeriodicJob(
            river.PeriodicInterval(1 * time.Minute),
            func() (river.JobArgs, *river.InsertOpts) {
                return SnoozeCheckArgs{}, nil
            },
            nil,
        ),
        // Digest builder: daily at midnight UTC (per-user times handled in worker)
        river.NewPeriodicJob(
            river.PeriodicInterval(1 * time.Hour),
            func() (river.JobArgs, *river.InsertOpts) {
                return DigestCheckArgs{}, nil
            },
            nil,
        ),
    },
    Workers: workers,
})

// Register workers
workers := river.NewWorkers()
river.AddWorker(workers, &InAppNotificationWorker{wsHub: wsHub, notifRepo: notifRepo})
river.AddWorker(workers, &EmailNotificationWorker{emailClient: emailClient, notifRepo: notifRepo})
river.AddWorker(workers, &DigestWorker{emailClient: emailClient, notifRepo: notifRepo, prefRepo: prefRepo})
river.AddWorker(workers, &SnoozeCheckWorker{notifRepo: notifRepo, wsHub: wsHub})

// Start River client (runs workers in background goroutines)
riverClient.Start(ctx)
```

### 12.2 Transactional Enqueue Pattern

```go
// In StoryService.AssignStory()
func (s *StoryService) AssignStory(ctx context.Context, storyID, assigneeID string) error {
    return s.db.Transaction(func(tx *gorm.DB) error {
        // 1. Business logic
        if err := tx.Model(&Story{}).Where("id = ?", storyID).
            Update("assignee_id", assigneeID).Error; err != nil {
            return err
        }

        // 2. Emit notification (within same transaction)
        return s.notificationService.Emit(ctx, tx, NotificationEvent{
            WorkspaceID: wsID,
            ActorID:     currentUserID,
            EventType:   "story.assigned",
            EntityType:  "story",
            EntityID:    storyID,
            Title:       fmt.Sprintf("%s assigned you \"%s\"", actorName, storyTitle),
            Metadata:    map[string]interface{}{"story_identifier": "ENG-142"},
            ExplicitRecipients: []string{assigneeID},
        })
    })
}
```

---

## 13. Migration Plan

### Migration `023_notifications.sql`

```sql
-- Notification tables
-- See Section 6.1 for full SQL

-- River manages its own tables (river_job, river_leader, river_queue)
-- via river.Migrate() called in application startup
```

### GORM AutoMigrate

Add to `cmd/api/main.go`:
```go
db.AutoMigrate(
    &model.Notification{},
    &model.NotificationEvent{},
    &model.NotificationDelivery{},
    &model.NotificationPreference{},
    &model.EntityFollower{},
)
```

---

## 14. Implementation Phases

### Phase 1: Foundation (Week 1-2)
- [ ] Database tables + GORM models
- [ ] River integration (go.mod, client setup, migrations)
- [ ] Follower system (model, repository, service, auto-follow hooks)
- [ ] Notification service (Emit, recipient resolution, preference checking)
- [ ] InAppNotificationWorker (write to DB + WebSocket push)
- [ ] Basic notification API (list, mark read, unread count)
- [ ] Frontend: NotificationCenter popover + NotificationBadge
- [ ] Frontend: useNotifications + useUnreadCount hooks
- [ ] Frontend: useRealtimeSync extension for notification events

### Phase 2: Preferences & UX (Week 3)
- [ ] Notification preferences API (GET/PUT)
- [ ] Frontend: NotificationPreferences settings page
- [ ] Tab filters (All / Mentions / Assigned)
- [ ] Notification row actions (read/unread, archive)
- [ ] Snooze functionality (API + SnoozeCheckWorker + UI)
- [ ] Follow/Unfollow button on story/epic/objective detail pages
- [ ] Entity-centric consolidation (upsert on same entity)

### Phase 3: Email (Week 4)
- [ ] EmailNotificationWorker (immediate for urgent/mentions)
- [ ] Smart email deduplication (skip if already read)
- [ ] DigestWorker (daily digest email)
- [ ] Email templates (individual notification + digest)
- [ ] Digest frequency configuration (daily/weekly)

### Phase 4: Polish (Week 5)
- [ ] Notification row hover actions + context menu
- [ ] Mark all as read / Archive all read
- [ ] Empty state for notification center
- [ ] Keyboard shortcuts (if applicable)
- [ ] Notification count in page title / favicon badge
- [ ] Performance optimization (query indexes, pagination)
- [ ] Integration tests

---

## 15. Observability & Monitoring

### Metrics to Track
- Notifications created per minute (by event_type)
- River job queue depth (by queue name)
- Worker processing latency (p50, p95, p99)
- Email delivery success/failure rate
- Digest email open rate (via Postmark)
- WebSocket notification delivery latency

### River UI
River provides a self-hosted web UI for monitoring job queues. Can be added as an admin-only route at `/admin/river` (future).

---

## 16. Future Considerations

### Mattermost Integration (Phase 2)

The `notification_deliveries` table already supports Mattermost as a channel. Implementation requires:

- **Workspace integration state**: Mattermost server URL, bot token, stored in workspace settings or a new `workspace_integrations` table.
- **User identity link**: Optional `user_id → mattermost_user_id` mapping. If a user has no Mattermost link, fall back to in-app/email only.
- **Team/entity → channel mapping**: Link a Helpin team or project to a Mattermost channel for automated posts.
- **MattermostNotificationWorker**: River worker that resolves DM vs channel post, applies preferences, renders Mattermost-specific message format (Markdown), posts via Mattermost API, and records delivery with `external_message_id`.
- **v1 scope (narrow)**: `story.mentioned` (direct), `story.assigned`, `story.blocked`, `sprint.ending_soon`. Team mentions should be either DM fan-out OR mapped channel posts, not both.
- **Preference extension**: Add `"mattermost"` key to `channel_preferences` JSONB (e.g., `{ "story.assigned": { "in_app": true, "email": true, "mattermost": true } }`).
- See also: [Mattermost integration doc](../mattermost-integration.md).

### Slack Integration (Phase 3 - Separate PRD)
- Personal DMs for @-mentions and assignments
- Team-to-channel mapping for squad awareness
- Bi-directional comment sync (reply in Slack → comment in Helpin)

### Mobile Push Notifications
- Firebase Cloud Messaging (FCM) for Android
- Apple Push Notification Service (APNS) for iOS
- New PushNotificationWorker in River

### Browser Push Notifications
- Web Push API with VAPID keys
- Service worker for background delivery

### Triage / Team Inbox
- Shared notification queue for incoming external requests
- Rotation assignment for triage duty

### Notification Search
- Full-text search across notification titles and body
- Filter by date range, entity type, event type
- PostgreSQL `tsvector` index on notifications table

---

## Appendix A: Research References

- [Linear Notification System Research](../notifications/linear-notification-system.md)
- [Shortcut Notification System Research](../notifications/research-shortcut-notifications.md)
- [Notification Queue Comparison (River vs Watermill vs Asynq vs Temporal)](../notifications/notification-queue-comparison.md)

## Appendix B: Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Job queue | River | PostgreSQL-native, transactional enqueue, all required primitives built-in |
| Notification model | Entity-centric | Prevents notification flood; matches Linear's proven UX |
| Delivery tracking | Per-event `notification_deliveries` table | Row-level delivery flags on aggregated entity rows cause missed/duplicated emails when new events arrive |
| Render data strategy | Relational IDs + JSONB snapshots on `notifications` and `notification_events` | Eliminates inbox-time joins while keeping FK-backed identifiers and point-in-time display context |
| Mention taxonomy | `{entity}.mentioned` + `metadata.mention_type` | Avoids dual event types (`mention.direct` vs `story.mentioned`) for the same action |
| Subscription model | Follower-based, migrated from `pm_story_followers` | Per-entity opt-in/opt-out; proven by both Linear and Shortcut; avoids dual follower tables |
| Email strategy | Digest + immediate for urgent/high | Balances awareness with inbox noise; proven by Shortcut |
| Digest timezone | Explicit IANA timezone on `notification_preferences` | Bare `TIME` without timezone makes digest scheduling ambiguous |
| State transitions | Explicit rules per (current_status, new_priority) | Prevents undefined behavior when new events hit read/archived/snoozed notifications |
| Preference storage | JSONB channel_preferences | Flexible per-event-type toggles without schema changes per new event |
| Real-time delivery | Existing WebSocket hub | Already built, per-workspace broadcasting, zero new infra |
| Workspace kill switch | Existing `notifications_enabled` setting | Workspace-level master switch; per-user DND operates independently |
