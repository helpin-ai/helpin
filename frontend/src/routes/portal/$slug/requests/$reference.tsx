import { createFileRoute } from '@tanstack/react-router'
import { CustomerPortalRequestPage } from '@/components/customer-portal/CustomerPortal'

export const Route = createFileRoute('/portal/$slug/requests/$reference')({
  component: () => {
    const { reference } = Route.useParams()
    return <CustomerPortalRequestPage reference={reference} />
  },
})
