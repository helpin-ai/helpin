import { createFileRoute } from '@tanstack/react-router';
import { CustomerPortalSettingsPage } from '@/pages/settings/CustomerPortalSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/customer-portal')({
  component: CustomerPortalSettingsRoute,
});

function CustomerPortalSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <CustomerPortalSettingsPage />
    </SettingsRouteViewport>
  );
}
