import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  MoreVerticalIcon,
  PinIcon,
  UndoIcon,
  ViewIcon,
  GlobeIcon,
  LockIcon,
  PencilEdit01Icon,
  FloppyDiskIcon,
  Delete01Icon,
  Cancel01Icon,
  Copy01Icon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command';
import { usePMBoardStore, type BoardFilters } from '@/stores/pmBoardStore';
import type { PMView } from '@/lib/pmTypes';
import { isDefaultView } from '@/lib/pmDefaultViews';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { SaveViewDialog } from './SaveViewDialog';

interface ViewBarProps {
  workspaceId: string;
  currentUserId: string;
}

const OPEN_VIEWS_KEY = (wsId: string) => `pm_open_views_${wsId}`;

function getOpenViewIds(workspaceId: string): string[] {
  try {
    const raw = localStorage.getItem(OPEN_VIEWS_KEY(workspaceId));
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

function setOpenViewIds(workspaceId: string, ids: string[]) {
  localStorage.setItem(OPEN_VIEWS_KEY(workspaceId), JSON.stringify(ids));
}

function normalizeFilters(f: BoardFilters): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(f)) {
    if (v !== undefined && v !== '') out[k] = v;
  }
  return out;
}

function filtersEqual(a: BoardFilters, b: BoardFilters): boolean {
  const na = normalizeFilters(a);
  const nb = normalizeFilters(b);
  const aKeys = Object.keys(na);
  const bKeys = Object.keys(nb);
  if (aKeys.length !== bKeys.length) return false;
  return aKeys.every((k) => na[k] === nb[k]);
}

// ── Active View Context Menu ────────────────────────────────────────

function ActiveViewMenu({
  view,
  workspaceId,
  hasChanges,
}: {
  view: PMView;
  workspaceId: string;
  hasChanges: boolean;
}) {
  const { saveChangesToView, discardChanges, deleteView, updateView } = usePMBoardStore();
  const [renameOpen, setRenameOpen] = useState(false);
  const [saveAsOpen, setSaveAsOpen] = useState(false);

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            className="ml-0.5 rounded p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
            aria-label="View options"
          >
            <MoreVerticalIcon className="h-3.5 w-3.5" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-48">
          <DropdownMenuItem
            disabled={!hasChanges}
            onClick={() => saveChangesToView(workspaceId)}
          >
            <FloppyDiskIcon className="mr-2 h-4 w-4" />
            Update
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => setSaveAsOpen(true)}>
            <Copy01Icon className="mr-2 h-4 w-4" />
            Save as new
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={!hasChanges}
            onClick={discardChanges}
          >
            <UndoIcon className="mr-2 h-4 w-4" />
            Discard changes
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={() => setRenameOpen(true)}>
            <PencilEdit01Icon className="mr-2 h-4 w-4" />
            Edit
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            className="text-destructive focus:text-destructive"
            onClick={() => deleteView(workspaceId, view.id)}
          >
            <Delete01Icon className="mr-2 h-4 w-4" />
            Delete
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <SaveViewDialog
        open={renameOpen}
        onOpenChange={setRenameOpen}
        initialName={view.name}
        initialIsShared={view.is_shared}
        title="Edit View"
        onSave={(name, isShared) => {
          updateView(workspaceId, view.id, { name, is_shared: isShared });
        }}
      />

      <SaveViewDialog
        open={saveAsOpen}
        onOpenChange={setSaveAsOpen}
        title="Save As New View"
        onSave={(name, isShared) => {
          usePMBoardStore.getState().saveCurrentAsView(workspaceId, name, isShared);
        }}
      />
    </>
  );
}

// ── Views Dropdown ──────────────────────────────────────────────────

function ViewsDropdownRow({
  view,
  onOpen,
  onTogglePin,
}: {
  view: PMView;
  onOpen: () => void;
  onTogglePin: () => void;
}) {
  return (
    <CommandItem
      key={view.id}
      value={view.name}
      onSelect={onOpen}
      className="group pr-8"
    >
      <span className="min-w-0 flex-1 truncate">{view.name}</span>
      <QuickTooltip label={view.is_pinned ? 'Unpin' : 'Pin'}>
        <button
          type="button"
          onMouseDown={(e) => e.stopPropagation()}
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            onTogglePin();
          }}
          className={`absolute right-1.5 top-1/2 z-10 flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded text-muted-foreground transition-opacity duration-150 hover:bg-accent hover:text-foreground ${
            view.is_pinned ? 'opacity-100' : 'opacity-0 group-hover:opacity-60 hover:!opacity-100'
          }`}
          aria-label={view.is_pinned ? 'Unpin view' : 'Pin view'}
        >
          <PinIcon className="h-3 w-3" />
        </button>
      </QuickTooltip>
    </CommandItem>
  );
}

function ViewsDropdown({
  currentUserId,
  onOpenView,
  onTogglePin,
}: {
  currentUserId: string;
  onOpenView: (view: PMView) => void;
  onTogglePin: (view: PMView) => void;
}) {
  const { views } = usePMBoardStore();
  const [open, setOpen] = useState(false);

  const personalViews = views.filter(
    (v) => !isDefaultView(v.id) && !v.is_shared && v.created_by === currentUserId
  );
  const sharedViews = views.filter((v) => !isDefaultView(v.id) && v.is_shared);
  const totalCustom = personalViews.length + sharedViews.length;
  const hasAnyCustom = totalCustom > 0;
  const showSearch = totalCustom > 10;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="sm" className="h-7 gap-1 px-2 text-xs text-muted-foreground">
          <ViewIcon className="h-3.5 w-3.5" />
          Views
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-64 p-0" align="start">
        <Command>
          {showSearch ? <CommandInput placeholder="Search views..." /> : null}
          <CommandList
            className="[scrollbar-width:thin] [&::-webkit-scrollbar]:!block [&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-border [&::-webkit-scrollbar-thumb:hover]:bg-muted-foreground/40"
          >
            {!hasAnyCustom ? (
              <div className="px-3 py-4 text-center text-xs text-muted-foreground">
                No saved views yet.
              </div>
            ) : (
              <CommandEmpty>No views found.</CommandEmpty>
            )}
            {personalViews.length > 0 && (
              <CommandGroup heading="Personal">
                {personalViews.map((view) => (
                  <ViewsDropdownRow
                    key={view.id}
                    view={view}
                    onOpen={() => {
                      onOpenView(view);
                      setOpen(false);
                    }}
                    onTogglePin={() => onTogglePin(view)}
                  />
                ))}
              </CommandGroup>
            )}
            {personalViews.length > 0 && sharedViews.length > 0 && <CommandSeparator />}
            {sharedViews.length > 0 && (
              <CommandGroup heading="Shared">
                {sharedViews.map((view) => (
                  <ViewsDropdownRow
                    key={view.id}
                    view={view}
                    onOpen={() => {
                      onOpenView(view);
                      setOpen(false);
                    }}
                    onTogglePin={() => onTogglePin(view)}
                  />
                ))}
              </CommandGroup>
            )}
          </CommandList>
          <div className="border-t border-border/60 px-3 py-2 text-[11px] text-muted-foreground">
            Want to create a view? Apply filters, then <span className="text-foreground/80">save as view</span>.
          </div>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

// ── View Tab ────────────────────────────────────────────────────────

function ViewTab({
  view,
  isActive,
  isDefault,
  canClose,
  onActivate,
  onClose,
  workspaceId,
  hasChanges,
}: {
  view: PMView;
  isActive: boolean;
  isDefault: boolean;
  canClose: boolean;
  onActivate: () => void;
  onClose: () => void;
  workspaceId: string;
  hasChanges: boolean;
}) {
  return (
    <button
      onClick={() => { if (!isActive) onActivate(); }}
      className={`group relative inline-flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium transition-colors whitespace-nowrap ${
        isActive
          ? 'text-foreground border-b-2 border-primary'
          : 'text-muted-foreground hover:text-foreground'
      }`}
    >
      {view.name}
      {isActive && hasChanges && !isDefault && (
        <span className="h-1.5 w-1.5 rounded-full bg-primary" title="Unsaved changes" />
      )}
      {isActive && !isDefault && (
        <ActiveViewMenu view={view} workspaceId={workspaceId} hasChanges={hasChanges} />
      )}
      {!isActive && canClose && (
        <QuickTooltip label="Close tab (view stays saved)">
          <span
            role="button"
            onClick={(e) => {
              e.stopPropagation();
              onClose();
            }}
            className="ml-0.5 rounded p-0.5 opacity-0 group-hover:opacity-100 hover:bg-accent transition-all"
          >
            <Cancel01Icon className="h-3 w-3" />
          </span>
        </QuickTooltip>
      )}
    </button>
  );
}

// ── Main ViewBar ────────────────────────────────────────────────────

export function ViewBar({ workspaceId, currentUserId }: ViewBarProps) {
  const { views, activeViewId, filters, savedViewFilters, applyView } = usePMBoardStore();

  const [openNonPinnedIds, setOpenNonPinnedIds] = useState<Set<string>>(() => {
    return new Set(getOpenViewIds(workspaceId));
  });

  // Persist open non-pinned view IDs
  useEffect(() => {
    setOpenViewIds(workspaceId, Array.from(openNonPinnedIds));
  }, [workspaceId, openNonPinnedIds]);

  const hasChanges = useMemo(
    () => !filtersEqual(filters, savedViewFilters),
    [filters, savedViewFilters]
  );

  // Visible tabs: defaults + pinned custom + open non-pinned
  const visibleViews = useMemo(() => {
    const result: PMView[] = [];
    for (const view of views) {
      if (isDefaultView(view.id) || view.is_pinned || openNonPinnedIds.has(view.id) || view.id === activeViewId) {
        result.push(view);
      }
    }
    return result.sort((a, b) => a.position - b.position);
  }, [views, openNonPinnedIds, activeViewId]);

  const handleOpenView = useCallback(
    (view: PMView) => {
      if (!isDefaultView(view.id) && !view.is_pinned) {
        setOpenNonPinnedIds((prev) => new Set([...prev, view.id]));
      }
      applyView(view);
    },
    [applyView]
  );

  const handleCloseTab = useCallback(
    (viewId: string) => {
      setOpenNonPinnedIds((prev) => {
        const next = new Set(prev);
        next.delete(viewId);
        return next;
      });
      // If closing the active view, switch to Everything
      if (viewId === activeViewId) {
        const everything = views.find((v) => v.id === '__default_everything__');
        if (everything) applyView(everything);
      }
    },
    [activeViewId, views, applyView]
  );

  const handleTogglePin = useCallback(
    async (view: PMView) => {
      const willBePinned = !view.is_pinned;
      const result = await usePMBoardStore
        .getState()
        .updateView(workspaceId, view.id, { is_pinned: willBePinned });
      if (!result) return; // API failed — leave UI as-is; store didn't change either.
      if (!willBePinned) {
        // Unpinning: remove from tab strip entirely.
        setOpenNonPinnedIds((prev) => {
          if (!prev.has(view.id)) return prev;
          const next = new Set(prev);
          next.delete(view.id);
          return next;
        });
        if (view.id === activeViewId) {
          const everything = views.find((v) => v.id === '__default_everything__');
          if (everything) applyView(everything);
        }
      }
    },
    [workspaceId, activeViewId, views, applyView]
  );

  return (
    <div className="ui-divider-bottom-fade flex items-center gap-0.5 px-3 overflow-x-auto">
      <ViewsDropdown
        currentUserId={currentUserId}
        onOpenView={handleOpenView}
        onTogglePin={handleTogglePin}
      />

      <div className="mx-1 h-4 w-px bg-border" />

      {visibleViews.map((view) => {
        const isDef = isDefaultView(view.id);
        const canClose = !isDef && !view.is_pinned;
        return (
          <ViewTab
            key={view.id}
            view={view}
            isActive={view.id === activeViewId}
            isDefault={isDef}
            canClose={canClose}
            onActivate={() => applyView(view)}
            onClose={() => handleCloseTab(view.id)}
            workspaceId={workspaceId}
            hasChanges={view.id === activeViewId && hasChanges}
          />
        );
      })}
    </div>
  );
}
