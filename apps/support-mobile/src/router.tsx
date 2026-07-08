import type { User } from '@mobile/lib/types'
import {
  Link,
  Navigate,
  createRootRouteWithContext,
  createRoute,
  createRouter,
  redirect,
} from '@tanstack/react-router'
import { AppThemeProvider } from '@mobile/ui/theme-provider'
import { ScreenStack } from '@mobile/navigation/screen-stack'
import { useKeyboardInset } from '@mobile/lib/use-keyboard-inset'
import { useAuthStore } from '@mobile/stores/auth-store'
import { LoginScreen } from '@mobile/screens/login-screen'
import { WorkspacesScreen } from '@mobile/screens/workspaces-screen'
import { InboxScreen } from '@mobile/screens/inbox-screen'
import { ConversationScreen } from '@mobile/screens/conversation-screen'
import { YouScreen } from '@mobile/screens/you-screen'

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

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginRouteComponent,
})

const workspacesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/workspaces',
  beforeLoad: requireAuth,
  component: WorkspacesScreen,
})

const supportInboxRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/w/$slug/support',
  beforeLoad: requireAuth,
  component: InboxScreen,
})

const supportConversationRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/w/$slug/support/$conversationId',
  beforeLoad: requireAuth,
  component: ConversationScreen,
})

const youRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/w/$slug/you',
  beforeLoad: requireAuth,
  component: YouScreen,
})

const routeTree = rootRoute.addChildren([
  loginRoute,
  workspacesRoute,
  supportInboxRoute,
  supportConversationRoute,
  youRoute,
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

function RootComponent() {
  useKeyboardInset()
  return (
    <AppThemeProvider>
      <ScreenStack />
      {/* TabBar rendered inside ScreenStack chrome for tab-level routes only */}
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
