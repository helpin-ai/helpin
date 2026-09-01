// @vitest-environment jsdom

import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useSearchCommandStore } from '@/stores/searchCommandStore';
import { WorkspaceCommandSearch } from '../WorkspaceCommandSearch';

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock('@/components/search/SearchCommandPalette', () => ({
  SearchCommandPalette: ({ open }: { open: boolean }) => <div data-testid="search-palette" data-open={String(open)} />,
}));

describe('WorkspaceCommandSearch', () => {
  beforeEach(() => useSearchCommandStore.setState({ open: false }));

  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('opens the shared palette from command and control shortcuts', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => root.render(<WorkspaceCommandSearch />));
    expect(container.querySelector('[data-testid="search-palette"]')?.getAttribute('data-open')).toBe('false');

    act(() => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true, bubbles: true })));
    expect(useSearchCommandStore.getState().open).toBe(true);
    expect(container.querySelector('[data-testid="search-palette"]')?.getAttribute('data-open')).toBe('true');

    act(() => useSearchCommandStore.setState({ open: false }));
    act(() => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'K', ctrlKey: true, bubbles: true })));
    expect(useSearchCommandStore.getState().open).toBe(true);

    act(() => root.unmount());
  });
});
