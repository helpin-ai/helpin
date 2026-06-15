ALTER TABLE IF EXISTS workspace_agent_preset_versions
  ADD COLUMN IF NOT EXISTS available_skills JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE workspace_agent_preset_versions
SET available_skills = '[
  "docs_architecture_review",
  "public_help_doc_writing",
  "api_reference_doc_writing",
  "internal_docs_maintenance",
  "public_help_docs_maintenance",
  "api_docs_maintenance",
  "post_release_docs_update",
  "support_gap_docs_update"
]'::jsonb
WHERE family_key = 'documentation_agent'
  AND available_skills = '[]'::jsonb;

UPDATE workspace_agent_preset_versions
SET available_skills = '[
  "marketing_context_setup",
  "marketing_plan",
  "customer_research_synthesis",
  "marketing_copywriting",
  "conversion_optimization",
  "lifecycle_messaging",
  "launch_marketing",
  "seo_content_strategy",
  "competitive_positioning",
  "lead_generation_strategy",
  "outbound_campaign_planning",
  "ads_creative_planning",
  "community_partnerships_planning",
  "marketing_revops_planning",
  "monetization_strategy",
  "market_research",
  "competitor_research",
  "distribution_research",
  "seo_research",
  "release_marketing"
]'::jsonb
WHERE family_key = 'marketer'
  AND available_skills = '[]'::jsonb;
