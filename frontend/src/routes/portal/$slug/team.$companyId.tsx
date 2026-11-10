import { createFileRoute } from '@tanstack/react-router'
import { CustomerPortalTeamPage } from '@/components/customer-portal/CustomerPortal'

export const Route = createFileRoute('/portal/$slug/team/$companyId')({
  component: TeamRoute,
})

function TeamRoute() {
  const { companyId } = Route.useParams()
  return <CustomerPortalTeamPage companyId={companyId} />
}
