import { useEffect } from 'react';
import { createFileRoute } from '@tanstack/react-router';
import { toast } from 'sonner';
import { ExternalToolsSettingsPage } from '@/pages/settings/ExternalToolsSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';
import { validateExternalToolsSearch } from '@/lib/externalToolsSearch';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/external-tools')({
  validateSearch: validateExternalToolsSearch,
  component: ExternalToolsRoute,
});

function ExternalToolsRoute() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();

  useEffect(() => {
    if (!search.external_mcp_oauth) return;

    if (search.external_mcp_popup === '1' && window.opener) {
      window.opener.postMessage({
        type: 'helpin.external-mcp.oauth',
        status: search.external_mcp_oauth,
        serverId: search.external_mcp_server_id,
      }, window.location.origin);
      window.close();
      return;
    }

    if (search.external_mcp_oauth === 'connected') {
      toast.success(search.external_mcp_resume === 'failed'
        ? 'Server connected. A paused run could not be resumed automatically.'
        : 'External MCP server connected');
    } else {
      toast.error('External MCP connection could not be completed. Check the server status for details.');
    }

    void navigate({ search: { tab: 'mcp' }, replace: true });
  }, [navigate, search.external_mcp_oauth, search.external_mcp_popup, search.external_mcp_resume, search.external_mcp_server_id]);

  return (
    <SettingsRouteViewport>
      <ExternalToolsSettingsPage
        requestedTab={search.external_mcp_oauth ? 'mcp' : search.tab}
        onTabChange={tab => void navigate({ search: { tab } })}
      />
    </SettingsRouteViewport>
  );
}
