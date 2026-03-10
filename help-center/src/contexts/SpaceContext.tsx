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
  const { subdomain, spaces } = useDocsContext()
  const { data: navigation, isLoading } = useSpaceNavigation(subdomain, spaceSlug)

  const space = spaces.find((s) => s.slug === spaceSlug)
  const nav = navigation ?? []

  const value = useMemo<SpaceContextValue>(
    () => ({
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
    }),
    [space, spaceSlug, nav, isLoading],
  )

  return <SpaceContext.Provider value={value}>{children}</SpaceContext.Provider>
}

// ─── Hook ───────────────────────────────────────────────────────────────────

export function useSpaceContext() {
  const ctx = useContext(SpaceContext)
  if (!ctx) throw new Error('useSpaceContext must be used within SpaceProvider')
  return ctx
}
