import { useEffect, useMemo, useState } from 'react';
import {
  ArrowRightLeft,
  Building2,
  FileText,
  GitBranch,
  Link2,
  Loader2,
  Search,
  MessageSquareText,
  Trash2,
} from 'lucide-react';

import {
  useCreateDocAssociation,
  useCreatePMAssociation,
  useCreateStoryRelationship,
  useDeleteDocAssociation,
  useDeletePMAssociation,
  useDeleteStoryRelationship,
  useEpicAssociations,
  useStoryAssociations,
  useConversationAssociations,
} from '@/hooks/queries';
import { crmSearchService } from '@/lib/services/crmService';
import { searchService, type SearchResult } from '@/lib/services/searchService';
import { supportService } from '@/lib/services/supportService';
import { cn } from '@/lib/utils';
import type { CRMSearchResult, CRMObjectType } from '@/lib/crmTypes';
import type {
  AssociationObjectSummary,
  CreateStoryRelationshipRequest,
  GroupedAssociations,
  StoryRelationshipAction,
  StoryRelationshipSummary,
  SupportConversation,
} from '@/lib/pmTypes';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';

type AssociationsObjectType = 'story' | 'epic' | 'support_conversation';
type AssociationsTab = 'relationships' | 'stories' | 'support' | 'crm' | 'docs';

interface AssociationsPanelProps {
  workspaceId: string;
  objectType: AssociationsObjectType;
  objectId: string;
  className?: string;
  includeStoryRelationships?: boolean;
}

const RELATIONSHIP_OPTIONS: Array<{ value: StoryRelationshipAction; label: string }> = [
  { value: 'relates_to', label: 'relates to' },
  { value: 'blocks', label: 'blocks' },
  { value: 'is_blocked_by', label: 'is blocked by' },
  { value: 'duplicates', label: 'duplicates' },
  { value: 'is_duplicated_by', label: 'is duplicated by' },
];

function getTabs(objectType: AssociationsObjectType, includeStoryRelationships: boolean): AssociationsTab[] {
  switch (objectType) {
    case 'story':
      return includeStoryRelationships ? ['relationships', 'support', 'crm', 'docs'] : ['support', 'crm', 'docs'];
    case 'support_conversation':
      return ['stories', 'crm', 'docs'];
    default:
      return ['support', 'crm', 'docs'];
  }
}

function tabLabel(tab: AssociationsTab) {
  switch (tab) {
    case 'relationships':
      return 'Relationships';
    case 'stories':
      return 'Stories';
    case 'support':
      return 'Support';
    case 'crm':
      return 'CRM';
    case 'docs':
      return 'Docs';
    default:
      return tab;
  }
}

function ObjectChip({
  item,
  onRemove,
  muted = false,
}: {
  item: AssociationObjectSummary;
  onRemove?: () => void;
  muted?: boolean;
}) {
  return (
    <div
      className={cn(
        'flex items-start justify-between gap-2 rounded-lg border border-border/70 px-3 py-2',
        muted && 'bg-muted/30 text-muted-foreground'
      )}
    >
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm font-medium leading-none">{item.title}</span>
          {item.display_id ? (
            <Badge variant="outline" className="h-5 px-1.5 text-[10px] uppercase tracking-wide">
              {item.display_id}
            </Badge>
          ) : null}
          {item.status ? (
            <span className="text-[11px] text-muted-foreground">{item.status}</span>
          ) : null}
          {item.completed ? (
            <span className="text-[11px] text-muted-foreground">Completed</span>
          ) : null}
        </div>
        <p className="mt-1 text-[11px] text-muted-foreground">{item.object_type.replace('_', ' ')}</p>
      </div>
      {onRemove ? (
        <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={onRemove}>
          <Trash2 className="h-3.5 w-3.5" />
        </Button>
      ) : null}
    </div>
  );
}

function RelationshipGroup({
  title,
  description,
  items,
  onRemove,
}: {
  title: string;
  description: string;
  items: StoryRelationshipSummary[];
  onRemove: (relationshipId: string) => void;
}) {
  return (
    <div className="space-y-2">
      <div>
        <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</h4>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>
      {items.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border/70 px-3 py-3 text-xs text-muted-foreground">
          No relationships
        </div>
      ) : (
        <div className="space-y-2">
          {items.map((item) => (
            <ObjectChip
              key={item.relationship_id}
              item={item.story}
              muted={!item.is_active && item.link_type === 'blocks'}
              onRemove={() => onRemove(item.relationship_id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function AssociationsPanel({
  workspaceId,
  objectType,
  objectId,
  className,
  includeStoryRelationships = true,
}: AssociationsPanelProps) {
  const tabs = useMemo(() => getTabs(objectType, includeStoryRelationships), [includeStoryRelationships, objectType]);
  const [activeTab, setActiveTab] = useState<AssociationsTab>(tabs[0]);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [relationshipType, setRelationshipType] = useState<StoryRelationshipAction>('blocks');
  const [searching, setSearching] = useState(false);
  const [storyResults, setStoryResults] = useState<SearchResult[]>([]);
  const [crmResults, setCRMResults] = useState<CRMSearchResult[]>([]);
  const [docResults, setDocResults] = useState<SearchResult[]>([]);
  const [conversationResults, setConversationResults] = useState<SupportConversation[]>([]);

  const associationsQuery =
    objectType === 'story'
      ? useStoryAssociations(workspaceId, objectId)
      : objectType === 'epic'
        ? useEpicAssociations(workspaceId, objectId)
        : useConversationAssociations(workspaceId, objectId);
  const data = associationsQuery.data as GroupedAssociations | undefined;

  const createRelationship = useCreateStoryRelationship(workspaceId, objectId);
  const deleteRelationship = useDeleteStoryRelationship(workspaceId, objectId);
  const createAssociation = useCreatePMAssociation(workspaceId);
  const deleteAssociation = useDeletePMAssociation(workspaceId);
  const createDocAssociation = useCreateDocAssociation(workspaceId, objectType as 'epic' | 'story' | 'support_conversation', objectId);
  const deleteDocAssociation = useDeleteDocAssociation(workspaceId, objectType as 'epic' | 'story' | 'support_conversation', objectId);

  useEffect(() => {
    setActiveTab((current) => (tabs.includes(current) ? current : tabs[0]));
  }, [tabs]);

  useEffect(() => {
    if (!pickerOpen) {
      setQuery('');
      setStoryResults([]);
      setCRMResults([]);
      setDocResults([]);
      setConversationResults([]);
      setSearching(false);
      return;
    }

    const handle = window.setTimeout(async () => {
      if (activeTab === 'support') {
        setSearching(true);
        const response = await supportService.listConversations(workspaceId);
        const items = response.data?.data ?? [];
        const normalized = query.trim().toLowerCase();
        setConversationResults(
          items.filter((conversation) => {
            if (!normalized) return true;
            return (
              conversation.subject.toLowerCase().includes(normalized) ||
              conversation.display_id.toString().includes(normalized)
            );
          })
        );
        setSearching(false);
        return;
      }

      if (query.trim().length < 2) {
        setStoryResults([]);
        setCRMResults([]);
        setDocResults([]);
        return;
      }

      setSearching(true);
      if (activeTab === 'relationships' || activeTab === 'stories') {
        const response = await searchService.search(workspaceId, query.trim());
        setStoryResults((response.data?.stories ?? []).filter((story) => story.id !== objectId));
      } else if (activeTab === 'crm') {
        const response = await crmSearchService.search(workspaceId, query.trim());
        setCRMResults(response.data ?? []);
      } else if (activeTab === 'docs') {
        const response = await searchService.search(workspaceId, query.trim());
        setDocResults(response.data?.documents ?? []);
      }
      setSearching(false);
    }, 250);

    return () => window.clearTimeout(handle);
  }, [activeTab, objectId, pickerOpen, query, workspaceId]);

  const handleCreateStoryRelationship = async (payload: CreateStoryRelationshipRequest) => {
    await createRelationship.mutateAsync(payload);
    setPickerOpen(false);
  };

  const handleCreateGenericAssociation = async (toObjectType: CRMObjectType, toObjectID: string) => {
    await createAssociation.mutateAsync({
      workspace_id: workspaceId,
      from_object_type: objectType,
      from_object_id: objectId,
      to_object_type: toObjectType,
      to_object_id: toObjectID,
    });
    setPickerOpen(false);
  };

  const handleCreateDocLink = async (documentId: string) => {
    await createDocAssociation.mutateAsync({
      documentId,
      payload: {
        linked_object_type: objectType,
        linked_object_id: objectId,
        link_context: 'attached',
      },
    });
    setPickerOpen(false);
  };

  const isBusy =
    createRelationship.isPending ||
    deleteRelationship.isPending ||
    createAssociation.isPending ||
    deleteAssociation.isPending ||
    createDocAssociation.isPending ||
    deleteDocAssociation.isPending;

  return (
    <section className={cn('mt-6 rounded-xl border border-border/70 bg-background', className)}>
      <div className="flex items-center justify-between gap-3 border-b border-border/70 px-4 py-3">
        <div>
          <h3 className="text-sm font-semibold">Associations</h3>
          <p className="text-xs text-muted-foreground">
            {objectType === 'story' && !includeStoryRelationships
              ? 'Support traceability, CRM context, and linked docs in one place.'
              : 'Relationships, support traceability, CRM context, and linked docs in one place.'}
          </p>
        </div>
        {activeTab ? (
          <Popover open={pickerOpen} onOpenChange={setPickerOpen}>
            <PopoverTrigger asChild>
              <Button size="sm" variant="outline" className="h-8 gap-1.5 text-xs">
                <Link2 className="h-3.5 w-3.5" />
                Add
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" className="w-96 p-3">
              <div className="space-y-3">
                <div className="space-y-1">
                  <h4 className="text-sm font-semibold">Add {tabLabel(activeTab)}</h4>
                  <p className="text-xs text-muted-foreground">
                    Search existing records and connect them to this {objectType.replace('_', ' ')}.
                  </p>
                </div>

                {activeTab === 'relationships' ? (
                  <div className="flex flex-wrap gap-1">
                    {RELATIONSHIP_OPTIONS.map((option) => (
                      <Button
                        key={option.value}
                        type="button"
                        size="sm"
                        variant={relationshipType === option.value ? 'default' : 'outline'}
                        className="h-7 text-[11px]"
                        onClick={() => setRelationshipType(option.value)}
                      >
                        {option.label}
                      </Button>
                    ))}
                  </div>
                ) : null}

                <div className="relative">
                  <Search className="pointer-events-none absolute left-2.5 top-2 h-3.5 w-3.5 text-muted-foreground" />
                  <Input
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                    placeholder={
                      activeTab === 'support'
                        ? 'Filter conversations by subject or ID'
                        : activeTab === 'docs'
                          ? 'Search documents'
                          : activeTab === 'crm'
                            ? 'Search contacts, companies, or deals'
                            : 'Search stories by title or ID'
                    }
                    className="pl-8 text-sm"
                  />
                </div>

                <div className="max-h-72 space-y-1 overflow-y-auto pr-1">
                  {searching ? (
                    <div className="flex items-center gap-2 rounded-lg border border-dashed border-border/70 px-3 py-4 text-sm text-muted-foreground">
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Searching…
                    </div>
                  ) : null}

                  {!searching && activeTab === 'relationships' && storyResults.map((story) => (
                    <button
                      key={story.id}
                      type="button"
                      className="w-full rounded-lg border border-border/70 px-3 py-2 text-left transition hover:bg-accent"
                      onClick={() => handleCreateStoryRelationship({ relationship_type: relationshipType, other_story_id: story.id })}
                    >
                      <div className="flex items-center gap-2">
                        <GitBranch className="h-3.5 w-3.5 text-muted-foreground" />
                        <span className="text-sm font-medium">{story.name}</span>
                        {story.display_id ? (
                          <Badge variant="outline" className="h-5 px-1.5 text-[10px]">
                            TP-{story.display_id}
                          </Badge>
                        ) : null}
                      </div>
                    </button>
                  ))}

                  {!searching && activeTab === 'stories' && storyResults.map((story) => (
                    <button
                      key={story.id}
                      type="button"
                      className="w-full rounded-lg border border-border/70 px-3 py-2 text-left transition hover:bg-accent"
                      onClick={() => handleCreateGenericAssociation('story', story.id)}
                    >
                      <div className="flex items-center gap-2">
                        <ArrowRightLeft className="h-3.5 w-3.5 text-muted-foreground" />
                        <span className="text-sm font-medium">{story.name}</span>
                        {story.display_id ? (
                          <Badge variant="outline" className="h-5 px-1.5 text-[10px]">
                            TP-{story.display_id}
                          </Badge>
                        ) : null}
                      </div>
                    </button>
                  ))}

                  {!searching && activeTab === 'crm' && crmResults.map((result) => (
                    <button
                      key={`${result.type}-${result.id}`}
                      type="button"
                      className="w-full rounded-lg border border-border/70 px-3 py-2 text-left transition hover:bg-accent"
                      onClick={() => handleCreateGenericAssociation(result.type as CRMObjectType, result.id)}
                    >
                      <div className="flex items-center gap-2">
                        <Building2 className="h-3.5 w-3.5 text-muted-foreground" />
                        <span className="text-sm font-medium">{result.name}</span>
                      </div>
                      {result.detail ? (
                        <p className="mt-1 text-xs text-muted-foreground">{result.detail}</p>
                      ) : null}
                    </button>
                  ))}

                  {!searching && activeTab === 'support' && conversationResults.map((conversation) => (
                    <button
                      key={conversation.id}
                      type="button"
                      className="w-full rounded-lg border border-border/70 px-3 py-2 text-left transition hover:bg-accent"
                      onClick={() => handleCreateGenericAssociation('support_conversation', conversation.id)}
                    >
                      <div className="flex items-center gap-2">
                        <MessageSquareText className="h-3.5 w-3.5 text-muted-foreground" />
                        <span className="text-sm font-medium">{conversation.subject}</span>
                        <Badge variant="outline" className="h-5 px-1.5 text-[10px]">
                          C-{conversation.display_id}
                        </Badge>
                      </div>
                    </button>
                  ))}

                  {!searching && activeTab === 'docs' && docResults.map((doc) => (
                    <button
                      key={doc.id}
                      type="button"
                      className="w-full rounded-lg border border-border/70 px-3 py-2 text-left transition hover:bg-accent"
                      onClick={() => handleCreateDocLink(doc.id)}
                    >
                      <div className="flex items-center gap-2">
                        <FileText className="h-3.5 w-3.5 text-muted-foreground" />
                        <span className="text-sm font-medium">{doc.name}</span>
                      </div>
                    </button>
                  ))}

                  {!searching &&
                  ((activeTab === 'relationships' && query.trim().length >= 2 && storyResults.length === 0) ||
                    (activeTab === 'stories' && query.trim().length >= 2 && storyResults.length === 0) ||
                    (activeTab === 'crm' && query.trim().length >= 2 && crmResults.length === 0) ||
                    (activeTab === 'docs' && query.trim().length >= 2 && docResults.length === 0) ||
                    (activeTab === 'support' && conversationResults.length === 0)) ? (
                    <div className="rounded-lg border border-dashed border-border/70 px-3 py-4 text-sm text-muted-foreground">
                      No results found.
                    </div>
                  ) : null}
                </div>
              </div>
            </PopoverContent>
          </Popover>
        ) : null}
      </div>

      <div className="space-y-4 p-4">
        <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as AssociationsTab)}>
          <TabsList className="h-8 flex-wrap bg-muted/60">
            {tabs.map((tab) => (
              <TabsTrigger key={tab} value={tab} className="h-7 px-3 text-xs">
                {tabLabel(tab)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        {associationsQuery.isLoading ? (
          <div className="flex items-center gap-2 rounded-lg border border-dashed border-border/70 px-3 py-4 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            Loading associations…
          </div>
        ) : null}

        {associationsQuery.error ? (
          <div className="rounded-lg border border-destructive/20 bg-destructive/5 px-3 py-3 text-sm text-destructive">
            {(associationsQuery.error as Error).message}
          </div>
        ) : null}

        {!associationsQuery.isLoading && !associationsQuery.error ? (
          <>
            {activeTab === 'relationships' && data ? (
              <div className="space-y-5">
                <RelationshipGroup
                  title="Blocked By"
                  description="Inbound blockers. Completed blockers stay visible here as history."
                  items={data.story_relationships.blocked_by}
                  onRemove={(relationshipId) => deleteRelationship.mutate(relationshipId)}
                />
                <RelationshipGroup
                  title="Blocking"
                  description="Stories that depend on this story."
                  items={data.story_relationships.blocking}
                  onRemove={(relationshipId) => deleteRelationship.mutate(relationshipId)}
                />
                <RelationshipGroup
                  title="Relates To"
                  description="Loose related work without workflow impact."
                  items={data.story_relationships.relates_to}
                  onRemove={(relationshipId) => deleteRelationship.mutate(relationshipId)}
                />
                <RelationshipGroup
                  title="Related By"
                  description="Stories that reference this one."
                  items={data.story_relationships.related_by}
                  onRemove={(relationshipId) => deleteRelationship.mutate(relationshipId)}
                />
                <RelationshipGroup
                  title="Duplicates"
                  description="Stories this story supersedes or duplicates."
                  items={data.story_relationships.duplicates}
                  onRemove={(relationshipId) => deleteRelationship.mutate(relationshipId)}
                />
                <RelationshipGroup
                  title="Is Duplicated By"
                  description="Stories that supersede or duplicate this one."
                  items={data.story_relationships.duplicated_by}
                  onRemove={(relationshipId) => deleteRelationship.mutate(relationshipId)}
                />
              </div>
            ) : null}

            {activeTab === 'stories' && data ? (
              <AssociationList
                emptyLabel="No linked stories"
                items={data.stories}
                onRemove={(associationId) => deleteAssociation.mutate(associationId)}
              />
            ) : null}

            {activeTab === 'support' && data ? (
              <AssociationList
                emptyLabel="No linked support conversations"
                items={data.support_conversations}
                onRemove={(associationId) => deleteAssociation.mutate(associationId)}
              />
            ) : null}

            {activeTab === 'crm' && data ? (
              <AssociationList
                emptyLabel="No linked CRM records"
                items={data.crm_records}
                onRemove={(associationId) => deleteAssociation.mutate(associationId)}
              />
            ) : null}

            {activeTab === 'docs' && data ? (
              <AssociationList
                emptyLabel="No linked docs"
                items={data.docs}
                onRemove={(associationId) => deleteDocAssociation.mutate(associationId)}
              />
            ) : null}
          </>
        ) : null}

        {isBusy ? (
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
            Updating associations…
          </div>
        ) : null}
      </div>
    </section>
  );
}

function AssociationList({
  items,
  emptyLabel,
  onRemove,
}: {
  items: AssociationObjectSummary[];
  emptyLabel: string;
  onRemove: (associationId: string) => void;
}) {
  if (items.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-border/70 px-3 py-4 text-sm text-muted-foreground">
        {emptyLabel}
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {items.map((item) => (
        <ObjectChip
          key={`${item.object_type}-${item.object_id}-${item.association_id ?? 'fallback'}`}
          item={item}
          onRemove={item.association_id ? () => onRemove(item.association_id!) : undefined}
        />
      ))}
    </div>
  );
}
