interface BuildTaskUrlOptions {
  origin: string;
  slug: string;
  taskId: string;
}

interface BuildTaskCopyUrlOptions {
  currentHref: string;
  displayId: number | string;
  origin?: string;
  slug?: string | null;
  taskId?: string | null;
}

export function buildTaskPath(slug: string, taskId: string) {
  return `/w/${encodeURIComponent(slug)}/pm/tasks/${encodeURIComponent(taskId)}`;
}

export function buildTaskUrl({ origin, slug, taskId }: BuildTaskUrlOptions) {
  return new URL(buildTaskPath(slug, taskId), origin).toString();
}

export function buildTaskCopyUrl({
  currentHref,
  displayId,
  origin,
  slug,
  taskId,
}: BuildTaskCopyUrlOptions) {
  if (origin && slug && taskId) {
    return buildTaskUrl({ origin, slug, taskId });
  }

  const url = new URL(currentHref);
  url.searchParams.set('task', String(displayId));
  return url.toString();
}
