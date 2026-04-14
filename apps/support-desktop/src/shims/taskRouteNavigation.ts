export interface TaskOverlayLocationLike {
  pathname: string
  href?: string
}

export interface TaskRouteMatch {
  slug: string
  taskId: string
}

export type TaskRouteNavigate = (options: Record<string, unknown>) => unknown

const TASK_PATH_RE = /^\/w\/([^/]+)\/pm\/tasks\/([^/]+)\/?$/

export function matchTaskRoute(pathname: string): TaskRouteMatch | null {
  const match = pathname.match(TASK_PATH_RE)
  if (!match) {
    return null
  }

  return {
    slug: decodeURIComponent(match[1]),
    taskId: decodeURIComponent(match[2]),
  }
}

export function getActiveTaskRoute(location: TaskOverlayLocationLike): TaskRouteMatch | null {
  return matchTaskRoute(location.pathname)
}

export function openTaskRoute(
  navigate: TaskRouteNavigate,
  _location: TaskOverlayLocationLike,
  slug: string,
  taskId: string,
) {
  return navigate({
    to: '/w/$slug/pm/tasks/$taskId',
    params: { slug, taskId },
  })
}

export function getTaskOverlayCloseBehavior(_location: TaskOverlayLocationLike, slug: string) {
  return {
    type: 'fallback-route' as const,
    to: '/w/$slug/support' as const,
    params: { slug },
  }
}

export function closeTaskRoute(
  navigate: TaskRouteNavigate,
  _location: TaskOverlayLocationLike,
  slug: string,
) {
  return navigate({
    to: '/w/$slug/support',
    params: { slug },
  })
}
