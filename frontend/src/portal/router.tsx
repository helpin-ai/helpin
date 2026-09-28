import { createRootRoute, createRoute, createRouter, Outlet } from '@tanstack/react-router'
import { ThemeProvider } from 'next-themes'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import {
  CustomerPortalCallback,
  CustomerPortalHome,
  CustomerPortalNewRequestPage,
  CustomerPortalProvider,
  CustomerPortalRequestPage,
  CustomerPortalSignIn,
} from '@/components/customer-portal/CustomerPortal'
import { portalMountRewrite, type PortalMount } from './mount'

/**
 * createPortalRouter builds the portal-only app served on help centers. It
 * reuses the app's portal pages and route paths, rewritten to /requests.
 */
export function createPortalRouter(mount: PortalMount) {
  const rootRoute = createRootRoute({
    component: () => (
      <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
        <TooltipProvider>
          <Outlet />
          <Toaster closeButton />
        </TooltipProvider>
      </ThemeProvider>
    ),
  })
  const portalRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/portal/$slug',
    component: function PortalRoute() {
      const { slug } = portalRoute.useParams()
      return <CustomerPortalProvider key={slug} slug={slug} />
    },
  })
  const homeRoute = createRoute({ getParentRoute: () => portalRoute, path: '/', component: CustomerPortalHome })
  const signInRoute = createRoute({ getParentRoute: () => portalRoute, path: '/sign-in', component: CustomerPortalSignIn })
  const newRoute = createRoute({ getParentRoute: () => portalRoute, path: '/new', component: CustomerPortalNewRequestPage })
  const callbackRoute = createRoute({
    getParentRoute: () => portalRoute,
    path: '/callback',
    validateSearch: (search: Record<string, unknown>): { token?: string } => ({
      token: typeof search.token === 'string' ? search.token : undefined,
    }),
    component: function CallbackRoute() {
      const { token } = callbackRoute.useSearch()
      return <CustomerPortalCallback token={token} />
    },
  })
  const requestRoute = createRoute({
    getParentRoute: () => portalRoute,
    path: '/requests/$reference',
    component: function RequestRoute() {
      const { reference } = requestRoute.useParams()
      return <CustomerPortalRequestPage reference={reference} />
    },
  })

  const router = createRouter({
    routeTree: rootRoute.addChildren([
      portalRoute.addChildren([homeRoute, signInRoute, newRoute, callbackRoute, requestRoute]),
    ]),
    rewrite: portalMountRewrite(mount),
    defaultNotFoundComponent: () => {
      void router.navigate({ to: '/portal/$slug', params: { slug: mount.slug }, replace: true } as never)
      return null
    },
  })
  return router
}
