// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { createMemoryHistory, createRootRoute, createRoute, createRouter, RouterProvider } from '@tanstack/react-router';
import { expect, it, vi } from 'vitest';
import { Route as workspaceEntry } from '@/routes/_authenticated/w/$slug/index';

const access = vi.hoisted(() => ({ modules: ['pm'], isLoading: false }));
vi.mock('@/hooks/queries/useSession', () => ({ useWorkspaceAccess: () => ({ data: { modules: access.modules }, isLoading: access.isLoading }) }));
vi.mock('@/stores/workspaceStore', () => ({ useWorkspaceStore: () => ({ id: 'ws-1' }) }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

it.each([
  { modules: ['pm', 'support'], destination: '/w/acme/pm/my-work' },
  { modules: ['support', 'docs', 'agents'], destination: '/w/acme/support' },
])('preserves shared-chat links when the enabled modules select $destination', async ({ modules, destination }) => {
  access.modules = modules;
  const params = vi.spyOn(workspaceEntry, 'useParams').mockReturnValue({ slug: 'acme' } as never);
  const rootRoute = createRootRoute();
  const entry = createRoute({
    getParentRoute: () => rootRoute,
    path: '/w/$slug/',
    component: workspaceEntry.options.component,
  });
  const target = createRoute({
    getParentRoute: () => rootRoute,
    path: destination.replace('/acme/', '/$slug/'),
    validateSearch: (search: Record<string, unknown>) => search,
    component: () => <p>Destination</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([entry, target]),
    history: createMemoryHistory({ initialEntries: ['/w/acme?ask_chat=shared-chat-123'] }),
  });
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  try {
    await act(async () => {
      root.render(<RouterProvider router={router} />);
      await router.load();
    });
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 0)); });
    expect(router.state.location.pathname).toBe(destination);
    expect(router.state.location.search).toMatchObject({ ask_chat: 'shared-chat-123' });
  } finally {
    await act(async () => root.unmount());
    container.remove();
    params.mockRestore();
  }
});
