import { createFileRoute, Navigate } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/w/$slug/pm/')({
  component: PmIndex,
})

function PmIndex() {
  const { slug } = Route.useParams()
  return <Navigate to="/w/$slug/pm/my-work" params={{ slug }} replace />
}
