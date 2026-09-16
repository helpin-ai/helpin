// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { SupportAIControl } from '../SupportAIControl';
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
  state.edit = true; state.enabled = true;
  vi.mocked(supportService.changeConversationAIControl).mockResolvedValue({ data: { updated: true }, error: null } as never);
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } });
  container = document.createElement('div'); document.body.append(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); client.clear(); vi.clearAllMocks(); });
async function render(conv = conversation) { await act(async () => root.render(<QueryClientProvider client={client}><SupportAIControl conversation={conv} /></QueryClientProvider>)); }
async function click(text: string) {
  const button = [...document.querySelectorAll('button')].find(button => button.textContent === text)!;
  expect(button).toBeTruthy(); await act(async () => button.click());
}
it('pauses without assignment or a customer-facing message and includes the displayed version', async () => {
  await render(); await click('Pause AI');
  expect(supportService.changeConversationAIControl).toHaveBeenCalledWith('ws', 'conv', { action: 'pause', expected_version: 4, confirm_human_request: false });
});
it('shows who paused AI and explains that return waits for the next customer message', async () => {
  await render({ ...conversation, human_takeover: true, ai_paused_by_user_id: 'teammate' });
  expect(container.textContent).toContain('AI paused by Sam');
  expect(container.textContent).toContain('next customer message');
  await click('Return to AI');
  expect(supportService.changeConversationAIControl).toHaveBeenCalledWith('ws', 'conv', { action: 'return', expected_version: 4, confirm_human_request: false });
});
it('requires deliberate confirmation when the customer requested a human', async () => {
  await render({ ...conversation, human_takeover: true, customer_requested_human_at: '2026-09-16T12:00:00Z' });
  await click('Return to AI');
  expect(supportService.changeConversationAIControl).not.toHaveBeenCalled();
  expect(document.querySelector('[role="alertdialog"]')?.textContent).toContain('customer requested a human');
  const confirm = [...document.querySelectorAll('[role="alertdialog"] button')].find(button => button.textContent === 'Return to AI') as HTMLButtonElement;
  await act(async () => confirm.click());
  expect(supportService.changeConversationAIControl).toHaveBeenCalledWith('ws', 'conv', { action: 'return', expected_version: 4, confirm_human_request: true });
});
it('does not offer mutation to a read-only teammate', async () => {
  state.edit = false; await render(); expect(container.querySelector('button')).toBeNull();
});
it('prevents return when workspace AI is disabled', async () => {
  state.enabled = false; await render({ ...conversation, human_takeover: true });
  expect(container.querySelector('button')?.disabled).toBe(true);
});
it('hides controls for deleted and resolved conversations', async () => {
  await render({ ...conversation, anonymized_at: '2026-09-16T12:00:00Z' }); expect(container.textContent).toBe('');
  await render({ ...conversation, status: 'resolved' }); expect(container.textContent).toBe('');
});
