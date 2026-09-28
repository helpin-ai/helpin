import { createFileRoute } from '@tanstack/react-router'
import { CustomerPortalNewRequestPage } from '@/components/customer-portal/CustomerPortal'

export const Route = createFileRoute('/portal/$slug/new')({
  component: CustomerPortalNewRequestPage,
})
