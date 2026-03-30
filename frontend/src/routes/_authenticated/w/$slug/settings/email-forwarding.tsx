import { createFileRoute } from '@tanstack/react-router';
import { EmailForwardingSettingsPage } from '@/pages/settings/EmailForwardingSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/email-forwarding')({
  component: EmailForwardingSettingsRoute,
});

function EmailForwardingSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <EmailForwardingSettingsPage />
    </SettingsRouteViewport>
  );
}
