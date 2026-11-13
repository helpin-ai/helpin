import { SupportChannels } from '@/components/settings/channels/SupportChannels'
import { SettingsPageFrame } from './SettingsPageFrame'

export function SupportChannelsSettingsPage() {
  return (
    <SettingsPageFrame section="support-channels">
      {({ workspaceId, permissions }) => (
        <SupportChannels workspaceId={workspaceId} editable={permissions.has('support.admin')} />
      )}
    </SettingsPageFrame>
  )
}
