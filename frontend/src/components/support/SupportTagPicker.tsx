import { useMemo, useState } from 'react';
import { ArrowLeft02Icon, Cancel01Icon, Delete01Icon, Loading01Icon, PencilEdit01Icon, PlusSignIcon, Tag01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { QuietSearchInput } from '@/components/design-system/quiet';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { ColorPicker, PRESET_COLORS } from '@/components/pm/ColorPicker';
import {
  useAddConversationTag,
  useCreateSupportTag,
  useDeleteSupportTag,
  useRemoveConversationTag,
  useSupportTags,
  useUpdateSupportTag,
} from '@/hooks/queries/useSupport';
import type { SupportSystemTag, SupportTag } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { getSupportTagStyle, hexToRgb, normalizeHexColor, tintBg, tintBorder } from './supportTagColorStyle';

export const SUPPORT_SYSTEM_TAGS: Record<SupportSystemTag, { name: string; color: string }> = {
  ai_handoff: { name: 'AI handoff', color: '#e58c3a' },
  ai_resolved: { name: 'AI resolved', color: '#45a557' },
};

const AUTO_TAG_COLORS = [
  '#5e6ad2',
  '#4e8fea',
  '#3daed4',
  '#2da88e',
  '#45a557',
  '#e2564a',
  '#e54e78',
  '#b44ec9',
  '#8b5cf6',
  '#788596',
];

function colorDistance(a: string, b: string) {
  const left = hexToRgb(a);
  const right = hexToRgb(b);
  if (!left || !right) return 0;
  const dr = left.r - right.r;
  const dg = left.g - right.g;
  const db = left.b - right.b;
  return Math.sqrt((dr * dr) + (dg * dg) + (db * db));
}

function automaticTagColor(name: string, existingColors: Array<string | null | undefined>) {
  let hash = 0;
  for (const char of name) {
    hash = (hash * 31 + char.charCodeAt(0)) >>> 0;
  }
  const usedPresetColors = existingColors
    .map(normalizeHexColor)
    .filter((color): color is string => !!color && AUTO_TAG_COLORS.includes(color));
  if (usedPresetColors.length === 0) {
    return AUTO_TAG_COLORS[hash % AUTO_TAG_COLORS.length];
  }
  const startIndex = hash % AUTO_TAG_COLORS.length;
  return [...AUTO_TAG_COLORS]
    .map((color, index) => ({
      color,
      distance: Math.min(...usedPresetColors.map((used) => colorDistance(color, used))),
      offset: (index - startIndex + AUTO_TAG_COLORS.length) % AUTO_TAG_COLORS.length,
    }))
    .sort((a, b) => b.distance - a.distance || a.offset - b.offset)[0]?.color ?? AUTO_TAG_COLORS[startIndex];
}

export function SupportTagBadge({
  name,
  color,
  onRemove,
  className,
  showDot = false,
}: {
  name: string;
  color?: string | null;
  onRemove?: () => void;
  className?: string;
  showDot?: boolean;
}) {
  const tagColor = normalizeHexColor(color);
  return (
    <span
      className={cn(
        'inline-flex h-5 max-w-full min-w-0 items-center gap-1 rounded-sm border-[0.5px] px-2 text-[11px] font-medium',
        className,
      )}
      style={getSupportTagStyle(color)}
    >
      {showDot && (
        <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: tagColor ?? 'var(--muted-foreground)' }} />
      )}
      <span className="min-w-0 truncate">{name}</span>
      {onRemove && (
        <button
          type="button"
          aria-label={`Remove ${name}`}
          className="ml-0.5 rounded-sm opacity-60 transition-opacity hover:opacity-100"
          onClick={(event) => {
            event.stopPropagation();
            onRemove();
          }}
        >
          <Cancel01Icon className="h-3 w-3" />
        </button>
      )}
    </span>
  );
}

function ManageTagsDialog({
  workspaceId,
  tags,
  open,
  onOpenChange,
}: {
  workspaceId: string;
  tags: SupportTag[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [mode, setMode] = useState<'list' | 'create' | 'edit'>('list');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formName, setFormName] = useState('');
  const [formColor, setFormColor] = useState(PRESET_COLORS[0]);
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [query, setQuery] = useState('');
  const [submitAttempted, setSubmitAttempted] = useState(false);
  const updateTag = useUpdateSupportTag(workspaceId);
  const createTag = useCreateSupportTag(workspaceId);
  const deleteTag = useDeleteSupportTag(workspaceId);
  const showSearch = tags.length > 10;
  const filteredTags = useMemo(() => {
    const normalized = showSearch ? query.trim().toLowerCase() : '';
    if (!normalized) return tags;
    return tags.filter((tag) => tag.name.toLowerCase().includes(normalized));
  }, [query, showSearch, tags]);
  const normalizedName = formName.trim();
  const duplicate = tags.some((tag) => tag.id !== editingId && tag.name.toLowerCase() === normalizedName.toLowerCase());
  const pending = createTag.isPending || updateTag.isPending || deleteTag.isPending;
  const canSubmit = normalizedName.length > 0 && !duplicate && !pending;

  const startEdit = (tag: SupportTag) => {
    setMode('edit');
    setEditingId(tag.id);
    setFormName(tag.name);
    setFormColor(tag.color || PRESET_COLORS[0]);
    setDeleteConfirm(false);
    setSubmitAttempted(false);
  };

  const startCreate = () => {
    setMode('create');
    setEditingId(null);
    setFormName('');
    setFormColor(automaticTagColor(`tag-${tags.length + 1}`, tags.map((tag) => tag.color)));
    setDeleteConfirm(false);
    setSubmitAttempted(false);
  };

  const returnToList = () => {
    setMode('list');
    setEditingId(null);
    setFormName('');
    setFormColor(PRESET_COLORS[0]);
    setDeleteConfirm(false);
    setSubmitAttempted(false);
  };

  const submitForm = () => {
    setSubmitAttempted(true);
    if (!canSubmit) return;
    if (mode === 'edit' && editingId) {
      updateTag.mutate(
        { tagId: editingId, payload: { name: normalizedName, color: formColor } },
        { onSuccess: returnToList },
      );
      return;
    }
    if (mode === 'create') {
      createTag.mutate(
        { name: normalizedName, color: formColor },
        {
          onSuccess: () => {
            setQuery('');
            returnToList();
          },
        },
      );
    }
  };

  const confirmDelete = () => {
    if (!editingId) return;
    deleteTag.mutate(editingId, { onSuccess: returnToList });
  };

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => {
      onOpenChange(nextOpen);
      if (!nextOpen) returnToList();
    }}>
      <DialogContent aria-describedby={undefined} className="sm:max-w-[560px]">
        <DialogHeader className={mode === 'list' ? undefined : 'flex-row items-center gap-2 space-y-0'}>
          {mode !== 'list' ? (
            <button
              type="button"
              onClick={returnToList}
              className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
              aria-label="Back to tags"
            >
              <ArrowLeft02Icon className="h-4 w-4" />
            </button>
          ) : null}
          <DialogTitle>{mode === 'list' ? 'Manage tags' : mode === 'create' ? 'New tag' : `Edit ${normalizedName || 'tag'}`}</DialogTitle>
        </DialogHeader>
        {mode === 'list' ? (
          <div className="flex max-h-[520px] flex-col gap-4">
            <div className="space-y-3">
              {showSearch && (
                <div className="flex items-center gap-2">
                  <QuietSearchInput
                    autoFocus
                    placeholder="Search tags..."
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                  />
                </div>
              )}
              {tags.length === 0 ? (
                <div className="rounded-md border border-dashed px-3 py-8 text-center text-sm text-muted-foreground">
                  No tags yet.
                </div>
              ) : filteredTags.length === 0 ? (
                <div className="rounded-md border border-dashed px-3 py-8 text-center text-sm text-muted-foreground">
                  No matching tags.
                </div>
              ) : (
                <div className="max-h-[380px] space-y-1 overflow-y-auto">
                  {filteredTags.map((tag) => (
                    <div
                      key={tag.id}
                      role="button"
                      tabIndex={0}
                      className="group flex w-full items-center rounded-md px-1 py-1 text-left focus-visible:outline-none"
                      onClick={() => startEdit(tag)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault();
                          startEdit(tag);
                        }
                      }}
                    >
                      <div
                        className="flex min-w-0 flex-1 items-center gap-2 rounded-sm border-[0.5px] px-2 py-1 transition-colors"
                        style={{
                          backgroundColor: tintBg(tag.color) ?? 'var(--muted)',
                          borderColor: tintBorder(tag.color) ?? 'var(--border)',
                        }}
                      >
                        <span
                          className="h-2.5 w-2.5 shrink-0 rounded-full"
                          style={{ backgroundColor: tag.color ?? 'var(--muted-foreground)' }}
                        />
                        <span className="min-w-0 flex-1 truncate text-sm font-medium text-foreground/85">{tag.name}</span>
                        <div className="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <span
                                role="button"
                                tabIndex={0}
                                className="inline-flex h-6 w-6 items-center justify-center rounded-sm text-muted-foreground hover:bg-background/60 hover:text-foreground"
                                onClick={(event) => {
                                  event.stopPropagation();
                                  startEdit(tag);
                                }}
                                onKeyDown={(event) => {
                                  if (event.key === 'Enter' || event.key === ' ') {
                                    event.preventDefault();
                                    event.stopPropagation();
                                    startEdit(tag);
                                  }
                                }}
                              >
                                <PencilEdit01Icon className="h-3.5 w-3.5" />
                              </span>
                            </TooltipTrigger>
                            <TooltipContent side="top">
                              <span className="text-xs">Edit tag</span>
                            </TooltipContent>
                          </Tooltip>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <span
                                role="button"
                                tabIndex={0}
                                className="inline-flex h-6 w-6 items-center justify-center rounded-sm text-muted-foreground hover:bg-background/60 hover:text-destructive"
                                onClick={(event) => {
                                  event.stopPropagation();
                                  startEdit(tag);
                                  setDeleteConfirm(true);
                                }}
                                onKeyDown={(event) => {
                                  if (event.key === 'Enter' || event.key === ' ') {
                                    event.preventDefault();
                                    event.stopPropagation();
                                    startEdit(tag);
                                    setDeleteConfirm(true);
                                  }
                                }}
                              >
                                <Delete01Icon className="h-3.5 w-3.5" />
                              </span>
                            </TooltipTrigger>
                            <TooltipContent side="top">
                              <span className="text-xs">Delete tag</span>
                            </TooltipContent>
                          </Tooltip>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
            <div className="flex items-center justify-between gap-2 border-t border-border/40 pt-4">
              <span className="text-sm text-muted-foreground">{tags.length} tags</span>
              <Button type="button" size="sm" onClick={startCreate}>
                <PlusSignIcon className="h-4 w-4" />
                New tag
              </Button>
            </div>
          </div>
        ) : (
          <div className="flex max-h-[520px] flex-col gap-4">
            <div className="space-y-4 overflow-y-auto">
              <div className="space-y-1.5">
                <Label htmlFor="support-tag-name" className="text-xs">Name <span className="text-destructive">*</span></Label>
                <Input
                  id="support-tag-name"
                  autoFocus
                  className="h-9 text-sm"
                  value={formName}
                  onChange={(event) => {
                    setFormName(event.target.value);
                    setDeleteConfirm(false);
                  }}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      event.preventDefault();
                      submitForm();
                    }
                    if (event.key === 'Escape') {
                      returnToList();
                    }
                  }}
                />
                {submitAttempted && !normalizedName ? (
                  <p className="text-xs text-destructive">Name is required.</p>
                ) : submitAttempted && duplicate ? (
                  <p className="text-xs text-destructive">That tag already exists.</p>
                ) : null}
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">Color</Label>
                <ColorPicker value={formColor} onChange={setFormColor} />
              </div>
            </div>
            <div className="flex items-center justify-between gap-2 border-t border-border/40 pt-4">
              {mode === 'edit' ? (
                <Button
                  type="button"
                  variant={deleteConfirm ? 'destructive' : 'ghost'}
                  size="sm"
                  onClick={() => {
                    if (deleteConfirm) {
                      confirmDelete();
                    } else {
                      setDeleteConfirm(true);
                    }
                  }}
                  disabled={pending}
                >
                  <Delete01Icon className="h-3.5 w-3.5" />
                  {deleteConfirm ? 'Confirm delete' : 'Delete'}
                </Button>
              ) : <span />}
              <div className="flex items-center gap-2">
                <Button type="button" variant="ghost" size="sm" onClick={returnToList}>Cancel</Button>
                <Button type="button" size="sm" onClick={submitForm} disabled={pending}>
                  {pending ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : null}
                  {mode === 'create' ? 'Create' : 'Save'}
                </Button>
              </div>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

export function SupportTagPicker({
  workspaceId,
  conversationId,
  selectedTags,
  selectedTagIds,
  onSelectedTagIdsChange,
  className,
}: {
  workspaceId: string;
  conversationId?: string;
  selectedTags: SupportTag[];
  selectedTagIds?: string[];
  onSelectedTagIdsChange?: (tagIds: string[]) => void;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const [manageOpen, setManageOpen] = useState(false);
  const [search, setSearch] = useState('');
  const { data: tags = [] } = useSupportTags(workspaceId);
  const createTag = useCreateSupportTag(workspaceId);
  const addTag = useAddConversationTag(workspaceId);
  const removeTag = useRemoveConversationTag(workspaceId);
  const controlled = !!onSelectedTagIdsChange;
  const selectedIds = useMemo(
    () => new Set(controlled ? (selectedTagIds ?? []) : selectedTags.map((tag) => tag.id)),
    [controlled, selectedTagIds, selectedTags],
  );
  const trimmed = search.trim();
  const hasExactMatch = trimmed
    ? tags.some((tag) => tag.name.toLowerCase() === trimmed.toLowerCase())
    : true;

  const toggleTag = (tagId: string) => {
    if (controlled) {
      onSelectedTagIdsChange(
        selectedIds.has(tagId)
          ? (selectedTagIds ?? []).filter((id) => id !== tagId)
          : [...(selectedTagIds ?? []), tagId],
      );
      return;
    }
    if (!conversationId) return;
    if (selectedIds.has(tagId)) {
      removeTag.mutate({ conversationId, tagId });
    } else {
      addTag.mutate({ conversationId, tagId });
    }
  };

  const createAndSelect = () => {
    if (!trimmed || hasExactMatch || createTag.isPending) return;
    createTag.mutate(
      { name: trimmed, color: automaticTagColor(trimmed, tags.map((tag) => tag.color)) },
      {
        onSuccess: (tag) => {
          if (controlled) {
            onSelectedTagIdsChange([...(selectedTagIds ?? []), tag.id]);
          } else if (conversationId) {
            addTag.mutate({ conversationId, tagId: tag.id });
          }
          setSearch('');
        },
      },
    );
  };

  return (
    <div className={cn('flex min-w-0 flex-wrap items-center gap-1', className)}>
      {selectedTags.map((tag) => (
        <SupportTagBadge
          key={tag.id}
          name={tag.name}
          color={tag.color}
          onRemove={() => {
            if (controlled) {
              onSelectedTagIdsChange((selectedTagIds ?? []).filter((id) => id !== tag.id));
              return;
            }
            if (conversationId) {
              removeTag.mutate({ conversationId, tagId: tag.id });
            }
          }}
        />
      ))}

      <Popover open={open} onOpenChange={(value) => { setOpen(value); if (!value) setSearch(''); }}>
        <PopoverTrigger asChild>
          <button
            type="button"
            className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition-colors hover:bg-accent"
          >
            <Tag01Icon className="h-3 w-3" />
            {selectedTags.length === 0 ? 'Add tag' : 'Add'}
          </button>
        </PopoverTrigger>
        <PopoverContent align="start" className="w-[240px] p-0">
          <Command shouldFilter>
            <CommandInput placeholder="Search tags..." className="h-8 text-xs" value={search} onValueChange={setSearch} />
            <CommandList className="max-h-60">
              <CommandEmpty className="px-3 py-2 text-xs text-muted-foreground">No tags found</CommandEmpty>
              <CommandGroup>
                {tags.map((tag) => {
                  const isSelected = selectedIds.has(tag.id);
                  const color = tag.color?.startsWith('#') ? tag.color : tag.color ? `#${tag.color}` : undefined;
                  return (
                    <CommandItem
                      key={tag.id}
                      value={tag.name}
                      data-checked={isSelected}
                      className="flex items-center gap-2 text-xs"
                      onSelect={() => toggleTag(tag.id)}
                    >
                      <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: color ?? 'var(--muted-foreground)' }} />
                      <span className="min-w-0 flex-1 truncate">{tag.name}</span>
                    </CommandItem>
                  );
                })}
              </CommandGroup>
            </CommandList>
          </Command>
          <div className="space-y-1 border-t p-1.5">
            {trimmed && !hasExactMatch ? (
              <Button
                variant="ghost"
                size="sm"
                className="h-7 w-full justify-start px-2 text-xs"
                onClick={createAndSelect}
                disabled={createTag.isPending}
              >
                {createTag.isPending ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <PlusSignIcon className="h-3.5 w-3.5" />}
                Create "{trimmed}"
              </Button>
            ) : null}
            {tags.length > 0 && (
              <Button
                variant="ghost"
                size="sm"
                className="h-7 w-full justify-start px-2 text-xs"
                onClick={() => {
                  setOpen(false);
                  setManageOpen(true);
                }}
              >
                Manage tags
              </Button>
            )}
          </div>
        </PopoverContent>
      </Popover>

      <ManageTagsDialog workspaceId={workspaceId} tags={tags} open={manageOpen} onOpenChange={setManageOpen} />
    </div>
  );
}
