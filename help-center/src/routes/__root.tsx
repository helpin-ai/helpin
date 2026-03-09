import { createRootRouteWithContext } from '@tanstack/react-router'
import { AppShell } from '@/components/layout/AppShell'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { useHelpCenterConfig, useNavigation } from '@/hooks/queries'
import type { HelpCenterContext } from '@/lib/types'

export const Route = createRootRouteWithContext<HelpCenterContext>()({
  component: RootLayout,
})

function RootLayout() {
  const subdomain = Route.useRouteContext({ select: (s) => s.subdomain })

  const {
    data: config,
    isLoading: configLoading,
    error: configError,
  } = useHelpCenterConfig(subdomain)

  const {
    data: navigation,
    isLoading: navLoading,
  } = useNavigation(subdomain)

  // Loading
  if (configLoading || navLoading) {
    return <LoadingState message="Loading help center..." />
  }

  // Config error — invalid domain / inactive
  if (configError) {
    return (
      <ErrorState
        title="Help Center not found"
        message="This help center does not exist or is not currently available."
        statusCode={404}
      />
    )
  }

  // Not published
  if (config && !config.is_published) {
    return (
      <ErrorState
        title="Help Center unavailable"
        message="This help center is not currently published."
      />
    )
  }

  return <AppShell config={config} navigation={navigation ?? []} />
}
