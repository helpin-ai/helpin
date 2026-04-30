import { createFileRoute } from '@tanstack/react-router';
import { MessageShortcutsSettingsPage } from '@/pages/settings/MessageShortcutsSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/message-shortcuts')({
  component: MessageShortcutsRoute,
});

function MessageShortcutsRoute() {
  return (
    <SettingsRouteViewport>
      <MessageShortcutsSettingsPage />
    </SettingsRouteViewport>
  );
}
