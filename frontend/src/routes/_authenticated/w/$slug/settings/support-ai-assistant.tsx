import { createFileRoute } from '@tanstack/react-router';
import { SupportAIAssistantSettingsPage } from '@/pages/settings/SupportAIAssistantSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/support-ai-assistant')({
  component: SupportAIAssistantSettingsRoute,
});

function SupportAIAssistantSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <SupportAIAssistantSettingsPage />
    </SettingsRouteViewport>
  );
}
