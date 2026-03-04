import { createFileRoute, Navigate } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/w/$slug/settings/')({
  component: SettingsIndex,
})

function SettingsIndex() {
  const { slug } = Route.useParams()
  return <Navigate to="/w/$slug/settings/$section" params={{ slug, section: 'profile' }} replace />
}
