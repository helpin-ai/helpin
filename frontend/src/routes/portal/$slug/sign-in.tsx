import { createFileRoute } from '@tanstack/react-router'
import { CustomerPortalSignIn } from '@/components/customer-portal/CustomerPortal'

export const Route = createFileRoute('/portal/$slug/sign-in')({
  component: CustomerPortalSignIn,
})
