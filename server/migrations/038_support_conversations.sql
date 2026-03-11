-- Migration 038: Support Conversations and Canned Responses
-- Phase 2: Data Modeling

-- Rename support_tickets to support_conversations (via new table + alias)
CREATE TABLE IF NOT EXISTS support_conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    display_id SERIAL NOT NULL,
    subject TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    priority TEXT NOT NULL DEFAULT 'medium',
    channel TEXT NOT NULL DEFAULT 'widget',
    customer_name TEXT,
    customer_email TEXT,
    opened_by_user_id UUID,
    assigned_agent_id UUID,
    linked_story_id UUID,
    source TEXT NOT NULL DEFAULT 'internal',
    crm_contact_id UUID,
    resolved_at TIMESTAMP,
    closed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Add index on workspace_id for conversations
CREATE INDEX IF NOT EXISTS idx_support_conversations_workspace ON support_conversations(workspace_id);
CREATE INDEX IF NOT EXISTS idx_support_conversations_display ON support_conversations(display_id);
CREATE INDEX IF NOT EXISTS idx_support_conversations_crm_contact ON support_conversations(crm_contact_id);

-- Add new columns to support_messages
ALTER TABLE support_messages ADD COLUMN IF NOT EXISTS message_type TEXT NOT NULL DEFAULT 'reply';
ALTER TABLE support_messages ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}';

-- Create support_canned_responses table
CREATE TABLE IF NOT EXISTS support_canned_responses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    short_code TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    created_by_id UUID,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Add index on workspace_id for canned responses
CREATE INDEX IF NOT EXISTS idx_support_canned_responses_workspace ON support_canned_responses(workspace_id);
CREATE INDEX IF NOT EXISTS idx_support_canned_responses_short_code ON support_canned_responses(short_code);

-- Add conversation_id column to support_messages (new reference)
ALTER TABLE support_messages ADD COLUMN IF NOT EXISTS conversation_id UUID;
CREATE INDEX IF NOT EXISTS idx_support_messages_conversation ON support_messages(conversation_id);

-- Add conversation_id column to support_widget_sessions
ALTER TABLE support_widget_sessions ADD COLUMN IF NOT EXISTS conversation_id UUID;
CREATE INDEX IF NOT EXISTS idx_support_widget_sessions_conversation ON support_widget_sessions(conversation_id);

-- Add resolved_at and closed_at columns to support_tickets (for backward compat)
ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMP;
ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS closed_at TIMESTAMP;
ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS channel TEXT NOT NULL DEFAULT 'widget';

-- Create type for message_type enum if not exists
DO $$ BEGIN
    CREATE TYPE message_type AS ENUM ('reply', 'csat_survey', 'system');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Backfill conversation_id from ticket_id for existing messages
UPDATE support_messages SET conversation_id = ticket_id WHERE conversation_id IS NULL AND ticket_id IS NOT NULL;

-- Backfill conversation_id from ticket_id for existing sessions
UPDATE support_widget_sessions SET conversation_id = ticket_id WHERE conversation_id IS NULL AND ticket_id IS NOT NULL;
