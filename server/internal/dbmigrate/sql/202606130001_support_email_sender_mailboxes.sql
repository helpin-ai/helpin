CREATE TABLE IF NOT EXISTS support_email_sender_mailboxes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    sender_id UUID NOT NULL,
    mailbox_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO support_email_sender_mailboxes (workspace_id, sender_id, mailbox_id, created_at, updated_at)
SELECT workspace_id, id, mailbox_id, now(), now()
FROM support_email_senders
WHERE default_scope = 'mailbox'
  AND mailbox_id IS NOT NULL
ON CONFLICT DO NOTHING;

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_email_sender_mailbox_sender_mailbox
ON support_email_sender_mailboxes(sender_id, mailbox_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_email_sender_mailbox_mailbox
ON support_email_sender_mailboxes(mailbox_id);

CREATE INDEX IF NOT EXISTS idx_support_email_sender_mailbox_workspace
ON support_email_sender_mailboxes(workspace_id);

CREATE INDEX IF NOT EXISTS idx_support_email_sender_mailbox_sender
ON support_email_sender_mailboxes(sender_id);
