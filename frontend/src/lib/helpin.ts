import { defaultSupportWidgetKey, defaultSupportWidgetHost } from '@edition/config'
import { createClient } from '@helpin-ai/react'
import type { OrganizationWithRole, User } from '@/lib/types'

const widgetKey = import.meta.env.VITE_HELPIN_WIDGET_KEY || defaultSupportWidgetKey
const widgetHost = import.meta.env.VITE_HELPIN_HOST || defaultSupportWidgetHost
export const helpinClient = widgetKey && widgetHost ? createClient({ widgetKey, host: widgetHost }) : null


export function buildHelpinIdentity(user: User, organization?: OrganizationWithRole | null) {
  const nameParts = user.full_name.trim().split(/\s+/).filter(Boolean)
  const firstName = nameParts[0]
  const lastName = nameParts.length > 1 ? nameParts.slice(1).join(' ') : undefined

  return {
    id: user.id,
    email: user.email,
    first_name: firstName,
    last_name: lastName,
    created_at: user.created_at,
    company: organization
      ? {
          id: organization.id,
          name: organization.name,
          created_at: organization.created_at,
          custom: {
            role: organization.role,
            slug: organization.slug,
          },
        }
      : undefined,
  }
}

export async function resetHelpinIdentity() {
  await helpinClient?.reset(true)
}
