import { useDealPanelStore } from '@/stores/dealPanelStore';

export interface DealOverlayLocationLike {
  pathname: string;
}

export interface DealRouteMatch {
  slug: string;
  dealId: string;
}

export type DealRouteNavigate = (options: Record<string, unknown>) => unknown;

const DEAL_PATH_RE = /^\/w\/([^/]+)\/crm\/deals\/([^/]+)\/?$/;

export function matchDealRoute(pathname: string): DealRouteMatch | null {
  const match = pathname.match(DEAL_PATH_RE);
  if (!match) return null;
  return { slug: decodeURIComponent(match[1]), dealId: decodeURIComponent(match[2]) };
}

export function getActiveDealRoute(location: DealOverlayLocationLike): DealRouteMatch | null {
  return matchDealRoute(location.pathname);
}

export function openDealRoute(
  navigate: DealRouteNavigate,
  location: DealOverlayLocationLike,
  slug: string,
  dealId: string,
) {
  if (useDealPanelStore.getState().shouldSuppressOpen(dealId)) return undefined;
  if (matchDealRoute(location.pathname)) {
    return navigate({
      to: '/w/$slug/crm/deals/$dealId',
      params: { slug, dealId },
    });
  }
  useDealPanelStore.getState().openDeal(dealId);
  return undefined;
}

export function closeDealRoute(
  navigate: DealRouteNavigate,
  location: DealOverlayLocationLike,
  slug: string,
) {
  const activeRoute = getActiveDealRoute(location);
  const contextualDealId = useDealPanelStore.getState().dealId;
  const closingDealId = activeRoute?.dealId ?? contextualDealId;
  if (closingDealId) useDealPanelStore.getState().rememberClosedDeal(closingDealId);

  if (!activeRoute) {
    useDealPanelStore.getState().close();
    return undefined;
  }

  return navigate({
    to: '/w/$slug/crm/deals/',
    params: { slug },
  });
}
