import { createFileRoute, redirect } from '@tanstack/react-router'
import { LoadingState } from '@/components/LoadingState'

export const Route = createFileRoute(
  '/$locale/$spaceSlug/$collectionSlug/$articleSlug',
)({
  beforeLoad: ({ params }) => {
    throw redirect({
      statusCode: 301,
      to: '/$locale/$spaceSlug/$collectionSlug',
      params: {
        locale: params.locale,
        spaceSlug: params.collectionSlug,
        collectionSlug: params.articleSlug,
      },
    })
  },
  component: LegacyLocalizedArticleRedirect,
})

function LegacyLocalizedArticleRedirect() {
  return <LoadingState message="Redirecting..." />
}
