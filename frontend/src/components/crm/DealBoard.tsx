import { defaultStageColor } from '@/lib/crmStageColors';
import { useDealEdits } from './useDealEdits';
import { commitDropBeforeClearingPreview, resolveBoardDropTarget, PM_BOARD_DRAG_ACTIVATION_DISTANCE } from '@/components/pm/KanbanBoard.dnd';
import { comparableDealTotal } from './dealCreationDefaults';
import { useCallback, useMemo, useState } from 'react';
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCenter,
  MeasuringStrategy,
  KeyboardSensor,
  useSensor,
  useSensors,
  useDroppable,
  type KeyboardCoordinateGetter,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { CollapseIcon, ExpandIcon, PlusSignIcon, KanbanIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
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
  onCreateClick?: (stageId?: string) => void;
  onDealUpdated?: (deal: CRMDeal) => void;
  showEmptyStages: boolean;
}

interface ColumnProps {
  isPending: (id: string) => boolean;
  stage: CRMPipelineStage;
  deals: CRMDeal[];
  collapsed: boolean;
  onToggleCollapse: (stageId: string) => void;
  onCreateClick?: () => void;
  onDealClick: (id: string) => void;
  workspaceId: string;
  assignableMembers: AssignableMember[];
  ownerNameMap: Map<string, string>;
  onOwnerChange: (id: string, ownerId: string) => void;
}

function Column({
  isPending,
  stage,
  deals,
  collapsed,
  onToggleCollapse,
  onCreateClick,
  onDealClick,
  workspaceId,
  assignableMembers,
  ownerNameMap,
  onOwnerChange,
}: ColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id: stage.id });
  const stageTotal = comparableDealTotal(deals);
  const stageColor = stage.color || defaultStageColor(stage.stage_type, stage.position);

  if (collapsed) {
    return (
      <QuickTooltip label={`Expand ${stage.name}`}>
        <button type="button" ref={setNodeRef} data-stage-id={stage.id}
          aria-label={`Expand ${stage.name}`} aria-expanded={false}
          className="group/collapsed relative flex h-full w-[44px] shrink-0 cursor-pointer flex-col items-center rounded-md border border-border/50 bg-muted/30 pt-4 transition-colors hover:bg-muted/50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-quiet-text-primary"
          onClick={() => onToggleCollapse(stage.id)}
        >
          <span aria-hidden="true" className="absolute top-0 left-2 right-2 h-[3px] rounded-b-full" style={{ backgroundColor: stageColor }} />
          <ExpandIcon className="mb-3 h-3.5 w-3.5 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover/collapsed:opacity-100 group-focus-visible/collapsed:opacity-100 [@media(hover:none)]:opacity-100" />
          <span className="text-xs font-medium text-muted-foreground">{deals.length}</span>
          <div className="mt-3 flex flex-1 items-start">
            <span
              className="text-xs font-semibold whitespace-nowrap"
              style={{ writingMode: 'vertical-rl', textOrientation: 'mixed' }}
            >
              {stage.name}
            </span>
          </div>
        </button>
      </QuickTooltip>
    );
  }

  return (
    <section data-stage-id={stage.id} className="flex h-full w-[300px] shrink-0 flex-col">
      <header className="group/header relative flex items-center justify-between px-3 pt-4 pb-3">
        <span aria-hidden="true" className="absolute top-0 left-3 right-3 h-[3px] rounded-b-full" style={{ backgroundColor: stageColor }} />
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 truncate text-sm font-semibold">
            {stage.name}
          </p>
          <p className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
            <span>{deals.length} deals</span>
            {stageTotal && (
              <span>{stageTotal}</span>
            )}
          </p>
        </div>
        <div className="flex items-center gap-0.5">
          <QuickTooltip label="Collapse column">
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 opacity-0 transition-opacity group-hover/header:opacity-100 group-focus-within/header:opacity-100 [@media(hover:none)]:opacity-100"
              aria-label={`Collapse ${stage.name}`} aria-expanded={true}
              onClick={() => onToggleCollapse(stage.id)}
            >
              <CollapseIcon className="h-3.5 w-3.5" />
            </Button>
          </QuickTooltip>
          {onCreateClick && (
            <QuickTooltip label="Add deal">
              <Button variant="ghost" size="icon" className="h-7 w-7" aria-label={`Add deal to ${stage.name}`} onClick={onCreateClick}>
                <PlusSignIcon className="h-4 w-4" />
              </Button>
            </QuickTooltip>
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
              pending={isPending(deal.id)}
              onOpen={(d) => onDealClick(d.id)}
              workspaceId={workspaceId}
              assignableMembers={assignableMembers}
              ownerNameMap={ownerNameMap}
              onOwnerChange={onOwnerChange}
            />
          ))}

          {onCreateClick && (
            <Button
              variant="ghost"
              className="w-full justify-center text-xs text-muted-foreground"
              onClick={onCreateClick}
            >
              <PlusSignIcon className="h-3.5 w-3.5" />
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
  }, [pipeline]);

  const { localDeals, updateDealField, isPending } = useDealEdits(workspaceId, pipeline?.id, deals, stages, onDealUpdated);

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
      try { localStorage.setItem(COLLAPSED_KEY, JSON.stringify([...next])); } catch { /* Keep the in-memory preference when storage is unavailable. */ }
      return next;
    });
  }, [COLLAPSED_KEY]);

  // ── Drag and Drop ─────────────────────────────────────────────
  // Deals move between stages; Left/Right targets the adjacent stage, including empty columns.
  const keyboardCoordinates = useCallback<KeyboardCoordinateGetter>((event, { context }) => {
    if (event.code !== 'ArrowLeft' && event.code !== 'ArrowRight') return;
    event.preventDefault();
    const { active, over, collisionRect, droppableRects } = context;
    if (!active || !collisionRect) return;
    const overId = String(over?.id ?? active.id);
    const currentStageId = stages.some(stage => stage.id === overId)
      ? overId
      : localDeals.find(deal => deal.id === overId)?.stage_id;
    const currentIndex = stages.findIndex(stage => stage.id === currentStageId);
    if (currentIndex < 0) return;
    const target = stages[currentIndex + (event.code === 'ArrowRight' ? 1 : -1)];
    const rect = target && droppableRects.get(target.id);
    if (!rect) return;
    return {
      x: rect.left + (rect.width - collisionRect.width) / 2,
      y: rect.top + (rect.height - collisionRect.height) / 2,
    };
  }, [localDeals, stages]);
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: PM_BOARD_DRAG_ACTIVATION_DISTANCE } }), useSensor(KeyboardSensor, {coordinateGetter:keyboardCoordinates}));
  const [activeDeal, setActiveDeal] = useState<CRMDeal | null>(null);

  const onDragStart = useCallback(
    (event: DragStartEvent) => {
      const deal = localDeals.find((d) => d.id === String(event.active.id));
      setActiveDeal(deal ?? null);
    },
    [localDeals],
  );

  const onDragEnd = useCallback(async (event: DragEndEvent) => {
    const clearPreview = () => setActiveDeal(null);
    const {active,over} = event;
    const deal = localDeals.find(d => d.id === String(active.id));
    if (!over || !deal) { clearPreview(); return; }
    const target = resolveBoardDropTarget({
      activeId:deal.id, fromColumnId:deal.stage_id, overId:String(over.id), pointerBelowMid:false, previewTarget:null,
      columns:stages.map(stage => ({id:stage.id,tasks:dealsByStage.get(stage.id) ?? []})),
    });
    if (!target || target.toColumnId === deal.stage_id) { clearPreview(); return; }
    await commitDropBeforeClearingPreview({commit:() => updateDealField(deal.id,{stage_id:target.toColumnId}),clearPreview});
  },[localDeals,stages,dealsByStage,updateDealField]);

  if (!pipeline || stages.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <KanbanIcon className="h-8 w-8 text-muted-foreground/50" />
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
      collisionDetection={closestCenter}
      measuring={{droppable:{strategy:MeasuringStrategy.Always}}}
      onDragCancel={() => setActiveDeal(null)}
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
              isPending={isPending}
              deals={dealsByStage.get(stage.id) ?? []}
              collapsed={collapsedColumns.has(stage.id)}
              onToggleCollapse={toggleCollapse}
              onCreateClick={onCreateClick ? () => onCreateClick(stage.id) : undefined}
              onDealClick={onDealClick}
              workspaceId={workspaceId}
              assignableMembers={assignableMembers}
              ownerNameMap={ownerNameMap}
              onOwnerChange={(id, ownerId) => { void updateDealField(id, { owner_member_id: ownerId }); }}
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
