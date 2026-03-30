import { createFileRoute } from '@tanstack/react-router';
import { AIAutomationsSettingsPage } from '@/pages/settings/AIAutomationsSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/ai-automations')({
  component: AIAutomationsSettingsRoute,
});

function AIAutomationsSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <AIAutomationsSettingsPage />
    </SettingsRouteViewport>
  );
}
