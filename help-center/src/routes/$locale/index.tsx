import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { LocalizedHomePage } from '@/components/home/LocalizedHomePage'
import { CollectionRouteView } from '@/components/routes/CollectionRouteView'
import { LoadingState } from '@/components/LoadingState'
import { useDocsContext } from '@/contexts/DocsContext'
import {
  buildCanonicalCollectionPath,
  buildCanonicalHomePath,
  isMultilingualEnabled,
} from '@/lib/locale'

export const Route = createFileRoute('/$locale/')({
  component: LocalizedHomeRoute,
})

function LocalizedHomeRoute() {
  const { defaultLocale, enabledLocales } = useDocsContext()
  const { locale: localeParam } = Route.useParams()
  const navigate = useNavigate()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)
  const isValidLocaleParam = enabledLocales.some(
    (enabledLocale) => enabledLocale.toLowerCase() === localeParam.toLowerCase(),
  )

  useEffect(() => {
    if (!isValidLocaleParam) {
      if (multilingualEnabled) {
        navigate({
          to: buildCanonicalCollectionPath(true, defaultLocale, localeParam),
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
      to: buildCanonicalHomePath(false, defaultLocale),
      replace: true,
    })
  }, [
    defaultLocale,
    isValidLocaleParam,
    localeParam,
    multilingualEnabled,
    navigate,
  ])

  if (!isValidLocaleParam && !multilingualEnabled) {
    return (
      <CollectionRouteView
        locale={defaultLocale}
        collectionOrSpaceSlug={localeParam}
        multilingualEnabled={false}
      />
    )
  }

  if (!isValidLocaleParam || !multilingualEnabled) {
    return <LoadingState message="Redirecting..." />
  }

  return <LocalizedHomePage />
}
