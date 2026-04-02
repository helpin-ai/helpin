import { useCallback, useMemo, useState } from 'react';
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCorners,
  useSensor,
  useSensors,
  useDroppable,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { Maximize2, Minimize2, Plus, SquareKanban } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { StageTypeIcon } from '@/lib/crmConstants';
import { crmDealService } from '@/lib/services/crmService';
import { DealCard } from './DealCard';
import type { CRMDeal, CRMPipeline, CRMPipelineStage } from '@/lib/crmTypes';
import type { AssignableMember } from '@/lib/types';

interface DealBoardProps {
  deals: CRMDeal[];
  pipeline?: CRMPipeline;
  workspaceId: string;
  assignableMembers: AssignableMember[];
  ownerNameMap: Map<string, string>;
  onDealClick: (id: string) => void;
  onCreateClick?: () => void;
  onDealUpdated?: (deal: CRMDeal) => void;
  showEmptyStages: boolean;
}

interface ColumnProps {
  stage: CRMPipelineStage;
  deals: CRMDeal[];
  collapsed: boolean;
  onToggleCollapse: (stageId: string) => void;
  onCreateClick?: () => void;
  onDealClick: (id: string) => void;
  workspaceId: string;
  assignableMembers: AssignableMember[];
  ownerNameMap: Map<string, string>;
  onDealUpdated?: (deal: CRMDeal) => void;
}

function Column({
  stage,
  deals,
  collapsed,
  onToggleCollapse,
  onCreateClick,
  onDealClick,
  workspaceId,
  assignableMembers,
  ownerNameMap,
  onDealUpdated,
}: ColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id: stage.id });
  const stageTotal = deals.reduce((sum, d) => sum + (d.amount ?? 0), 0);

  if (collapsed) {
    return (
      <QuickTooltip label={`Expand ${stage.name}`}>
        <section
          className="flex h-full w-[44px] shrink-0 cursor-pointer flex-col items-center rounded-md border border-border/50 bg-muted/30 pt-4 transition-colors hover:bg-muted/50"
          onClick={() => onToggleCollapse(stage.id)}
        >
          <Maximize2 className="mb-3 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          <StageTypeIcon stageType={stage.stage_type} className="mb-2 h-4 w-4 shrink-0" />
          <span className="text-xs font-medium text-muted-foreground">{deals.length}</span>
          <div className="mt-3 flex flex-1 items-start">
            <span
              className="text-xs font-semibold whitespace-nowrap"
              style={{ writingMode: 'vertical-rl', textOrientation: 'mixed' }}
            >
              {stage.name}
            </span>
          </div>
        </section>
      </QuickTooltip>
    );
  }

  return (
    <section className="flex h-full w-[300px] shrink-0 flex-col">
      <header className="flex items-center justify-between px-3 pt-4 pb-3">
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 truncate text-sm font-semibold">
            <StageTypeIcon stageType={stage.stage_type} className="h-4 w-4 shrink-0" />
            {stage.name}
          </p>
          <p className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
            <span>{deals.length} deals</span>
            {stageTotal > 0 && (
              <span>${new Intl.NumberFormat().format(stageTotal)}</span>
            )}
          </p>
        </div>
        <div className="flex items-center gap-0.5">
          <QuickTooltip label="Collapse column">
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onToggleCollapse(stage.id)}
            >
              <Minimize2 className="h-3.5 w-3.5" />
            </Button>
          </QuickTooltip>
          {onCreateClick && (
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onCreateClick}>
              <Plus className="h-4 w-4" />
            </Button>
          )}
        </div>
      </header>

      <SortableContext items={deals.map((d) => d.id)} strategy={verticalListSortingStrategy}>
        <div
          ref={setNodeRef}
          className={`min-h-0 flex-1 space-y-2 overflow-y-auto p-2 transition-colors ${isOver ? 'bg-primary/5' : ''}`}
        >
          {deals.map((deal) => (
            <DealCard
              key={deal.id}
              deal={deal}
              onOpen={(d) => onDealClick(d.id)}
              workspaceId={workspaceId}
              assignableMembers={assignableMembers}
              ownerNameMap={ownerNameMap}
              onOwnerChanged={onDealUpdated}
            />
          ))}

          {onCreateClick && (
            <Button
              variant="ghost"
              className="w-full justify-start text-xs text-muted-foreground"
              onClick={onCreateClick}
            >
              <Plus className="h-3.5 w-3.5" />
              Add deal
            </Button>
          )}
        </div>
      </SortableContext>
    </section>
  );
}

export function DealBoard({
  deals,
  pipeline,
  workspaceId,
  assignableMembers,
  ownerNameMap,
  onDealClick,
  onCreateClick,
  onDealUpdated,
  showEmptyStages,
}: DealBoardProps) {
  const stages = useMemo(() => {
    if (!pipeline?.stages) return [];
    return [...pipeline.stages].sort((a, b) => a.position - b.position);
  }, [pipeline?.stages]);

  const [localDeals, setLocalDeals] = useState<CRMDeal[]>(deals);
  // Keep local deals in sync with prop changes
  useMemo(() => { setLocalDeals(deals); }, [deals]);

  const dealsByStage = useMemo(() => {
    const map = new Map<string, CRMDeal[]>();
    for (const stage of stages) {
      map.set(stage.id, []);
    }
    for (const deal of localDeals) {
      const existing = map.get(deal.stage_id);
      if (existing) {
        existing.push(deal);
      }
    }
    return map;
  }, [localDeals, stages]);

  const COLLAPSED_KEY = `crm_deal_collapsed_${workspaceId}`;
  const [collapsedColumns, setCollapsedColumns] = useState<Set<string>>(() => {
    try {
      const saved = localStorage.getItem(COLLAPSED_KEY);
      return saved ? new Set(JSON.parse(saved) as string[]) : new Set();
    } catch { return new Set(); }
  });

  const toggleCollapse = useCallback((stageId: string) => {
    setCollapsedColumns((prev) => {
      const next = new Set(prev);
      if (next.has(stageId)) next.delete(stageId);
      else next.add(stageId);
      try { localStorage.setItem(COLLAPSED_KEY, JSON.stringify([...next])); } catch {}
      return next;
    });
  }, [COLLAPSED_KEY]);

  // ── Drag and Drop ─────────────────────────────────────────────
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 8 } }));
  const [activeDeal, setActiveDeal] = useState<CRMDeal | null>(null);

  const findStageIdByItemId = useCallback(
    (id: string) => {
      // Check if id is a stage id directly
      if (stages.some((s) => s.id === id)) return id;
      // Otherwise find the deal's stage
      const deal = localDeals.find((d) => d.id === id);
      return deal?.stage_id ?? null;
    },
    [stages, localDeals],
  );

  const onDragStart = useCallback(
    (event: DragStartEvent) => {
      const deal = localDeals.find((d) => d.id === String(event.active.id));
      setActiveDeal(deal ?? null);
    },
    [localDeals],
  );

  const onDragEnd = useCallback(
    async (event: DragEndEvent) => {
      setActiveDeal(null);
      const { active, over } = event;
      if (!over) return;

      const activeId = String(active.id);
      const overId = String(over.id);
      if (activeId === overId) return;

      const fromStageId = findStageIdByItemId(activeId);
      const toStageId = findStageIdByItemId(overId);
      if (!fromStageId || !toStageId) return;
      if (fromStageId === toStageId) return;

      // Optimistic update
      const snapshot = localDeals;
      const targetStage = stages.find((s) => s.id === toStageId);
      setLocalDeals((current) =>
        current.map((d) =>
          d.id === activeId ? { ...d, stage_id: toStageId, stage: targetStage } : d,
        ),
      );

      const { error } = await crmDealService.update(workspaceId, activeId, { stage_id: toStageId });
      if (error) {
        setLocalDeals(snapshot);
      } else {
        onDealUpdated?.({ ...localDeals.find((d) => d.id === activeId)!, stage_id: toStageId, stage: targetStage } as CRMDeal);
      }
    },
    [findStageIdByItemId, stages, localDeals, workspaceId, onDealUpdated],
  );

  if (!pipeline || stages.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <SquareKanban className="h-8 w-8 text-muted-foreground/50" />
        </div>
        <h3 className="mt-4 text-base font-medium">No pipeline configured</h3>
        <p className="mt-1 max-w-sm text-sm text-muted-foreground">
          Create a pipeline in settings to use the board view
        </p>
      </div>
    );
  }

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCorners}
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
    >
      <div className="flex h-full min-w-full gap-3 overflow-x-auto pb-2">
        {stages
          .filter((stage) => showEmptyStages || (dealsByStage.get(stage.id)?.length ?? 0) > 0)
          .map((stage) => (
            <Column
              key={stage.id}
              stage={stage}
              deals={dealsByStage.get(stage.id) ?? []}
              collapsed={collapsedColumns.has(stage.id)}
              onToggleCollapse={toggleCollapse}
              onCreateClick={onCreateClick}
              onDealClick={onDealClick}
              workspaceId={workspaceId}
              assignableMembers={assignableMembers}
              ownerNameMap={ownerNameMap}
              onDealUpdated={onDealUpdated}
            />
          ))}
      </div>

      <DragOverlay>
        {activeDeal ? (
          <DealCard
            deal={activeDeal}
            onOpen={() => {}}
            workspaceId={workspaceId}
            ownerNameMap={ownerNameMap}
            isOverlay
          />
        ) : null}
      </DragOverlay>
    </DndContext>
  );
}
