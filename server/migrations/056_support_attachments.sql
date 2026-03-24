-- Support file attachments (Crisp-style presigned URL upload pattern).
CREATE TABLE IF NOT EXISTS support_attachments (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID NOT NULL,
    conversation_id  UUID NOT NULL,
    message_id       UUID,
    file_name        TEXT NOT NULL,
    file_size        BIGINT NOT NULL,
    content_type     TEXT NOT NULL,
    storage_key      TEXT NOT NULL DEFAULT '',
    public_url       TEXT NOT NULL DEFAULT '',
    is_uploaded      BOOLEAN NOT NULL DEFAULT false,
    uploaded_by_type TEXT NOT NULL DEFAULT 'user',
    uploaded_by_id   UUID,
    session_id       UUID,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_support_attachments_workspace ON support_attachments(workspace_id);
CREATE INDEX IF NOT EXISTS idx_support_attachments_conversation ON support_attachments(conversation_id);
CREATE INDEX IF NOT EXISTS idx_support_attachments_message ON support_attachments(message_id);
