// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { expect, it, vi } from 'vitest';
import { SidebarProvider } from '@/components/ui/sidebar';
import { Tag01Icon } from '@/lib/icons';
import { TooltipProvider } from '@/components/ui/tooltip';
import { SettingsRailNav } from '../SettingsRailNav';

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

it('expands Teams when its link is selected and keeps the chevron independently toggleable', () => {
  vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  const onNavigate = vi.fn();
  try {
    act(() => root.render(<TooltipProvider><SidebarProvider><SettingsRailNav workspaceSlug="acme"
      groups={[{ label: 'Workspace', items: [{ label: 'Teams', link: '/w/acme/settings/teams', icon: Tag01Icon,
        children: [{ label: 'Engineering', link: '/w/acme/settings/teams/engineering' }] }] }]}
      isActive={() => false} collapsedGroups={new Set()} toggleGroup={() => {}} onNavigate={onNavigate}
    /></SidebarProvider></TooltipProvider>));
    const link = container.querySelector<HTMLAnchorElement>('a[href="/w/acme/settings/teams"]')!;
    expect(container.textContent).not.toContain('Engineering');
    act(() => link.click());
    expect(onNavigate).toHaveBeenCalledWith('/w/acme/settings/teams');
    expect(container.textContent).toContain('Engineering');
    const collapse = container.querySelector<HTMLButtonElement>('button[aria-label="Collapse Teams"]')!;
    act(() => collapse.click());
    expect(container.textContent).not.toContain('Engineering');
    expect(onNavigate).toHaveBeenCalledTimes(1);
    act(() => link.click());
    expect(container.textContent).toContain('Engineering');
  } finally {
    act(() => root.unmount());
    container.remove();
    vi.unstubAllGlobals();
  }
});
