import {
  Link,
  Navigate,
  Outlet,
  createRootRouteWithContext,
  createRoute,
  createRouter,
  redirect,
} from '@tanstack/react-router'
import { AppLayout } from '@/components/app-layout'
import { ChatPlaygroundPage } from '@/features/chat-playground/chat-playground-page'
import { EmailDiagnosticsPage } from '@/features/email-diagnostics/email-diagnostics-page'
import { WebhookEventsPage } from '@/features/webhook-events/webhook-events-page'
import { EmailQueuePage } from '@/features/email-queue/email-queue-page'
import { Button } from '@/components/ui/button'
import { LoginPage } from '@/pages/login-page'
import { useAuthStore } from '@/stores/authStore'
import type { User } from '@/lib/types'

export interface RouterContext {
  auth: {
    user: User | null
    loading: boolean
    serverUnreachable: boolean
  }
}

function clearLocalAuth() {
  localStorage.removeItem('access_token')
  localStorage.removeItem('refresh_token')
  localStorage.removeItem('remember_me')
}

function requirePlatformAdmin({ context }: { context: RouterContext }) {
  if (!context.auth.loading && !context.auth.user && !context.auth.serverUnreachable) {
    throw redirect({ to: '/login' })
  }
  if (
    !context.auth.loading &&
    context.auth.user &&
    (!context.auth.user.is_platform_admin || !context.auth.user.mfa_satisfied_in_token)
  ) {
    clearLocalAuth()
    throw redirect({ to: '/forbidden' })
  }
}

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: RootComponent,
  notFoundComponent: NotFoundComponent,
})

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: IndexPage,
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginRouteComponent,
})

const forbiddenRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/forbidden',
  component: ForbiddenPage,
})

const authenticatedLayout = createRoute({
  getParentRoute: () => rootRoute,
  id: 'authenticated',
  beforeLoad: requirePlatformAdmin,
  component: AppLayout,
})

const chatPlaygroundRoute = createRoute({
  getParentRoute: () => authenticatedLayout,
  path: '/chat-playground',
  component: ChatPlaygroundPage,
})

const webhookEventsRoute = createRoute({
  getParentRoute: () => authenticatedLayout,
  path: '/webhook-events',
  component: WebhookEventsPage,
})

const emailQueueRoute = createRoute({
  getParentRoute: () => authenticatedLayout,
  path: '/email-queue',
  component: EmailQueuePage,
})

const emailDiagnosticsRoute = createRoute({
  getParentRoute: () => authenticatedLayout,
  path: '/email-diagnostics',
  component: EmailDiagnosticsPage,
})

const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  forbiddenRoute,
  authenticatedLayout.addChildren([
    chatPlaygroundRoute,
    webhookEventsRoute,
    emailQueueRoute,
    emailDiagnosticsRoute,
  ]),
])

export const router = createRouter({
  routeTree,
  basepath: '/admin',
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
  return <Outlet />
}

function IndexPage() {
  const user = useAuthStore((state) => state.user)
  const loading = useAuthStore((state) => state.loading)

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-primary" />
      </div>
    )
  }

  return <Navigate to={user ? '/chat-playground' : '/login'} />
}

function LoginRouteComponent() {
  const user = useAuthStore((state) => state.user)

  if (user) {
    return <Navigate to="/chat-playground" />
  }

  return <LoginPage />
}

function ForbiddenPage() {
  const signOut = useAuthStore((state) => state.signOut)
  const mainAppUrl = import.meta.env.VITE_MAIN_APP_URL || 'https://app.helpin.ai'

  return (
    <div className="flex min-h-screen items-center justify-center px-6">
      <div className="w-full max-w-md border bg-card p-8">
        <h1 className="text-xl font-semibold">Admin access required</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Use a Helpin platform admin account with a registered passkey.
        </p>
        <div className="mt-6 flex gap-3">
          <Button asChild>
            <a href={mainAppUrl}>Open main app</a>
          </Button>
          <Button variant="outline" onClick={signOut}>
            Back to login
          </Button>
        </div>
      </div>
    </div>
  )
}

function NotFoundComponent() {
  return (
    <div className="flex min-h-screen items-center justify-center px-6">
      <div className="w-full max-w-md rounded-none border bg-card p-8 text-center">
        <h1 className="text-xl font-semibold">Page not found</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          The admin route you requested does not exist.
        </p>
        <Button asChild className="mt-6">
          <Link to="/chat-playground">Back to dashboard</Link>
        </Button>
      </div>
    </div>
  )
}
