import { useEffect, useState } from 'react';
import {
  ChevronDown,
  ChevronRight,
  FileText,
  GitBranch,
  Loader2,
  Plus,
  Search,
  Trash2,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { CreateStoryModal } from '@/components/pm/CreateStoryModal';
import {
  useConversationAssociations,
  useCreatePMAssociation,
  useDeletePMAssociation,
  useCreateDocAssociation,
  useDeleteDocAssociation,
  useWorkflows,
} from '@/hooks/queries';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { searchService, type SearchResult } from '@/lib/services/searchService';
import type { AssociationObjectSummary, GroupedAssociations, CreateStoryRequest } from '@/lib/pmTypes';

interface SidebarAssociationsProps {
  workspaceId: string;
  conversationId: string;
}

type SectionKey = 'stories' | 'docs';

function CollapsibleSection({
  title,
  icon: Icon,
  count,
  defaultOpen = false,
  onAdd,
  children,
}: {
  title: string;
  icon: React.ElementType;
  count: number;
  defaultOpen?: boolean;
  onAdd?: () => void;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <div className="border-b last:border-b-0">
      <div className="flex items-center">
        <button
          type="button"
          className="flex flex-1 items-center gap-2 px-3 py-2.5 text-xs font-medium text-muted-foreground hover:bg-muted/50 transition-colors"
          onClick={() => setOpen(!open)}
        >
          {open ? <ChevronDown className="h-3.5 w-3.5 shrink-0" /> : <ChevronRight className="h-3.5 w-3.5 shrink-0" />}
          <Icon className="h-3.5 w-3.5 shrink-0" />
          <span className="flex-1 text-left">{title}</span>
          {count > 0 && (
            <Badge variant="secondary" className="h-4 px-1 text-[10px]">{count}</Badge>
          )}
        </button>
        {onAdd && (
          <button
            type="button"
            className="mr-2 rounded p-1 text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
            onClick={(e) => { e.stopPropagation(); onAdd(); }}
            aria-label={`Add ${title.toLowerCase()}`}
          >
            <Plus className="h-3.5 w-3.5" />
          </button>
        )}
      </div>
      {open && (
        <div className="px-3 pb-2.5 space-y-1.5">
          {children}
        </div>
      )}
    </div>
  );
}

function CompactChip({
  item,
  onRemove,
}: {
  item: AssociationObjectSummary;
  onRemove?: () => void;
}) {
  return (
    <div className="flex items-center justify-between gap-1.5 rounded-md border px-2 py-1.5 text-xs">
      <div className="min-w-0 flex-1">
        <span className="font-medium truncate block">{item.title}</span>
      </div>
      {item.display_id && (
        <span className="shrink-0 text-[10px] text-muted-foreground">{item.display_id}</span>
      )}
      {onRemove && (
        <button type="button" onClick={onRemove} className="shrink-0 text-muted-foreground hover:text-destructive transition-colors">
          <Trash2 className="h-3 w-3" />
        </button>
      )}
    </div>
  );
}

export function SidebarAssociations({ workspaceId, conversationId }: SidebarAssociationsProps) {
  const associationsQuery = useConversationAssociations(workspaceId, conversationId);
  const data = associationsQuery.data as GroupedAssociations | undefined;

  const createAssociation = useCreatePMAssociation(workspaceId);
  const deleteAssociation = useDeletePMAssociation(workspaceId);
  const createDocAssociation = useCreateDocAssociation(workspaceId, 'support_conversation', conversationId);
  const deleteDocAssociation = useDeleteDocAssociation(workspaceId, 'support_conversation', conversationId);

  const [pickerSection, setPickerSection] = useState<SectionKey | null>(null);
  const [query, setQuery] = useState('');
  const [searching, setSearching] = useState(false);
  const [results, setResults] = useState<SearchResult[]>([]);
  const [createStoryOpen, setCreateStoryOpen] = useState(false);

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
      if (pickerSection === 'stories') {
        setResults(response.data?.stories ?? []);
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
        to_object_type: 'story',
        to_object_id: id,
      });
    }
    setPickerSection(null);
  };

  const handleCreateAndLinkStory = async (payload: CreateStoryRequest) => {
    const { data, error } = await pmStoryService.create(payload);
    if (error) throw new Error(error);
    const storyId = data?.story?.id;
    if (storyId) {
      await createAssociation.mutateAsync({
        workspace_id: workspaceId,
        from_object_type: 'support_conversation',
        from_object_id: conversationId,
        to_object_type: 'story',
        to_object_id: storyId,
      });
    }
    return storyId ? { id: storyId } : undefined;
  };

  if (associationsQuery.isLoading) {
    return (
      <div className="flex items-center gap-2 px-3 py-3 text-xs text-muted-foreground">
        <Loader2 className="h-3.5 w-3.5 animate-spin" />
        Loading...
      </div>
    );
  }

  const stories = data?.stories ?? [];
  const docs = data?.docs ?? [];

  return (
    <div>
      <CollapsibleSection
        title="Stories"
        icon={GitBranch}
        count={stories.length}
        defaultOpen={stories.length > 0}
        onAdd={() => setPickerSection('stories')}
      >
        {stories.length === 0 ? (
          <p className="text-[11px] text-muted-foreground italic py-1">No linked stories</p>
        ) : (
          stories.map((item) => (
            <CompactChip
              key={`${item.object_type}-${item.object_id}`}
              item={item}
              onRemove={item.association_id ? () => deleteAssociation.mutate(item.association_id!) : undefined}
            />
          ))
        )}
      </CollapsibleSection>

      <CollapsibleSection
        title="Docs"
        icon={FileText}
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
              item={item}
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
              Link {pickerSection === 'stories' ? 'Story' : 'Document'}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            {pickerSection === 'stories' && (
              <Button
                variant="outline"
                size="sm"
                className="w-full gap-1.5 text-xs"
                onClick={() => {
                  setPickerSection(null);
                  setCreateStoryOpen(true);
                }}
              >
                <Plus className="h-3.5 w-3.5" />
                Create New Story
              </Button>
            )}
            <div className="relative">
              <Search className="pointer-events-none absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={pickerSection === 'stories' ? 'Search existing stories...' : 'Search documents...'}
                className="pl-9"
                autoFocus
              />
            </div>
            <div className="max-h-64 space-y-1 overflow-y-auto">
              {searching && (
                <div className="flex items-center gap-2 py-4 justify-center text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" /> Searching...
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
                    {pickerSection === 'stories' ? <GitBranch className="h-3.5 w-3.5 text-muted-foreground shrink-0" /> : <FileText className="h-3.5 w-3.5 text-muted-foreground shrink-0" />}
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

      {/* Create Story modal — creates and auto-links to this conversation */}
      {workflow && (
        <CreateStoryModal
          open={createStoryOpen}
          onOpenChange={setCreateStoryOpen}
          workspaceId={workspaceId}
          workflow={workflow}
          initialStateId={workflow.states?.[0]?.id ?? ''}
          onCreate={handleCreateAndLinkStory}
        />
      )}
    </div>
  );
}
