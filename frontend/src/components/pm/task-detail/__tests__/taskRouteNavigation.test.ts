import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  closeTaskRoute,
  getActiveTaskRoute,
  getTaskOverlayCloseBehavior,
  matchTaskRoute,
  openTaskRoute,
} from '../taskRouteNavigation';
import { useTaskPanelStore } from '@/stores/taskPanelStore';

describe('taskRouteNavigation', () => {
  beforeEach(() => {
    useTaskPanelStore.setState({
      taskId: null,
      requestKey: 0,
      lastClosedTaskId: null,
      lastClosedAt: 0,
    });
  });

  it('matches canonical task routes', () => {
    expect(matchTaskRoute('/w/test-docs/pm/tasks/task-123')).toEqual({
      slug: 'test-docs',
      taskId: 'task-123',
    });
  });

  it('treats direct canonical task routes as active route entries', () => {
    expect(
      getActiveTaskRoute({
        pathname: '/w/test-docs/pm/tasks/task-123',
      }),
    ).toEqual({
      slug: 'test-docs',
      taskId: 'task-123',
    });
  });

  it('treats non-task routes as contextual overlay entries', () => {
    expect(
      getTaskOverlayCloseBehavior({
        pathname: '/w/test-docs/pm/sprints',
      }, 'test-docs'),
    ).toEqual({
      type: 'close-panel',
    });
  });

  it('closes direct entries to the tasks index fallback route', () => {
    expect(
      getTaskOverlayCloseBehavior({
        pathname: '/w/test-docs/pm/tasks/task-123',
      }, 'test-docs'),
    ).toEqual({
      type: 'fallback-route',
      to: '/w/$slug/pm/tasks/',
      params: { slug: 'test-docs' },
    });
  });

  it('opens contextual tasks through the overlay store', () => {
    const navigate = vi.fn();

    openTaskRoute(
      navigate,
      {
        href: 'http://localhost:5173/w/test-docs/pm/sprints?team=abc',
        pathname: '/w/test-docs/pm/sprints',
      },
      'test-docs',
      'task-123',
    );

    expect(navigate).not.toHaveBeenCalled();
    expect(useTaskPanelStore.getState().taskId).toBe('task-123');
  });

  it('navigates directly when already on a canonical task route', () => {
    const navigate = vi.fn();

    openTaskRoute(
      navigate,
      {
        pathname: '/w/test-docs/pm/tasks/task-111',
      },
      'test-docs',
      'task-123',
    );

    expect(navigate).toHaveBeenCalledWith({
      to: '/w/$slug/pm/tasks/$taskId',
      params: {
        slug: 'test-docs',
        taskId: 'task-123',
      },
    });
  });

  it('closes direct entries by navigating to the tasks index fallback route', () => {
    const navigate = vi.fn();

    closeTaskRoute(
      navigate,
      {
        pathname: '/w/test-docs/pm/tasks/task-123',
      },
      'test-docs',
    );

    expect(navigate).toHaveBeenCalledWith({
      to: '/w/$slug/pm/tasks/',
      params: { slug: 'test-docs' },
    });
  });

  it('closes contextual overlays through the overlay store', () => {
    const navigate = vi.fn();
    useTaskPanelStore.setState({ taskId: 'task-123', requestKey: 1 });

    closeTaskRoute(
      navigate,
      {
        pathname: '/w/test-docs/pm/sprints',
      },
      'test-docs',
    );

    expect(navigate).not.toHaveBeenCalled();
    expect(useTaskPanelStore.getState().taskId).toBeNull();
  });

  it('does not immediately reopen the same contextual task right after close', () => {
    const navigate = vi.fn();
    useTaskPanelStore.setState({ taskId: 'task-123', requestKey: 1 });

    closeTaskRoute(
      navigate,
      {
        pathname: '/w/test-docs/pm/sprints',
      },
      'test-docs',
    );

    openTaskRoute(
      navigate,
      {
        pathname: '/w/test-docs/pm/sprints',
      },
      'test-docs',
      'task-123',
    );

    expect(useTaskPanelStore.getState().taskId).toBeNull();
  });

  it('still allows opening a different contextual task after close', () => {
    const navigate = vi.fn();
    useTaskPanelStore.setState({ taskId: 'task-123', requestKey: 1 });

    closeTaskRoute(
      navigate,
      {
        pathname: '/w/test-docs/pm/sprints',
      },
      'test-docs',
    );

    openTaskRoute(
      navigate,
      {
        pathname: '/w/test-docs/pm/sprints',
      },
      'test-docs',
      'task-456',
    );

    expect(useTaskPanelStore.getState().taskId).toBe('task-456');
  });
});
