// @vitest-environment jsdom

import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { TooltipProvider } from '@/components/ui/tooltip';
import { useSearchCommandStore } from '@/stores/searchCommandStore';
import { SidebarSearchButton } from '../SidebarSearchButton';

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('SidebarSearchButton', () => {
  beforeEach(() => useSearchCommandStore.setState({ open: false }));

  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('opens global search from the icon with an accessible label and shortcut', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => root.render(<TooltipProvider><SidebarSearchButton /></TooltipProvider>));

    const button = container.querySelector('button');
    expect(button?.getAttribute('aria-label')).toBe('Search your workspace');
    expect(button?.getAttribute('aria-keyshortcuts')).toBe('Meta+K Control+K');

    act(() => button?.click());
    expect(useSearchCommandStore.getState().open).toBe(true);

    act(() => root.unmount());
  });
});
