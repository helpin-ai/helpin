import type {
  ArticleDetail,
  CollectionPage,
  HelpCenterConfig,
  PreviewArticleDetail,
} from '@/lib/types'
import type { RootRouteData } from '@/lib/rootLoader'
import type { AlternateLink } from '@/lib/alternateLinks'
import {
  buildCanonicalArticlePath,
  buildCanonicalCollectionPath,
  buildCanonicalHomePath,
  buildCanonicalSearchPath,
} from '@/lib/locale'
import { prefixBasepath } from '@/lib/pathUtils'

function absoluteUrl(rootData: RootRouteData, path: string) {
  return new URL(prefixBasepath(rootData.basepath, path), rootData.origin).toString()
}

function createBaseMeta(title: string, description: string) {
  return [
    { title },
    { name: 'description', content: description },
    { property: 'og:title', content: title },
    { property: 'og:description', content: description },
    { property: 'twitter:card', content: 'summary_large_image' },
    { property: 'twitter:title', content: title },
    { property: 'twitter:description', content: description },
  ]
}

function createCanonicalLinks(canonicalUrl: string) {
  return [{ rel: 'canonical', href: canonicalUrl }]
}

function createHomeHreflangLinks(rootData: RootRouteData) {
  if (!rootData.multilingualEnabled) {
    return []
  }

  return [
    ...rootData.config.enabled_locales.map((locale) => ({
      rel: 'alternate' as const,
      hrefLang: locale,
      href: absoluteUrl(rootData, buildCanonicalHomePath(true, locale)),
    })),
    {
      rel: 'alternate' as const,
      hrefLang: 'x-default',
      href: absoluteUrl(rootData, buildCanonicalHomePath(true, rootData.config.default_locale)),
    },
  ]
}

export function buildHomeHead(rootData: RootRouteData) {
  const title = rootData.config.seo_title || `${rootData.config.brand_name} Help Center`
  const description =
    rootData.config.seo_description ||
    `Browse help articles and guides from ${rootData.config.brand_name}.`
  const canonicalUrl = absoluteUrl(
    rootData,
    buildCanonicalHomePath(rootData.multilingualEnabled, rootData.activeLocale),
  )

  return {
    links: [
      ...createCanonicalLinks(canonicalUrl),
      ...createHomeHreflangLinks(rootData),
    ],
    meta: [
      ...createBaseMeta(title, description),
      { property: 'og:type', content: 'website' },
      { property: 'og:url', content: canonicalUrl },
      { name: 'twitter:url', content: canonicalUrl },
    ],
    scripts: [
      {
        type: 'application/ld+json',
        children: JSON.stringify({
          '@context': 'https://schema.org',
          '@type': 'WebSite',
          name: rootData.config.brand_name,
          url: canonicalUrl,
          inLanguage: rootData.activeLocale,
        }),
      },
    ],
  }
}

export function buildCollectionHead(
  rootData: RootRouteData,
  collection: CollectionPage,
  fallbackSlug: string,
  alternateLinks: AlternateLink[] = [],
) {
  const title = `${collection.collection.name} | ${rootData.config.brand_name}`
  const description =
    `Browse articles in ${collection.collection.name} from ${rootData.config.brand_name}.`
  const canonicalUrl = absoluteUrl(
    rootData,
    buildCanonicalCollectionPath(
      rootData.multilingualEnabled,
      rootData.activeLocale,
      collection.collection.slug || fallbackSlug,
    ),
  )

  return {
    links: [...createCanonicalLinks(canonicalUrl), ...alternateLinks],
    meta: [
      ...createBaseMeta(title, description),
      { property: 'og:type', content: 'website' },
      { property: 'og:url', content: canonicalUrl },
      { name: 'twitter:url', content: canonicalUrl },
    ],
  }
}

export function buildArticleHead(
  rootData: RootRouteData,
  article: ArticleDetail,
  fallbackCollectionSlug: string,
  fallbackArticleSlug: string,
  alternateLinks: AlternateLink[] = [],
) {
  const title = article.seo_title || `${article.title} | ${rootData.config.brand_name}`
  const description =
    article.seo_description ||
    article.excerpt ||
    rootData.config.seo_description ||
    `Read ${article.title} in ${rootData.config.brand_name}.`
  const canonicalUrl = absoluteUrl(
    rootData,
    buildCanonicalArticlePath(
      rootData.multilingualEnabled,
      rootData.activeLocale,
      article.collection_slug || fallbackCollectionSlug,
      article.slug || fallbackArticleSlug,
    ),
  )

  return {
    links: [...createCanonicalLinks(canonicalUrl), ...alternateLinks],
    meta: [
      ...createBaseMeta(title, description),
      { property: 'og:type', content: 'article' },
      { property: 'og:url', content: canonicalUrl },
      { name: 'twitter:url', content: canonicalUrl },
    ],
    scripts: [
      {
        type: 'application/ld+json',
        children: JSON.stringify({
          '@context': 'https://schema.org',
          '@type': 'Article',
          headline: article.title,
          description,
          url: canonicalUrl,
          datePublished: article.published_at || undefined,
          inLanguage: rootData.activeLocale,
          publisher: {
            '@type': 'Organization',
            name: rootData.config.brand_name,
          },
        }),
      },
    ],
  }
}

export function buildNoIndexHead(
  title: string,
  description: string,
  canonicalUrl: string,
) {
  return {
    links: createCanonicalLinks(canonicalUrl),
    meta: [
      ...createBaseMeta(title, description),
      { name: 'robots', content: 'noindex, nofollow' },
      { property: 'og:url', content: canonicalUrl },
      { name: 'twitter:url', content: canonicalUrl },
    ],
  }
}

export function buildSearchHead(
  rootData: RootRouteData,
  query?: string,
  spaceSlug?: string,
) {
  const title = query
    ? `Search: ${query} | ${rootData.config.brand_name}`
    : `Search | ${rootData.config.brand_name}`
  const description = query
    ? `Search results for ${query} in ${rootData.config.brand_name}.`
    : `Search the ${rootData.config.brand_name} help center.`

  return buildNoIndexHead(
    title,
    description,
    absoluteUrl(
      rootData,
      buildCanonicalSearchPath(
        rootData.multilingualEnabled,
        rootData.activeLocale,
        query,
        spaceSlug,
      ),
    ),
  )
}

export function buildPreviewHead(
  rootData: RootRouteData,
  article?: PreviewArticleDetail | null,
) {
  const title = article
    ? `Preview: ${article.title} | ${rootData.config.brand_name}`
    : `Article Preview | ${rootData.config.brand_name}`
  const description = 'Preview mode is private and should not be indexed.'

  return buildNoIndexHead(
    title,
    description,
    absoluteUrl(rootData, '/preview'),
  )
}

export function buildRootHead(loaderData?: RootRouteData) {
  const config: HelpCenterConfig | undefined = loaderData?.config

  return {
    links: config?.favicon_url
      ? [{ rel: 'icon', href: config.favicon_url }]
      : undefined,
    meta: config?.brand_color
      ? [{ name: 'theme-color', content: config.brand_color }]
      : undefined,
  }
}
