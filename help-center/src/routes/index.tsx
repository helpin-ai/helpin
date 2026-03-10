import { useState, useCallback } from 'react'
import { createFileRoute, Link } from '@tanstack/react-router'
import { Search, Menu, ArrowRight } from 'lucide-react'
import { useDocsContext } from '@/contexts/DocsContext'
import { useSpaceNavigation } from '@/hooks/queries'
import { Sidebar } from '@/components/layout/Sidebar'
import { MobileNav } from '@/components/navigation/MobileNav'
import { PhIcon } from '@/components/PhIcon'
import type { HomepageFeaturedCard } from '@/lib/types'

export const Route = createFileRoute('/')({
  component: HomePage,
})

function HomePage() {
  const { config, spaces, subdomain } = useDocsContext()
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const closeMobileNav = useCallback(() => setMobileNavOpen(false), [])

  // Load first space's navigation for the sidebar
  const firstSpace = spaces[0]
  const { data: navigation } = useSpaceNavigation(
    subdomain,
    firstSpace?.slug ?? '',
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

  // Use featured cards if configured, otherwise auto-generate from spaces
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
      {/* Mobile nav bar */}
      {firstSpace && (
        <div className="lg:hidden fixed top-[var(--hc-header-height)] left-0 right-0 z-20 flex items-center gap-2 px-4 py-2 border-b border-border bg-background">
          <button
            onClick={() => setMobileNavOpen(true)}
            className="inline-flex items-center justify-center h-7 w-7 -ml-1 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-colors"
            aria-label="Open navigation"
          >
            <Menu size={18} />
          </button>
          <span className="text-[13px] font-medium truncate">
            {firstSpace.name}
          </span>
        </div>
      )}

      {/* Mobile nav drawer */}
      {mobileNavOpen && firstSpace && (
        <MobileNav
          navigation={nav}
          spaceSlug={firstSpace.slug}
          onClose={closeMobileNav}
        />
      )}

      {/* Desktop sidebar */}
      {firstSpace && <Sidebar navigation={nav} spaceSlug={firstSpace.slug} />}

      {/* Main content */}
      <main className="flex-1 min-w-0 pt-[41px] lg:pt-0">
        <div className="w-full max-w-3xl mx-auto px-8 pt-24 pb-16">
          {/* Hero */}
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

          {/* Search */}
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

          {/* Featured Cards Grid */}
          {cards.length > 0 && (
            <div className="mt-12 grid grid-cols-1 gap-4 sm:grid-cols-2">
              {cards.map((card, i) => (
                <FeaturedCard key={i} card={card} />
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

function FeaturedCard({ card }: { card: HomepageFeaturedCard }) {
  const inner = (
    <>
      {card.icon && (
        <div className="mb-3 text-primary">
          <CardIcon name={card.icon} />
        </div>
      )}
      <h3 className="font-semibold text-foreground">{card.title}</h3>
      {card.description && (
        <p className="mt-1.5 text-[13.5px] leading-relaxed text-muted-foreground line-clamp-3">
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

  if (card.link_type === 'article') {
    return (
      <Link
        to="/$spaceSlug/$articleSlug"
        params={{ spaceSlug: card.space_slug, articleSlug: card.link_value }}
        className={cls}
      >
        {inner}
      </Link>
    )
  }

  // space or collection → navigate to space
  const spaceSlug =
    card.link_type === 'space' ? card.link_value : card.space_slug

  return (
    <Link to="/$spaceSlug" params={{ spaceSlug }} className={cls}>
      {inner}
    </Link>
  )
}
