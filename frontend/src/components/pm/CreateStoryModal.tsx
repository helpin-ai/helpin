import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AlertTriangle,
  CalendarDays,
  Check,
  Gauge,
  GitBranch,
  Hash,
  Layers,
  LayoutGrid,
  Loader2,
  Maximize2,
  Minimize2,
  Sparkles,
  Tag,
  User,
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
import { PRIORITY_CONFIG, PriorityIcon, SEVERITY_CONFIG, SeverityIcon, StateTypeIcon, STORY_TYPE_CONFIG, StoryTypeIcon } from "@/lib/pmConstants";
import type {
  CreateStoryRequest,
  Label,
  SprintWithStats,
  Priority,
  Severity,
  StoryType,
  WorkflowWithStates,
  EpicWithStats,
} from "@/lib/pmTypes";
import { pmEpicService } from "@/lib/services/pmEpicService";
import { pmSprintService } from "@/lib/services/pmSprintService";
import { pmLabelService } from "@/lib/services/pmLabelService";
import { LabelPicker } from "@/components/pm/LabelPicker";
import { useWorkspaceTeams } from "@/hooks/useWorkspaceTeams";
import { useWorkspaceMembers } from "@/hooks/useWorkspaceMembers";
import { DatePicker } from "@/components/ui/date-picker";

interface CreateStoryModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  workflow: WorkflowWithStates;
  initialStateId: string;
  onCreate: (payload: CreateStoryRequest) => Promise<void>;
}

const priorityOptions: Priority[] = ["none", "low", "medium", "high", "urgent"];
const severityOptions: Severity[] = ["none", "minor", "major", "critical"];
const storyTypeOptions: StoryType[] = ["feature", "bug", "chore"];

const defaultState = {
  name: "",
  story_type: "feature" as StoryType,
  description: "",
  priority: "none" as Priority,
  severity: "none" as Severity,
  estimate: "",
  epic_id: "",
  sprint_id: "",
  team_id: "",
  owner_id: "",
  requester_id: "",
  deadline: "",
  label_ids: [] as string[],
};

// ── Metadata Row ───────────────────────────────────────────────────

function MetadataRow({
  icon: Icon,
  label,
  children,
}: {
  icon: React.ElementType;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <>
      <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
      <span className="text-xs text-muted-foreground self-center">{label}</span>
      <div className="min-w-0 self-center">{children}</div>
    </>
  );
}

// ── Sidebar Popover Select ─────────────────────────────────────────

function SidebarPopoverSelect<T extends string>({
  value,
  options,
  onChange,
  renderTrigger,
  renderOption,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (value: T) => void;
  renderTrigger: () => React.ReactNode;
  renderOption?: (value: T) => React.ReactNode;
}) {
  const [open, setOpen] = useState(false);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
        >
          {renderTrigger()}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-40 p-0.5" align="start">
        <div className="flex max-h-60 flex-col overflow-y-auto">
          {options.map((option) => (
            <button
              key={option.value}
              type="button"
              className={`flex items-center gap-1.5 rounded-sm px-2 py-1 text-xs transition-colors cursor-pointer
                ${value === option.value ? "bg-accent text-foreground font-medium" : "text-muted-foreground hover:bg-accent hover:text-foreground"}
              `}
              onClick={() => {
                onChange(option.value);
                setOpen(false);
              }}
            >
              {renderOption ? renderOption(option.value) : null}
              <span className="truncate">{option.label}</span>
              {value === option.value && <Check className="ml-auto h-3 w-3 shrink-0" />}
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
  const [sprints, setSprints] = useState<SprintWithStats[]>([]);
  const [labels, setLabels] = useState<Label[]>([]);
  const { teams } = useWorkspaceTeams(workspaceId);
  const { members } = useWorkspaceMembers(workspaceId);

  useEffect(() => {
    if (!open) return;
    setForm(defaultState);
    setStateId(initialStateId);
    setError(null);
  }, [open, initialStateId]);

  useEffect(() => {
    if (!open) return;
    (async () => {
      const [epicsRes, sprintsRes, labelsRes] = await Promise.all([
        pmEpicService.list(workspaceId, { archived: false }),
        pmSprintService.list(workspaceId, { archived: false }),
        pmLabelService.list(workspaceId),
      ]);
      setEpics(epicsRes.data ?? []);
      setSprints(sprintsRes.data ?? []);
      setLabels(labelsRes.data ?? []);
    })();
  }, [open, workspaceId]);

  useEffect(() => {
    setForm((current) => {
      const nextLabelIds = current.label_ids.filter((labelId) => {
        const label = labels.find((entry) => entry.id === labelId);
        if (!label) return false;
        if (!current.team_id) return !label.team_id;
        return !label.team_id || label.team_id === current.team_id;
      });
      if (nextLabelIds.length === current.label_ids.length) return current;
      return { ...current, label_ids: nextLabelIds };
    });
  }, [labels, form.team_id]);

  const canSubmit = useMemo(
    () => form.name.trim().length > 0 && stateId.trim().length > 0,
    [form.name, stateId]
  );

  const currentStateName = useMemo(
    () => workflow.states.find((s) => s.id === stateId)?.name ?? "State",
    [workflow.states, stateId]
  );

  const currentEpicName = useMemo(() => {
    if (!form.epic_id) return "None";
    return epics.find((e) => e.epic.id === form.epic_id)?.epic.name ?? "None";
  }, [form.epic_id, epics]);

  const currentSprintName = useMemo(() => {
    if (!form.sprint_id) return "None";
    return (
      sprints.find((i) => i.sprint.id === form.sprint_id)?.sprint
        .name ?? "None"
    );
  }, [form.sprint_id, sprints]);

  const currentTeamName = useMemo(() => {
    if (!form.team_id) return "None";
    return teams.find((t) => t.id === form.team_id)?.name ?? "None";
  }, [form.team_id, teams]);

  const currentOwnerName = useMemo(() => {
    if (!form.owner_id) return "No owner";
    return members.find((m) => m.user_id === form.owner_id)?.full_name ?? "No owner";
  }, [form.owner_id, members]);

  const currentRequesterName = useMemo(() => {
    if (!form.requester_id) return "No requester";
    return members.find((m) => m.user_id === form.requester_id)?.full_name ?? "No requester";
  }, [form.requester_id, members]);

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
        severity: form.severity !== "none" ? form.severity : undefined,
        estimate: form.estimate ? Number(form.estimate) : undefined,
        epic_id: form.epic_id || undefined,
        sprint_id: form.sprint_id || undefined,
        team_id: form.team_id || undefined,
        owner_id: form.owner_id || undefined,
        requester_id: form.requester_id || undefined,
        deadline: form.deadline || undefined,
        label_ids: form.label_ids.length > 0 ? form.label_ids : undefined,
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

          {/* Two-column grid */}
          <div className="grid min-h-0 flex-1 grid-cols-[1fr_280px] overflow-hidden">
            {/* Left column — title + description */}
            <div className="min-h-0 flex-1 flex flex-col overflow-y-auto px-6 py-3 gap-4">
              {/* Workflow badge */}
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
                  uploadConfig={{ workspaceId, entityType: 'editor_upload', entityId: workspaceId }}
                  teams={teams}
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

            {/* Right sidebar — metadata */}
            <aside className="min-h-0 overflow-y-auto border-l border-border/50 px-4 py-4">
              <div className="grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
                {/* State */}
                <MetadataRow icon={Hash} label="State">
                  <SidebarPopoverSelect
                    value={stateId}
                    options={workflow.states.map((s) => ({ value: s.id, label: s.name }))}
                    onChange={setStateId}
                    renderTrigger={() => {
                      const st = workflow.states.find((s) => s.id === stateId);
                      return (
                        <>
                          {st && <StateTypeIcon stateType={st.state_type} className="h-3.5 w-3.5" />}
                          <span>{currentStateName}</span>
                        </>
                      );
                    }}
                    renderOption={(v) => {
                      const s = workflow.states.find((st) => st.id === v);
                      return s ? <StateTypeIcon stateType={s.state_type} className="h-4 w-4 shrink-0" /> : null;
                    }}
                  />
                </MetadataRow>

                {/* Priority */}
                <MetadataRow icon={Gauge} label="Priority">
                  <SidebarPopoverSelect
                    value={form.priority}
                    options={priorityOptions.map((p) => ({ value: p, label: PRIORITY_CONFIG[p].label }))}
                    onChange={(value) =>
                      setForm((prev) => ({ ...prev, priority: value as Priority }))
                    }
                    renderTrigger={() => (
                      <>
                        <PriorityIcon priority={form.priority} className="h-3.5 w-3.5" />
                        <span>{PRIORITY_CONFIG[form.priority].label}</span>
                      </>
                    )}
                    renderOption={(v) => <PriorityIcon priority={v as Priority} className="h-4 w-4 shrink-0" />}
                  />
                </MetadataRow>

                {/* Type */}
                <MetadataRow icon={Hash} label="Type">
                  <SidebarPopoverSelect
                    value={form.story_type}
                    options={storyTypeOptions.map((t) => ({ value: t, label: STORY_TYPE_CONFIG[t].label }))}
                    onChange={(value) =>
                      setForm((prev) => ({ ...prev, story_type: value as StoryType }))
                    }
                    renderTrigger={() => (
                      <>
                        <StoryTypeIcon storyType={form.story_type} className="h-3.5 w-3.5" />
                        <span>{STORY_TYPE_CONFIG[form.story_type].label}</span>
                      </>
                    )}
                    renderOption={(v) => <StoryTypeIcon storyType={v as StoryType} className="h-4 w-4 shrink-0" />}
                  />
                </MetadataRow>

                {/* Owner */}
                <MetadataRow icon={User} label="Owner">
                  <SidebarPopoverSelect
                    value={form.owner_id || "__none__"}
                    options={[
                      { value: "__none__", label: "No owner" },
                      ...members.map((m) => ({ value: m.user_id, label: m.full_name || m.email })),
                    ]}
                    onChange={(value) =>
                      setForm((prev) => ({
                        ...prev,
                        owner_id: value === "__none__" ? "" : value,
                      }))
                    }
                    renderTrigger={() => <span>{currentOwnerName}</span>}
                  />
                </MetadataRow>

                {/* Requester */}
                <MetadataRow icon={User} label="Requester">
                  <SidebarPopoverSelect
                    value={form.requester_id || "__none__"}
                    options={[
                      { value: "__none__", label: "No requester" },
                      ...members.map((m) => ({ value: m.user_id, label: m.full_name || m.email })),
                    ]}
                    onChange={(value) =>
                      setForm((prev) => ({
                        ...prev,
                        requester_id: value === "__none__" ? "" : value,
                      }))
                    }
                    renderTrigger={() => <span>{currentRequesterName}</span>}
                  />
                </MetadataRow>

                {/* Severity */}
                <MetadataRow icon={AlertTriangle} label="Severity">
                  <SidebarPopoverSelect
                    value={form.severity}
                    options={severityOptions.map((s) => ({ value: s, label: SEVERITY_CONFIG[s].label }))}
                    onChange={(value) =>
                      setForm((prev) => ({ ...prev, severity: value as Severity }))
                    }
                    renderTrigger={() => (
                      <>
                        <SeverityIcon severity={form.severity} className="h-3.5 w-3.5" />
                        <span>{SEVERITY_CONFIG[form.severity].label}</span>
                      </>
                    )}
                    renderOption={(v) => <SeverityIcon severity={v as Severity} className="h-4 w-4 shrink-0" />}
                  />
                </MetadataRow>

                {/* Team */}
                {teams.length > 0 && (
                  <MetadataRow icon={Users} label="Team">
                    <SidebarPopoverSelect
                      value={form.team_id || "__none__"}
                      options={[
                        { value: "__none__", label: "No team" },
                        ...teams.map((t) => ({ value: t.id, label: t.name })),
                      ]}
                      onChange={(value) =>
                        setForm((prev) => ({
                          ...prev,
                          team_id: value === "__none__" ? "" : value,
                        }))
                      }
                      renderTrigger={() => <span>{currentTeamName}</span>}
                    />
                  </MetadataRow>
                )}

                {/* Epic */}
                <MetadataRow icon={Layers} label="Epic">
                  <SidebarPopoverSelect
                    value={form.epic_id || "__none__"}
                    options={[
                      { value: "__none__", label: "No epic" },
                      ...epics.map((e) => ({ value: e.epic.id, label: e.epic.name })),
                    ]}
                    onChange={(value) =>
                      setForm((prev) => ({
                        ...prev,
                        epic_id: value === "__none__" ? "" : value,
                      }))
                    }
                    renderTrigger={() => <span>{currentEpicName}</span>}
                  />
                </MetadataRow>

                {/* Sprint */}
                <MetadataRow icon={GitBranch} label="Sprint">
                  <SidebarPopoverSelect
                    value={form.sprint_id || "__none__"}
                    options={[
                      { value: "__none__", label: "No sprint" },
                      ...sprints.map((i) => ({ value: i.sprint.id, label: i.sprint.name })),
                    ]}
                    onChange={(value) =>
                      setForm((prev) => ({
                        ...prev,
                        sprint_id: value === "__none__" ? "" : value,
                      }))
                    }
                    renderTrigger={() => <span>{currentSprintName}</span>}
                  />
                </MetadataRow>

                {/* Estimate */}
                <MetadataRow icon={LayoutGrid} label="Estimate">
                  <Popover>
                    <PopoverTrigger asChild>
                      <button
                        type="button"
                        className="inline-flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
                      >
                        {form.estimate ? `${form.estimate} pts` : "None"}
                      </button>
                    </PopoverTrigger>
                    <PopoverContent className="w-36 p-3" align="start">
                      <Input
                        type="number"
                        min={0}
                        placeholder="Points"
                        className="h-8 text-sm"
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
                </MetadataRow>

                {/* Due date */}
                <MetadataRow icon={CalendarDays} label="Due date">
                  <DatePicker
                    value={form.deadline}
                    onChange={(v) => setForm((prev) => ({ ...prev, deadline: v }))}
                    placeholder="None"
                    disablePast
                    className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
                  />
                </MetadataRow>

                {/* Labels */}
                <MetadataRow icon={Tag} label="Labels">
                  <LabelPicker
                    workspaceId={workspaceId}
                    teamId={form.team_id || undefined}
                    labels={labels}
                    selectedLabelIds={form.label_ids}
                    onLabelsChange={setLabels}
                    onChange={(ids) => setForm((prev) => ({ ...prev, label_ids: ids }))}
                  />
                </MetadataRow>
              </div>
            </aside>
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
