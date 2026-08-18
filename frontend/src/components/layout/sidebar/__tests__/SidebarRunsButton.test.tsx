// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { useDockStore } from '@/stores/dockStore';
import { SidebarRunsButton } from '../SidebarRunsButton';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('SidebarRunsButton', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    localStorage.clear();
    useDockStore.setState({ collapsed: true, tab: 'chats' });
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    document.body.innerHTML = '';
  });

  it('opens the global Ask Agents dock', () => {
    const dispatch = vi.spyOn(window, 'dispatchEvent');
    act(() => {
      root.render(<TooltipProvider><SidebarRunsButton /></TooltipProvider>);
    });

    act(() => container.querySelector<HTMLButtonElement>('button')?.click());

    expect(dispatch).toHaveBeenCalledWith(expect.objectContaining({ type: 'helpin:ask-agents' }));
    expect(useDockStore.getState()).toMatchObject({ collapsed: false, tab: 'agents' });
  });

  it('labels the sidebar action as Ask Agents', () => {
    act(() => {
      root.render(<TooltipProvider><SidebarRunsButton /></TooltipProvider>);
    });

    const button = container.querySelector<HTMLButtonElement>('button');
    expect(button?.getAttribute('aria-label')).toBe('Ask Agents');
  });
});
