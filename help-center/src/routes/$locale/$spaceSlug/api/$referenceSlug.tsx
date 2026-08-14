import { createFileRoute } from '@tanstack/react-router'
import { APIReferenceRouteView } from '@/components/routes/APIReferenceRouteView'

export const Route = createFileRoute('/$locale/$spaceSlug/api/$referenceSlug')({
  component: LocalizedAPIReferenceRoute,
})

function LocalizedAPIReferenceRoute() {
  const { locale, spaceSlug, referenceSlug } = Route.useParams()
  return (
    <APIReferenceRouteView
      locale={locale}
      spaceSlug={spaceSlug}
      referenceSlug={referenceSlug}
      multilingualEnabled
    />
  )
}
