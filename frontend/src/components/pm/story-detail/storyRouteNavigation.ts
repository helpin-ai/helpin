import { useStoryPanelStore } from '@/stores/storyPanelStore';

export interface StoryOverlayLocationLike {
  pathname: string;
  href?: string;
}

export interface StoryRouteMatch {
  slug: string;
  storyId: string;
}

export type StoryRouteNavigate = (options: Record<string, unknown>) => unknown;

const STORY_PATH_RE = /^\/w\/([^/]+)\/pm\/stories\/([^/]+)\/?$/;

export function matchStoryRoute(pathname: string): Omit<StoryRouteMatch, 'direct'> | null {
  const match = pathname.match(STORY_PATH_RE);
  if (!match) return null;
  return {
    slug: decodeURIComponent(match[1]),
    storyId: decodeURIComponent(match[2]),
  };
}

export function getActiveStoryRoute(location: StoryOverlayLocationLike): StoryRouteMatch | null {
  return matchStoryRoute(location.pathname);
}

export function openStoryRoute(
  navigate: StoryRouteNavigate,
  location: StoryOverlayLocationLike,
  slug: string,
  storyId: string,
) {
  if (useStoryPanelStore.getState().shouldSuppressOpen(storyId)) {
    return undefined;
  }

  if (matchStoryRoute(location.pathname)) {
    return navigate({
      to: '/w/$slug/pm/stories/$storyId',
      params: { slug, storyId },
    });
  }

  useStoryPanelStore.getState().openStory(storyId);
  return undefined;
}

export function getStoryOverlayCloseBehavior(location: StoryOverlayLocationLike, slug: string) {
  const active = getActiveStoryRoute(location);
  if (active) {
    return {
      type: 'fallback-route' as const,
      to: '/w/$slug/pm/stories/' as const,
      params: { slug },
    };
  }

  return {
    type: 'close-panel' as const,
  };
}

export function closeStoryRoute(
  navigate: StoryRouteNavigate,
  location: StoryOverlayLocationLike,
  slug: string,
) {
  const active = getActiveStoryRoute(location);
  if (active?.storyId) {
    useStoryPanelStore.getState().rememberClosedStory(active.storyId);
  } else {
    const contextualStoryId = useStoryPanelStore.getState().storyId;
    if (contextualStoryId) {
      useStoryPanelStore.getState().rememberClosedStory(contextualStoryId);
    }
  }

  const behavior = getStoryOverlayCloseBehavior(location, slug);
  if (behavior.type === 'close-panel') {
    useStoryPanelStore.getState().close();
    return undefined;
  }

  return navigate({
    to: behavior.to,
    params: behavior.params,
  });
}
