-- CRM Phase 4: AI-Driven Intelligence

-- Enrichment results
CREATE TABLE IF NOT EXISTS crm_enrichment_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    object_type VARCHAR(20) NOT NULL,
    object_id UUID NOT NULL,
    source VARCHAR(20) NOT NULL DEFAULT 'manual',
    data JSONB DEFAULT '{}',
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_enrichment_workspace ON crm_enrichment_results(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_enrichment_object ON crm_enrichment_results(object_id);

-- Buyer signals
CREATE TABLE IF NOT EXISTS crm_buyer_signals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    contact_id UUID,
    deal_id UUID,
    signal_type VARCHAR(50) NOT NULL,
    source_type VARCHAR(20) NOT NULL DEFAULT 'manual',
    source_id UUID,
    summary TEXT NOT NULL,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    detected_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_signals_workspace ON crm_buyer_signals(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_signals_contact ON crm_buyer_signals(contact_id);
CREATE INDEX IF NOT EXISTS idx_crm_signals_deal ON crm_buyer_signals(deal_id);

-- Deal health scores
CREATE TABLE IF NOT EXISTS crm_deal_health_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    deal_id UUID NOT NULL,
    score INTEGER NOT NULL DEFAULT 0 CHECK (score >= 0 AND score <= 100),
    factors JSONB DEFAULT '{}',
    calculated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_health_workspace ON crm_deal_health_scores(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_health_deal ON crm_deal_health_scores(deal_id);

-- AI Suggestions
CREATE TABLE IF NOT EXISTS crm_suggestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID,
    suggestion_type VARCHAR(50) NOT NULL,
    object_type VARCHAR(20),
    object_id UUID,
    title TEXT NOT NULL,
    description TEXT,
    context JSONB DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_suggestions_workspace ON crm_suggestions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_crm_suggestions_user ON crm_suggestions(user_id);
CREATE INDEX IF NOT EXISTS idx_crm_suggestions_object ON crm_suggestions(object_id);
