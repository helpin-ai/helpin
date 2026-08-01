import { createFileRoute, Navigate } from '@tanstack/react-router';
import { MCPSettingsPage } from '@/pages/settings/MCPSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

type LegacyMCPSettingsSearch = {
  tab?: 'external' | 'ai-clients';
  external_mcp_oauth?: 'connected' | 'failed';
  external_mcp_server_id?: string;
  external_mcp_resume?: 'failed';
};

function optionalString(value: unknown) {
  return typeof value === 'string' && value.trim() ? value : undefined;
}

export const Route = createFileRoute('/_authenticated/w/$slug/settings/mcp')({
  validateSearch: (search: Record<string, unknown>): LegacyMCPSettingsSearch => ({
    tab: search.tab === 'external' || search.tab === 'ai-clients' ? search.tab : undefined,
    external_mcp_oauth: search.external_mcp_oauth === 'connected' || search.external_mcp_oauth === 'failed'
      ? search.external_mcp_oauth
      : undefined,
    external_mcp_server_id: optionalString(search.external_mcp_server_id),
    external_mcp_resume: search.external_mcp_resume === 'failed' ? 'failed' : undefined,
  }),
  component: MCPSettingsRoute,
});

function MCPSettingsRoute() {
  const { slug } = Route.useParams();
  const search = Route.useSearch();

  if (search.tab === 'external') {
    return (
      <Navigate
        to="/w/$slug/automation/tools/connections"
        params={{ slug }}
        search={{
          external_mcp_oauth: search.external_mcp_oauth,
          external_mcp_server_id: search.external_mcp_server_id,
          external_mcp_resume: search.external_mcp_resume,
        }}
        replace
      />
    );
  }

  if (search.tab === 'ai-clients') {
    return <Navigate to="/w/$slug/settings/mcp" params={{ slug }} search={{}} replace />;
  }

  return (
    <SettingsRouteViewport>
      <MCPSettingsPage />
    </SettingsRouteViewport>
  );
}
