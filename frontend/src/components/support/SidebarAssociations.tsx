import { useEffect, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { openDealRoute } from '@/components/crm/deal-detail/dealRouteNavigation';
import {
  BookOpen01Icon,
  DollarCircleIcon,
  File01Icon,
  GitBranchIcon,
  Image01Icon,
  Loading01Icon,
  PlusSignIcon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import {
  QuietRelationshipDialogContent,
  QuietRelationshipResults,
  QuietSearchInput,
  quietRelatedItemTitleClassName,
  quietRelationshipResultRowClassName,
} from '@/components/design-system/quiet';
import { Badge } from '@/components/ui/badge';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import {
  Dialog,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { SupportCreateTaskModal } from './SupportCreateTaskModal';
import { SupportAttachmentLightbox } from './SupportAttachmentLightbox';
import { isLightboxAttachment } from './attachmentLightbox';
import { useSupportImageContent } from './useSupportImageContent';
import {
  useConversationAssociations,
  useCreatePMAssociation,
  useWorkflows,
  useWorkspaceAccess,
} from '@/hooks/queries';
import { useConversationSharedItems } from '@/hooks/queries/useSupport';
import { crmSearchService } from '@/lib/services/crmService';
import { searchService, type SearchResult } from '@/lib/services/searchService';
import { formatFileSize } from '@/lib/supportAttachmentFiles';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import type { CRMSearchResult } from '@/lib/crmTypes';
import type { GroupedAssociations, SupportAttachmentPayload, SupportSharedItem } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

interface SidebarAssociationsProps {
  workspaceId: string;
  conversationId: string;
}

type SectionKey = 'tasks' | 'deals';
const SIDEBAR_PREVIEW_LIMIT = 3;

const sidebarRowClassName = 'group flex items-center gap-2 rounded-md px-1 py-1.5 text-xs transition-colors hover:bg-quiet-hover';

function sharedItemIcon(item: SupportSharedItem) {
  if (item.kind === 'doc') return BookOpen01Icon;
  return item.file_type?.startsWith('image/') ? Image01Icon : File01Icon;
}

function sharedFileAttachment(item: SupportSharedItem): SupportAttachmentPayload {
  return {
    id: item.id,
    file_key: '',
    file_name: item.title,
    file_type: item.file_type ?? '',
    file_size: item.file_size ?? 0,
    url: item.url,
  };
}

export function SidebarAssociations({ workspaceId, conversationId }: SidebarAssociationsProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const slug = useWorkspaceStore((s) => s.currentWorkspace?.slug ?? '');
  const { data: access } = useWorkspaceAccess(workspaceId);
  const pmEnabled = access?.modules?.includes('pm') ?? false;

  const associationsQuery = useConversationAssociations(workspaceId, conversationId);
  const associationsData = associationsQuery.data as GroupedAssociations | undefined;
  const { data: sharedItems = [] } = useConversationSharedItems(workspaceId, conversationId);
  const loadImageContent = useSupportImageContent(workspaceId, conversationId);
  const previewFiles = sharedItems
    .filter((item) => item.kind === 'file')
    .map(sharedFileAttachment)
    .filter(isLightboxAttachment);

  const createAssociation = useCreatePMAssociation(workspaceId);

  const handleNavigateTask = (taskId: string) => {
    if (!slug) return;
    openTaskRoute(navigate as never, location as never, slug, taskId);
  };

  const [pickerSection, setPickerSection] = useState<SectionKey | null>(null);
  const [query, setQuery] = useState('');
  const [searching, setSearching] = useState(false);
  const [results, setResults] = useState<SearchResult[]>([]);
  const [dealResults, setDealResults] = useState<CRMSearchResult[]>([]);
  const [createTaskOpen, setCreateTaskOpen] = useState(false);
  const [tasksExpanded, setTasksExpanded] = useState(false);
  const [lightboxIndex, setLightboxIndex] = useState<number | null>(null);

  const { data: workflows = [] } = useWorkflows(pmEnabled ? workspaceId : '');
  const workflowStateColorById = new Map(
    workflows.flatMap((entry) =>
      entry.states
        .filter((state) => !!state.color)
        .map((state) => [state.id, state.color!] as const),
    ),
  );

  useEffect(() => {
    setTasksExpanded(false);
    setLightboxIndex(null);
  }, [conversationId]);

  useEffect(() => {
    if (!pickerSection) {
      setQuery('');
      setResults([]);
      setDealResults([]);
      setSearching(false);
      return;
    }
    if (query.trim().length < 2) {
      setResults([]);
      setDealResults([]);
      return;
    }

    const handle = window.setTimeout(async () => {
      setSearching(true);
      if (pickerSection === 'deals') {
        const response = await crmSearchService.search(workspaceId, query.trim());
        setDealResults((response.data ?? []).filter((result) => result.type === 'deal'));
        setResults([]);
      } else {
        const response = await searchService.search(workspaceId, query.trim());
        setDealResults([]);
        setResults(response.data?.tasks ?? []);
      }
      setSearching(false);
    }, 250);

    return () => window.clearTimeout(handle);
  }, [pickerSection, query, workspaceId]);

  const linkObject = async (toObjectType: 'task' | 'deal', toObjectId: string) => {
    await createAssociation.mutateAsync({
      workspace_id: workspaceId,
      from_object_type: 'support_conversation',
      from_object_id: conversationId,
      to_object_type: toObjectType,
      to_object_id: toObjectId,
    });
    setPickerSection(null);
  };

  if (associationsQuery.isLoading) {
    return (
      <div className="flex items-center gap-2 px-3 py-3 text-xs text-muted-foreground">
        <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
        Loading...
      </div>
    );
  }

  const tasks = associationsData?.tasks ?? [];
  // Contacts and companies already appear in the identity card and Company details.
  const deals = (associationsData?.crm_records ?? []).filter((record) => record.object_type === 'deal');
  const visibleTasks = tasksExpanded ? tasks : tasks.slice(0, SIDEBAR_PREVIEW_LIMIT);

  return (
    <div>
      {pmEnabled && <CollapsibleSection
        key={`${workspaceId}:${conversationId}:tasks`}
        title="Tasks"
        count={tasks.length}
        autoOpenWhenPopulated
        persistKey="support-details.tasks"
        onAdd={() => setPickerSection('tasks')}
      >
        {tasks.length === 0 ? (
          <button
            type="button"
            onClick={() => setPickerSection('tasks')}
            className="flex w-full items-center justify-center gap-1.5 rounded-md border border-dashed border-border/60 px-3 py-2 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-muted/40 hover:text-foreground"
          >
            <PlusSignIcon className="h-3.5 w-3.5" />
            Link a task
          </button>
        ) : (
          <div className="space-y-0.5">
            {visibleTasks.map((item) => (
              <div key={`${item.object_type}-${item.object_id}`} className={sidebarRowClassName}>
                <button
                  type="button"
                  className="flex min-w-0 flex-1 items-center gap-2 text-left"
                  onClick={() => handleNavigateTask(item.object_id)}
                >
                  {item.status ? (
                    <QuickTooltip label={item.status}>
                      <span
                        className="h-2.5 w-2.5 shrink-0 rounded-full bg-muted-foreground/30"
                        style={{
                          backgroundColor:
                            workflowStateColorById.get(item.workflow_state_id ?? '') ??
                            (item.completed ? 'rgb(34 197 94)' : undefined),
                        }}
                        aria-label={item.status}
                      />
                    </QuickTooltip>
                  ) : (
                    <span
                      className="h-2.5 w-2.5 shrink-0 rounded-full bg-muted-foreground/30"
                      style={{
                        backgroundColor:
                          workflowStateColorById.get(item.workflow_state_id ?? '') ??
                          (item.completed ? 'rgb(34 197 94)' : undefined),
                      }}
                      aria-hidden="true"
                    />
                  )}
                  <span className={cn('truncate', quietRelatedItemTitleClassName)}>{item.title}</span>
                  {item.display_id && (
                    <span className="ml-auto shrink-0 text-[10px] text-muted-foreground">
                      {item.display_id}
                    </span>
                  )}
                </button>
              </div>
            ))}
            {tasks.length > SIDEBAR_PREVIEW_LIMIT && (
              <button
                type="button"
                onClick={() => setTasksExpanded((current) => !current)}
                className="px-1 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
              >
                {tasksExpanded ? 'Show less' : `Show ${tasks.length - SIDEBAR_PREVIEW_LIMIT} more`}
              </button>
            )}
          </div>
        )}
      </CollapsibleSection>}

      <CollapsibleSection
        key={`${workspaceId}:${conversationId}:deals`}
        title="Deals"
        count={deals.length}
        autoOpenWhenPopulated
        persistKey="support-details.deals"
        onAdd={() => setPickerSection('deals')}
      >
        {deals.length === 0 ? (
          <button
            type="button"
            onClick={() => setPickerSection('deals')}
            className="flex w-full items-center justify-center gap-1.5 rounded-md border border-dashed border-border/60 px-3 py-2 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-muted/40 hover:text-foreground"
          >
            <PlusSignIcon className="h-3.5 w-3.5" />
            Link a deal
          </button>
        ) : (
          <div className="space-y-0.5">
            {deals.map((item) => (
              <div key={`${item.object_type}-${item.object_id}`} className={sidebarRowClassName}>
                <button
                  type="button"
                  className="flex min-w-0 flex-1 items-center gap-2 text-left"
                  onClick={() => openDealRoute(navigate as never, location, slug, item.object_id)}
                >
                  <DollarCircleIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  <span className={cn('truncate', quietRelatedItemTitleClassName)}>{item.title}</span>
                  {item.context_label && (
                    <span className="ml-auto shrink-0 text-[10px] text-muted-foreground">
                      {item.context_label}
                    </span>
                  )}
                </button>
              </div>
            ))}
          </div>
        )}
      </CollapsibleSection>

      {sharedItems.length > 0 && (
        <CollapsibleSection
          key={`${workspaceId}:${conversationId}:shared`}
          title="Docs & files"
          count={sharedItems.length}
          autoOpenWhenPopulated
          persistKey="support-details.shared-items"
        >
          <div className="space-y-0.5">
            {sharedItems.map((item) => {
              const ItemIcon = sharedItemIcon(item);
              const key = `${item.kind}-${item.id}-${item.message_id}`;
              const content = (
                <>
                  <ItemIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  <span className={cn('min-w-0 truncate', quietRelatedItemTitleClassName)}>{item.title}</span>
                  <span className="ml-auto shrink-0 text-[10px] text-muted-foreground">
                    {item.kind === 'doc' ? 'Article' : item.file_size ? formatFileSize(item.file_size) : 'File'}
                  </span>
                </>
              );
              const previewIndex = item.kind === 'file' ? previewFiles.findIndex((file) => file.id === item.id) : -1;
              // Images, videos and PDFs open in the lightbox; articles and other files open in a new tab.
              return previewIndex >= 0 ? (
                <button
                  key={key}
                  type="button"
                  onClick={() => setLightboxIndex(previewIndex)}
                  className={cn(sidebarRowClassName, 'w-full text-left')}
                >
                  {content}
                </button>
              ) : (
                <a key={key} href={item.url} target="_blank" rel="noopener noreferrer" className={sidebarRowClassName}>
                  {content}
                </a>
              );
            })}
          </div>
        </CollapsibleSection>
      )}

      {lightboxIndex !== null && (
        <SupportAttachmentLightbox
          attachments={previewFiles}
          index={lightboxIndex}
          onIndexChange={setLightboxIndex}
          onClose={() => setLightboxIndex(null)}
          loadImageContent={loadImageContent}
        />
      )}

      {/* Link existing modal */}
      <Dialog open={!!pickerSection} onOpenChange={(open) => { if (!open) setPickerSection(null); }}>
        <QuietRelationshipDialogContent>
          <DialogHeader>
            <DialogTitle className="text-sm">
              Link {pickerSection === 'tasks' ? 'Task' : 'Deal'}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            {pickerSection === 'tasks' && (
              <Button
                variant="outline"
                size="sm"
                className="w-full gap-1.5 text-xs"
                onClick={() => {
                  setPickerSection(null);
                  setCreateTaskOpen(true);
                }}
              >
                <PlusSignIcon className="h-3.5 w-3.5" />
                Create New Task
              </Button>
            )}
            <QuietSearchInput
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={pickerSection === 'tasks' ? 'Search existing tasks...' : 'Search deals...'}
              autoFocus
            />
            <QuietRelationshipResults className="max-h-64 space-y-1 overflow-y-auto">
              {searching && (
                <div className="flex items-center gap-2 py-4 justify-center text-sm text-muted-foreground">
                  <Loading01Icon className="h-4 w-4 animate-spin" /> Searching...
                </div>
              )}
              {!searching && pickerSection === 'deals' && dealResults.map((result) => (
                <button
                  key={`${result.type}-${result.id}`}
                  type="button"
                  className={cn(quietRelationshipResultRowClassName, 'w-full rounded-md border px-3 py-2 text-left text-sm transition hover:bg-accent')}
                  onClick={() => void linkObject('deal', result.id)}
                >
                  <div className="flex min-w-0 items-center gap-2">
                    <DollarCircleIcon className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    <span className="min-w-0 flex-1 truncate font-medium">{result.name}</span>
                    {'display_id' in result.object && result.object.display_id && (
                      <Badge variant="outline" className="h-5 px-1.5 text-[10px] shrink-0">
                        #{result.object.display_id}
                      </Badge>
                    )}
                  </div>
                </button>
              ))}
              {!searching && pickerSection === 'tasks' && results.map((r) => (
                <button
                  key={r.id}
                  type="button"
                  className={cn(quietRelationshipResultRowClassName, 'w-full rounded-md border px-3 py-2 text-left text-sm transition hover:bg-accent')}
                  onClick={() => void linkObject('task', r.id)}
                >
                  <div className="flex min-w-0 items-center gap-2">
                    <GitBranchIcon className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    <span className="min-w-0 flex-1 truncate font-medium">{r.name}</span>
                    {r.display_id && (
                      <Badge variant="outline" className="h-5 px-1.5 text-[10px] shrink-0">
                        #{r.display_id}
                      </Badge>
                    )}
                  </div>
                </button>
              ))}
              {!searching && query.trim().length >= 2 && pickerSection === 'tasks' && results.length === 0 && (
                <p className="py-4 text-sm text-muted-foreground text-center">No results found</p>
              )}
              {!searching && query.trim().length >= 2 && pickerSection === 'deals' && dealResults.length === 0 && (
                <p className="py-4 text-sm text-muted-foreground text-center">No results found</p>
              )}
              {!searching && query.trim().length < 2 && (
                <p className="py-4 text-sm text-muted-foreground text-center">Type at least 2 characters to search</p>
              )}
            </QuietRelationshipResults>
          </div>
        </QuietRelationshipDialogContent>
      </Dialog>

      {/* Create Task modal — creates and auto-links to this conversation */}
      {pmEnabled && createTaskOpen && <SupportCreateTaskModal key={`${workspaceId}:${conversationId}`} workspaceId={workspaceId} workspaceSlug={slug} conversationId={conversationId} onOpenChange={setCreateTaskOpen} />}
    </div>
  );
}
