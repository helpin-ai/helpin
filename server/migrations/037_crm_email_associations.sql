-- CRM email message participant associations

CREATE TABLE IF NOT EXISTS crm_email_message_contacts (
    message_id UUID NOT NULL REFERENCES crm_email_messages(id) ON DELETE CASCADE,
    contact_id UUID NOT NULL REFERENCES crm_contacts(id) ON DELETE CASCADE,
    participant_role VARCHAR(20) NOT NULL,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, contact_id, participant_role)
);

CREATE INDEX IF NOT EXISTS idx_crm_email_message_contacts_workspace_contact
    ON crm_email_message_contacts(workspace_id, contact_id);

CREATE INDEX IF NOT EXISTS idx_crm_email_message_contacts_message
    ON crm_email_message_contacts(message_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_contacts_workspace_email_lower_unique
    ON crm_contacts(workspace_id, LOWER(email))
    WHERE email IS NOT NULL;
