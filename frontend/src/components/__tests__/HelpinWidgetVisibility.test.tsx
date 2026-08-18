// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { HelpinWidgetVisibility, isSupportModulePath } from '../HelpinWidgetVisibility';

const mocks = vi.hoisted(() => ({
  pathname: '/w/acme/support',
  hide: vi.fn(),
  show: vi.fn(),
}));

vi.mock('@tanstack/react-router', () => ({
  useLocation: () => ({ pathname: mocks.pathname }),
}));

vi.mock('@helpin-ai/react', () => ({
  useHelpin: () => ({ hide: mocks.hide, show: mocks.show }),
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('HelpinWidgetVisibility', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    vi.clearAllMocks();
  });

  it('recognizes the full support module without matching similarly named routes', () => {
    expect(isSupportModulePath('/w/acme/support')).toBe(true);
    expect(isSupportModulePath('/w/acme/support/inbox/conversation-1')).toBe(true);
    expect(isSupportModulePath('/w/acme/support/search')).toBe(true);
    expect(isSupportModulePath('/w/acme/support-settings')).toBe(false);
    expect(isSupportModulePath('/w/acme/docs')).toBe(false);
  });

  it('hides the widget in support and restores it after navigating elsewhere', () => {
    mocks.pathname = '/w/acme/support/inbox/conversation-1';
    act(() => root.render(<HelpinWidgetVisibility />));
    expect(mocks.hide).toHaveBeenCalledOnce();
    expect(mocks.show).not.toHaveBeenCalled();

    mocks.pathname = '/w/acme/docs';
    act(() => root.render(<HelpinWidgetVisibility />));
    expect(mocks.show).toHaveBeenCalledOnce();
  });
});
