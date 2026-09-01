import { AutomationShell } from '@/components/automation/AutomationShell';
import { AutomationOverviewPanel } from '@/components/automation/AutomationOverviewPanel';
import { QuietEmptyState } from '@/components/design-system/quiet';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useTitle } from '@/hooks/useTitle';
import { buildAutomationActivityPath, buildAutomationFlowsPath } from '@/lib/automationUi';
import { useWorkspaceStore } from '@/stores/workspaceStore';

export function AutomationLibraryPage() {
  useTitle('Automation Trigger Catalog');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const workspaceSlug = workspace?.slug;
  const { data: access } = useWorkspaceAccess(workspaceId);
  const permissions = usePermissions(access);

  if (!workspaceId) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <AutomationShell
      title="Trigger Catalog"
      description="Browse every event surface your workspace can react to. Go to Flows to connect a trigger to an agent and action."
    >
      {!permissions.canManageSettings ? (
        <QuietEmptyState
          title="Trigger catalog unavailable"
          description="You do not have permission to view the workspace automation library. Ask a workspace administrator for access."
        />
      ) : (
        <div className="space-y-4">
          <AutomationOverviewPanel
            workspaceId={workspaceId}
            search={{ page: 1 }}
            onSearchChange={() => {}}
            sectionVisibility={{
              statusBar: false,
              triggerExecutions: false,
              builtIns: false,
              automationRules: false,
            }}
            pathOverrides={{
              activityBasePath: buildAutomationActivityPath(workspaceSlug),
              flowsBasePath: buildAutomationFlowsPath(workspaceSlug),
            }}
          />

        </div>
      )}
    </AutomationShell>
  );
}
