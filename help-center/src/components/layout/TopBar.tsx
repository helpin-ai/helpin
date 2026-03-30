import { useMemo } from 'react'
import { useQueries } from '@tanstack/react-query'
import { Search, Moon, Sun } from 'lucide-react'
import {
  useParams,
  useRouterState,
  useSearch,
} from '@tanstack/react-router'
import { cn } from '@/lib/utils'
import { useDocsContext } from '@/contexts/DocsContext'
import { useSpaceNavigation } from '@/hooks/queries'
import { useTheme } from '@/hooks/useTheme'
import { queryKeys } from '@/lib/queryKeys'
import { helpCenterService } from '@/lib/services'
import type { LocaleRouteState } from '@/lib/locale'
import {
  buildCanonicalCollectionPath,
  buildCanonicalHomePath,
  isMultilingualEnabled,
  resolveLocaleSwitchPath,
} from '@/lib/locale'
import { LocaleSwitcher } from './LocaleSwitcher'
import type { NavItem, Space } from '@/lib/types'

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
  return navigation.find((collection) => collection.slug === collectionSlug)?.id
}

function findArticleId(navigation: NavItem[], articleSlug?: string) {
  if (!articleSlug) return undefined
  for (const collection of navigation) {
    const article = collection.articles.find((candidate) => candidate.slug === articleSlug)
    if (article) return article.id
  }
  return undefined
}

function findSpaceId(spaces: Space[], spaceSlug?: string) {
  if (!spaceSlug) return undefined
  return spaces.find((space) => space.slug === spaceSlug)?.id
}

export function TopBar({ onSearchClick }: TopBarProps) {
  const { config, subdomain, locale, defaultLocale, enabledLocales, spaces } =
    useDocsContext()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)
  const params = useParams({ strict: false }) as {
    locale?: string
    spaceSlug?: string
    collectionSlug?: string
    articleSlug?: string
  }
  const search = useSearch({ strict: false }) as { q?: string; space?: string }
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const activeSpaceSlug = spaces.some((space) => space.slug === params.spaceSlug)
    ? params.spaceSlug
    : undefined
  const canonicalCollectionSlug = !activeSpaceSlug ? params.spaceSlug : undefined
  const canonicalArticleSlug = !activeSpaceSlug ? params.collectionSlug : undefined
  const { theme, toggleTheme, canToggle } = useTheme(config.theme_mode)
  const { data: currentNavigation = [] } = useSpaceNavigation(
    subdomain,
    locale,
    activeSpaceSlug ?? '',
  )
  const needsCrossSpaceLookup =
    !activeSpaceSlug && (!!canonicalCollectionSlug || !!canonicalArticleSlug)

  const currentLocaleNavigationQueries = useQueries({
    queries: needsCrossSpaceLookup
      ? spaces.map((space) => ({
          queryKey: queryKeys.spaces.navigation(subdomain, locale, space.slug),
          queryFn: async () =>
            unwrap(
              await helpCenterService.getSpaceNavigation(subdomain, locale, space.slug),
            ),
          enabled: !!subdomain && !!locale && !!space.slug,
          staleTime: 60_000,
        }))
      : [],
  })

  const currentSpaceContext = useMemo(() => {
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
      const matchesArticle = params.articleSlug
        ? findArticleId(navigation, params.articleSlug)
        : undefined
      const matchesCanonicalArticle = canonicalArticleSlug
        ? findArticleId(navigation, canonicalArticleSlug)
        : undefined
      const matchesCollection = params.collectionSlug
        ? findCollectionId(navigation, params.collectionSlug)
        : undefined
      const matchesCanonicalCollection = canonicalCollectionSlug
        ? findCollectionId(navigation, canonicalCollectionSlug)
        : undefined
      if (
        matchesArticle ||
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
    canonicalArticleSlug,
    canonicalCollectionSlug,
    currentLocaleNavigationQueries,
    currentNavigation,
    needsCrossSpaceLookup,
    params.articleSlug,
    params.collectionSlug,
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

    if (params.articleSlug) {
      return {
        kind: 'article',
        spaceId: currentSpaceId,
        collectionId: findCollectionId(currentResolvedNavigation, params.collectionSlug),
        articleId: findArticleId(currentResolvedNavigation, params.articleSlug),
      }
    }

    if (params.collectionSlug) {
      return {
        kind: 'collection',
        spaceId: currentSpaceId,
        collectionId: findCollectionId(currentResolvedNavigation, params.collectionSlug),
      }
    }

    if (canonicalArticleSlug) {
      return {
        kind: 'article',
        spaceId: currentSpaceId,
        collectionId: findCollectionId(currentResolvedNavigation, canonicalCollectionSlug),
        articleId: findArticleId(currentResolvedNavigation, canonicalArticleSlug),
      }
    }

    if (canonicalCollectionSlug) {
      return {
        kind: 'collection',
        spaceId: currentSpaceId,
        collectionId: findCollectionId(currentResolvedNavigation, canonicalCollectionSlug),
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
    canonicalArticleSlug,
    canonicalCollectionSlug,
    params.articleSlug,
    params.collectionSlug,
    params.spaceSlug,
    pathname,
    search.q,
    search.space,
    spaces,
  ])

  const needsNavigationLookup =
    currentRouteState.kind === 'collection' || currentRouteState.kind === 'article'

  const localeSpaceQueries = useQueries({
    queries: enabledLocales.map((code) => ({
      queryKey: queryKeys.helpCenter.spaces(subdomain, code),
      queryFn: async () => unwrap(await helpCenterService.getSpaces(subdomain, code)),
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
        href: resolveLocaleSwitchPath({
          multilingualEnabled,
          targetLocale: code,
          defaultLocale,
          current: currentRouteState,
          targetSpaces,
          targetNavigation,
          fallbackSpaces,
          fallbackNavigation,
        }),
        active: code === locale,
      }
    })
  }, [
    config.show_language_switcher,
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

  return (
    <header className="sticky top-0 z-30 grid grid-cols-[1fr_auto_1fr] items-center border-b border-border bg-background/95 backdrop-blur-sm px-5 h-[var(--hc-header-height)]">
      {/* Left: Brand + Space tabs */}
      <div className="flex items-center min-w-0">
        <a
          href={buildCanonicalHomePath(multilingualEnabled, locale)}
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
        </a>

        {spaces.length > 1 && (
          <>
          <div className="h-5 w-px bg-border ml-5 mr-1.5 shrink-0" />
          <nav className="hidden sm:flex items-center gap-1">
            {spaces.map((space) => {
              const isActive = currentResolvedSpaceSlug === space.slug
              return (
                <a
                  key={space.id}
                  href={buildCanonicalCollectionPath(
                    multilingualEnabled,
                    locale,
                    space.slug,
                  )}
                  className={cn(
                    'px-3 py-1.5 text-[13.5px] rounded-md transition-colors',
                    isActive
                      ? 'text-foreground font-semibold'
                      : 'text-muted-foreground font-medium hover:text-foreground',
                  )}
                >
                  {space.icon && <span className="mr-1.5">{space.icon}</span>}
                  {space.name}
                </a>
              )
            })}
          </nav>
          </>
        )}
      </div>

      {/* Center: Search */}
      <button
        onClick={onSearchClick}
        className="flex items-center gap-2 pl-3 pr-2 h-8 rounded-lg border border-border bg-muted/40 hover:bg-muted/70 transition-colors cursor-text w-[280px]"
      >
        <Search size={14} className="text-muted-foreground shrink-0" />
        <span className="flex-1 text-left text-[13px] text-muted-foreground">
          {config.search_placeholder || 'Search...'}
        </span>
        <kbd className="hidden sm:inline-flex items-center rounded-[4px] px-1.5 py-0.5 text-[10px] font-mono font-medium bg-background border border-border text-muted-foreground">
          ⌘K
        </kbd>
      </button>

      {/* Right: Header links + Theme toggle */}
      <div className="flex items-center justify-end gap-1">
        {sortedLinks.length > 0 && (
          <nav className="hidden md:flex items-center gap-1">
            {sortedLinks.map((link, i) =>
              link.style === 'button' ? (
                <a
                  key={i}
                  href={link.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center px-3 py-1.5 text-[13px] font-medium rounded-md transition-colors text-primary-foreground"
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
