// @vitest-environment jsdom

import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { useSearchCommandStore } from '@/stores/searchCommandStore';
import { SidebarSearchFooter } from '../SidebarSearchFooter';

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('SidebarSearchFooter', () => {
  beforeEach(() => useSearchCommandStore.setState({ open: false }));

  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('opens global search with the workspace label and keyboard hint', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => root.render(<SidebarSearchFooter workspaceName="Acme" />));

    const button = container.querySelector('button');
    expect(button?.textContent).toContain('Search Acme');
    expect(button?.textContent).toContain('⌘K');
    expect(button?.getAttribute('aria-keyshortcuts')).toBe('Meta+K Control+K');

    act(() => button?.click());
    expect(useSearchCommandStore.getState().open).toBe(true);

    act(() => root.unmount());
  });
});
