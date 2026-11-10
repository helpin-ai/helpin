import { useEffect, useState } from 'react'
import { customerPortalService, type CustomerPortalCCOptions } from '@/lib/services/customerPortalService'

export const PORTAL_CC_LIMIT = 10

/** Loads whom the signed-in customer may copy, while copying is on. */
export function usePortalCCOptions(slug: string, enabled: boolean) {
  const [options, setOptions] = useState<CustomerPortalCCOptions | null>(null)
  useEffect(() => {
    if (!enabled) return
    let active = true
    customerPortalService.ccOptions(slug)
      .then((result) => { if (active) setOptions(result) })
      // Suggestions are optional; the server still checks every address.
      .catch(() => { if (active) setOptions(null) })
    return () => { active = false }
  }, [enabled, slug])
  return options
}

