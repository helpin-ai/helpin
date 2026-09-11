import { useState } from "react";
import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type Modifier,
} from "@dnd-kit/core";
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import {
  QuietTextAction,
  QuietIconAction as Button,
} from "@/components/design-system/quiet";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/design-system/quiet-dropdown-select";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { StageTypeIcon } from "@/lib/crmConstants";
import {
  Delete01Icon,
  DragDropVerticalIcon,
  PencilEdit01Icon,
  PlusSignIcon,
} from "@/lib/icons";
import { cn } from "@/lib/utils";
import type { CRMPipelineStage, PipelineStageType } from "@/lib/crmTypes";
import {
  deletionBlock,
  STAGE_LABELS,
  STAGE_TYPES,
  sortStages,
} from "./pipelineSettingsUtils";

const restrictToVerticalAxis: Modifier = ({ transform }) => ({
  ...transform,
  x: 0,
});

type Props = {
  stages: CRMPipelineStage[];
  editable: boolean;
  pending: boolean;
  onReorder: (stages: CRMPipelineStage[]) => Promise<void>;
  onEdit: (stage: CRMPipelineStage) => void;
  onDelete: (stage: CRMPipelineStage) => void;
  onAdd: (type: PipelineStageType) => void;
};

export function PipelineStageTable(props: Props) {
  const [optimisticOrder, setOptimisticOrder] = useState<
    CRMPipelineStage[] | null
  >(null);
  const stages = optimisticOrder ?? sortStages(props.stages);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );
  const move = async (stageId: string, targetId: string) => {
    if (props.pending || optimisticOrder || stageId === targetId) return;
    const from = stages.findIndex((stage) => stage.id === stageId);
    const to = stages.findIndex((stage) => stage.id === targetId);
    if (from < 0 || to < 0 || stages[from].stage_type !== stages[to].stage_type)
      return;
    const next = arrayMove(stages, from, to).map((stage, position) => ({
      ...stage,
      position,
    }));
    setOptimisticOrder(next);
    try {
      await props.onReorder(next);
    } finally {
      setOptimisticOrder(null);
    }
  };
  const disabled = props.pending || !!optimisticOrder;

  return (
    <div className="border-t border-quiet-divider-strong">
      <div className="hidden grid-cols-[76px_minmax(0,1fr)_112px_72px_76px] items-center gap-3 border-b border-quiet-divider-strong px-4 py-3 text-xs font-medium text-quiet-text-tertiary sm:grid">
        <span>Position</span>
        <span>Stage name</span>
        <span>Win probability</span>
        <span className="text-right">Deals</span>
        <span className="sr-only">Actions</span>
      </div>
      {STAGE_TYPES.map((type) => {
        const group = stages.filter((stage) => stage.stage_type === type);
        return (
          <section key={type} aria-label={`${STAGE_LABELS[type]} stages`}>
            <div className="flex items-center justify-between gap-2 border-b border-quiet-divider-strong px-4 py-2">
              <div className="flex items-center gap-2 text-xs font-medium">
                <StageTypeIcon stageType={type} className="h-4 w-4" />
                {type === "open"
                  ? "Active stages"
                  : `${STAGE_LABELS[type]} outcomes`}
                <span className="text-quiet-text-tertiary">{group.length}</span>
              </div>
              {props.editable && (
                <QuietTextAction
                  className="h-7 text-xs"
                  disabled={disabled}
                  onClick={() => props.onAdd(type)}
                >
                  <PlusSignIcon className="size-3.5" />
                  Add {type === "open" ? "stage" : "outcome"}
                </QuietTextAction>
              )}
            </div>
            <DndContext
              sensors={sensors}
              collisionDetection={closestCenter}
              modifiers={[restrictToVerticalAxis]}
              accessibility={{
                announcements: {
                  onDragStart: ({ active }) =>
                    `Picked up ${group.find((stage) => stage.id === active.id)?.name}.`,
                  onDragOver: ({ active, over }) =>
                    over
                      ? `${group.find((stage) => stage.id === active.id)?.name} is over position ${group.findIndex((stage) => stage.id === over.id) + 1} of ${group.length}.`
                      : undefined,
                  onDragEnd: ({ active, over }) =>
                    over
                      ? `${group.find((stage) => stage.id === active.id)?.name} dropped at position ${group.findIndex((stage) => stage.id === over.id) + 1} of ${group.length}.`
                      : "Reordering cancelled.",
                  onDragCancel: () => "Reordering cancelled.",
                },
              }}
              onDragEnd={({ active, over }) => {
                if (over) void move(String(active.id), String(over.id));
              }}
            >
              <SortableContext
                items={group.map((stage) => stage.id)}
                strategy={verticalListSortingStrategy}
              >
                {group.map((stage, index) => (
                  <StageRow
                    key={stage.id}
                    stage={stage}
                    group={group}
                    index={index}
                    disabled={disabled}
                    editable={props.editable}
                    blockReason={deletionBlock(stages, stage)}
                    onMove={(targetId) => void move(stage.id, targetId)}
                    onEdit={() => props.onEdit(stage)}
                    onDelete={() => props.onDelete(stage)}
                  />
                ))}
              </SortableContext>
            </DndContext>
            {group.length === 0 && (
              <p className="border-b px-4 py-4 text-sm text-quiet-text-tertiary">
                No {STAGE_LABELS[type].toLowerCase()} stages yet.
              </p>
            )}
          </section>
        );
      })}
      <p className="px-4 py-3 text-xs leading-relaxed text-quiet-text-tertiary">
        Win probability estimates the chance of closing a deal. A deal’s own
        probability takes priority. Won and lost outcomes stay at the end.
      </p>
    </div>
  );
}

function StageRow({
  stage,
  group,
  index,
  editable,
  disabled,
  blockReason,
  onMove,
  onEdit,
  onDelete,
}: {
  stage: CRMPipelineStage;
  group: CRMPipelineStage[];
  index: number;
  editable: boolean;
  disabled: boolean;
  blockReason: string | null;
  onMove: (id: string) => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({
    id: stage.id,
    disabled: !editable || disabled || group.length < 2,
  });
  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={cn(
        "relative grid grid-cols-[68px_minmax(0,1fr)_64px] items-center gap-x-3 gap-y-2 border-b border-quiet-divider-light bg-quiet-surface hover:bg-quiet-row-hover px-4 py-3 sm:grid-cols-[76px_minmax(0,1fr)_112px_72px_76px]",
        !editable && "grid-cols-[24px_minmax(0,1fr)]",
        isDragging && "z-10 bg-quiet-hover outline outline-quiet-field",
      )}
    >
      <div className="flex items-center gap-1">
        {editable && (
          <button
            type="button"
            {...attributes}
            {...listeners}
            aria-label={`Drag ${stage.name}`}
            disabled={disabled || group.length < 2}
            className="touch-none rounded p-1 text-quiet-text-tertiary hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-30"
            title="Drag to reorder. With a keyboard, press Space, use arrow keys, then Space to drop."
          >
            <DragDropVerticalIcon className="size-4" />
          </button>
        )}
        {editable ? (
          <Select
            value={stage.id}
            disabled={disabled || group.length < 2}
            onValueChange={onMove}
            size="sm"
          >
            <SelectTrigger
              variant="ghost"
              aria-label={`Position of ${stage.name}`}
              className="w-11 px-1 text-xs tabular-nums"
              title="Move directly to a position in this group"
            >
              <SelectValue>{index + 1}</SelectValue>
            </SelectTrigger>
            <SelectContent>
              {group.map((candidate, position) => (
                <SelectItem key={candidate.id} value={candidate.id}>
                  {position + 1} · {candidate.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : (
          <span className="text-xs tabular-nums text-quiet-text-tertiary">
            {index + 1}
          </span>
        )}
      </div>
      <div className="min-w-0 text-sm font-medium break-words">
        {stage.name}
      </div>
      <div className="col-start-2 row-start-2 flex items-center gap-2 sm:col-start-auto sm:row-start-auto">
        <div
          className="h-1 w-10 overflow-hidden rounded-full bg-muted"
          aria-hidden="true"
        >
          <div
            className={cn(
              "h-full rounded-full bg-quiet-text-secondary",
              stage.stage_type === "won" && "bg-quiet-positive",
              stage.stage_type === "lost" && "bg-muted-foreground",
            )}
            style={{ width: `${stage.probability}%` }}
          />
        </div>
        <span className="text-xs tabular-nums text-quiet-text-tertiary">
          {stage.probability}%
        </span>
        <span className="text-xs text-quiet-text-tertiary sm:hidden">
          · {stage.deal_count ?? 0} deals
        </span>
      </div>
      <span className="hidden text-right text-sm tabular-nums text-quiet-text-tertiary sm:block">
        {stage.deal_count ?? 0}
      </span>
      <div className="col-start-3 row-start-1 flex justify-end sm:col-start-auto sm:row-start-auto">
        {editable && (
          <>
            <Button
              variant="ghost"
              size="icon"
              className="size-8 text-quiet-text-tertiary"
              disabled={disabled}
              aria-label={`Edit ${stage.name}`}
              onClick={onEdit}
            >
              <PencilEdit01Icon className="size-4" />
            </Button>
            <Tooltip>
              <TooltipTrigger asChild>
                <span
                  tabIndex={blockReason ? 0 : undefined}
                  aria-label={blockReason ?? undefined}
                >
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-8 text-quiet-text-tertiary hover:text-destructive"
                    disabled={disabled || !!blockReason}
                    aria-label={`Delete ${stage.name}`}
                    onClick={onDelete}
                  >
                    <Delete01Icon className="size-4" />
                  </Button>
                </span>
              </TooltipTrigger>
              <TooltipContent>{blockReason ?? "Delete stage"}</TooltipContent>
            </Tooltip>
          </>
        )}
      </div>
    </div>
  );
}
