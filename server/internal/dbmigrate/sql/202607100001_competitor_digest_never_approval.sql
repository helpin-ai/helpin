-- Keep competitor digest runs fully autonomous, including document and task writes.

UPDATE agent_templates
SET approval_mode = 'never',
    updated_at = NOW()
WHERE key IN ('competitive_intelligence_digest', 'competitors_changelog_tracking_report')
  AND approval_mode = 'always';

UPDATE agents
SET approval_mode = 'never',
    updated_at = NOW()
WHERE approval_mode = 'always'
  AND (
    template_key IN ('competitive_intelligence_digest', 'competitors_changelog_tracking_report')
    OR source_template_key IN ('competitive_intelligence_digest', 'competitors_changelog_tracking_report')
  );
