// @vitest-environment jsdom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ExternalMCPConnections } from '../ExternalMCPConnections';

const { confirmMock, updateServerMock } = vi.hoisted(() => ({ confirmMock: vi.fn(), updateServerMock: vi.fn() }));

vi.mock('@tanstack/react-router', () => ({ Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a> }));
vi.mock('@/hooks/queries/useAgents', () => ({ useAgents: () => ({
  data: [
    { id: 'agent-ava', name: 'Ava', icon_key: 'violet_star', is_system: false, allowed_tools: ['mcp__crm__search'] },
    { id: 'agent-ben', name: 'Ben', icon_key: 'ocean_orbit', is_system: false, allowed_tools: [] },
  ], isLoading: false, isError: false,
}) }));
vi.mock('@/hooks/queries/useExternalMCP', () => {
  const mutation = () => ({ isPending: false, mutateAsync: vi.fn() });
  return {
    useExternalMCPProviders: () => ({ data: { enabled: true, providers: [] }, isLoading: false, isError: false }),
    useExternalMCPServers: () => ({ data: [{
      id: 'server-1', name: 'CRM server', endpoint_url: 'https://example.com/mcp', enabled: true,
      status: 'connected', auth_type: 'none', tools: [{ id: 'tool-1', remote_name: 'search', runtime_alias: 'mcp__crm__search', enabled: true, access: 'read' }],
    }], isLoading: false, isError: false, refetch: vi.fn() }),
    useDeleteExternalMCPServer: mutation,
    useRefreshExternalMCPTools: mutation,
    useStartExternalMCPOAuth: mutation,
    useUpdateExternalMCPServer: () => ({ isPending: false, mutateAsync: updateServerMock }),
    useUpdateExternalMCPTools: mutation,
  };
});
vi.mock('@/components/ui/confirm-dialog', () => ({ useConfirm: () => confirmMock }));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('ExternalMCPConnections agent access', () => {
  let root: Root | undefined;
  let container: HTMLDivElement | undefined;

  afterEach(() => {
    if (root) act(() => root?.unmount());
    container?.remove();
    root = undefined;
    container = undefined;
    confirmMock.mockReset();
    updateServerMock.mockReset();
  });

  it('warns before disabling a server assigned to an agent', async () => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    await act(async () => root?.render(
      <QueryClientProvider client={client}>
        <ExternalMCPConnections workspaceId="ws-1" workspaceSlug="acme" canManageSettings canEditCustomAgents canEditPresetAgents createOpen={false} onCreateOpenChange={vi.fn()} />
      </QueryClientProvider>,
    ));
    const toggle = container.querySelector<HTMLButtonElement>('section [role="switch"]')!;
    confirmMock.mockResolvedValueOnce(false);
    await act(async () => toggle.click());
    expect(confirmMock).toHaveBeenCalledWith(expect.objectContaining({
      title: 'Disable CRM server?',
      description: expect.stringContaining('1 agent'),
    }));
    expect(updateServerMock).not.toHaveBeenCalled();

    confirmMock.mockResolvedValueOnce(true);
    updateServerMock.mockResolvedValue(undefined);
    await act(async () => toggle.click());
    expect(updateServerMock).toHaveBeenCalledWith({ serverId: 'server-1', request: { enabled: false } });
    client.clear();
  });

  it('expands inline with agent avatars and clear access states', async () => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    await act(async () => root?.render(
      <QueryClientProvider client={client}>
        <ExternalMCPConnections workspaceId="ws-1" workspaceSlug="acme" canManageSettings canEditCustomAgents canEditPresetAgents createOpen={false} onCreateOpenChange={vi.fn()} />
      </QueryClientProvider>,
    ));

    const card = container.querySelector('section')!;
    await act(async () => (card.querySelector('button[aria-label="Show agent access"]') as HTMLButtonElement).click());
    const panel = card.querySelector('[data-external-mcp-agent-access]')!;
    expect(panel).toBeTruthy();
    expect(panel.querySelectorAll('[data-agent-access-row]')).toHaveLength(2);
    expect(panel.textContent).toContain('Has access');
    expect(panel.textContent).toContain('No access');
    expect(panel.querySelectorAll('[data-agent-access-row] svg')).toHaveLength(2);

    await act(async () => (panel.querySelector('[data-agent-access-row]') as HTMLButtonElement).click());
    expect(panel.textContent).toContain('Choose which CRM server tools this agent can use.');
    expect(panel.querySelector('[role="checkbox"]')).toBeTruthy();
    await act(async () => (Array.from(panel.querySelectorAll('button')).find((button) => button.textContent?.includes('All agents'))!).click());

    await act(async () => (Array.from(card.querySelectorAll('button')).find((button) => button.textContent?.includes('Show tools'))!).click());
    expect(card.querySelector('[data-external-mcp-agent-access]')).toBeNull();
    expect(card.textContent).toContain('Workspace tools');

    await act(async () => (card.querySelector('button[aria-label="Show agent access"]') as HTMLButtonElement).click());

    await act(async () => (card.querySelector('button[aria-label="Hide agent access"]') as HTMLButtonElement).click());
    expect(card.querySelector('[data-external-mcp-agent-access]')).toBeNull();
    client.clear();
  });
});
