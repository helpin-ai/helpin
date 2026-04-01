import { Clock, Loader2, CheckCircle2, KeyRound, MessageSquareMore, XCircle, ShieldCheck } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { AGENT_RUNTIME_LABELS } from '@/lib/agentRuntime';
import { TABLE_HEADER, TABLE_HEADER_CELL, TABLE_ROW, TABLE_CELL } from '@/lib/tableStyles';
import { getAgentRunDisplayStatus, STATUS_META } from './agentRunConstants';
import type { AgentRun } from '@/lib/pmTypes';
import { formatDistanceToNow, parseISO } from 'date-fns';

interface Props {
  runs: AgentRun[];
  selectedRunId: string | null;
  onSelectRun: (run: AgentRun) => void;
  loading: boolean;
}

const STATUS_ICONS: Record<string, React.ReactNode> = {
  queued: <Clock className="h-3 w-3" />,
  running: <Loader2 className="h-3 w-3 animate-spin" />,
  awaiting_input: <MessageSquareMore className="h-3 w-3" />,
  awaiting_approval: <ShieldCheck className="h-3 w-3" />,
  awaiting_auth: <KeyRound className="h-3 w-3" />,
  completed: <CheckCircle2 className="h-3 w-3" />,
  failed: <XCircle className="h-3 w-3" />,
  cancelled: <XCircle className="h-3 w-3" />,
};

export function AgentRunTable({ runs, selectedRunId, onSelectRun, loading }: Props) {
  if (loading) {
    return (
      <div className="flex items-center gap-2 px-3 py-4 text-xs text-muted-foreground">
        <Loader2 className="h-3.5 w-3.5 animate-spin" />
        Loading runs...
      </div>
    );
  }

  if (runs.length === 0) {
    return (
      <p className="px-3 py-4 text-xs text-muted-foreground">No runs yet. Click "Run Agent" to start.</p>
    );
  }

  return (
    <div>
      {/* Header */}
      <div className={`${TABLE_HEADER} flex`}>
        <div className={TABLE_HEADER_CELL} style={{ width: 120 }}>Status</div>
        <div className={TABLE_HEADER_CELL} style={{ width: 90 }}>Runtime</div>
        <div className={TABLE_HEADER_CELL} style={{ flex: '1 1 0%', minWidth: 100 }}>Branch</div>
        <div className={TABLE_HEADER_CELL} style={{ width: 100 }}>Time</div>
        <div className={`${TABLE_HEADER_CELL} text-right`} style={{ width: 70 }}>Tokens</div>
      </div>

      {/* Rows */}
      {runs.map((run) => {
        const displayStatus = getAgentRunDisplayStatus(run);
        const meta = STATUS_META[displayStatus] ?? STATUS_META.queued;
        const isSelected = run.id === selectedRunId;
        const branch = run.working_branch || run.base_branch || '';

        return (
          <button
            key={run.id}
            onClick={() => onSelectRun(run)}
            className={`${TABLE_ROW} w-full cursor-pointer text-left text-xs ${
              isSelected ? 'border-l-2 border-l-primary bg-accent/30' : 'border-l-2 border-l-transparent'
            }`}
          >
            <div className={TABLE_CELL} style={{ width: 120 }}>
              <Badge variant={meta.variant} className="gap-1 px-1.5 py-0 text-[10px]">
                {STATUS_ICONS[displayStatus]}
                {meta.label}
              </Badge>
            </div>
            <div className={`${TABLE_CELL} text-muted-foreground`} style={{ width: 90 }}>
              <span className="truncate">{AGENT_RUNTIME_LABELS[run.runtime_kind] ?? run.runtime_kind}</span>
            </div>
            <div className={`${TABLE_CELL} text-muted-foreground`} style={{ flex: '1 1 0%', minWidth: 100 }}>
              <span className="truncate" title={branch}>{branch || '-'}</span>
            </div>
            <div className={`${TABLE_CELL} text-muted-foreground`} style={{ width: 100 }}>
              {formatDistanceToNow(parseISO(run.created_at), { addSuffix: true })}
            </div>
            <div className={`${TABLE_CELL} justify-end text-muted-foreground`} style={{ width: 70 }}>
              {run.tokens_used > 0 ? `${(run.tokens_used / 1000).toFixed(1)}k` : '-'}
            </div>
          </button>
        );
      })}
    </div>
  );
}
