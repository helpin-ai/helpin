import { useCallback, useEffect, useMemo, useState } from "react";
import {
  CalendarDays,
  CircleDot,
  GitBranch,
  Hash,
  Layers,
  LayoutGrid,
  Loader2,
  Maximize2,
  Minimize2,
  Sparkles,
  Tag,
  Users,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { TiptapEditor } from "@/components/ui/tiptap-editor";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { PRIORITY_CONFIG, PriorityIcon, StateTypeIcon, StoryTypeIcon } from "@/lib/pmConstants";
import type {
  CreateStoryRequest,
  IterationWithStats,
  Priority,
  StoryType,
  WorkflowWithStates,
  EpicWithStats,
} from "@/lib/pmTypes";
import { pmEpicService } from "@/lib/services/pmEpicService";
import { pmIterationService } from "@/lib/services/pmIterationService";

interface CreateStoryModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  workflow: WorkflowWithStates;
  initialStateId: string;
  onCreate: (payload: CreateStoryRequest) => Promise<void>;
}

const priorityOptions: Priority[] = ["none", "low", "medium", "high", "urgent"];
const storyTypeOptions: StoryType[] = ["feature", "bug", "chore"];

const defaultState = {
  name: "",
  story_type: "feature" as StoryType,
  description: "",
  priority: "none" as Priority,
  estimate: "",
  epic_id: "",
  iteration_id: "",
  deadline: "",
};

const priorityLabels: Record<Priority, string> = {
  none: "None",
  low: "Low",
  medium: "Medium",
  high: "High",
  urgent: "Urgent",
};

const storyTypeLabels: Record<StoryType, string> = {
  feature: "Feature",
  bug: "Bug",
  chore: "Chore",
};

function ChipButton({
  icon: Icon,
  label,
  onClick,
  active = false,
  disabled = false,
}: {
  icon: React.ElementType;
  label: string;
  onClick?: () => void;
  active?: boolean;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`inline-flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs transition-colors
        ${active ? "border-border bg-accent text-foreground" : "border-border/60 bg-transparent text-muted-foreground"}
        ${disabled ? "cursor-not-allowed opacity-50" : "hover:border-border hover:bg-accent hover:text-foreground cursor-pointer"}
      `}
    >
      <Icon className="h-3.5 w-3.5 shrink-0" />
      <span className="whitespace-nowrap">{label}</span>
    </button>
  );
}

function ChipSelect<T extends string>({
  icon: Icon,
  label,
  value,
  options,
  getLabel,
  onChange,
  renderTriggerIcon,
  renderOptionIcon,
}: {
  icon: React.ElementType;
  label: string;
  value: T;
  options: { value: T; label: string }[];
  getLabel: (value: T) => string;
  onChange: (value: T) => void;
  renderTriggerIcon?: (value: T) => React.ReactNode;
  renderOptionIcon?: (value: T) => React.ReactNode;
}) {
  const [open, setOpen] = useState(false);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-transparent px-2 py-1 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-accent hover:text-foreground cursor-pointer"
        >
          {renderTriggerIcon ? renderTriggerIcon(value) : <Icon className="h-3.5 w-3.5 shrink-0" />}
          <span className="whitespace-nowrap">{getLabel(value) || label}</span>
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-44 p-1" align="start">
        <div className="flex flex-col">
          {options.map((option) => (
            <button
              key={option.value}
              type="button"
              className={`flex items-center gap-2 rounded-sm px-2 py-1.5 text-sm transition-colors cursor-pointer
                ${value === option.value ? "bg-accent text-foreground font-medium" : "text-muted-foreground hover:bg-accent hover:text-foreground"}
              `}
              onClick={() => {
                onChange(option.value);
                setOpen(false);
              }}
            >
              {renderOptionIcon ? renderOptionIcon(option.value) : null}
              {option.label}
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}

export function CreateStoryModal({
  open,
  onOpenChange,
  workspaceId,
  workflow,
  initialStateId,
  onCreate,
}: CreateStoryModalProps) {
  const [form, setForm] = useState(defaultState);
  const [stateId, setStateId] = useState(initialStateId);
  const [createMore, setCreateMore] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [editorExpanded, setEditorExpanded] = useState(false);
  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [iterations, setIterations] = useState<IterationWithStats[]>([]);

  useEffect(() => {
    if (!open) return;
    setForm(defaultState);
    setStateId(initialStateId);
    setError(null);
  }, [open, initialStateId]);

  useEffect(() => {
    if (!open) return;
    (async () => {
      const [epicsRes, iterationsRes] = await Promise.all([
        pmEpicService.list(workspaceId, { archived: false }),
        pmIterationService.list(workspaceId, { archived: false }),
      ]);
      setEpics(epicsRes.data ?? []);
      setIterations(iterationsRes.data ?? []);
    })();
  }, [open, workspaceId]);

  const canSubmit = useMemo(
    () => form.name.trim().length > 0 && stateId.trim().length > 0,
    [form.name, stateId]
  );

  const currentStateName = useMemo(
    () => workflow.states.find((s) => s.id === stateId)?.name ?? "State",
    [workflow.states, stateId]
  );

  const currentEpicName = useMemo(() => {
    if (!form.epic_id) return "Epic";
    return epics.find((e) => e.epic.id === form.epic_id)?.epic.name ?? "Epic";
  }, [form.epic_id, epics]);

  const currentIterationName = useMemo(() => {
    if (!form.iteration_id) return "Iteration";
    return (
      iterations.find((i) => i.iteration.id === form.iteration_id)?.iteration
        .name ?? "Iteration"
    );
  }, [form.iteration_id, iterations]);

  const submit = useCallback(async () => {
    if (!canSubmit || submitting) return;
    setSubmitting(true);
    setError(null);
    try {
      await onCreate({
        workspace_id: workspaceId,
        name: form.name.trim(),
        description: form.description.trim() || undefined,
        story_type: form.story_type,
        workflow_id: workflow.workflow.id,
        workflow_state_id: stateId,
        priority: form.priority,
        estimate: form.estimate ? Number(form.estimate) : undefined,
        epic_id: form.epic_id || undefined,
        iteration_id: form.iteration_id || undefined,
        deadline: form.deadline || undefined,
      });

      if (createMore) {
        setForm(defaultState);
        setStateId(initialStateId);
      } else {
        onOpenChange(false);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create story");
    } finally {
      setSubmitting(false);
    }
  }, [
    canSubmit,
    submitting,
    form,
    stateId,
    createMore,
    workspaceId,
    workflow,
    initialStateId,
    onCreate,
    onOpenChange,
  ]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="max-w-6xl sm:max-w-6xl gap-0 overflow-hidden p-0"
        showCloseButton={false}
      >
        <div className="flex h-[85vh] flex-col">
          {/* Header */}
          <div className="px-6 pt-6 pb-2">
            <h2 className="text-lg font-semibold tracking-tight">
              Create new work item
            </h2>
          </div>

          {/* Body */}
          <div className="min-h-0 flex-1 flex flex-col overflow-hidden px-6 py-3 gap-4">
            {/* Workflow badge + Title (always visible, never scrolled) */}
            {!editorExpanded && (
              <div className="inline-flex items-center gap-2 rounded-md border border-border/60 px-3 py-1.5 text-sm font-medium text-foreground self-start">
                <GitBranch className="h-3.5 w-3.5 text-muted-foreground" />
                {workflow.workflow.name}
              </div>
            )}

            {/* Title */}
            <Input
              id="story-title"
              autoFocus
              placeholder="Title"
              className="h-12 shrink-0 border-border/60 text-base shadow-none focus-visible:border-border"
              value={form.name}
              onChange={(event) =>
                setForm((prev) => ({ ...prev, name: event.target.value }))
              }
            />

            {/* Description — Tiptap rich text editor */}
            <div className={`relative flex flex-col ${editorExpanded ? "min-h-0 flex-1" : ""}`}>
              <TiptapEditor
                content={form.description}
                onChange={(html) =>
                  setForm((prev) => ({ ...prev, description: html }))
                }
                placeholder="Press '/' for commands"
                className={editorExpanded ? "min-h-0 flex-1 flex flex-col" : ""}
              />
              <div className="absolute bottom-3 right-3 flex items-center gap-1.5">
                <button
                  type="button"
                  onClick={() => setEditorExpanded((prev) => !prev)}
                  className="inline-flex items-center gap-1 rounded-md bg-muted/80 px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground cursor-pointer"
                  title={editorExpanded ? "Minimize editor" : "Maximize editor"}
                >
                  {editorExpanded ? <Minimize2 className="h-3 w-3" /> : <Maximize2 className="h-3 w-3" />}
                </button>
                <div className="pointer-events-none inline-flex items-center gap-1 rounded-md bg-muted/80 px-2 py-1 text-xs text-muted-foreground">
                  <Sparkles className="h-3 w-3" />
                  AI
                </div>
              </div>
            </div>

            {error ? <p className="text-sm text-destructive">{error}</p> : null}
          </div>

          {/* Metadata chips row — pinned above footer */}
          <div className="flex flex-wrap items-center gap-2 border-t border-border/50 px-6 py-3">
            <ChipSelect
              icon={CircleDot}
              label="State"
              value={stateId}
              options={workflow.states.map((s) => ({
                value: s.id,
                label: s.name,
              }))}
              getLabel={() => currentStateName}
              onChange={setStateId}
              renderTriggerIcon={(v) => {
                const st = workflow.states.find((s) => s.id === v);
                return st ? <StateTypeIcon stateType={st.state_type} className="h-3.5 w-3.5 shrink-0" /> : <CircleDot className="h-3.5 w-3.5 shrink-0" />;
              }}
              renderOptionIcon={(v) => {
                const st = workflow.states.find((s) => s.id === v);
                return st ? <StateTypeIcon stateType={st.state_type} className="h-4 w-4 shrink-0" /> : null;
              }}
            />

            <ChipSelect
              icon={PRIORITY_CONFIG.none.icon}
              label="Priority"
              value={form.priority}
              options={priorityOptions.map((p) => ({
                value: p,
                label: priorityLabels[p],
              }))}
              getLabel={(v) => priorityLabels[v]}
              onChange={(value) =>
                setForm((prev) => ({ ...prev, priority: value }))
              }
              renderTriggerIcon={(v) => <PriorityIcon priority={v} className="h-3.5 w-3.5 shrink-0" />}
              renderOptionIcon={(v) => <PriorityIcon priority={v} className="h-4 w-4 shrink-0" />}
            />

            <ChipSelect
              icon={Hash}
              label="Type"
              value={form.story_type}
              options={storyTypeOptions.map((t) => ({
                value: t,
                label: storyTypeLabels[t],
              }))}
              getLabel={(v) => storyTypeLabels[v]}
              onChange={(value) =>
                setForm((prev) => ({ ...prev, story_type: value }))
              }
              renderTriggerIcon={(v) => <StoryTypeIcon storyType={v} className="h-3.5 w-3.5 shrink-0" />}
              renderOptionIcon={(v) => <StoryTypeIcon storyType={v} className="h-4 w-4 shrink-0" />}
            />

            <ChipButton icon={Users} label="Assignees" disabled />

            <ChipButton icon={Tag} label="Labels" disabled />

            <ChipSelect
              icon={Layers}
              label="Epic"
              value={form.epic_id || "__none__"}
              options={[
                { value: "__none__", label: "No epic" },
                ...epics.map((e) => ({
                  value: e.epic.id,
                  label: e.epic.name,
                })),
              ]}
              getLabel={() => currentEpicName}
              onChange={(value) =>
                setForm((prev) => ({
                  ...prev,
                  epic_id: value === "__none__" ? "" : value,
                }))
              }
            />

            <ChipSelect
              icon={GitBranch}
              label="Iteration"
              value={form.iteration_id || "__none__"}
              options={[
                { value: "__none__", label: "No iteration" },
                ...iterations.map((i) => ({
                  value: i.iteration.id,
                  label: i.iteration.name,
                })),
              ]}
              getLabel={() => currentIterationName}
              onChange={(value) =>
                setForm((prev) => ({
                  ...prev,
                  iteration_id: value === "__none__" ? "" : value,
                }))
              }
            />

            {/* Due date chip */}
            <Popover>
              <PopoverTrigger asChild>
                <button
                  type="button"
                  className="inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-transparent px-2 py-1 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-accent hover:text-foreground cursor-pointer"
                >
                  <CalendarDays className="h-3.5 w-3.5 shrink-0" />
                  <span className="whitespace-nowrap">
                    {form.deadline || "Due date"}
                  </span>
                </button>
              </PopoverTrigger>
              <PopoverContent className="w-auto p-3" align="start">
                <Input
                  type="date"
                  className="h-9"
                  value={form.deadline}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...prev,
                      deadline: event.target.value,
                    }))
                  }
                />
              </PopoverContent>
            </Popover>

            {/* Estimate chip */}
            <Popover>
              <PopoverTrigger asChild>
                <button
                  type="button"
                  className="inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-transparent px-2 py-1 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-accent hover:text-foreground cursor-pointer"
                >
                  <LayoutGrid className="h-3.5 w-3.5 shrink-0" />
                  <span className="whitespace-nowrap">
                    {form.estimate ? `${form.estimate} pts` : "Estimate"}
                  </span>
                </button>
              </PopoverTrigger>
              <PopoverContent className="w-44 p-3" align="start">
                <Input
                  type="number"
                  min={0}
                  placeholder="Points"
                  className="h-9"
                  value={form.estimate}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...prev,
                      estimate: event.target.value,
                    }))
                  }
                />
              </PopoverContent>
            </Popover>
          </div>

          {/* Footer */}
          <div className="flex items-center justify-end gap-4 border-t border-border/50 px-6 py-3">
            <div className="mr-auto flex items-center gap-2">
              <Switch checked={createMore} onCheckedChange={setCreateMore} />
              <span className="text-sm text-muted-foreground">Create more</span>
            </div>

            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={submitting}
            >
              Discard
            </Button>
            <Button
              type="button"
              onClick={submit}
              disabled={!canSubmit || submitting}
            >
              {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              {submitting ? "Saving..." : "Save"}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
