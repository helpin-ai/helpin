import type { QueryClient } from '@tanstack/react-query'
import type { HelpCenterConfig, Space } from '@/lib/types'
import { isMultilingualEnabled } from '@/lib/locale'
import { stripBasepath } from '@/lib/pathUtils'
import { getHelpCenterRequestContext } from '@/lib/requestContext'
import { queryKeys } from '@/lib/queryKeys'
import { helpCenterService } from '@/lib/services'

export interface RootRouteData {
  activeLocale: string
  basepath: string
  config: HelpCenterConfig
  host: string
  multilingualEnabled: boolean
  origin: string
  spaces: Space[]
  subdomain: string
}

/**
 * Groups bootstrap data by effective locale rather than by full pathname so
 * navigating between collections reuses the already-hydrated root shell.
 */
export function getBootstrapLocaleScope(pathname: string) {
  const firstSegment = pathname.split('/').filter(Boolean)[0]?.toLowerCase() ?? ''
  if (/^[a-z]{2}(?:-[a-z0-9]+)?$/.test(firstSegment)) {
    return `locale:${firstSegment}`
  }
  return 'default'
}

export async function loadRootRouteData(
  queryClient: QueryClient,
  pathname: string,
): Promise<RootRouteData> {
  const requestContext = await getHelpCenterRequestContext()
  const normalizedPathname = stripBasepath(pathname, requestContext.basepath)
  const localeScope = getBootstrapLocaleScope(normalizedPathname)
  const bootstrap = await queryClient.ensureQueryData({
    queryKey: queryKeys.helpCenter.bootstrap(
      requestContext.subdomain,
      localeScope,
    ),
    queryFn: async () => {
      const response = await helpCenterService.getBootstrap(
        requestContext.subdomain,
        normalizedPathname,
      )
      if (response.error || !response.data) {
        throw new Error(response.error || 'Help center unavailable')
      }
      return response.data
    },
  })

  const { config, locale: activeLocale, spaces } = bootstrap
  const multilingualEnabled = isMultilingualEnabled(config.enabled_locales)
  queryClient.setQueryData(
    queryKeys.helpCenter.config(requestContext.subdomain),
    config,
  )
  queryClient.setQueryData(
    queryKeys.helpCenter.spaces(requestContext.subdomain, activeLocale),
    spaces,
  )

  return {
    activeLocale,
    basepath: requestContext.basepath,
    config,
    host: requestContext.host,
    multilingualEnabled,
    origin: `${requestContext.protocol}://${requestContext.host}`,
    spaces,
    subdomain: requestContext.subdomain,
  }
}
