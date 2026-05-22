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
    <OrgGitConnectionsTab
      organizationId={currentOrganization?.id}
      workspaceId={workspaceId}
      canManage={canManage}
    />
  );
}
