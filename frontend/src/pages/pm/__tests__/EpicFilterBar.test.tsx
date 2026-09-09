// @vitest-environment jsdom
import { act, useState } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import { EpicFilterBar } from '../EpicFilterBar';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let root: Root;

beforeEach(() => {
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
  HTMLElement.prototype.scrollIntoView = vi.fn();
});
afterEach(() => {
  act(() => root?.unmount());
  document.body.innerHTML = '';
  vi.unstubAllGlobals();
});

async function render(initial: Record<string, string[]> = {}) {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  function Filters() {
    const [values, setValues] = useState(initial);
    const [search, setSearch] = useState('Launch');
    const [archived, setArchived] = useState(true);
    return <TooltipProvider><EpicFilterBar
      search={search} onSearchChange={setSearch}
      categories={[
        { key: 'owner', label: 'Owner', emptyLabel: 'Any owner', selected: values.owner ?? [], options: [
          { value: 'alice', label: 'Alice', leading: <span data-testid="owner-avatar">A</span> },
          { value: 'bob', label: 'Bob' },
        ] },
        { key: 'state', label: 'State', emptyLabel: 'Any state', selected: values.state ?? [], options: [
          { value: 'active', label: 'Active' },
        ] },
      ]}
      onCategoryChange={(key, next) => setValues(current => ({ ...current, [key]: next }))}
      onClearAll={() => { setValues({}); setSearch(''); setArchived(false); }}
      showArchived={archived} onToggleShowArchived={setArchived}
      groupBy="none" groupByOptions={[{ value: 'none', label: 'None' }]} onGroupByChange={() => {}}
      displayProperties={[]} visibleProperties={[]} onVisiblePropertiesChange={() => {}}
    /></TooltipProvider>;
  }
  await act(async () => root.render(<Filters />));
  return container;
}

function pill(container: HTMLElement, label: string) {
  return container.querySelector<HTMLButtonElement>(`[aria-label="Remove ${label} filter"]`)?.parentElement;
}

describe('epic applied filters', () => {
  it('shows applied filters in order, edits their values, and removes one without clearing other filters', async () => {
    const container = await render({ owner: ['alice'], state: ['active'] });
    expect(Array.from(container.querySelectorAll('[aria-label^="Remove "]')).map(node => node.getAttribute('aria-label')))
      .toEqual(['Remove Owner filter', 'Remove State filter']);
    expect(pill(container, 'Owner')?.textContent).toContain('OwnerisAlice');
    await act(async () => pill(container, 'Owner')!.querySelector('button')!.click());
    expect(document.querySelector('[data-testid="owner-avatar"]')).not.toBeNull();
    await act(async () => document.querySelector<HTMLElement>('[cmdk-item][data-value="bob"]')!.click());
    expect(pill(container, 'Owner')?.textContent).toContain('2 selected');
    await act(async () => document.querySelector<HTMLElement>('[cmdk-item][data-value="alice"]')!.click());
    expect(pill(container, 'Owner')?.textContent).toContain('Bob');
    await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="Remove Owner filter"]')!.click());
    expect(pill(container, 'Owner')).toBeUndefined();
    expect(pill(container, 'State')?.textContent).toContain('Active');
    expect(container.querySelector<HTMLInputElement>('input')?.value).toBe('Launch');
    expect(container.querySelector('[aria-label="Show archived"]')?.getAttribute('aria-checked')).toBe('true');
  });

  it('adds a pill from a category dropdown and clears selections, search, and archived together', async () => {
    const container = await render();
    expect(container.querySelector('[aria-label^="Remove "]')).toBeNull();
    await act(async () => container.querySelector<HTMLButtonElement>('[aria-label^="State:"]')!.click());
    await act(async () => document.querySelector<HTMLElement>('[cmdk-item][data-value="active"]')!.click());
    expect(pill(container, 'State')?.textContent).toContain('Active');
    const clear = Array.from(container.querySelectorAll('button')).find(button => button.textContent === 'Clear all')!;
    expect(Array.from(container.querySelectorAll('button')).some(button => button.textContent === 'Clear Filters')).toBe(false);
    await act(async () => clear.click());
    expect(container.querySelector('[aria-label^="Remove "]')).toBeNull();
    expect(container.querySelector<HTMLInputElement>('input')?.value).toBe('');
    expect(container.querySelector('[aria-label="Show archived"]')?.getAttribute('aria-checked')).toBe('false');
  });
});
