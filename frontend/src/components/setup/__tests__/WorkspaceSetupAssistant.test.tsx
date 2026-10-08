// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import { WorkspaceSetupAssistant, WorkspaceSetupAssistantContent } from '../WorkspaceSetupAssistant';
import { button, click, renderWithQuery as renderQuery, type Rendered } from './setupTestUtils';
import { TooltipProvider } from '@/components/ui/tooltip';
import type { ReactNode } from 'react';
const renderWithQuery = (node: ReactNode) => renderQuery(<TooltipProvider>{node}</TooltipProvider>);
vi.mock('@/lib/api', () => ({ api: { get: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
let rendered: Rendered | undefined;
const writeText=vi.fn().mockResolvedValue(undefined);
const guide={workspace_id:'ws-1',goals:['internal_docs'],instructions:'Confirm recipients and roles before invitations.',sections:[{key:'foundation',title:'Workspace essentials',steps:[{key:'foundation.invitation_created',title:'Invite members',status:'available',path:'/settings/members',verification:'Invitation creation does not prove delivery.'}]}]};
function respond(enabled=true,id='ws-1') {
 vi.mocked(api.get).mockImplementation(async (path:string)=>({data:path.includes('/setup/workspace')?{...guide,workspace_id:id}:{mcp_url:'https://mcp.example.test/mcp',workspace_setup_available:true,platform_enabled:true,can_use_mcp:enabled,policy:{enabled,allowed_toolsets:['context'],allowed_scopes:['helpin.context.read']},available_toolsets:['context'],available_scopes:['helpin.context.read']},error:null,status:200}) as never);
 Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText}});
}
afterEach(async()=>{await rendered?.unmount();rendered=undefined;vi.mocked(api.get).mockReset();writeText.mockClear();});
describe('WorkspaceSetupAssistant',()=>{
 it('loads on opening and copies a complete bound prompt',async()=>{
  respond();rendered=await renderWithQuery(<WorkspaceSetupAssistant workspaceId="ws-1" slug="acme" />);
  expect(api.get).not.toHaveBeenCalled();await click(button(rendered.container,'Set up with AI'));
  await click(button(document.body,'Copy setup prompt'));
  const prompt=writeText.mock.calls[0][0];for(const text of ['ws-1','get_workspace_setup','internal_docs','/w/acme/settings/members','Confirm recipients and roles','does not prove delivery'])expect(prompt).toContain(text);
 });
 it('keeps UI links usable when MCP is disabled',async()=>{
  respond(false);rendered=await renderWithQuery(<WorkspaceSetupAssistantContent workspaceId="ws-1" slug="acme" />);
  expect(button(rendered.container,'Copy setup prompt')?.disabled).toBe(true);
  expect(rendered.container.querySelector('a[href="/w/acme/settings/members"]')).not.toBeNull();
 });
 it('refuses another workspace snapshot and exposes retry',async()=>{
  respond(true,'other');rendered=await renderWithQuery(<WorkspaceSetupAssistantContent workspaceId="ws-1" slug="acme" />);
  expect(button(rendered.container,'Copy setup prompt')).toBeUndefined();
  expect(rendered.container.querySelector('a[href="/w/acme/settings/members"]')).toBeNull();
  respond();await click(button(rendered.container,'Try again'));expect(button(rendered.container,'Copy setup prompt')?.disabled).toBe(false);
 });
 it('does not copy stale permission data after a refresh fails',async()=>{
  respond();rendered=await renderWithQuery(<WorkspaceSetupAssistantContent workspaceId="ws-1" slug="acme" />);
  vi.mocked(api.get).mockResolvedValue({data:null,error:'unavailable',status:503});
  await click(button(rendered.container,'Refresh'));
  expect(button(rendered.container,'Copy setup prompt')).toBeUndefined();
 });
});
