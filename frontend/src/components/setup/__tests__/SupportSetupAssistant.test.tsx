// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import { SupportSetupAssistant } from '../SupportSetupAssistant';
import { button, click, renderWithQuery, type Rendered } from './setupTestUtils';
vi.mock('@/lib/api', () => ({ api: { get: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { success:vi.fn(), error:vi.fn() } }));
let rendered: Rendered | undefined;
const writeText = vi.fn().mockResolvedValue(undefined);
const guide = {workspace_id:'ws-1', instructions:'Reuse existing configuration. Verify after each step.', steps:[{key:'support.email_inbox_connected',title:'Support email',status:'available',path:'/settings/inboxes-routing?tab=email',verification:'Verify receipt separately.'},{key:'support.test_conversation',title:'Test a conversation',status:'unable_to_verify',path:'/support',verification:'Observe delivery and handoff.'}]};
function respond(enabled=true) {
 vi.mocked(api.get).mockImplementation(async (path:string) => ({data:path.includes('/setup/support') ? guide : {mcp_url:'https://mcp.example.test/mcp',platform_enabled:true,can_use_mcp:enabled,policy:{enabled,allowed_toolsets:['support'],allowed_scopes:['helpin.support.read']},available_toolsets:['support'],available_scopes:['helpin.support.read']},error:null,status:200}) as never);
 Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText}});
}
afterEach(async()=>{await rendered?.unmount();rendered=undefined;vi.mocked(api.get).mockReset();writeText.mockClear();});
async function open(){rendered=await renderWithQuery(<SupportSetupAssistant workspaceId="ws-1" slug="acme" />);await click(button(rendered.container,'Set up with AI'));}
describe('SupportSetupAssistant',()=>{
 it('loads only when opened and copies a workspace-bound prompt with browser handoffs',async()=>{
  respond();rendered=await renderWithQuery(<SupportSetupAssistant workspaceId="ws-1" slug="acme" />);
  expect(api.get).not.toHaveBeenCalled();await click(button(rendered.container,'Set up with AI'));
  await click(button(document.body,'Copy setup prompt'));
  const prompt=writeText.mock.calls[0][0];expect(prompt).toContain('ws-1');expect(prompt).toContain('get_support_setup');expect(prompt).toContain('/w/acme/settings/inboxes-routing?tab=email');expect(prompt).toContain('Reuse existing configuration');
  expect(document.body.textContent).toContain('Test needed');
 });
 it('explains disabled MCP and keeps manual setup accessible',async()=>{
  respond(false);await open();expect(button(document.body,'Copy setup prompt')?.disabled).toBe(true);
  expect(document.body.textContent).toContain('MCP access is disabled');
  expect(document.querySelector('a[href="/w/acme/settings/mcp"]')).not.toBeNull();
  expect(document.querySelector('a[href="/w/acme/settings/inboxes-routing?tab=email"]')).not.toBeNull();
 });
 it('refreshes both setup evidence and connection policy',async()=>{
  respond();await open();const calls=vi.mocked(api.get).mock.calls.length;
  await click(button(document.body,'Refresh'));expect(vi.mocked(api.get).mock.calls.length).toBeGreaterThanOrEqual(calls+2);
 });
 it('offers retry instead of copying an incomplete prompt after inspection fails',async()=>{
  respond();vi.mocked(api.get).mockResolvedValue({data:null,error:'unavailable',status:503});await open();
  expect(document.body.textContent).toContain('Couldn’t load setup');expect(button(document.body,'Copy setup prompt')).toBeUndefined();
  respond();await click(button(document.body,'Try again'));expect(button(document.body,'Copy setup prompt')?.disabled).toBe(false);
 });
});
