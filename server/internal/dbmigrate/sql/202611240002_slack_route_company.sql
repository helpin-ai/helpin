ALTER TABLE slack_channel_routes ADD COLUMN IF NOT EXISTS crm_company_id uuid REFERENCES crm_companies(id) ON DELETE SET NULL;
