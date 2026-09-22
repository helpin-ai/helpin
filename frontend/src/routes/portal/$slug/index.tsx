import { createFileRoute } from '@tanstack/react-router'
import { CustomerPortalHome } from '@/components/customer-portal/CustomerPortal'

export const Route = createFileRoute('/portal/$slug/')({
  component: CustomerPortalHome,
})
