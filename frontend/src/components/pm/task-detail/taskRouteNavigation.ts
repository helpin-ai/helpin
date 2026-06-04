import { clearTaskSearchParam, useTaskPanelStore } from '@/stores/taskPanelStore';

export interface TaskOverlayLocationLike {
  pathname: string;
  href?: string;
}

export interface TaskRouteMatch {
  slug: string;
  taskId: string;
}

export type TaskRouteNavigate = (options: Record<string, unknown>) => unknown;

const TASK_PATH_RE = /^\/w\/([^/]+)\/pm\/tasks\/([^/]+)\/?$/;

export function matchTaskRoute(pathname: string): TaskRouteMatch | null {
  const match = pathname.match(TASK_PATH_RE);
  if (!match) return null;
  return {
    slug: decodeURIComponent(match[1]),
    taskId: decodeURIComponent(match[2]),
  };
}

export function getActiveTaskRoute(location: TaskOverlayLocationLike): TaskRouteMatch | null {
  return matchTaskRoute(location.pathname);
}

export interface OpenTaskRouteOptions {
  run?: string;
  team?: string;
}

export function openTaskRoute(
  navigate: TaskRouteNavigate,
  location: TaskOverlayLocationLike,
  slug: string,
  taskId: string,
  opts?: OpenTaskRouteOptions,
) {
  if (useTaskPanelStore.getState().shouldSuppressOpen(taskId)) {
    return undefined;
  }

  const search: Record<string, string> = {};
  if (opts?.run) search.run = opts.run;
  if (opts?.team) search.team = opts.team;

  // When a specific run is requested, force the route form so AgentRunPanel
  // (which reads the `run` param via useSearch) can open the drawer.
  if (opts?.run || matchTaskRoute(location.pathname)) {
    return navigate({
      to: '/w/$slug/pm/tasks/$taskId',
      params: { slug, taskId },
      search: Object.keys(search).length > 0 ? search : undefined,
    });
  }

  useTaskPanelStore.getState().openTask(taskId);
  return undefined;
}

export function getTaskOverlayCloseBehavior(location: TaskOverlayLocationLike, slug: string) {
  const active = getActiveTaskRoute(location);
  if (active) {
    return {
      type: 'fallback-route' as const,
      to: '/w/$slug/pm/tasks/' as const,
      params: { slug },
    };
  }

  return {
    type: 'close-panel' as const,
  };
}

export function closeTaskRoute(
  navigate: TaskRouteNavigate,
  location: TaskOverlayLocationLike,
  slug: string,
) {
  const active = getActiveTaskRoute(location);
  if (active?.taskId) {
    useTaskPanelStore.getState().rememberClosedTask(active.taskId);
  } else {
    const contextualTaskId = useTaskPanelStore.getState().taskId;
    if (contextualTaskId) {
      useTaskPanelStore.getState().rememberClosedTask(contextualTaskId);
    }
  }

  const behavior = getTaskOverlayCloseBehavior(location, slug);
  if (behavior.type === 'close-panel') {
    useTaskPanelStore.getState().close();
    return undefined;
  }

  clearTaskSearchParam();
  return navigate({
    to: behavior.to,
    params: behavior.params,
    search: (prev: Record<string, unknown>) => {
      const next = { ...prev };
      delete next.task;
      delete next.run;
      return next;
    },
  });
}
