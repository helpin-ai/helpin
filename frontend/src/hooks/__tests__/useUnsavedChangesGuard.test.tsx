// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { createBrowserHistory, createRootRoute, createRoute, createRouter, Outlet, RouterProvider } from '@tanstack/react-router';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { prepareAppRefresh } from '@/lib/appLoadRecovery';
import { useUnsavedChangesGuard } from '../useUnsavedChangesGuard';

const { confirm } = vi.hoisted(() => ({ confirm: vi.fn() }));
vi.mock('@/components/ui/confirm-dialog', () => ({ useConfirm: () => confirm }));

let root: Root;
let container: HTMLDivElement;
let dirty = false;
let history: ReturnType<typeof createBrowserHistory>;
const reason = 'Save your document before refreshing.';
beforeEach(() => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
  confirm.mockReset().mockResolvedValue(false);
  dirty = false;
  window.history.replaceState(null, '', '/');
  history = createBrowserHistory();
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  history.destroy();
  container.remove();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

async function mount() {
  function Editor() {
    useUnsavedChangesGuard(() => dirty, reason);
    return <textarea defaultValue="Unfinished work" />;
  }
  const rootRoute = createRootRoute({ component: Outlet });
  const loadDestination = vi.fn();
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      createRoute({ getParentRoute: () => rootRoute, path: '/', component: Editor }),
      createRoute({ getParentRoute: () => rootRoute, path: '/next', loader: loadDestination, component: () => <p>Next page</p> }),
    ]),
    history,
  });
  await act(async () => {
    await router.load();
    root.render(<RouterProvider router={router} />);
  });
  return { router, loadDestination };
}

it('blocks release refresh and browser refresh while work is unsaved, then allows them after saving', async () => {
  await mount();
  expect(prepareAppRefresh()).toBeNull();
  dirty = true;
  expect(prepareAppRefresh()).toBe(reason);
  const pending = new Event('beforeunload', { cancelable: true });
  window.dispatchEvent(pending);
  expect(pending.defaultPrevented).toBe(true);
  dirty = false;
  expect(prepareAppRefresh()).toBeNull();
  const saved = new Event('beforeunload', { cancelable: true });
  window.dispatchEvent(saved);
  expect(saved.defaultPrevented).toBe(false);
});

it('keeps the editor mounted and does not start loading a destination when leaving is cancelled', async () => {
  const { router, loadDestination } = await mount();
  dirty = true;
  const editor = container.querySelector('textarea');
  await act(async () => { void router.navigate({ to: '/next' }); await new Promise(resolve => setTimeout(resolve, 25)); });
  expect(container.querySelector('textarea')).toBe(editor);
  expect(editor?.value).toBe('Unfinished work');
  expect(loadDestination).not.toHaveBeenCalled();
  expect(confirm).toHaveBeenCalledWith(expect.objectContaining({ cancelText: 'Stay here', confirmText: 'Leave without saving' }));
  expect(prepareAppRefresh()).toBe(reason);
});

it('allows navigation after saving without prompting', async () => {
  const { router } = await mount();
  dirty = true;
  expect(prepareAppRefresh()).toBe(reason);
  dirty = false;
  await act(async () => { void router.navigate({ to: '/next' }); await new Promise(resolve => setTimeout(resolve, 25)); });
  expect(container.textContent).toContain('Next page');
  expect(confirm).not.toHaveBeenCalled();
  expect(prepareAppRefresh()).toBeNull();
});

it('allows an explicit discard and unregisters the refresh guard on leaving', async () => {
  const { router } = await mount();
  dirty = true;
  confirm.mockResolvedValue(true);
  await act(async () => { void router.navigate({ to: '/next' }); await new Promise(resolve => setTimeout(resolve, 25)); });
  expect(container.textContent).toContain('Next page');
  expect(prepareAppRefresh()).toBeNull();
});
