// @vitest-environment jsdom
import { act, lazy, Suspense } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, expect, it, vi } from 'vitest';
import { AppLoadBoundary, AppLoadError } from '../AppLoadError';
import { lazyRouteComponent } from '@tanstack/react-router';
import { installAppLoadRecovery } from '@/lib/appLoadRecovery';
import { useUnsavedChangesGuard } from '@/hooks/useUnsavedChangesGuard';

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); document.body.innerHTML = ''; });

it('refuses the release Refresh action while an editor has unsaved work', async () => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
  document.head.innerHTML = '<script type="module" src="/assets/index-old.js"></script>';
  vi.stubGlobal('fetch', vi.fn().mockImplementation(async () => new Response('<div id="root"></div><script type="module" src="/assets/index-new.js"></script>', { headers: { 'Content-Type': 'text/html' } })));
  function Draft() {
    useUnsavedChangesGuard(true, 'Save your document before refreshing.');
    return <textarea defaultValue="Unfinished document" />;
  }
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  await act(async () => root.render(<><Draft /><AppLoadError error={new TypeError('Failed to fetch dynamically imported module: /assets/old.js')} reset={() => {}} /></>));
  const editor = container.querySelector('textarea');
  await act(async () => container.querySelector('button')?.click());
  expect(container.textContent).toContain('Save your document before refreshing.');
  expect(container.querySelector('textarea')).toBe(editor);
  expect(editor?.value).toBe('Unfinished document');
  await act(async () => root.unmount());
});

it('contains a failed sidebar, preserves a sibling draft, and offers a refresh for a confirmed update', async () => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
  vi.spyOn(console, 'error').mockImplementation(() => {});
  document.head.innerHTML = '<script type="module" src="/assets/index-old.js"></script>';
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('<div id="root"></div><script type="module" src="/assets/index-new.js"></script>', { headers: { 'Content-Type': 'text/html' } })));
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  let fail!: (reason: Error) => void;
  const Sidebar = lazy(() => new Promise<{ default: () => null }>((_, reject) => { fail = reject; }));
  await act(async () => root.render(<><textarea defaultValue="Unsent reply" /><AppLoadBoundary><Suspense fallback="Loading"><Sidebar /></Suspense></AppLoadBoundary></>));
  const draft = container.querySelector('textarea');
  await act(async () => fail(new TypeError('Failed to fetch dynamically imported module: /assets/old.js')));
  expect(container.textContent).toContain('Helpin has been updated');
  expect(container.querySelector('textarea')).toBe(draft);
  expect(draft?.value).toBe('Unsent reply');
  expect([...container.querySelectorAll('button')].some(button => button.textContent === 'Refresh')).toBe(true);
  await act(async () => root.unmount());
});

it('also gives TanStack lazy routes a user-controlled recovery instead of its automatic reload', async () => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
  vi.spyOn(console, 'error').mockImplementation(() => {});
  document.head.innerHTML = '<script type="module" src="/assets/index-old.js"></script>';
  vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')));
  sessionStorage.clear();
  const cleanup = installAppLoadRecovery();
  const Route = lazyRouteComponent(async () => {
    const error = new TypeError('Failed to fetch dynamically imported module: /assets/route-old.js');
    window.dispatchEvent(Object.assign(new Event('vite:preloadError', { cancelable: true }), { payload: error }));
    throw error;
  });
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  await act(async () => root.render(<AppLoadBoundary><Suspense fallback="Loading"><Route /></Suspense></AppLoadBoundary>));
  expect(container.textContent).toContain('This part of Helpin couldn’t load');
  expect([...container.querySelectorAll('button')].some(button => button.textContent === 'Retry')).toBe(true);
  await act(async () => root.unmount());
  cleanup();
});
