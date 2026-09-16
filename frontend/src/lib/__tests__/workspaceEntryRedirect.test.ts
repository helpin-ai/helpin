// @vitest-environment jsdom
import { createMemoryHistory, createRootRoute, createRoute, createRouter } from '@tanstack/react-router';
import { expect, it, vi } from 'vitest';
import { Route as workspaceEntry } from '@/routes/_authenticated/w/$slug/index';
import { Route as myWork } from '@/routes/_authenticated/w/$slug/pm/my-work';

vi.mock('@/pages/pm/MyWork', () => ({ MyWorkPage: () => null }));

it('preserves the shared chat when the workspace entry redirects to My work', async () => {
  const root = createRootRoute();
  const entry = createRoute({
    getParentRoute: () => root,
    path: '/w/$slug/',
    beforeLoad: workspaceEntry.options.beforeLoad as never,
  });
  const destination = createRoute({
    getParentRoute: () => root,
    path: '/w/$slug/pm/my-work',
    validateSearch: myWork.options.validateSearch,
  });
  const router = createRouter({
    routeTree: root.addChildren([entry, destination]),
    history: createMemoryHistory({ initialEntries: ['/w/acme?ask_chat=shared-chat-123'] }),
  });
  await router.load();
  expect(router.state.location.pathname).toBe('/w/acme/pm/my-work');
  expect(router.state.location.search).toMatchObject({ ask_chat: 'shared-chat-123' });
});
