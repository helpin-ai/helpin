import { useState, type Dispatch, type SetStateAction } from 'react';
import { Collapsible } from 'radix-ui';
import { ChevronRight, CircleHelp, EllipsisVertical, FolderOpen, Inbox, Plus, Settings, Trash2 } from 'lucide-react';
import { ICON_MAP } from '@/components/ui/icon-picker';
import { useDocsCollections, useDocsDocuments, useDocsSpaces, useDeleteDocsSpace } from '@/hooks/queries';
import type { DocsSpace } from '@/lib/docsTypes';
import { useTruncationDetection } from '@/hooks/useTruncationDetection';
import { SpaceDialog } from '@/components/docs/SpaceDialog';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
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

function SidebarCollectionIcon({ name }: { name?: string | null }) {
  if (name) {
    const Icon = ICON_MAP[name];
    if (Icon) {
      return <Icon className="h-3.5 w-3.5" />;
    }
  }

  return <FolderOpen className="h-3.5 w-3.5" />;
}

function DocsSpaceCollections({
  wsId,
  spaceId,
  wsSlug,
  isActive,
  openCreate,
  onNavigate,
}: {
  wsId: string;
  spaceId: string;
  wsSlug: string;
  isActive: (link: string) => boolean;
  openCreate: (modal: 'docs_collection', options?: { spaceId?: string }) => void;
  onNavigate: (args: { to: string; params?: Record<string, string>; search?: Record<string, string> }) => void;
}) {
  const { data: collections } = useDocsCollections(wsId, spaceId);
  const { data: documents } = useDocsDocuments(wsId, { space_id: spaceId });
  const uncollectedCount = (documents ?? []).filter((document) => !document.collection_id).length;
  const uncollectedLink = `/w/${wsSlug}/docs/spaces/${spaceId}?collection=__uncollected__`;
  const { checkRef: checkColTruncation, isTruncated: isColTruncated } = useTruncationDetection();

  return (
    <SidebarMenuSub>
      {(collections ?? []).map((collection) => {
        const link = `/w/${wsSlug}/docs/spaces/${spaceId}?collection=${collection.id}`;
        const showTooltip = isColTruncated(collection.id);

        return (
          <SidebarMenuSubItem key={collection.id}>
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
                      onNavigate({
                        to: '/w/$slug/docs/spaces/$spaceId',
                        params: { slug: wsSlug, spaceId },
                        search: { collection: collection.id },
                      });
                    }}
                  >
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
              <Inbox className="h-3.5 w-3.5" />
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
          <Plus className="h-3.5 w-3.5" />
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
  const [editingSpace, setEditingSpace] = useState<DocsSpace | null>(null);
  const [deletingSpace, setDeletingSpace] = useState<DocsSpace | null>(null);
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
                  className="h-8 flex-1 rounded-md px-2"
                  isActive={isActive(spaceLink)}
                  onClick={() => {
                    toggleDocSpace(spaceKey);
                    onNavigate({
                      to: '/w/$slug/docs/spaces/$spaceId',
                      params: { slug: wsSlug, spaceId: space.id },
                    });
                  }}
                >
                  <ChevronRight className={`h-3.5 w-3.5 shrink-0 transition-transform ${isExpanded ? 'rotate-90' : ''}`} />
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
                  className="absolute right-1 flex h-5 w-5 items-center justify-center rounded opacity-0 transition-opacity hover:bg-muted group-hover/space:opacity-100 data-[state=open]:opacity-100"
                >
                  <EllipsisVertical className="h-3.5 w-3.5 text-muted-foreground" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="right" align="start">
                <DropdownMenuItem onClick={() => setEditingSpace(space)}>
                  <Settings className="h-4 w-4" />
                  Edit space
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => setDeletingSpace(space)}
                  className="text-destructive focus:text-destructive"
                >
                  <Trash2 className="h-4 w-4" />
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
          <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
            Team Spaces
          </SidebarGroupLabel>
          <SidebarMenu>
            {internalSpaces.map(renderSpaceItem)}
          </SidebarMenu>
        </SidebarGroup>
      )}
      {externalSpaces.length > 0 && (
        <SidebarGroup className="p-0 pb-3">
          <SidebarGroupLabel className="flex h-7 items-center gap-1 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
            External Spaces
            <QuickTooltip label="Published to your public help center">
              <CircleHelp className="h-[10px] w-[10px] text-muted-foreground/50" />
            </QuickTooltip>
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

      <ConfirmDialog
        open={deletingSpace !== null}
        onOpenChange={(open) => {
          if (!open) {
            setTimeout(() => setDeletingSpace(null), 150);
          }
        }}
        title="Delete space"
        description="This will permanently delete this space and all its documents. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => {
          if (!deletingSpace) {
            return;
          }

          deleteSpace.mutate(deletingSpace.id, {
            onSuccess: () => {
              setDeletingSpace(null);
              onNavigate({ to: '/w/$slug/docs', params: { slug: wsSlug } });
            },
          });
        }}
      />
    </>
  );
}
