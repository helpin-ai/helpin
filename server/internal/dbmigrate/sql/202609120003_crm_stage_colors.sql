-- Existing stage order, names and deal references are preserved.
ALTER TABLE crm_pipeline_stages ADD COLUMN IF NOT EXISTS color TEXT;
UPDATE crm_pipeline_stages SET color = CASE
 WHEN stage_type = 'won' THEN '#45a557'
 WHEN stage_type = 'lost' THEN '#e2564a'
 WHEN GREATEST(position, 0) % 3 = 1 THEN '#4e8fea'
 WHEN GREATEST(position, 0) % 3 = 2 THEN '#c7a53d'
 ELSE '#788596' END WHERE color IS NULL;
ALTER TABLE crm_pipeline_stages ALTER COLUMN color SET DEFAULT '#788596';
ALTER TABLE crm_pipeline_stages ALTER COLUMN color SET NOT NULL;
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'crm_pipeline_stages'::regclass AND conname = 'crm_stage_color_hex') THEN
  ALTER TABLE crm_pipeline_stages ADD CONSTRAINT crm_stage_color_hex CHECK (color ~ '^#[0-9a-fA-F]{6}$');
 END IF;
END $$;
