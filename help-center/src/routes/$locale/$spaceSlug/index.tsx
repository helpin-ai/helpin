import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { ArticleRouteView } from '@/components/routes/ArticleRouteView'
import { CollectionRouteView } from '@/components/routes/CollectionRouteView'
import { useDocsContext } from '@/contexts/DocsContext'
import { LoadingState } from '@/components/LoadingState'
import {
  buildCanonicalArticlePath,
  buildCanonicalCollectionPath,
  isMultilingualEnabled,
} from '@/lib/locale'

export const Route = createFileRoute('/$locale/$spaceSlug/')({
  component: LocalizedSpaceIndex,
})

function LocalizedSpaceIndex() {
  const { locale: localeParam, spaceSlug } = Route.useParams()
  const { defaultLocale, enabledLocales } = useDocsContext()
  const navigate = useNavigate()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)
  const isValidLocaleParam = enabledLocales.some(
    (enabledLocale) => enabledLocale.toLowerCase() === localeParam.toLowerCase(),
  )

  useEffect(() => {
    if (!isValidLocaleParam) {
      if (multilingualEnabled) {
        navigate({
          to: buildCanonicalArticlePath(true, defaultLocale, localeParam, spaceSlug),
          replace: true,
        })
        return
      }
      return
    }

    if (multilingualEnabled) {
      return
    }
    navigate({
      to: buildCanonicalCollectionPath(false, defaultLocale, spaceSlug),
      replace: true,
    })
  }, [
    defaultLocale,
    isValidLocaleParam,
    localeParam,
    multilingualEnabled,
    navigate,
    spaceSlug,
  ])

  if (!isValidLocaleParam && !multilingualEnabled) {
    return (
      <ArticleRouteView
        locale={defaultLocale}
        collectionSlug={localeParam}
        articleSlug={spaceSlug}
        multilingualEnabled={false}
      />
    )
  }

  if (!isValidLocaleParam || !multilingualEnabled) {
    return <LoadingState message="Redirecting..." />
  }

  return (
    <CollectionRouteView
      locale={localeParam}
      collectionOrSpaceSlug={spaceSlug}
      multilingualEnabled
    />
  )
}
