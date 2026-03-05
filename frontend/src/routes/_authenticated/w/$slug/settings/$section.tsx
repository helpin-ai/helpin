import { createFileRoute, Navigate } from '@tanstack/react-router'
import Settings, { isSettingsSection } from '@/pages/Settings'
import Profile from '@/pages/Profile'

type SettingsSearch = {
  workflow?: string
}

export const Route = createFileRoute('/_authenticated/w/$slug/settings/$section')({
  component: SettingsSectionRoute,
  validateSearch: (search: Record<string, unknown>): SettingsSearch => ({
    workflow: typeof search.workflow === 'string' ? search.workflow : undefined,
  }),
})

function SettingsSectionRoute() {
  const { slug, section } = Route.useParams()
  const { workflow } = Route.useSearch()

  if (section === 'profile') {
    return (
      <div className="h-full overflow-auto p-4 md:p-6">
        <Profile />
      </div>
    )
  }

  if (!isSettingsSection(section)) {
    return <Navigate to="/w/$slug/settings/$section" params={{ slug, section: 'profile' }} replace />
  }

  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <Settings section={section} initialWorkflowId={workflow} />
    </div>
  )
}
