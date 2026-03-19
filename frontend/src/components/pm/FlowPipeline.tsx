import { CheckCircle2, Loader2, Wrench, XCircle } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { NODE_ICONS, NODE_TYPE_LABELS, NODE_STATUS_DOT, nodeLabel } from '@/components/pm/flowConstants';
import type { FlowNodeSpec, FlowNodeRun } from '@/lib/pmTypes';

export function FlowPipeline({
  nodes,
  nodeRuns,
  compact = false,
  onNodeClick,
}: {
  nodes: FlowNodeSpec[];
  nodeRuns?: FlowNodeRun[];
  compact?: boolean;
  onNodeClick?: (nodeId: string, nodeRun?: FlowNodeRun) => void;
}) {
  const getNodeRun = (nodeId: string): FlowNodeRun | undefined => {
    if (!nodeRuns) return undefined;
    for (let i = nodeRuns.length - 1; i >= 0; i--) {
      if (nodeRuns[i].node_id === nodeId) return nodeRuns[i];
    }
    return undefined;
  };

  return (
    <div className="flex flex-col">
      {nodes.map((node, idx) => {
        const nr = getNodeRun(node.id);
        const Icon = NODE_ICONS[node.type] ?? Wrench;
        const isLast = idx === nodes.length - 1;
        const label = nodeLabel(node.id, node.label);
        const statusDot = nr ? NODE_STATUS_DOT[nr.status] : 'bg-zinc-200 dark:bg-zinc-700';
        const isClickable = !!onNodeClick && !!nr;

        return (
          <div key={node.id} className="flex gap-3">
            {/* Timeline column */}
            <div className="flex flex-col items-center w-5 shrink-0">
              <div
                className={`h-5 w-5 rounded-full flex items-center justify-center ring-2 ring-background ${statusDot}`}
              >
                {nr?.status === 'completed' && <CheckCircle2 className="h-3 w-3 text-white" />}
                {nr?.status === 'running' && <Loader2 className="h-3 w-3 text-white animate-spin" />}
                {(nr?.status === 'awaiting_input' || nr?.status === 'awaiting_approval') && (
                  <div className="h-2 w-2 rounded-full bg-white" />
                )}
                {nr?.status === 'failed' && <XCircle className="h-3 w-3 text-white" />}
              </div>
              {!isLast && (
                <div className={`w-px flex-1 min-h-4 ${nr?.status === 'completed' ? 'bg-green-500/40' : 'bg-border'}`} />
              )}
            </div>

            {/* Content */}
            <div
              className={`pb-${compact ? '2' : '3'} flex-1 min-w-0 ${isClickable ? 'cursor-pointer hover:bg-muted/50 -mx-1 px-1 rounded' : ''}`}
              onClick={isClickable ? () => onNodeClick(node.id, nr) : undefined}
            >
              <div className="flex items-center gap-2">
                <Icon className={`h-3.5 w-3.5 shrink-0 ${nr ? 'text-foreground' : 'text-muted-foreground'}`} />
                <span className={`text-sm font-medium truncate ${nr ? '' : 'text-muted-foreground'}`}>
                  {label}
                </span>
              </div>
              {!compact && (
                <div className="flex items-center gap-1.5 mt-0.5 ml-5.5">
                  <span className="text-[11px] text-muted-foreground">
                    {NODE_TYPE_LABELS[node.type]}
                    {node.required_mode && ` · ${node.required_mode}`}
                  </span>
                  {nr && nr.status !== 'queued' && (
                    <Badge variant="secondary" className="text-[10px] h-4 px-1">
                      {nr.status.replace(/_/g, ' ')}
                    </Badge>
                  )}
                </div>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
