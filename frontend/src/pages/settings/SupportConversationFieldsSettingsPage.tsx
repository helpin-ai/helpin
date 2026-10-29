import { CustomFieldsSettings } from '@/components/settings/fields/CustomFieldsSettings'
import { SettingsPageFrame } from './SettingsPageFrame'

export function SupportConversationFieldsSettingsPage() {
  return (
    <SettingsPageFrame section="support-conversation-fields">
      {({ workspaceId, permissions }) => (
        <CustomFieldsSettings
          workspaceId={workspaceId}
          objectType="conversation"
          editable={permissions.has('support.admin')}
          title="Fields"
          description="Teammates fill these in on a conversation. Request types choose which ones customers see on the portal."
          recordNoun={['conversation', 'conversations']}
        />
      )}
    </SettingsPageFrame>
  )
}
