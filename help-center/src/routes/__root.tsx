import { createRootRouteWithContext } from '@tanstack/react-router'
import { AppShell } from '@/components/layout/AppShell'
import { LoadingState } from '@/components/LoadingState'
import { ErrorState } from '@/components/ErrorState'
import { DocsProvider } from '@/contexts/DocsContext'
import { useHelpCenterConfig, useSpaces } from '@/hooks/queries'
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

  const { data: spaces, isLoading: spacesLoading } = useSpaces(subdomain)

  if (configLoading || spacesLoading) {
    return <LoadingState message="Loading help center..." fullScreen />
  }

  if (configError) {
    return (
      <ErrorState
        title="Help Center not found"
        message="This help center does not exist or is not currently available."
        statusCode={404}
        fullScreen
      />
    )
  }

  if (config && !config.is_published) {
    return (
      <ErrorState
        title="Help Center unavailable"
        message="This help center is not currently published."
        fullScreen
      />
    )
  }

  return (
    <DocsProvider subdomain={subdomain} config={config!} spaces={spaces ?? []}>
      <AppShell />
    </DocsProvider>
  )
}
