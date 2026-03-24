ALTER TABLE pm_stories
    ADD COLUMN IF NOT EXISTS plan_document_id UUID REFERENCES docs_documents(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_pm_stories_plan_document_id ON pm_stories (plan_document_id);
