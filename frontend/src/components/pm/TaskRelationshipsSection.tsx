import { type ReactNode, type RefObject, useEffect, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import {
  ArrowLeftRightIcon,
  Tick01Icon,
  Copy01Icon,
  File01Icon,
  Loading01Icon,
  MoreHorizontalIcon,
  PlusSignIcon,
  Search01Icon,
  Shield02Icon,
  Delete01Icon,
  Alert01Icon,
} from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { DocumentPreviewDialog } from '@/components/docs/DocumentPreviewDialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  useCreateDocAssociation,
  useCreateTask,
  useCreateTaskRelationship,
  useDeleteDocAssociation,
  useDeleteTaskRelationship,
  useTaskAssociations,
} from '@/hooks/queries';
import { searchService, type SearchResult } from '@/lib/services/searchService';
import type {
  CreateTaskRequest,
  GroupedAssociations,
  TaskRelationshipAction,
} from '@/lib/pmTypes';
import { TaskTypeIcon } from '@/lib/pmConstants';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { TaskDetailSectionHeading } from '@/components/pm/task-detail/TaskDetailSectionHeading';

interface TaskRelationshipsSectionProps {
  workspaceId: string;
  taskId: string;
  taskName: string;
  taskDisplayId: number;
  workflowId?: string;
  workflowStateId?: string;
  epicId?: string;
  sprintId?: string;
  teamId?: string;
  taskType?: 'feature' | 'bug' | 'chore';
  priority?: 'none' | 'low' | 'medium' | 'high' | 'urgent';
  severity?: 'none' | 'minor' | 'major' | 'critical';
  externalBlocker: string;
  onExternalBlockerChange: (value: string) => void;
  composerOpen: boolean;
  onComposerOpenChange: (open: boolean) => void;
  visible?: boolean;
  /** Ref to an external trigger (e.g. the action-bar "Relationships" button).
   *  When set, clicking that element opens the popover anchored there instead of inline. */
  externalTriggerRef?: RefObject<HTMLElement | null>;
  className?: string;
}

const RELATIONSHIP_OPTIONS: Array<{
  value: TaskRelationshipAction;
  label: string;
  icon: typeof ArrowLeftRightIcon | typeof Alert01Icon;
}> = [
  { value: 'relates_to', label: 'relates to', icon: ArrowLeftRightIcon },
  { value: 'blocks', label: 'blocks', icon: Alert01Icon },
  { value: 'is_blocked_by', label: 'is blocked by', icon: Shield02Icon },
  { value: 'duplicates', label: 'duplicates', icon: Copy01Icon },
  { value: 'is_duplicated_by', label: 'is duplicated by', icon: Copy01Icon },
];

const UPDATE_TYPE_OPTIONS: Array<{
  value: TaskRelationshipAction;
  label: string;
  icon: typeof ArrowLeftRightIcon | typeof Alert01Icon;
}> = [
  { value: 'blocks', label: 'Blocks', icon: Alert01Icon },
  { value: 'is_blocked_by', label: 'Blocked by', icon: Shield02Icon },
  { value: 'duplicates', label: 'Duplicates', icon: Copy01Icon },
  { value: 'is_duplicated_by', label: 'Duplicated by', icon: Copy01Icon },
  { value: 'relates_to', label: 'Relates to', icon: ArrowLeftRightIcon },
];

const RELATIONSHIP_COLORS: Record<TaskRelationshipAction, { active: string; icon: string }> = {
  relates_to: {
    active: 'border-blue-200/80 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-950/50 dark:text-blue-300',
    icon: 'text-blue-500',
  },
  blocks: {
    active: 'border-amber-200/80 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-300',
    icon: 'text-amber-500',
  },
  is_blocked_by: {
    active: 'border-red-200/80 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-950/50 dark:text-red-300',
    icon: 'text-red-500',
  },
  duplicates: {
    active: 'border-violet-200/80 bg-violet-50 text-violet-700 dark:border-violet-800 dark:bg-violet-950/50 dark:text-violet-300',
    icon: 'text-violet-500',
  },
  is_duplicated_by: {
    active: 'border-violet-200/80 bg-violet-50 text-violet-700 dark:border-violet-800 dark:bg-violet-950/50 dark:text-violet-300',
    icon: 'text-violet-500',
  },
};

function getRelationshipMeta(linkType: string) {
  switch (linkType) {
    case 'blocks':
      return { label: 'Blocks', icon: Alert01Icon, color: 'text-amber-600 dark:text-amber-400', bg: 'bg-amber-50 dark:bg-amber-950/40' };
    case 'is_blocked_by':
      return { label: 'Blocked by', icon: Shield02Icon, color: 'text-red-600 dark:text-red-400', bg: 'bg-red-50 dark:bg-red-950/40' };
    case 'relates_to':
    case 'related_by':
      return { label: 'Relates to', icon: ArrowLeftRightIcon, color: 'text-blue-600 dark:text-blue-400', bg: 'bg-blue-50 dark:bg-blue-950/40' };
    case 'duplicates':
      return { label: 'Duplicates', icon: Copy01Icon, color: 'text-violet-600 dark:text-violet-400', bg: 'bg-violet-50 dark:bg-violet-950/40' };
    case 'is_duplicated_by':
      return { label: 'Duplicated by', icon: Copy01Icon, color: 'text-violet-600 dark:text-violet-400', bg: 'bg-violet-50 dark:bg-violet-950/40' };
    default:
      return { label: linkType, icon: ArrowLeftRightIcon, color: 'text-muted-foreground', bg: 'bg-muted/60' };
  }
}

/* ------------------------------------------------------------------ */
/*  Floating popover rendered via a portal, positioned to an anchor   */
/* ------------------------------------------------------------------ */

function FloatingPopover({
  open,
  onOpenChange,
  anchorEl,
  children,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  anchorEl: HTMLElement | null;
  children: ReactNode;
}) {
  const popoverRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ top: number; left: number }>({ top: 0, left: 0 });

  // Track anchor position on scroll / resize
  useEffect(() => {
    if (!open || !anchorEl) return;
    const update = () => {
      const rect = anchorEl.getBoundingClientRect();
      setPos({ top: rect.bottom + 4, left: rect.left });
    };
    update();
    // Listen on the nearest scrollable ancestor + window resize
    const scrollParent = anchorEl.closest('[class*="overflow"]') ?? window;
    scrollParent.addEventListener('scroll', update, { passive: true });
    window.addEventListener('resize', update, { passive: true });
    return () => {
      scrollParent.removeEventListener('scroll', update);
      window.removeEventListener('resize', update);
    };
  }, [open, anchorEl]);

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (
        popoverRef.current &&
        !popoverRef.current.contains(e.target as Node) &&
        anchorEl &&
        !anchorEl.contains(e.target as Node)
      ) {
        onOpenChange(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open, anchorEl, onOpenChange]);

  // Close on Escape
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onOpenChange(false);
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [open, onOpenChange]);

  if (!open || !anchorEl) return null;

  return (
    <div
      ref={popoverRef}
      className="fixed z-50 w-[min(38rem,calc(100vw-2rem))] rounded-xl border border-border/60 bg-popover text-popover-foreground shadow-lg ring-1 ring-black/[0.04] dark:ring-white/[0.04] animate-in fade-in-0 zoom-in-95 slide-in-from-top-2"
      style={{
        top: pos.top,
        left: pos.left,
      }}
    >
      {children}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Main component                                                    */
/* ------------------------------------------------------------------ */

export function TaskRelationshipsSection({
  workspaceId,
  taskId,
  taskName: _taskName,
  taskDisplayId: _taskDisplayId,
  workflowId,
  workflowStateId,
  epicId,
  sprintId,
  teamId,
  taskType,
  priority,
  severity,
  externalBlocker,
  onExternalBlockerChange,
  composerOpen,
  onComposerOpenChange,
  visible = false,
  externalTriggerRef,
  className,
}: TaskRelationshipsSectionProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const inlineAddRef = useRef<HTMLButtonElement>(null);
  const [anchorSource, setAnchorSource] = useState<'external' | 'inline'>('inline');
  const [popoverTab, setPopoverTab] = useState<'tasks' | 'docs'>('tasks');
  const [query, setQuery] = useState('');
  const [relationshipType, setRelationshipType] = useState<TaskRelationshipAction>('relates_to');
  const [searching, setSearching] = useState(false);
  const [taskResults, setTaskResults] = useState<SearchResult[]>([]);
  const [docResults, setDocResults] = useState<SearchResult[]>([]);
  const [previewDocId, setPreviewDocId] = useState<string | null>(null);

  const associationsQuery = useTaskAssociations(workspaceId, taskId);
  const data = associationsQuery.data as GroupedAssociations | undefined;
  const createRelationship = useCreateTaskRelationship(workspaceId, taskId);
  const deleteRelationship = useDeleteTaskRelationship(workspaceId, taskId);
  const createTask = useCreateTask(workspaceId);
  const createDocAssociation = useCreateDocAssociation(workspaceId, 'task', taskId);
  const deleteDocAssociation = useDeleteDocAssociation(workspaceId, 'task', taskId);

  // Detect which trigger opened the popover
  useEffect(() => {
    if (composerOpen) {
      const active = document.activeElement;
      if (externalTriggerRef?.current && externalTriggerRef.current.contains(active as Node)) {
        setAnchorSource('external');
      }
      // "inline" is set explicitly in the onClick handler
    }
  }, [composerOpen, externalTriggerRef]);

  useEffect(() => {
    if (!composerOpen) {
      setQuery('');
      setSearching(false);
      setTaskResults([]);
      setDocResults([]);
      return;
    }

    const handle = window.setTimeout(async () => {
      if (query.trim().length < 2) {
        setTaskResults([]);
        setDocResults([]);
        setSearching(false);
        return;
      }

      setSearching(true);
      const response = await searchService.search(workspaceId, query.trim());
      if (popoverTab === 'tasks') {
        setTaskResults((response.data?.tasks ?? []).filter(( s) => s.id !== taskId));
      } else {
        setDocResults(response.data?.documents ?? []);
      }
      setSearching(false);
    }, 220);

    return () => window.clearTimeout(handle);
  }, [composerOpen, query, taskId, workspaceId, popoverTab]);

  const allRelationships = useMemo(() => {
    const taskRels = data?.task_relationships;
    if (!taskRels) return [];
    const rels = taskRels;
    return [
      ...rels.blocked_by.map((r) => ({ ...r, link_type: 'is_blocked_by' as const })),
      ...rels.blocking.map((r) => ({ ...r, link_type: 'blocks' as const })),
      ...rels.relates_to.map((r) => ({ ...r, link_type: 'relates_to' as const })),
      ...rels.related_by.map((r) => ({ ...r, link_type: 'related_by' as const })),
      ...rels.duplicates.map((r) => ({ ...r, link_type: 'duplicates' as const })),
      ...rels.duplicated_by.map((r) => ({ ...r, link_type: 'is_duplicated_by' as const })),
    ];
  }, [data]);

  const linkedDocs = useMemo(() => data?.docs ?? [], [data]);

  const handleOpenTask = (targetTaskId: string) => {
    if (!workspace?.slug) return;
    openTaskRoute(navigate as never, location as never, workspace.slug, targetTaskId);
  };

  const handleCreateRelationship = async (otherTaskId: string) => {
    await createRelationship.mutateAsync({
      relationship_type: relationshipType,
      other_task_id: otherTaskId,
    });
    onComposerOpenChange(false);
  };

  const handleCreateRelatedTask = async () => {
    const name = query.trim();
    if (!name) return;

    const payload: CreateTaskRequest = {
      workspace_id: workspaceId,
      name,
      workflow_id: workflowId,
      workflow_state_id: workflowStateId,
      epic_id: epicId,
      sprint_id: sprintId,
      team_id: teamId,
      task_type: taskType,
      priority,
      severity,
    };

    const created = await createTask.mutateAsync(payload);
    await createRelationship.mutateAsync({
      relationship_type: relationshipType,
      other_task_id: created.task.id,
    });
    onComposerOpenChange(false);
  };

  const handleLinkDoc = async (documentId: string) => {
    await createDocAssociation.mutateAsync({
      documentId,
      payload: {
        linked_object_type: 'task',
        linked_object_id: taskId,
        link_context: 'attached',
      },
    });
    onComposerOpenChange(false);
  };

  const handleUpdateRelationshipType = async (
    relationshipId: string,
    otherTaskId: string,
    newType: TaskRelationshipAction,
  ) => {
    await deleteRelationship.mutateAsync(relationshipId);
    await createRelationship.mutateAsync({
      relationship_type: newType,
      other_task_id: otherTaskId,
    });
  };

  const busy =
    createRelationship.isPending ||
    deleteRelationship.isPending ||
    createTask.isPending ||
    createDocAssociation.isPending ||
    deleteDocAssociation.isPending;

  // Resolve the popover anchor element
  const anchorEl =
    anchorSource === 'external' && externalTriggerRef?.current
      ? externalTriggerRef.current
      : inlineAddRef.current;

  const popoverBody: ReactNode = (
    <>
      <div className="border-b border-border/60 bg-muted/20 px-3 pt-2.5 pb-2">
        <Tabs
          value={popoverTab}
          onValueChange={(v) => {
            setPopoverTab(v as 'tasks' | 'docs');
            setQuery('');
            setTaskResults([]);
            setDocResults([]);
          }}
        >
          <TabsList className="h-8 w-fit gap-0.5 rounded-lg bg-muted/60 p-0.5">
            <TabsTrigger
              value="tasks"
              className="h-7 rounded-md px-3.5 text-xs font-medium data-[state=active]:bg-background data-[state=active]:shadow-sm"
            >
              Tasks
            </TabsTrigger>
            <TabsTrigger
              value="docs"
              className="h-7 rounded-md px-3.5 text-xs font-medium data-[state=active]:bg-background data-[state=active]:shadow-sm"
            >
              Docs
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      <div className="space-y-2.5 px-3 py-2.5">
        {popoverTab === 'tasks' ? (
          <>
            <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/70">This Task...</p>
            <div className="flex flex-wrap gap-1.5">
              {RELATIONSHIP_OPTIONS.map((option) => {
                const OptionIcon = option.icon;
                const isActive = relationshipType === option.value;
                const colors = RELATIONSHIP_COLORS[option.value];
                return (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() => setRelationshipType(option.value)}
                    className={cn(
                      'inline-flex h-8 items-center gap-1.5 rounded-lg border px-2.5 text-xs font-medium transition-all',
                      isActive
                        ? `${colors.active} shadow-sm`
                        : 'border-border/60 bg-background text-muted-foreground hover:border-border hover:bg-accent/50',
                    )}
                  >
                    <OptionIcon className={cn('h-3.5 w-3.5', isActive ? colors.icon : 'text-muted-foreground/60')} />
                    {option.label}
                  </button>
                );
              })}
            </div>
          </>
        ) : null}

        <div className="relative">
          <Search01Icon className="pointer-events-none absolute left-2.5 top-2.5 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={popoverTab === 'tasks' ? 'Search Task Title or ID' : 'Search documents'}
            className="pl-8"
          />
        </div>

        <div className="max-h-56 space-y-1 overflow-y-auto">
          {searching ? (
            <div className="flex items-center gap-2 rounded-lg border border-dashed border-border/70 px-3 py-3 text-sm text-muted-foreground">
              <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
              Searching...
            </div>
          ) : null}

          {!searching &&
            popoverTab === 'tasks' &&
            taskResults.map((task) => (
              <button
                key={task.id}
                type="button"
                onClick={() => handleCreateRelationship(task.id)}
                className="flex w-full items-center gap-2.5 rounded-lg border border-transparent px-3 py-2 text-left transition-all hover:border-border/40 hover:bg-accent/50"
              >
                <span className="min-w-0 flex-1 truncate text-ui font-medium">{task.name}</span>
                {(task.task_key || task.display_id) ? (
                  <Badge variant="outline" className="h-5 shrink-0 rounded-full px-1.5 text-[10px] text-muted-foreground">
                    {task.task_key ?? task.display_id}
                  </Badge>
                ) : null}
              </button>
            ))}

          {!searching &&
            popoverTab === 'docs' &&
            docResults.map((doc) => (
              <button
                key={doc.id}
                type="button"
                onClick={() => handleLinkDoc(doc.id)}
                className="flex w-full items-center gap-2.5 rounded-lg border border-transparent px-3 py-2 text-left transition-all hover:border-border/40 hover:bg-accent/50"
              >
                <File01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span className="min-w-0 flex-1 truncate text-ui font-medium">{doc.name}</span>
              </button>
            ))}

          {!searching &&
            query.trim().length >= 2 &&
            ((popoverTab === 'tasks' && taskResults.length === 0) ||
              (popoverTab === 'docs' && docResults.length === 0)) ? (
            <div className="rounded-lg border border-dashed border-border/70 px-3 py-3 text-sm text-muted-foreground">
              No results found.
            </div>
          ) : null}
        </div>

        {popoverTab === 'tasks' ? (
          <div className="flex items-center justify-between gap-3 border-t border-border/60 pt-3">
            <span className="text-xs text-muted-foreground/70">Search for an existing task or</span>
            <Button
              type="button"
              variant="outline"
              className="h-7 gap-1 rounded-lg border-border/60 px-2.5 text-xs font-medium transition-all hover:border-primary/30 hover:bg-primary/5 hover:text-primary"
              disabled={query.trim().length === 0}
              onClick={handleCreateRelatedTask}
            >
              <PlusSignIcon className="h-3 w-3" />
              Create Related Task
            </Button>
          </div>
        ) : null}
      </div>
    </>
  );

  // The action-bar toggle controls section visibility; the inline button controls the composer.
  if (!visible && !composerOpen) {
    return null;
  }

  return (
    <section id="task-relationships-section" className={className}>
      <TaskDetailSectionHeading title="Task Relationships" icon={ArrowLeftRightIcon} />

      {associationsQuery.error ? (
        <div className="mt-3 rounded-lg border border-destructive/20 bg-destructive/5 px-3 py-2.5 text-sm text-destructive">
          {(associationsQuery.error as Error).message}
        </div>
      ) : null}

      {/* Flat relationship list — no borders */}
      <div className="mt-3">
        {allRelationships.map((item) => {
          const meta = getRelationshipMeta(item.link_type);
          const Icon = meta.icon;
          const resolved = item.link_type === 'blocks' && !item.is_active;
          const relatedTask = item.task;
          if (!relatedTask) {
            return null;
          }
          return (
            <div
              key={item.relationship_id}
              className={cn(
                'group flex items-center gap-1.5 rounded-md px-1.5 py-1 -mx-1.5 transition-colors hover:bg-accent/40',
                resolved && 'opacity-50',
              )}
            >
              <div className="flex min-w-0 flex-1 items-center gap-1.5">
                <Icon className={cn('h-3.5 w-3.5 shrink-0', meta.color)} />
                <span className={cn('shrink-0 rounded-md px-1.5 py-0.5 text-[10px] font-medium', meta.bg, meta.color)}>{meta.label}</span>
                <button
                  type="button"
                  onClick={() => handleOpenTask(relatedTask.object_id)}
                  className="min-w-0 truncate text-ui font-medium text-left transition-colors hover:text-primary"
                >
                  {relatedTask.title}
                </button>
              </div>
              <div className="flex shrink-0 items-center gap-1.5">
                {(relatedTask.task_key || relatedTask.display_id) ? (
                  <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px] font-medium gap-1">
                    {relatedTask.task_type ? (
                      <TaskTypeIcon taskType={relatedTask.task_type} className="h-3.5 w-3.5" />
                    ) : null}
                    {relatedTask.task_key ?? relatedTask.display_id}
                    {(relatedTask.completed || resolved) ? (
                      <Tick01Icon className="h-3 w-3 text-green-600" />
                    ) : null}
                  </Badge>
                ) : null}
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      className="h-6 w-6 shrink-0 rounded-md flex items-center justify-center opacity-0 transition-opacity hover:bg-accent group-hover:opacity-100"
                    >
                      <MoreHorizontalIcon className="h-3.5 w-3.5 text-muted-foreground" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-48">
                    <DropdownMenuLabel className="text-xs">Update Relationship Type</DropdownMenuLabel>
                    {UPDATE_TYPE_OPTIONS.map((opt) => {
                      const OptIcon = opt.icon;
                      return (
                        <DropdownMenuItem
                          key={opt.value}
                          onClick={() => handleUpdateRelationshipType(item.relationship_id, relatedTask.object_id, opt.value)}
                        >
                          <OptIcon className="mr-2 h-3.5 w-3.5" />
                          {opt.label}
                        </DropdownMenuItem>
                      );
                    })}
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      className="text-destructive focus:text-destructive"
                      onClick={() => deleteRelationship.mutate(item.relationship_id)}
                    >
                      <Delete01Icon className="mr-2 h-3.5 w-3.5" />
                      Remove relationship
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>
          );
        })}

        {/* Linked docs */}
        {linkedDocs.map((doc) => (
          <div
            key={`doc-${doc.object_id}-${doc.association_id ?? 'f'}`}
            className="group flex items-center gap-1.5 rounded-md px-1.5 py-1 -mx-1.5 transition-colors hover:bg-accent/40"
          >
            <button
              type="button"
              className="flex min-w-0 flex-1 items-center gap-1.5 text-left"
              onClick={() => setPreviewDocId(doc.object_id)}
            >
              <File01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              <span className="shrink-0 rounded-md bg-muted/60 px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">Doc</span>
              <span className="min-w-0 flex-1 truncate text-ui font-medium">{doc.title}</span>
            </button>
            {doc.association_id ? (
              <div className="ml-auto flex shrink-0 items-center">
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      className="h-6 w-6 shrink-0 rounded-md flex items-center justify-center opacity-0 transition-opacity hover:bg-accent group-hover:opacity-100"
                    >
                      <MoreHorizontalIcon className="h-3.5 w-3.5 text-muted-foreground" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-48">
                    <DropdownMenuItem
                      className="text-destructive focus:text-destructive"
                      onClick={() => deleteDocAssociation.mutate(doc.association_id!)}
                    >
                      <Delete01Icon className="mr-2 h-3.5 w-3.5" />
                      Remove link
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            ) : null}
          </div>
        ))}
      </div>

      {/* + Add Relationship */}
      <div className="mt-2 flex items-center gap-2">
        <Button
          ref={inlineAddRef}
          type="button"
          size="sm"
          variant="ghost"
          className="inline-flex h-auto items-center gap-2 rounded-md px-3 py-1.5 text-sm font-medium text-muted-foreground hover:bg-accent hover:text-foreground transition-colors cursor-pointer"
          onClick={() => {
            setAnchorSource('inline');
            onComposerOpenChange(true);
          }}
        >
          <PlusSignIcon className="h-4 w-4" />
          Add relationship
        </Button>

        {busy ? (
          <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
            <Loading01Icon className="h-3 w-3 animate-spin" />
          </span>
        ) : null}
      </div>

      {/* Floating popover — anchored to whichever trigger was clicked */}
      <FloatingPopover open={composerOpen} onOpenChange={onComposerOpenChange} anchorEl={anchorEl}>
        {popoverBody}
      </FloatingPopover>

      <DocumentPreviewDialog
        workspaceId={workspaceId}
        slug={workspace?.slug ?? ''}
        docId={previewDocId}
        open={!!previewDocId}
        onOpenChange={(open) => {
          if (!open) setPreviewDocId(null);
        }}
      />

      {/* External blocker */}
      {externalBlocker || allRelationships.some((r) => r.link_type === 'is_blocked_by') ? (
        <div className="mt-4 space-y-1.5">
          <div className="flex items-center gap-1.5">
            <Shield02Icon className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">External blocker</span>
          </div>
          <textarea
            value={externalBlocker}
            onChange={(event) => onExternalBlockerChange(event.target.value)}
            placeholder="Customer dependency, vendor issue, legal review..."
            className="min-h-[60px] w-full rounded-2xl border border-transparent bg-input/50 px-3 py-2 text-sm outline-none transition-[color,box-shadow,background-color] placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/30"
          />
        </div>
      ) : null}
    </section>
  );
}
