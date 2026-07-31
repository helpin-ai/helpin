import { createFileRoute, Outlet, redirect, useLocation } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/authStore'
import { Button } from '@/components/ui/button'
import { useEffect, useRef, useState } from 'react'
import { currentPathForLoginRedirect, storeRedirectAfterLogin } from '@/lib/authRedirect'
import { EmailVerificationBanner } from '@/components/auth/EmailVerificationBanner'
import { shouldShowEmailVerificationBanner } from '@/lib/emailVerificationBanner'
import { getAuthenticatedLayoutStyle } from '@/lib/authenticatedLayout'

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
  component: AuthenticatedLayout,
})

function AuthenticatedLayout() {
  const { auth } = Route.useRouteContext()
  const user = useAuthStore((state) => state.user)
  const [retrying, setRetrying] = useState(false)
  const [topBannerHeight, setTopBannerHeight] = useState(0)
  const topBannerRef = useRef<HTMLDivElement>(null)
  const location = useLocation()
  const showEmailVerificationBanner = shouldShowEmailVerificationBanner({
    emailVerified: user?.email_verified,
    pathname: location.pathname,
    search: location.search as Record<string, unknown>,
  })

  const handleRetry = async () => {
    setRetrying(true)
    await useAuthStore.getState().initialize()
    setRetrying(false)
  }

  if (auth.loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
      </div>
    )
  }

  if (auth.serverUnreachable && !auth.user) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen gap-4">
        <div className="text-center space-y-2">
          <h2 className="text-xl font-semibold">Unable to reach the server</h2>
          <p className="text-muted-foreground">
            The server appears to be unavailable. Your session is preserved — please try again.
          </p>
        </div>
        <Button onClick={handleRetry} disabled={retrying}>
          {retrying ? 'Retrying...' : 'Retry'}
        </Button>
      </div>
    )
  }

  useEffect(() => {
    if (!showEmailVerificationBanner) {
      setTopBannerHeight(0)
      return
    }

    const node = topBannerRef.current
    if (!node) return

    const updateHeight = () => setTopBannerHeight(node.getBoundingClientRect().height)
    updateHeight()

    if (typeof ResizeObserver === 'undefined') {
      window.addEventListener('resize', updateHeight)
      return () => window.removeEventListener('resize', updateHeight)
    }

    const observer = new ResizeObserver(updateHeight)
    observer.observe(node)
    return () => observer.disconnect()
  }, [showEmailVerificationBanner])

  return (
    <div style={getAuthenticatedLayoutStyle(topBannerHeight)}>
      {showEmailVerificationBanner && (
        <div ref={topBannerRef}>
          <EmailVerificationBanner emailVerified={user?.email_verified} />
        </div>
      )}
      <Outlet />
    </div>
  )
}
