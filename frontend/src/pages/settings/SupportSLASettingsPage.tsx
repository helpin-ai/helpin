import { SupportAIPrioritySetting } from '@/components/settings/sla/SupportAIPrioritySetting'
import { SupportSLAPolicies } from '@/components/settings/sla/SupportSLAPolicies'
import { SettingsPageFrame } from './SettingsPageFrame'

export function SupportSLASettingsPage() {
  return (
    <SettingsPageFrame section="support-sla">
      {({ workspaceId, permissions }) => (
        <div className="space-y-6">
          <SupportAIPrioritySetting workspaceId={workspaceId} editable={permissions.has('support.admin')} />
          <SupportSLAPolicies workspaceId={workspaceId} editable={permissions.has('support.admin')} />
        </div>
      )}
    </SettingsPageFrame>
  )
}
