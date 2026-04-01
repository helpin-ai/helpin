import { useMemo, useState } from 'react';
import { Bot, Loader2, MessageSquareMore, ShieldCheck } from 'lucide-react';

import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import type { AgentRun } from '@/lib/pmTypes';

import { timeAgo } from './helpers';

interface AgentRunsCardProps {
  workspaceId: string;
  agentRuns: AgentRun[];
  onApprove: (runId: string) => Promise<void>;
}

export function AgentRunsCard({ workspaceId: _workspaceId, agentRuns, onApprove }: AgentRunsCardProps) {
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [approvingRun, setApprovingRun] = useState<string | null>(null);

  const latestPendingRun = useMemo(
    () => agentRuns.find((run) => run.approval_state === 'pending') ?? null,
    [agentRuns],
  );

  const handleApprove = async (runId: string) => {
    setApprovingRun(runId);
    try {
      await onApprove(runId);
    } finally {
      setApprovingRun(null);
    }
  };

  if (agentRuns.length === 0) return null;

  return (
    <>
      <Card className="space-y-2 p-3">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <Bot className="h-4 w-4 text-muted-foreground" />
            <span className="text-sm font-medium">Agent Runs</span>
          </div>
          {latestPendingRun ? (
            <Button
              size="sm"
              variant="outline"
              className="h-7 gap-1 text-xs"
              disabled={approvingRun === latestPendingRun.id}
              onClick={() => void handleApprove(latestPendingRun.id)}
            >
              {approvingRun === latestPendingRun.id ? (
                <Loader2 className="h-3 w-3 animate-spin" />
              ) : (
                <ShieldCheck className="h-3 w-3" />
              )}
              Approve Draft
            </Button>
          ) : null}
        </div>

        <div className="space-y-2">
          {agentRuns.map((run) => (
            <button
              key={run.id}
              type="button"
              className="w-full rounded border border-border/60 px-2 py-2 text-left text-xs transition-colors hover:bg-muted/50"
              onClick={() => {
                setSelectedRunId(run.id);
                setDrawerOpen(true);
              }}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium">{run.status}</span>
                <div className="flex items-center gap-2">
                  {run.invocation_mode === 'interactive' ? (
                    <Badge variant="secondary" className="gap-1 text-[10px]">
                      <MessageSquareMore className="h-3 w-3" />
                      Interactive
                    </Badge>
                  ) : null}
                  <span className="text-muted-foreground">{run.approval_state}</span>
                </div>
              </div>
              <div className="mt-1 text-[10px] text-muted-foreground">
                {timeAgo(run.created_at)}
              </div>
            </button>
          ))}
        </div>
      </Card>

      <CodingSessionDrawer
        sessionId={selectedRunId}
        open={drawerOpen && !!selectedRunId}
        onOpenChange={setDrawerOpen}
        title="Support Agent Run"
      />
    </>
  );
}
