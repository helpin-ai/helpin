import { useState } from 'react';
import { Plus, Trash2, Pencil, ArrowUp, ArrowDown, ChevronDown, ChevronUp } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { usePipelines, useCreatePipeline, useUpdatePipeline, useDeletePipeline } from '@/hooks/queries';
import { StageTypeIcon } from '@/lib/crmConstants';
import type { CRMPipeline, CRMPipelineStage, PipelineStageType } from '@/lib/crmTypes';

const STAGE_TYPE_ORDER: PipelineStageType[] = ['open', 'won', 'lost'];
const STAGE_TYPE_LABEL: Record<PipelineStageType, string> = {
  open: 'Open',
  won: 'Won',
  lost: 'Lost',
};
const LINEAR_CARD_CLASS = 'rounded-none border-border shadow-none dark:border-transparent';

const DEFAULT_STAGES = [
  { name: 'Qualification', stage_type: 'open' as PipelineStageType, position: 0, probability: 10 },
  { name: 'Proposal', stage_type: 'open' as PipelineStageType, position: 1, probability: 30 },
  { name: 'Negotiation', stage_type: 'open' as PipelineStageType, position: 2, probability: 60 },
  { name: 'Closed Won', stage_type: 'won' as PipelineStageType, position: 3, probability: 100 },
  { name: 'Closed Lost', stage_type: 'lost' as PipelineStageType, position: 4, probability: 0 },
];

export function PipelineSettings() {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const { data: pipelines, isLoading } = usePipelines(wsId);
  const createPipeline = useCreatePipeline(wsId);
  const updatePipeline = useUpdatePipeline(wsId);
  const deletePipeline = useDeletePipeline(wsId);

  const [expandedPipelineId, setExpandedPipelineId] = useState<string | null>(null);
  const [pipelineDialogOpen, setPipelineDialogOpen] = useState(false);
  const [editPipeline, setEditPipeline] = useState<CRMPipeline | null>(null);
  const [pipelineName, setPipelineName] = useState('');
  const [pipelineIsDefault, setPipelineIsDefault] = useState(false);

  const [stageDialogOpen, setStageDialogOpen] = useState(false);
  const [editStage, setEditStage] = useState<CRMPipelineStage | null>(null);
  const [targetPipelineId, setTargetPipelineId] = useState<string | null>(null);
  const [stageName, setStageName] = useState('');
  const [stageType, setStageType] = useState<PipelineStageType>('open');
  const [stageProbability, setStageProbability] = useState(0);

  const [deletePipelineConfirm, setDeletePipelineConfirm] = useState<string | null>(null);
  const [deleteStageConfirm, setDeleteStageConfirm] = useState<{ pipelineId: string; stageId: string } | null>(null);

  // ── Pipeline dialog ──

  const openCreatePipeline = () => {
    setEditPipeline(null);
    setPipelineName('');
    setPipelineIsDefault(false);
    setPipelineDialogOpen(true);
  };

  const openEditPipeline = (p: CRMPipeline) => {
    setEditPipeline(p);
    setPipelineName(p.name);
    setPipelineIsDefault(p.is_default);
    setPipelineDialogOpen(true);
  };

  const handleSavePipeline = async () => {
    if (!pipelineName.trim()) return;
    try {
      if (editPipeline) {
        await updatePipeline.mutateAsync({
          id: editPipeline.id,
          name: pipelineName.trim(),
          is_default: pipelineIsDefault,
        });
        toast.success('Pipeline updated');
      } else {
        await createPipeline.mutateAsync({
          workspace_id: wsId,
          name: pipelineName.trim(),
          is_default: pipelineIsDefault,
          stages: DEFAULT_STAGES,
        });
        toast.success('Pipeline created');
      }
      setPipelineDialogOpen(false);
    } catch {
      toast.error(editPipeline ? 'Failed to update pipeline' : 'Failed to create pipeline');
    }
  };

  const handleDeletePipeline = async () => {
    if (!deletePipelineConfirm) return;
    try {
      await deletePipeline.mutateAsync(deletePipelineConfirm);
      toast.success('Pipeline deleted');
      if (expandedPipelineId === deletePipelineConfirm) setExpandedPipelineId(null);
    } catch {
      toast.error('Failed to delete pipeline');
    }
    setDeletePipelineConfirm(null);
  };

  // ── Stage helpers ──

  const getPipeline = (id: string) => pipelines?.find((p) => p.id === id);

  const openAddStage = (pipelineId: string) => {
    setEditStage(null);
    setTargetPipelineId(pipelineId);
    setStageName('');
    setStageType('open');
    setStageProbability(0);
    setStageDialogOpen(true);
  };

  const openEditStage = (pipelineId: string, stage: CRMPipelineStage) => {
    setEditStage(stage);
    setTargetPipelineId(pipelineId);
    setStageName(stage.name);
    setStageType(stage.stage_type);
    setStageProbability(stage.probability);
    setStageDialogOpen(true);
  };

  const saveStages = async (pipelineId: string, stages: CRMPipelineStage[]) => {
    const mapped = stages.map((s, i) => ({
      id: s.id || undefined,
      name: s.name,
      stage_type: s.stage_type,
      position: i,
      probability: s.probability,
    }));
    try {
      await updatePipeline.mutateAsync({ id: pipelineId, stages: mapped });
    } catch {
      toast.error('Failed to update stages');
    }
  };

  const handleSaveStage = async () => {
    if (!stageName.trim() || !targetPipelineId) return;
    const pipeline = getPipeline(targetPipelineId);
    if (!pipeline) return;

    const existing = [...(pipeline.stages ?? [])];

    if (editStage) {
      const idx = existing.findIndex((s) => s.id === editStage.id);
      if (idx >= 0) {
        existing[idx] = { ...existing[idx], name: stageName.trim(), stage_type: stageType, probability: stageProbability };
      }
    } else {
      const newStage = {
        id: '',
        pipeline_id: targetPipelineId,
        name: stageName.trim(),
        stage_type: stageType,
        position: existing.length,
        probability: stageProbability,
        created_at: '',
        updated_at: '',
      } satisfies CRMPipelineStage;
      existing.push(newStage);
    }

    // Sort by stage_type order, preserving relative order within each group
    const sorted = sortStages(existing);
    await saveStages(targetPipelineId, sorted);
    toast.success(editStage ? 'Stage updated' : 'Stage added');
    setStageDialogOpen(false);
  };

  const handleDeleteStage = async () => {
    if (!deleteStageConfirm) return;
    const pipeline = getPipeline(deleteStageConfirm.pipelineId);
    if (!pipeline) return;

    const remaining = (pipeline.stages ?? []).filter((s) => s.id !== deleteStageConfirm.stageId);
    await saveStages(deleteStageConfirm.pipelineId, remaining);
    toast.success('Stage deleted');
    setDeleteStageConfirm(null);
  };

  const handleReorder = async (pipelineId: string, stageId: string, direction: 'up' | 'down') => {
    const pipeline = getPipeline(pipelineId);
    if (!pipeline) return;

    const stages = sortStages([...(pipeline.stages ?? [])]);
    // Find stage and its group
    const stage = stages.find((s) => s.id === stageId);
    if (!stage) return;

    const group = stages.filter((s) => s.stage_type === stage.stage_type);
    const groupIdx = group.findIndex((s) => s.id === stageId);
    if (direction === 'up' && groupIdx <= 0) return;
    if (direction === 'down' && groupIdx >= group.length - 1) return;

    const swapIdx = direction === 'up' ? groupIdx - 1 : groupIdx + 1;
    // Swap within group
    [group[groupIdx], group[swapIdx]] = [group[swapIdx], group[groupIdx]];

    // Rebuild full list preserving group order
    const rebuilt: CRMPipelineStage[] = [];
    for (const type of STAGE_TYPE_ORDER) {
      if (type === stage.stage_type) {
        rebuilt.push(...group);
      } else {
        rebuilt.push(...stages.filter((s) => s.stage_type === type));
      }
    }
    await saveStages(pipelineId, rebuilt);
  };

  if (isLoading) {
    return <div className="p-4 text-muted-foreground">Loading pipelines...</div>;
  }

  const pipelineList = pipelines ?? [];

  return (
    <>
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
          <div className="flex items-center gap-2">
            <CardTitle className="text-base font-medium">Pipelines</CardTitle>
            <Badge variant="secondary" className="text-xs">{pipelineList.length}</Badge>
          </div>
          <Button size="sm" variant="outline" onClick={openCreatePipeline}>
            <Plus className="mr-1 h-3.5 w-3.5" />
            Create
          </Button>
        </CardHeader>
        <CardContent className="space-y-1.5 pt-0">
          {pipelineList.length === 0 && (
            <p className="text-xs text-muted-foreground py-4 text-center">
              No pipelines configured. Create one to start managing deals.
            </p>
          )}
          {pipelineList.map((pipeline) => {
            const isExpanded = expandedPipelineId === pipeline.id;
            const hasDealCount = pipeline.deal_count > 0;

            return (
              <div key={pipeline.id} className="rounded-none border border-border">
                {/* Pipeline row */}
                <div className="flex items-center justify-between px-3 py-2">
                  <div className="flex items-center gap-2 min-w-0">
                    <span className="text-sm font-medium truncate">{pipeline.name}</span>
                    {pipeline.is_default && (
                      <Badge variant="secondary" className="text-[10px] shrink-0">Default</Badge>
                    )}
                    {hasDealCount && (
                      <Badge variant="outline" className="text-[10px] shrink-0">
                        {pipeline.deal_count} {pipeline.deal_count === 1 ? 'deal' : 'deals'}
                      </Badge>
                    )}
                  </div>
                  <div className="flex items-center gap-0.5 shrink-0">
                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => openEditPipeline(pipeline)}>
                      <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      onClick={() => setExpandedPipelineId(isExpanded ? null : pipeline.id)}
                    >
                      {isExpanded ? <ChevronUp className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7 text-destructive"
                      disabled={hasDealCount}
                      title={hasDealCount ? 'Pipeline has active deals' : 'Delete pipeline'}
                      onClick={() => setDeletePipelineConfirm(pipeline.id)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>

                {/* Expanded stage management */}
                {isExpanded && (
                  <div className="border-t border-border px-3 py-2 space-y-4">
                    {STAGE_TYPE_ORDER.map((type) => {
                      const groupStages = sortStages(pipeline.stages ?? []).filter((s) => s.stage_type === type);
                      return (
                        <section key={type} className="space-y-1.5">
                          <div className="flex items-center justify-between">
                            <h4 className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide">
                              <StageTypeIcon stageType={type} className="h-3.5 w-3.5" />
                              {STAGE_TYPE_LABEL[type]}
                            </h4>
                            <Button variant="ghost" size="sm" onClick={() => {
                              setEditStage(null);
                              setTargetPipelineId(pipeline.id);
                              setStageName('');
                              setStageType(type);
                              setStageProbability(type === 'won' ? 100 : type === 'lost' ? 0 : 0);
                              setStageDialogOpen(true);
                            }}>
                              <Plus className="h-3.5 w-3.5 mr-1" /> Add
                            </Button>
                          </div>
                          {groupStages.length === 0 ? (
                            <div className="rounded-none border border-dashed border-border px-3 py-2 text-xs text-muted-foreground">
                              No stages in this group.
                            </div>
                          ) : (
                            <div className="space-y-1.5">
                              {groupStages.map((stage, idx) => (
                                <div key={stage.id} className="rounded-none border border-border px-2.5 py-1.5">
                                  <div className="flex items-start justify-between gap-2">
                                    <div>
                                      <div className="flex items-center gap-1.5">
                                        <p className="text-sm font-medium">{stage.name}</p>
                                        <Badge variant="outline" className="text-xs">{stage.probability}%</Badge>
                                      </div>
                                    </div>
                                    <div className="flex items-center gap-1">
                                      <Button
                                        size="icon"
                                        variant="ghost"
                                        className="h-7 w-7"
                                        disabled={idx === 0}
                                        onClick={() => handleReorder(pipeline.id, stage.id, 'up')}
                                      >
                                        <ArrowUp className="h-3.5 w-3.5" />
                                      </Button>
                                      <Button
                                        size="icon"
                                        variant="ghost"
                                        className="h-7 w-7"
                                        disabled={idx === groupStages.length - 1}
                                        onClick={() => handleReorder(pipeline.id, stage.id, 'down')}
                                      >
                                        <ArrowDown className="h-3.5 w-3.5" />
                                      </Button>
                                      <Button
                                        size="icon"
                                        variant="ghost"
                                        className="h-7 w-7"
                                        onClick={() => openEditStage(pipeline.id, stage)}
                                      >
                                        <Pencil className="h-3.5 w-3.5" />
                                      </Button>
                                    </div>
                                  </div>
                                </div>
                              ))}
                            </div>
                          )}
                        </section>
                      );
                    })}
                  </div>
                )}
              </div>
            );
          })}
        </CardContent>
      </Card>

      {/* Create/Edit Pipeline Dialog */}
      <Dialog open={pipelineDialogOpen} onOpenChange={setPipelineDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{editPipeline ? 'Edit Pipeline' : 'Create Pipeline'}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="pipeline-name">Name</Label>
              <Input
                id="pipeline-name"
                value={pipelineName}
                onChange={(e) => setPipelineName(e.target.value)}
                placeholder="e.g. Sales Pipeline"
              />
            </div>
            <div className="flex items-center gap-2">
              <Switch
                id="pipeline-default"
                checked={pipelineIsDefault}
                onCheckedChange={setPipelineIsDefault}
              />
              <Label htmlFor="pipeline-default">Default pipeline</Label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPipelineDialogOpen(false)}>Cancel</Button>
            <Button onClick={handleSavePipeline} disabled={!pipelineName.trim()}>
              {editPipeline ? 'Save' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Add/Edit Stage Dialog */}
      <Dialog open={stageDialogOpen} onOpenChange={setStageDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{editStage ? 'Edit Stage' : 'Add Stage'}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="stage-name">Name</Label>
              <Input
                id="stage-name"
                value={stageName}
                onChange={(e) => setStageName(e.target.value)}
                placeholder="e.g. Qualification"
              />
            </div>
            <div className="space-y-2">
              <Label>Stage Type</Label>
              <Select value={stageType} onValueChange={(v) => setStageType(v as PipelineStageType)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="open">Open</SelectItem>
                  <SelectItem value="won">Won</SelectItem>
                  <SelectItem value="lost">Lost</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="stage-probability">Probability (%)</Label>
              <Input
                id="stage-probability"
                type="number"
                min={0}
                max={100}
                value={stageProbability}
                onChange={(e) => setStageProbability(Math.min(100, Math.max(0, parseInt(e.target.value) || 0)))}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setStageDialogOpen(false)}>Cancel</Button>
            <Button onClick={handleSaveStage} disabled={!stageName.trim()}>
              {editStage ? 'Save' : 'Add'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Pipeline Confirm */}
      <ConfirmDialog
        open={deletePipelineConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeletePipelineConfirm(null); }}
        title="Delete Pipeline"
        description="This will permanently delete this pipeline and all its stages. This action cannot be undone."
        confirmLabel="Delete"
        onConfirm={handleDeletePipeline}
      />

      {/* Delete Stage Confirm */}
      <ConfirmDialog
        open={deleteStageConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeleteStageConfirm(null); }}
        title="Delete Stage"
        description={
          deleteStageConfirm && (getPipeline(deleteStageConfirm.pipelineId)?.deal_count ?? 0) > 0
            ? 'This pipeline has active deals. Removing this stage may affect deals currently in this stage.'
            : 'This will remove the stage from the pipeline.'
        }
        confirmLabel="Delete"
        onConfirm={handleDeleteStage}
      />
    </>
  );
}

/** Sort stages by stage_type order, preserving relative position within each group. */
function sortStages(stages: CRMPipelineStage[]): CRMPipelineStage[] {
  const byType: Record<string, CRMPipelineStage[]> = { open: [], won: [], lost: [] };
  for (const s of stages) {
    (byType[s.stage_type] ?? byType.open).push(s);
  }
  for (const type of STAGE_TYPE_ORDER) {
    byType[type].sort((a, b) => a.position - b.position);
  }
  return [...byType.open, ...byType.won, ...byType.lost];
}
