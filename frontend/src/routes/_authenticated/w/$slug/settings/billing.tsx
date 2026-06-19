import { createFileRoute } from '@tanstack/react-router';
import OrganizationBilling from '@/pages/OrganizationBilling';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/billing')({
  component: BillingSettingsRoute,
});

function BillingSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <OrganizationBilling />
    </SettingsRouteViewport>
  );
}
