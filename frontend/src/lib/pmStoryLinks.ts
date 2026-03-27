interface BuildStoryUrlOptions {
  origin: string;
  slug: string;
  storyId: string;
}

interface BuildStoryCopyUrlOptions {
  currentHref: string;
  displayId: number | string;
  origin?: string;
  slug?: string | null;
  storyId?: string | null;
}

export function buildStoryPath(slug: string, storyId: string) {
  return `/w/${encodeURIComponent(slug)}/pm/stories/${encodeURIComponent(storyId)}`;
}

export function buildStoryUrl({ origin, slug, storyId }: BuildStoryUrlOptions) {
  return new URL(buildStoryPath(slug, storyId), origin).toString();
}

export function buildStoryCopyUrl({
  currentHref,
  displayId,
  origin,
  slug,
  storyId,
}: BuildStoryCopyUrlOptions) {
  if (origin && slug && storyId) {
    return buildStoryUrl({ origin, slug, storyId });
  }

  const url = new URL(currentHref);
  url.searchParams.set('story', String(displayId));
  return url.toString();
}
