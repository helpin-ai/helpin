import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Editor } from "@tiptap/react";
import { useNavigate } from "@tanstack/react-router";
import {
  Alert01Icon,
  Calendar03Icon,
  Tick01Icon,
  CheckListIcon,
  SourceCodeIcon,
  LinkSquare01Icon as ExternalLinkIcon,
  File01Icon,
  DashboardSpeed01Icon,
  DragDropVerticalIcon,
  HashtagIcon,
  Layers01Icon,
  LayoutGridIcon,
  Link01Icon,
  Loading01Icon,
  AttachmentIcon,
  PlusSignIcon,
  ArrowReloadHorizontalIcon,
  SparklesIcon,
  Tag01Icon,
  Delete01Icon,
  Upload01Icon,
  UserIcon,
  UserGroupIcon,
  Cancel01Icon,
} from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { TiptapEditor } from "@/components/ui/tiptap-editor";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command";
import { PRIORITY_CONFIG, PriorityIcon, SEVERITY_CONFIG, SeverityIcon, SprintIcon, TASK_TYPE_CONFIG, TaskTypeIcon } from "@/lib/pmConstants";
import type {
  CreateTaskRequest,
  Label,
  SprintWithStats,
  Priority,
  Severity,
  Task,
  TaskType,
  WorkflowWithStates,
  EpicWithStats,
} from "@/lib/pmTypes";
import { pmEpicService } from "@/lib/services/pmEpicService";
import { pmSprintService } from "@/lib/services/pmSprintService";
import { pmLabelService } from "@/lib/services/pmLabelService";
import { pmTaskTemplateService } from "@/lib/services/pmTaskTemplateService";
import { pmWorkflowService } from "@/lib/services/pmWorkflowService";
import type { TaskTemplate } from "@/lib/pmTypes";
import { LabelPicker } from "@/components/pm/LabelPicker";
import { EstimatePicker } from "@/components/pm/EstimatePicker";
import { useAccessibleTeams } from "@/hooks/useAccessibleTeams";
import { useAssignableWorkspaceMembers } from "@/hooks/useAssignableWorkspaceMembers";
import { useTeamFieldVisibilityForTeam } from "@/hooks/queries/useSettings";
import { useSession } from "@/hooks/queries/useSession";
import { DatePicker } from "@/components/ui/date-picker";
import { MemberPickerPopover, MultiMemberPickerPopover } from "@/components/pm/MemberPickerPopover";
import { OwnerAvatarStack } from "@/components/pm/OwnerAvatarStack";
import { SidebarPopoverSelect } from '@/components/pm/SidebarPopoverSelect';
import { TaskStateSelectContent } from '@/components/pm/task-detail/TaskStateSelectContent';
import { UserAvatar } from "@/components/pm/UserAvatar";
import { filterMentionTeams } from "@/components/pm/mentionSuggestions";
import { extractInlineAttachmentIds } from "@/components/pm/editorImageAttachments";
import { buildAssignableMemberNameMap, findAssignableMember } from "@/lib/assignableMembers";
import { pmAttachmentService } from "@/lib/services/pmAttachmentService";
import { pmRecurringTemplateService } from "@/lib/services/pmRecurringTemplateService";
import { uploadToS3 } from "@/lib/api";
import { buildSprintOptionGroups } from "@/lib/pmSprintOptions";
import { isEpicSelectableForTaskTeam } from '@/components/pm/task-detail/taskPlanningScope';
import { htmlToMarkdown, markdownToHtml } from "@/lib/tiptapMarkdown";
import { toast } from "sonner";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { RecurringTemplateForm, type RecurringTemplateFormValue } from "@/components/pm/RecurringTemplateForm";
import { formatRecurringRuleSummary } from "@/components/pm/recurringTemplateUtils";
import { RecurringTemplateBadge } from "@/components/pm/RecurringTemplateBadge";
import { showEntityCreatedToast } from "@/components/ui/entity-created-toast";
import { useWorkspaceStore } from "@/stores/workspaceStore";

interface CreateTaskModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  workflow?: WorkflowWithStates;
  initialStateId?: string;
  initialTeamId?: string;
  initialEpicId?: string;
  initialOwnerMemberId?: string;
  initialSprintId?: string;
  onCreate?: (payload: CreateTaskRequest) => Promise<CreatedTaskResult | void>;
  mode?: 'task' | 'template';
  editingTemplate?: TaskTemplate | null;
  onSaveTemplate?: (template: TaskTemplate) => void;
}

interface CreatedTaskResult {
  id: string;
  task?: Pick<Task, 'id' | 'name' | 'display_id' | 'task_key'>;
}

const priorityOptions: Priority[] = ["none", "low", "medium", "high", "urgent"];
const severityOptions: Severity[] = ["none", "minor", "major", "critical"];
const taskTypeOptions: TaskType[] = ["feature", "bug", "chore"];

interface ChecklistTemplateItem {
  text: string;
  position?: number;
}

interface ExternalLinkItem {
  url: string;
  title?: string;
}

const defaultState = {
  name: "",
  task_type: "feature" as TaskType,
  description: "",
  priority: "medium" as Priority,
  severity: "none" as Severity,
  estimate: "",
  epic_id: "",
  sprint_id: "",
  team_id: "",
  owner_member_ids: [] as string[],
  requester_member_id: "",
  deadline: "",
  label_ids: [] as string[],
  checklist_items: [] as ChecklistTemplateItem[],
  external_links: [] as ExternalLinkItem[],
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

function GroupedSidebarPopoverSelect<T extends string>({
  value,
  groups,
  onChange,
  renderTrigger,
  searchPlaceholder = 'Search...',
  emptyLabel = 'No options',
  showGroupHeadings = true,
}: {
  value: T;
  groups: Array<{ key: string; label: string; options: { value: T; label: string }[] }>;
  onChange: (value: T) => void;
  renderTrigger: () => React.ReactNode;
  searchPlaceholder?: string;
  emptyLabel?: string;
  showGroupHeadings?: boolean;
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
      <PopoverContent className="w-48 p-0.5" align="start">
        <Command>
          <CommandInput placeholder={searchPlaceholder} className="h-8 text-xs" />
          <CommandList className="max-h-56">
            <CommandEmpty>{emptyLabel}</CommandEmpty>
            <CommandGroup>
              <CommandItem
                className="gap-1.5 px-2 py-1 text-xs"
                value="No sprint"
                onSelect={() => {
                  onChange('__none__' as T);
                  setOpen(false);
                }}
              >
                <span>No sprint</span>
                {value === '__none__' ? <Tick01Icon className="ml-auto h-3 w-3 shrink-0" /> : null}
              </CommandItem>
            </CommandGroup>
            {showGroupHeadings ? (
              groups.map((group) => (
                <CommandGroup key={group.key} heading={group.label}>
                  {group.options.map((option) => (
                    <CommandItem
                      className="gap-1.5 px-2 py-1 text-xs"
                      key={option.value}
                      value={`${group.label} ${option.label}`}
                      onSelect={() => {
                        onChange(option.value);
                        setOpen(false);
                      }}
                    >
                      <span className="truncate">{option.label}</span>
                      {value === option.value ? <Tick01Icon className="ml-auto h-3 w-3 shrink-0" /> : null}
                    </CommandItem>
                  ))}
                </CommandGroup>
              ))
            ) : (
              <CommandGroup>
                {groups.flatMap((group) => group.options).map((option) => (
                  <CommandItem
                    className="gap-1.5 px-2 py-1 text-xs"
                    key={option.value}
                    value={option.label}
                    onSelect={() => {
                      onChange(option.value);
                      setOpen(false);
                    }}
                  >
                    <span className="truncate">{option.label}</span>
                    {value === option.value ? <Tick01Icon className="ml-auto h-3 w-3 shrink-0" /> : null}
                  </CommandItem>
                ))}
              </CommandGroup>
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

export function CreateTaskModal({
  open,
  onOpenChange,
  workspaceId,
  workflow,
  initialStateId,
  initialTeamId,
  initialEpicId,
  initialOwnerMemberId,
  initialSprintId,
  onCreate,
  mode = 'task',
  editingTemplate,
  onSaveTemplate,
}: CreateTaskModalProps) {
  const navigate = useNavigate();
  const { currentWorkspace } = useWorkspaceStore();
  const isTemplateMode = mode === 'template';
  const [form, setForm] = useState(defaultState);
  const initialDescRef = useRef('');
  const [stateId, setStateId] = useState(initialStateId ?? '');
  const [createMore, setCreateMore] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const confirm = useConfirm();
  const [descriptionPendingUploads, setDescriptionPendingUploads] = useState(0);
  const [descriptionEditorKey, setDescriptionEditorKey] = useState(0);
  const [descriptionMode, setDescriptionMode] = useState<'rich' | 'markdown'>('rich');
  const [sourceMarkdown, setSourceMarkdown] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [taskTypeDirty, setTaskTypeDirty] = useState(false);
  const [pendingFiles, setPendingFiles] = useState<File[]>([]);
  const [showChecklist, setShowChecklist] = useState(false);
  const [showExternalLinks, setShowExternalLinks] = useState(false);
  const [showAttachments, setShowAttachments] = useState(false);
  const [recurringDraft, setRecurringDraft] = useState<RecurringTemplateFormValue | null>(null);
  const [recurringDialogOpen, setRecurringDialogOpen] = useState(false);
  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [sprints, setSprints] = useState<SprintWithStats[]>([]);
  const [labels, setLabels] = useState<Label[]>([]);
  const [templates, setTemplates] = useState<TaskTemplate[]>([]);
  const descriptionEditorRef = useRef<Editor | null>(null);
  const { teams } = useAccessibleTeams(workspaceId);
  const teamsRef = useRef(teams);
  teamsRef.current = teams;
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const fieldVis = useTeamFieldVisibilityForTeam(workspaceId, form.team_id || null);
  const { data: membership } = useSession(workspaceId);
  const currentMemberId = membership?.id ?? '';
  const memberNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );
  const selectedTeam = useMemo(() => teams.find((team) => team.id === form.team_id), [teams, form.team_id]);
  const teamSprintsEnabled = selectedTeam?.sprints_enabled !== false;
  const selectedTeamDefaultTaskType = useMemo(
    () => (selectedTeam?.default_task_type as TaskType | undefined) ?? 'feature',
    [selectedTeam],
  );
  const mentionTeams = useMemo(
    () => filterMentionTeams(teams, form.team_id ? [form.team_id] : []),
    [teams, form.team_id],
  );
  const cleanupInlineDraftUploads = useCallback(async () => {
    const attachmentIds = extractInlineAttachmentIds(form.description);
    if (attachmentIds.length === 0) return;
    await Promise.allSettled(
      attachmentIds.map((attachmentId) => pmAttachmentService.remove(workspaceId, attachmentId)),
    );
  }, [form.description, workspaceId]);

  useEffect(() => {
    if (!open) return;
    setDescriptionEditorKey((current) => current + 1);
    if (isTemplateMode && editingTemplate) {
      setForm({
        name: editingTemplate.name,
        description: editingTemplate.description || '',
        task_type: (editingTemplate.task_type as TaskType) || 'feature',
        priority: (editingTemplate.priority as Priority) || 'none',
        severity: (editingTemplate.severity as Severity) || 'none',
        estimate: editingTemplate.estimate !== undefined && editingTemplate.estimate !== null ? String(editingTemplate.estimate) : '',
        epic_id: editingTemplate.epic_id || '',
        sprint_id: editingTemplate.sprint_id || '',
        team_id: editingTemplate.team_id || initialTeamId || '',
        owner_member_ids: editingTemplate.owner_member_id ? [editingTemplate.owner_member_id] : [],
        requester_member_id: '',
        deadline: editingTemplate.deadline || '',
        label_ids: editingTemplate.label_ids ? (() => { try { return JSON.parse(editingTemplate.label_ids!); } catch { return []; } })() : [],
        checklist_items: editingTemplate.checklist_items ? (() => { try { return JSON.parse(editingTemplate.checklist_items!); } catch { return []; } })() : [],
        external_links: editingTemplate.external_links ? (() => { try { return JSON.parse(editingTemplate.external_links!); } catch { return []; } })() : [],
      });
      setTaskTypeDirty(true);
      initialDescRef.current = editingTemplate.description || '';
      // Auto-open sections that have data
      if (editingTemplate.checklist_items) { try { if (JSON.parse(editingTemplate.checklist_items).length > 0) setShowChecklist(true); } catch {} }
      if (editingTemplate.external_links) { try { if (JSON.parse(editingTemplate.external_links).length > 0) setShowExternalLinks(true); } catch {} }
    } else {
      const effectiveTeamId = initialTeamId ?? teamsRef.current[0]?.id ?? '';
      const initialTeam = teamsRef.current.find((team) => team.id === effectiveTeamId);
      setForm({
        ...defaultState,
        task_type: (initialTeam?.default_task_type as TaskType | undefined) ?? 'feature',
        requester_member_id: isTemplateMode ? '' : currentMemberId,
        team_id: effectiveTeamId,
        epic_id: initialEpicId ?? '',
        owner_member_ids: initialOwnerMemberId ? [initialOwnerMemberId] : [],
        sprint_id: initialSprintId ?? '',
      });
      setTaskTypeDirty(false);
      initialDescRef.current = '';
    }
    setStateId(initialStateId ?? '');
    setError(null);
    setPendingFiles([]);
    setDescriptionMode('rich');
    setSourceMarkdown('');
    setShowChecklist(isTemplateMode);
    setShowExternalLinks(isTemplateMode);
    setShowAttachments(false);
    setRecurringDraft(null);
    setRecurringDialogOpen(false);
  // eslint-disable-next-line react-hooks/exhaustive-deps -- `teams` excluded: only used to derive initial task type; including it causes form reset on background refetch
  }, [open, initialStateId, initialTeamId, initialEpicId, initialOwnerMemberId, initialSprintId, currentMemberId, isTemplateMode, editingTemplate]);

  useEffect(() => {
    if (taskTypeDirty || (isTemplateMode && editingTemplate)) return;
    setForm((current) => {
      if (current.task_type === selectedTeamDefaultTaskType) return current;
      return { ...current, task_type: selectedTeamDefaultTaskType };
    });
  }, [selectedTeamDefaultTaskType, taskTypeDirty, isTemplateMode, editingTemplate]);

  useEffect(() => {
    if (!open) return;
    (async () => {
      if (isTemplateMode) {
        const [epicsRes, sprintsRes, labelsRes] = await Promise.all([
          pmEpicService.list(workspaceId, { archived: false }),
          pmSprintService.list(workspaceId, { archived: false }),
          pmLabelService.list(workspaceId),
        ]);
        setEpics(epicsRes.data ?? []);
        setSprints(sprintsRes.data ?? []);
        setLabels(labelsRes.data ?? []);
      } else {
        const [epicsRes, sprintsRes, labelsRes, templatesRes] = await Promise.all([
          pmEpicService.list(workspaceId, { archived: false }),
          pmSprintService.list(workspaceId, { archived: false }),
          pmLabelService.list(workspaceId),
          pmTaskTemplateService.list(workspaceId, { archived: false }),
        ]);
        setEpics(epicsRes.data ?? []);
        setSprints(sprintsRes.data ?? []);
        setLabels(labelsRes.data ?? []);
        setTemplates(templatesRes.data ?? []);
      }
    })();
  }, [open, workspaceId, isTemplateMode]);

  useEffect(() => {
    if (descriptionMode !== 'markdown') return;
    setSourceMarkdown(htmlToMarkdown(form.description));
  }, [descriptionMode, form.description]);

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

  useEffect(() => {
    if (!form.sprint_id) return;
    const selectedSprint = sprints.find((entry) => entry.sprint.id === form.sprint_id);
    if (!selectedSprint) return;
    if (!form.team_id || selectedSprint.sprint.team_id === form.team_id) return;
    setForm((current) => (current.sprint_id ? { ...current, sprint_id: '' } : current));
  }, [form.sprint_id, form.team_id, sprints]);

  useEffect(() => {
    if (!form.epic_id) return;
    const selectedEpic = epics.find((entry) => entry.epic.id === form.epic_id);
    if (!selectedEpic) return;
    if (isEpicSelectableForTaskTeam(selectedEpic.epic.team_id ?? null, form.team_id || null)) return;
    setForm((current) => (current.epic_id ? { ...current, epic_id: '' } : current));
  }, [epics, form.epic_id, form.team_id]);

  const canSubmit = useMemo(
    () =>
      descriptionPendingUploads === 0 &&
      form.name.trim().length > 0 &&
      form.team_id.trim().length > 0 &&
      (isTemplateMode || stateId.trim().length > 0),
    [descriptionPendingUploads, form.name, form.team_id, stateId, isTemplateMode]
  );

  const currentStateName = useMemo(
    () => workflow?.states.find((s) => s.id === stateId)?.name ?? "State",
    [workflow?.states, stateId]
  );

  const currentEpicName = useMemo(() => {
    if (!form.epic_id) return "None";
    return epics.find((e) => e.epic.id === form.epic_id)?.epic.name ?? "None";
  }, [form.epic_id, epics]);

  const availableEpics = useMemo(
    () =>
      epics.filter((entry) =>
        isEpicSelectableForTaskTeam(entry.epic.team_id ?? null, form.team_id || null),
      ),
    [epics, form.team_id],
  );

  // Group epics by lifecycle: not started → in progress → completed.
  const epicGroups = useMemo(() => {
    const notStarted: typeof availableEpics = [];
    const inProgress: typeof availableEpics = [];
    const completed: typeof availableEpics = [];
    for (const entry of availableEpics) {
      if (entry.epic.completed) completed.push(entry);
      else if (entry.epic.started) inProgress.push(entry);
      else notStarted.push(entry);
    }
    return [
      { label: undefined as string | undefined, options: [{ value: "__none__", label: "No epic" }] },
      { label: "Not started", options: notStarted.map((e) => ({ value: e.epic.id, label: e.epic.name })) },
      { label: "In progress", options: inProgress.map((e) => ({ value: e.epic.id, label: e.epic.name })) },
      { label: "Completed", options: completed.map((e) => ({ value: e.epic.id, label: e.epic.name })) },
    ];
  }, [availableEpics]);

  const currentSprintName = useMemo(() => {
    if (!form.sprint_id) return "None";
    return (
      sprints.find((i) => i.sprint.id === form.sprint_id)?.sprint
        .name ?? "None"
    );
  }, [form.sprint_id, sprints]);

  const sprintOptionGroups = useMemo(
    () => buildSprintOptionGroups(sprints, teams, form.team_id || null),
    [form.team_id, sprints, teams],
  );

  const currentTeamName = useMemo(() => {
    if (!form.team_id) return "Select team";
    return teams.find((t) => t.id === form.team_id)?.name ?? "Select team";
  }, [form.team_id, teams]);

  const currentOwnerName = useMemo(() => {
    if (form.owner_member_ids.length === 0) return "No owner";
    return form.owner_member_ids
      .map((ownerId) => memberNameMap.get(ownerId) ?? "Unknown")
      .join(", ");
  }, [form.owner_member_ids, memberNameMap]);

  const currentRequesterName = useMemo(() => {
    if (!form.requester_member_id) return "No requester";
    return memberNameMap.get(form.requester_member_id) ?? "No requester";
  }, [form.requester_member_id, memberNameMap]);

  const recurringDraftSummary = useMemo(() => {
    if (!recurringDraft) return 'Not recurring';
    return formatRecurringRuleSummary(recurringDraft.config);
  }, [recurringDraft]);

  const openMarkdownMode = useCallback(() => {
    const editor = descriptionEditorRef.current;
    const markdown =
      ((editor?.storage as Record<string, unknown> | undefined)?.markdown as { getMarkdown?: () => string } | undefined)?.getMarkdown?.() ??
      htmlToMarkdown(form.description);
    setSourceMarkdown(markdown);
    setDescriptionMode('markdown');
  }, [form.description]);

  const applyMarkdownSource = useCallback(() => {
    setForm((prev) => ({ ...prev, description: markdownToHtml(sourceMarkdown) }));
    setDescriptionMode('rich');
  }, [sourceMarkdown]);

  const discardMarkdownSource = useCallback(() => {
    setSourceMarkdown(htmlToMarkdown(form.description));
    setDescriptionMode('rich');
  }, [form.description]);

  const resolveSubmitWorkflow = useCallback(async () => {
    if (!workflow) {
      throw new Error('Workflow is required');
    }

    if (!form.team_id || workflow.workflow.team_id === form.team_id) {
      return { workflowId: workflow.workflow.id, workflowStateId: stateId };
    }

    const resolved = await pmWorkflowService.resolveTeamWorkflow(workspaceId, form.team_id);
    if (resolved.error || !resolved.data) {
      throw new Error(resolved.error ?? 'Failed to resolve team workflow');
    }

    const resolvedData = resolved.data;
    const currentState = workflow.states.find((state) => state.id === stateId);
    const nextState =
      resolvedData.states.find((state) => state.id === stateId) ??
      (currentState
        ? resolvedData.states.find((state) => state.state_type === currentState.state_type) ??
          resolvedData.states.find((state) => state.name === currentState.name)
        : undefined) ??
      (resolvedData.workflow.default_state_id
        ? resolvedData.states.find((state) => state.id === resolvedData.workflow.default_state_id)
        : undefined) ??
      resolvedData.states[0];

    if (!nextState) {
      throw new Error('No workflow state available for the selected team');
    }

    return {
      workflowId: resolvedData.workflow.id,
      workflowStateId: nextState.id,
    };
  }, [form.team_id, workflow, stateId, workspaceId]);

  const submit = useCallback(async () => {
    if (!canSubmit || submitting) return;
    setSubmitting(true);
    setError(null);
    try {
      const descriptionForSubmit =
        descriptionMode === 'markdown' ? markdownToHtml(sourceMarkdown) : form.description;

      if (isTemplateMode) {
        const labelIds = form.label_ids.length > 0 ? JSON.stringify(form.label_ids) : undefined;
        const filteredChecklist = form.checklist_items.filter((i) => i.text.trim());
        const filteredLinks = form.external_links.filter((l) => l.url.trim());
        const checklistJson = filteredChecklist.length > 0 ? JSON.stringify(filteredChecklist) : undefined;
        const externalLinksJson = filteredLinks.length > 0 ? JSON.stringify(filteredLinks) : undefined;
        const templatePayload = {
          name: form.name.trim(),
          description: descriptionForSubmit.trim() || undefined,
          task_type: form.task_type !== 'feature' ? form.task_type : undefined,
          priority: form.priority !== 'none' ? form.priority : undefined,
          severity: form.severity !== 'none' ? form.severity : undefined,
          estimate: form.estimate ? Number(form.estimate) : undefined,
          team_id: form.team_id || undefined,
          label_ids: labelIds,
          owner_member_id: form.owner_member_ids[0] || undefined,
          epic_id: form.epic_id || undefined,
          sprint_id: form.sprint_id || undefined,
          deadline: form.deadline || undefined,
          checklist_items: checklistJson,
          external_links: externalLinksJson,
        };
        if (editingTemplate) {
          const { data, error: err } = await pmTaskTemplateService.update(workspaceId, editingTemplate.id, templatePayload);
          if (err) throw new Error(err);
          if (data && onSaveTemplate) onSaveTemplate(data);
          toast.success('Template updated');
        } else {
          const { data, error: err } = await pmTaskTemplateService.create({ workspace_id: workspaceId, ...templatePayload });
          if (err) throw new Error(err);
          if (data && onSaveTemplate) onSaveTemplate(data);
          toast.success('Template created');
        }
        onOpenChange(false);
      } else {
        const { workflowId, workflowStateId } = await resolveSubmitWorkflow();
        const result = await onCreate!({
          workspace_id: workspaceId,
          name: form.name.trim(),
          description: descriptionForSubmit.trim() || undefined,
          task_type: form.task_type,
          workflow_id: workflowId,
          workflow_state_id: workflowStateId,
          priority: form.priority,
          severity: form.severity !== "none" ? form.severity : undefined,
          estimate: form.estimate ? Number(form.estimate) : undefined,
          epic_id: form.epic_id || undefined,
          sprint_id: form.sprint_id || undefined,
          team_id: form.team_id || undefined,
          owner_member_ids: form.owner_member_ids.length > 0 ? form.owner_member_ids : undefined,
          requester_member_id: form.requester_member_id || undefined,
          deadline: form.deadline || undefined,
          label_ids: form.label_ids.length > 0 ? form.label_ids : undefined,
          checklist_items: (() => { const f = form.checklist_items.filter((i) => i.text.trim()); return f.length > 0 ? f : undefined; })(),
          external_links: (() => { const f = form.external_links.filter((l) => l.url.trim()); return f.length > 0 ? f : undefined; })(),
        });

        // Upload pending files after task creation.
        if (result?.id && pendingFiles.length > 0) {
          for (const file of pendingFiles) {
            try {
              const { data: initData } = await pmAttachmentService.initiateUpload(workspaceId, {
                entity_type: 'task',
                entity_id: result.id,
                file_name: file.name,
                file_size: file.size,
                content_type: file.type || 'application/octet-stream',
              });
              if (!initData) continue;
              const uploadResult = await uploadToS3(initData.url, file, undefined, { 'x-amz-acl': 'public-read' });
              if (uploadResult.ok) {
                await pmAttachmentService.confirmUpload(workspaceId, initData.attachment.id);
              }
            } catch {
              // Non-blocking — task already created
            }
          }
        }

        let recurringSetupError: string | null = null;
        if (result?.id && recurringDraft) {
          const { error: recurringError } = await pmRecurringTemplateService.create({
            workspace_id: workspaceId,
            task_id: result.id,
            title: recurringDraft.title.trim() || form.name.trim(),
            description: recurringDraft.description.trim() || undefined,
            config: recurringDraft.config,
          });
          if (recurringError) {
            recurringSetupError = recurringError;
          }
        }

        if (recurringSetupError) {
          toast.error(`Task created, but recurring setup failed: ${recurringSetupError}`);
        } else {
          if (result?.id) {
            const createdTitle = result.task?.name || form.name.trim();
            const taskKey = result.task?.task_key
              || (result.task?.display_id !== undefined ? `#${result.task.display_id}` : undefined);
            showEntityCreatedToast({
              entityLabel: 'Task',
              title: createdTitle,
              subtitle: recurringDraft ? 'Recurring schedule added.' : undefined,
              identifier: taskKey ? { label: 'Story ID', value: taskKey } : undefined,
              tone: 'pm',
              icon: TASK_TYPE_CONFIG[form.task_type].icon,
              onOpen: currentWorkspace?.slug
                ? () => navigate({ to: `/w/${currentWorkspace.slug}/pm/tasks/${result.id}` } as any)
                : undefined,
            });
          } else {
            toast.success(recurringDraft ? 'Task created with recurring schedule' : 'Task created');
          }
        }

        if (createMore) {
          const resetTeam = teams.find((team) => team.id === (initialTeamId ?? ''));
          setDescriptionEditorKey((current) => current + 1);
          setDescriptionMode('rich');
          setSourceMarkdown('');
          setForm({
            ...defaultState,
            task_type: (resetTeam?.default_task_type as TaskType | undefined) ?? 'feature',
            requester_member_id: currentMemberId,
            team_id: initialTeamId ?? '',
            epic_id: initialEpicId ?? '',
            owner_member_ids: initialOwnerMemberId ? [initialOwnerMemberId] : [],
            sprint_id: initialSprintId ?? '',
          });
          setTaskTypeDirty(false);
          setStateId(initialStateId ?? '');
          setPendingFiles([]);
          setRecurringDraft(null);
        } else {
          onOpenChange(false);
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : isTemplateMode ? "Failed to save template" : "Failed to create task");
    } finally {
      setSubmitting(false);
    }
  }, [
    canSubmit,
    submitting,
    form,
    stateId,
    descriptionMode,
    createMore,
    workspaceId,
    workflow,
    resolveSubmitWorkflow,
    initialStateId,
    currentMemberId,
    initialTeamId,
    initialEpicId,
    initialOwnerMemberId,
    initialSprintId,
    isTemplateMode,
    editingTemplate,
    onCreate,
    onOpenChange,
    onSaveTemplate,
    pendingFiles,
    recurringDraft,
    sourceMarkdown,
    teams,
  ]);

  const stripHtml = (html: string) => html.replace(/<[^>]*>/g, '').trim();
  const currentDescriptionForCompare =
    descriptionMode === 'markdown' ? markdownToHtml(sourceMarkdown) : form.description;
  const hasUnsavedChanges =
    form.name.trim() !== '' || stripHtml(currentDescriptionForCompare) !== stripHtml(initialDescRef.current);

  const handleOpenChange = async (nextOpen: boolean) => {
    if (!nextOpen && hasUnsavedChanges) {
      const ok = await confirm({
        title: 'Discard changes?',
        description: 'You have unsaved changes that will be lost.',
        confirmText: 'Discard',
        variant: 'destructive',
      });
      if (!ok) return;
    }
    if (!nextOpen) {
      void cleanupInlineDraftUploads();
    }
    onOpenChange(nextOpen);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        className="max-w-6xl sm:max-w-6xl gap-0 overflow-hidden p-0"
        showCloseButton={false}
      >
        <div className="flex h-[85vh] max-h-[960px] flex-col">
          {/* Header */}
          <div className="flex items-center justify-between border-b border-border/60 px-6 pt-4 pb-3">
            <span className="text-lg font-semibold">
              {isTemplateMode ? (editingTemplate ? 'Edit template' : 'Create template') : 'Create task'}
            </span>
            <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={() => handleOpenChange(false)}>
              <Cancel01Icon className="h-4 w-4" />
            </Button>
          </div>

          {/* Two-column grid */}
          <div className="grid min-h-0 flex-1 grid-cols-[1fr_300px] overflow-hidden">
            {/* Left column — title + description */}
            <div className="min-h-0 flex-1 flex flex-col overflow-y-auto px-6 py-3 gap-4">

              {/* Title */}
              <Input
                id="task-title"
                autoFocus
                placeholder={isTemplateMode ? "Template name" : "Title"}
                className="h-12 shrink-0 border-border/60 text-base shadow-none focus-visible:border-border"
                value={form.name}
                onChange={(event) =>
                  setForm((prev) => ({ ...prev, name: event.target.value }))
                }
                onKeyDown={(e) => {
                  if (e.key === 'Tab' && !e.shiftKey) {
                    e.preventDefault();
                    const editor = e.currentTarget.closest('.flex')?.querySelector<HTMLElement>('.tiptap.ProseMirror');
                    editor?.focus();
                  }
                }}
              />

              {/* Description — Tiptap rich text editor */}
              <div className="relative flex flex-col min-h-0 flex-1">
                {descriptionMode === 'markdown' ? (
                  <div className="flex min-h-0 flex-1 flex-col rounded-2xl border border-transparent bg-input/50">
                    <div className="flex items-center justify-between border-b border-border/40 px-3 py-2">
                      <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                        Markdown Source
                      </span>
                      <div className="flex items-center gap-1.5">
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="h-7 gap-1.5 text-xs"
                          onClick={discardMarkdownSource}
                        >
                          <Cancel01Icon className="h-3 w-3" />
                          Discard
                        </Button>
                        <Button
                          type="button"
                          size="sm"
                          className="h-7 gap-1.5 text-xs"
                          onClick={applyMarkdownSource}
                        >
                          <Tick01Icon className="h-3 w-3" />
                          Apply
                        </Button>
                      </div>
                    </div>
                    <textarea
                      value={sourceMarkdown}
                      onChange={(event) => setSourceMarkdown(event.target.value)}
                      className="min-h-0 flex-1 resize-none border-0 bg-transparent p-4 font-mono text-sm leading-relaxed text-foreground placeholder:text-muted-foreground/40 focus:outline-none"
                      placeholder="## Write the task in Markdown"
                      spellCheck={false}
                    />
                  </div>
                ) : (
                  <TiptapEditor
                    key={descriptionEditorKey}
                    content={form.description}
                    onChange={(html) =>
                      setForm((prev) => ({ ...prev, description: html }))
                    }
                    placeholder="Add a description..."
                    className="min-h-0 flex-1 flex flex-col"
                    uploadConfig={{ workspaceId, entityType: 'editor_upload', entityId: workspaceId }}
                    onUploadStateChange={setDescriptionPendingUploads}
                    teams={mentionTeams}
                    members={assignableMembers}
                    onEditorReady={(editor) => {
                      descriptionEditorRef.current = editor;
                    }}
                  />
                )}
              </div>

              {/* ── Action bar — toggle pills ─────────────────────── */}
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
                      showChecklist
                        ? 'border-primary/30 bg-primary/10 text-primary'
                        : 'border-border/60 text-muted-foreground hover:bg-accent'
                    }`}
                    onClick={() => setShowChecklist((v) => !v)}
                  >
                    <CheckListIcon className="h-3 w-3" />
                    Checklist
                    {form.checklist_items.length > 0 && (
                      <span className="text-[10px] opacity-70">({form.checklist_items.length})</span>
                    )}
                  </button>
                  <button
                    type="button"
                    className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
                      showExternalLinks
                        ? 'border-primary/30 bg-primary/10 text-primary'
                        : 'border-border/60 text-muted-foreground hover:bg-accent'
                    }`}
                    onClick={() => setShowExternalLinks((v) => !v)}
                  >
                    <Link01Icon className="h-3 w-3" />
                    External Links
                    {form.external_links.length > 0 && (
                      <span className="text-[10px] opacity-70">({form.external_links.length})</span>
                    )}
                  </button>
                  {!isTemplateMode && (
                    <button
                      type="button"
                      className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition-colors cursor-pointer ${
                        showAttachments
                          ? 'border-primary/30 bg-primary/10 text-primary'
                          : 'border-border/60 text-muted-foreground hover:bg-accent'
                      }`}
                      onClick={() => setShowAttachments((v) => !v)}
                    >
                      <AttachmentIcon className="h-3 w-3" />
                      Attach Files
                      {pendingFiles.length > 0 && (
                        <span className="text-[10px] opacity-70">({pendingFiles.length})</span>
                      )}
                    </button>
                  )}
                </div>
                <div className="ml-auto inline-flex rounded-md border border-border/60 bg-muted/20 p-0.5">
                  <button
                    type="button"
                    className={`inline-flex items-center gap-1.5 rounded px-2.5 py-1 text-xs font-medium transition-colors cursor-pointer ${
                      descriptionMode === 'rich'
                        ? 'bg-background text-foreground shadow-sm'
                        : 'text-muted-foreground hover:text-foreground'
                    }`}
                    onClick={() => {
                      if (descriptionMode === 'markdown') {
                        applyMarkdownSource();
                      }
                    }}
                  >
                    <SparklesIcon className="h-3 w-3" />
                    Rich
                  </button>
                  <button
                    type="button"
                    className={`inline-flex items-center gap-1.5 rounded px-2.5 py-1 text-xs font-medium transition-colors cursor-pointer ${
                      descriptionMode === 'markdown'
                        ? 'bg-background text-foreground shadow-sm'
                        : 'text-muted-foreground hover:text-foreground'
                    }`}
                    onClick={openMarkdownMode}
                  >
                    <SourceCodeIcon className="h-3 w-3" />
                    Markdown
                  </button>
                </div>
              </div>

              {/* Checklist */}
              {showChecklist && (
                <div className="shrink-0 rounded-lg border border-border/60 bg-card">
                  <div className="flex items-center justify-between px-4 py-2 border-b border-border/40">
                    <div className="flex items-center gap-1.5 text-sm font-medium text-foreground">
                      <CheckListIcon className="h-3.5 w-3.5 text-muted-foreground" />
                      Checklist
                      {form.checklist_items.length > 0 && (
                        <span className="text-xs text-muted-foreground font-normal">({form.checklist_items.length})</span>
                      )}
                    </div>
                  </div>
                  <div className="px-4 py-2 space-y-1">
                    {form.checklist_items.map((item, idx) => (
                      <div key={idx} className="group flex items-center gap-2">
                        <DragDropVerticalIcon className="h-3 w-3 text-muted-foreground/40 shrink-0" />
                        <input
                          type="text"
                          value={item.text}
                          autoFocus={idx === form.checklist_items.length - 1 && item.text === ''}
                          onChange={(e) => {
                            const next = [...form.checklist_items];
                            next[idx] = { ...next[idx], text: e.target.value };
                            setForm((prev) => ({ ...prev, checklist_items: next }));
                          }}
                          placeholder="Item text"
                          className="flex-1 bg-transparent text-sm py-1 outline-none placeholder:text-muted-foreground/50"
                        />
                        <button
                          type="button"
                          className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-opacity cursor-pointer"
                          onClick={() => {
                            const next = form.checklist_items.filter((_, i) => i !== idx);
                            setForm((prev) => ({ ...prev, checklist_items: next }));
                          }}
                        >
                          <Delete01Icon className="h-3 w-3" />
                        </button>
                      </div>
                    ))}
                    <button
                      type="button"
                      className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors py-1 cursor-pointer"
                      onClick={() => {
                        setForm((prev) => ({
                          ...prev,
                          checklist_items: [...prev.checklist_items, { text: '', position: prev.checklist_items.length }],
                        }));
                      }}
                    >
                      <PlusSignIcon className="h-3 w-3" />
                      Add item
                    </button>
                  </div>
                </div>
              )}

              {/* External Links */}
              {showExternalLinks && (
                <div className="shrink-0 rounded-lg border border-border/60 bg-card">
                  <div className="flex items-center justify-between px-4 py-2 border-b border-border/40">
                    <div className="flex items-center gap-1.5 text-sm font-medium text-foreground">
                      <Link01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                      External Links
                      {form.external_links.length > 0 && (
                        <span className="text-xs text-muted-foreground font-normal">({form.external_links.length})</span>
                      )}
                    </div>
                  </div>
                  <div className="px-4 py-2 space-y-1">
                    {form.external_links.map((link, idx) => (
                      <div key={idx} className="group flex items-center gap-2">
                        <ExternalLinkIcon className="h-3 w-3 text-muted-foreground/40 shrink-0" />
                        <input
                          type="url"
                          value={link.url}
                          autoFocus={idx === form.external_links.length - 1 && link.url === ''}
                          onChange={(e) => {
                            const next = [...form.external_links];
                            next[idx] = { ...next[idx], url: e.target.value };
                            setForm((prev) => ({ ...prev, external_links: next }));
                          }}
                          placeholder="https://..."
                          className="flex-1 bg-transparent text-sm py-1 outline-none placeholder:text-muted-foreground/50"
                        />
                        <button
                          type="button"
                          className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-opacity cursor-pointer"
                          onClick={() => {
                            const next = form.external_links.filter((_, i) => i !== idx);
                            setForm((prev) => ({ ...prev, external_links: next }));
                          }}
                        >
                          <Delete01Icon className="h-3 w-3" />
                        </button>
                      </div>
                    ))}
                    <button
                      type="button"
                      className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors py-1 cursor-pointer"
                      onClick={() => {
                        setForm((prev) => ({
                          ...prev,
                          external_links: [...prev.external_links, { url: '' }],
                        }));
                      }}
                    >
                      <PlusSignIcon className="h-3 w-3" />
                      Add link
                    </button>
                  </div>
                </div>
              )}

              {/* Attachments (task mode only) */}
              {!isTemplateMode && showAttachments && (
                <div className="shrink-0 rounded-lg border border-border/60 bg-card">
                  <div className="flex items-center justify-between px-4 py-2 border-b border-border/40">
                    <div className="flex items-center gap-1.5 text-sm font-medium text-foreground">
                      <AttachmentIcon className="h-3.5 w-3.5 text-muted-foreground" />
                      Attachments
                      {pendingFiles.length > 0 && (
                        <span className="text-xs text-muted-foreground font-normal">({pendingFiles.length})</span>
                      )}
                    </div>
                  </div>
                  <div className="px-4 py-2 space-y-2">
                    {pendingFiles.length > 0 && (
                      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-4">
                        {pendingFiles.map((file, idx) => {
                          const isImage = file.type.startsWith('image/');
                          const ext = file.name.split('.').pop()?.toUpperCase() || 'FILE';
                          return (
                            <div key={idx} className="group relative">
                              <div className="overflow-hidden rounded-lg border border-border/60">
                                {isImage ? (
                                  <img
                                    src={URL.createObjectURL(file)}
                                    alt={file.name}
                                    className="h-20 w-full object-cover"
                                  />
                                ) : (
                                  <div className="flex h-20 flex-col items-center justify-center gap-1.5 bg-muted/30">
                                    <AttachmentIcon className="h-6 w-6 text-muted-foreground/50" />
                                    <span className="text-[9px] font-medium uppercase text-muted-foreground tracking-wide">{ext}</span>
                                  </div>
                                )}
                              </div>
                              <div className="absolute top-1.5 right-1.5 opacity-0 group-hover:opacity-100 transition-opacity">
                                <button
                                  type="button"
                                  className="flex h-6 w-6 items-center justify-center rounded bg-background/80 backdrop-blur-sm text-muted-foreground hover:text-destructive"
                                  onClick={() => setPendingFiles((prev) => prev.filter((_, i) => i !== idx))}
                                >
                                  <Delete01Icon className="h-3 w-3" />
                                </button>
                              </div>
                              <p className="mt-1 truncate text-[10px] text-muted-foreground" title={file.name}>{file.name}</p>
                            </div>
                          );
                        })}
                      </div>
                    )}
                    <label className="flex items-center justify-center gap-2 rounded-md border border-dashed border-border/60 px-3 py-2 cursor-pointer hover:border-border hover:bg-muted/30 transition-colors">
                      <Upload01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                      <span className="text-xs text-muted-foreground">Drop files or click to upload (max 50MB)</span>
                      <input
                        type="file"
                        multiple
                        className="hidden"
                        onChange={(e) => {
                          if (e.target.files?.length) {
                            const newFiles = Array.from(e.target.files).filter((f) => f.size <= 50 * 1024 * 1024);
                            setPendingFiles((prev) => [...prev, ...newFiles]);
                          }
                          e.target.value = '';
                        }}
                      />
                    </label>
                  </div>
                </div>
              )}

              {error ? <p className="text-sm text-destructive">{error}</p> : null}
            </div>

            {/* Right sidebar — metadata */}
            <aside className="min-h-0 overflow-y-auto border-l border-border/50 px-5 py-4">
              <div className="grid grid-cols-[16px_80px_1fr] items-center gap-x-3 gap-y-3">
                {/* Template */}
                {!isTemplateMode && templates.length > 0 && (
                <MetadataRow icon={File01Icon} label="Template">
                  <SidebarPopoverSelect
                    value=""
                    options={[
                      { value: '', label: 'None' },
                      ...templates
                        .filter((t) => !t.team_id || t.team_id === form.team_id || !form.team_id)
                        .map((t) => ({ value: t.id, label: t.name })),
                    ]}
                    onChange={(templateId) => {
                      const tmpl = templates.find((t) => t.id === templateId);
                      if (!tmpl) return;
                      setTaskTypeDirty(false);
                      setForm((prev) => ({
                        ...prev,
                        team_id: tmpl.team_id || prev.team_id,
                        description: tmpl.description || prev.description,
                        task_type: (tmpl.task_type as TaskType) || prev.task_type,
                        priority: (tmpl.priority as Priority) || prev.priority,
                        severity: (tmpl.severity as Severity) || prev.severity,
                        estimate: tmpl.estimate !== undefined && tmpl.estimate !== null ? String(tmpl.estimate) : prev.estimate,
                        label_ids: tmpl.label_ids ? (() => { try { return JSON.parse(tmpl.label_ids!); } catch { return prev.label_ids; } })() : prev.label_ids,
                        owner_member_ids: tmpl.owner_member_id ? [tmpl.owner_member_id] : prev.owner_member_ids,
                        epic_id: tmpl.epic_id || prev.epic_id,
                        sprint_id: tmpl.sprint_id || prev.sprint_id,
                        deadline: tmpl.deadline || prev.deadline,
                        checklist_items: tmpl.checklist_items ? (() => { try { return JSON.parse(tmpl.checklist_items!); } catch { return prev.checklist_items; } })() : prev.checklist_items,
                        external_links: tmpl.external_links ? (() => { try { return JSON.parse(tmpl.external_links!); } catch { return prev.external_links; } })() : prev.external_links,
                      }));
                      // Auto-open sections with template data
                      if (tmpl.checklist_items) { try { if (JSON.parse(tmpl.checklist_items).length > 0) setShowChecklist(true); } catch {} }
                      if (tmpl.external_links) { try { if (JSON.parse(tmpl.external_links).length > 0) setShowExternalLinks(true); } catch {} }
                    }}
                    renderTrigger={() => (
                      <>
                        <File01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                        <span>Apply template</span>
                      </>
                    )}
                  />
                </MetadataRow>
                )}

                {/* Team */}
                {teams.length > 0 && (
                  <MetadataRow icon={UserGroupIcon} label="Team *">
                    <SidebarPopoverSelect
                      value={form.team_id || "__none__"}
                      options={[
                        ...(teams.length === 0 ? [{ value: "__none__", label: "Select team" }] : []),
                        ...teams.map((t) => ({ value: t.id, label: t.name })),
                      ]}
                      onChange={(value) =>
                        {
                          setTaskTypeDirty(false);
                          setForm((prev) => ({
                            ...prev,
                            team_id: value === "__none__" ? "" : value,
                          }));
                        }
                      }
                      renderTrigger={() => <span>{currentTeamName}</span>}
                    />
                  </MetadataRow>
                )}

                {/* State */}
                {!isTemplateMode && workflow && (
                <MetadataRow icon={HashtagIcon} label="State">
                  <SidebarPopoverSelect
                    value={stateId}
                    options={workflow.states.map((s) => ({ value: s.id, label: s.name }))}
                    onChange={setStateId}
                    renderTrigger={() => {
                      const st = workflow.states.find((s) => s.id === stateId);
                      return st ? (
                        <TaskStateSelectContent stateType={st.state_type} label={st.name} color={st.color} />
                      ) : (
                        <span>{currentStateName}</span>
                      );
                    }}
                    renderOption={(v) => {
                      const s = workflow.states.find((st) => st.id === v);
                      return s ? (
                        <span className="inline-flex items-center gap-1.5">
                          <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: s.color || '#a1a1aa' }} />
                          <span>{s.name}</span>
                        </span>
                      ) : null;
                    }}
                  />
                </MetadataRow>
                )}

                {/* ── People ── */}
                <div className="col-span-3 h-px bg-border/40 my-1" />

                {/* Owner */}
                <MetadataRow icon={UserIcon} label={isTemplateMode ? "Owner" : "Owners"}>
                  {isTemplateMode ? (
                    <MemberPickerPopover
                      value={form.owner_member_ids[0] || "__none__"}
                      members={assignableMembers}
                      noneLabel="No owner"
                      onChange={(value) =>
                        setForm((prev) => ({
                          ...prev,
                          owner_member_ids: value === "__none__" ? [] : [value],
                        }))
                      }
                      renderTrigger={() => {
                        const selectedMember = findAssignableMember(assignableMembers, form.owner_member_ids[0]);
                        return (
                          <>
                            {selectedMember ? (
                              <UserAvatar
                                name={selectedMember.display_name || selectedMember.email}
                                avatarUrl={selectedMember.avatar_url}
                                avatarStyle={selectedMember.avatar_style}
                                avatarSeed={selectedMember.avatar_seed}
                                avatarBackgroundMode={selectedMember.avatar_background_mode}
                                avatarBackgroundColor={selectedMember.avatar_background_color}
                                className="h-4 w-4"
                                fallbackClassName="text-[7px]"
                              />
                            ) : null}
                            <span>{currentOwnerName}</span>
                          </>
                        );
                      }}
                    />
                  ) : (
                    <MultiMemberPickerPopover
                      values={form.owner_member_ids}
                      members={assignableMembers}
                      onChange={(nextOwnerIds) =>
                        setForm((prev) => ({ ...prev, owner_member_ids: nextOwnerIds }))
                      }
                      renderTrigger={() => (
                        <>
                          {form.owner_member_ids.length > 0 ? (
                            <OwnerAvatarStack
                              memberIds={form.owner_member_ids}
                              nameMap={memberNameMap}
                              members={assignableMembers}
                              size="sm"
                              max={3}
                              singleAvatarClassName="h-4 w-4"
                              singleFallbackClassName="text-[7px]"
                            />
                          ) : (
                            <span>{currentOwnerName}</span>
                          )}
                        </>
                      )}
                    />
                  )}
                </MetadataRow>

                {/* Requester */}
                {!isTemplateMode && (
                <MetadataRow icon={UserIcon} label="Requester">
                  <MemberPickerPopover
                    value={form.requester_member_id || "__none__"}
                    members={assignableMembers}
                    noneLabel="No requester"
                    onChange={(value) =>
                      setForm((prev) => ({
                        ...prev,
                        requester_member_id: value === "__none__" ? "" : value,
                      }))
                    }
                    renderTrigger={() => {
                      const selectedMember = findAssignableMember(assignableMembers, form.requester_member_id);
                      return (
                        <>
                          {selectedMember ? (
                            <UserAvatar
                              name={selectedMember.display_name || selectedMember.email}
                              avatarUrl={selectedMember.avatar_url}
                              avatarStyle={selectedMember.avatar_style}
                              avatarSeed={selectedMember.avatar_seed}
                              avatarBackgroundMode={selectedMember.avatar_background_mode}
                              avatarBackgroundColor={selectedMember.avatar_background_color}
                              className="h-4 w-4"
                              fallbackClassName="text-[7px]"
                            />
                          ) : null}
                          <span>{currentRequesterName}</span>
                        </>
                      );
                    }}
                  />
                </MetadataRow>
                )}

                {/* ── Classification ── */}
                {(fieldVis.priority || fieldVis.severity || fieldVis.task_type || fieldVis.labels) && <div className="col-span-3 h-px bg-border/40 my-1" />}

                {/* Priority */}
                {fieldVis.priority && (
                <MetadataRow icon={DashboardSpeed01Icon} label="Priority">
                  <SidebarPopoverSelect
                    value={form.priority}
                    options={priorityOptions.map((p) => ({ value: p, label: PRIORITY_CONFIG[p].label }))}
                    onChange={(value) =>
                      setForm((prev) => ({ ...prev, priority: value as Priority }))
                    }
                    renderTrigger={() => (
                      <span className="inline-flex min-w-0 items-center gap-1.5">
                        <PriorityIcon priority={form.priority} className="h-3.5 w-3.5 shrink-0" />
                        <span className="truncate">{PRIORITY_CONFIG[form.priority].label}</span>
                      </span>
                    )}
                    renderOption={(v) => <><PriorityIcon priority={v as Priority} className="h-4 w-4 shrink-0" /><span>{PRIORITY_CONFIG[v as Priority].label}</span></>}
                  />
                </MetadataRow>
                )}

                {/* Severity */}
                {fieldVis.severity && (
                <MetadataRow icon={Alert01Icon} label="Severity">
                  <SidebarPopoverSelect
                    value={form.severity}
                    options={severityOptions.map((s) => ({ value: s, label: SEVERITY_CONFIG[s].label }))}
                    onChange={(value) =>
                      setForm((prev) => ({ ...prev, severity: value as Severity }))
                    }
                    renderTrigger={() => (
                      <span className="inline-flex min-w-0 items-center gap-1.5">
                        <SeverityIcon severity={form.severity} className="h-3.5 w-3.5 shrink-0" />
                        <span className="truncate">{SEVERITY_CONFIG[form.severity].label}</span>
                      </span>
                    )}
                    renderOption={(v) => <><SeverityIcon severity={v as Severity} className="h-4 w-4 shrink-0" /><span>{SEVERITY_CONFIG[v as Severity].label}</span></>}
                  />
                </MetadataRow>
                )}

                {/* Type */}
                {fieldVis.task_type && (
                <MetadataRow icon={HashtagIcon} label="Type">
                  <SidebarPopoverSelect
                    value={form.task_type}
                    options={taskTypeOptions.map((t) => ({ value: t, label: TASK_TYPE_CONFIG[t].label }))}
                    onChange={(value) => {
                      setTaskTypeDirty(true);
                      setForm((prev) => ({ ...prev, task_type: value as TaskType }));
                    }}
                    renderTrigger={() => (
                      <span className="inline-flex min-w-0 items-center gap-1.5">
                        <TaskTypeIcon taskType={form.task_type} className="h-4 w-4 shrink-0" />
                        <span className="truncate">{TASK_TYPE_CONFIG[form.task_type].label}</span>
                      </span>
                    )}
                    renderOption={(v) => <><TaskTypeIcon taskType={v as TaskType} className="h-[18px] w-[18px] shrink-0" /><span>{TASK_TYPE_CONFIG[v as TaskType].label}</span></>}
                  />
                </MetadataRow>
                )}

                {/* Labels */}
                {fieldVis.labels && (
                <MetadataRow icon={Tag01Icon} label="Labels">
                  <LabelPicker
                    workspaceId={workspaceId}
                    teamId={form.team_id || undefined}
                    labels={labels}
                    selectedLabelIds={form.label_ids}
                    onLabelsChange={setLabels}
                    onChange={(ids) => setForm((prev) => ({ ...prev, label_ids: ids }))}
                  />
                </MetadataRow>
                )}

                {/* ── Planning ── */}
                {(fieldVis.epic || (fieldVis.sprint && teamSprintsEnabled)) && <div className="col-span-3 h-px bg-border/40 my-1" />}

                {/* Epic */}
                {fieldVis.epic && (
                <MetadataRow icon={Layers01Icon} label="Epic">
                  <SidebarPopoverSelect
                    value={form.epic_id || "__none__"}
                    groups={epicGroups}
                    onChange={(value) =>
                      setForm((prev) => ({
                        ...prev,
                        epic_id: value === "__none__" ? "" : value,
                      }))
                    }
                    renderTrigger={() => <span>{currentEpicName}</span>}
                  />
                </MetadataRow>
                )}

                {/* Sprint */}
                {fieldVis.sprint && teamSprintsEnabled && (
                <MetadataRow icon={SprintIcon} label="Sprint">
                  <GroupedSidebarPopoverSelect
                    value={form.sprint_id || "__none__"}
                    groups={sprintOptionGroups}
                    showGroupHeadings={!form.team_id}
                    onChange={(value) =>
                      setForm((prev) => ({
                        ...prev,
                        sprint_id: value === "__none__" ? "" : value,
                      }))
                    }
                    searchPlaceholder="Search sprints..."
                    emptyLabel="No sprints"
                    renderTrigger={() => <span>{currentSprintName}</span>}
                  />
                </MetadataRow>
                )}

                {/* ── Tracking ── */}
                {(fieldVis.estimate || fieldVis.due_date) && <div className="col-span-3 h-px bg-border/40 my-1" />}

                {/* Estimate */}
                {fieldVis.estimate && (
                <MetadataRow icon={LayoutGridIcon} label="Estimate">
                  <EstimatePicker
                    value={form.estimate}
                    teamId={form.team_id || undefined}
                    onChange={(displayValue) =>
                      setForm((prev) => ({ ...prev, estimate: displayValue }))
                    }
                  />
                </MetadataRow>
                )}

                {/* Due date */}
                {fieldVis.due_date && (
                <MetadataRow icon={Calendar03Icon} label="Due date">
                  <DatePicker
                    value={form.deadline}
                    onChange={(v) => setForm((prev) => ({ ...prev, deadline: v }))}
                    kind="due"
                    label="Due date"
                    placeholder="None"
                    disablePast
                    hideIcon
                    urgencyColor
                    className="h-auto border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent"
                  />
                </MetadataRow>
                )}

                {!isTemplateMode && (
                  <>
                    <div className="col-span-3 h-px bg-border/40 my-1" />
                    <MetadataRow icon={ArrowReloadHorizontalIcon} label="Recurring">
                      <button
                        type="button"
                        className="inline-flex min-w-0 items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent"
                        onClick={() => setRecurringDialogOpen(true)}
                      >
                        {recurringDraft ? <RecurringTemplateBadge compact /> : null}
                        <span className="truncate">{recurringDraftSummary}</span>
                      </button>
                    </MetadataRow>
                  </>
                )}
              </div>
            </aside>
          </div>

          {/* Footer */}
          <div className="flex items-center justify-end gap-3 border-t border-border/50 px-6 py-3">
            {!isTemplateMode && (
            <div className="mr-auto flex items-center gap-2">
              <Switch checked={createMore} onCheckedChange={setCreateMore} />
              <span className="text-sm text-muted-foreground">Create more</span>
            </div>
            )}

            <Button
              type="button"
              variant="outline"
              onClick={() => handleOpenChange(false)}
              disabled={submitting}
            >
              Discard
            </Button>
            <Button
              type="button"
              onClick={submit}
              disabled={!canSubmit || submitting}
            >
              {submitting ? <Loading01Icon className="h-4 w-4 animate-spin" /> : null}
              {submitting ? "Saving..." : "Save"}
            </Button>
          </div>
        </div>

        {!isTemplateMode && (
          <Dialog open={recurringDialogOpen} onOpenChange={setRecurringDialogOpen}>
            <DialogContent className="sm:max-w-lg">
              <DialogHeader>
                <DialogTitle>Configure recurrence</DialogTitle>
                <DialogDescription>
                  Automatically create copies of this task on a schedule.
                </DialogDescription>
              </DialogHeader>
              <RecurringTemplateForm
                initialValue={recurringDraft ?? {
                  title: form.name.trim() || 'Recurring task',
                  description: '',
                  config: {
                    schedule_type: 'time',
                    frequency: 'weekly',
                    interval: 1,
                    weekdays: [1],
                    due_date_mode: 'scheduled_date',
                    sprint_assignment_mode: 'current_sprint',
                  },
                }}
                workflowStates={workflow?.states ?? []}
                submitLabel="Apply"
                onCancel={() => setRecurringDialogOpen(false)}
                onSubmit={(value) => {
                  setRecurringDraft(value);
                  setRecurringDialogOpen(false);
                }}
                onRemove={recurringDraft ? () => {
                  setRecurringDraft(null);
                  setRecurringDialogOpen(false);
                } : undefined}
              />
            </DialogContent>
          </Dialog>
        )}
      </DialogContent>
    </Dialog>
  );
}
