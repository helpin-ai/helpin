import { createFileRoute } from '@tanstack/react-router'
import { APIReferenceRouteView } from '@/components/routes/APIReferenceRouteView'
import { useDocsContext } from '@/contexts/DocsContext'

export const Route = createFileRoute('/$spaceSlug/api/$referenceSlug')({
  component: APIReferenceRoute,
})

function APIReferenceRoute() {
  const { spaceSlug, referenceSlug } = Route.useParams()
  const { defaultLocale } = useDocsContext()
  return (
    <APIReferenceRouteView
      locale={defaultLocale}
      spaceSlug={spaceSlug}
      referenceSlug={referenceSlug}
      multilingualEnabled={false}
    />
  )
}
