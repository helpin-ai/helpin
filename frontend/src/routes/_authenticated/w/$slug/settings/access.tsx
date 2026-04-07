import { createFileRoute } from '@tanstack/react-router'
import { AccessSettingsPage } from '@/pages/settings/AccessSettingsPage'
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport'

export const Route = createFileRoute('/_authenticated/w/$slug/settings/access')({
  component: AccessSettingsRoute,
})

function AccessSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <AccessSettingsPage />
    </SettingsRouteViewport>
  )
}
