import { useEffect, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import {
  File01Icon,
  GitBranchIcon,
  Loading01Icon,
  PlusSignIcon,
  Search01Icon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
import { CompactChip } from '@/components/ui/compact-chip';
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
import { pmTaskService } from '@/lib/services/pmTaskService';
import { searchService, type SearchResult } from '@/lib/services/searchService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import type { CreateTaskRequest, GroupedAssociations } from '@/lib/pmTypes';

interface SidebarAssociationsProps {
  workspaceId: string;
  conversationId: string;
}

type SectionKey = 'tasks' | 'docs';

export function SidebarAssociations({ workspaceId, conversationId }: SidebarAssociationsProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const slug = useWorkspaceStore((s) => s.currentWorkspace?.slug ?? '');

  const associationsQuery = useConversationAssociations(workspaceId, conversationId);
  const data = associationsQuery.data as GroupedAssociations | undefined;

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

  const [pickerSection, setPickerSection] = useState<SectionKey | null>(null);
  const [query, setQuery] = useState('');
  const [searching, setSearching] = useState(false);
  const [results, setResults] = useState<SearchResult[]>([]);
  const [createTaskOpen, setCreateTaskOpen] = useState(false);

  const { data: workflows = [] } = useWorkflows(workspaceId);
  const workflow = workflows[0] ?? null;

  useEffect(() => {
    if (!pickerSection) {
      setQuery('');
      setResults([]);
      setSearching(false);
      return;
    }
    if (query.trim().length < 2) {
      setResults([]);
      return;
    }

    const handle = window.setTimeout(async () => {
      setSearching(true);
      const response = await searchService.search(workspaceId, query.trim());
      if (pickerSection === 'tasks') {
        setResults(response.data?.tasks ?? []);
      } else if (pickerSection === 'docs') {
        setResults(response.data?.documents ?? []);
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

  const handleCreateAndLinkStory = async (payload: CreateTaskRequest) => {
    const { data, error } = await pmTaskService.create(payload);
    if (error) throw new Error(error);
    const taskId = data?.task?.id;
    if (taskId) {
      await createAssociation.mutateAsync({
        workspace_id: workspaceId,
        from_object_type: 'support_conversation',
        from_object_id: conversationId,
        to_object_type: 'task',
        to_object_id: taskId,
      });
    }
    return taskId ? { id: taskId } : undefined;
  };

  if (associationsQuery.isLoading) {
    return (
      <div className="flex items-center gap-2 px-3 py-3 text-xs text-muted-foreground">
        <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
        Loading...
      </div>
    );
  }

  const tasks = data?.tasks ?? [];
  const docs = data?.docs ?? [];

  return (
    <div>
      <CollapsibleSection
        title="Tasks"
        icon={GitBranchIcon}
        count={tasks.length}
        defaultOpen={tasks.length > 0}
        onAdd={() => setPickerSection('tasks')}
      >
        {tasks.length === 0 ? (
          <p className="text-[11px] text-muted-foreground italic py-1">No linked tasks</p>
        ) : (
          tasks.map((item) => (
            <CompactChip
              key={`${item.object_type}-${item.object_id}`}
              title={item.title}
              displayId={item.display_id}
              onClick={() => handleNavigateTask(item.object_id)}
              onRemove={item.association_id ? () => deleteAssociation.mutate(item.association_id!) : undefined}
            />
          ))
        )}
      </CollapsibleSection>

      <CollapsibleSection
        title="Docs"
        icon={File01Icon}
        count={docs.length}
        defaultOpen={docs.length > 0}
        onAdd={() => setPickerSection('docs')}
      >
        {docs.length === 0 ? (
          <p className="text-[11px] text-muted-foreground italic py-1">No linked docs</p>
        ) : (
          docs.map((item) => (
            <CompactChip
              key={`${item.object_type}-${item.object_id}`}
              title={item.title}
              displayId={item.display_id}
              onClick={() => handleNavigateDoc(item.object_id)}
              onRemove={item.association_id ? () => deleteDocAssociation.mutate(item.association_id!) : undefined}
            />
          ))
        )}
      </CollapsibleSection>

      {/* Link existing modal */}
      <Dialog open={!!pickerSection} onOpenChange={(open) => { if (!open) setPickerSection(null); }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-sm">
              Link {pickerSection === 'tasks' ? 'Task' : 'Document'}
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
                placeholder={pickerSection === 'tasks' ? 'Search existing tasks...' : 'Search documents...'}
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
              {!searching && query.trim().length >= 2 && results.length === 0 && (
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
