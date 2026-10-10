ALTER TABLE support_ai_follow_ups
 ADD COLUMN IF NOT EXISTS cancelled_by_user_id uuid;
