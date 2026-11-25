import { createFileRoute } from '@tanstack/react-router'
import { SupportStart } from '@/components/onboarding/SupportStart'

type StartSearch = {
  /** The company website, as typed: acme.com or a full address. */
  website?: string
}

export const Route = createFileRoute('/start')({
  component: StartRoute,
  validateSearch: (search: Record<string, unknown>): StartSearch => {
    const website = typeof search.website === 'string' ? search.website.trim().slice(0, 300) : ''
    return { website: website || undefined }
  },
})

function StartRoute() {
  const { website } = Route.useSearch()
  return <SupportStart website={website} />
}
