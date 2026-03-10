-- 031: Split notification preferences into account-level (user_notification_settings)
-- and workspace-level (notification_preferences retains channel_preferences + mute).

CREATE TABLE IF NOT EXISTS user_notification_settings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    email_enabled   BOOLEAN NOT NULL DEFAULT true,
    email_digest_frequency VARCHAR(20) NOT NULL DEFAULT 'daily',
    email_digest_time      VARCHAR(10) NOT NULL DEFAULT '09:00',
    email_digest_day       INTEGER NOT NULL DEFAULT 1,
    do_not_disturb  BOOLEAN NOT NULL DEFAULT false,
    dnd_until       TIMESTAMPTZ,
    badge_mode      VARCHAR(20) NOT NULL DEFAULT 'all',
    timezone        VARCHAR(50) NOT NULL DEFAULT 'UTC',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Migrate existing data: for each user, pick the most recently updated
-- workspace-level notification_preferences row and copy account-level fields.
INSERT INTO user_notification_settings (
    user_id, email_enabled, email_digest_frequency, email_digest_time,
    email_digest_day, do_not_disturb, dnd_until, badge_mode, timezone
)
SELECT DISTINCT ON (user_id)
    user_id, email_enabled, email_digest_frequency, email_digest_time,
    email_digest_day, do_not_disturb, dnd_until, badge_mode, timezone
FROM notification_preferences
WHERE team_id IS NULL
ORDER BY user_id, updated_at DESC
ON CONFLICT (user_id) DO NOTHING;

-- Add mute_workspace to notification_preferences for workspace-level muting.
ALTER TABLE notification_preferences
    ADD COLUMN IF NOT EXISTS mute_workspace BOOLEAN NOT NULL DEFAULT false;
