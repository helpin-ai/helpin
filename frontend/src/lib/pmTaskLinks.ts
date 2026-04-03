interface BuildTaskUrlOptions {
  origin: string;
  slug: string;
  taskId: string;
}

interface BuildTaskCopyUrlOptions {
  currentHref: string;
  displayId: number | string;
  taskKey?: string | null;
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
  taskKey,
  origin,
  slug,
  taskId,
}: BuildTaskCopyUrlOptions) {
  if (origin && slug && taskId) {
    return buildTaskUrl({ origin, slug, taskId });
  }

  const url = new URL(currentHref);
  // Prefer task_key format (e.g. "HLP-123") over bare display_id for shareable URLs.
  url.searchParams.set('task', taskKey || String(displayId));
  return url.toString();
}
