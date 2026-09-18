-- Explicit conversation ownership and a durable fence for earlier AI work.
ALTER TABLE support_conversations
 ADD COLUMN IF NOT EXISTS ai_control_version bigint NOT NULL DEFAULT 0,
 ADD COLUMN IF NOT EXISTS ai_resumed_at timestamptz,
 ADD COLUMN IF NOT EXISTS ai_paused_at timestamptz,
 ADD COLUMN IF NOT EXISTS ai_paused_by_user_id uuid;
