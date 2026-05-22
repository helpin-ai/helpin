-- Migration: backfill_git_connection_scope
-- Preserve existing GitHub/self-hosted rows after git connections move to org scope.

UPDATE git_integrations gi
   SET organization_id = w.organization_id
  FROM workspaces w
 WHERE gi.workspace_id = w.id
   AND gi.organization_id IS NULL
   AND w.organization_id IS NOT NULL;

UPDATE git_repositories gr
   SET base_url = gi.base_url
  FROM git_integrations gi
 WHERE gr.integration_id = gi.id
   AND gr.base_url IS NULL
   AND gi.base_url IS NOT NULL;

UPDATE task_git_links tgl
   SET base_url = COALESCE(gr.base_url, gi.base_url)
  FROM git_repositories gr
  LEFT JOIN git_integrations gi ON gi.id = gr.integration_id
 WHERE tgl.repository_id = gr.id
   AND tgl.base_url IS NULL
   AND COALESCE(gr.base_url, gi.base_url) IS NOT NULL;

UPDATE task_git_links tgl
   SET base_url = gi.base_url
  FROM git_integrations gi
 WHERE tgl.integration_id = gi.id
   AND tgl.base_url IS NULL
   AND gi.base_url IS NOT NULL;
