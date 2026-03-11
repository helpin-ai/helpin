import { useMemo } from 'react';
import { Bot, Clock, Loader2, CheckCircle2, XCircle, ShieldCheck, StopCircle } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { STATUS_META } from './agentRunConstants';
import { AgentRunArtifactView } from './AgentRunArtifactView';
import type { AgentRun, AgentRunArtifact } from '@/lib/pmTypes';
import { formatDistanceToNow, parseISO, differenceInSeconds } from 'date-fns';

interface Props {
  run: AgentRun;
  artifacts: AgentRunArtifact[];
  actingOnRun: string | null;
  onCancel: (runId: string) => void;
  onApprove: (runId: string) => void;
}

const STATUS_ICONS: Record<string, React.ReactNode> = {
  queued: <Clock className="h-3 w-3" />,
  running: <Loader2 className="h-3 w-3 animate-spin" />,
  awaiting_approval: <ShieldCheck className="h-3 w-3" />,
  completed: <CheckCircle2 className="h-3 w-3" />,
  failed: <XCircle className="h-3 w-3" />,
  cancelled: <XCircle className="h-3 w-3" />,
};

function formatDuration(startedAt?: string, completedAt?: string): string | null {
  if (!startedAt) return null;
  const start = parseISO(startedAt);
  const end = completedAt ? parseISO(completedAt) : new Date();
  const secs = differenceInSeconds(end, start);
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  const remSecs = secs % 60;
  return remSecs > 0 ? `${mins}m ${remSecs}s` : `${mins}m`;
}

function MetadataItem({ label, value }: { label: string; value?: string | null }) {
  if (!value) return null;
  return (
    <div className="min-w-0">
      <dt className="text-[10px] text-muted-foreground">{label}</dt>
      <dd className="truncate text-xs" title={value}>{value}</dd>
    </div>
  );
}

export function AgentRunDetail({ run, artifacts, actingOnRun, onCancel, onApprove }: Props) {
  const meta = STATUS_META[run.status] ?? STATUS_META.queued;
  const duration = formatDuration(run.started_at, run.completed_at);
  const isActive = run.status === 'queued' || run.status === 'running' || run.status === 'awaiting_approval';
  const acting = actingOnRun === run.id;

  const outputArtifacts = useMemo(
    () => artifacts.filter((a) => a.artifact_type === 'opencode_stdout' || a.artifact_type === 'opencode_stderr'),
    [artifacts],
  );
  const otherArtifacts = useMemo(
    () => artifacts.filter((a) => a.artifact_type !== 'opencode_stdout' && a.artifact_type !== 'opencode_stderr'),
    [artifacts],
  );

  const defaultTab = outputArtifacts.length > 0 ? 'output' : otherArtifacts.length > 0 ? 'artifacts' : 'output';

  return (
    <div className="p-3 space-y-3">
      {/* Header bar */}
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 min-w-0">
          <Badge variant={meta.variant} className="gap-1 px-1.5 py-0 text-[10px] shrink-0">
            {STATUS_ICONS[run.status]}
            {meta.label}
          </Badge>
          <span className="text-xs text-muted-foreground">
            {formatDistanceToNow(parseISO(run.created_at), { addSuffix: true })}
          </span>
          {duration && (
            <span className="text-xs text-muted-foreground">({duration})</span>
          )}
        </div>
        <div className="flex gap-1.5 shrink-0">
          {isActive && (
            <Button size="sm" variant="outline" className="h-7 gap-1 text-[11px]" disabled={acting} onClick={() => onCancel(run.id)}>
              {acting ? <Loader2 className="h-3 w-3 animate-spin" /> : <StopCircle className="h-3 w-3" />}
              Cancel
            </Button>
          )}
          {run.approval_state === 'pending' && (
            <Button size="sm" variant="outline" className="h-7 gap-1 text-[11px]" disabled={acting} onClick={() => onApprove(run.id)}>
              {acting ? <Loader2 className="h-3 w-3 animate-spin" /> : <ShieldCheck className="h-3 w-3" />}
              Approve
            </Button>
          )}
        </div>
      </div>

      {/* Metadata grid */}
      <dl className="grid grid-cols-2 md:grid-cols-3 gap-x-4 gap-y-1.5">
        <MetadataItem label="Runtime" value={run.runtime_kind} />
        <MetadataItem label="Pool" value={run.runner_pool} />
        <MetadataItem label="Stage" value={run.execution_stage} />
        <MetadataItem label="Approval" value={run.approval_state} />
        <MetadataItem label="Repo" value={run.repo_full_name} />
        <MetadataItem label="Branch" value={run.working_branch || run.base_branch} />
        <MetadataItem label="Heartbeat" value={run.last_heartbeat_at ? formatDistanceToNow(parseISO(run.last_heartbeat_at), { addSuffix: true }) : undefined} />
        <MetadataItem label="Tokens" value={run.tokens_used > 0 ? `${(run.tokens_used / 1000).toFixed(1)}k` : undefined} />
        {run.handoff_state && <MetadataItem label="Handoff" value={run.handoff_state} />}
      </dl>

      {/* Error message */}
      {run.error_message && (
        <div className="rounded border border-destructive/30 bg-destructive/5 p-2 text-xs text-destructive">
          {run.error_message}
        </div>
      )}

      {/* Tabbed artifacts */}
      {(outputArtifacts.length > 0 || otherArtifacts.length > 0) && (
        <Tabs defaultValue={defaultTab} className="w-full">
          <TabsList className="h-8">
            {outputArtifacts.length > 0 && (
              <TabsTrigger value="output" className="text-xs h-7">Output</TabsTrigger>
            )}
            {otherArtifacts.length > 0 && (
              <TabsTrigger value="artifacts" className="text-xs h-7">
                Artifacts ({otherArtifacts.length})
              </TabsTrigger>
            )}
          </TabsList>

          {outputArtifacts.length > 0 && (
            <TabsContent value="output" className="mt-2 space-y-2">
              {outputArtifacts.map((artifact) => (
                <AgentRunArtifactView key={artifact.id} artifact={artifact} maxContentHeight="max-h-[300px]" />
              ))}
            </TabsContent>
          )}

          {otherArtifacts.length > 0 && (
            <TabsContent value="artifacts" className="mt-2 space-y-2">
              {otherArtifacts.map((artifact) => (
                <AgentRunArtifactView key={artifact.id} artifact={artifact} />
              ))}
            </TabsContent>
          )}
        </Tabs>
      )}

      {artifacts.length === 0 && !run.error_message && (
        <p className="py-2 text-xs text-muted-foreground">No artifacts yet.</p>
      )}
    </div>
  );
}

export function AgentRunDetailEmpty() {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
      <Bot className="h-6 w-6 mb-2 opacity-40" />
      <p className="text-xs">Select a run to view details</p>
    </div>
  );
}
