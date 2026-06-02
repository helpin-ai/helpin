// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { describe, expect, it, vi } from 'vitest';

import { TaskFilterBar, TaskFilterProvider, TaskFilterTrigger } from '../TaskFilters';
import type { BoardFilters } from '@/stores/pmBoardStore';
import type { AssignableMember } from '@/lib/types';
import type { Label } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock('@/hooks/queries', () => ({
  useCompanies: () => ({ data: { data: [] } }),
  useContacts: () => ({ data: { data: [] } }),
  useConversations: () => ({ data: { data: [] } }),
  useDeals: () => ({ data: { data: [] } }),
  useWorkspaceMemberPresenceMap: () => ({ data: new Map() }),
}));

vi.stubGlobal(
  'ResizeObserver',
  class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
);

window.HTMLElement.prototype.scrollIntoView = vi.fn();

function renderFilters({
  onChange = vi.fn(),
  externalFilters,
  assignableMembers = [],
  labels = [],
}: {
  onChange?: (filters: BoardFilters) => void;
  externalFilters?: BoardFilters;
  assignableMembers?: AssignableMember[];
  labels?: Label[];
} = {}) {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);

  act(() => {
    root.render(
      <TaskFilterProvider
        workspaceId="workspace-1"
        assignableMembers={assignableMembers}
        labels={labels}
        epics={[]}
        sprints={[]}
        onChange={onChange}
        externalFilters={externalFilters}
      >
        <TaskFilterTrigger />
        <TaskFilterBar />
      </TaskFilterProvider>,
    );
  });

  return {
    container,
    onChange,
    cleanup: () => {
      act(() => {
        root.unmount();
      });
      container.remove();
    },
  };
}

function clickElement(element: Element | null | undefined) {
  expect(element).toBeTruthy();
  act(() => {
    element?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  });
}

function findCommandItem(label: string) {
  return Array.from(document.body.querySelectorAll('[cmdk-item]')).find((element) =>
    element.textContent?.includes(label),
  );
}

describe('TaskFilters', () => {
  it('lets users pick a filter value directly without applying the first option', () => {
    const onChange = vi.fn();
    const rendered = renderFilters({ onChange });

    clickElement(rendered.container.querySelector('button'));
    clickElement(findCommandItem('Priority'));

    expect(onChange).not.toHaveBeenCalled();
    expect(document.body.textContent).toContain('Urgent');
    expect(document.body.textContent).toContain('High');
    expect(rendered.container.textContent).toContain('Choose value');
    expect(document.body.querySelector('input[placeholder="Search priority..."]')).toBeFalsy();
    expect(document.body.textContent).not.toContain('Back');
    expect(document.body.querySelector('button[aria-label="Back to filter fields"]')).toBeTruthy();

    clickElement(findCommandItem('High'));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith({ priority: 'high' });

    rendered.cleanup();
  });

  it('keeps search for filter value lists that need lookup', () => {
    const rendered = renderFilters({
      assignableMembers: [
        {
          id: 'member-1',
          user_id: 'user-1',
          role: 'member',
          email: 'alice@example.com',
          display_name: 'Alice Johnson',
          status: 'active',
        },
      ],
    });

    clickElement(rendered.container.querySelector('button'));
    clickElement(findCommandItem('Owner'));

    expect(document.body.querySelector('input[placeholder="Search owner..."]')).toBeTruthy();

    rendered.cleanup();
  });

  it('keeps checkboxes fixed width and widens lookup value pickers for long values', () => {
    const rendered = renderFilters({
      assignableMembers: [
        {
          id: 'member-1',
          user_id: 'user-1',
          role: 'member',
          email: 'alexandria.owner.with.a.long.email@example.com',
          display_name: 'Alexandria Owner With A Very Long Display Name',
          status: 'active',
        },
      ],
    });

    clickElement(rendered.container.querySelector('button'));
    clickElement(findCommandItem('Owner'));

    const popover = document.body.querySelector('[data-radix-popper-content-wrapper] [data-slot="popover-content"]');
    const ownerOption = findCommandItem('Alexandria Owner With A Very Long Display Name');
    const checkbox = ownerOption?.querySelector('div');
    const label = ownerOption?.querySelector('span.truncate');

    expect(popover?.className).toContain('w-80');
    expect(checkbox?.className).toContain('shrink-0');
    expect(label?.className).toContain('min-w-0');
    expect(label?.className).toContain('flex-1');

    rendered.cleanup();
  });

  it('shows label color swatches in filter value lists', () => {
    const rendered = renderFilters({
      labels: [
        {
          id: 'label-1',
          workspace_id: 'workspace-1',
          name: 'Escalated',
          color: 'ef4444',
          archived: false,
          created_at: '',
          updated_at: '',
        },
      ],
    });

    clickElement(rendered.container.querySelector('button'));
    clickElement(findCommandItem('Label'));

    const swatch = findCommandItem('Escalated')?.querySelector('span[style*="background-color"]') as HTMLElement | null;

    expect(swatch?.style.backgroundColor).toBe('rgb(239, 68, 68)');

    rendered.cleanup();
  });

  it('keeps a filter pill editable after the last value is unselected', () => {
    const onChange = vi.fn();
    const rendered = renderFilters({ onChange, externalFilters: { priority: 'high' } });

    clickElement(Array.from(rendered.container.querySelectorAll('button')).find((button) => button.textContent?.includes('High')));
    clickElement(findCommandItem('High'));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith({});
    expect(rendered.container.textContent).toContain('Priority');
    expect(rendered.container.textContent).toContain('Choose value');
    expect(document.body.textContent).toContain('Medium');

    rendered.cleanup();
  });
});
