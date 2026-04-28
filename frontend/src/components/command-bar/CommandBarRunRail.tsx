import { useCallback, useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { BotIcon, Cancel01Icon, Loading01Icon, Menu01Icon, Tick01Icon, ViewIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { ACTIVE_RUN_STATUSES, getAgentRunDisplayStatus, STATUS_META } from '@/components/pm/agentRunConstants';
import { agentService } from '@/lib/services/agentService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCommandBarRunStore } from '@/stores/commandBarStore';
import type { AgentRun } from '@/lib/pmTypes';

function targetLabel(run: AgentRun) {
  return run.target_info?.title || run.target_info?.task_key || `${run.target_type} ${run.target_id.slice(0, 8)}`;
}

export function CommandBarRunRail() {
  const workspaceId = useWorkspaceStore((s) => s.currentWorkspace?.id);
  const runIds = useCommandBarRunStore((s) => s.runIds);
  const runsById = useCommandBarRunStore((s) => s.runsById);
  const railOpen = useCommandBarRunStore((s) => s.railOpen);
  const updateRun = useCommandBarRunStore((s) => s.updateRun);
  const setRailOpen = useCommandBarRunStore((s) => s.setRailOpen);
  const clear = useCommandBarRunStore((s) => s.clear);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [busyRunId, setBusyRunId] = useState<string | null>(null);

  const runs = useMemo(
    () => runIds.map((id) => runsById[id]).filter(Boolean),
    [runIds, runsById],
  );
  const activeCount = runs.filter((run) => ACTIVE_RUN_STATUSES.has(run.status)).length;

  const refreshRun = useCallback(
    async (runId: string) => {
      if (!workspaceId || !runIds.includes(runId)) return;
      const res = await agentService.getRun(workspaceId, runId);
      if (res.data) updateRun(res.data);
    },
    [runIds, updateRun, workspaceId],
  );

  useEffect(() => {
    const handler = (event: Event) => {
      const detail = (event as CustomEvent<{ entity_id?: string }>).detail;
      if (detail?.entity_id) void refreshRun(detail.entity_id);
    };
    window.addEventListener('agent_run-created', handler);
    window.addEventListener('agent_run-updated', handler);
    return () => {
      window.removeEventListener('agent_run-created', handler);
      window.removeEventListener('agent_run-updated', handler);
    };
  }, [refreshRun]);

  const runAction = async (run: AgentRun, action: 'approve' | 'cancel') => {
    if (!workspaceId) return;
    setBusyRunId(run.id);
    try {
      const res = action === 'approve'
        ? await agentService.approveRun(workspaceId, run.id)
        : await agentService.cancelRun(workspaceId, run.id);
      if (res.error || !res.data) {
        toast.error(res.error ?? `Failed to ${action} run`);
        return;
      }
      updateRun(res.data);
    } finally {
      setBusyRunId(null);
    }
  };

  if (runs.length === 0) return null;

  if (!railOpen) {
    return (
      <Button
        type="button"
        size="sm"
        variant="outline"
        className="fixed right-4 top-20 z-40 h-9 gap-2 border-border/70 bg-background shadow-sm"
        onClick={() => setRailOpen(true)}
      >
        <Menu01Icon className="h-4 w-4" />
        {activeCount} active
      </Button>
    );
  }

  return (
    <>
      <aside className="fixed right-4 top-20 z-40 w-[min(24rem,calc(100vw-2rem))] overflow-hidden rounded-md border border-border/70 bg-background shadow-xl">
        <div className="flex items-center justify-between border-b px-3 py-2">
          <div className="flex items-center gap-2">
            <BotIcon className="h-4 w-4 text-muted-foreground" />
            <span className="text-sm font-medium">Command Runs</span>
            <Badge variant="outline" className="text-[10px]">{activeCount} active</Badge>
          </div>
          <div className="flex items-center gap-1">
            <Button type="button" variant="ghost" size="icon" className="h-7 w-7" onClick={() => setRailOpen(false)}>
              <Menu01Icon className="h-4 w-4" />
            </Button>
            <Button type="button" variant="ghost" size="icon" className="h-7 w-7" onClick={clear}>
              <Cancel01Icon className="h-4 w-4" />
            </Button>
          </div>
        </div>
        <div className="max-h-[min(28rem,calc(100vh-8rem))] overflow-y-auto p-2">
          {runs.map((run) => {
            const displayStatus = getAgentRunDisplayStatus(run);
            const meta = STATUS_META[displayStatus] ?? STATUS_META[run.status];
            const awaitingApproval = displayStatus === 'awaiting_approval';
            const busy = busyRunId === run.id;
            return (
              <div key={run.id} className="space-y-2 border-b border-border/60 px-2 py-2 last:border-0">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">{targetLabel(run)}</p>
                    <p className="truncate text-xs text-muted-foreground">
                      {run.target_type} • {run.execution_stage || run.id.slice(0, 8)}
                    </p>
                  </div>
                  <Badge variant={meta?.variant ?? 'outline'} className={meta?.className}>
                    {meta?.label ?? run.status}
                  </Badge>
                </div>
                <div className="flex items-center justify-end gap-1.5">
                  {awaitingApproval ? (
                    <Button type="button" variant="outline" size="sm" className="h-7 gap-1.5" disabled={busy} onClick={() => void runAction(run, 'approve')}>
                      {busy ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <Tick01Icon className="h-3.5 w-3.5" />}
                      Approve
                    </Button>
                  ) : null}
                  {ACTIVE_RUN_STATUSES.has(run.status) ? (
                    <Button type="button" variant="ghost" size="sm" className="h-7" disabled={busy} onClick={() => void runAction(run, 'cancel')}>
                      Cancel
                    </Button>
                  ) : null}
                  <Button type="button" variant="ghost" size="sm" className="h-7 gap-1.5" onClick={() => setSelectedRunId(run.id)}>
                    <ViewIcon className="h-3.5 w-3.5" />
                    Open
                  </Button>
                </div>
              </div>
            );
          })}
        </div>
      </aside>
      <CodingSessionDrawer
        sessionId={selectedRunId}
        open={!!selectedRunId}
        onOpenChange={(open) => {
          if (!open) setSelectedRunId(null);
        }}
        title="Command Run"
      />
    </>
  );
}
