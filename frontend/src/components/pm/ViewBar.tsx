import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  ChevronDown,
  Eye,
  Globe,
  Lock,
  MoreHorizontal,
  Pencil,
  Pin,
  PinOff,
  Plus,
  Save,
  Search,
  Trash2,
  Undo2,
  X,
  Copy,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
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

function filtersEqual(a: BoardFilters, b: BoardFilters): boolean {
  const aKeys = Object.keys(a).filter((k) => a[k] !== undefined && a[k] !== '');
  const bKeys = Object.keys(b).filter((k) => b[k] !== undefined && b[k] !== '');
  if (aKeys.length !== bKeys.length) return false;
  return aKeys.every((k) => a[k] === b[k]);
}

// ── Save View Dialog ────────────────────────────────────────────────

function SaveViewDialog({
  open,
  onOpenChange,
  onSave,
  initialName,
  title,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSave: (name: string, isShared: boolean) => void;
  initialName?: string;
  title: string;
}) {
  const [name, setName] = useState(initialName ?? '');
  const [isShared, setIsShared] = useState(false);

  useEffect(() => {
    if (open) {
      setName(initialName ?? '');
      setIsShared(false);
    }
  }, [open, initialName]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div>
            <label className="text-sm font-medium">Name</label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="View name"
              className="mt-1"
              autoFocus
              onKeyDown={(e) => {
                if (e.key === 'Enter' && name.trim()) {
                  onSave(name.trim(), isShared);
                  onOpenChange(false);
                }
              }}
            />
          </div>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              {isShared ? <Globe className="h-4 w-4 text-muted-foreground" /> : <Lock className="h-4 w-4 text-muted-foreground" />}
              <span className="text-sm">{isShared ? 'Shared with workspace' : 'Personal view'}</span>
            </div>
            <Switch checked={isShared} onCheckedChange={setIsShared} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            disabled={!name.trim()}
            onClick={() => {
              onSave(name.trim(), isShared);
              onOpenChange(false);
            }}
          >
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
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
          <button className="ml-0.5 rounded p-0.5 hover:bg-accent transition-colors">
            <ChevronDown className="h-3 w-3" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-48">
          <DropdownMenuItem
            disabled={!hasChanges}
            onClick={() => saveChangesToView(workspaceId)}
          >
            <Save className="mr-2 h-4 w-4" />
            Save Changes
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={!hasChanges}
            onClick={discardChanges}
          >
            <Undo2 className="mr-2 h-4 w-4" />
            Discard Changes
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={() => setRenameOpen(true)}>
            <Pencil className="mr-2 h-4 w-4" />
            Edit
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => setSaveAsOpen(true)}>
            <Copy className="mr-2 h-4 w-4" />
            Save As...
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() =>
              updateView(workspaceId, view.id, { is_pinned: !view.is_pinned })
            }
          >
            {view.is_pinned ? (
              <>
                <PinOff className="mr-2 h-4 w-4" />
                Unpin
              </>
            ) : (
              <>
                <Pin className="mr-2 h-4 w-4" />
                Pin
              </>
            )}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            className="text-destructive focus:text-destructive"
            onClick={() => deleteView(workspaceId, view.id)}
          >
            <Trash2 className="mr-2 h-4 w-4" />
            Delete
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <SaveViewDialog
        open={renameOpen}
        onOpenChange={setRenameOpen}
        initialName={view.name}
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

function ViewsDropdown({
  workspaceId,
  currentUserId,
  openViewIds,
  onOpenView,
}: {
  workspaceId: string;
  currentUserId: string;
  openViewIds: Set<string>;
  onOpenView: (view: PMView) => void;
}) {
  const { views } = usePMBoardStore();
  const [open, setOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const personalViews = views.filter(
    (v) => !isDefaultView(v.id) && !v.is_shared && v.created_by === currentUserId
  );
  const sharedViews = views.filter((v) => !isDefaultView(v.id) && v.is_shared);

  return (
    <>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button variant="ghost" size="sm" className="h-7 gap-1 px-2 text-xs text-muted-foreground">
            <Plus className="h-3.5 w-3.5" />
            Views
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-56 p-0" align="start">
          <Command>
            <CommandInput placeholder="Search views..." />
            <CommandList>
              <CommandEmpty>No views found.</CommandEmpty>
              <CommandGroup>
                <CommandItem
                  onSelect={() => {
                    setOpen(false);
                    setCreateOpen(true);
                  }}
                >
                  <Plus className="mr-2 h-4 w-4" />
                  Create New View
                </CommandItem>
              </CommandGroup>
              {personalViews.length > 0 && (
                <>
                  <CommandSeparator />
                  <CommandGroup heading="Personal Views">
                    {personalViews.map((view) => (
                      <CommandItem
                        key={view.id}
                        value={view.name}
                        onSelect={() => {
                          onOpenView(view);
                          setOpen(false);
                        }}
                      >
                        <Eye className="mr-2 h-4 w-4" />
                        {view.name}
                        {view.is_pinned && <Pin className="ml-auto h-3 w-3 text-muted-foreground" />}
                      </CommandItem>
                    ))}
                  </CommandGroup>
                </>
              )}
              {sharedViews.length > 0 && (
                <>
                  <CommandSeparator />
                  <CommandGroup heading="Shared Views">
                    {sharedViews.map((view) => (
                      <CommandItem
                        key={view.id}
                        value={view.name}
                        onSelect={() => {
                          onOpenView(view);
                          setOpen(false);
                        }}
                      >
                        <Globe className="mr-2 h-4 w-4" />
                        {view.name}
                        {view.is_pinned && <Pin className="ml-auto h-3 w-3 text-muted-foreground" />}
                      </CommandItem>
                    ))}
                  </CommandGroup>
                </>
              )}
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>

      <SaveViewDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Create New View"
        onSave={(name, isShared) => {
          usePMBoardStore.getState().saveCurrentAsView(workspaceId, name, isShared);
        }}
      />
    </>
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
      onClick={onActivate}
      className={`group relative inline-flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium transition-colors whitespace-nowrap ${
        isActive
          ? 'text-foreground border-b-2 border-primary'
          : 'text-muted-foreground hover:text-foreground'
      }`}
    >
      {view.name}
      {isActive && hasChanges && (
        <span className="h-1.5 w-1.5 rounded-full bg-primary" title="Unsaved changes" />
      )}
      {isActive && !isDefault && (
        <ActiveViewMenu view={view} workspaceId={workspaceId} hasChanges={hasChanges} />
      )}
      {!isActive && canClose && (
        <span
          role="button"
          onClick={(e) => {
            e.stopPropagation();
            onClose();
          }}
          className="ml-0.5 rounded p-0.5 opacity-0 group-hover:opacity-100 hover:bg-accent transition-all"
        >
          <X className="h-3 w-3" />
        </span>
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

  return (
    <div className="flex items-center gap-0.5 border-b border-border/70 px-3 overflow-x-auto">
      <ViewsDropdown
        workspaceId={workspaceId}
        currentUserId={currentUserId}
        openViewIds={openNonPinnedIds}
        onOpenView={handleOpenView}
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
