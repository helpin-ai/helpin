// @vitest-environment jsdom
import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

let serverAdmin = false;

vi.mock('../SettingsPageFrame', () => ({
  SettingsPageFrame: ({ section, children }: { section: string; children: (context: unknown) => ReactNode }) => (
    <div data-section={section}>
      {children({ workspaceId: 'ws-1', currentWorkspaceSlug: 'acme', permissions: { has: () => true, isServerAdmin: serverAdmin } })}
    </div>
  ),
}));
vi.mock('@/components/settings/server/ServerSignupCard', () => ({
  ServerSignupCard: ({ slug }: { slug: string }) => <div data-testid="signup" data-slug={slug} />,
}));
vi.mock('@/components/settings/server/ServerAdminsCard', () => ({
  ServerAdminsCard: () => <div data-testid="admins" />,
}));

import { ServerSettingsPage } from '../ServerSettingsPage';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  container = null;
  root = null;
  serverAdmin = false;
});

function render() {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => root?.render(<ServerSettingsPage />));
  return container;
}

describe('ServerSettingsPage', () => {
  it('shows signup policy and server admins to server admins', () => {
    serverAdmin = true;
    const page = render();
    expect(page.querySelector('[data-section="server"]')).not.toBeNull();
    expect(page.querySelector('[data-testid="signup"]')?.getAttribute('data-slug')).toBe('acme');
    expect(page.querySelector('[data-testid="admins"]')).not.toBeNull();
  });

  it('keeps workspace admins who are not server admins out', () => {
    const page = render();
    expect(page.querySelector('[data-testid="signup"]')).toBeNull();
    expect(page.textContent).toContain('Only server admins can manage signup and server admins.');
  });
});
