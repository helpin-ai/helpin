import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  ChevronDown,
  Eye,
  Globe,
  Lock,
  Pencil,
  Pin,
  PinOff,
  Plus,
  Save,
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
  DialogDescription,
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
      <DialogContent className="sm:max-w-[420px] gap-0 p-0">
        <DialogHeader className="px-5 pt-5 pb-4">
          <DialogTitle className="text-base">{title}</DialogTitle>
          <DialogDescription className="text-xs text-muted-foreground">
            Save the current filters as a reusable view.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 px-5 pb-5">
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">View name</label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. My bug tracker, Sprint 4..."
              autoFocus
              onKeyDown={(e) => {
                if (e.key === 'Enter' && name.trim()) {
                  onSave(name.trim(), isShared);
                  onOpenChange(false);
                }
              }}
            />
          </div>
          <div className="flex items-center justify-between rounded-md border border-border/70 px-3 py-2.5">
            <div className="flex items-center gap-2.5">
              <div className={`flex h-7 w-7 items-center justify-center rounded-md ${isShared ? 'bg-blue-500/10 text-blue-500' : 'bg-muted text-muted-foreground'}`}>
                {isShared ? <Globe className="h-3.5 w-3.5" /> : <Lock className="h-3.5 w-3.5" />}
              </div>
              <div>
                <p className="text-sm font-medium leading-none">{isShared ? 'Shared' : 'Personal'}</p>
                <p className="mt-0.5 text-[11px] text-muted-foreground">{isShared ? 'Visible to all workspace members' : 'Only visible to you'}</p>
              </div>
            </div>
            <Switch checked={isShared} onCheckedChange={setIsShared} />
          </div>
        </div>
        <DialogFooter className="border-t border-border/70 px-5 py-3">
          <Button variant="ghost" size="sm" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            size="sm"
            disabled={!name.trim()}
            onClick={() => {
              onSave(name.trim(), isShared);
              onOpenChange(false);
            }}
          >
            Save View
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
  onOpenView,
}: {
  workspaceId: string;
  currentUserId: string;
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
  onSaveAsNew,
}: {
  view: PMView;
  isActive: boolean;
  isDefault: boolean;
  canClose: boolean;
  onActivate: () => void;
  onClose: () => void;
  workspaceId: string;
  hasChanges: boolean;
  onSaveAsNew: () => void;
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
      {isActive && hasChanges && isDefault && (
        <span
          role="button"
          onClick={(e) => {
            e.stopPropagation();
            onSaveAsNew();
          }}
          className="ml-0.5 inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-medium text-primary hover:bg-primary/10 transition-colors"
        >
          <Plus className="h-3 w-3" />
          New View
        </span>
      )}
      {isActive && hasChanges && !isDefault && (
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
  const [saveAsNewOpen, setSaveAsNewOpen] = useState(false);

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
            onSaveAsNew={() => setSaveAsNewOpen(true)}
          />
        );
      })}

      <SaveViewDialog
        open={saveAsNewOpen}
        onOpenChange={setSaveAsNewOpen}
        title="Save as New View"
        onSave={(name, isShared) => {
          usePMBoardStore.getState().saveCurrentAsView(workspaceId, name, isShared);
        }}
      />
    </div>
  );
}
