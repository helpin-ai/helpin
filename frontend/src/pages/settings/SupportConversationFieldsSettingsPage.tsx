import { CustomFieldsSettings } from '@/components/settings/fields/CustomFieldsSettings'
import { RequestTypesSettings } from '@/components/settings/fields/RequestTypesSettings'
import { SettingsPageFrame } from './SettingsPageFrame'

export function SupportConversationFieldsSettingsPage() {
  return (
    <SettingsPageFrame section="support-conversation-fields">
      {({ workspaceId, permissions }) => (
        <div className="space-y-8">
        <RequestTypesSettings workspaceId={workspaceId} editable={permissions.has('support.admin')} />
        <CustomFieldsSettings
          workspaceId={workspaceId}
          objectType="conversation"
          editable={permissions.has('support.admin')}
          title="Fields"
          description="Teammates fill these in on a conversation. Request types choose which ones customers see on the portal."
          recordNoun={['conversation', 'conversations']}
        />
        </div>
      )}
    </SettingsPageFrame>
  )
}
