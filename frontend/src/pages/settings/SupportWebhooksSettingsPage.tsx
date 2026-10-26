import { SupportWebhooks } from '@/components/settings/webhooks/SupportWebhooks'
import { SettingsPageFrame } from './SettingsPageFrame'

export function SupportWebhooksSettingsPage() {
  return (
    <SettingsPageFrame section="support-webhooks">
      {({ workspaceId, permissions }) => (
        <SupportWebhooks workspaceId={workspaceId} editable={permissions.has('support.admin')} />
      )}
    </SettingsPageFrame>
  )
}
