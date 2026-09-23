import { createFileRoute, Navigate } from '@tanstack/react-router';
import { systemStatusEnabled } from '@edition/config';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';
import { SystemStatusSettingsPage } from '@/pages/settings/SystemStatusSettingsPage';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/system-status')({
  component: SystemStatusSettingsRoute,
});

function SystemStatusSettingsRoute() {
  const { slug } = Route.useParams();
  // Server services are platform-managed outside Community.
  if (!systemStatusEnabled) return <Navigate to="/w/$slug/settings/general" params={{ slug }} replace />;
  return (
    <SettingsRouteViewport>
      <SystemStatusSettingsPage />
    </SettingsRouteViewport>
  );
}
