import { useCallback, useState } from 'react'
import type { CustomerPortalCompany } from '@/lib/services/customerPortalService'

/** The request list a customer can choose: their own, a member company's, or those copied to them. */
export const MINE_SCOPE = 'mine'
export const COPIED_SCOPE = 'copied'

const storageKey = (slug: string) => `helpin.portal.${slug}.requestScope`

function readStoredScope(slug: string) {
  try {
    return window.sessionStorage.getItem(storageKey(slug)) ?? MINE_SCOPE
  } catch {
    return MINE_SCOPE
  }
}

/**
 * usePortalRequestScope remembers the chosen request list for this browser
 * tab, so returning from a request keeps it. A stored company the customer
 * is no longer a member of falls back to their own requests.
 */
export function usePortalRequestScope(slug: string, companies: CustomerPortalCompany[], copiedEnabled = false) {
  const [stored, setStored] = useState(() => readStoredScope(slug))
  const memberCompanies = companies.filter((company) => company.role === 'member' || company.role === 'admin')
  const company = memberCompanies.find((item) => item.id === stored)
  const key = company ? company.id : copiedEnabled && stored === COPIED_SCOPE ? COPIED_SCOPE : MINE_SCOPE
  const select = useCallback((next: string) => {
    setStored(next)
    try {
      window.sessionStorage.setItem(storageKey(slug), next)
    } catch {
      // Storage can be unavailable; the choice still applies to this page.
    }
  }, [slug])
  return { key, company, memberCompanies, copiedEnabled, select }
}
