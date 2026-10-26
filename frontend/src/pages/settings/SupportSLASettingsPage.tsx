import { SupportSLAPolicies } from '@/components/settings/sla/SupportSLAPolicies'
import { SettingsPageFrame } from './SettingsPageFrame'

export function SupportSLASettingsPage() {
  return (
    <SettingsPageFrame section="support-sla">
      {({ workspaceId, permissions }) => (
        <SupportSLAPolicies workspaceId={workspaceId} editable={permissions.has('support.admin')} />
      )}
    </SettingsPageFrame>
  )
}
