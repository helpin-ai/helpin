import { useEffect } from 'react'
import { useHelpin } from '@helpin-ai/react'
import { buildHelpinIdentity } from '@/lib/helpin'
import { useAuthStore } from '@/stores/authStore'
import { useOrganizationStore } from '@/stores/organizationStore'

export function HelpinIdentitySync() {
  const user = useAuthStore((state) => state.user)
  const organization = useOrganizationStore((state) => state.currentOrganization)
  const { id } = useHelpin()

  useEffect(() => {
    if (!user) return

    void id(buildHelpinIdentity(user, organization)).catch((error) => {
      console.warn('Helpin identify failed', error)
    })
  }, [id, organization, user])

  return null
}
