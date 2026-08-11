// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { describe, expect, it, vi, afterEach, beforeEach } from 'vitest';

import { TaskRelationshipsSection } from '../TaskRelationshipsSection';
import type { GroupedAssociations } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const associations: GroupedAssociations = {
  task_relationships: {
    blocked_by: [],
    blocking: [],
    relates_to: [],
    related_by: [],
    duplicates: [],
    duplicated_by: [],
  },
  tasks: [],
  support_conversations: [],
  crm_records: [],
  docs: [],
};

const openTaskRouteMock = vi.hoisted(() => vi.fn());

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children }: { children: React.ReactNode }) => <a href="#">{children}</a>,
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

vi.mock('@/components/pm/CreateTaskModal', () => ({
  CreateTaskModal: () => null,
}));

vi.mock('@/components/pm/task-detail/taskRouteNavigation', () => ({
  openTaskRoute: openTaskRouteMock,
}));

vi.mock('@/hooks/queries', () => ({
  useTaskAssociations: () => ({ data: associations, isLoading: false, error: null }),
  useWorkflows: () => ({ data: [{ workflow: { id: 'workflow-1', default_state_id: 'state-1' }, states: [{ id: 'state-1' }] }] }),
  useCreateTaskRelationship: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteTaskRelationship: () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false }),
  useCreateTask: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useCreateDocAssociation: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteDocAssociation: () => ({ mutate: vi.fn(), isPending: false }),
}));

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  openTaskRouteMock.mockReset();
  associations.task_relationships.relates_to = [];
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
    workflowId: 'workflow-1',
    workflowStateId: 'state-1',
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

  it('places the add relationship action in the section heading', () => {
    renderSection();

    const addButton = container.querySelector<HTMLButtonElement>(
      'button[aria-label="Add task relationship"]',
    );

    expect(addButton).toBeTruthy();
    expect(addButton?.textContent).toBe('+');
    expect(addButton?.className).toContain('p-0.5');
    expect(addButton?.className).toContain('text-muted-foreground');
  });

  it('opens the relationship modal from the heading action', async () => {
    const onComposerOpenChange = vi.fn();
    renderSection({ onComposerOpenChange });

    const addButton = container.querySelector<HTMLButtonElement>(
      'button[aria-label="Add task relationship"]',
    );

    act(() => {
      addButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(onComposerOpenChange).toHaveBeenCalledWith(true);

    renderSection({ composerOpen: true, onComposerOpenChange });
    await act(async () => {
      await Promise.resolve();
    });

    expect(document.body.querySelector('[role="dialog"]')).toBeTruthy();
    expect(document.body.textContent).toContain('Add relationship');
  });

  it('keeps create related task available before search text is entered', async () => {
    const trigger = document.createElement('button');
    document.body.appendChild(trigger);
    trigger.focus();

    const externalTriggerRef: React.RefObject<HTMLButtonElement> = { current: trigger };
    renderSection({ composerOpen: true, externalTriggerRef });

    await act(async () => {
      await Promise.resolve();
    });

    const createButton = Array.from(document.body.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === 'Create related task',
    );

    expect(createButton).toBeTruthy();
    expect(createButton?.disabled).toBe(false);
  });

  it('uses the relationship icon as the relationship-type menu trigger', async () => {
    associations.task_relationships.relates_to = [
      {
        relationship_id: 'rel-1',
        link_type: 'relates_to',
        is_active: true,
        task: {
          object_type: 'task',
          object_id: 'task-2',
          title: 'Review launch copy',
          task_key: 'MKT-12',
          task_type: 'chore',
        },
      },
    ];
    renderSection({ flat: true, hideDocs: true });

    const relationshipTrigger = container.querySelector<HTMLButtonElement>(
      'button[aria-label="Change relationship: Relates to"]',
    );

    expect(relationshipTrigger).toBeTruthy();
    expect(container.textContent).not.toContain('Relates to');

    await act(async () => {
      relationshipTrigger?.dispatchEvent(
        new MouseEvent('pointerdown', { bubbles: true, button: 0 }),
      );
      await Promise.resolve();
    });

    expect(document.body.textContent).toContain('Update Relationship Type');
  });

  it('shows three compact task rows before revealing the remaining relationships', () => {
    associations.task_relationships.relates_to = Array.from({ length: 4 }, (_, index) => ({
      relationship_id: `rel-${index + 1}`,
      link_type: 'relates_to',
      is_active: true,
      task: {
        object_type: 'task',
        object_id: `task-${index + 2}`,
        title: `Related task ${index + 1}`,
        task_key: `MKT-${index + 1}`,
      },
    }));
    renderSection({ flat: true, hideDocs: true });

    expect(container.querySelectorAll('[data-testid="related-task-row"]')).toHaveLength(3);
    expect(container.textContent).not.toContain('Related task 4');

    const taskId = container.querySelector('[data-testid="related-task-id"]');
    expect(taskId?.className).toContain('absolute');
    expect(taskId?.className).toContain('bg-background');
    expect(taskId?.className).toContain('text-foreground');
    expect(taskId?.className).toContain('group-hover:opacity-100');
    expect(taskId?.className).toContain('opacity-0');

    const moreButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === 'Show 1 more',
    );
    expect(moreButton).toBeTruthy();

    act(() => {
      moreButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(container.querySelectorAll('[data-testid="related-task-row"]')).toHaveLength(4);
    expect(container.textContent).toContain('Related task 4');
    expect(container.textContent).toContain('Show less');
  });

  it('opens a related task in the existing task panel instead of using a page link', () => {
    associations.task_relationships.relates_to = [
      {
        relationship_id: 'rel-1',
        link_type: 'relates_to',
        is_active: true,
        task: {
          object_type: 'task',
          object_id: 'task-2',
          title: 'Review launch copy',
          task_key: 'MKT-12',
        },
      },
    ];
    renderSection({ flat: true, hideDocs: true });

    const taskButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === 'Review launch copy',
    );
    expect(taskButton).toBeTruthy();

    act(() => {
      taskButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(openTaskRouteMock).toHaveBeenCalledWith(
      expect.anything(),
      expect.anything(),
      'acme',
      'task-2',
    );
  });
});
