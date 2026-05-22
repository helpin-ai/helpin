import { OrgGitConnectionsTab } from '@/components/settings';
import { useTitle } from '@/hooks/useTitle';
import { useOrganizationStore } from '@/stores/organizationStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';

export function OrgGitConnectionsSettingsPage() {
  useTitle('Git Connections Settings');
  const currentOrganization = useOrganizationStore((state) => state.currentOrganization);
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspace?.id);
  const role = currentOrganization?.role;
  const canManage = role === 'owner' || role === 'admin';

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold">Git Connections</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          Organization provider access for repositories used across workspaces.
        </p>
      </div>
      <OrgGitConnectionsTab
        organizationId={currentOrganization?.id}
        workspaceId={workspaceId}
        canManage={canManage}
      />
    </div>
  );
}
