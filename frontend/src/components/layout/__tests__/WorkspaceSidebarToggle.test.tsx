// @vitest-environment jsdom

import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { SidebarProvider } from '@/components/ui/sidebar';
import { SidebarHeaderToggle, WorkspaceMainContent } from '../WorkspaceSidebarToggle';

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const responsiveState = vi.hoisted(() => ({ isMobile: false }));

vi.mock('@/hooks/use-mobile', () => ({
  useIsMobile: () => responsiveState.isMobile,
}));

describe('WorkspaceSidebarToggle', () => {
  beforeEach(() => {
    responsiveState.isMobile = false;
    document.cookie = 'sidebar_state=; max-age=0';
  });

  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('moves the desktop control from the sidebar header into the page-header line when collapsed', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => root.render(
      <SidebarProvider defaultOpen>
        <SidebarHeaderToggle />
        <WorkspaceMainContent><div>Page</div></WorkspaceMainContent>
      </SidebarProvider>,
    ));

    const headerButton = container.querySelector<HTMLButtonElement>('button[aria-label="Collapse sidebar"]');
    const main = container.querySelector('main');
    expect(headerButton?.getAttribute('aria-expanded')).toBe('true');
    expect(main?.getAttribute('data-sidebar-toggle-visible')).toBe('false');
    expect(container.querySelector('button[aria-label="Open navigation"]')).toBeNull();

    act(() => headerButton?.click());
    const desktopOpener = container.querySelector<HTMLButtonElement>('button[aria-label="Open navigation"]');
    expect(main?.getAttribute('data-sidebar-toggle-visible')).toBe('true');
    expect(desktopOpener?.getAttribute('aria-label')).toBe('Open navigation');
    expect(desktopOpener?.getAttribute('aria-expanded')).toBe('false');
    expect(desktopOpener?.getAttribute('data-sidebar')).toBe('trigger');
    expect(desktopOpener?.parentElement?.className).toContain('absolute');

    act(() => root.unmount());
  });

  it('keeps the mobile opener in the page-header line without reserving a row', () => {
    responsiveState.isMobile = true;
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => root.render(
      <SidebarProvider>
        <WorkspaceMainContent><div>Page</div></WorkspaceMainContent>
        <SidebarHeaderToggle />
      </SidebarProvider>,
    ));

    const main = container.querySelector('main');
    const mobileButton = container.querySelector<HTMLButtonElement>('button[aria-label="Open navigation"]');
    expect(main?.getAttribute('data-sidebar-toggle-visible')).toBe('true');
    expect(mobileButton?.getAttribute('aria-label')).toBe('Open navigation');
    expect(mobileButton?.parentElement?.className).toContain('absolute');
    expect(mobileButton?.parentElement?.className).not.toContain('w-full');

    act(() => mobileButton?.click());
    expect(main?.getAttribute('data-sidebar-toggle-visible')).toBe('false');
    expect(container.querySelector('button[aria-label="Open navigation"]')).toBeNull();

    const drawerButton = container.querySelector<HTMLButtonElement>('button[aria-label="Collapse sidebar"]');
    act(() => drawerButton?.click());
    const restoredMobileOpener = container.querySelector<HTMLButtonElement>('button[aria-label="Open navigation"]');
    expect(main?.getAttribute('data-sidebar-toggle-visible')).toBe('true');
    expect(restoredMobileOpener?.getAttribute('aria-label')).toBe('Open navigation');

    act(() => root.unmount());
  });
});
