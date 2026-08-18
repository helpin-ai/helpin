import { createFileRoute, redirect } from '@tanstack/react-router'
import { AuthenticatedLayout } from '@/components/layout/AuthenticatedLayout'
import { currentPathForLoginRedirect, storeRedirectAfterLogin } from '@/lib/authRedirect'

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: ({ context, location }) => {
    // Don't redirect to login if server is just unreachable — user may still have valid tokens
    if (!context.auth.loading && !context.auth.user && !context.auth.serverUnreachable) {
      storeRedirectAfterLogin(currentPathForLoginRedirect(location))
      throw redirect({
        to: '/login',
        search: { redirect: currentPathForLoginRedirect(location) },
      })
    }
  },
  component: AuthenticatedRoute,
})

function AuthenticatedRoute() {
  const { auth } = Route.useRouteContext()
  return <AuthenticatedLayout auth={auth} />
}
