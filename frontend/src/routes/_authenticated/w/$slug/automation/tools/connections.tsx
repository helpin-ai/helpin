import { createFileRoute, Navigate } from '@tanstack/react-router';

type ExternalMCPConnectionsSearch = {
  external_mcp_oauth?: 'connected' | 'failed';
  external_mcp_server_id?: string;
  external_mcp_resume?: 'failed';
  external_mcp_popup?: '1';
};

function optionalString(value: unknown) {
  return typeof value === 'string' && value.trim() ? value : undefined;
}

export const Route = createFileRoute('/_authenticated/w/$slug/automation/tools/connections')({
  validateSearch: (search: Record<string, unknown>): ExternalMCPConnectionsSearch => ({
    external_mcp_oauth: search.external_mcp_oauth === 'connected' || search.external_mcp_oauth === 'failed'
      ? search.external_mcp_oauth
      : undefined,
    external_mcp_server_id: optionalString(search.external_mcp_server_id),
    external_mcp_resume: search.external_mcp_resume === 'failed' ? 'failed' : undefined,
    external_mcp_popup: search.external_mcp_popup === '1' ? '1' : undefined,
  }),
  component: ExternalMCPConnectionsRoute,
});

function ExternalMCPConnectionsRoute() {
  const { slug } = Route.useParams();
  const search = Route.useSearch();

  return (
    <Navigate
      to="/w/$slug/settings/external-mcp"
      params={{ slug }}
      search={search}
      replace
    />
  );
}
