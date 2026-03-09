-- CRM Phase 3: Email & Calendar Sync

-- Email accounts (connected Gmail/Microsoft accounts)
CREATE TABLE IF NOT EXISTS crm_email_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    member_id UUID NOT NULL,
    provider VARCHAR(20) NOT NULL DEFAULT 'gmail',
    email_address VARCHAR(255) NOT NULL,
    access_token_encrypted TEXT,
    refresh_token_encrypted TEXT,
    sync_state JSONB DEFAULT '{}',
    last_synced_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_email_accounts_workspace ON crm_email_accounts(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_email_accounts_member ON crm_email_accounts(member_id);

-- Email threads
CREATE TABLE IF NOT EXISTS crm_email_threads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email_account_id UUID NOT NULL REFERENCES crm_email_accounts(id) ON DELETE CASCADE,
    thread_external_id VARCHAR(255) NOT NULL,
    subject TEXT NOT NULL,
    last_message_at TIMESTAMPTZ NOT NULL,
    message_count INTEGER NOT NULL DEFAULT 0,
    contact_ids JSONB DEFAULT '[]',
    deal_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_email_threads_workspace ON crm_email_threads(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_email_threads_account ON crm_email_threads(email_account_id);
CREATE INDEX IF NOT EXISTS idx_crm_email_threads_deal ON crm_email_threads(deal_id);

-- Email messages
CREATE TABLE IF NOT EXISTS crm_email_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email_account_id UUID NOT NULL REFERENCES crm_email_accounts(id) ON DELETE CASCADE,
    thread_id UUID REFERENCES crm_email_threads(id) ON DELETE SET NULL,
    message_external_id VARCHAR(255),
    from_address VARCHAR(255) NOT NULL,
    from_name VARCHAR(255),
    to_addresses JSONB DEFAULT '[]',
    cc_addresses JSONB DEFAULT '[]',
    subject TEXT,
    body_text TEXT,
    body_html TEXT,
    direction VARCHAR(20) NOT NULL DEFAULT 'inbound',
    sent_at TIMESTAMPTZ NOT NULL,
    contact_id UUID,
    deal_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_email_messages_workspace ON crm_email_messages(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_email_messages_account ON crm_email_messages(email_account_id);
CREATE INDEX IF NOT EXISTS idx_crm_email_messages_thread ON crm_email_messages(thread_id);
CREATE INDEX IF NOT EXISTS idx_crm_email_messages_contact ON crm_email_messages(contact_id);
CREATE INDEX IF NOT EXISTS idx_crm_email_messages_deal ON crm_email_messages(deal_id);

-- Calendar events
CREATE TABLE IF NOT EXISTS crm_calendar_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email_account_id UUID NOT NULL REFERENCES crm_email_accounts(id) ON DELETE CASCADE,
    external_event_id VARCHAR(255),
    title TEXT NOT NULL,
    description TEXT,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    location TEXT,
    attendees JSONB DEFAULT '[]',
    contact_ids JSONB DEFAULT '[]',
    deal_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_calendar_events_workspace ON crm_calendar_events(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_calendar_events_account ON crm_calendar_events(email_account_id);
CREATE INDEX IF NOT EXISTS idx_crm_calendar_events_deal ON crm_calendar_events(deal_id);
CREATE INDEX IF NOT EXISTS idx_crm_calendar_events_start ON crm_calendar_events(start_time);
