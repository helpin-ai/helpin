import { Outlet, useLocation } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { EmailVerificationBanner } from '@/components/auth/EmailVerificationBanner'
import { Button } from '@/components/ui/button'
import { getAuthenticatedLayoutStyle } from '@/lib/authenticatedLayout'
import { shouldShowEmailVerificationBanner } from '@/lib/emailVerificationBanner'
import { useAuthStore } from '@/stores/authStore'

interface AuthenticatedLayoutProps {
  auth: {
    user: unknown | null
    loading: boolean
    serverUnreachable: boolean
  }
}

export function AuthenticatedLayout({ auth }: AuthenticatedLayoutProps) {
  const user = useAuthStore((state) => state.user)
  const configuration = useAuthStore((state) => state.configuration)
  const [retrying, setRetrying] = useState(false)
  const [topBannerHeight, setTopBannerHeight] = useState(0)
  const topBannerRef = useRef<HTMLDivElement>(null)
  const location = useLocation()
  const showEmailVerificationBanner = shouldShowEmailVerificationBanner({
    emailVerified: user?.email_verified,
    verificationRequired: configuration?.email_verification_required,
    pathname: location.pathname,
    search: location.search as Record<string, unknown>,
  })

  const handleRetry = async () => {
    setRetrying(true)
    await useAuthStore.getState().initialize()
    setRetrying(false)
  }

  useEffect(() => {
    if (!showEmailVerificationBanner) return

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

  return (
    <div style={getAuthenticatedLayoutStyle(showEmailVerificationBanner ? topBannerHeight : 0)}>
      {showEmailVerificationBanner && (
        <div ref={topBannerRef}>
          <EmailVerificationBanner emailVerified={user?.email_verified} />
        </div>
      )}
      <Outlet />
    </div>
  )
}
