import { type ReactNode, type RefObject, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import {
  ArrowRightLeft,
  Check,
  Copy,
  FileText,
  Loader2,
  MoreHorizontal,
  Plus,
  Search,
  ShieldAlert,
  Trash2,
  TriangleAlert,
} from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
  useCreateStory,
  useCreateStoryRelationship,
  useDeleteDocAssociation,
  useDeleteStoryRelationship,
  useStoryAssociations,
} from '@/hooks/queries';
import { searchService, type SearchResult } from '@/lib/services/searchService';
import type {
  CreateStoryRequest,
  GroupedAssociations,
  StoryRelationshipAction,
} from '@/lib/pmTypes';
import { StoryTypeIcon } from '@/lib/pmConstants';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';

interface StoryRelationshipsSectionProps {
  workspaceId: string;
  storyId: string;
  storyName: string;
  storyDisplayId: number;
  workflowId?: string;
  workflowStateId?: string;
  epicId?: string;
  sprintId?: string;
  teamId?: string;
  storyType?: 'feature' | 'bug' | 'chore';
  priority?: 'none' | 'low' | 'medium' | 'high' | 'urgent';
  severity?: 'none' | 'minor' | 'major' | 'critical';
  externalBlocker: string;
  onExternalBlockerChange: (value: string) => void;
  composerOpen: boolean;
  onComposerOpenChange: (open: boolean) => void;
  /** Ref to an external trigger (e.g. the action-bar "Relationships" button).
   *  When set, clicking that element opens the popover anchored there instead of inline. */
  externalTriggerRef?: RefObject<HTMLElement | null>;
  className?: string;
}

const RELATIONSHIP_OPTIONS: Array<{
  value: StoryRelationshipAction;
  label: string;
  icon: typeof ArrowRightLeft;
}> = [
  { value: 'relates_to', label: 'relates to', icon: ArrowRightLeft },
  { value: 'blocks', label: 'blocks', icon: TriangleAlert },
  { value: 'is_blocked_by', label: 'is blocked by', icon: ShieldAlert },
  { value: 'duplicates', label: 'duplicates', icon: Copy },
  { value: 'is_duplicated_by', label: 'is duplicated by', icon: Copy },
];

const UPDATE_TYPE_OPTIONS: Array<{
  value: StoryRelationshipAction;
  label: string;
  icon: typeof ArrowRightLeft;
}> = [
  { value: 'blocks', label: 'Blocks', icon: TriangleAlert },
  { value: 'is_blocked_by', label: 'Blocked by', icon: ShieldAlert },
  { value: 'duplicates', label: 'Duplicates', icon: Copy },
  { value: 'is_duplicated_by', label: 'Duplicated by', icon: Copy },
  { value: 'relates_to', label: 'Relates to', icon: ArrowRightLeft },
];

function getRelationshipMeta(linkType: string) {
  switch (linkType) {
    case 'blocks':
      return { label: 'Blocks', icon: TriangleAlert, color: 'text-amber-500' };
    case 'is_blocked_by':
      return { label: 'Blocked by', icon: ShieldAlert, color: 'text-red-500' };
    case 'relates_to':
    case 'related_by':
      return { label: 'Relates to', icon: ArrowRightLeft, color: 'text-blue-500' };
    case 'duplicates':
      return { label: 'Duplicates', icon: Copy, color: 'text-violet-500' };
    case 'is_duplicated_by':
      return { label: 'Duplicated by', icon: Copy, color: 'text-violet-500' };
    default:
      return { label: linkType, icon: ArrowRightLeft, color: 'text-muted-foreground' };
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

  const rect = anchorEl.getBoundingClientRect();

  return (
    <div
      ref={popoverRef}
      className="fixed z-50 w-[min(28rem,calc(100vw-2rem))] rounded-xl border bg-popover text-popover-foreground shadow-md animate-in fade-in-0 zoom-in-95 slide-in-from-top-2"
      style={{
        top: rect.bottom + 4,
        left: rect.left,
      }}
    >
      {children}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Main component                                                    */
/* ------------------------------------------------------------------ */

export function StoryRelationshipsSection({
  workspaceId,
  storyId,
  storyName: _storyName,
  storyDisplayId: _storyDisplayId,
  workflowId,
  workflowStateId,
  epicId,
  sprintId,
  teamId,
  storyType,
  priority,
  severity,
  externalBlocker,
  onExternalBlockerChange,
  composerOpen,
  onComposerOpenChange,
  externalTriggerRef,
  className,
}: StoryRelationshipsSectionProps) {
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const inlineAddRef = useRef<HTMLButtonElement>(null);
  const [anchorSource, setAnchorSource] = useState<'external' | 'inline'>('inline');
  const [popoverTab, setPopoverTab] = useState<'stories' | 'docs'>('stories');
  const [query, setQuery] = useState('');
  const [relationshipType, setRelationshipType] = useState<StoryRelationshipAction>('relates_to');
  const [searching, setSearching] = useState(false);
  const [storyResults, setStoryResults] = useState<SearchResult[]>([]);
  const [docResults, setDocResults] = useState<SearchResult[]>([]);

  const associationsQuery = useStoryAssociations(workspaceId, storyId);
  const data = associationsQuery.data as GroupedAssociations | undefined;
  const createRelationship = useCreateStoryRelationship(workspaceId, storyId);
  const deleteRelationship = useDeleteStoryRelationship(workspaceId, storyId);
  const createStory = useCreateStory(workspaceId);
  const createDocAssociation = useCreateDocAssociation(workspaceId, 'story', storyId);
  const deleteDocAssociation = useDeleteDocAssociation(workspaceId, 'story', storyId);

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
      setStoryResults([]);
      setDocResults([]);
      return;
    }

    const handle = window.setTimeout(async () => {
      if (query.trim().length < 2) {
        setStoryResults([]);
        setDocResults([]);
        setSearching(false);
        return;
      }

      setSearching(true);
      const response = await searchService.search(workspaceId, query.trim());
      if (popoverTab === 'stories') {
        setStoryResults((response.data?.stories ?? []).filter((story) => story.id !== storyId));
      } else {
        setDocResults(response.data?.documents ?? []);
      }
      setSearching(false);
    }, 220);

    return () => window.clearTimeout(handle);
  }, [composerOpen, query, storyId, workspaceId, popoverTab]);

  const allRelationships = useMemo(() => {
    if (!data?.story_relationships) return [];
    const rels = data.story_relationships;
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
  const hasContent = allRelationships.length > 0 || linkedDocs.length > 0 || !!externalBlocker;

  const handleOpenStory = (targetStoryId: string) => {
    if (!workspace?.slug) return;
    navigate({
      to: '/w/$slug/pm/stories/$storyId',
      params: { slug: workspace.slug, storyId: targetStoryId },
    });
  };

  const handleCreateRelationship = async (otherStoryId: string) => {
    await createRelationship.mutateAsync({
      relationship_type: relationshipType,
      other_story_id: otherStoryId,
    });
    onComposerOpenChange(false);
  };

  const handleCreateRelatedStory = async () => {
    const name = query.trim();
    if (!name) return;

    const payload: CreateStoryRequest = {
      workspace_id: workspaceId,
      name,
      workflow_id: workflowId,
      workflow_state_id: workflowStateId,
      epic_id: epicId,
      sprint_id: sprintId,
      team_id: teamId,
      story_type: storyType,
      priority,
      severity,
    };

    const created = await createStory.mutateAsync(payload);
    await createRelationship.mutateAsync({
      relationship_type: relationshipType,
      other_story_id: created.story.id,
    });
    onComposerOpenChange(false);
  };

  const handleLinkDoc = async (documentId: string) => {
    await createDocAssociation.mutateAsync({
      documentId,
      payload: {
        linked_object_type: 'story',
        linked_object_id: storyId,
        link_context: 'attached',
      },
    });
    onComposerOpenChange(false);
  };

  const handleUpdateRelationshipType = async (
    relationshipId: string,
    otherStoryId: string,
    newType: StoryRelationshipAction,
  ) => {
    await deleteRelationship.mutateAsync(relationshipId);
    await createRelationship.mutateAsync({
      relationship_type: newType,
      other_story_id: otherStoryId,
    });
  };

  const busy =
    createRelationship.isPending ||
    deleteRelationship.isPending ||
    createStory.isPending ||
    createDocAssociation.isPending ||
    deleteDocAssociation.isPending;

  // Resolve the popover anchor element
  const anchorEl =
    anchorSource === 'external' && externalTriggerRef?.current
      ? externalTriggerRef.current
      : inlineAddRef.current;

  const popoverBody: ReactNode = (
    <>
      <div className="border-b border-border/70 px-4 pt-3 pb-0">
        <Tabs
          value={popoverTab}
          onValueChange={(v) => {
            setPopoverTab(v as 'stories' | 'docs');
            setQuery('');
            setStoryResults([]);
            setDocResults([]);
          }}
        >
          <TabsList className="h-8 bg-transparent p-0">
            <TabsTrigger
              value="stories"
              className="h-7 rounded-none border-b-2 border-transparent px-3 text-xs data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none"
            >
              Stories
            </TabsTrigger>
            <TabsTrigger
              value="docs"
              className="h-7 rounded-none border-b-2 border-transparent px-3 text-xs data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:shadow-none"
            >
              Docs
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      <div className="space-y-3 px-4 py-3">
        {popoverTab === 'stories' ? (
          <>
            <p className="text-sm font-medium">This Story...</p>
            <div className="flex flex-wrap gap-1.5">
              {RELATIONSHIP_OPTIONS.map((option) => {
                const OptionIcon = option.icon;
                return (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() => setRelationshipType(option.value)}
                    className={cn(
                      'inline-flex h-9 items-center gap-1.5 rounded-lg border px-2.5 text-xs transition-colors',
                      relationshipType === option.value
                        ? 'border-primary bg-primary/10 text-primary font-medium'
                        : 'border-border/70 bg-background text-muted-foreground hover:bg-accent',
                    )}
                  >
                    <OptionIcon className="h-3.5 w-3.5" />
                    {option.label}
                  </button>
                );
              })}
            </div>
          </>
        ) : null}

        <div className="relative">
          <Search className="pointer-events-none absolute left-2.5 top-2.5 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={popoverTab === 'stories' ? 'Search Story Title or ID' : 'Search documents'}
            className="h-9 rounded-lg pl-8 text-sm"
          />
        </div>

        <div className="max-h-56 space-y-1 overflow-y-auto">
          {searching ? (
            <div className="flex items-center gap-2 rounded-lg border border-dashed border-border/70 px-3 py-3 text-sm text-muted-foreground">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              Searching...
            </div>
          ) : null}

          {!searching &&
            popoverTab === 'stories' &&
            storyResults.map((story) => (
              <button
                key={story.id}
                type="button"
                onClick={() => handleCreateRelationship(story.id)}
                className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left transition hover:bg-accent/50"
              >
                <span className="min-w-0 flex-1 truncate text-sm font-medium">{story.name}</span>
                {story.display_id ? (
                  <Badge variant="outline" className="h-5 shrink-0 rounded-full px-1.5 text-[10px]">
                    TP-{story.display_id}
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
                className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left transition hover:bg-accent/50"
              >
                <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span className="min-w-0 flex-1 truncate text-sm font-medium">{doc.name}</span>
              </button>
            ))}

          {!searching &&
            query.trim().length >= 2 &&
            ((popoverTab === 'stories' && storyResults.length === 0) ||
              (popoverTab === 'docs' && docResults.length === 0)) ? (
            <div className="rounded-lg border border-dashed border-border/70 px-3 py-3 text-sm text-muted-foreground">
              No results found.
            </div>
          ) : null}
        </div>

        {popoverTab === 'stories' ? (
          <div className="flex items-center justify-between gap-3 border-t border-border/70 pt-3">
            <span className="text-xs text-muted-foreground">Search for an existing story or</span>
            <Button
              type="button"
              variant="outline"
              className="h-7 rounded-lg px-2.5 text-xs"
              disabled={query.trim().length === 0}
              onClick={handleCreateRelatedStory}
            >
              Create Related Story
            </Button>
          </div>
        ) : null}
      </div>
    </>
  );

  // Hide entire section when no relationships and composer is closed
  if (!hasContent && !composerOpen && !associationsQuery.isLoading) {
    return null;
  }

  return (
    <section id="story-relationships-section" className={cn('mt-6', className)}>
      <h3 className="text-sm font-semibold">Story Relationships</h3>

      {associationsQuery.error ? (
        <div className="mt-3 rounded-lg border border-destructive/20 bg-destructive/5 px-3 py-2.5 text-sm text-destructive">
          {(associationsQuery.error as Error).message}
        </div>
      ) : null}

      {/* Flat relationship list — no borders */}
      <div className="mt-1">
        {allRelationships.map((item) => {
          const meta = getRelationshipMeta(item.link_type);
          const Icon = meta.icon;
          const resolved = item.link_type === 'blocks' && !item.is_active;
          return (
            <div
              key={item.relationship_id}
              className={cn(
                'group flex items-center gap-1 py-0.5 transition-colors',
                resolved && 'opacity-60',
              )}
            >
              <div className="flex min-w-0 flex-1 items-center gap-1">
                <Icon className={cn('h-3.5 w-3.5 shrink-0', meta.color)} />
                <span className="shrink-0 text-xs text-muted-foreground">{meta.label}</span>
                <button
                  type="button"
                  onClick={() => handleOpenStory(item.story.object_id)}
                  className="min-w-0 truncate text-sm font-medium hover:underline text-left"
                >
                  {item.story.title}
                </button>
              </div>
              <div className="flex shrink-0 items-center gap-1.5">
                {item.story.display_id ? (
                  <Badge variant="outline" className="h-5 rounded-full px-1.5 text-[10px] font-medium gap-1">
                    {item.story.story_type ? (
                      <StoryTypeIcon storyType={item.story.story_type} className="h-3 w-3" />
                    ) : null}
                    {item.story.display_id}
                    {(item.story.completed || resolved) ? (
                      <Check className="h-3 w-3 text-green-600" />
                    ) : null}
                  </Badge>
                ) : null}
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      className="h-6 w-6 shrink-0 rounded-md flex items-center justify-center opacity-0 transition-opacity hover:bg-accent group-hover:opacity-100"
                    >
                      <MoreHorizontal className="h-3.5 w-3.5 text-muted-foreground" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-48">
                    <DropdownMenuLabel className="text-xs">Update Relationship Type</DropdownMenuLabel>
                    {UPDATE_TYPE_OPTIONS.map((opt) => {
                      const OptIcon = opt.icon;
                      return (
                        <DropdownMenuItem
                          key={opt.value}
                          onClick={() => handleUpdateRelationshipType(item.relationship_id, item.story.object_id, opt.value)}
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
                      <Trash2 className="mr-2 h-3.5 w-3.5" />
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
            className="group flex items-center gap-2 py-1.5 transition-colors"
          >
            <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <span className="text-xs font-medium text-muted-foreground">Doc</span>
            <span className="min-w-0 flex-1 truncate text-sm font-medium">{doc.title}</span>
            {doc.association_id ? (
              <div className="ml-auto flex shrink-0 items-center">
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      className="h-6 w-6 shrink-0 rounded-md flex items-center justify-center opacity-0 transition-opacity hover:bg-accent group-hover:opacity-100"
                    >
                      <MoreHorizontal className="h-3.5 w-3.5 text-muted-foreground" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-48">
                    <DropdownMenuItem
                      className="text-destructive focus:text-destructive"
                      onClick={() => deleteDocAssociation.mutate(doc.association_id!)}
                    >
                      <Trash2 className="mr-2 h-3.5 w-3.5" />
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
      <div className="mt-1 flex items-center gap-2">
        <Button
          ref={inlineAddRef}
          type="button"
          variant="outline"
          size="sm"
          className="h-7 gap-1 text-xs"
          onClick={() => {
            setAnchorSource('inline');
            onComposerOpenChange(true);
          }}
        >
          <Plus className="h-3.5 w-3.5" />
          Add Relationship
        </Button>

        {busy ? (
          <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
            <Loader2 className="h-3 w-3 animate-spin" />
          </span>
        ) : null}
      </div>

      {/* Floating popover — anchored to whichever trigger was clicked */}
      <FloatingPopover open={composerOpen} onOpenChange={onComposerOpenChange} anchorEl={anchorEl}>
        {popoverBody}
      </FloatingPopover>

      {/* External blocker */}
      {externalBlocker || allRelationships.some((r) => r.link_type === 'is_blocked_by') ? (
        <div className="mt-4 space-y-1.5">
          <div className="flex items-center gap-1.5">
            <ShieldAlert className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">External blocker</span>
          </div>
          <textarea
            value={externalBlocker}
            onChange={(event) => onExternalBlockerChange(event.target.value)}
            placeholder="Customer dependency, vendor issue, legal review..."
            className="min-h-[60px] w-full rounded-lg border border-border bg-background px-3 py-2 text-sm outline-none transition-colors focus:border-primary/40"
          />
        </div>
      ) : null}
    </section>
  );
}
