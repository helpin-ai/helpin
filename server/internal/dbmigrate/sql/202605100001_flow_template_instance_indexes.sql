ALTER TABLE agents
    ADD COLUMN IF NOT EXISTS template_key text,
    ADD COLUMN IF NOT EXISTS template_instance_id uuid,
    ADD COLUMN IF NOT EXISTS template_version integer;

ALTER TABLE automation_rules
    ADD COLUMN IF NOT EXISTS template_key text,
    ADD COLUMN IF NOT EXISTS template_instance_id uuid,
    ADD COLUMN IF NOT EXISTS template_version integer;

CREATE INDEX IF NOT EXISTS idx_agents_template_instance
    ON agents(template_instance_id)
    WHERE template_instance_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_automation_rules_template_instance
    ON automation_rules(template_instance_id)
    WHERE template_instance_id IS NOT NULL;
