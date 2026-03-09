-- CRM Module: Core Foundation Tables
-- Idempotent migration

-- Contacts
CREATE TABLE IF NOT EXISTS crm_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    display_id VARCHAR NOT NULL,
    first_name VARCHAR NOT NULL,
    last_name VARCHAR,
    email VARCHAR,
    phone VARCHAR,
    job_title VARCHAR,
    lifecycle_stage VARCHAR NOT NULL DEFAULT 'subscriber',
    lead_status VARCHAR NOT NULL DEFAULT 'new',
    owner_member_id UUID,
    avatar_url VARCHAR,
    source VARCHAR,
    custom_properties JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_contacts_workspace_id ON crm_contacts(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_contacts_email ON crm_contacts(email);
CREATE INDEX IF NOT EXISTS idx_crm_contacts_owner_member_id ON crm_contacts(owner_member_id);
CREATE INDEX IF NOT EXISTS idx_crm_contacts_custom_props ON crm_contacts USING GIN(custom_properties);

-- Companies
CREATE TABLE IF NOT EXISTS crm_companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    display_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    domain VARCHAR,
    industry VARCHAR,
    employee_count INTEGER,
    annual_revenue DOUBLE PRECISION,
    description TEXT,
    logo_url VARCHAR,
    owner_member_id UUID,
    custom_properties JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_companies_workspace_id ON crm_companies(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_companies_domain ON crm_companies(domain);
CREATE INDEX IF NOT EXISTS idx_crm_companies_owner_member_id ON crm_companies(owner_member_id);
CREATE INDEX IF NOT EXISTS idx_crm_companies_custom_props ON crm_companies USING GIN(custom_properties);

-- Pipelines
CREATE TABLE IF NOT EXISTS crm_pipelines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    name VARCHAR NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_pipelines_workspace_id ON crm_pipelines(workspace_id);

-- Pipeline Stages
CREATE TABLE IF NOT EXISTS crm_pipeline_stages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id UUID NOT NULL REFERENCES crm_pipelines(id) ON DELETE CASCADE,
    name VARCHAR NOT NULL,
    stage_type VARCHAR NOT NULL DEFAULT 'open',
    position INTEGER NOT NULL DEFAULT 0,
    probability INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_pipeline_stages_pipeline_id ON crm_pipeline_stages(pipeline_id);

-- Deals
CREATE TABLE IF NOT EXISTS crm_deals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    display_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    pipeline_id UUID NOT NULL REFERENCES crm_pipelines(id),
    stage_id UUID NOT NULL REFERENCES crm_pipeline_stages(id),
    amount DOUBLE PRECISION,
    currency VARCHAR NOT NULL DEFAULT 'USD',
    close_date DATE,
    owner_member_id UUID,
    probability INTEGER,
    custom_properties JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_deals_workspace_id ON crm_deals(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_deals_pipeline_id ON crm_deals(pipeline_id);
CREATE INDEX IF NOT EXISTS idx_crm_deals_stage_id ON crm_deals(stage_id);
CREATE INDEX IF NOT EXISTS idx_crm_deals_owner_member_id ON crm_deals(owner_member_id);
CREATE INDEX IF NOT EXISTS idx_crm_deals_custom_props ON crm_deals USING GIN(custom_properties);

-- Associations
CREATE TABLE IF NOT EXISTS crm_associations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    from_object_type VARCHAR NOT NULL,
    from_object_id UUID NOT NULL,
    to_object_type VARCHAR NOT NULL,
    to_object_id UUID NOT NULL,
    association_label VARCHAR,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_associations_workspace_id ON crm_associations(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_associations_from ON crm_associations(from_object_type, from_object_id);
CREATE INDEX IF NOT EXISTS idx_crm_associations_to ON crm_associations(to_object_type, to_object_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_associations_unique ON crm_associations(workspace_id, from_object_type, from_object_id, to_object_type, to_object_id);

-- Activities
CREATE TABLE IF NOT EXISTS crm_activities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    activity_type VARCHAR NOT NULL DEFAULT 'note',
    contact_id UUID,
    company_id UUID,
    deal_id UUID,
    owner_member_id UUID,
    subject VARCHAR,
    body TEXT,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_activities_workspace_id ON crm_activities(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_activities_contact_id ON crm_activities(contact_id);
CREATE INDEX IF NOT EXISTS idx_crm_activities_company_id ON crm_activities(company_id);
CREATE INDEX IF NOT EXISTS idx_crm_activities_deal_id ON crm_activities(deal_id);
CREATE INDEX IF NOT EXISTS idx_crm_activities_owner_member_id ON crm_activities(owner_member_id);
