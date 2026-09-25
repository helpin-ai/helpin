import { createFileRoute } from '@tanstack/react-router'
import { CustomerPortalProvider } from '@/components/customer-portal/CustomerPortal'

export const Route = createFileRoute('/portal/$slug')({
  component: PortalRoute,
})

function PortalRoute() {
  const { slug } = Route.useParams()
  return <CustomerPortalProvider slug={slug} />
}
