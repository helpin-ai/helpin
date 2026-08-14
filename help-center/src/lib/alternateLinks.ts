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
import { absolutePublicUrl } from '@/lib/publicUrl'
import type { CollectionPage, NavItem, Space } from '@/lib/types'

export interface AlternateLink {
  rel: 'alternate'
  hrefLang: string
  href: string
}

function absoluteUrl(rootData: RootRouteData, path: string) {
  return absolutePublicUrl(rootData, path)
}

/**
 * Builds collection hreflang links from the compact alternate-path map that
 * ships with the collection response. This keeps SSR metadata complete while
 * avoiding spaces + navigation requests for every enabled locale.
 */
export function buildCollectionAlternateLinks(
  rootData: RootRouteData,
  collection: CollectionPage,
): AlternateLink[] {
  const paths = collection.alternate_paths ?? {}
  const links = rootData.config.enabled_locales.flatMap((configuredLocale) => {
    const locale = configuredLocale.trim().toLowerCase()
    const path = paths[locale]
    if (!locale || !path) return []
    return [{
      rel: 'alternate' as const,
      hrefLang: locale,
      href: absoluteUrl(rootData, path),
    }]
  })

  const defaultLocale = rootData.config.default_locale.trim().toLowerCase()
  const defaultPath = paths[defaultLocale]
  if (defaultPath) {
    links.push({
      rel: 'alternate',
      hrefLang: 'x-default',
      href: absoluteUrl(rootData, defaultPath),
    })
  }

  return links
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

  const localeEntries = await Promise.all(
    rootData.config.enabled_locales.map(async (locale) => {
      const targetSpaces =
        locale === defaultLocale
          ? defaultSpaces
          : await loadSpacesForLocale(queryClient, rootData, locale)
      const targetNavigation =
        locale === defaultLocale
          ? defaultNavigation
          : await loadNavigationForLocale(
              queryClient,
              rootData,
              locale,
              targetSpaces,
              current,
            )

      return { locale, targetSpaces, targetNavigation }
    }),
  )

  for (const { locale, targetSpaces, targetNavigation } of localeEntries) {
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
