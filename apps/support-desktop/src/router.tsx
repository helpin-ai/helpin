import { useEffect } from 'react'
import {
  Link,
  Navigate,
  Outlet,
  createRootRouteWithContext,
  createRoute,
  createRouter,
  redirect,
} from '@tanstack/react-router'
import { ThemeProvider, useTheme } from 'next-themes'
import { ConfirmProvider } from '@/components/ui/confirm-dialog'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useQuery } from '@tanstack/react-query'
import { workspacesService } from '@/lib/services/workspacesService'
import { useAuthStore } from '@/stores/authStore'
import type { User } from '@/lib/types'
import { LoginPage } from '@desktop/pages/login-page'
import { OpenInWebPage } from '@desktop/pages/open-in-web-page'
import { SupportWorkspacePage } from '@desktop/pages/support-workspace-page'
import { WorkspacesPage } from '@desktop/pages/workspaces-page'

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

const workspacesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/workspaces',
  beforeLoad: requireAuth,
  component: WorkspacesPage,
})

const supportWorkspaceRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/w/$slug/support',
  beforeLoad: requireAuth,
  component: SupportWorkspacePage,
})

const supportConversationRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/w/$slug/support/$conversationId',
  beforeLoad: requireAuth,
  component: SupportWorkspacePage,
})

const crmContactRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/w/$slug/crm/contacts/$contactId',
  beforeLoad: requireAuth,
  component: () => (
    <OpenInWebPage
      title="Open CRM Contact"
      pathBuilder={({ slug, contactId }) => `/w/${slug}/crm/contacts/${contactId}`}
    />
  ),
})

const docsDocumentRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/w/$slug/docs/documents/$docId',
  beforeLoad: requireAuth,
  component: () => (
    <OpenInWebPage
      title="Open Document"
      pathBuilder={({ slug, docId }) => `/w/${slug}/docs/documents/${docId}`}
    />
  ),
})

const pmTaskRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/w/$slug/pm/tasks/$taskId',
  beforeLoad: requireAuth,
  component: () => (
    <OpenInWebPage
      title="Open Task"
      pathBuilder={({ slug, taskId }) => `/w/${slug}/pm/tasks/${taskId}`}
    />
  ),
})

const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  workspacesRoute,
  supportWorkspaceRoute,
  supportConversationRoute,
  crmContactRoute,
  docsDocumentRoute,
  pmTaskRoute,
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

/** Listens for Tauri menu "theme-change" events and applies them via next-themes. */
function TauriThemeListener() {
  const { theme, setTheme } = useTheme()

  useEffect(() => {
    let unlisten: (() => void) | undefined

    async function setup() {
      try {
        const { listen } = await import('@tauri-apps/api/event')
        const unlistenFn = await listen<string>('theme-change', (event) => {
          if (event.payload === 'toggle') {
            setTheme(theme === 'dark' ? 'light' : 'dark')
          } else {
            setTheme(event.payload)
          }
        })
        unlisten = unlistenFn
      } catch {
        // Not running inside Tauri (e.g. browser dev mode) — ignore
      }
    }

    setup()
    return () => unlisten?.()
  }, [theme, setTheme])

  return null
}

function RootComponent() {
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
      <TooltipProvider>
        <ConfirmProvider>
          <TauriThemeListener />
          <Outlet />
        </ConfirmProvider>
      </TooltipProvider>
    </ThemeProvider>
  )
}

function IndexPage() {
  const user = useAuthStore((state) => state.user)
  const loading = useAuthStore((state) => state.loading)

  const { data: workspaces } = useQuery({
    queryKey: ['desktop-index-workspaces'],
    queryFn: async () => {
      const response = await workspacesService.list()
      return response.data ?? []
    },
    enabled: !!user,
    staleTime: 60_000,
  })

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-primary" />
      </div>
    )
  }

  if (!user) {
    return <Navigate to="/login" />
  }

  if (!workspaces || workspaces.length === 0) {
    return <Navigate to="/workspaces" />
  }

  const defaultWorkspace = user.default_workspace_id
    ? workspaces.find((workspace) => workspace.id === user.default_workspace_id) ?? null
    : null

  return (
    <Navigate
      to="/w/$slug/support"
      params={{ slug: (defaultWorkspace ?? workspaces[0]).slug }}
    />
  )
}

function LoginRouteComponent() {
  const user = useAuthStore((state) => state.user)

  if (user) {
    return <Navigate to="/" />
  }

  return <LoginPage />
}

function NotFoundComponent() {
  return (
    <div className="flex min-h-screen items-center justify-center px-6">
      <div className="w-full max-w-md rounded-3xl border border-border/70 bg-card/95 p-8 text-center shadow-lg shadow-black/5">
        <h1 className="text-xl font-semibold">Page not found</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          The support desktop route you requested does not exist.
        </p>
        <Link
          to="/"
          className="mt-6 inline-flex h-10 items-center justify-center rounded-xl bg-primary px-4 text-sm font-medium text-primary-foreground transition hover:opacity-95"
        >
          Back to app
        </Link>
      </div>
    </div>
  )
}
