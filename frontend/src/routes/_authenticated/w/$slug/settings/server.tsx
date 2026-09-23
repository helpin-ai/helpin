import { createFileRoute, Navigate } from '@tanstack/react-router';
import { systemStatusEnabled } from '@edition/config';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';
import { ServerSettingsPage } from '@/pages/settings/ServerSettingsPage';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/server')({
  component: ServerSettingsRoute,
});

function ServerSettingsRoute() {
  const { slug } = Route.useParams();
  // Signup and server admins are platform-managed outside Community.
  if (!systemStatusEnabled) return <Navigate to="/w/$slug/settings/general" params={{ slug }} replace />;
  return (
    <SettingsRouteViewport>
      <ServerSettingsPage />
    </SettingsRouteViewport>
  );
}
