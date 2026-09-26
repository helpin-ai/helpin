ALTER TABLE crm_deals ADD COLUMN IF NOT EXISTS revenue_type TEXT NOT NULL DEFAULT 'one_time' CHECK (revenue_type IN ('one_time', 'monthly', 'annual'));
