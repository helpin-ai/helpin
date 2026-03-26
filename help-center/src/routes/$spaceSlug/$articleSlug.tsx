import { createFileRoute } from '@tanstack/react-router'
import { useEffect } from 'react'
import { useDocsContext } from '@/contexts/DocsContext'
import { LoadingState } from '@/components/LoadingState'
import { useNavigate } from '@tanstack/react-router'
import { useSpaceNavigation } from '@/hooks/queries'

export const Route = createFileRoute('/$spaceSlug/$articleSlug')({
  component: LegacyArticleRedirect,
})

function LegacyArticleRedirect() {
  const { spaceSlug, articleSlug } = Route.useParams()
  const { subdomain, defaultLocale } = useDocsContext()
  const navigate = useNavigate()
  const { data: navigation, isLoading } = useSpaceNavigation(
    subdomain,
    defaultLocale,
    spaceSlug,
  )

  useEffect(() => {
    if (!navigation) return

    const collection = navigation.find((item) =>
      item.articles.some((article) => article.slug === articleSlug),
    )

    if (collection) {
      navigate({
        to: '/$locale/$spaceSlug/$collectionSlug/$articleSlug',
        params: {
          locale: defaultLocale,
          spaceSlug,
          collectionSlug: collection.slug,
          articleSlug,
        },
        replace: true,
      })
      return
    }

    navigate({
      to: '/$locale/$spaceSlug',
      params: { locale: defaultLocale, spaceSlug },
      replace: true,
    })
  }, [articleSlug, defaultLocale, navigate, navigation, spaceSlug])

  if (isLoading) {
    return <LoadingState message="Redirecting..." />
  }

  return <LoadingState message="Redirecting..." />
}
