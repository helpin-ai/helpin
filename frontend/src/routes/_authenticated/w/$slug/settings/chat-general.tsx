import { createFileRoute } from '@tanstack/react-router';
import { ChatWidgetSettingsPage } from '@/pages/settings/ChatWidgetSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/chat-general')({
  component: ChatWidgetSettingsRoute,
});

function ChatWidgetSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <ChatWidgetSettingsPage />
    </SettingsRouteViewport>
  );
}
