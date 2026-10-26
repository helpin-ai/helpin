-- Customer satisfaction ratings, one per conversation. Ratings are submitted
-- from the widget or the customer portal after a conversation is resolved and
-- are updated in place when the customer changes their mind.

CREATE TABLE IF NOT EXISTS support_csat_responses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    conversation_id uuid NOT NULL REFERENCES support_conversations (id) ON DELETE CASCADE,
    rating integer NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment text,
    channel text NOT NULL CHECK (channel IN ('widget', 'portal')),
    resolved_by text NOT NULL CHECK (resolved_by IN ('ai', 'human')),
    assigned_user_id uuid,
    submitted_at timestamptz NOT NULL,
    created_at timestamptz,
    updated_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_csat_responses_conversation_id
    ON support_csat_responses (conversation_id);

CREATE INDEX IF NOT EXISTS idx_support_csat_responses_workspace_submitted
    ON support_csat_responses (workspace_id, submitted_at);
