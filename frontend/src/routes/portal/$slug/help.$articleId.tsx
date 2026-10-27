import { createFileRoute } from '@tanstack/react-router'
import { CustomerPortalHelpArticlePage } from '@/components/customer-portal/CustomerPortal'

export const Route = createFileRoute('/portal/$slug/help/$articleId')({
  component: HelpArticleRoute,
})

function HelpArticleRoute() {
  const { articleId } = Route.useParams()
  return <CustomerPortalHelpArticlePage articleId={articleId} />
}
