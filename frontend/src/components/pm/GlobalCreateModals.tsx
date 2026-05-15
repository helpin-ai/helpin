import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import {
  Calendar03Icon,
  Loading01Icon,
  UserIcon,
  UserGroupIcon,
  Cancel01Icon,
  HashtagIcon,
  Layers01Icon,
  AttachmentIcon,
  Delete01Icon,
  Upload01Icon,
  Link01Icon,
  LinkSquare01Icon as ExternalLinkIcon,
  PlusSignIcon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { DatePicker } from '@/components/ui/date-picker';
import { CreateTaskModal } from '@/components/pm/CreateTaskModal';
import { AgentPickerCard } from '@/components/pm/AgentPickerCard';
import { CreateDocumentDialog } from '@/components/docs/CreateDocumentDialog';
import { CreateSpaceDialog } from '@/components/docs/CreateSpaceDialog';
import { CreateCollectionDialog } from '@/components/docs/CreateCollectionDialog';
import { CreateContactDialog } from '@/components/crm/CreateContactDialog';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useEpicStates } from '@/hooks/queries/useWorkflows';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { pmAutomationService } from '@/lib/services/pmAutomationService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { pmExternalLinkService } from '@/lib/services/pmExternalLinkService';
import { uploadToS3 } from '@/lib/api';
import { gitService } from '@/lib/services/gitService';
import { toast } from 'sonner';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { usePMBoardStore } from '@/stores/pmBoardStore';
import type { GitRepository, ObjectiveType, ObjectiveState, WorkflowWithStates } from '@/lib/pmTypes';
import { OBJECTIVE_STATE_CONFIG } from '@/lib/pmConstants';
import { filterMentionTeams } from '@/components/pm/mentionSuggestions';
import { extractInlineAttachmentIds } from '@/components/pm/editorImageAttachments';
import { MemberPickerPopover, MultiMemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { findAssignableMember } from '@/lib/assignableMembers';
import { normalizeTeamType } from '@/lib/teamPresets';
import {
  dismissSprintAutomationPrompt,
  shouldPromptSprintAutomation,
  type SprintAutomationPromptState,
} from '@/components/pm/sprintAutomationPrompt';
import { showEntityCreatedToast, entityCreatedToastIcons } from '@/components/ui/entity-created-toast';
import { getOptionalSectionActionClass } from '@/components/pm/optionalSectionActionPill';

import pdfIcon from '@/assets/attachment/pdf-icon.png';
import csvIcon from '@/assets/attachment/csv-icon.png';
import excelIcon from '@/assets/attachment/excel-icon.png';
import docIcon from '@/assets/attachment/doc-icon.png';
import pngIcon from '@/assets/attachment/png-icon.png';
import jpgIcon from '@/assets/attachment/jpg-icon.png';
import svgIcon from '@/assets/attachment/svg-icon.png';
import txtIcon from '@/assets/attachment/txt-icon.png';
import zipIcon from '@/assets/attachment/zip-icon.png';
import rarIcon from '@/assets/attachment/rar-icon.png';
import htmlIcon from '@/assets/attachment/html-icon.png';
import cssIcon from '@/assets/attachment/css-icon.png';
import jsIcon from '@/assets/attachment/js-icon.png';
import audioIcon from '@/assets/attachment/audio-icon.png';
import videoIcon from '@/assets/attachment/video-icon.png';
import defaultIcon from '@/assets/attachment/default-icon.png';

const MAX_PENDING_ATTACHMENT_SIZE = 50 * 1024 * 1024;

function getFileExtension(filename: string): string {
  const parts = filename.split('.');
  return parts.length > 1 ? parts.pop()!.toLowerCase() : '';
}

function getFileTypeIcon(extension: string): string {
  const iconMap: Record<string, string> = {
    pdf: pdfIcon,
    csv: csvIcon,
    xlsx: excelIcon,
    xls: excelIcon,
    doc: docIcon,
    docx: docIcon,
    png: pngIcon,
    jpg: jpgIcon,
    jpeg: jpgIcon,
    svg: svgIcon,
    txt: txtIcon,
    md: txtIcon,
    zip: zipIcon,
    gz: zipIcon,
    tar: zipIcon,
    rar: rarIcon,
    html: htmlIcon,
    css: cssIcon,
    js: jsIcon,
    ts: jsIcon,
    mp3: audioIcon,
    wav: audioIcon,
    mp4: videoIcon,
    mkv: videoIcon,
    wmv: videoIcon,
    webm: videoIcon,
  };
  return iconMap[extension] || defaultIcon;
}


// ── Task wrapper ─────────────────────────────────────────────────────

function GlobalCreateTask({ workspaceId, onClose }: { workspaceId: string; onClose: () => void }) {
  const qc = useQueryClient();
  const [workflow, setWorkflow] = useState<WorkflowWithStates | null>(null);
  const initialTeamId = useGlobalCreateStore((s) => s.initialTeamId);
  const initialOwnerMemberId = useGlobalCreateStore((s) => s.initialOwnerMemberId);
  const initialSprintId = useGlobalCreateStore((s) => s.initialSprintId);

  useEffect(() => {
    // Try board store first (already loaded if on tasks page)
    const boardWorkflow = usePMBoardStore.getState().workflow;
    if (boardWorkflow) {
      setWorkflow(boardWorkflow);
      return;
    }
    pmWorkflowService.list(workspaceId).then((res) => {
      if (res.data?.[0]) setWorkflow(res.data[0]);
    });
  }, [workspaceId]);

  if (!workflow) return null;

  return (
    <CreateTaskModal
      open
      onOpenChange={(open) => !open && onClose()}
      workspaceId={workspaceId}
      workflow={workflow}
      initialStateId={workflow.states[0]?.id ?? ''}
      initialTeamId={initialTeamId}
      initialOwnerMemberId={initialOwnerMemberId}
      initialSprintId={initialSprintId}
      onCreate={async (payload) => {
        const { data, error } = await pmTaskService.create(payload);
        if (error) throw new Error(error);
        // Refresh the board if it's loaded
        const boardWs = usePMBoardStore.getState().workspaceId;
        if (boardWs) usePMBoardStore.getState().refreshBoard();
        // Invalidate TanStack Query caches
        qc.invalidateQueries({ queryKey: ['pm', workspaceId, 'tasks'] });
        qc.invalidateQueries({ queryKey: ['pm', workspaceId, 'sprints', 'planning'] });
        const createdTask = data?.task?.task;
        window.dispatchEvent(new CustomEvent('task-created', {
          detail: { ownerMemberIds: createdTask?.owner_member_ids ?? [], teamId: createdTask?.team_id },
        }));
        return createdTask
          ? {
              id: createdTask.id,
              agent_run_error: data?.agent_run_error,
              task: {
                id: createdTask.id,
                name: createdTask.name,
                display_id: createdTask.display_id,
                task_key: createdTask.task_key,
              },
            }
          : undefined;
      }}
    />
  );
}

// ── Epic dialog ──────────────────────────────────────────────────────

function GlobalCreateEpic({ workspaceId, onClose }: { workspaceId: string; onClose: () => void }) {
  const navigate = useNavigate();
  const currentWorkspace = useWorkspaceStore((s) => s.currentWorkspace);
  const confirm = useConfirm();
  const { data: epicStates = [] } = useEpicStates(workspaceId);
  const { teams } = useAccessibleTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const storeTeamId = useGlobalCreateStore((s) => s.initialTeamId);

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const descriptionRef = useRef('');
  const [meta, setMeta] = useState({
    stateId: '',
    teamId: storeTeamId ?? teams[0]?.id ?? '',
    ownerMemberId: '',
    planningRepositoryId: '',
    startDate: '',
    targetDate: '',
  });
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [assignedAgentId, setAssignedAgentId] = useState<string | undefined>();
  const [submitting, setSubmitting] = useState(false);
  const [descriptionPendingUploads, setDescriptionPendingUploads] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [pendingFiles, setPendingFiles] = useState<File[]>([]);
  const [showAttachments, setShowAttachments] = useState(false);
  const [showExternalLinks, setShowExternalLinks] = useState(false);
  const [epicExternalLinks, setEpicExternalLinks] = useState<{ url: string }[]>([]);
  const [isDraggingFiles, setIsDraggingFiles] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const selectedTeam = useMemo(
    () => teams.find((team) => team.id === meta.teamId),
    [teams, meta.teamId],
  );
  const showPlanningRepository = normalizeTeamType(selectedTeam?.team_type) === 'engineering';
  const mentionTeams = useMemo(
    () => filterMentionTeams(teams, meta.teamId ? [meta.teamId] : []),
    [teams, meta.teamId],
  );

  useEffect(() => {
    if (!showPlanningRepository) return;
    let cancelled = false;

    async function loadRepositories() {
      const { data } = await gitService.listRepositories(workspaceId);
      if (!cancelled) {
        setRepositories(data ?? []);
      }
    }

    void loadRepositories();
    return () => {
      cancelled = true;
    };
  }, [workspaceId, showPlanningRepository]);

  const cleanupInlineDraftUploads = useCallback(async () => {
    const attachmentIds = extractInlineAttachmentIds(descriptionRef.current);
    if (attachmentIds.length === 0) return;
    await Promise.allSettled(
      attachmentIds.map((attachmentId) => pmAttachmentService.remove(workspaceId, attachmentId)),
    );
  }, [workspaceId]);

  const addPendingFiles = useCallback((files: FileList | File[]) => {
    const newFiles = Array.from(files).filter((file) => file.size <= MAX_PENDING_ATTACHMENT_SIZE);
    if (newFiles.length === 0) return;
    setPendingFiles((prev) => [...prev, ...newFiles]);
  }, []);

  const handleDragOver = (event: DragEvent<HTMLLabelElement>) => {
    event.preventDefault();
    setIsDraggingFiles(true);
  };

  const handleDragLeave = () => {
    setIsDraggingFiles(false);
  };

  const handleDrop = (event: DragEvent<HTMLLabelElement>) => {
    event.preventDefault();
    setIsDraggingFiles(false);
    if (event.dataTransfer.files.length > 0) {
      addPendingFiles(event.dataTransfer.files);
    }
  };

  const create = async () => {
    if (!name.trim() || !meta.teamId || submitting || descriptionPendingUploads > 0) return;
    setSubmitting(true);
    try {
      const inlineAttachmentIds = extractInlineAttachmentIds(description);
      const { data, error: createError } = await pmEpicService.create({
        workspace_id: workspaceId,
        name: name.trim(),
        description: description.trim() || undefined,
        attachment_ids: inlineAttachmentIds.length > 0 ? inlineAttachmentIds : undefined,
        epic_state_id: meta.stateId || undefined,
        team_id: meta.teamId || undefined,
        owner_member_id: meta.ownerMemberId || undefined,
        planned_start_date: meta.startDate || undefined,
        deadline: meta.targetDate || undefined,
        planning_repository_id: showPlanningRepository ? (meta.planningRepositoryId || undefined) : undefined,
        assigned_agent_id: assignedAgentId,
        run_on_create: Boolean(assignedAgentId),
      });

      if (createError) {
        setError(createError);
        return;
      }

      const createdEpic = data?.epic?.epic;

      if (createdEpic && pendingFiles.length > 0) {
        for (const file of pendingFiles) {
          try {
            const { data: initData } = await pmAttachmentService.initiateUpload(workspaceId, {
              entity_type: 'epic',
              entity_id: createdEpic.id,
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
            // Non-blocking — epic already created
          }
        }
      }

      // Create external links after epic creation
      if (createdEpic) {
        const validLinks = epicExternalLinks.filter((l) => l.url.trim());
        for (const el of validLinks) {
          try {
            await pmExternalLinkService.createForEntity(workspaceId, 'epic', createdEpic.id, { url: el.url.trim() });
          } catch {
            // Non-blocking — epic already created
          }
        }
      }

      if (data?.agent_run_error) {
        toast.warning(`Epic created, but the agent did not start: ${data.agent_run_error}`);
      }

      if (createdEpic) {
        showEntityCreatedToast({
          entityLabel: 'Epic',
          title: createdEpic.name,
          tone: 'pm',
          icon: entityCreatedToastIcons.epic,
          onOpen: currentWorkspace?.slug
            ? () => navigate({
                to: '/w/$slug/pm/epics/$epicId',
                params: { slug: currentWorkspace.slug, epicId: createdEpic.id },
              })
            : undefined,
        });
      } else {
        toast.success('Epic created');
      }
      window.dispatchEvent(new CustomEvent('epic-created'));
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create epic');
    } finally {
      setSubmitting(false);
    }
  };

  const hasUnsavedChanges = name.trim() !== '' || description.trim() !== '' || pendingFiles.length > 0 || epicExternalLinks.some(l => l.url.trim());

  const handleClose = async () => {
    if (hasUnsavedChanges) {
      const ok = await confirm({
        title: 'Discard changes?',
        description: 'You have unsaved changes that will be lost.',
        confirmText: 'Discard',
        variant: 'destructive',
      });
      if (!ok) return;
    }
    void cleanupInlineDraftUploads();
    setPendingFiles([]);
    setShowAttachments(false);
    setEpicExternalLinks([]);
    setShowExternalLinks(false);
    onClose();
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open) handleClose(); }}>
      <DialogContent className="max-w-4xl sm:max-w-4xl gap-0 overflow-hidden p-0" showCloseButton={false}>
        <div className="flex h-[80vh] flex-col">
          <div className="flex items-center justify-between border-b border-border/60 px-6 pt-4 pb-3">
            <span className="text-lg font-semibold">Create epic</span>
            <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={handleClose}>
              <Cancel01Icon className="h-4 w-4" />
            </Button>
          </div>

          {error && (
            <div className="mx-4 mt-2 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
              {error}
            </div>
          )}

          <div className="grid min-h-0 flex-1 grid-cols-[1fr_280px] overflow-hidden">
            <div className="min-h-0 overflow-y-auto px-8 py-5">
              <Input
                autoFocus
                aria-label="Epic title"
                value={name}
                onChange={(e) => setName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Tab' && !e.shiftKey) {
                    e.preventDefault();
                    const editor = e.currentTarget.parentElement?.querySelector<HTMLElement>('.tiptap.ProseMirror');
                    editor?.focus();
                  }
                }}
                className="h-12 shrink-0 border-border/60 text-base shadow-none focus-visible:border-border"
                placeholder="Epic title"
              />
              <div className="mt-4">
                <TiptapEditor
                  content={description}
                  onChange={(html) => { descriptionRef.current = html; setDescription(html); }}
                  placeholder="Add a description (optional)..."
                  className="border-transparent shadow-none [&_.tiptap]:min-h-[220px]"
                  uploadConfig={{ workspaceId, entityType: 'editor_upload', entityId: workspaceId }}
                  onUploadStateChange={setDescriptionPendingUploads}
                  teams={mentionTeams}
                  members={assignableMembers}
                />
                <div className="mt-3">
                  <AgentPickerCard
                    workspaceId={workspaceId}
                    runnableTarget="epic"
                    targetTeamId={meta.teamId || null}
                    value={assignedAgentId}
                    onChange={setAssignedAgentId}
                    hasRepoContext={Boolean(meta.planningRepositoryId)}
                  />
                </div>
                <div className="mt-3 flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    className={getOptionalSectionActionClass(epicExternalLinks.length > 0 ? 'locked' : showExternalLinks ? 'open' : 'available')}
                    disabled={epicExternalLinks.length > 0}
                    onClick={() => setShowExternalLinks((v) => !v)}
                  >
                    <Link01Icon className="h-3 w-3" />
                    External Links
                    {epicExternalLinks.length > 0 && (
                      <span className="text-[10px] opacity-70">({epicExternalLinks.length})</span>
                    )}
                  </button>
                  <button
                    type="button"
                    className={getOptionalSectionActionClass(pendingFiles.length > 0 ? 'locked' : showAttachments ? 'open' : 'available')}
                    disabled={pendingFiles.length > 0}
                    onClick={() => setShowAttachments((value) => !value)}
                  >
                    <AttachmentIcon className="h-3 w-3" />
                    Attach Files
                    {pendingFiles.length > 0 && (
                      <span className="text-[10px] opacity-70">({pendingFiles.length})</span>
                    )}
                  </button>
                </div>
                {showExternalLinks && (
                  <div className="mt-3 shrink-0 rounded-lg border border-border/60 bg-card">
                    <div className="flex items-center justify-between px-4 py-2 border-b border-border/40">
                      <div className="flex items-center gap-1.5 text-sm font-medium text-foreground">
                        <Link01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                        External Links
                        {epicExternalLinks.length > 0 && (
                          <span className="text-xs text-muted-foreground font-normal">({epicExternalLinks.length})</span>
                        )}
                      </div>
                    </div>
                    <div className="px-4 py-2 space-y-1">
                      {epicExternalLinks.map((link, idx) => (
                        <div key={idx} className="group flex items-center gap-2">
                          <ExternalLinkIcon className="h-3 w-3 text-muted-foreground/40 shrink-0" />
                          <input
                            type="url"
                            value={link.url}
                            autoFocus={idx === epicExternalLinks.length - 1 && link.url === ''}
                            onChange={(e) => {
                              const next = [...epicExternalLinks];
                              next[idx] = { url: e.target.value };
                              setEpicExternalLinks(next);
                            }}
                            placeholder="https://..."
                            className="flex-1 bg-transparent text-sm py-1 outline-none placeholder:text-muted-foreground/50"
                          />
                          <button
                            type="button"
                            className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-opacity cursor-pointer"
                            onClick={() => setEpicExternalLinks(epicExternalLinks.filter((_, i) => i !== idx))}
                          >
                            <Delete01Icon className="h-3 w-3" />
                          </button>
                        </div>
                      ))}
                      <button
                        type="button"
                        className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors py-1 cursor-pointer"
                        onClick={() => setEpicExternalLinks([...epicExternalLinks, { url: '' }])}
                      >
                        <PlusSignIcon className="h-3 w-3" />
                        Add link
                      </button>
                    </div>
                  </div>
                )}
                {showAttachments && (
                  <div className="mt-3 shrink-0 rounded-lg border border-border/60 bg-card">
                    <div className="flex items-center justify-between border-b border-border/40 px-4 py-2">
                      <div className="flex items-center gap-1.5 text-sm font-medium text-foreground">
                        <AttachmentIcon className="h-3.5 w-3.5 text-muted-foreground" />
                        Attachments
                        {pendingFiles.length > 0 && (
                          <span className="text-xs font-normal text-muted-foreground">({pendingFiles.length})</span>
                        )}
                      </div>
                    </div>
                    <div className="space-y-2 px-4 py-2">
                      {pendingFiles.length > 0 && (
                        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-4">
                          {pendingFiles.map((file, idx) => {
                            const isImage = file.type.startsWith('image/') && !file.type.includes('svg');
                            const ext = getFileExtension(file.name);
                            return (
                              <div key={`${file.name}-${file.lastModified}-${idx}`} className="group relative">
                                <div className="overflow-hidden rounded-lg border border-border/60">
                                  {isImage ? (
                                    <img
                                      src={URL.createObjectURL(file)}
                                      alt={file.name}
                                      className="h-20 w-full object-cover"
                                    />
                                  ) : (
                                    <div className="flex h-20 flex-col items-center justify-center gap-1.5 bg-muted/30">
                                      <img
                                        src={getFileTypeIcon(ext)}
                                        alt={ext || 'file'}
                                        className="h-8 w-8"
                                      />
                                      <span className="text-[9px] font-medium uppercase tracking-wide text-muted-foreground">
                                        {ext || 'FILE'}
                                      </span>
                                    </div>
                                  )}
                                </div>
                                <div className="absolute right-1.5 top-1.5 opacity-0 transition-opacity group-hover:opacity-100">
                                  <button
                                    type="button"
                                    className="flex h-6 w-6 items-center justify-center rounded bg-background/80 text-muted-foreground backdrop-blur-sm hover:text-destructive"
                                    onClick={() => setPendingFiles((prev) => prev.filter((_, fileIdx) => fileIdx !== idx))}
                                  >
                                    <Delete01Icon className="h-3 w-3" />
                                  </button>
                                </div>
                                <p className="mt-1 truncate text-[10px] text-muted-foreground" title={file.name}>
                                  {file.name}
                                </p>
                              </div>
                            );
                          })}
                        </div>
                      )}
                      <label
                        className={`flex items-center justify-center gap-2 rounded-md border border-dashed px-3 py-2 transition-colors cursor-pointer ${
                          isDraggingFiles
                            ? 'border-primary bg-primary/5'
                            : 'border-border/60 hover:border-border hover:bg-muted/30'
                        }`}
                        onDragOver={handleDragOver}
                        onDragLeave={handleDragLeave}
                        onDrop={handleDrop}
                      >
                        <Upload01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                        <span className="text-xs text-muted-foreground">Drop files or click to upload (max 50MB)</span>
                        <input
                          ref={fileInputRef}
                          type="file"
                          multiple
                          className="hidden"
                          onChange={(event) => {
                            if (event.target.files?.length) {
                              addPendingFiles(event.target.files);
                            }
                            event.target.value = '';
                          }}
                        />
                      </label>
                    </div>
                  </div>
                )}
              </div>
            </div>

            <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-5">
              <p className="mb-4 text-xs text-muted-foreground">
                Epics are collections of tasks that together represent a major initiative or feature.
              </p>
              <div className="grid grid-cols-[16px_80px_1fr] items-center gap-x-2 gap-y-3">
                <UserGroupIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Team *</span>
                <Select value={meta.teamId || '__none__'} onValueChange={(v) => setMeta((m) => ({ ...m, teamId: v === '__none__' ? '' : v }))}>
                  <SelectTrigger className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent">
                    <SelectValue placeholder="Select team" />
                  </SelectTrigger>
                  <SelectContent>
                    {teams.map((t) => (
                      <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <UserIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Owner</span>
                <MemberPickerPopover
                  value={meta.ownerMemberId || '__none__'}
                  members={assignableMembers}
                  noneLabel="No owner"
                  onChange={(value) => setMeta((m) => ({ ...m, ownerMemberId: value === '__none__' ? '' : value }))}
                  renderTrigger={() => {
                    const selectedMember = findAssignableMember(assignableMembers, meta.ownerMemberId);
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
                        <span>{selectedMember?.display_name || selectedMember?.email || 'No owner'}</span>
                      </>
                    );
                  }}
                />

                <HashtagIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">State</span>
                <Select value={meta.stateId || '__none__'} onValueChange={(v) => setMeta((m) => ({ ...m, stateId: v === '__none__' ? '' : v }))}>
                  <SelectTrigger className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent">
                    <SelectValue placeholder="None" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__none__">None</SelectItem>
                    {epicStates.map((s) => (
                      <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <Calendar03Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Start date</span>
                <DatePicker
                  value={meta.startDate}
                  onChange={(v) => setMeta((m) => ({ ...m, startDate: v }))}
                  kind="start"
                  label="Start date"
                  linkedDate={{
                    label: 'Target date',
                    kind: 'target',
                    value: meta.targetDate,
                    onChange: (v) => setMeta((m) => ({ ...m, targetDate: v })),
                    placeholder: 'None',
                  }}
                  placeholder="None"
                  hideIcon
                  className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent"
                />

                <Calendar03Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Target date</span>
                <DatePicker
                  value={meta.startDate}
                  onChange={(v) => setMeta((m) => ({ ...m, startDate: v }))}
                  kind="start"
                  label="Start date"
                  linkedDate={{
                    label: 'Target date',
                    kind: 'target',
                    value: meta.targetDate,
                    onChange: (v) => setMeta((m) => ({ ...m, targetDate: v })),
                    placeholder: 'None',
                  }}
                  triggerField="linked"
                  defaultActiveField="linked"
                  placeholder="None"
                  hideIcon
                  className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent"
                />

                {showPlanningRepository ? (
                  <>
                    <Separator className="col-span-3 my-1" />

                    <Layers01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                    <span className="text-xs text-muted-foreground self-center">Plan repo</span>
                    <Select
                      value={meta.planningRepositoryId || '__none__'}
                      onValueChange={(v) => setMeta((m) => ({ ...m, planningRepositoryId: v === '__none__' ? '' : v }))}
                    >
                      <SelectTrigger className="min-h-8 h-auto border-0 bg-transparent px-1.5 py-1 shadow-none text-xs hover:bg-accent [&_[data-slot=select-value]]:line-clamp-none [&_[data-slot=select-value]]:whitespace-normal [&_[data-slot=select-value]]:break-words [&_[data-slot=select-value]]:text-left [&_[data-slot=select-value]]:leading-tight">
                        <SelectValue placeholder="Not configured" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="__none__">Not configured</SelectItem>
                        {repositories.map((repo) => (
                          <SelectItem key={repo.id} value={repo.id}>{repo.full_name}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </>
                ) : null}
              </div>
            </aside>
          </div>

          {/* Footer */}
          <div className="flex items-center justify-end gap-3 border-t border-border/50 px-6 py-3">
            <Button variant="outline" size="sm" onClick={handleClose} disabled={submitting}>
              Discard
            </Button>
            <Button size="sm" onClick={create} disabled={!name.trim() || !meta.teamId || submitting || descriptionPendingUploads > 0}>
              {submitting ? <Loading01Icon className="mr-1 h-3.5 w-3.5 animate-spin" /> : null}
              {submitting ? 'Creating...' : assignedAgentId ? 'Create & run agent' : 'Create Epic'}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

// ── Sprint dialog ─────────────────────────────────────────────────

function GlobalCreateSprint({ workspaceId, onClose }: { workspaceId: string; onClose: () => void }) {
  const confirm = useConfirm();
  const { teams } = useAccessibleTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const storeTeamId = useGlobalCreateStore((s) => s.initialTeamId);

  const [form, setForm] = useState({
    name: '',
    description: '',
    startDate: '',
    endDate: '',
    teamId: storeTeamId ?? teams.find((t) => t.sprints_enabled !== false)?.id ?? '',
  });
  const [submitting, setSubmitting] = useState(false);
  const [descriptionPendingUploads, setDescriptionPendingUploads] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [automationPrompt, setAutomationPrompt] = useState<SprintAutomationPromptState | null>(null);
  const [enablingAutomation, setEnablingAutomation] = useState(false);
  const [dismissingAutomation, setDismissingAutomation] = useState(false);
  const mentionTeams = useMemo(
    () => filterMentionTeams(teams, form.teamId ? [form.teamId] : []),
    [teams, form.teamId],
  );
  const cleanupInlineDraftUploads = useCallback(async () => {
    const attachmentIds = extractInlineAttachmentIds(form.description);
    if (attachmentIds.length === 0) return;
    await Promise.allSettled(
      attachmentIds.map((attachmentId) => pmAttachmentService.remove(workspaceId, attachmentId)),
    );
  }, [form.description, workspaceId]);

  const create = async () => {
    if (!form.name.trim() || !form.teamId || !form.startDate || !form.endDate || submitting || descriptionPendingUploads > 0) return;
    setSubmitting(true);
    const { error: createError } = await pmSprintService.create({
      workspace_id: workspaceId,
      name: form.name.trim(),
      description: form.description.trim() || undefined,
      start_date: form.startDate,
      end_date: form.endDate,
      team_id: form.teamId || undefined,
    });
    setSubmitting(false);
    if (createError) {
      setError(createError);
      return;
    }
    window.dispatchEvent(new CustomEvent('sprint-created'));
    toast.success('Sprint created');

    // Check if team has sprint automations — prompt if not
    if (form.teamId) {
      try {
        const { data: automations } = await pmAutomationService.list(workspaceId);
        if (shouldPromptSprintAutomation(automations, form.teamId)) {
          const team = teams.find((t) => t.id === form.teamId);
          const start = new Date(form.startDate);
          const end = new Date(form.endDate);
          const durationDays = Math.round((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24));
          const weeks = Math.max(1, Math.round(durationDays / 7));
          // Brief delay so the user sees the success toast before the prompt
          await new Promise((resolve) => setTimeout(resolve, 600));
          setAutomationPrompt({
            teamId: form.teamId,
            teamName: team?.name ?? 'this team',
            sprintCount: 1, // 1 unstarted sprint ahead
            weeks,
            startDay: 1, // Monday
            moveUnfinished: true,
          });
          return;
        }
      } catch {
        // Non-critical — just skip the prompt
      }
    }

    onClose();
  };

  const enableAutomations = async () => {
    if (!automationPrompt) return;
    setEnablingAutomation(true);
    try {
      const createResult = await pmAutomationService.upsert(workspaceId, {
        workspace_id: workspaceId,
        automation_type: 'sprint_auto_create',
        enabled: true,
        team_id: automationPrompt.teamId,
        config_int: automationPrompt.sprintCount,
        config_int2: automationPrompt.weeks,
        config_int3: automationPrompt.startDay,
      });
      if (createResult.error) {
        toast.error(createResult.error);
        return;
      }

      if (automationPrompt.moveUnfinished) {
        const moveResult = await pmAutomationService.upsert(workspaceId, {
          workspace_id: workspaceId,
          automation_type: 'sprint_move_unfinished',
          enabled: true,
          team_id: automationPrompt.teamId,
        });
        if (moveResult.error) {
          const rollbackResult = await pmAutomationService.remove(
            workspaceId,
            'sprint_auto_create',
            automationPrompt.teamId,
          );
          if (rollbackResult.error) {
            toast.error(`${moveResult.error} Rollback failed: ${rollbackResult.error}`);
          } else {
            toast.error(moveResult.error);
          }
          return;
        }
      }

      toast.success('Sprint automation enabled for ' + automationPrompt.teamName);
      onClose();
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to enable sprint automation';
      toast.error(message);
    } finally {
      setEnablingAutomation(false);
    }
  };

  const dismissAutomations = async () => {
    if (!automationPrompt) return;
    setDismissingAutomation(true);
    try {
      await dismissSprintAutomationPrompt({
        workspaceId,
        prompt: automationPrompt,
        upsert: pmAutomationService.upsert,
      });
      toast.success('Sprint automation dismissed');
      onClose();
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to save sprint automation preference';
      toast.error(message);
    } finally {
      setDismissingAutomation(false);
    }
  };

  const DAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

  if (automationPrompt) {
    return (
      <Dialog
        open
        onOpenChange={(open) => {
          if (open) return;
          if (enablingAutomation || dismissingAutomation) return;
          void dismissAutomations();
        }}
      >
        <DialogContent className="max-w-sm">
          <div className="space-y-5">
            <div>
              <h3 className="text-base font-semibold">Set up sprint automation</h3>
              <p className="mt-1.5 text-sm text-muted-foreground">
                Never run out of sprints — the system will automatically create new sprints for {automationPrompt.teamName} so there are always sprints ready to plan into.
              </p>
            </div>

            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <Label className="text-sm">Each sprint lasts</Label>
                <Select
                  value={String(automationPrompt.weeks)}
                  onValueChange={(val) => setAutomationPrompt((p) => p ? { ...p, weeks: Number(val) } : p)}
                >
                  <SelectTrigger className="w-[140px] h-8 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {[1, 2, 3, 4].map((w) => (
                      <SelectItem key={w} value={String(w)}>{w} {w === 1 ? 'week' : 'weeks'}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex items-center justify-between">
                <Label className="text-sm">Sprints start on</Label>
                <Select
                  value={String(automationPrompt.startDay)}
                  onValueChange={(val) => setAutomationPrompt((p) => p ? { ...p, startDay: Number(val) } : p)}
                >
                  <SelectTrigger className="w-[140px] h-8 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {DAYS.map((day, i) => (
                      <SelectItem key={i} value={String(i)}>{day}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex items-center justify-between">
                <Label className="text-sm">Upcoming sprints to create</Label>
                <Select
                  value={String(automationPrompt.sprintCount)}
                  onValueChange={(val) => setAutomationPrompt((p) => p ? { ...p, sprintCount: Number(val) } : p)}
                >
                  <SelectTrigger className="w-[140px] h-8 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {[1, 2, 3, 4, 5].map((n) => (
                      <SelectItem key={n} value={String(n)}>{n} {n === 1 ? 'sprint' : 'sprints'}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex items-center justify-between pt-1">
                <div>
                  <p className="text-sm font-medium">Roll over unfinished work</p>
                  <p className="text-xs text-muted-foreground">When a sprint ends, move incomplete tasks to the next sprint</p>
                </div>
                <Switch
                  checked={automationPrompt.moveUnfinished}
                  onCheckedChange={(checked) => setAutomationPrompt((p) => p ? { ...p, moveUnfinished: checked } : p)}
                />
              </div>
            </div>

            <p className="text-[11px] text-muted-foreground">
              You can change this anytime in Team Settings.
            </p>

            <div className="flex justify-end gap-2">
              <Button variant="outline" size="sm" onClick={dismissAutomations} disabled={enablingAutomation || dismissingAutomation}>
                No thanks
              </Button>
              <Button size="sm" onClick={enableAutomations} disabled={enablingAutomation || dismissingAutomation}>
                {enablingAutomation ? <Loading01Icon className="mr-1 h-3.5 w-3.5 animate-spin" /> : null}
                {enablingAutomation ? 'Enabling...' : 'Enable'}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    );
  }

  const hasUnsavedChanges = form.name.trim() !== '' || form.description.trim() !== '';

  const handleClose = async () => {
    if (hasUnsavedChanges) {
      const ok = await confirm({
        title: 'Discard changes?',
        description: 'You have unsaved changes that will be lost.',
        confirmText: 'Discard',
        variant: 'destructive',
      });
      if (!ok) return;
    }
    void cleanupInlineDraftUploads();
    onClose();
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open) handleClose(); }}>
      <DialogContent className="max-w-4xl sm:max-w-4xl gap-0 overflow-hidden p-0" showCloseButton={false}>
        <div className="flex h-[80vh] flex-col">
          <div className="flex items-center justify-between border-b border-border/60 px-6 pt-4 pb-3">
            <span className="text-lg font-semibold">Create sprint</span>
            <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={handleClose}>
              <Cancel01Icon className="h-4 w-4" />
            </Button>
          </div>

          {error && (
            <div className="mx-4 mt-2 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
              {error}
            </div>
          )}

          <div className="grid min-h-0 flex-1 grid-cols-[1fr_280px] overflow-hidden">
            <div className="min-h-0 overflow-y-auto px-8 py-5">
              <Input
                autoFocus
                aria-label="Sprint title"
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                onKeyDown={(e) => {
                  if (e.key === 'Tab' && !e.shiftKey) {
                    e.preventDefault();
                    const editor = e.currentTarget.parentElement?.querySelector<HTMLElement>('.tiptap.ProseMirror');
                    editor?.focus();
                  }
                }}
                className="h-12 shrink-0 border-border/60 text-base shadow-none focus-visible:border-border"
                placeholder="Sprint title"
              />
              <div className="mt-4">
                <TiptapEditor
                  content={form.description}
                  onChange={(html) => setForm((f) => ({ ...f, description: html }))}
                  placeholder="Add a description (optional)..."
                  uploadConfig={{ workspaceId, entityType: 'editor_upload', entityId: workspaceId }}
                  onUploadStateChange={setDescriptionPendingUploads}
                  className="border-transparent shadow-none [&_.tiptap]:min-h-[180px]"
                  teams={mentionTeams}
                  members={assignableMembers}
                />
              </div>
            </div>

            <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-5">
              <p className="mb-4 text-xs text-muted-foreground">
                Sprints are time-boxed periods for planning and tracking work.
              </p>
              <div className="grid grid-cols-[16px_80px_1fr] items-center gap-x-2 gap-y-3">
                <UserGroupIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Team *</span>
                <Select value={form.teamId || '__none__'} onValueChange={(v) => setForm((f) => ({ ...f, teamId: v === '__none__' ? '' : v }))}>
                  <SelectTrigger className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent">
                    <SelectValue placeholder="Select team" />
                  </SelectTrigger>
                  <SelectContent>
                    {teams.filter((t) => t.sprints_enabled !== false).map((t) => (
                      <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <Calendar03Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Start date</span>
                <DatePicker
                  value={form.startDate}
                  onChange={(v) => setForm((f) => ({ ...f, startDate: v }))}
                  kind="start"
                  label="Start date"
                  linkedDate={{
                    label: 'End date',
                    kind: 'end',
                    value: form.endDate,
                    onChange: (v) => setForm((f) => ({ ...f, endDate: v })),
                    placeholder: 'None',
                  }}
                  placeholder="None"
                  hideIcon
                  className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent"
                />

                <Calendar03Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">End date</span>
                <DatePicker
                  value={form.startDate}
                  onChange={(v) => setForm((f) => ({ ...f, startDate: v }))}
                  kind="start"
                  label="Start date"
                  linkedDate={{
                    label: 'End date',
                    kind: 'end',
                    value: form.endDate,
                    onChange: (v) => setForm((f) => ({ ...f, endDate: v })),
                    placeholder: 'None',
                  }}
                  triggerField="linked"
                  defaultActiveField="linked"
                  placeholder="None"
                  hideIcon
                  className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent"
                />
              </div>
            </aside>
          </div>

          {/* Footer */}
          <div className="flex items-center justify-end gap-3 border-t border-border/50 px-6 py-3">
            <Button variant="outline" size="sm" onClick={handleClose} disabled={submitting}>
              Discard
            </Button>
            <Button size="sm" onClick={create} disabled={!form.name.trim() || !form.teamId || !form.startDate || !form.endDate || submitting || descriptionPendingUploads > 0}>
              {submitting ? <Loading01Icon className="mr-1 h-3.5 w-3.5 animate-spin" /> : null}
              {submitting ? 'Creating...' : 'Create Sprint'}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

// ── Objective dialog ──────────────────────────────────────────────────

const objectiveStateOptions: { value: ObjectiveState; label: string }[] = (
  Object.entries(OBJECTIVE_STATE_CONFIG) as [ObjectiveState, typeof OBJECTIVE_STATE_CONFIG[ObjectiveState]][]
).map(([value, cfg]) => ({ value, label: cfg.label }));

function MultiSelectPopover({
  items,
  selected,
  onChange,
  placeholder,
}: {
  items: { id: string; name: string }[];
  selected: string[];
  onChange: (ids: string[]) => void;
  placeholder: string;
}) {
  const [open, setOpen] = useState(false);

  const toggle = (id: string) => {
    onChange(selected.includes(id) ? selected.filter((s) => s !== id) : [...selected, id]);
  };

  const selectedNames = items.filter((i) => selected.includes(i.id)).map((i) => i.name);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex min-w-0 items-center gap-1 rounded-md px-1.5 py-1 text-xs transition-colors hover:bg-accent cursor-pointer truncate"
        >
          {selectedNames.length > 0 ? selectedNames.join(', ') : <span className="text-muted-foreground">{placeholder}</span>}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-56 p-1" align="start">
        <div className="flex max-h-60 flex-col overflow-y-auto">
          {items.map((item) => (
            <button
              key={item.id}
              type="button"
              className={`flex items-center gap-2 rounded-sm px-2 py-1.5 text-xs cursor-pointer transition-colors ${
                selected.includes(item.id) ? 'bg-accent text-foreground font-medium' : 'text-muted-foreground hover:bg-accent hover:text-foreground'
              }`}
              onClick={() => toggle(item.id)}
            >
              <span className={`flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-sm border ${
                selected.includes(item.id) ? 'bg-primary border-primary text-primary-foreground' : 'border-muted-foreground/30'
              }`}>
                {selected.includes(item.id) && <span className="text-[9px]">✓</span>}
              </span>
              <span className="truncate">{item.name}</span>
            </button>
          ))}
          {items.length === 0 && (
            <p className="px-2 py-1.5 text-xs text-muted-foreground">No options</p>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}

function GlobalCreateObjective({ workspaceId, onClose }: { workspaceId: string; onClose: () => void }) {
  const confirm = useConfirm();
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { canEdit } = usePermissions(access);
  const { teams } = useAccessibleTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);

  // Close immediately if the user lacks edit permission
  useEffect(() => {
    if (!canEdit) onClose();
  }, [canEdit, onClose]);

  const [form, setForm] = useState({
    name: '',
    description: '',
    objectiveType: 'tactical' as ObjectiveType,
    state: 'not_started' as ObjectiveState,
    teamIds: [] as string[],
    ownerMemberIds: [] as string[],
    startDate: '',
    targetDate: '',
  });
  const [submitting, setSubmitting] = useState(false);
  const [descriptionPendingUploads, setDescriptionPendingUploads] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const mentionTeams = useMemo(
    () => filterMentionTeams(teams, form.teamIds),
    [teams, form.teamIds],
  );
  const cleanupInlineDraftUploads = useCallback(async () => {
    const attachmentIds = extractInlineAttachmentIds(form.description);
    if (attachmentIds.length === 0) return;
    await Promise.allSettled(
      attachmentIds.map((attachmentId) => pmAttachmentService.remove(workspaceId, attachmentId)),
    );
  }, [form.description, workspaceId]);

  const create = async () => {
    if (!form.name.trim() || submitting || descriptionPendingUploads > 0) return;
    setSubmitting(true);
    const { error: createError } = await pmObjectiveService.create({
      workspace_id: workspaceId,
      name: form.name.trim(),
      description: form.description.trim() || undefined,
      objective_type: form.objectiveType,
      state: form.state,
      team_ids: form.teamIds.length > 0 ? form.teamIds : undefined,
      owner_member_ids: form.ownerMemberIds.length > 0 ? form.ownerMemberIds : undefined,
      planned_start_date: form.startDate || undefined,
      deadline: form.targetDate || undefined,
    });
    setSubmitting(false);
    if (createError) {
      setError(createError);
      return;
    }
    window.dispatchEvent(new CustomEvent('objective-created'));
    onClose();
  };

  const hasUnsavedChanges = form.name.trim() !== '' || form.description.trim() !== '';

  const handleClose = async () => {
    if (hasUnsavedChanges) {
      const ok = await confirm({
        title: 'Discard changes?',
        description: 'You have unsaved changes that will be lost.',
        confirmText: 'Discard',
        variant: 'destructive',
      });
      if (!ok) return;
    }
    void cleanupInlineDraftUploads();
    onClose();
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open) handleClose(); }}>
      <DialogContent className="max-w-4xl sm:max-w-4xl gap-0 overflow-hidden p-0" showCloseButton={false}>
        <div className="flex h-[80vh] flex-col">
          <div className="flex items-center justify-between border-b border-border/60 px-6 pt-4 pb-3">
            <span className="text-lg font-semibold">Create objective</span>
            <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={handleClose}>
              <Cancel01Icon className="h-4 w-4" />
            </Button>
          </div>

          {error && (
            <div className="mx-4 mt-2 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
              {error}
            </div>
          )}

          <div className="grid min-h-0 flex-1 grid-cols-[1fr_280px] overflow-hidden">
            <div className="min-h-0 overflow-y-auto px-8 py-5">
              <Input
                autoFocus
                aria-label="Objective title"
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                onKeyDown={(e) => {
                  if (e.key === 'Tab' && !e.shiftKey) {
                    e.preventDefault();
                    const editor = e.currentTarget.parentElement?.querySelector<HTMLElement>('.tiptap.ProseMirror');
                    editor?.focus();
                  }
                }}
                className="h-12 shrink-0 border-border/60 text-base shadow-none focus-visible:border-border"
                placeholder="Objective title"
              />
              <div className="mt-4">
                <TiptapEditor
                  content={form.description}
                  onChange={(html) => setForm((f) => ({ ...f, description: html }))}
                  placeholder="Add a description (optional)..."
                  uploadConfig={{ workspaceId, entityType: 'editor_upload', entityId: workspaceId }}
                  onUploadStateChange={setDescriptionPendingUploads}
                  className="border-transparent shadow-none"
                  teams={mentionTeams}
                  members={assignableMembers}
                />
              </div>

              {/* Objective Type Selector — commented out for now */}
              {/* <div className="mt-6">
                <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Objective Type</p>
                <div className="grid grid-cols-2 gap-3">
                  <button
                    type="button"
                    className={`flex flex-col gap-1 rounded-lg border p-3 text-left transition-colors cursor-pointer ${
                      form.objectiveType === 'tactical'
                        ? 'border-primary bg-primary/5'
                        : 'border-border/60 hover:border-border'
                    }`}
                    onClick={() => setForm((f) => ({ ...f, objectiveType: 'tactical' }))}
                  >
                    <div className="flex items-center gap-1.5">
                      <Target className="h-4 w-4 text-blue-500" />
                      <span className="text-sm font-medium">Tactical</span>
                    </div>
                    <span className="text-xs text-muted-foreground">Track linked Epics</span>
                  </button>
                  <button
                    type="button"
                    className={`flex flex-col gap-1 rounded-lg border p-3 text-left transition-colors cursor-pointer ${
                      form.objectiveType === 'strategic'
                        ? 'border-primary bg-primary/5'
                        : 'border-border/60 hover:border-border'
                    }`}
                    onClick={() => setForm((f) => ({ ...f, objectiveType: 'strategic' }))}
                  >
                    <div className="flex items-center gap-1.5">
                      <Crosshair className="h-4 w-4 text-violet-500" />
                      <span className="text-sm font-medium">Strategic</span>
                    </div>
                    <span className="text-xs text-muted-foreground">Group Key Results &amp; Epics</span>
                  </button>
                </div>
              </div> */}
            </div>

            <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-5">
              <p className="mb-4 text-xs text-muted-foreground">
                Objectives define high-level goals. Tactical objectives track linked Epics; Strategic objectives combine Key Results and Epics.
              </p>
              <div className="grid grid-cols-[16px_80px_1fr] items-center gap-x-2 gap-y-3">
                <HashtagIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">State</span>
                <Select value={form.state} onValueChange={(v) => setForm((f) => ({ ...f, state: v as ObjectiveState }))}>
                  <SelectTrigger className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {objectiveStateOptions.map((s) => (
                      <SelectItem key={s.value} value={s.value}>{s.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <UserGroupIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Teams</span>
                <MultiSelectPopover
                  items={teams.map((t) => ({ id: t.id, name: t.name }))}
                  selected={form.teamIds}
                  onChange={(ids) => setForm((f) => ({ ...f, teamIds: ids }))}
                  placeholder="Select teams"
                />

                <UserIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Owners</span>
                <MultiMemberPickerPopover
                  values={form.ownerMemberIds}
                  members={assignableMembers}
                  onChange={(ids) => setForm((f) => ({ ...f, ownerMemberIds: ids }))}
                  renderTrigger={() => {
                    const selectedMembers = assignableMembers.filter((member) => form.ownerMemberIds.includes(member.id));
                    if (selectedMembers.length === 0) {
                      return <span className="text-muted-foreground">Select owners</span>;
                    }

                    const label = selectedMembers
                      .map((member) => member.display_name || member.email)
                      .join(', ');

                    return (
                      <>
                        <div className="flex items-center -space-x-1">
                          {selectedMembers.slice(0, 2).map((member) => (
                            <UserAvatar
                              key={member.id}
                              name={member.display_name || member.email}
                              avatarUrl={member.avatar_url}
                              avatarStyle={member.avatar_style}
                              avatarSeed={member.avatar_seed}
                              avatarBackgroundMode={member.avatar_background_mode}
                              avatarBackgroundColor={member.avatar_background_color}
                              className="h-4 w-4"
                              fallbackClassName="text-[7px]"
                            />
                          ))}
                        </div>
                        <span className="truncate">{label}</span>
                      </>
                    );
                  }}
                  contentClassName="w-[260px]"
                />

                <Calendar03Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Start date</span>
                <DatePicker
                  value={form.startDate}
                  onChange={(v) => setForm((f) => ({ ...f, startDate: v }))}
                  kind="start"
                  label="Start date"
                  linkedDate={{
                    label: 'Target date',
                    kind: 'target',
                    value: form.targetDate,
                    onChange: (v) => setForm((f) => ({ ...f, targetDate: v })),
                    placeholder: 'None',
                  }}
                  placeholder="None"
                  hideIcon
                  className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent"
                />

                <Calendar03Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                <span className="text-xs text-muted-foreground self-center">Target date</span>
                <DatePicker
                  value={form.startDate}
                  onChange={(v) => setForm((f) => ({ ...f, startDate: v }))}
                  kind="start"
                  label="Start date"
                  linkedDate={{
                    label: 'Target date',
                    kind: 'target',
                    value: form.targetDate,
                    onChange: (v) => setForm((f) => ({ ...f, targetDate: v })),
                    placeholder: 'None',
                  }}
                  triggerField="linked"
                  defaultActiveField="linked"
                  placeholder="None"
                  hideIcon
                  className="h-8 border-0 bg-transparent px-1.5 shadow-none text-xs hover:bg-accent"
                />
              </div>
            </aside>
          </div>

          {/* Footer */}
          <div className="flex items-center justify-end gap-3 border-t border-border/50 px-6 py-3">
            <Button variant="outline" size="sm" onClick={handleClose} disabled={submitting}>
              Discard
            </Button>
            <Button size="sm" onClick={create} disabled={!form.name.trim() || submitting || descriptionPendingUploads > 0}>
              {submitting ? <Loading01Icon className="mr-1 h-3.5 w-3.5 animate-spin" /> : null}
              {submitting ? 'Creating...' : 'Create Objective'}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

// ── Main export ──────────────────────────────────────────────────────

export function GlobalCreateModals({ workspaceId }: { workspaceId: string }) {
  const { activeModal, closeCreate, initialSpaceId, initialCollectionId, initialParentCollectionId } = useGlobalCreateStore();
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const wsSlug = workspace?.slug ?? '';

  if (!activeModal) return null;

  return (
    <>
      {activeModal === 'task' && <GlobalCreateTask workspaceId={workspaceId} onClose={closeCreate} />}
      {activeModal === 'epic' && <GlobalCreateEpic workspaceId={workspaceId} onClose={closeCreate} />}
      {activeModal === 'sprint' && <GlobalCreateSprint workspaceId={workspaceId} onClose={closeCreate} />}
      {activeModal === 'objective' && <GlobalCreateObjective workspaceId={workspaceId} onClose={closeCreate} />}
      {activeModal === 'crm_contact' && (
        <CreateContactDialog
          open
          onOpenChange={(open) => !open && closeCreate()}
        />
      )}
      {activeModal === 'docs_document' && (
        <CreateDocumentDialog
          wsId={workspaceId}
          open
          onOpenChange={(open) => !open && closeCreate()}
          defaultSpaceId={initialSpaceId}
          defaultCollectionId={initialCollectionId}
          onCreated={(docId) => {
            closeCreate();
            navigate({ to: '/w/$slug/docs/documents/$docId', params: { slug: wsSlug, docId } });
          }}
        />
      )}
      {activeModal === 'docs_space' && (
        <CreateSpaceDialog
          wsId={workspaceId}
          open
          onOpenChange={(open) => !open && closeCreate()}
        />
      )}
      {activeModal === 'docs_collection' && (
        <CreateCollectionDialog
          wsId={workspaceId}
          spaceId={initialSpaceId}
          defaultParentCollectionId={initialParentCollectionId}
          open
          onOpenChange={(open) => !open && closeCreate()}
        />
      )}
    </>
  );
}
