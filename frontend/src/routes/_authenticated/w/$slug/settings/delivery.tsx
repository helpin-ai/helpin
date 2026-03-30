import { createFileRoute } from '@tanstack/react-router';
import { DeliverySettingsPage } from '@/pages/settings/DeliverySettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/delivery')({
  component: DeliverySettingsRoute,
});

function DeliverySettingsRoute() {
  return (
    <SettingsRouteViewport>
      <DeliverySettingsPage />
    </SettingsRouteViewport>
  );
}
