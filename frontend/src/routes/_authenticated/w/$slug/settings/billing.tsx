import { createFileRoute } from '@tanstack/react-router';
import { BillingSettingsPage } from '@/pages/settings/BillingSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/billing')({
  component: BillingSettingsRoute,
});

function BillingSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <BillingSettingsPage />
    </SettingsRouteViewport>
  );
}
