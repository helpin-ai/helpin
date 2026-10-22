// @vitest-environment jsdom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { MCPDashboard } from '@/lib/mcpTypes';

const createServicePrincipal = vi.fn();
vi.mock('@/lib/services/mcpService', () => ({
  mcpService: {
    createServicePrincipal: (...args: unknown[]) => createServicePrincipal(...args),
  },
}));

import { CreateServiceAccountDialog } from '../MCPAccessPanel';

function dashboard(policy: Partial<MCPDashboard['policy']> = {}): MCPDashboard {
  return {
    policy: {
      workspace_id: 'ws-1',
      enabled: true,
      enforce_read_only: false,
      service_accounts_enabled: true,
      allowed_toolsets: ['context', 'crm'],
      allowed_scopes: ['helpin.context.read', 'helpin.crm.read', 'helpin.crm.write'],
      ...policy,
    },
    connections: [],
    service_principals: [],
    available_toolsets: ['context', 'crm'],
    available_scopes: ['helpin.context.read', 'helpin.crm.read', 'helpin.crm.write'],
    mcp_url: 'https://mcp.example.com/mcp',
    can_manage: true,
    can_view_activity: true,
    can_use_mcp: true,
    platform_enabled: true,
  } as MCPDashboard;
}

describe('CreateServiceAccountDialog', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    createServicePrincipal.mockReset();
    createServicePrincipal.mockResolvedValue({ data: { principal: { id: 'sp-1' }, secret: { token: 'secret' } }, error: null });
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    document.body.innerHTML = '';
  });

  function render(data: MCPDashboard) {
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    act(() => root.render(
      <QueryClientProvider client={client}>
        <CreateServiceAccountDialog open onOpenChange={vi.fn()} workspaceId="ws-1" dashboard={data} onSecret={vi.fn()} />
      </QueryClientProvider>,
    ));
  }

  function typeName(value: string) {
    const input = document.querySelector<HTMLInputElement>('#service-name')!;
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!;
    act(() => {
      setter.call(input, value);
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
  }

  async function clickCreate() {
    const button = Array.from(document.querySelectorAll('button')).find((element) => element.textContent === 'Create')!;
    await act(async () => {
      button.click();
    });
  }

  const writeSwitch = () => document.querySelector<HTMLButtonElement>('#service-allow-writes')!;

  it('creates a read-only account by default', async () => {
    render(dashboard());
    typeName('Reporting');
    await clickCreate();
    expect(createServicePrincipal).toHaveBeenCalledWith('ws-1', expect.objectContaining({
      name: 'Reporting',
      read_only: true,
      scopes: ['helpin.context.read', 'helpin.crm.read'],
    }));
  });

  it('requests write scopes and read_only false when updates are allowed', async () => {
    render(dashboard());
    typeName('Lead import');
    expect(writeSwitch().disabled).toBe(false);
    expect(document.body.textContent).toContain('Create and update records in CRM');
    act(() => writeSwitch().click());
    await clickCreate();
    expect(createServicePrincipal).toHaveBeenCalledWith('ws-1', expect.objectContaining({
      read_only: false,
      scopes: ['helpin.context.read', 'helpin.crm.read', 'helpin.crm.write'],
      toolsets: ['context', 'crm'],
    }));
  });

  it('disables updates and explains why when the policy enforces read-only', async () => {
    render(dashboard({ enforce_read_only: true }));
    expect(writeSwitch().disabled).toBe(true);
    expect(document.body.textContent).toContain('Turn on bounded writes under Permissions');
    typeName('Reporting');
    await clickCreate();
    expect(createServicePrincipal).toHaveBeenCalledWith('ws-1', expect.objectContaining({ read_only: true }));
  });

  it('turns updates off again when the write-capable toolset is deselected', async () => {
    render(dashboard());
    typeName('Lead import');
    act(() => writeSwitch().click());
    const crm = Array.from(document.querySelectorAll('label')).find((label) => label.textContent?.trim() === 'CRM');
    const checkbox = crm?.querySelector<HTMLButtonElement>('button[role="checkbox"]') ?? crm?.parentElement?.querySelector<HTMLButtonElement>('button[role="checkbox"]');
    act(() => checkbox!.click());
    expect(writeSwitch().disabled).toBe(true);
    await clickCreate();
    expect(createServicePrincipal).toHaveBeenCalledWith('ws-1', expect.objectContaining({ read_only: true, toolsets: ['context'] }));
  });
});
