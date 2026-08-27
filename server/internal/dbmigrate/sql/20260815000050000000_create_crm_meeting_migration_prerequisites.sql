-- Meeting backfills and indexes run before application startup, so the tables
-- they read cannot depend on GORM AutoMigrate having run first.
CREATE TABLE IF NOT EXISTS crm_meetings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    calendar_event_id UUID,
    activity_id UUID,
    owner_member_id UUID,
    title TEXT NOT NULL,
    meeting_url TEXT NOT NULL,
    platform TEXT NOT NULL,
    native_meeting_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'scheduled',
    summary_status TEXT NOT NULL DEFAULT 'pending',
    visibility TEXT NOT NULL DEFAULT 'workspace',
    record_audio BOOLEAN NOT NULL DEFAULT FALSE,
    scheduled_start_at TIMESTAMPTZ,
    scheduled_end_at TIMESTAMPTZ,
    actual_start_at TIMESTAMPTZ,
    actual_end_at TIMESTAMPTZ,
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    participants JSONB DEFAULT '[]'::jsonb,
    failure_code TEXT,
    failure_message TEXT,
    recording_object_key TEXT,
    created_by UUID,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_meeting_intelligence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    meeting_id UUID NOT NULL,
    generation_version TEXT NOT NULL,
    transcript_checksum TEXT NOT NULL,
    summary_markdown TEXT NOT NULL DEFAULT '',
    key_points JSONB DEFAULT '[]'::jsonb,
    decisions JSONB DEFAULT '[]'::jsonb,
    objections JSONB DEFAULT '[]'::jsonb,
    risks JSONB DEFAULT '[]'::jsonb,
    next_steps JSONB DEFAULT '[]'::jsonb,
    follow_up_draft JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
