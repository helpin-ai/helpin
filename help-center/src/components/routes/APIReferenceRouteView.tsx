import { OpenAPIReference } from '@/components/api-reference/OpenAPIReference'
import { ErrorState } from '@/components/ErrorState'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'
import { useAPIReference, useAPIReferences } from '@/hooks/queries'
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
  const { subdomain } = useDocsContext()
  const { data, isLoading, error } = useAPIReference(
    subdomain,
    locale,
    spaceSlug,
    referenceSlug,
    multilingualEnabled,
  )
  const { data: references = [] } = useAPIReferences(
    subdomain,
    locale,
    spaceSlug,
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

  return (
    <OpenAPIReference
      reference={data}
      references={references}
      locale={locale}
      spaceSlug={spaceSlug}
      multilingualEnabled={multilingualEnabled}
    />
  )
}
