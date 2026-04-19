import { useEffect, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { useQueryClient } from '@tanstack/react-query';
import {
  Building03Icon,
  Delete01Icon,
  DollarCircleIcon,
  File01Icon,
  GitBranchIcon,
  Loading01Icon,
  PlusSignIcon,
  Search01Icon,
  UserGroupIcon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { CreateTaskModal } from '@/components/pm/CreateTaskModal';
import {
  useConversationAssociations,
  useCreatePMAssociation,
  useDeletePMAssociation,
  useCreateDocAssociation,
  useDeleteDocAssociation,
  useWorkflows,
} from '@/hooks/queries';
import { crmSearchService } from '@/lib/services/crmService';
import { searchService, type SearchResult } from '@/lib/services/searchService';
import { supportService } from '@/lib/services/supportService';
import { queryKeys } from '@/lib/queryKeys';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import type { CRMObjectType, CRMSearchResult } from '@/lib/crmTypes';
import type { CreateTaskRequest, GroupedAssociations } from '@/lib/pmTypes';

interface SidebarAssociationsProps {
  workspaceId: string;
  conversationId: string;
}

type SectionKey = 'tasks' | 'crm' | 'docs';

const crmIconMap = {
  contact: UserGroupIcon,
  company: Building03Icon,
  deal: DollarCircleIcon,
} as const;

export function SidebarAssociations({ workspaceId, conversationId }: SidebarAssociationsProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const slug = useWorkspaceStore((s) => s.currentWorkspace?.slug ?? '');

  const associationsQuery = useConversationAssociations(workspaceId, conversationId);
  const associationsData = associationsQuery.data as GroupedAssociations | undefined;

  const createAssociation = useCreatePMAssociation(workspaceId);
  const deleteAssociation = useDeletePMAssociation(workspaceId);
  const createDocAssociation = useCreateDocAssociation(workspaceId, 'support_conversation', conversationId);
  const deleteDocAssociation = useDeleteDocAssociation(workspaceId, 'support_conversation', conversationId);

  const handleNavigateTask = (taskId: string) => {
    if (!slug) return;
    openTaskRoute(navigate as never, location as never, slug, taskId);
  };

  const handleNavigateDoc = (docId: string) => {
    navigate({ to: '/w/$slug/docs/documents/$docId', params: { slug, docId } } as any);
  };

  const handleNavigateCRM = (objectType: CRMObjectType, objectId: string) => {
    if (objectType === 'contact') {
      navigate({ to: '/w/$slug/crm/contacts/$contactId', params: { slug, contactId: objectId } } as any);
    } else if (objectType === 'company') {
      navigate({ to: '/w/$slug/crm/companies/$companyId', params: { slug, companyId: objectId } } as any);
    } else if (objectType === 'deal') {
      navigate({ to: '/w/$slug/crm/deals/$dealId', params: { slug, dealId: objectId } } as any);
    }
  };

  const [pickerSection, setPickerSection] = useState<SectionKey | null>(null);
  const [query, setQuery] = useState('');
  const [searching, setSearching] = useState(false);
  const [results, setResults] = useState<SearchResult[]>([]);
  const [crmResults, setCRMResults] = useState<CRMSearchResult[]>([]);
  const [createTaskOpen, setCreateTaskOpen] = useState(false);

  const { data: workflows = [] } = useWorkflows(workspaceId);
  const workflow = workflows[0] ?? null;

  const invalidateSupportAssociationViews = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['support', workspaceId] }),
      queryClient.invalidateQueries({ queryKey: ['crm', workspaceId] }),
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, conversationId) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversationAssociations(workspaceId, conversationId) }),
    ]);
  };

  useEffect(() => {
    if (!pickerSection) {
      setQuery('');
      setResults([]);
      setCRMResults([]);
      setSearching(false);
      return;
    }
    if (query.trim().length < 2) {
      setResults([]);
      setCRMResults([]);
      return;
    }

    const handle = window.setTimeout(async () => {
      setSearching(true);
      if (pickerSection === 'crm') {
        const response = await crmSearchService.search(workspaceId, query.trim());
        setCRMResults(response.data ?? []);
        setResults([]);
      } else {
        const response = await searchService.search(workspaceId, query.trim());
        setCRMResults([]);
        if (pickerSection === 'tasks') {
          setResults(response.data?.tasks ?? []);
        } else if (pickerSection === 'docs') {
          setResults(response.data?.documents ?? []);
        }
      }
      setSearching(false);
    }, 250);

    return () => window.clearTimeout(handle);
  }, [pickerSection, query, workspaceId]);

  const handleAdd = async (sectionKey: SectionKey, id: string) => {
    if (sectionKey === 'docs') {
      await createDocAssociation.mutateAsync({
        documentId: id,
        payload: { linked_object_type: 'support_conversation', linked_object_id: conversationId, link_context: 'attached' },
      });
    } else {
      await createAssociation.mutateAsync({
        workspace_id: workspaceId,
        from_object_type: 'support_conversation',
        from_object_id: conversationId,
        to_object_type: 'task',
        to_object_id: id,
      });
    }
    setPickerSection(null);
  };

  const handleAddCRM = async (toObjectType: CRMObjectType, toObjectId: string) => {
    if (toObjectType === 'contact') {
      const response = await supportService.updateConversationCRMContact(workspaceId, conversationId, {
        crm_contact_id: toObjectId,
      });
      if (response.error) throw new Error(response.error);
      await invalidateSupportAssociationViews();
      setPickerSection(null);
      return;
    }

    await createAssociation.mutateAsync({
      workspace_id: workspaceId,
      from_object_type: 'support_conversation',
      from_object_id: conversationId,
      to_object_type: toObjectType,
      to_object_id: toObjectId,
    });
    setPickerSection(null);
  };

  const handleRemoveCRM = async (item: GroupedAssociations['crm_records'][number]) => {
    if (item.object_type === 'contact' && !item.inferred) {
      const response = await supportService.updateConversationCRMContact(workspaceId, conversationId, {
        crm_contact_id: null,
      });
      if (response.error) throw new Error(response.error);
      await invalidateSupportAssociationViews();
      return;
    }
    if (item.association_id) {
      await deleteAssociation.mutateAsync(item.association_id);
    }
  };

  const handleCreateAndLinkStory = async (payload: CreateTaskRequest) => {
    const response = await supportService.createTaskFromConversation(workspaceId, conversationId, payload);
    if (response.error) throw new Error(response.error);

    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, conversationId) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversationAssociations(workspaceId, conversationId) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.tasks(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.board(workspaceId) }),
    ]);

    return response.data
      ? {
          id: response.data.task_id,
          task: {
            id: response.data.task_id,
            name: response.data.task_name,
            display_id: response.data.display_id,
            task_key: response.data.task_key,
          },
        }
      : undefined;
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
  const crmRecords = associationsData?.crm_records ?? [];
  const docs = associationsData?.docs ?? [];

  return (
    <div>
      <CollapsibleSection
        title="Tasks"
        count={tasks.length}
        defaultOpen={tasks.length > 0}
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
            {tasks.map((item) => (
              <div
                key={`${item.object_type}-${item.object_id}`}
                className="group flex items-center gap-2 rounded-md px-1 py-1.5 text-xs transition-colors hover:bg-muted/40"
              >
                <button
                  type="button"
                  className="flex min-w-0 flex-1 items-center gap-2 text-left"
                  onClick={() => handleNavigateTask(item.object_id)}
                >
                  {item.display_id && (
                    <span className="shrink-0 text-muted-foreground">{item.display_id}</span>
                  )}
                  <span className="truncate">{item.title}</span>
                  {item.status && (
                    <span className="ml-auto shrink-0 text-[10px] text-muted-foreground">{item.status}</span>
                  )}
                </button>
                {item.association_id ? (
                  <button
                    type="button"
                    className="shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-all hover:bg-background hover:text-destructive group-hover:opacity-100"
                    onClick={() => deleteAssociation.mutate(item.association_id!)}
                    aria-label="Remove task association"
                  >
                    <Delete01Icon className="h-3 w-3" />
                  </button>
                ) : null}
              </div>
            ))}
          </div>
        )}
      </CollapsibleSection>

      <CollapsibleSection
        title="CRM"
        count={crmRecords.length}
        defaultOpen={crmRecords.length > 0}
        onAdd={() => setPickerSection('crm')}
      >
        {crmRecords.length === 0 ? (
          <button
            type="button"
            onClick={() => setPickerSection('crm')}
            className="flex w-full items-center justify-center gap-1.5 rounded-md border border-dashed border-border/60 px-3 py-2 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-muted/40 hover:text-foreground"
          >
            <PlusSignIcon className="h-3.5 w-3.5" />
            Link a contact, company, or deal
          </button>
        ) : (
          <div className="space-y-0.5">
            {crmRecords.map((item) => {
              const CRMIcon = crmIconMap[item.object_type as keyof typeof crmIconMap] ?? Building03Icon;
              return (
                <div
                  key={`${item.object_type}-${item.object_id}`}
                  className="group flex items-center gap-2 rounded-md px-1 py-1.5 text-xs transition-colors hover:bg-muted/40"
                >
                  <button
                    type="button"
                    className="flex min-w-0 flex-1 items-center gap-2 text-left"
                    onClick={() => handleNavigateCRM(item.object_type as CRMObjectType, item.object_id)}
                  >
                    <CRMIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    <span className="truncate font-medium">{item.title}</span>
                    {(item.context_label || item.display_id) && (
                      <span className="ml-auto flex shrink-0 items-center gap-2">
                        {item.context_label && (
                          <span className="text-[10px] text-muted-foreground">
                            {item.context_label}
                          </span>
                        )}
                        {item.display_id && (
                          <span className="text-[10px] text-muted-foreground opacity-0 transition-opacity delay-0 group-hover:opacity-100 group-hover:delay-200">
                            {item.display_id}
                          </span>
                        )}
                      </span>
                    )}
                  </button>
                  {(item.association_id || (item.object_type === 'contact' && !item.inferred)) ? (
                    <button
                      type="button"
                      className="shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-all hover:bg-background hover:text-destructive group-hover:opacity-100"
                      onClick={() => void handleRemoveCRM(item)}
                      aria-label="Remove CRM association"
                    >
                      <Delete01Icon className="h-3 w-3" />
                    </button>
                  ) : null}
                </div>
              );
            })}
          </div>
        )}
      </CollapsibleSection>

      <CollapsibleSection
        title="Docs"
        count={docs.length}
        defaultOpen={docs.length > 0}
        onAdd={() => setPickerSection('docs')}
      >
        {docs.length === 0 ? (
          <button
            type="button"
            onClick={() => setPickerSection('docs')}
            className="flex w-full items-center justify-center gap-1.5 rounded-md border border-dashed border-border/60 px-3 py-2 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-muted/40 hover:text-foreground"
          >
            <PlusSignIcon className="h-3.5 w-3.5" />
            Link a doc
          </button>
        ) : (
          <div className="space-y-0.5">
            {docs.map((item) => (
              <div
                key={`${item.object_type}-${item.object_id}`}
                className="group flex items-center gap-2 rounded-md px-1 py-1.5 text-xs transition-colors hover:bg-muted/40"
              >
                <button
                  type="button"
                  className="flex min-w-0 flex-1 items-center gap-2 text-left"
                  onClick={() => handleNavigateDoc(item.object_id)}
                >
                  <File01Icon className="h-3 w-3 shrink-0 text-muted-foreground" />
                  <span className="truncate font-medium">{item.title}</span>
                  {item.display_id && (
                    <span className="ml-auto shrink-0 text-muted-foreground">{item.display_id}</span>
                  )}
                </button>
                {item.association_id ? (
                  <button
                    type="button"
                    className="shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-all hover:bg-background hover:text-destructive group-hover:opacity-100"
                    onClick={() => deleteDocAssociation.mutate(item.association_id!)}
                    aria-label="Remove document association"
                  >
                    <Delete01Icon className="h-3 w-3" />
                  </button>
                ) : null}
              </div>
            ))}
          </div>
        )}
      </CollapsibleSection>

      {/* Link existing modal */}
      <Dialog open={!!pickerSection} onOpenChange={(open) => { if (!open) setPickerSection(null); }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-sm">
              Link {pickerSection === 'tasks' ? 'Task' : pickerSection === 'crm' ? 'CRM Record' : 'Document'}
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
            <div className="relative">
              <Search01Icon className="pointer-events-none absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={
                  pickerSection === 'tasks'
                    ? 'Search existing tasks...'
                    : pickerSection === 'crm'
                      ? 'Search contacts, companies, or deals...'
                      : 'Search documents...'
                }
                className="pl-9"
                autoFocus
              />
            </div>
            <div className="max-h-64 space-y-1 overflow-y-auto">
              {searching && (
                <div className="flex items-center gap-2 py-4 justify-center text-sm text-muted-foreground">
                  <Loading01Icon className="h-4 w-4 animate-spin" /> Searching...
                </div>
              )}
              {!searching && pickerSection === 'crm' && crmResults.map((result) => (
                <button
                  key={`${result.type}-${result.id}`}
                  type="button"
                  className="w-full rounded-md border px-3 py-2 text-left text-sm transition hover:bg-accent"
                  onClick={() => handleAddCRM(result.type, result.id)}
                >
                  <div className="flex items-center gap-2">
                    <Building03Icon className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    <span className="font-medium truncate">{result.name}</span>
                    {'display_id' in result.object && result.object.display_id && (
                      <Badge variant="outline" className="h-5 px-1.5 text-[10px] shrink-0">
                        #{result.object.display_id}
                      </Badge>
                    )}
                  </div>
                </button>
              ))}
              {!searching && results.map((r) => (
                <button
                  key={r.id}
                  type="button"
                  className="w-full rounded-md border px-3 py-2 text-left text-sm transition hover:bg-accent"
                  onClick={() => handleAdd(pickerSection!, r.id)}
                >
                  <div className="flex items-center gap-2">
                    {pickerSection === 'tasks' ? <GitBranchIcon className="h-3.5 w-3.5 text-muted-foreground shrink-0" /> : <File01Icon className="h-3.5 w-3.5 text-muted-foreground shrink-0" />}
                    <span className="font-medium truncate">{r.name}</span>
                    {r.display_id && (
                      <Badge variant="outline" className="h-5 px-1.5 text-[10px] shrink-0">
                        #{r.display_id}
                      </Badge>
                    )}
                  </div>
                </button>
              ))}
              {!searching && query.trim().length >= 2 && pickerSection !== 'crm' && results.length === 0 && (
                <p className="py-4 text-sm text-muted-foreground text-center">No results found</p>
              )}
              {!searching && query.trim().length >= 2 && pickerSection === 'crm' && crmResults.length === 0 && (
                <p className="py-4 text-sm text-muted-foreground text-center">No results found</p>
              )}
              {!searching && query.trim().length < 2 && (
                <p className="py-4 text-sm text-muted-foreground text-center">Type at least 2 characters to search</p>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* Create Task modal — creates and auto-links to this conversation */}
      {workflow && (
        <CreateTaskModal
          open={createTaskOpen}
          onOpenChange={setCreateTaskOpen}
          workspaceId={workspaceId}
          workflow={workflow}
          initialStateId={workflow.states?.[0]?.id ?? ''}
          onCreate={handleCreateAndLinkStory}
        />
      )}
    </div>
  );
}
