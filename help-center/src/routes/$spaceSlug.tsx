import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { LoadingState } from '@/components/LoadingState'
import { LocalizedHomePage } from '@/components/home/LocalizedHomePage'
import { CollectionRouteView } from '@/components/routes/CollectionRouteView'
import { useDocsContext } from '@/contexts/DocsContext'
import {
  buildCanonicalCollectionPath,
  buildCanonicalHomePath,
  isMultilingualEnabled,
} from '@/lib/locale'

export const Route = createFileRoute('/$spaceSlug')({
  component: LegacySpaceRedirect,
})

function LegacySpaceRedirect() {
  const { spaceSlug } = Route.useParams()
  const { defaultLocale, enabledLocales } = useDocsContext()
  const navigate = useNavigate()
  const normalizedSlug = spaceSlug.trim().toLowerCase()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)
  const isKnownLocaleSlug = enabledLocales.some(
    (locale) => locale.toLowerCase() === normalizedSlug,
  )

  useEffect(() => {
    if (multilingualEnabled && isKnownLocaleSlug) {
      return
    }

    if (multilingualEnabled) {
      navigate({
        to: buildCanonicalCollectionPath(true, defaultLocale, spaceSlug),
        replace: true,
      })
      return
    }

    if (isKnownLocaleSlug) {
      navigate({
        to: buildCanonicalHomePath(false, defaultLocale),
        replace: true,
      })
    }
  }, [defaultLocale, isKnownLocaleSlug, multilingualEnabled, navigate, spaceSlug])

  if (multilingualEnabled && isKnownLocaleSlug) {
    return <LocalizedHomePage />
  }

  if (!multilingualEnabled && !isKnownLocaleSlug) {
    return (
      <CollectionRouteView
        locale={defaultLocale}
        collectionOrSpaceSlug={spaceSlug}
        multilingualEnabled={false}
      />
    )
  }

  return <LoadingState message="Redirecting..." />
}
