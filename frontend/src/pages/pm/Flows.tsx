import { useCallback, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import {
  ChevronRight,
  Play,
} from 'lucide-react';

import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import {
  useFlowRuns,
  useFlowTemplates,
} from '@/hooks/queries/useFlow';
import {
  RUN_STATUS_CONFIG,
  templateLabel,
  nodeLabel,
  targetLabel,
} from '@/components/pm/flowConstants';
import { FlowRunDetailSheet } from '@/components/pm/FlowRunDetailSheet';
import { StartFlowDialog } from '@/components/pm/StartFlowDialog';
import type { FlowRunView } from '@/lib/pmTypes';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';

// ---------------------------------------------------------------------------
// FlowRunRow
// ---------------------------------------------------------------------------

function FlowRunRow({
  run,
  onClick,
}: {
  run: FlowRunView;
  onClick: (run: FlowRunView) => void;
}) {
  const status = RUN_STATUS_CONFIG[run.run.status] ?? RUN_STATUS_CONFIG.running;

  const currentNodeLabel = run.run.current_node_id
    ? nodeLabel(run.run.current_node_id, run.spec.nodes.find((n) => n.id === run.run.current_node_id)?.label)
    : '—';

  return (
    <button
      type="button"
      className="flex items-center gap-3 px-4 py-2.5 w-full text-left border-b border-border/60 last:border-b-0 hover:bg-muted/40 transition-colors"
      onClick={() => onClick(run)}
    >
      <Badge variant="secondary" className={`text-[11px] shrink-0 ${status.className}`}>
        {status.label}
      </Badge>
      <span className="text-sm font-medium min-w-[140px] shrink-0">{templateLabel(run.spec.template_id, run.spec.name)}</span>
      <span className="text-sm text-muted-foreground truncate flex-1">
        {targetLabel(run.run.target_type)} · {currentNodeLabel}
      </span>
      <span className="text-[12px] text-muted-foreground shrink-0">
        {formatDistanceToNow(new Date(run.run.created_at), { addSuffix: true })}
      </span>
      <ChevronRight className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
    </button>
  );
}

// ---------------------------------------------------------------------------
// FlowsPage
// ---------------------------------------------------------------------------

export function FlowsPage() {
  useTitle('Flows');
  const { currentWorkspace: workspace } = useWorkspaceStore();
  const workspaceId = workspace?.id ?? '';
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { has } = usePermissions(access);
  const canEdit = has('pm.edit');

  const { data: templates } = useFlowTemplates(workspaceId);
  const { data: runsData, isLoading: runsLoading } = useFlowRuns(workspaceId);

  const [startDialogOpen, setStartDialogOpen] = useState(false);
  const [preferredTemplateId, setPreferredTemplateId] = useState<string | null>(null);
  const [selectedRun, setSelectedRun] = useState<FlowRunView | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);

  const runs = runsData?.data ?? [];

  const handleRunClick = useCallback((run: FlowRunView) => {
    setSelectedRun(run);
    setSheetOpen(true);
  }, []);

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="max-w-5xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Flows</h1>
        {canEdit && (
          <Button size="sm" onClick={() => {
            setPreferredTemplateId(null);
            setStartDialogOpen(true);
          }}>
            <Play className="mr-1.5 h-4 w-4" />
            Start Flow
          </Button>
        )}
      </div>

      {/* Recent Runs */}
      <section>
        <h2 className="text-[11px] font-medium text-muted-foreground uppercase tracking-wide mb-3">
          Recent Runs
        </h2>
        {runsLoading && (
          <p className="text-sm text-muted-foreground">Loading runs...</p>
        )}
        {!runsLoading && runs.length === 0 && (
          <div className="rounded-lg border border-dashed border-border/60 p-8 flex flex-col items-center text-center">
            <Play className="h-6 w-6 text-muted-foreground/50 mb-2" />
            <p className="text-sm text-muted-foreground">No flow runs yet.</p>
            <p className="text-[12px] text-muted-foreground/70 mt-1">
              Start a flow from a template to begin orchestrating work.
            </p>
          </div>
        )}
        {runs.length > 0 && (
          <div className="rounded-lg border border-border overflow-hidden">
            {runs.map((run) => (
              <FlowRunRow key={run.run.id} run={run} onClick={handleRunClick} />
            ))}
          </div>
        )}
      </section>

      {/* Start Dialog */}
      {templates && (
        <StartFlowDialog
          open={startDialogOpen}
          onOpenChange={(open) => {
            setStartDialogOpen(open);
            if (!open) setPreferredTemplateId(null);
          }}
          workspaceId={workspaceId}
          templates={templates}
          preferredTemplateId={preferredTemplateId}
        />
      )}

      {/* Run Detail Sheet */}
      <FlowRunDetailSheet
        open={sheetOpen}
        onOpenChange={setSheetOpen}
        runView={selectedRun}
        workspaceId={workspaceId}
      />
    </div>
  );
}
