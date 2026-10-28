function normalizePathname(pathname: string) {
  if (!pathname) return '/'
  const normalized = pathname.replace(/\/+$/, '')
  return normalized === '' ? '/' : normalized
}

/** isLogoutPath is true when the app was opened at /logout. */
export const isLogoutPath =
  typeof window !== 'undefined' && normalizePathname(window.location.pathname) === '/logout'

/**
 * isCustomerPortalPath is true on public customer pages: the customer portal
 * and the team-reply page from AI emails. Staff session checks and developer
 * tools never run there.
 */
export const isCustomerPortalPath =
  typeof window !== 'undefined' && /^\/(portal|team-reply)(\/|$)/.test(normalizePathname(window.location.pathname))
