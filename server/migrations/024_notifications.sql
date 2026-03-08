-- Notification system tables
-- Reference documentation for GORM AutoMigrate

-- Core notifications table: entity-centric (one row per recipient per entity)
CREATE TABLE IF NOT EXISTS notifications (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    recipient_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id              UUID REFERENCES users(id) ON DELETE SET NULL,
    entity_type           VARCHAR(50) NOT NULL,
    entity_id             UUID NOT NULL,
    event_type            VARCHAR(100) NOT NULL,
    title                 TEXT NOT NULL,
    body                  TEXT,
    metadata              JSONB DEFAULT '{}',
    latest_event_category VARCHAR(50) NOT NULL,
    actor_snapshot         JSONB NOT NULL DEFAULT '{}'::jsonb,
    entity_snapshot        JSONB NOT NULL DEFAULT '{}'::jsonb,
    parent_entity_snapshot JSONB,
    event_count           INTEGER DEFAULT 1,
    last_event_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status                VARCHAR(20) NOT NULL DEFAULT 'unread',
    snoozed_until         TIMESTAMPTZ,
    read_at               TIMESTAMPTZ,
    archived_at           TIMESTAMPTZ,
    priority              VARCHAR(10) NOT NULL DEFAULT 'normal',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_notification_recipient_entity
        UNIQUE (recipient_id, entity_type, entity_id, workspace_id)
);

CREATE INDEX IF NOT EXISTS idx_notifications_recipient_status ON notifications(recipient_id, status, last_event_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_recipient_workspace ON notifications(recipient_id, workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_notifications_snoozed ON notifications(snoozed_until) WHERE snoozed_until IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_notifications_entity ON notifications(entity_type, entity_id);

-- Individual events that contribute to a notification
CREATE TABLE IF NOT EXISTS notification_events (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id  UUID NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    actor_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    event_type       VARCHAR(100) NOT NULL,
    title            TEXT NOT NULL,
    metadata         JSONB DEFAULT '{}',
    category         VARCHAR(50) NOT NULL,
    actor_snapshot   JSONB NOT NULL DEFAULT '{}'::jsonb,
    priority         VARCHAR(10) NOT NULL DEFAULT 'normal',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_events_notification ON notification_events(notification_id, created_at DESC);

-- Per-event, per-channel delivery tracking
CREATE TABLE IF NOT EXISTS notification_deliveries (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_event_id UUID NOT NULL REFERENCES notification_events(id) ON DELETE CASCADE,
    channel               VARCHAR(30) NOT NULL,
    status                VARCHAR(20) NOT NULL DEFAULT 'pending',
    delivered_at          TIMESTAMPTZ,
    error                 TEXT,
    external_message_id   VARCHAR(255),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_event ON notification_deliveries(notification_event_id);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_pending ON notification_deliveries(channel, status) WHERE status = 'pending';

-- Per-user, per-workspace notification preferences
CREATE TABLE IF NOT EXISTS notification_preferences (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id               UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id          UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    do_not_disturb        BOOLEAN DEFAULT FALSE,
    dnd_until             TIMESTAMPTZ,
    email_enabled         BOOLEAN DEFAULT TRUE,
    email_digest_frequency VARCHAR(20) DEFAULT 'daily',
    email_digest_time     VARCHAR(10) DEFAULT '09:00',
    email_digest_day      INTEGER DEFAULT 1,
    timezone              VARCHAR(50) DEFAULT 'UTC',
    channel_preferences   JSONB DEFAULT '{}',
    badge_mode            VARCHAR(20) DEFAULT 'all',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_notification_pref_user_workspace
        UNIQUE (user_id, workspace_id)
);

-- Entity followers
CREATE TABLE IF NOT EXISTS entity_followers (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entity_type  VARCHAR(50) NOT NULL,
    entity_id    UUID NOT NULL,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    reason       VARCHAR(30) NOT NULL DEFAULT 'manual',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_entity_follower
        UNIQUE (user_id, entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_entity_followers_entity ON entity_followers(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_entity_followers_user ON entity_followers(user_id, workspace_id);

-- Backfill entity_followers from existing pm_story_followers
INSERT INTO entity_followers (user_id, entity_type, entity_id, workspace_id, reason, created_at)
SELECT sf.user_id, 'story', sf.story_id, s.workspace_id, 'manual', sf.created_at
FROM pm_story_followers sf
JOIN pm_stories s ON s.id = sf.story_id
ON CONFLICT (user_id, entity_type, entity_id) DO NOTHING;
