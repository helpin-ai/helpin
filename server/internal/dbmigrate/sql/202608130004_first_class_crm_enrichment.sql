ALTER TABLE crm_companies
    ADD COLUMN IF NOT EXISTS linkedin_url TEXT,
    ADD COLUMN IF NOT EXISTS headquarters TEXT;

UPDATE crm_contacts
SET linkedin_url = NULLIF(custom_properties->'agent_enrichment'->'linkedin_url'->>'value', '')
WHERE (linkedin_url IS NULL OR BTRIM(linkedin_url) = '')
  AND NULLIF(custom_properties->'agent_enrichment'->'linkedin_url'->>'value', '') IS NOT NULL;

UPDATE crm_contacts
SET primary_location = NULLIF(custom_properties->'agent_enrichment'->'location'->>'value', '')
WHERE (primary_location IS NULL OR BTRIM(primary_location) = '')
  AND NULLIF(custom_properties->'agent_enrichment'->'location'->>'value', '') IS NOT NULL;

UPDATE crm_companies
SET linkedin_url = NULLIF(custom_properties->'agent_enrichment'->'linkedin_url'->>'value', '')
WHERE (linkedin_url IS NULL OR BTRIM(linkedin_url) = '')
  AND NULLIF(custom_properties->'agent_enrichment'->'linkedin_url'->>'value', '') IS NOT NULL;

UPDATE crm_companies
SET headquarters = NULLIF(custom_properties->'agent_enrichment'->'headquarters'->>'value', '')
WHERE (headquarters IS NULL OR BTRIM(headquarters) = '')
  AND NULLIF(custom_properties->'agent_enrichment'->'headquarters'->>'value', '') IS NOT NULL;
