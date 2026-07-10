import { createFileRoute } from '@tanstack/react-router';
import { MCPSettingsPage } from '@/pages/settings/MCPSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/mcp')({
  component: MCPSettingsRoute,
});

function MCPSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <MCPSettingsPage />
    </SettingsRouteViewport>
  );
}
