// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { describe, expect, it, vi, afterEach, beforeEach } from 'vitest';

import { TaskRelationshipsSection } from '../TaskRelationshipsSection';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const associations = {
  task_relationships: {
    blocked_by: [],
    blocking: [],
    relates_to: [],
    related_by: [],
    duplicates: [],
    duplicated_by: [],
  },
  docs: [],
};

vi.mock('@tanstack/react-router', () => ({
  useLocation: () => ({ pathname: '/w/acme/pm/tasks' }),
  useNavigate: () => vi.fn(),
}));

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: { currentWorkspace: { slug: string } }) => unknown) =>
    selector({ currentWorkspace: { slug: 'acme' } }),
}));

vi.mock('@/components/docs/DocumentPreviewDialog', () => ({
  DocumentPreviewDialog: () => null,
}));

vi.mock('@/hooks/queries', () => ({
  useTaskAssociations: () => ({ data: associations, isLoading: false, error: null }),
  useCreateTaskRelationship: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteTaskRelationship: () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false }),
  useCreateTask: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useCreateDocAssociation: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteDocAssociation: () => ({ mutate: vi.fn(), isPending: false }),
}));

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root.unmount();
  });
  container.remove();
  document.body.innerHTML = '';
});

function renderSection(overrides: Partial<React.ComponentProps<typeof TaskRelationshipsSection>> = {}) {
  const props: React.ComponentProps<typeof TaskRelationshipsSection> = {
    workspaceId: 'ws-1',
    taskId: 'task-1',
    taskName: 'Build search',
    taskDisplayId: 42,
    externalBlocker: '',
    onExternalBlockerChange: vi.fn(),
    composerOpen: false,
    onComposerOpenChange: vi.fn(),
    visible: true,
    ...overrides,
  };

  act(() => {
    root.render(<TaskRelationshipsSection {...props} />);
  });

  return props;
}

describe('TaskRelationshipsSection', () => {
  it('stays hidden until the relationships section is toggled open', () => {
    renderSection({ visible: false });

    expect(container.querySelector('#task-relationships-section')).toBeNull();
  });

  it('uses the inline treatment for the add relationship button', () => {
    renderSection();

    const addButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === 'Add relationship',
    );

    expect(addButton).toBeTruthy();
    expect(addButton?.className).toContain('text-xs');
    expect(addButton?.className).toContain('text-muted-foreground');
    expect(addButton?.className).toContain('hover:text-foreground');
    expect(addButton?.querySelector('svg')?.className.baseVal).not.toContain('text-primary');
  });

  it('opens the composer from the add relationship button', () => {
    const onComposerOpenChange = vi.fn();
    renderSection({ onComposerOpenChange });

    const addButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === 'Add relationship',
    );

    act(() => {
      addButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(onComposerOpenChange).toHaveBeenCalledWith(true);
  });
});
