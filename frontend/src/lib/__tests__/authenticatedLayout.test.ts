import { describe, expect, it } from 'vitest';

import {
  AUTH_SIDEBAR_CONTAINER_CLASS_NAME,
  AUTH_TOP_BANNER_HEIGHT_VAR,
  WORKSPACE_AUTH_VIEWPORT_CLASS_NAME,
  getAuthenticatedLayoutStyle,
} from '../authenticatedLayout';

describe('authenticatedLayout', () => {
  it('stores the measured top banner height in a shared CSS variable', () => {
    expect(getAuthenticatedLayoutStyle(42)).toEqual({
      [AUTH_TOP_BANNER_HEIGHT_VAR]: '42px',
    });
  });

  it('sizes workspace shells to the remaining viewport height below the top banner', () => {
    expect(WORKSPACE_AUTH_VIEWPORT_CLASS_NAME).toContain('calc(100svh-var(--helpin-auth-top-banner-height,0px))');
  });

  it('keeps desktop sidebars inside the workspace shell height', () => {
    expect(AUTH_SIDEBAR_CONTAINER_CLASS_NAME).toContain('absolute');
    expect(AUTH_SIDEBAR_CONTAINER_CLASS_NAME).toContain('inset-y-0');
    expect(AUTH_SIDEBAR_CONTAINER_CLASS_NAME).toContain('h-full');
    expect(AUTH_SIDEBAR_CONTAINER_CLASS_NAME).not.toContain('fixed');
  });
});
