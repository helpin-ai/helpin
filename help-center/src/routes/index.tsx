import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { LoadingState } from '@/components/LoadingState'
import { LocalizedHomePage } from '@/components/home/LocalizedHomePage'
import { useDocsContext } from '@/contexts/DocsContext'
import { buildCanonicalHomePath, isMultilingualEnabled } from '@/lib/locale'

export const Route = createFileRoute('/')({
  component: RootLocaleRedirect,
})

function RootLocaleRedirect() {
  const { defaultLocale, enabledLocales } = useDocsContext()
  const navigate = useNavigate()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)

  useEffect(() => {
    if (!multilingualEnabled) {
      return
    }
    navigate({
      to: buildCanonicalHomePath(true, defaultLocale),
      replace: true,
    })
  }, [defaultLocale, multilingualEnabled, navigate])

  if (!multilingualEnabled) {
    return <LocalizedHomePage />
  }

  return <LoadingState message="Redirecting..." />
}
