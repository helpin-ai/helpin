import type { User } from '@mobile/lib/types'
import {
  Link,
  Navigate,
  createRootRouteWithContext,
  createRoute,
  createRouter,
  isRedirect,
  lazyRouteComponent,
  redirect,
  useParams,
} from '@tanstack/react-router'
import { AppThemeProvider } from '@mobile/ui/theme-provider'
import { ScreenStack } from '@mobile/navigation/screen-stack'
import { useKeyboardInset } from '@mobile/lib/use-keyboard-inset'
import { useMobileRealtime } from '@mobile/lib/use-mobile-realtime'
import { getLastWorkspaceSlug, setLastWorkspaceSlug } from '@mobile/lib/prefs'
import { queryClient } from '@mobile/lib/queryClient'
import { useAuthStore } from '@mobile/stores/auth-store'
import { useWorkspaceStore } from '@mobile/stores/workspace-store'
import { LoginScreen } from '@mobile/screens/login-screen'
import {
  WorkspacesScreen,
  resolveWorkspaceRedirect,
  workspacesQueryOptions,
} from '@mobile/screens/workspaces-screen'
import { InboxScreen } from '@mobile/screens/inbox-screen'
import { ConversationPending } from '@mobile/screens/conversation-pending'
import { SettingsScreen } from '@mobile/screens/you-screen'
import { SearchScreen } from '@mobile/screens/search-screen'
import { Spinner } from '@mobile/ui/spinner'

export interface RouterContext {
  auth: {
    user: User | null
    loading: boolean
    serverUnreachable: boolean
  }
}

function requireAuth({ context }: { context: RouterContext }) {
  if (!context.auth.loading && !context.auth.user && !context.auth.serverUnreachable) {
    throw redirect({ to: '/login' })
  }
}

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: RootComponent,
  notFoundComponent: NotFoundComponent,
})

export function resolveStartupRoute(
  loading: boolean,
  user: Pick<User, 'id'> | null,
  serverUnreachable: boolean,
): '/login' | '/workspaces' | null {
  if (loading) return null
  return user || serverUnreachable ? '/workspaces' : '/login'
}

function StartupRouteComponent() {
  const user = useAuthStore((state) => state.user)
  const loading = useAuthStore((state) => state.loading)
  const serverUnreachable = useAuthStore((state) => state.serverUnreachable)
  const destination = resolveStartupRoute(loading, user, serverUnreachable)

  if (!destination) {
    return (
      <div className="flex min-h-dvh items-center justify-center">
        <Spinner />
      </div>
    )
  }

  return <Navigate to={destination} replace />
}

const startupRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: StartupRouteComponent,
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginRouteComponent,
})

const workspacesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/workspaces',
  // Runs the auto-redirect decision (single workspace / stored
  // last_workspace_slug) BEFORE the picker component ever mounts, so a
  // redirecting user never sees a spinner or list flash.
  beforeLoad: async (opts) => {
    requireAuth(opts)
    // Only check when we know who's signed in — while auth is still loading
    // (or the server is unreachable) just render the picker, whose query
    // handles its own loading/error states.
    if (!opts.context.auth.user) return
    try {
      const [workspaces, storedSlug] = await Promise.all([
        queryClient.ensureQueryData(workspacesQueryOptions),
        getLastWorkspaceSlug(),
      ])
      const slug = resolveWorkspaceRedirect(workspaces, storedSlug)
      if (slug) {
        void setLastWorkspaceSlug(slug)
        throw redirect({ to: '/w/$slug/support', params: { slug } })
      }
    } catch (error) {
      if (isRedirect(error)) throw error
      // ensureQueryData failed (network etc.) — fall through to the picker,
      // which surfaces the error/retry state instead of bricking the route.
    }
  },
  component: WorkspacesScreen,
})

const supportInboxRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/w/$slug/support',
  beforeLoad: requireAuth,
  component: InboxScreen,
})

const supportSearchRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/w/$slug/support/search',
  beforeLoad: requireAuth,
  component: SearchScreen,
})

const supportConversationRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/w/$slug/support/$conversationId',
  beforeLoad: requireAuth,
  // Lazy-loaded (Task 22 bundle-budget pass): the thread screen pulls in the
  // composer, message list, and context sheet — none of that is needed to
  // paint the Inbox, so keeping it out of the initial chunk shaves a real
  // (if, per QA.md's measurements, not currently budget-critical) amount off
  // cold start. `defaultPreload: 'intent'` (below) still prefetches it the
  // moment a conversation row is pressed/hovered, so there's no added
  // loading flash on the common path.
  component: lazyRouteComponent(() => import('@mobile/screens/conversation-screen'), 'ConversationScreen'),
  // Fallback for a cold chunk load (worst case: a cold-start push-tap deep
  // link, where intent-preload never had a head start). Verified against
  // this router version's Match implementation (react-router/dist/esm/
  // Match.js, MatchView): setting `pendingComponent` is what upgrades the
  // match's boundary from SafeFragment to a real `React.Suspense` with this
  // as the fallback — without it the lazy component's thrown load promise
  // bubbles to the root's null fallback and the ScreenStack slides in a
  // blank panel.
  pendingComponent: ConversationPending,
  // ...and `pendingMs: 0` is ALSO required: the router preloads the lazy
  // component as part of match loading, holding the match in a pending
  // state that only *displays* the pendingComponent after `pendingMs`
  // (default 1000ms) — measured empirically in
  // src/screens/__tests__/conversation-pending.test.tsx, where the skeleton
  // first rendered at ~1032ms without this override (a full second of blank
  // panel on a slow chunk fetch). With 0 it renders immediately. The
  // router's default `pendingMinMs` (500ms) is deliberately kept: once
  // shown, the skeleton stays up briefly instead of flashing for one frame
  // when the chunk lands quickly — and the common warm path (chunk already
  // intent-preloaded from the inbox) never enters pending at all.
  pendingMs: 0,
})

const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/w/$slug/settings',
  beforeLoad: requireAuth,
  component: SettingsScreen,
})

const legacyYouRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/w/$slug/you',
  beforeLoad: (opts) => {
    requireAuth(opts)
    throw redirect({ to: '/w/$slug/settings', params: { slug: opts.params.slug }, replace: true })
  },
})

const routeTree = rootRoute.addChildren([
  startupRoute,
  loginRoute,
  workspacesRoute,
  supportInboxRoute,
  supportSearchRoute,
  supportConversationRoute,
  settingsRoute,
  legacyYouRoute,
])

export const router = createRouter({
  routeTree,
  context: {
    auth: {
      user: null,
      loading: true,
      serverUnreachable: false,
    },
  },
  defaultPreload: 'intent',
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

/**
 * Realtime is mounted here — at the root, above `ScreenStack` — rather than
 * inside `InboxScreen`/`ConversationScreen` or a `/w/$slug` layout route.
 *
 * `ScreenStack` keys its animated wrapper by `location.pathname`
 * (see navigation/screen-stack.tsx), so ANY route component rendered through
 * its `<Outlet/>` — including a hypothetical `/w/$slug` layout route nested
 * under it — fully unmounts and remounts on every inbox <-> thread
 * navigation (that's what drives the push/pop slide transitions). A layout
 * route would therefore NOT actually persist the websocket across
 * navigation; it would just move the reconnect-on-every-nav churn one level
 * up. `RootComponent` is the one place in the tree that survives all of
 * that, since it's rendered directly by `rootRoute`, above `ScreenStack`'s
 * pathname-keyed subtree.
 *
 * `workspaceId` comes from `useWorkspaceStore` (populated by whichever
 * screen resolves the workspace by slug — both InboxScreen and
 * ConversationScreen set it) rather than re-fetching here, per the store's
 * own doc comment anticipating this exact use. `conversationId` is read
 * directly from router state via `useParams({ strict: false })` — the same
 * "router-state read" already used by both screens — so it is non-null only
 * while the conversation route is actually matched, with no extra
 * zustand slice needed. `useMobileRealtime` itself no-ops safely with an
 * empty `workspaceId` (e.g. on /login, /workspaces, before the workspace
 * lookup resolves), so calling it unconditionally here is safe.
 */
function RootRealtimeMount() {
  const { conversationId } = useParams({ strict: false })
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspace?.id ?? '')
  useMobileRealtime(workspaceId, conversationId ?? null)
  return null
}

function RootComponent() {
  useKeyboardInset()
  return (
    <AppThemeProvider>
      <RootRealtimeMount />
      <ScreenStack />
    </AppThemeProvider>
  )
}

/** Mirrors desktop's LoginRouteComponent: an already-signed-in user visiting /login is bounced onward. */
function LoginRouteComponent() {
  const user = useAuthStore((state) => state.user)

  if (user) {
    return <Navigate to="/workspaces" />
  }

  return <LoginScreen />
}

function NotFoundComponent() {
  return (
    <div className="flex min-h-dvh items-center justify-center px-6 text-center">
      <div>
        <h1 className="text-headline">Page not found</h1>
        <p className="mt-2 text-footnote text-muted-foreground">
          The screen you requested does not exist.
        </p>
        <Link to="/workspaces" className="mt-4 inline-block text-primary underline">
          Back to workspaces
        </Link>
      </div>
    </div>
  )
}
