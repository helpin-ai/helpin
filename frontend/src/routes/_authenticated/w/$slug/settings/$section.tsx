import { createFileRoute, Navigate } from '@tanstack/react-router'
import Settings, { isSettingsSection } from '@/pages/Settings'
import Profile from '@/pages/Profile'
import AccountSettings from '@/pages/AccountSettings'
import NotificationSettings from '@/pages/NotificationSettings'

type SettingsSearch = {
  workflow?: string
  team?: string
}

export const Route = createFileRoute('/_authenticated/w/$slug/settings/$section')({
  component: SettingsSectionRoute,
  validateSearch: (search: Record<string, unknown>): SettingsSearch => ({
    workflow: typeof search.workflow === 'string' ? search.workflow : undefined,
    team: typeof search.team === 'string' ? search.team : undefined,
  }),
})

function SettingsSectionRoute() {
  const { slug, section } = Route.useParams()
  const { workflow, team } = Route.useSearch()

  if (section === 'profile') {
    return (
      <div className="h-full overflow-auto p-4 pb-32 md:p-6 md:pb-32">
        <Profile />
      </div>
    )
  }

  if (section === 'account') {
    return (
      <div className="h-full overflow-auto p-4 pb-32 md:p-6 md:pb-32">
        <AccountSettings />
      </div>
    )
  }

  if (section === 'notifications') {
    return (
      <div className="h-full overflow-auto p-4 pb-32 md:p-6 md:pb-32">
        <NotificationSettings />
      </div>
    )
  }

  if (!isSettingsSection(section)) {
    return <Navigate to="/w/$slug/settings/$section" params={{ slug, section: 'profile' }} replace />
  }

  return (
    <div className="h-full overflow-auto p-4 pb-32 md:p-6 md:pb-32">
      <Settings section={section} initialWorkflowId={workflow} initialTeamId={team} />
    </div>
  )
}
