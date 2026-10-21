import { describe, expect, it } from 'vitest';
import { supportSetupConnectionIssue, buildSupportSetupPrompt } from './supportSetup';
import type { MCPDashboard } from './mcpTypes';
import { resolveSetupAction } from './setupActions';
const dashboard = {platform_enabled:true,can_use_mcp:true,mcp_url:'https://mcp.example.test/mcp',policy:{enabled:true,allowed_toolsets:['support'],allowed_scopes:['helpin.support.read']},available_toolsets:['support'],available_scopes:['helpin.support.read']} as MCPDashboard;
describe('support setup connection',()=>{
 it('requires the server capability and workspace grant',()=>{
  expect(supportSetupConnectionIssue(dashboard)).toBeNull();
  expect(supportSetupConnectionIssue({...dashboard,support_setup_available:false})).toContain('unavailable');
  expect(supportSetupConnectionIssue({...dashboard,policy:{...dashboard.policy,allowed_toolsets:[]}})).toContain('restricted');
 });
 it('builds a prompt bound to one workspace with the same browser destinations as the UI',()=>{
  const path='/settings/inboxes-routing?tab=email';
  const prompt=buildSupportSetupPrompt({workspace_id:'ws-1',instructions:'Verify current state.',steps:[{key:'email',title:'Email',path,status:'available',verification:''}]},'acme','https://helpin.example');
  expect(prompt).toContain(`https://helpin.example${resolveSetupAction('support_email_inbox','acme')}`);
  expect(prompt).toContain('stop if it does not');
 });
});
