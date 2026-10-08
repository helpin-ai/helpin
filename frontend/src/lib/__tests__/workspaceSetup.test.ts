import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { buildWorkspaceSetupPrompt, workspaceSetupConnectionIssue, workspaceSetupPath, workspaceSetupAccessLabels, workspaceSetupAccess, workspaceSetupPolicy, workspaceSetupNeedsAccess } from '../workspaceSetup';
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
  expect(workspaceSetupAccessLabels(guide.goals)).toEqual(['Context','Support','Docs','Agents']);
 });
 it('includes dependencies and limits setup writes to the selected goals', () => {
  const internal = workspaceSetupAccess(['internal_docs']);
  expect(internal.scopes).toContain('helpin.docs.write');
  expect(internal.scopes).toContain('helpin.agents.run');
  expect(internal.scopes).not.toContain('helpin.docs.publish');
  expect(internal.scopes).not.toContain('helpin.crm.write');
  const support = workspaceSetupAccess(['customer_support']);
  expect(support.scopes).toEqual(expect.arrayContaining(['helpin.support.read', 'helpin.support.write', 'helpin.docs.write', 'helpin.docs.publish']));
  expect(workspaceSetupAccess(['help_center_docs']).scopes).toContain('helpin.support.read');
  expect(workspaceSetupAccess(['sales_crm']).scopes).toContain('helpin.crm.write');
  expect(new Set(support.scopes).size).toBe(support.scopes.length);
 });
 it('adds setup permissions without removing other grants or changing automation account policy', () => {
  const required = workspaceSetupAccess(['internal_docs']);
  const dashboard = {platform_enabled:true,workspace_setup_available:true,can_manage:true,policy:{enabled:true,enforce_read_only:true,service_accounts_enabled:false,allowed_toolsets:['context','crm'],allowed_scopes:['helpin.context.read','helpin.crm.read']},available_toolsets:[...required.toolsets,'crm'],available_scopes:[...required.scopes,'helpin.crm.read']} as MCPDashboard;
  expect(workspaceSetupNeedsAccess(dashboard,['internal_docs'])).toBe(true);
  const policy = workspaceSetupPolicy(dashboard,['internal_docs']);
  expect(policy.enforce_read_only).toBe(false);
  expect(policy.service_accounts_enabled).toBe(false);
  expect(policy.allowed_scopes).toContain('helpin.crm.read');
  expect(policy.allowed_scopes).not.toContain('helpin.crm.write');
  expect(dashboard.policy.enforce_read_only).toBe(true);
  expect(workspaceSetupNeedsAccess({...dashboard,policy:{...dashboard.policy,...policy}},['internal_docs'])).toBe(false);
  expect(() => workspaceSetupPolicy({...dashboard,can_manage:false},['internal_docs'])).toThrow();
  expect(() => workspaceSetupPolicy({...dashboard,available_scopes:['helpin.context.read']},['internal_docs'])).toThrow();
 });
 it('puts the same requested permissions into the setup prompt', () => {
  const prompt = buildWorkspaceSetupPrompt(guide,'acme','https://app.example.test');
  for (const scope of workspaceSetupAccess(guide.goals).scopes) expect(prompt).toContain(scope);
 });
 it('keeps backend handoff paths aligned with the actual Setup guide routes',()=>{
  const source=readFileSync(new URL('../../../../server/internal/service/workspace_setup.go',import.meta.url),'utf8');
  const block=source.split('var workspaceSetupPaths = map[string]string{')[1].split('\n}')[0];
  const entries=[...block.matchAll(/"([a-z_]+)":\s*"([^"]+)"/g)];
  expect(entries.length).toBeGreaterThan(25);
  for(const [,key,path] of entries) expect(resolveSetupAction(key,'acme'),key).toBe(`/w/acme${path}`);
 });
});
