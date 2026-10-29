import { useEffect, useState } from 'react'
import { customerPortalService, type CustomerPortalRequestScope } from '@/lib/services/customerPortalService'

const DEBOUNCE_MS = 300

/**
 * Searches every message of a request list on the server once the customer
 * has typed two characters, after a short pause. It returns the matching
 * references with an excerpt, or null while there is no result for the
 * query; callers keep their instant local filter meanwhile.
 */
export function usePortalRequestSearch(slug: string, scope: CustomerPortalRequestScope, query: string) {
  const [result, setResult] = useState<{ key: string; matches: Map<string, string> } | null>(null)
  const scopeKey = scope.kind === 'company' ? `company:${scope.companyId}` : scope.kind
  const key = `${scopeKey}\u0000${query}`
  useEffect(() => {
    if (query.length < 2) return
    const controller = new AbortController()
    const timer = window.setTimeout(() => {
      customerPortalService.searchRequests(slug, query, scope, controller.signal)
        .then((rows) => setResult({ key, matches: new Map(rows.map((row) => [row.reference, row.snippet ?? ''])) }))
        // The local filter still answers; a failed search adds nothing.
        .catch(() => undefined)
    }, DEBOUNCE_MS)
    return () => {
      window.clearTimeout(timer)
      controller.abort()
    }
    // scope is represented by scopeKey.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slug, key])
  return query.length >= 2 && result?.key === key ? result.matches : null
}
