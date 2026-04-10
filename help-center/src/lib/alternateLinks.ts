import type { QueryClient } from '@tanstack/react-query'
import {
  spaceNavigationQueryOptions,
  spacesQueryOptions,
} from '@/hooks/queries'
import type { RootRouteData } from '@/lib/rootLoader'
import {
  resolveExactLocalePath,
  resolveLocaleSwitchPath,
  type LocaleRouteState,
} from '@/lib/locale'
import type { NavItem, Space } from '@/lib/types'

export interface AlternateLink {
  rel: 'alternate'
  hrefLang: string
  href: string
}

function tenantPath(basepath: string, path: string) {
  const normalizedBasepath = basepath ? basepath.replace(/\/+$/, '') : ''
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  if (!normalizedBasepath) return normalizedPath
  if (normalizedPath === '/') return `${normalizedBasepath}/`
  return `${normalizedBasepath}${normalizedPath}`
}

function absoluteUrl(rootData: RootRouteData, path: string) {
  return new URL(tenantPath(rootData.basepath, path), rootData.origin).toString()
}

async function loadSpacesForLocale(
  queryClient: QueryClient,
  rootData: RootRouteData,
  locale: string,
) {
  if (locale === rootData.activeLocale) {
    return rootData.spaces
  }

  return queryClient.ensureQueryData(
    spacesQueryOptions(rootData.subdomain, locale, rootData.multilingualEnabled),
  )
}

async function loadNavigationForLocale(
  queryClient: QueryClient,
  rootData: RootRouteData,
  locale: string,
  spaces: Space[],
  current: LocaleRouteState,
) {
  if (current.kind !== 'collection' && current.kind !== 'article') {
    return [] as NavItem[]
  }

  const targetSpace = spaces.find((space) => space.id === current.spaceId)
  if (!targetSpace) {
    return [] as NavItem[]
  }

  return queryClient.ensureQueryData(
    spaceNavigationQueryOptions(
      rootData.subdomain,
      locale,
      targetSpace.slug,
      rootData.multilingualEnabled,
    ),
  )
}

export async function loadAlternateLinks(
  queryClient: QueryClient,
  rootData: RootRouteData,
  current: LocaleRouteState,
): Promise<AlternateLink[]> {
  if (!rootData.multilingualEnabled) {
    return []
  }

  const links: AlternateLink[] = []
  const defaultLocale = rootData.config.default_locale
  const defaultSpaces = await loadSpacesForLocale(queryClient, rootData, defaultLocale)
  const defaultNavigation = await loadNavigationForLocale(
    queryClient,
    rootData,
    defaultLocale,
    defaultSpaces,
    current,
  )

  for (const locale of rootData.config.enabled_locales) {
    const targetSpaces = await loadSpacesForLocale(queryClient, rootData, locale)
    const targetNavigation = await loadNavigationForLocale(
      queryClient,
      rootData,
      locale,
      targetSpaces,
      current,
    )
    const href = resolveExactLocalePath({
      multilingualEnabled: rootData.multilingualEnabled,
      targetLocale: locale,
      current,
      targetSpaces,
      targetNavigation,
    })

    if (!href) {
      continue
    }

    links.push({
      rel: 'alternate',
      hrefLang: locale,
      href: absoluteUrl(rootData, href),
    })
  }

  const xDefaultHref = resolveLocaleSwitchPath({
    multilingualEnabled: rootData.multilingualEnabled,
    targetLocale: defaultLocale,
    defaultLocale,
    current,
    targetSpaces: defaultSpaces,
    targetNavigation: defaultNavigation,
    fallbackSpaces: defaultSpaces,
    fallbackNavigation: defaultNavigation,
  })

  links.push({
    rel: 'alternate',
    hrefLang: 'x-default',
    href: absoluteUrl(rootData, xDefaultHref),
  })

  return links
}
