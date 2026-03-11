-- Backfill organization_members for workspace members who are missing org membership.
-- When users were invited to workspaces, they were not added to the parent organization.
INSERT INTO organization_members (organization_id, user_id, role)
SELECT DISTINCT w.organization_id, wm.user_id, 'member'
FROM workspace_members wm
JOIN workspaces w ON w.id = wm.workspace_id
WHERE w.organization_id IS NOT NULL
  AND wm.user_id IS NOT NULL
  AND wm.status = 'active'
ON CONFLICT (organization_id, user_id) DO NOTHING;
