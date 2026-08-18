import type { CSSProperties } from 'react';

export const AUTH_TOP_BANNER_HEIGHT_VAR = '--helpin-auth-top-banner-height';
export const WORKSPACE_AUTH_VIEWPORT_CLASS_NAME =
  'h-[calc(100svh-var(--helpin-auth-top-banner-height,0px))] min-h-0';
export const AUTH_SIDEBAR_CONTAINER_CLASS_NAME = 'absolute inset-y-0 h-full';

export function getAuthenticatedLayoutStyle(topBannerHeight: number): CSSProperties {
  return {
    [AUTH_TOP_BANNER_HEIGHT_VAR]: `${Math.max(0, Math.round(topBannerHeight))}px`,
  } as CSSProperties;
}
