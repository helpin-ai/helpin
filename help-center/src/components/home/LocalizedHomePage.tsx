import { useState, useCallback } from 'react'
import { Search, Menu, ArrowRight } from 'lucide-react'
import { useDocsContext } from '@/contexts/DocsContext'
import { useSpaceNavigation } from '@/hooks/queries'
import {
  buildCanonicalCollectionPath,
  buildCanonicalSpacePath,
} from '@/lib/locale'
import { DocsLink } from '@/components/DocsLink'
import { Sidebar } from '@/components/layout/Sidebar'
import { MobileNav } from '@/components/navigation/MobileNav'
import { PhIcon } from '@/components/PhIcon'
import type { HomepageFeaturedCard } from '@/lib/types'

export function LocalizedHomePage() {
  const { config, spaces, subdomain, locale, multilingualEnabled } =
    useDocsContext()
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const closeMobileNav = useCallback(() => setMobileNavOpen(false), [])

  const firstSpace = spaces[0]
  const { data: navigation } = useSpaceNavigation(
    subdomain,
    locale,
    firstSpace?.slug ?? '',
    multilingualEnabled,
  )
  const nav = navigation ?? []

  const homepage = config.homepage_config
  const heroTitle =
    homepage?.hero_title || `${config.brand_name} documentation`
  const heroSubtitle =
    homepage?.hero_subtitle ||
    'Search our knowledge base or browse topics below'
  const searchPlaceholder =
    config.search_placeholder || 'Search documentation...'

  const cards: HomepageFeaturedCard[] =
    homepage?.featured_cards && homepage.featured_cards.length > 0
      ? homepage.featured_cards
      : spaces.map((space) => ({
          title: space.name,
          description: space.description || '',
          icon: space.icon || '',
          link_type: 'space' as const,
          link_value: space.slug,
          space_slug: '',
        }))

  const openSearch = () => {
    window.dispatchEvent(new Event('open-help-search'))
  }

  return (
    <div className="flex">
      {firstSpace && (
        <div className="fixed left-0 right-0 top-[var(--hc-header-height)] z-20 flex items-center gap-2 border-b border-border bg-background px-4 py-2 lg:hidden">
          <button
            onClick={() => setMobileNavOpen(true)}
            className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
            aria-label="Open navigation"
          >
            <Menu size={18} />
          </button>
          <span className="truncate text-[13px] font-medium">
            {firstSpace.name}
          </span>
        </div>
      )}

      {mobileNavOpen && firstSpace && (
        <MobileNav
          locale={locale}
          navigation={nav}
          onClose={closeMobileNav}
        />
      )}

      {firstSpace && (
        <Sidebar locale={locale} navigation={nav} />
      )}

      <main className="min-w-0 flex-1 pt-[41px] lg:pt-0">
        <div className="mx-auto w-full max-w-3xl px-8 pb-16 pt-24">
          <div className="text-center">
            <h1 className="text-3xl font-bold tracking-tight text-foreground sm:text-4xl">
              {heroTitle}
            </h1>
            {heroSubtitle && (
              <p className="mt-3 text-base text-muted-foreground sm:text-lg">
                {heroSubtitle}
              </p>
            )}
          </div>

          <button
            onClick={openSearch}
            className="mx-auto mt-8 flex w-full max-w-xl items-center gap-3 rounded-xl border border-border bg-muted/40 px-5 py-3.5 text-left transition-colors hover:bg-muted/60"
          >
            <Search size={18} className="shrink-0 text-muted-foreground" />
            <span className="flex-1 text-[15px] text-muted-foreground">
              {searchPlaceholder}
            </span>
            <ArrowRight
              size={18}
              className="shrink-0 text-muted-foreground"
            />
          </button>

          {cards.length > 0 && (
            <div className="mt-12 grid grid-cols-1 gap-4 sm:grid-cols-2">
              {cards.map((card, i) => (
                <FeaturedCard
                  key={i}
                  locale={locale}
                  card={card}
                  multilingualEnabled={multilingualEnabled}
                />
              ))}
            </div>
          )}
        </div>
      </main>
    </div>
  )
}

function CardIcon({ name }: { name: string }) {
  return <PhIcon name={name} size={36} weight="duotone" />
}

function FeaturedCard({
  locale,
  card,
  multilingualEnabled,
}: {
  locale: string
  card: HomepageFeaturedCard
  multilingualEnabled: boolean
}) {
  const inner = (
    <>
      {card.icon && (
        <div className="mb-3 text-primary">
          <CardIcon name={card.icon} />
        </div>
      )}
      <h3 className="font-semibold text-foreground">{card.title}</h3>
      {card.description && (
        <p className="mt-1.5 line-clamp-3 text-[13.5px] leading-relaxed text-muted-foreground">
          {card.description}
        </p>
      )}
    </>
  )

  const cls =
    'block rounded-xl border border-border bg-card p-6 transition-all hover:border-primary/30 hover:shadow-sm'

  if (card.link_type === 'url') {
    return (
      <a
        href={card.link_value}
        target="_blank"
        rel="noopener noreferrer"
        className={cls}
      >
        {inner}
      </a>
    )
  }

  if (card.link_type === 'space') {
    return (
      <DocsLink
        to={buildCanonicalSpacePath(
          multilingualEnabled,
          locale,
          card.link_value,
        )}
        className={cls}
      >
        {inner}
      </DocsLink>
    )
  }

  if (card.link_type === 'collection' && card.space_slug) {
    return (
      <DocsLink
        to={buildCanonicalCollectionPath(
          multilingualEnabled,
          locale,
          card.link_value,
        )}
        className={cls}
      >
        {inner}
      </DocsLink>
    )
  }

  const spaceSlug = card.space_slug || card.link_value
  return (
    <DocsLink
      to={buildCanonicalSpacePath(
        multilingualEnabled,
        locale,
        spaceSlug,
      )}
      className={cls}
    >
      {inner}
    </DocsLink>
  )
}
