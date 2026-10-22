import { createFileRoute } from '@tanstack/react-router'
import { CustomerPortalCallback } from '@/components/customer-portal/CustomerPortal'

interface CallbackSearch {
  token?: string
}

export const Route = createFileRoute('/portal/$slug/callback')({
  validateSearch: (search: Record<string, unknown>): CallbackSearch => ({
    token: typeof search.token === 'string' ? search.token : undefined,
  }),
  component: CallbackRoute,
})

function CallbackRoute() {
  const { token } = Route.useSearch()
  return <CustomerPortalCallback token={token} />
}
