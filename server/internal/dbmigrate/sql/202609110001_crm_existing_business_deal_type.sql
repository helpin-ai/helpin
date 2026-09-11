-- Broader customer deal classification. Preserve all existing specific values;
-- relationship defaults do not rewrite historical expansion or renewal intent.
ALTER TABLE crm_pipelines DROP CONSTRAINT IF EXISTS crm_pipelines_default_commercial_motion_check;
ALTER TABLE crm_pipelines ADD CONSTRAINT crm_pipelines_default_commercial_motion_check
    CHECK (default_commercial_motion IN ('new_business', 'existing_business', 'expansion', 'renewal'));

ALTER TABLE crm_deals DROP CONSTRAINT IF EXISTS crm_deals_commercial_motion_check;
ALTER TABLE crm_deals ADD CONSTRAINT crm_deals_commercial_motion_check
    CHECK (commercial_motion IS NULL OR commercial_motion IN ('new_business', 'existing_business', 'expansion', 'renewal'));
