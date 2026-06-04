import { useMemo, useState, type Dispatch, type SetStateAction } from 'react';
import { Collapsible } from 'radix-ui';
import { ArrowRight01Icon, HelpCircleIcon, MoreVerticalIcon, FolderOpenIcon, InboxIcon, PlusSignIcon, PencilEdit01Icon, Delete01Icon } from '@/lib/icons';
import { ICON_MAP } from '@/components/ui/icon-picker';
import { useDocsCollections, useDocsDocuments, useDocsSpaces, useDeleteDocsSpace, useDeleteDocsCollection } from '@/hooks/queries';
import type { DocsCollection, DocsSpace } from '@/lib/docsTypes';
import { CreateCollectionDialog } from '@/components/docs/CreateCollectionDialog';
import { buildCollectionTreeOptions } from '@/components/docs/CollectionTreePicker';
import { useTruncationDetection } from '@/hooks/useTruncationDetection';
import { SpaceDialog } from '@/components/docs/SpaceDialog';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { DeleteCollectionDialog } from '@/components/docs/DeleteCollectionDialog';
import { DeleteSpaceDialog } from '@/components/docs/DeleteSpaceDialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from '@/components/ui/sidebar';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { SidebarSectionAction } from './SidebarSectionAction';

function SidebarCollectionIcon({ name }: { name?: string | null }) {
  if (name) {
    const Icon = ICON_MAP[name];
    if (Icon) {
      return <Icon className="h-3.5 w-3.5" />;
    }
  }

  return <FolderOpenIcon className="h-3.5 w-3.5" />;
}

function DocsSpaceCollections({
  wsId,
  spaceId,
  wsSlug,
  isActive,
  openCreate,
  onNavigate,
  onEditCollection,
  onDeleteCollection,
}: {
  wsId: string;
  spaceId: string;
  wsSlug: string;
  isActive: (link: string) => boolean;
  openCreate: (modal: 'docs_collection', options?: { spaceId?: string }) => void;
  onNavigate: (args: { to: string; params?: Record<string, string>; search?: Record<string, string> }) => void;
  onEditCollection: (collection: DocsCollection) => void;
  onDeleteCollection: (collection: DocsCollection) => void;
}) {
  const { data: collections } = useDocsCollections(wsId, spaceId);
  const { data: documents } = useDocsDocuments(wsId, { space_id: spaceId });
  const uncollectedCount = (documents ?? []).filter((document) => !document.collection_id).length;
  const uncollectedLink = `/w/${wsSlug}/docs/spaces/${spaceId}?collection=__uncollected__`;
  const { checkRef: checkColTruncation, isTruncated: isColTruncated } = useTruncationDetection();

  // Fold the flat collection list into tree order so children render
  // directly under their parent, and pass depth through to each row
  // for depth-based indentation. Memoised on (spaceId, collections)
  // so rerenders caused by unrelated sidebar state don't rebuild
  // the tree options on every pass.
  const treeOptions = useMemo(
    () => buildCollectionTreeOptions(spaceId, collections ?? []),
    [spaceId, collections],
  );
  const collectionById = useMemo(
    () => new Map((collections ?? []).map((c) => [c.id, c])),
    [collections],
  );

  // Track which parent collections are expanded in the sidebar tree.
  const [expandedColls, setExpandedColls] = useState<Set<string>>(new Set());
  const toggleColl = (id: string) => {
    setExpandedColls((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  // Enrich each option with a hasChildren flag (next item in DFS is deeper).
  // Filter to only show items whose parent is expanded.
  const visibleOptions = useMemo(() => {
    const enriched = treeOptions.map((opt, i) => ({
      ...opt,
      hasChildren: i + 1 < treeOptions.length && treeOptions[i + 1].depth > opt.depth,
    }));
    const result: (typeof enriched)[number][] = [];
    // skipUntilDepth: when a collapsed parent is hit, hide all deeper
    // items until we return to the same depth or shallower.
    let skipUntilDepth = Infinity;
    for (const opt of enriched) {
      if (opt.depth > skipUntilDepth) continue;
      skipUntilDepth = Infinity;
      result.push(opt);
      if (opt.hasChildren && !expandedColls.has(opt.id)) {
        skipUntilDepth = opt.depth;
      }
    }
    return result;
  }, [treeOptions, expandedColls]);

  return (
    <SidebarMenuSub className="mr-0 pr-0">
      {visibleOptions.map((option) => {
        const collection = collectionById.get(option.id);
        if (!collection) return null;
        const link = `/w/${wsSlug}/docs/spaces/${spaceId}?collection=${collection.id}`;
        const showTooltip = isColTruncated(collection.id);
        const depthStyle = option.depth > 0 ? { paddingLeft: `${option.depth * 12}px` } : undefined;

        return (
          <SidebarMenuSubItem key={collection.id} style={depthStyle}>
            <div className="group/collection relative flex items-center">
              <Tooltip open={showTooltip ? undefined : false}>
                <TooltipTrigger asChild>
                  <SidebarMenuSubButton
                    asChild
                    size="sm"
                    isActive={isActive(link)}
                  >
                    <a
                      href={link}
                      onClick={(event) => {
                        event.preventDefault();
                        if (option.hasChildren) {
                          toggleColl(collection.id);
                        }
                        onNavigate({
                          to: '/w/$slug/docs/spaces/$spaceId',
                          params: { slug: wsSlug, spaceId },
                          search: { collection: collection.id },
                        });
                      }}
                    >
                      <ArrowRight01Icon className={`h-3 w-3 shrink-0 transition-transform ${option.hasChildren ? `text-muted-foreground ${expandedColls.has(collection.id) ? 'rotate-90' : ''}` : 'invisible'}`} />
                      <SidebarCollectionIcon name={collection.icon} />
                      <span className="truncate" ref={(element) => checkColTruncation(collection.id, element)}>
                        {collection.name}
                      </span>
                    </a>
                  </SidebarMenuSubButton>
                </TooltipTrigger>
                <TooltipContent side="right" align="center">
                  {collection.name}
                </TooltipContent>
              </Tooltip>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button
                    type="button"
                    className="absolute inset-y-0 right-0 flex w-9 items-center justify-end pr-1.5 opacity-0 transition-opacity bg-gradient-to-l from-sidebar from-65% to-transparent group-hover/collection:opacity-100 data-[state=open]:opacity-100"
                  >
                    <span className="flex h-5 w-5 items-center justify-center rounded hover:bg-muted transition-colors">
                      <MoreVerticalIcon className="h-3.5 w-3.5 text-foreground/80" />
                    </span>
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent side="right" align="start">
                  <DropdownMenuItem onClick={() => onEditCollection(collection)}>
                    <PencilEdit01Icon className="h-4 w-4" />
                    Edit collection
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={() => onDeleteCollection(collection)}
                    className="text-destructive focus:text-destructive"
                  >
                    <Delete01Icon className="h-4 w-4" />
                    Delete collection
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </SidebarMenuSubItem>
        );
      })}
      {uncollectedCount > 0 && (
        <SidebarMenuSubItem>
          <SidebarMenuSubButton
            asChild
            size="sm"
            isActive={isActive(uncollectedLink)}
          >
            <a
              href={uncollectedLink}
              onClick={(event) => {
                event.preventDefault();
                onNavigate({
                  to: '/w/$slug/docs/spaces/$spaceId',
                  params: { slug: wsSlug, spaceId },
                  search: { collection: '__uncollected__' },
                });
              }}
            >
              <InboxIcon className="h-3.5 w-3.5" />
              <span className="truncate">Uncategorized</span>
            </a>
          </SidebarMenuSubButton>
        </SidebarMenuSubItem>
      )}
      <SidebarMenuSubItem>
        <SidebarMenuSubButton
          size="sm"
          className="cursor-pointer text-muted-foreground/70 hover:text-foreground"
          onClick={() => openCreate('docs_collection', { spaceId })}
        >
          <PlusSignIcon className="h-3.5 w-3.5" />
          <span>Add collection</span>
        </SidebarMenuSubButton>
      </SidebarMenuSubItem>
    </SidebarMenuSub>
  );
}

type DocsSpacesNavProps = {
  wsId: string;
  wsSlug: string;
  expandedTeams: Set<string>;
  setExpandedTeams: Dispatch<SetStateAction<Set<string>>>;
  isActive: (link: string) => boolean;
  openCreate: (modal: 'docs_collection', options?: { spaceId?: string }) => void;
  onNavigate: (args: { to: string; params?: Record<string, string>; search?: Record<string, string> }) => void;
};

export function DocsSpacesNav({
  wsId,
  wsSlug,
  expandedTeams,
  setExpandedTeams,
  isActive,
  openCreate,
  onNavigate,
}: DocsSpacesNavProps) {
  const { data: spaces } = useDocsSpaces(wsId);
  const deleteSpace = useDeleteDocsSpace(wsId);
  const deleteCollection = useDeleteDocsCollection(wsId);
  const [editingSpace, setEditingSpace] = useState<DocsSpace | null>(null);
  const [showCreateSpace, setShowCreateSpace] = useState(false);
  const [createSpaceType, setCreateSpaceType] = useState<'internal' | 'external_capable'>('internal');
  const [deletingSpace, setDeletingSpace] = useState<DocsSpace | null>(null);
  const [editingCollection, setEditingCollection] = useState<DocsCollection | null>(null);
  const [deletingCollection, setDeletingCollection] = useState<DocsCollection | null>(null);
  const { checkRef: checkSpaceTruncation, isTruncated: isSpaceTruncated } = useTruncationDetection();

  const toggleDocSpace = (spaceKey: string) => {
    setExpandedTeams((previous) => {
      const next = new Set(previous);
      for (const key of previous) {
        if (key.startsWith('docs_space_') && key !== spaceKey) {
          next.delete(key);
        }
      }

      if (next.has(spaceKey)) {
        next.delete(spaceKey);
      } else {
        next.add(spaceKey);
      }

      return next;
    });
  };

  if (!spaces || spaces.length === 0) {
    return null;
  }

  const internalSpaces = spaces.filter((space) => space.type === 'internal');
  const externalSpaces = spaces.filter((space) => space.type === 'external_capable');

  const renderSpaceItem = (space: (typeof spaces)[number]) => {
    const spaceKey = `docs_space_${space.id}`;
    const isExpanded = expandedTeams.has(spaceKey);
    const spaceLink = `/w/${wsSlug}/docs/spaces/${space.id}`;
    const showTooltip = isSpaceTruncated(space.id);

    return (
      <Collapsible.Root
        key={space.id}
        asChild
        open={isExpanded}
      >
        <SidebarMenuItem>
          <div className="group/space relative flex items-center">
            <Tooltip open={showTooltip ? undefined : false}>
              <TooltipTrigger asChild>
                <SidebarMenuButton
                  className="h-8 flex-1 rounded-md px-2 cursor-pointer"
                  isActive={isActive(spaceLink)}
                  onClick={() => {
                    toggleDocSpace(spaceKey);
                    onNavigate({
                      to: '/w/$slug/docs/spaces/$spaceId',
                      params: { slug: wsSlug, spaceId: space.id },
                    });
                  }}
                >
                  <ArrowRight01Icon className={`h-3.5 w-3.5 shrink-0 transition-transform ${isExpanded ? 'rotate-90' : ''}`} />
                  <span className="truncate" ref={(element) => checkSpaceTruncation(space.id, element)}>
                    {space.name}
                  </span>
                </SidebarMenuButton>
              </TooltipTrigger>
              <TooltipContent side="right" align="center">
                {space.name}
              </TooltipContent>
            </Tooltip>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button
                  type="button"
                  className="absolute inset-y-0 right-0 flex w-9 items-center justify-end pr-1.5 opacity-0 transition-opacity bg-gradient-to-l from-sidebar from-65% to-transparent group-hover/space:opacity-100 data-[state=open]:opacity-100"
                >
                  <span className="flex h-5 w-5 items-center justify-center rounded hover:bg-muted transition-colors group/dots">
                    <MoreVerticalIcon className="h-3.5 w-3.5 text-foreground/70 group-hover/dots:text-foreground" />
                  </span>
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="right" align="start">
                <DropdownMenuItem onClick={() => setEditingSpace(space)}>
                  <PencilEdit01Icon className="h-4 w-4" />
                  Edit space
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => setDeletingSpace(space)}
                  className="text-destructive focus:text-destructive"
                >
                  <Delete01Icon className="h-4 w-4" />
                  Delete space
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
          <Collapsible.Content>
            <DocsSpaceCollections
              wsId={wsId}
              spaceId={space.id}
              wsSlug={wsSlug}
              isActive={isActive}
              openCreate={openCreate}
              onNavigate={onNavigate}
              onEditCollection={setEditingCollection}
              onDeleteCollection={setDeletingCollection}
            />
          </Collapsible.Content>
        </SidebarMenuItem>
      </Collapsible.Root>
    );
  };

  return (
    <>
      {internalSpaces.length > 0 && (
        <SidebarGroup className="p-0 pb-3">
          <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90 flex items-center justify-between">
            <span>Team Spaces</span>
            <SidebarSectionAction
              label="Create space"
              onClick={() => { setShowCreateSpace(false); setTimeout(() => { setCreateSpaceType('internal'); setShowCreateSpace(true); }, 0); }}
            />
          </SidebarGroupLabel>
          <SidebarMenu>
            {internalSpaces.map(renderSpaceItem)}
          </SidebarMenu>
        </SidebarGroup>
      )}
      {externalSpaces.length > 0 && (
        <SidebarGroup className="p-0 pb-3">
          <SidebarGroupLabel className="flex h-7 items-center px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90 justify-between">
            <span className="flex items-center gap-1">
              External Spaces
              <QuickTooltip label="Published to your public help center" side="right">
                <span className="inline-flex h-3 w-3 cursor-help items-center justify-center rounded text-muted-foreground/50 hover:text-muted-foreground">
                  <HelpCircleIcon className="h-[10px] w-[10px]" />
                </span>
              </QuickTooltip>
            </span>
            <SidebarSectionAction
              label="Create space"
              onClick={() => { setShowCreateSpace(false); setTimeout(() => { setCreateSpaceType('external_capable'); setShowCreateSpace(true); }, 0); }}
            />
          </SidebarGroupLabel>
          <SidebarMenu>
            {externalSpaces.map(renderSpaceItem)}
          </SidebarMenu>
        </SidebarGroup>
      )}

      <SpaceDialog
        wsId={wsId}
        open={editingSpace !== null}
        onOpenChange={(open) => {
          if (!open) {
            setTimeout(() => setEditingSpace(null), 150);
          }
        }}
        space={editingSpace}
      />

      <SpaceDialog
        wsId={wsId}
        open={showCreateSpace}
        onOpenChange={setShowCreateSpace}
        defaultType={createSpaceType}
      />

      <DeleteSpaceDialog
        wsId={wsId}
        space={deletingSpace}
        open={deletingSpace !== null}
        onOpenChange={(open) => {
          if (!open) {
            setTimeout(() => setDeletingSpace(null), 150);
          }
        }}
        onConfirm={async () => {
          if (!deletingSpace) return;
          await deleteSpace.mutateAsync(deletingSpace.id);
          setDeletingSpace(null);
          onNavigate({ to: '/w/$slug/docs', params: { slug: wsSlug } });
        }}
      />

      <CreateCollectionDialog
        wsId={wsId}
        open={editingCollection !== null}
        onOpenChange={(open) => {
          if (!open) {
            setTimeout(() => setEditingCollection(null), 150);
          }
        }}
        collection={editingCollection}
      />

      <DeleteCollectionDialog
        wsId={wsId}
        collection={deletingCollection}
        open={deletingCollection !== null}
        onOpenChange={(open) => {
          if (!open) {
            setTimeout(() => setDeletingCollection(null), 150);
          }
        }}
        onConfirm={async () => {
          if (!deletingCollection) {
            return;
          }
          await deleteCollection.mutateAsync({ id: deletingCollection.id, spaceId: deletingCollection.space_id });
          setDeletingCollection(null);
        }}
      />
    </>
  );
}
