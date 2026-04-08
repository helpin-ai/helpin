import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Clock01Icon } from '@/lib/icons';
import { AutomationShell } from '@/components/automation/AutomationShell';
import { AutomationOverviewPanel } from '@/components/automation/AutomationOverviewPanel';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useTitle } from '@/hooks/useTitle';
import { automationService } from '@/lib/services/automationService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import type { Agent, AgentRun } from '@/lib/pmTypes';
import { buildAutomationActivityPath, buildAutomationFlowsPath, buildAutomationRunsPath } from '@/lib/automationUi';
import { useWorkspaceStore } from '@/stores/workspaceStore';

type AutomationActivitySearch = {
  page: number;
  agent_id?: string;
  binding_id?: string;
  trigger_type?: string;
  status?: string;
  source?: string;
  reference_id?: string;
  fired_after?: string;
  fired_before?: string;
};

function relativeTime(value?: string) {
  if (!value) return 'Unknown';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'Unknown';
  const diff = Date.now() - date.getTime();
  if (diff < 60_000) return 'just now';
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`;
  return `${Math.floor(diff / 86_400_000)}d ago`;
}

function RunPreviewRow({ run, agent }: { run: AgentRun; agent?: Agent }) {
  const statusTone = run.status === 'failed'
    ? 'destructive'
    : run.status === 'completed'
      ? 'outline'
      : 'secondary';

  return (
    <div className="flex flex-wrap items-start justify-between gap-3 rounded-md border border-border/60 bg-card/70 px-4 py-3">
      <div className="space-y-1">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-sm font-medium">{agent?.name ?? 'Agent'}</p>
          <Badge variant={statusTone} className="text-[10px] capitalize">{run.status}</Badge>
          <Badge variant="outline" className="text-[10px]">{run.invocation_mode}</Badge>
        </div>
        <p className="text-xs text-muted-foreground">
          {run.target_type}{run.execution_stage ? ` · ${run.execution_stage}` : ''}
        </p>
      </div>
      <div className="space-y-1 text-right">
        <p className="text-sm">{relativeTime(run.created_at)}</p>
        <p className="text-xs text-muted-foreground">
          {run.tokens_used > 0 ? `${(run.tokens_used / 1000).toFixed(1)}k tokens` : 'No tokens yet'}
        </p>
      </div>
    </div>
  );
}

export function AutomationActivityPage({
  search,
  onSearchChange,
}: {
  search: AutomationActivitySearch;
  onSearchChange: (updates: Partial<AutomationActivitySearch>) => void;
}) {
  useTitle('Automation Activity');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const workspaceSlug = workspace?.slug;
  const { data: access } = useWorkspaceAccess(workspaceId);
  const permissions = usePermissions(access);
  const runsQuery = useQuery({
    queryKey: queryKeys.automation.runs(workspaceId, 1, 8),
    queryFn: async () => unwrap(await automationService.listWorkspaceRuns(workspaceId, 1, 8)),
    enabled: !!workspaceId && permissions.canManageSettings,
    staleTime: 30_000,
  });
  const agentsQuery = useQuery({
    queryKey: queryKeys.automation.agents(workspaceId),
    queryFn: async () => unwrap(await automationService.listAgents(workspaceId)),
    enabled: !!workspaceId && permissions.canManageSettings,
    staleTime: 30_000,
  });

  const agentsById = useMemo(
    () => new Map((agentsQuery.data ?? []).map((agent) => [agent.id, agent])),
    [agentsQuery.data],
  );

  if (!workspaceId) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <AutomationShell
      title="Automation Activity"
      description="See what triggered, what matched, what launched, and what each agent run did."
    >
      {!permissions.canManageSettings ? (
        <Card className="border-border/60 bg-card/80">
          <CardContent className="px-5 py-6 text-sm text-muted-foreground">
            You do not have permission to view workspace-wide automation activity.
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          <AutomationOverviewPanel
            workspaceId={workspaceId}
            search={search}
            onSearchChange={onSearchChange}
            sectionVisibility={{
              triggerCatalog: false,
            }}
            pathOverrides={{
              activityBasePath: buildAutomationActivityPath(workspaceSlug),
              flowsBasePath: buildAutomationFlowsPath(workspaceSlug),
              agentRunsBasePath: buildAutomationRunsPath(workspaceSlug),
            }}
          />

          <Card className="border-border/60 bg-card/80">
            <CardHeader className="pb-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <Clock01Icon className="h-4 w-4 text-muted-foreground" />
                    <CardTitle className="text-base">Recent agent runs</CardTitle>
                    <Badge variant="outline" className="text-[10px]">{runsQuery.data?.data.length ?? 0}</Badge>
                  </div>
                  <p className="text-sm text-muted-foreground">
                    Agent runs started by automations in this workspace.
                  </p>
                </div>
                <a href={buildAutomationRunsPath(workspaceSlug)} className="text-sm text-muted-foreground hover:text-foreground">
                  Open full run history
                </a>
              </div>
            </CardHeader>
            <CardContent className="space-y-3">
              {runsQuery.isLoading || agentsQuery.isLoading ? (
                <Skeleton className="h-36 w-full rounded-xl" />
              ) : runsQuery.data?.data?.length ? (
                runsQuery.data.data.map((run) => (
                  <RunPreviewRow key={run.id} run={run} agent={agentsById.get(run.agent_id)} />
                ))
              ) : (
                <div className="rounded-md border border-dashed border-border/70 px-6 py-8 text-center text-sm text-muted-foreground">
                  No agent runs yet.
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      )}
    </AutomationShell>
  );
}
