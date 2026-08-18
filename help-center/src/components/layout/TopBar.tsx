import { useEffect, useMemo, useRef } from 'react'
import { useQueries } from '@tanstack/react-query'
import { Search, Moon, Sun } from 'lucide-react'
import {
  useParams,
  useMatches,
  useRouterState,
  useSearch,
} from '@tanstack/react-router'
import { DocsLink } from '@/components/DocsLink'
import { cn } from '@/lib/utils'
import { useDocsContext } from '@/contexts/DocsContext'
import { useSpaceNavigation } from '@/hooks/queries'
import { useTheme } from '@/hooks/useTheme'
import { queryKeys } from '@/lib/queryKeys'
import { helpCenterService } from '@/lib/services'
import { parseArticleKey } from '@/lib/articleKey'
import { parseCollectionKey } from '@/lib/collectionKey'
import type { LocaleRouteState } from '@/lib/locale'
import {
  buildCanonicalHomePath,
  buildCanonicalSpacePath,
  resolveLocaleSwitchPath,
} from '@/lib/locale'
import { prefixBasepath } from '@/lib/pathUtils'
import { PublicIcon } from '@/components/PublicIcon'
import { LocaleSwitcher } from './LocaleSwitcher'
import type { ArticleDetail, CollectionPage, NavItem, Space } from '@/lib/types'

interface TopBarProps {
  onSearchClick: () => void
}

function unwrap<T>(res: { data: T | null; error: string | null }) {
  if (res.error) throw new Error(res.error)
  return res.data as T
}

function getLocaleLabel(code: string) {
  try {
    return (
      new Intl.DisplayNames([code, 'en'], { type: 'language' }).of(code) ??
      code.toUpperCase()
    )
  } catch {
    return code.toUpperCase()
  }
}

function findCollectionId(navigation: NavItem[], collectionSlug?: string) {
  if (!collectionSlug) return undefined
  const parsed = parseCollectionKey(collectionSlug)
  return navigation.find((collection) =>
    parsed
      ? collection.public_id === parsed.publicId
      : collection.slug === collectionSlug,
  )?.id
}

function findArticleId(navigation: NavItem[], articleKey?: string) {
  if (!articleKey) return undefined
  const parsed = parseArticleKey(articleKey)
  if (!parsed) return undefined
  for (const collection of navigation) {
    const article = collection.articles.find(
      (candidate) => candidate.public_id === parsed.publicId,
    )
    if (article) return article.id
  }
  return undefined
}

function findSpaceId(spaces: Space[], spaceSlug?: string) {
  if (!spaceSlug) return undefined
  return spaces.find((space) => space.slug === spaceSlug)?.id
}

interface RouteLoaderData {
  article?: ArticleDetail | null
  collection?: CollectionPage | null
}

function getRouteLoaderData(matches: Array<{ loaderData?: unknown }>) {
  for (let index = matches.length - 1; index >= 0; index -= 1) {
    const loaderData = matches[index]?.loaderData as RouteLoaderData | undefined
    if (loaderData?.article || loaderData?.collection) {
      return loaderData
    }
  }
  return undefined
}

export function TopBar({ onSearchClick }: TopBarProps) {
  const headerRef = useRef<HTMLElement>(null)
  const leftContentRef = useRef<HTMLDivElement>(null)
  const rightContentRef = useRef<HTMLDivElement>(null)
  const searchRef = useRef<HTMLButtonElement>(null)
  const {
    config,
    subdomain,
    locale,
    defaultLocale,
    enabledLocales,
    spaces,
    multilingualEnabled,
    basepath,
  } = useDocsContext()
  const params = useParams({ strict: false }) as {
    locale?: string
    spaceSlug?: string
    collectionSlug?: string
    articleKey?: string
    referenceSlug?: string
  }
  const search = useSearch({ strict: false }) as { q?: string; space?: string }
  const matches = useMatches()
  const routeLoaderData = getRouteLoaderData(matches)
  const routeArticle = routeLoaderData?.article ?? null
  const routeCollection = routeLoaderData?.collection ?? null
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const activeSpaceSlug = spaces.some((space) => space.slug === params.spaceSlug)
    ? params.spaceSlug
    : undefined
  const canonicalCollectionSlug = !activeSpaceSlug ? params.collectionSlug : undefined
  const canonicalArticleKey = params.articleKey
  const routeResolvedSpaceSlug = routeArticle?.space_slug || routeCollection?.space_slug || undefined
  const routeResolvedSpaceId = findSpaceId(spaces, routeResolvedSpaceSlug)
  const { theme, toggleTheme, canToggle } = useTheme(config.theme_mode)
  const { data: currentNavigation = [] } = useSpaceNavigation(
    subdomain,
    locale,
    activeSpaceSlug ?? '',
    multilingualEnabled,
  )
  const needsCrossSpaceLookup =
    !activeSpaceSlug &&
    !routeResolvedSpaceSlug &&
    (!!canonicalCollectionSlug || !!canonicalArticleKey)

  const currentLocaleNavigationQueries = useQueries({
    queries: needsCrossSpaceLookup
      ? spaces.map((space) => ({
          queryKey: queryKeys.spaces.navigation(subdomain, locale, space.slug),
          queryFn: async () =>
            unwrap(
              await helpCenterService.getSpaceNavigation(
                subdomain,
                locale,
                space.slug,
                multilingualEnabled,
              ),
            ),
          enabled: !!subdomain && !!locale && !!space.slug,
          staleTime: 60_000,
        }))
      : [],
  })

  const currentSpaceContext = useMemo(() => {
    if (routeResolvedSpaceSlug) {
      return {
        spaceId: routeResolvedSpaceId,
        spaceSlug: routeResolvedSpaceSlug,
        navigation: [] as NavItem[],
      }
    }

    if (activeSpaceSlug) {
      return {
        spaceId: findSpaceId(spaces, activeSpaceSlug),
        spaceSlug: activeSpaceSlug,
        navigation: currentNavigation,
      }
    }

    if (!needsCrossSpaceLookup) {
      return { spaceId: undefined, spaceSlug: undefined, navigation: [] as NavItem[] }
    }

    for (let index = 0; index < spaces.length; index += 1) {
      const navigation = currentLocaleNavigationQueries[index]?.data ?? []
      const matchesCanonicalArticle = canonicalArticleKey
        ? findArticleId(navigation, canonicalArticleKey)
        : undefined
      const matchesCollection = params.collectionSlug
        ? findCollectionId(navigation, params.collectionSlug)
        : undefined
      const matchesCanonicalCollection = canonicalCollectionSlug
        ? findCollectionId(navigation, canonicalCollectionSlug)
        : undefined
      if (
        matchesCanonicalArticle ||
        matchesCollection ||
        matchesCanonicalCollection
      ) {
        return {
          spaceId: spaces[index]?.id,
          spaceSlug: spaces[index]?.slug,
          navigation,
        }
      }
    }

    return { spaceId: undefined, spaceSlug: undefined, navigation: [] as NavItem[] }
  }, [
    activeSpaceSlug,
    canonicalArticleKey,
    canonicalCollectionSlug,
    currentLocaleNavigationQueries,
    currentNavigation,
    needsCrossSpaceLookup,
    params.collectionSlug,
    routeResolvedSpaceId,
    routeResolvedSpaceSlug,
    spaces,
  ])

  const currentSpaceId = currentSpaceContext.spaceId
  const currentResolvedSpaceSlug = currentSpaceContext.spaceSlug
  const currentResolvedNavigation = currentSpaceContext.navigation

  const sortedLinks = [...(config.header_links ?? [])].sort(
    (a, b) => (a.position ?? 0) - (b.position ?? 0),
  )

  const currentRouteState = useMemo<LocaleRouteState>(() => {
    if (pathname.endsWith('/search')) {
      return {
        kind: 'search',
        spaceId: findSpaceId(spaces, search.space),
        searchQuery: search.q,
      }
    }

    if (params.articleKey) {
      return {
        kind: 'article',
        spaceId: currentSpaceId,
        collectionId:
          routeArticle?.collection_id ??
          findCollectionId(currentResolvedNavigation, canonicalCollectionSlug),
        articleId:
          routeArticle?.id ??
          findArticleId(currentResolvedNavigation, params.articleKey),
      }
    }

    if (params.referenceSlug && params.spaceSlug) {
      return {
        kind: 'api_reference',
        spaceId: currentSpaceId,
        apiReferenceSlug: params.referenceSlug,
      }
    }

    if (canonicalCollectionSlug) {
      return {
        kind: 'collection',
        spaceId: currentSpaceId,
        collectionId:
          routeCollection?.collection.id ??
          findCollectionId(currentResolvedNavigation, canonicalCollectionSlug),
      }
    }

    if (params.spaceSlug) {
      return {
        kind: 'space',
        spaceId: currentSpaceId,
      }
    }

    return { kind: 'home' }
  }, [
    currentResolvedNavigation,
    currentSpaceId,
    canonicalArticleKey,
    canonicalCollectionSlug,
    params.articleKey,
    params.referenceSlug,
    params.collectionSlug,
    params.spaceSlug,
    pathname,
    routeArticle,
    routeCollection,
    search.q,
    search.space,
    spaces,
  ])

  const needsNavigationLookup =
    currentRouteState.kind === 'collection' || currentRouteState.kind === 'article'

  const localeSpaceQueries = useQueries({
    queries: enabledLocales.map((code) => ({
      queryKey: queryKeys.helpCenter.spaces(subdomain, code),
      queryFn: async () =>
        unwrap(await helpCenterService.getSpaces(subdomain, code, multilingualEnabled)),
      enabled:
        config.show_language_switcher &&
        multilingualEnabled &&
        !!subdomain &&
        !!code,
      staleTime: 60_000,
    })),
  })

  const localeNavigationQueries = useQueries({
    queries: enabledLocales.map((code, index) => {
      const localeSpaces = code === locale ? spaces : localeSpaceQueries[index]?.data ?? []
      const targetSpace = currentSpaceId
        ? localeSpaces.find((space) => space.id === currentSpaceId)
        : undefined

      return {
        queryKey: queryKeys.spaces.navigation(
          subdomain,
          code,
          targetSpace?.slug ?? '',
        ),
        queryFn: async () =>
          unwrap(
            await helpCenterService.getSpaceNavigation(
              subdomain,
              code,
              targetSpace!.slug,
              multilingualEnabled,
            ),
          ),
        enabled:
          config.show_language_switcher &&
          needsNavigationLookup &&
          !!subdomain &&
          !!targetSpace?.slug,
        staleTime: 60_000,
      }
    }),
  })

  const localeOptions = useMemo(() => {
    if (!config.show_language_switcher || enabledLocales.length <= 1) {
      return []
    }

    const defaultIndex = enabledLocales.indexOf(defaultLocale)
    const fallbackSpaces =
      defaultLocale === locale
        ? spaces
        : defaultIndex >= 0
          ? localeSpaceQueries[defaultIndex]?.data ?? []
          : []
    const fallbackNavigation =
      !needsNavigationLookup
        ? []
        : defaultLocale === locale
          ? currentResolvedNavigation
          : defaultIndex >= 0
            ? localeNavigationQueries[defaultIndex]?.data ?? []
            : []

    return enabledLocales.map((code, index) => {
      const targetSpaces = code === locale ? spaces : localeSpaceQueries[index]?.data ?? []
      const targetNavigation =
        !needsNavigationLookup
          ? []
          : code === locale
            ? currentResolvedNavigation
            : localeNavigationQueries[index]?.data ?? []

      return {
        code,
        label: getLocaleLabel(code),
        href: prefixBasepath(
          basepath,
          resolveLocaleSwitchPath({
            multilingualEnabled,
            targetLocale: code,
            defaultLocale,
            current: currentRouteState,
            targetSpaces,
            targetNavigation,
            fallbackSpaces,
            fallbackNavigation,
          }),
        ),
        active: code === locale,
      }
    })
  }, [
    config.show_language_switcher,
    basepath,
    currentResolvedNavigation,
    currentRouteState,
    defaultLocale,
    enabledLocales,
    locale,
    localeNavigationQueries,
    localeSpaceQueries,
    multilingualEnabled,
    needsNavigationLookup,
    spaces,
  ])

  useEffect(() => {
    const header = headerRef.current
    const leftContent = leftContentRef.current
    const rightContent = rightContentRef.current
    const searchButton = searchRef.current
    if (!header || !leftContent || !rightContent || !searchButton) return

    const updateSearchWidth = () => {
      const headerRect = header.getBoundingClientRect()
      const leftRect = leftContent.getBoundingClientRect()
      const rightRect = rightContent.getBoundingClientRect()
      const center = headerRect.left + headerRect.width / 2
      const clearance = 12
      const availableOnLeft = center - leftRect.right - clearance
      const availableOnRight = rightRect.left - center - clearance
      const centeredWidth = Math.floor(
        Math.min(520, availableOnLeft * 2, availableOnRight * 2),
      )

      // Keep the search action usable as an icon button when the header is very
      // crowded. The label truncates naturally and grows back as room returns.
      searchButton.style.width = `${Math.max(40, centeredWidth)}px`
    }

    updateSearchWidth()

    if (typeof ResizeObserver === 'undefined') {
      window.addEventListener('resize', updateSearchWidth)
      return () => window.removeEventListener('resize', updateSearchWidth)
    }

    const observer = new ResizeObserver(updateSearchWidth)
    observer.observe(header)
    observer.observe(leftContent)
    observer.observe(rightContent)

    return () => observer.disconnect()
  }, [])

  return (
    <header
      ref={headerRef}
      className="sticky top-0 z-30 grid grid-cols-[1fr_auto_1fr] items-center border-b border-border bg-background/95 backdrop-blur-sm px-5 h-[var(--hc-header-height)]"
    >
      {/* Left: Brand + Space tabs */}
      <div
        ref={leftContentRef}
        className="flex w-max min-w-0 items-center"
      >
        <DocsLink
          to={buildCanonicalHomePath(multilingualEnabled, locale)}
          className="flex items-center gap-2.5 shrink-0"
        >
          {config.brand_logo_url ? (
            <>
              <img
                src={config.brand_logo_url}
                alt={config.brand_name}
                className={cn(
                  'h-8 w-auto object-contain',
                  config.brand_logo_dark_url && 'dark:hidden',
                )}
              />
              {config.brand_logo_dark_url && (
                <img
                  src={config.brand_logo_dark_url}
                  alt={config.brand_name}
                  className="h-8 w-auto object-contain hidden dark:block"
                />
              )}
            </>
          ) : (
            <span className="font-semibold text-[15px] tracking-tight text-foreground">
              {config.brand_name || 'Docs'}
            </span>
          )}
        </DocsLink>

        {spaces.length > 1 && (
          <>
          <div className="h-5 w-px bg-border ml-5 mr-1.5 shrink-0" />
          <nav className="hidden sm:flex items-center gap-1">
            {spaces.map((space) => {
              const isActive = currentResolvedSpaceSlug === space.slug
              return (
                <DocsLink
                  key={space.id}
                  to={buildCanonicalSpacePath(
                    multilingualEnabled,
                    locale,
                    space.slug,
                  )}
                  className={cn(
                    'whitespace-nowrap px-3 py-1.5 text-[13.5px] rounded-md transition-colors',
                    isActive
                      ? 'text-foreground font-semibold'
                      : 'text-muted-foreground font-medium hover:text-foreground',
                  )}
                >
                  {space.icon && (
                    <PublicIcon
                      name={space.icon}
                      size={15}
                      className="mr-1.5 inline-block align-text-bottom"
                    />
                  )}
                  {space.name}
                </DocsLink>
              )
            })}
          </nav>
          </>
        )}
      </div>

      {/* Center: Search */}
      <button
        ref={searchRef}
        onClick={onSearchClick}
        className="flex min-w-0 items-center gap-2 pl-3 pr-2 h-8 rounded-lg border border-border bg-muted/40 hover:bg-muted/70 transition-[width,background-color] cursor-text w-[min(280px,calc(100vw-120px))] sm:w-[360px] lg:w-[440px] xl:w-[520px]"
      >
        <Search size={14} className="text-muted-foreground shrink-0" />
        <span className="min-w-0 flex-1 truncate text-left text-[13px] text-muted-foreground">
          {config.search_placeholder || 'Search...'}
        </span>
        <span className="hidden flex-none text-xs font-semibold text-muted-foreground sm:inline-flex">
          ⌘K
        </span>
      </button>

      {/* Right: Header links + Theme toggle */}
      <div
        ref={rightContentRef}
        className="flex w-max items-center justify-end justify-self-end gap-1"
      >
        {sortedLinks.length > 0 && (
          <nav className="hidden md:flex items-center gap-1">
            {sortedLinks.map((link, i) =>
              link.style === 'button' ? (
                <a
                  key={i}
                  href={link.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center px-3 py-1.5 text-[13px] font-medium rounded-md text-primary-foreground transition-[filter,transform] hover:brightness-95 active:translate-y-px"
                  style={{ backgroundColor: config.brand_color || 'var(--color-primary)' }}
                >
                  {link.label}
                </a>
              ) : (
                <a
                  key={i}
                  href={link.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="px-3 py-1.5 text-[13px] font-medium text-muted-foreground hover:text-foreground transition-colors rounded-md"
                >
                  {link.label}
                </a>
              ),
            )}
          </nav>
        )}

        <LocaleSwitcher currentLocale={locale} options={localeOptions} />

        {canToggle && (
          <div className="relative group ml-2">
            <button
              onClick={toggleTheme}
              className="inline-flex items-center justify-center h-8 w-8 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-colors"
              aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
            >
              {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
            </button>
            <span className="pointer-events-none absolute left-1/2 -translate-x-1/2 top-full mt-1.5 px-2 py-1 rounded-md bg-foreground text-background text-[11px] font-medium whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity duration-150">
              {theme === 'dark' ? 'Light mode' : 'Dark mode'}
            </span>
          </div>
        )}
      </div>
    </header>
  )
}
