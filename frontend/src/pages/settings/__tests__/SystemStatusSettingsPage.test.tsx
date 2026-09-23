// @vitest-environment jsdom
import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

let permissions = new Set<string>();
let serverAdmin = false;
let role = 'admin';

vi.mock('../SettingsPageFrame', () => ({
  SettingsPageFrame: ({ section, children }: { section: string; children: (context: unknown) => ReactNode }) => (
    <div data-section={section}>
      {children({
        workspaceId: 'ws-1',
        currentWorkspaceSlug: 'acme',
        access: { membership: { role } },
        permissions: { has: (permission: string) => permissions.has(permission), isServerAdmin: serverAdmin },
      })}
    </div>
  ),
}));
vi.mock('@/components/setup/SystemStatusPanel', () => ({
  SystemStatusPanel: ({ workspaceId, slug, canManage, isOwner }: { workspaceId: string; slug: string; canManage: boolean; isOwner: boolean }) => (
    <div data-testid="panel" data-workspace={workspaceId} data-slug={slug} data-can-manage={String(canManage)} data-owner={String(isOwner)} />
  ),
}));
vi.mock('@/components/settings/server/AppEmailSettingsCard', () => ({
  AppEmailSettingsCard: () => <div data-testid="email-settings" />,
}));

import { SystemStatusSettingsPage } from '../SystemStatusSettingsPage';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  container = null;
  root = null;
  permissions = new Set();
  serverAdmin = false;
  role = 'admin';
});

function render() {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => root?.render(<QueryClientProvider client={new QueryClient()}><SystemStatusSettingsPage /></QueryClientProvider>));
  return container;
}

describe('SystemStatusSettingsPage', () => {
  it('shows server services and application email to server admins', () => {
    serverAdmin = true;
    role = 'owner';
    const page = render();

    expect(page.querySelector('[data-section="system-status"]')).not.toBeNull();
    const panel = page.querySelector('[data-testid="panel"]');
    expect(panel?.getAttribute('data-workspace')).toBe('ws-1');
    expect(panel?.getAttribute('data-can-manage')).toBe('true');
    expect(panel?.getAttribute('data-owner')).toBe('true');
    expect(page.querySelector('[data-testid="email-settings"]')).not.toBeNull();
  });

  it('keeps server services from workspace admins who are not server admins', () => {
    permissions = new Set(['workspace.update', 'settings.manage']);
    const page = render();

    expect(page.querySelector('[data-testid="panel"]')).toBeNull();
    expect(page.querySelector('[data-testid="email-settings"]')).toBeNull();
    expect(page.textContent).toContain('Only server admins can view system status.');
  });

  it('announces the GitHub result it returned with and strips it from the URL', async () => {
    serverAdmin = true;
    window.history.replaceState(null, '', '/w/acme/settings/system-status?github=error&github_message=GitHub+could+not+confirm+the+installation.');
    const page = render();
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });

    const status = page.querySelector('[role="status"]');
    expect(status?.textContent).toBe('GitHub could not confirm the installation.');
    expect(status?.className).toContain('text-quiet-accent');
    expect(window.location.search).toBe('');
    expect(page.querySelector('[data-testid="panel"]')).not.toBeNull();
    window.history.replaceState(null, '', '/');
  });
});
