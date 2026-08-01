import { useEffect } from 'react';
import { createFileRoute } from '@tanstack/react-router';
import { toast } from 'sonner';
import { ExternalMCPConnections } from '@/components/automation/ExternalMCPConnections';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';

type ExternalMCPConnectionsSearch = {
  external_mcp_oauth?: 'connected' | 'failed';
  external_mcp_server_id?: string;
  external_mcp_resume?: 'failed';
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
  }),
  component: ExternalMCPConnectionsRoute,
});

function ExternalMCPConnectionsRoute() {
  useTitle('External MCP Connections');
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { canManageSettings } = usePermissions(access);

  useEffect(() => {
    if (!search.external_mcp_oauth) return;

    if (search.external_mcp_oauth === 'connected') {
      toast.success(search.external_mcp_resume === 'failed'
        ? 'Server connected. A paused run could not be resumed automatically.'
        : 'External MCP server connected');
    } else {
      toast.error('External MCP connection could not be completed. Check the server status for details.');
    }

    void navigate({ search: {}, replace: true });
  }, [navigate, search.external_mcp_oauth, search.external_mcp_resume]);

  if (!workspaceId) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <ExternalMCPConnections
      workspaceId={workspaceId}
      workspaceName={workspace?.name ?? ''}
      canManageSettings={canManageSettings}
    />
  );
}
