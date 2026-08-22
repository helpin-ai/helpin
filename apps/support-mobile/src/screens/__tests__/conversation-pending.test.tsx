import { render, screen, waitFor } from '@testing-library/react'
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  lazyRouteComponent,
} from '@tanstack/react-router'
import { ConversationPending } from '../conversation-pending'

test('renders the TopBar-shaped header and three skeleton bubbles', () => {
  render(<ConversationPending />)
  const pending = screen.getByTestId('conversation-pending')
  expect(pending).toBeDefined()
  // 1 title-bar skeleton + 3 message bubbles = 4 shimmer blocks.
  expect(pending.querySelectorAll('.bg-muted').length).toBe(4)
})

/**
 * The mechanism test for the Task 22 code-split fix: a route whose
 * `component` is a `lazyRouteComponent` with a chunk that NEVER resolves
 * must render its `pendingComponent` as the Suspense fallback — not a blank
 * panel. This drives the exact code path from the app's real
 * `supportConversationRoute` (src/router.tsx) through a minimal in-test
 * router: per this router version's Match implementation, it is the
 * PRESENCE of `pendingComponent` that upgrades the match's boundary from
 * SafeFragment to a real `React.Suspense`, so this test fails (blank
 * output, findByTestId times out) if `pendingComponent` is ever dropped
 * from the route again.
 */
test('pendingComponent shows promptly while a lazy route component chunk is still loading', async () => {
  const rootRoute = createRootRoute()
  const conversationRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    // A chunk load that never settles — the worst-case cold network.
    component: lazyRouteComponent(
      () => new Promise<{ default: () => null }>(() => {}),
    ),
    pendingComponent: ConversationPending,
    // Mirrors the real route (src/router.tsx). Without this, the skeleton
    // only appeared after the router's defaultPendingMs (1000ms) —
    // empirically measured in this very test before the fix (1032ms to
    // first render) — i.e. a full second of blank panel on a cold chunk.
    pendingMs: 0,
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([conversationRoute]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })

  render(<RouterProvider router={router} />)

  // The tight timeout is the regression guard for `pendingMs: 0`: with the
  // router's 1000ms default delay this findBy times out; with 0 the skeleton
  // renders on the first few waitFor polls.
  await waitFor(() => expect(screen.getByTestId('conversation-pending')).toBeDefined(), { timeout: 500 })
})
