import { createContext, useContext, useMemo, type ReactNode } from 'react'
import { useSpaceNavigation } from '@/hooks/queries'
import { useDocsContext } from './DocsContext'
import type { NavItem, Space } from '@/lib/types'
import { getArticlePager, type ArticlePagerLink } from '@/lib/navigation'

// ─── Context Value ──────────────────────────────────────────────────────────

interface SpaceContextValue {
  space: Space | undefined
  spaceSlug: string
  navigation: NavItem[]
  isLoading: boolean
  /** Get prev/next article links for the pager */
  getPager: (articleSlug: string) => { prev?: ArticlePagerLink; next?: ArticlePagerLink }
  /** Find the collection name for a given article slug */
  getCollectionName: (articleSlug: string) => string | undefined
  /** Find the collection slug for a given article slug */
  getCollectionSlug: (articleSlug: string) => string | undefined
}

const SpaceContext = createContext<SpaceContextValue | null>(null)

// ─── Provider ───────────────────────────────────────────────────────────────

export function SpaceProvider({
  spaceSlug,
  children,
}: {
  spaceSlug: string
  children: ReactNode
}) {
  const { subdomain, locale, spaces, multilingualEnabled } = useDocsContext()
  const { data: navigation, isLoading } = useSpaceNavigation(
    subdomain,
    locale,
    spaceSlug,
    multilingualEnabled,
  )

  const space = spaces.find((s) => s.slug === spaceSlug)

  const value = useMemo<SpaceContextValue>(
    () => {
      const nav = navigation ?? []

      return {
        space,
        spaceSlug,
        navigation: nav,
        isLoading,
        getPager: (articleSlug: string) => getArticlePager(nav, articleSlug),
        getCollectionName: (articleSlug: string) => {
          for (const collection of nav) {
            if (collection.articles.some((a) => a.slug === articleSlug)) {
              return collection.name
            }
          }
          return undefined
        },
        getCollectionSlug: (articleSlug: string) => {
          for (const collection of nav) {
            if (collection.articles.some((a) => a.slug === articleSlug)) {
              return collection.slug
            }
          }
          return undefined
        },
      }
    },
    [space, spaceSlug, navigation, isLoading],
  )

  return <SpaceContext.Provider value={value}>{children}</SpaceContext.Provider>
}

// ─── Hook ───────────────────────────────────────────────────────────────────

export function useSpaceContext() {
  const ctx = useContext(SpaceContext)
  if (!ctx) throw new Error('useSpaceContext must be used within SpaceProvider')
  return ctx
}
