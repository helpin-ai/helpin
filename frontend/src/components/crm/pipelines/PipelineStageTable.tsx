import { useRef, useState, type ReactNode } from "react";
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
  QuietUnderlineInput,
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

const PROBABILITY_HELP =
  "Win probability estimates the chance of closing a deal. A deal’s own probability takes priority.";
const POSITION_HELP =
  "Drag a handle or choose a position to reorder stages within their outcome. Won and lost outcomes stay at the end.";

type Props = {
  stages: CRMPipelineStage[];
  editable: boolean;
  pending: boolean;
  editVersions: Record<string, number>;
  onReorder: (stages: CRMPipelineStage[]) => Promise<void>;
  onUpdate: (
    stage: CRMPipelineStage,
    changes: Partial<Pick<CRMPipelineStage, "name" | "probability">>,
  ) => Promise<boolean>;
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
        <StageHelp label="Position" description={POSITION_HELP} />
        <span>Stage name</span>
        <StageHelp label="Win probability" description={PROBABILITY_HELP} />
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
                <StageHelp
                  label={
                    type === "open"
                      ? "Active stages"
                      : `${STAGE_LABELS[type]} outcomes`
                  }
                  description={POSITION_HELP}
                />
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
                    key={`${stage.id}-${props.editVersions[stage.id] ?? 0}`}
                    stage={stage}
                    group={group}
                    index={index}
                    disabled={disabled}
                    editable={props.editable}
                    blockReason={deletionBlock(stages, stage)}
                    onMove={(targetId) => void move(stage.id, targetId)}
                    onEdit={() => props.onEdit(stage)}
                    onUpdate={(changes) => props.onUpdate(stage, changes)}
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
  onUpdate,
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
  onUpdate: (
    changes: Partial<Pick<CRMPipelineStage, "name" | "probability">>,
  ) => Promise<boolean>;
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
        "group/stage relative grid grid-cols-[68px_minmax(0,1fr)_64px] items-center gap-x-3 gap-y-2 border-b border-quiet-divider-light bg-quiet-surface hover:bg-quiet-row-hover px-4 py-3 sm:grid-cols-[76px_minmax(0,1fr)_112px_72px_76px]",
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
        {editable ? (
          <InlineStageField
            label={`Stage name: ${stage.name}`}
            value={stage.name}
            disabled={disabled}
            onSave={(name) => onUpdate({ name })}
          />
        ) : (
          stage.name
        )}
      </div>
      <div className="col-start-2 row-start-2 flex items-center gap-2 sm:col-start-auto sm:row-start-auto">
        <StageHelp
          label={`Win probability for ${stage.name}`}
          description={PROBABILITY_HELP}
        >
          <span
            className="block h-1 w-10 overflow-hidden rounded-full bg-muted"
            aria-hidden="true"
          >
            <span
              className={cn(
                "block h-full rounded-full bg-quiet-text-secondary",
                stage.stage_type === "won" && "bg-quiet-positive",
                stage.stage_type === "lost" && "bg-muted-foreground",
              )}
              style={{ width: `${stage.probability}%` }}
            />
          </span>
        </StageHelp>
        {editable && stage.stage_type === "open" ? (
          <div className="flex min-w-0 items-center text-xs text-quiet-text-tertiary">
            <InlineStageField
              label={`Win probability: ${stage.name}`}
              value={String(stage.probability)}
              numeric
              disabled={disabled}
              onSave={(probability) =>
                onUpdate({ probability: Number(probability) })
              }
            />
            <span>%</span>
          </div>
        ) : (
          <span className="text-xs tabular-nums text-quiet-text-tertiary">
            {stage.probability}%
          </span>
        )}
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
                  className="opacity-0 transition-opacity group-hover/stage:opacity-100 group-focus-within/stage:opacity-100 [@media(hover:none)]:opacity-100"
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

function StageHelp({
  label,
  description,
  children,
}: {
  label: string;
  description: string;
  children?: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  return (
    <Tooltip open={open} onOpenChange={setOpen}>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={`About ${label.toLowerCase()}`}
          onClick={() => setOpen(!open)}
          className={cn(
            "w-fit shrink-0 text-left focus-visible:outline-2 focus-visible:outline-ring",
            !children && "border-b border-dotted border-quiet-field",
            children && "flex min-h-6 items-center",
          )}
        >
          {children ?? label}
        </button>
      </TooltipTrigger>
      <TooltipContent>{description}</TooltipContent>
    </Tooltip>
  );
}

function InlineStageField({
  label,
  value,
  numeric,
  disabled,
  onSave,
}: {
  label: string;
  value: string;
  numeric?: boolean;
  disabled: boolean;
  onSave: (value: string) => Promise<boolean>;
}) {
  const [draft, setDraft] = useState<string | null>(null);
  const [invalid, setInvalid] = useState(false);
  const saving = useRef(false);
  const cancelled = useRef(false);
  const commit = async () => {
    if (cancelled.current) {
      cancelled.current = false;
      return;
    }
    if (disabled || saving.current || draft === null) return;
    const next = draft.trim();
    if (
      !next ||
      (numeric &&
        (!Number.isInteger(Number(next)) ||
          Number(next) < 0 ||
          Number(next) > 100))
    ) {
      setInvalid(true);
      return;
    }
    if (next === value) {
      setDraft(null);
      return;
    }
    saving.current = true;
    try {
      if (await onSave(next)) setDraft(null);
    } finally {
      saving.current = false;
    }
  };
  return (
    <QuietUnderlineInput
      aria-label={label}
      aria-invalid={invalid}
      title={
        invalid
          ? numeric
            ? "Enter a whole number from 0 to 100."
            : "Enter a stage name."
          : undefined
      }
      type={numeric ? "number" : "text"}
      min={numeric ? 0 : undefined}
      max={numeric ? 100 : undefined}
      step={numeric ? 1 : undefined}
      maxLength={numeric ? undefined : 200}
      value={draft ?? value}
      disabled={disabled}
      className={cn(
        "min-w-0 border-b-transparent py-0.5",
        numeric &&
          "w-8 text-right text-xs tabular-nums [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none",
      )}
      onChange={(event) => {
        setDraft(event.target.value);
        setInvalid(false);
      }}
      onBlur={() => void commit()}
      onKeyDown={(event) => {
        if (event.nativeEvent.isComposing) return;
        if (event.key === "Enter") {
          event.preventDefault();
          event.currentTarget.blur();
        } else if (event.key === "Escape") {
          event.preventDefault();
          cancelled.current = true;
          setDraft(null);
          setInvalid(false);
          event.currentTarget.blur();
        }
      }}
    />
  );
}
