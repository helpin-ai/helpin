import { useEffect, useState } from 'react';
import { createFileRoute } from '@tanstack/react-router';
import { toast } from 'sonner';
import { ExternalMCPAddButton, ExternalMCPConnections } from '@/components/automation/ExternalMCPConnections';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';

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
  useTitle('External MCP Connections');
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { canManageSettings } = usePermissions(access);
  const [createOpen, setCreateOpen] = useState(false);

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

    void navigate({ search: {}, replace: true });
  }, [navigate, search.external_mcp_oauth, search.external_mcp_popup, search.external_mcp_resume, search.external_mcp_server_id]);

  if (!workspaceId) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="space-y-4">
      {canManageSettings ? <div className="flex justify-end"><ExternalMCPAddButton workspaceId={workspaceId} onAdd={() => setCreateOpen(true)} /></div> : null}
      <ExternalMCPConnections
        workspaceId={workspaceId}
        workspaceName={workspace?.name ?? ''}
        canManageSettings={canManageSettings}
        createOpen={createOpen}
        onCreateOpenChange={setCreateOpen}
      />
    </div>
  );
}
