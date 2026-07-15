-- Repair MCP grants whose explicit read_only=false value was replaced by the
-- database default during GORM creation. Writable scopes are conclusive here:
-- the authorization service removes them before persistence for any genuinely
-- read-only grant.

UPDATE mcp_connections
SET read_only = false,
    updated_at = now()
WHERE read_only = true
  AND (
      scopes @> '["helpin.pm.write"]'::jsonb
      OR scopes @> '["helpin.docs.write"]'::jsonb
      OR scopes @> '["helpin.crm.write"]'::jsonb
      OR scopes @> '["helpin.agents.run"]'::jsonb
  );

UPDATE mcp_service_principals
SET read_only = false,
    updated_at = now()
WHERE read_only = true
  AND (
      scopes @> '["helpin.pm.write"]'::jsonb
      OR scopes @> '["helpin.docs.write"]'::jsonb
      OR scopes @> '["helpin.crm.write"]'::jsonb
      OR scopes @> '["helpin.agents.run"]'::jsonb
  );
