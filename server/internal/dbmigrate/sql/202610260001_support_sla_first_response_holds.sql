-- Preserve first-response time remaining across AI holds and worker restarts.
ALTER TABLE support_conversation_slas
    ADD COLUMN IF NOT EXISTS first_response_paused_seconds bigint NOT NULL DEFAULT 0;
