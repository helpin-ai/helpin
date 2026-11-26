import { createFileRoute, Navigate } from '@tanstack/react-router'
import Profile from '@/pages/Profile'
import SecuritySettings from '@/pages/SecuritySettings'
import AccountSettings from '@/pages/AccountSettings'
import NotificationSettings from '@/pages/NotificationSettings'
import { OrgGitConnectionsSettingsPage } from '@/pages/settings/OrgGitConnectionsSettingsPage'
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport'

type SettingsSearch = {
  slack?: string
  workflow?: string
  team?: string
}

export const Route = createFileRoute('/_authenticated/w/$slug/settings/$section')({
  component: SettingsSectionRoute,
  validateSearch: (search: Record<string, unknown>): SettingsSearch => ({
    slack: typeof search.slack === 'string' ? search.slack : undefined,
    workflow: typeof search.workflow === 'string' ? search.workflow : undefined,
    team: typeof search.team === 'string' ? search.team : undefined,
  }),
})

function SettingsSectionRoute() {
  const { slug, section } = Route.useParams()

  const { slack } = Route.useSearch()

  if (section === 'profile') {
    return (
      <SettingsRouteViewport>
        <Profile />
      </SettingsRouteViewport>
    )
  }

  if (section === 'security') {
    return (
      <SettingsRouteViewport>
        <SecuritySettings />
      </SettingsRouteViewport>
    )
  }

  if (section === 'account') {
    return (
      <SettingsRouteViewport>
        <AccountSettings />
      </SettingsRouteViewport>
    )
  }

  if (section === 'git-connections') {
    return (
      <SettingsRouteViewport>
        <OrgGitConnectionsSettingsPage />
      </SettingsRouteViewport>
    )
  }

  if (section === 'notifications') {
    return (
      <SettingsRouteViewport>
        <NotificationSettings slackRow={slack} />
      </SettingsRouteViewport>
    )
  }

  return <Navigate to="/w/$slug/settings/$section" params={{ slug, section: 'profile' }} replace />
}
