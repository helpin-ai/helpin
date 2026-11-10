import { createFileRoute } from '@tanstack/react-router';
import { SupportConversationFieldsSettingsPage } from '@/pages/settings/SupportConversationFieldsSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/support-conversation-fields')({
  component: SupportConversationFieldsSettingsPageRoute,
});

function SupportConversationFieldsSettingsPageRoute() {
  return (
    <SettingsRouteViewport>
      <SupportConversationFieldsSettingsPage />
    </SettingsRouteViewport>
  );
}
