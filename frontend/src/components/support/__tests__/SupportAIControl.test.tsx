// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { useSupportAIControl } from '../SupportAIControl';
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { supportService } from '@/lib/services/supportService';
import type { SupportConversation } from '@/lib/pmTypes';

const state = vi.hoisted(() => ({ edit: true, enabled: true }));
vi.mock('@/hooks/queries/useSession', () => ({ useWorkspaceAccess: () => ({ data: {} }), usePermissions: () => ({ has: () => state.edit }) }));
vi.mock('@/hooks/queries/useWorkspaces', () => ({ useWorkspaceMembers: () => ({ data: [{ user_id: 'teammate', full_name: 'Sam' }] }) }));
vi.mock('@/hooks/queries/useSupport', () => ({ useChatSettings: () => ({ data: { active: true, settings: { ai_enabled: state.enabled, ai_agent_id: 'agent', ai_response_mode: 'ai_first' } } }) }));
vi.mock('@/lib/services/supportService', () => ({ supportService: { changeConversationAIControl: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let root: Root; let container: HTMLDivElement; let client: QueryClient;
const conversation = { id: 'conv', workspace_id: 'ws', source: 'widget', channel: 'widget', status: 'open', ai_state: 'pending', ai_control_version: 4 } as SupportConversation;
beforeEach(() => {
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
  state.edit = true; state.enabled = true;
  vi.mocked(supportService.changeConversationAIControl).mockResolvedValue({ data: { updated: true }, error: null } as never);
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } });
  container = document.createElement('div'); document.body.append(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); client.clear(); vi.clearAllMocks(); vi.unstubAllGlobals(); });
function Harness({ conv }: { conv: SupportConversation }) {
  const control = useSupportAIControl(conv);
  return <><DropdownMenu open><DropdownMenuTrigger>Actions</DropdownMenuTrigger><DropdownMenuContent>{control.item}</DropdownMenuContent></DropdownMenu>{control.confirmation}</>;
}
async function render(conv = conversation) { await act(async () => root.render(<QueryClientProvider client={client}><Harness conv={conv} /></QueryClientProvider>)); }
function item() { return document.querySelector<HTMLElement>('[role="menuitem"]'); }
async function select() { expect(item()).toBeTruthy(); await act(async () => item()!.click()); }
it('pauses from the menu and includes the displayed version', async () => {
  await render(); expect(item()?.getAttribute('aria-label')).toBe('Pause AI'); await select();
  expect(supportService.changeConversationAIControl).toHaveBeenCalledWith('ws', 'conv', { action: 'pause', expected_version: 4, confirm_human_request: false });
});
it('explains ownership release and the next-message wait beside Return to AI', async () => {
  await render({ ...conversation, human_takeover: true, ai_paused_by_user_id: 'teammate' });
  expect(item()?.textContent).toContain('AI paused by Sam');
  expect(item()?.textContent).toContain('releases human assignment');
  expect(item()?.textContent).toContain('next customer message');
  await select();
  expect(supportService.changeConversationAIControl).toHaveBeenCalledWith('ws', 'conv', { action: 'return', expected_version: 4, confirm_human_request: false });
});
it('keeps deliberate confirmation when the customer requested a human', async () => {
  await render({ ...conversation, human_takeover: true, customer_requested_human_at: '2026-09-16T12:00:00Z' });
  await select();
  expect(supportService.changeConversationAIControl).not.toHaveBeenCalled();
  expect(document.querySelector('[role="alertdialog"]')?.textContent).toContain('customer requested a human');
  const confirm = [...document.querySelectorAll('[role="alertdialog"] button')].find(button => button.textContent === 'Return to AI') as HTMLButtonElement;
  await act(async () => confirm.click());
  expect(supportService.changeConversationAIControl).toHaveBeenCalledWith('ws', 'conv', { action: 'return', expected_version: 4, confirm_human_request: true });
});
it('does not offer mutation to a read-only teammate', async () => {
  state.edit = false; await render(); expect(item()).toBeNull();
});
it('shows why return is unavailable when workspace AI is disabled', async () => {
  state.enabled = false; await render({ ...conversation, human_takeover: true });
  expect(item()?.getAttribute('aria-disabled')).toBe('true');
  expect(item()?.textContent).toContain('Enable AI for this channel');
  await select(); expect(supportService.changeConversationAIControl).not.toHaveBeenCalled();
});
it('hides controls for deleted, spam and resolved conversations', async () => {
  for (const conv of [{ ...conversation, anonymized_at: '2026-09-16T12:00:00Z' }, { ...conversation, status: 'resolved' }, { ...conversation, status: 'spam' }]) {
    await render(conv as SupportConversation); expect(item()).toBeNull();
  }
  await render(); expect(item()?.getAttribute('aria-label')).toBe('Pause AI');
});
it('hides when AI is unconfigured and has never been involved', async () => {
  state.enabled = false; await render({ ...conversation, ai_state: undefined }); expect(item()).toBeNull();
});
it('shows the next-message wait after returning to AI', async () => {
  await render({ ...conversation, ai_resumed_at: '2026-09-16T12:00:00Z', last_customer_message_at: '2026-09-16T11:00:00Z' });
  expect(item()?.textContent).toContain('Waiting for the next customer message.');
});
it('prevents duplicate requests while an update is pending', async () => {
  let finish!: (value: never) => void;
  vi.mocked(supportService.changeConversationAIControl).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  await render(); await select();
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
  expect(item()?.getAttribute('aria-busy')).toBe('true');
  await select(); expect(supportService.changeConversationAIControl).toHaveBeenCalledTimes(1);
  await act(async () => finish({ data: { updated: true }, error: null } as never));
});
it('does not carry a human-request confirmation into another conversation', async () => {
  await render({ ...conversation, human_takeover: true, customer_requested_human_at: '2026-09-16T12:00:00Z' });
  await select(); expect(document.querySelector('[role="alertdialog"]')).toBeTruthy();
  await render({ ...conversation, id: 'other', human_takeover: true });
  expect(document.querySelector('[role="alertdialog"]')).toBeNull();
  expect(supportService.changeConversationAIControl).not.toHaveBeenCalled();
});
