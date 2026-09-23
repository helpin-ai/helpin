// @vitest-environment jsdom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { CRMDataEmptyState } from '../CRMDataEmptyState';

describe('CRMDataEmptyState', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  it.each([
    ['contacts', 'Import contacts'],
    ['companies', 'Import companies'],
  ] as const)('shows %s import as coming soon and keeps create available', (kind, importLabel) => {
    const onCreateClick = vi.fn();
    act(() => root.render(<CRMDataEmptyState kind={kind} onCreateClick={onCreateClick} />));

    const buttons = Array.from(container.querySelectorAll('button'));
    const importButton = buttons.find((button) => button.textContent?.includes(importLabel));
    expect(importButton).toBeDefined();
    expect(importButton?.disabled).toBe(true);
    expect(importButton?.textContent).toContain('Coming soon');

    const createButton = buttons.find((button) => button !== importButton && !button.disabled);
    act(() => createButton?.click());
    expect(onCreateClick).toHaveBeenCalledTimes(1);
  });
});
