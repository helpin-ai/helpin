import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { ArticleRouteView } from '@/components/routes/ArticleRouteView'
import { useDocsContext } from '@/contexts/DocsContext'
import { LoadingState } from '@/components/LoadingState'
import { buildCanonicalArticlePath, isMultilingualEnabled } from '@/lib/locale'

export const Route = createFileRoute('/$locale/$spaceSlug/$collectionSlug/')({
  component: LocalizedArticleOrLegacyCollectionRoute,
})

function LocalizedArticleOrLegacyCollectionRoute() {
  const { locale, spaceSlug, collectionSlug } = Route.useParams()
  const { defaultLocale, enabledLocales } = useDocsContext()
  const navigate = useNavigate()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)

  useEffect(() => {
    if (multilingualEnabled) {
      return
    }
    navigate({
      to: buildCanonicalArticlePath(false, defaultLocale, spaceSlug, collectionSlug),
      replace: true,
    })
  }, [collectionSlug, defaultLocale, multilingualEnabled, navigate, spaceSlug])

  if (!multilingualEnabled) {
    return <LoadingState message="Redirecting..." />
  }

  return (
    <ArticleRouteView
      locale={locale}
      collectionSlug={spaceSlug}
      articleSlug={collectionSlug}
      multilingualEnabled
    />
  )
}
