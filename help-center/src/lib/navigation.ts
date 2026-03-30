import type { NavItem } from './types'

export interface ArticlePagerLink {
  title: string
  slug: string
  collectionSlug: string
  collectionName: string
}

export function getArticlePager(
  navigation: NavItem[],
  currentArticleSlug: string,
): { prev?: ArticlePagerLink; next?: ArticlePagerLink } {
  const flat: ArticlePagerLink[] = []
  for (const collection of navigation) {
    for (const article of collection.articles) {
      flat.push({
        title: article.title,
        slug: article.slug,
        collectionSlug: collection.slug,
        collectionName: collection.name,
      })
    }
  }

  const idx = flat.findIndex((a) => a.slug === currentArticleSlug)
  if (idx === -1) return {}

  return {
    prev: idx > 0 ? flat[idx - 1] : undefined,
    next: idx < flat.length - 1 ? flat[idx + 1] : undefined,
  }
}
