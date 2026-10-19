// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { beforeEach, afterEach, describe, it, expect, vi } from 'vitest';
import { InboxSenderCell, InboxAIRepliesCell } from '../InboxTableControls';
import type { SupportMailbox } from '@/lib/pmTypes';
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
const state = vi.hoisted(() => ({ save: vi.fn(), update: vi.fn(), canManage: true }));
vi.mock('@/hooks/queries', () => ({ useWorkspaceAccess: () => ({ data: {} }), usePermissions: () => ({ has: () => state.canManage }) }));
vi.mock('@/hooks/queries/useSupport', () => ({
 useChatSettings: () => ({ data: { settings: { ai_enabled: true, ai_agent_id: 'agent', ai_response_mode: 'ai_first', ai_reply_channels: 'both' } } }),
 useSupportEmailSenders: () => ({ data: [ { id:'verified', email:'support@example.com', dkim_verified:true, return_path_domain_verified:true }, { id:'pending', email:'pending@example.com', dkim_verified:false } ] }),
 useSetDefaultSupportEmailSender: () => ({ mutateAsync: state.save }),
 useUpdateMailbox: () => ({ mutateAsync: state.update }),
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn() } }));
const mailbox = { id:'admin', workspace_id:'ws', name:'Admin', active:true } as SupportMailbox;
let root: Root;
let container: HTMLDivElement;
beforeEach(() => { state.canManage = true; state.save.mockReset().mockResolvedValue({}); state.update.mockReset().mockResolvedValue({}); container=document.createElement('div'); document.body.append(container); root=createRoot(container); });
afterEach(() => { act(() => root.unmount()); container.remove(); });
async function click(text:string) { const button=Array.from(document.querySelectorAll('button')).find(el=>el.textContent?.includes(text)); expect(button).toBeDefined(); await act(async()=>button!.click()); }
describe('inbox table controls', () => {
 it('selects only verified senders and changes only the selected inbox', async () => {
  act(()=>root.render(<InboxSenderCell workspaceId="ws" mailbox={mailbox} />));
  await click('Choose sender');
  expect(document.body.textContent).not.toContain('pending@example.com');
  expect(document.querySelector('a')?.getAttribute('href')).toBe('./inboxes-routing?tab=senders&create_sender=true');
  await click('support@example.com');
  expect(state.save).toHaveBeenCalledWith({ senderId:'verified', payload:{ default_scope:'mailbox',mailbox_ids:['admin'],single_mailbox:true } });
 });
 it('shows the effective mode and allows disabling replies', async () => {
  act(()=>root.render(<InboxAIRepliesCell workspaceId="ws" mailbox={mailbox} />));
  expect(container.textContent).toBe('On');
  await click('On'); await click('Off for this inbox');
  expect(state.update).toHaveBeenCalledWith({ mailboxId:'admin', payload:{ ai_replies_disabled:true } });
 });
 it('disables editing without support administration permission', () => {
  state.canManage=false;
  act(()=>root.render(<InboxSenderCell workspaceId="ws" mailbox={mailbox} />));
  expect(container.querySelector('button')?.disabled).toBe(true);
 });
 it('keeps the sender selector open on save failure', async () => {
  state.save.mockRejectedValueOnce(new Error('network'));
  act(()=>root.render(<InboxSenderCell workspaceId="ws" mailbox={mailbox} />));
  await click('Choose sender'); await click('support@example.com');
  expect(document.body.textContent).toContain('New sender address');
 });
});
