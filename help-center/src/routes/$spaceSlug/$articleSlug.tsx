import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { useDocsContext } from '@/contexts/DocsContext'
import { LoadingState } from '@/components/LoadingState'
import { ArticleRouteView } from '@/components/routes/ArticleRouteView'
import {
  buildCanonicalArticlePath,
  buildCanonicalCollectionPath,
  isMultilingualEnabled,
} from '@/lib/locale'

export const Route = createFileRoute('/$spaceSlug/$articleSlug')({
  component: LegacyArticleRedirect,
})

function LegacyArticleRedirect() {
  const { spaceSlug, articleSlug } = Route.useParams()
  const { defaultLocale, enabledLocales } = useDocsContext()
  const navigate = useNavigate()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)
  const normalizedSpaceSlug = spaceSlug.trim().toLowerCase()
  const isKnownLocaleSlug = enabledLocales.some(
    (locale) => locale.toLowerCase() === normalizedSpaceSlug,
  )

  useEffect(() => {
    if (multilingualEnabled) {
      navigate({
        to: buildCanonicalArticlePath(true, defaultLocale, spaceSlug, articleSlug),
        replace: true,
      })
      return
    }

    if (isKnownLocaleSlug) {
      navigate({
        to: buildCanonicalCollectionPath(false, defaultLocale, articleSlug),
        replace: true,
      })
    }
  }, [articleSlug, defaultLocale, isKnownLocaleSlug, multilingualEnabled, navigate, spaceSlug])

  if (!multilingualEnabled && !isKnownLocaleSlug) {
    return (
      <ArticleRouteView
        locale={defaultLocale}
        collectionSlug={spaceSlug}
        articleSlug={articleSlug}
        multilingualEnabled={false}
      />
    )
  }

  return <LoadingState message="Redirecting..." />
}
