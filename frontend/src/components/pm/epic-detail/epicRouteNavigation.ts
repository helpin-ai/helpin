import { useEpicPanelStore } from '@/stores/epicPanelStore';

export interface EpicOverlayLocationLike {
  pathname: string;
}

export interface EpicRouteMatch {
  slug: string;
  epicId: string;
}

export type EpicRouteNavigate = (options: Record<string, unknown>) => unknown;

const EPIC_PATH_RE = /^\/w\/([^/]+)\/pm\/epics\/([^/]+)\/?$/;

export function matchEpicRoute(pathname: string): EpicRouteMatch | null {
  const match = pathname.match(EPIC_PATH_RE);
  if (!match) return null;
  return {
    slug: decodeURIComponent(match[1]),
    epicId: decodeURIComponent(match[2]),
  };
}

export function getActiveEpicRoute(location: EpicOverlayLocationLike): EpicRouteMatch | null {
  return matchEpicRoute(location.pathname);
}

export function openEpicRoute(
  navigate: EpicRouteNavigate,
  _location: EpicOverlayLocationLike,
  slug: string,
  epicId: string,
) {
  if (useEpicPanelStore.getState().shouldSuppressOpen(epicId)) {
    return undefined;
  }

  return navigate({
    to: '/w/$slug/pm/epics/$epicId',
    params: { slug, epicId },
  });
}

export function closeEpicRoute(
  navigate: EpicRouteNavigate,
  location: EpicOverlayLocationLike,
  slug: string,
) {
  const active = getActiveEpicRoute(location);
  if (active?.epicId) {
    useEpicPanelStore.getState().rememberClosedEpic(active.epicId);
    return navigate({
      to: '/w/$slug/pm/epics/',
      params: { slug },
    });
  }

  const contextualEpicId = useEpicPanelStore.getState().epicId;
  if (contextualEpicId) {
    useEpicPanelStore.getState().rememberClosedEpic(contextualEpicId);
  }
  useEpicPanelStore.getState().close();
  return undefined;
}
