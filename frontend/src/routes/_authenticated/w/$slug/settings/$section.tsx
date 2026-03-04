import { createFileRoute, Navigate } from '@tanstack/react-router'
import Settings, { isSettingsSection } from '@/pages/Settings'
import Profile from '@/pages/Profile'

export const Route = createFileRoute('/_authenticated/w/$slug/settings/$section')({
  component: SettingsSectionRoute,
})

function SettingsSectionRoute() {
  const { slug, section } = Route.useParams()

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
      <Settings section={section} />
    </div>
  )
}
