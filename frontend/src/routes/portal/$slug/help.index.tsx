import { createFileRoute } from '@tanstack/react-router'
import { CustomerPortalHelpPage } from '@/components/customer-portal/CustomerPortal'

export const Route = createFileRoute('/portal/$slug/help/')({
  component: CustomerPortalHelpPage,
})
