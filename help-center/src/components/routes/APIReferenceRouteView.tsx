import { ApiReferenceReact } from '@scalar/api-reference-react'
import { ErrorState } from '@/components/ErrorState'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'
import { useAPIReference } from '@/hooks/queries'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'

interface APIReferenceRouteViewProps {
  locale: string
  spaceSlug: string
  referenceSlug: string
  multilingualEnabled: boolean
}

export function APIReferenceRouteView({
  locale,
  spaceSlug,
  referenceSlug,
  multilingualEnabled,
}: APIReferenceRouteViewProps) {
  const { subdomain, config } = useDocsContext()
  const { data, isLoading, error } = useAPIReference(
    subdomain,
    locale,
    spaceSlug,
    referenceSlug,
    multilingualEnabled,
  )
  useDocumentTitle(data?.name)

  if (isLoading) return <LoadingState message="Loading API reference..." />
  if (error || !data) {
    return (
      <ErrorState
        title="API reference not found"
        message="This API reference does not exist or has not been published."
        statusCode={404}
      />
    )
  }

  const darkMode =
    config.theme_mode === 'dark' ||
    (config.theme_mode === 'system' &&
      typeof document !== 'undefined' &&
      document.documentElement.classList.contains('dark'))

  return (
    <main className="api-reference-page min-w-0">
      <ApiReferenceReact
        configuration={{
          content: data.specification,
          layout: 'modern',
          theme: 'none',
          showSidebar: true,
          hideModels: false,
          hideClientButton: false,
          hideTestRequestButton: false,
          documentDownloadType: 'both',
          darkMode,
          forceDarkModeState: darkMode ? 'dark' : 'light',
          hideDarkModeToggle: true,
          customCss: `
            .scalar-app { --scalar-color-accent: ${config.brand_color}; }
            .references-rendered { min-height: calc(100vh - var(--hc-header-height)); }
          `,
        }}
      />
    </main>
  )
}
