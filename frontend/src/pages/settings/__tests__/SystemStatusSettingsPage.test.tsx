// @vitest-environment jsdom
import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

let permissions = new Set<string>();
let role = 'admin';

vi.mock('../SettingsPageFrame', () => ({
  SettingsPageFrame: ({ section, children }: { section: string; children: (context: unknown) => ReactNode }) => (
    <div data-section={section}>
      {children({
        workspaceId: 'ws-1',
        currentWorkspaceSlug: 'acme',
        access: { membership: { role } },
        permissions: { has: (permission: string) => permissions.has(permission) },
      })}
    </div>
  ),
}));
vi.mock('@/components/setup/SystemStatusPanel', () => ({
  SystemStatusPanel: ({ workspaceId, slug, canManage, isOwner }: { workspaceId: string; slug: string; canManage: boolean; isOwner: boolean }) => (
    <div data-testid="panel" data-workspace={workspaceId} data-slug={slug} data-can-manage={String(canManage)} data-owner={String(isOwner)} />
  ),
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
  role = 'admin';
});

function render() {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => root?.render(<SystemStatusSettingsPage />));
  return container;
}

describe('SystemStatusSettingsPage', () => {
  it('shows server services to workspace admins', () => {
    permissions = new Set(['workspace.update']);
    role = 'owner';
    const page = render();

    expect(page.querySelector('[data-section="system-status"]')).not.toBeNull();
    const panel = page.querySelector('[data-testid="panel"]');
    expect(panel?.getAttribute('data-workspace')).toBe('ws-1');
    expect(panel?.getAttribute('data-can-manage')).toBe('true');
    expect(panel?.getAttribute('data-owner')).toBe('true');
  });

  it('keeps server services from members without workspace admin access', () => {
    permissions = new Set(['workspace.read', 'settings.read']);
    const page = render();

    expect(page.querySelector('[data-testid="panel"]')).toBeNull();
    expect(page.textContent).toContain('Only workspace admins can view system status.');
  });
});
