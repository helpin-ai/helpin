import { createFileRoute } from '@tanstack/react-router'
import { TeamReplyPage } from '@/components/customer-portal/TeamReplyPage'

export const Route = createFileRoute('/team-reply/$token')({
  component: TeamReplyRoute,
})

function TeamReplyRoute() {
  const { token } = Route.useParams()
  return <TeamReplyPage token={token} />
}
