// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, expect, it, vi } from 'vitest';
import { ExternalMCPAgentAccessPanel } from '../ExternalMCPAgentAccessPanel';
import { automationService } from '@/lib/services/automationService';
import type { Agent } from '@/lib/pmTypes';
import type { ExternalMCPServer } from '@/lib/externalMCPTypes';

vi.mock('@tanstack/react-router', () => ({ Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a> }));
vi.mock('@/lib/services/automationService', () => ({ automationService: { getAgent: vi.fn(), listAgentVersions: vi.fn(), updateAgentVersion: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const server = { id: 'crm', name: 'CRM', enabled: true, status: 'connected', tools: [
  { id: 'search', remote_name: 'search', runtime_alias: 'mcp__crm__search', enabled: true, access: 'read' },
  { id: 'update', remote_name: 'update', runtime_alias: 'mcp__crm__update', enabled: true, access: 'write' },
  { id: 'remove', remote_name: 'remove', runtime_alias: 'mcp__crm__remove', enabled: false, access: 'write' },
] } as ExternalMCPServer;
let root: Root;
let container: HTMLDivElement;
let client: QueryClient;
afterEach(() => { act(() => root?.unmount()); container?.remove(); client?.clear(); vi.resetAllMocks(); });

async function render(allowed: string[] = [], canEdit = true) {
  const agent = { id: 'ava', name: 'Ava', is_system: false, active_version_id: 'v1', allowed_tools: allowed } as Agent;
  vi.mocked(automationService.getAgent).mockResolvedValue({ data: agent } as Awaited<ReturnType<typeof automationService.getAgent>>);
  vi.mocked(automationService.listAgentVersions).mockResolvedValue({ data: [{ id: 'v1', allowed_tools: [...allowed, 'other_tool'] }] } as Awaited<ReturnType<typeof automationService.listAgentVersions>>);
  vi.mocked(automationService.updateAgentVersion).mockResolvedValue({ data: {} } as Awaited<ReturnType<typeof automationService.updateAgentVersion>>);
  container = document.createElement('div'); document.body.appendChild(container); root = createRoot(container);
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  await act(async () => root.render(<QueryClientProvider client={client}><ExternalMCPAgentAccessPanel server={server} workspaceId="ws" workspaceSlug="acme" agents={[agent]} agentsLoading={false} agentsError={false} canManageSettings canEditCustomAgents={canEdit} canEditPresetAgents={false} /></QueryClientProvider>));
  await act(async () => container.querySelector<HTMLButtonElement>('[data-agent-access-row]')!.click());
}
const button = (name: string) => Array.from(container.querySelectorAll('button')).find(b => b.textContent === name)!;
const selected = () => Array.from(container.querySelectorAll('[role="checkbox"]')).map(b => b.getAttribute('aria-checked'));

it('preselects enabled tools for new access, supports all/none and individual choices, and saves explicitly', async () => {
  await render();
  expect(selected()).toEqual(['true', 'true', 'false']);
  expect(automationService.updateAgentVersion).not.toHaveBeenCalled();
  expect(button('Select all').disabled).toBe(true);
  await act(async () => button('Select none').click());
  expect(selected()).toEqual(['false', 'false', 'false']);
  expect(button('Save access').disabled).toBe(true);
  await act(async () => button('Select all').click());
  await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="Remove update for Ava"]')!.click());
  await act(async () => button('Save access').click());
  expect(automationService.updateAgentVersion).toHaveBeenCalledWith('ws', 'ava', 'v1', { allowed_tools: ['other_tool', 'mcp__crm__search'] });
});

it('preserves existing selections including disabled grants, and clears only this server on save', async () => {
  await render(['mcp__crm__search', 'mcp__crm__remove']);
  expect(selected()).toEqual(['true', 'false', 'true']);
  await act(async () => button('Select all').click());
  expect(selected()).toEqual(['true', 'true', 'true']);
  await act(async () => button('Select none').click());
  await act(async () => button('Save access').click());
  expect(automationService.updateAgentVersion).toHaveBeenCalledWith('ws', 'ava', 'v1', { allowed_tools: ['other_tool'] });
});

it('shows actual assignments without granting defaults or bulk controls for read-only access', async () => {
  await render([], false);
  expect(selected()).toEqual(['false', 'false', 'false']);
  expect(button('Select all')).toBeUndefined();
  expect(button('Save access')).toBeUndefined();
  expect(Array.from(container.querySelectorAll<HTMLButtonElement>('[role="checkbox"]')).every(b => b.disabled)).toBe(true);
});
