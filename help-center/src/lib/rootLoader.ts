import type { QueryClient } from '@tanstack/react-query'
import type { HelpCenterConfig, Space } from '@/lib/types'
import {
  helpCenterConfigQueryOptions,
  spacesQueryOptions,
} from '@/hooks/queries'
import { isMultilingualEnabled, resolveActiveLocale } from '@/lib/locale'
import { getHelpCenterRequestContext } from '@/lib/requestContext'

export interface RootRouteData {
  activeLocale: string
  config: HelpCenterConfig
  host: string
  multilingualEnabled: boolean
  origin: string
  spaces: Space[]
  subdomain: string
}

function getPrimaryPathSegment(pathname: string) {
  const [firstSegment] = pathname.split('/').filter(Boolean)
  return firstSegment
}

export async function loadRootRouteData(
  queryClient: QueryClient,
  pathname: string,
): Promise<RootRouteData> {
  const requestContext = await getHelpCenterRequestContext()
  const config = await queryClient.ensureQueryData(
    helpCenterConfigQueryOptions(requestContext.subdomain),
  )

  const firstSegment = getPrimaryPathSegment(pathname)
  const activeLocale = resolveActiveLocale({
    paramsLocale: firstSegment,
    paramsSpaceSlug: firstSegment,
    enabledLocales: config.enabled_locales,
    defaultLocale: config.default_locale || 'en',
  })
  const multilingualEnabled = isMultilingualEnabled(config.enabled_locales)
  const spaces = await queryClient.ensureQueryData(
    spacesQueryOptions(
      requestContext.subdomain,
      activeLocale,
      multilingualEnabled,
    ),
  )

  return {
    activeLocale,
    config,
    host: requestContext.host,
    multilingualEnabled,
    origin: `${requestContext.protocol}://${requestContext.host}`,
    spaces,
    subdomain: requestContext.subdomain,
  }
}
