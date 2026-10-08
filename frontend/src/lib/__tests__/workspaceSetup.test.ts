import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { buildWorkspaceSetupPrompt, workspaceSetupConnectionIssue, workspaceSetupPath, workspaceSetupAccessLabels } from '../workspaceSetup';
import { resolveSetupAction } from '../setupActions';
import type { WorkspaceSetupGuide } from '../setupTypes';
import type { MCPDashboard } from '../mcpTypes';

const guide: WorkspaceSetupGuide = { workspace_id: 'ws-1', goals: ['customer_support', 'help_center_docs'], instructions: 'Confirm recipients and roles. Reuse existing configuration.', sections: [{ key: 'foundation', title: 'Workspace essentials', steps: [{ key: 'foundation.invitation_created', title: 'Invite members', status: 'available', path: '/settings/members', verification: 'An invitation does not prove delivery or membership.' }, { key: 'private', title: 'Unavailable step', status: 'blocked', blocked_reason: 'Access needed', verification: 'No evidence available.' }] }] };

describe('workspace setup handoff', () => {
 it('includes identity, goals, each current check, exact links and verification limits', () => {
  const prompt=buildWorkspaceSetupPrompt(guide,'acme','https://app.example.test');
  for (const text of ['ws-1','get_current_context','get_workspace_setup','customer_support','help_center_docs','https://app.example.test/w/acme/settings/members','Invite members','available','Access needed','does not prove delivery','Confirm recipients and roles','do not create another workspace','Refresh']) expect(prompt).toContain(text);
 });
 it('only builds same-workspace relative links',()=>{
  expect(workspaceSetupPath('/settings/teams','acme')).toBe('/w/acme/settings/teams');
  for(const path of ['//other.test','https://other.test','/../other','/..#other','/..?other','/settings/../../other','/settings\\members']) expect(workspaceSetupPath(path,'acme')).toBeUndefined();
 });
 it('requires the deployed inspection tool and Context grant',()=>{
  const dashboard={platform_enabled:true,can_use_mcp:true,workspace_setup_available:true,mcp_url:'https://mcp.example.test',policy:{enabled:true,allowed_toolsets:['context'],allowed_scopes:['helpin.context.read']},available_toolsets:['context'],available_scopes:['helpin.context.read']} as MCPDashboard;
  expect(workspaceSetupConnectionIssue(dashboard)).toBeNull();
  expect(workspaceSetupConnectionIssue({...dashboard,workspace_setup_available:undefined})).toContain('unavailable');
  expect(workspaceSetupConnectionIssue({...dashboard,policy:{...dashboard.policy,allowed_scopes:[]}})).toContain('Context read');
  expect(workspaceSetupAccessLabels(guide.goals)).toEqual(['Context','Support','Docs']);
 });
 it('keeps backend handoff paths aligned with the actual Setup guide routes',()=>{
  const source=readFileSync(new URL('../../../../server/internal/service/workspace_setup.go',import.meta.url),'utf8');
  const block=source.split('var workspaceSetupPaths = map[string]string{')[1].split('\n}')[0];
  const entries=[...block.matchAll(/"([a-z_]+)":\s*"([^"]+)"/g)];
  expect(entries.length).toBeGreaterThan(25);
  for(const [,key,path] of entries) expect(resolveSetupAction(key,'acme'),key).toBe(`/w/acme${path}`);
 });
});
