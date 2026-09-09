import { useState } from 'react';
import { Tick01Icon, Loading01Icon, PlusSignIcon, Tag01Icon, Cancel01Icon } from '@/lib/icons';
import { Popover, PopoverTrigger } from '@/components/ui/popover';
import { PMDropdownContent } from './PMDropdownContent';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { cn } from '@/lib/utils';
import { pmLabelService } from '@/lib/services/pmLabelService';
import { PRESET_COLORS } from '@/components/pm/ColorPicker';
import type { Label } from '@/lib/pmTypes';

// ── Helpers ─────────────────────────────────────────────────────────

/** Convert a hex color to a subtle background tint (12% opacity). */
function tintBg(hex: string | undefined) {
  if (!hex) return undefined;
  const c = hex.startsWith('#') ? hex : `#${hex}`;
  return `${c}1f`; // ~12% alpha
}

function tintBorder(hex: string | undefined) {
  if (!hex) return undefined;
  const c = hex.startsWith('#') ? hex : `#${hex}`;
  return `${c}40`; // ~25% alpha
}

// ── LabelBadge ──────────────────────────────────────────────────────

interface LabelBadgeProps {
  label: Label;
  onRemove?: () => void;
  className?: string;
}

export function LabelBadge({ label, onRemove, className }: LabelBadgeProps) {
  const color = label.color?.startsWith('#') ? label.color : label.color ? `#${label.color}` : undefined;

  return (
    <span
      title={label.name}
      className={cn(
        'inline-flex h-5 max-w-full min-w-0 items-center gap-1 overflow-hidden rounded-sm border-[0.5px] px-2 text-[11px] font-medium text-foreground/80',
        className,
      )}
      style={{
        backgroundColor: tintBg(label.color) ?? 'var(--muted)',
        borderColor: tintBorder(label.color) ?? 'var(--border)',
      }}
    >
      <span
        className="h-2 w-2 shrink-0 rounded-full"
        style={{ backgroundColor: color ?? 'var(--muted-foreground)' }}
      />
      <span className="min-w-0 truncate">{label.name}</span>
      {onRemove && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
          className="ml-0.5 rounded-sm opacity-60 transition-opacity hover:opacity-100"
        >
          <Cancel01Icon className="h-3 w-3" />
        </button>
      )}
    </span>
  );
}

// ── LabelPicker ─────────────────────────────────────────────────────

interface LabelPickerProps {
  workspaceId: string;
  teamId?: string;
  selectedLabelIds: string[];
  onChange: (labelIds: string[]) => void;
  labels: Label[];
  /** Called when the labels list changes (e.g. a new label was created inline). */
  onLabelsChange?: (labels: Label[]) => void;
  className?: string;
  triggerClassName?: string;
  /** Show only the trigger button, no badges — used for compact inline table cells */
  triggerOnly?: boolean;
  /** Use a compact color summary with names available on hover. */
  singleLine?: boolean;
  /** Label ids present on some but not all items in the selection (rendered italic + muted). */
  partialLabelIds?: string[];
}

export function LabelPicker({
  workspaceId,
  teamId,
  selectedLabelIds,
  onChange,
  labels,
  onLabelsChange,
  className,
  triggerClassName,
  triggerOnly = false,
  singleLine = false,
  partialLabelIds,
}: LabelPickerProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [creating, setCreating] = useState(false);

  const availableLabels = labels.filter((l) => {
    if (l.archived) return false;
    if (!teamId) return !l.team_id;
    return !l.team_id || l.team_id === teamId;
  });
  const selectedLabels = availableLabels.filter((l) => selectedLabelIds.includes(l.id));

  const toggleLabel = (labelId: string) => {
    if (selectedLabelIds.includes(labelId)) {
      onChange(selectedLabelIds.filter((id) => id !== labelId));
    } else {
      onChange([...selectedLabelIds, labelId]);
    }
  };

  const removeLabel = (labelId: string) => {
    onChange(selectedLabelIds.filter((id) => id !== labelId));
  };

  const trimmed = search.trim().toLowerCase();
  const hasExactMatch = trimmed
    ? availableLabels.some((l) => l.name.toLowerCase() === trimmed)
    : true;

  const createAndSelect = async () => {
    if (!trimmed || hasExactMatch || creating) return;
    setCreating(true);
    try {
      const { data } = await pmLabelService.create({
        workspace_id: workspaceId,
        team_id: teamId || undefined,
        name: search.trim(),
        color: PRESET_COLORS[0],
      });
      if (data) {
        onLabelsChange?.([...labels, data]);
        onChange([...selectedLabelIds, data.id]);
        setSearch('');
      }
    } finally {
      setCreating(false);
    }
  };

  const labelBadges = selectedLabels.map((label) => (
    <LabelBadge key={label.id} label={label} onRemove={() => removeLabel(label.id)} />
  ));
  const visibleColorLabels = selectedLabels.slice(0, 10);
  const hiddenColorCount = selectedLabels.length - visibleColorLabels.length;

  return (
    <div className={cn(
      'flex min-w-0 items-center gap-1',
      singleLine ? 'w-full flex-nowrap overflow-hidden' : 'flex-wrap',
      className,
    )}>
      {!triggerOnly && singleLine && selectedLabels.length > 0 ? (
        <div
          data-slot="label-picker-viewport"
          className="relative min-w-0 flex-1 overflow-hidden"
        >
          <Tooltip>
            <TooltipTrigger asChild>
              <span
                className="flex h-5 min-w-0 items-center gap-1 overflow-hidden px-0.5"
                aria-label={selectedLabels.map((label) => label.name).join(', ')}
              >
                {visibleColorLabels.map((label) => (
                  <span
                    key={label.id}
                    className="size-2.5 shrink-0 rounded-full"
                    style={{ backgroundColor: label.color?.startsWith('#') ? label.color : label.color ? `#${label.color}` : 'var(--muted-foreground)' }}
                  />
                ))}
                {hiddenColorCount > 0 ? (
                  <span className="shrink-0 text-[10px] text-muted-foreground">+{hiddenColorCount}</span>
                ) : null}
              </span>
            </TooltipTrigger>
            <TooltipContent className="flex max-w-64 flex-col items-start gap-1 py-2">
              {selectedLabels.map((label) => (
                <span key={label.id} className="flex max-w-full items-center gap-1.5">
                  <span
                    className="size-2 shrink-0 rounded-full"
                    style={{ backgroundColor: label.color?.startsWith('#') ? label.color : label.color ? `#${label.color}` : 'var(--muted-foreground)' }}
                  />
                  <span className="truncate">{label.name}</span>
                </span>
              ))}
            </TooltipContent>
          </Tooltip>
        </div>
      ) : !triggerOnly ? labelBadges : null}

      <Popover open={open} onOpenChange={(v) => { setOpen(v); if (!v) setSearch(''); }}>
        <PopoverTrigger asChild>
          <button
            type="button"
            className={cn(
              'inline-flex shrink-0 items-center gap-1 rounded-md px-1.5 py-0.5 text-ui text-muted-foreground transition-colors hover:bg-accent cursor-pointer',
              selectedLabels.length === 0 && 'text-muted-foreground',
              triggerClassName,
            )}
            onClick={(e) => {
              e.stopPropagation();
              setOpen(true);
            }}
          >
            {selectedLabels.length > 0 && <Tag01Icon className="h-3 w-3" />}
            {selectedLabels.length === 0 ? '+ Add label' : 'Add'}
          </button>
        </PopoverTrigger>
        {open && (
          <PMDropdownContent
            className="w-[220px] p-0"
            align="start"
            side="bottom"
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => e.stopPropagation()}
          >
            <Command shouldFilter={true}>
              <CommandInput
                placeholder="Search labels..."
                className="h-8 text-ui"
                value={search}
                onValueChange={setSearch}
              />
              <CommandList>
                <CommandEmpty className="py-1.5 px-2">
                  <button
                    type="button"
                    className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-ui text-muted-foreground hover:bg-accent hover:text-foreground transition-colors cursor-pointer"
                    onClick={createAndSelect}
                    disabled={creating}
                  >
                    {creating ? (
                      <Loading01Icon className="h-3.5 w-3.5 animate-spin shrink-0" />
                    ) : (
                      <PlusSignIcon className="h-3.5 w-3.5 shrink-0" />
                    )}
                    Create &ldquo;{search.trim()}&rdquo;
                  </button>
                </CommandEmpty>
                <CommandGroup>
                  {availableLabels.map((label) => {
                    const isSelected = selectedLabelIds.includes(label.id);
                    const color = label.color?.startsWith('#')
                      ? label.color
                      : label.color
                        ? `#${label.color}`
                        : undefined;

                    const isPartial = !!partialLabelIds?.includes(label.id);
                    return (
                      <CommandItem
                        key={label.id}
                        value={label.name}
                        onSelect={() => toggleLabel(label.id)}
                        className={cn(
                          'flex items-center gap-2 text-ui',
                          isPartial && 'italic text-muted-foreground',
                        )}
                      >
                        <span
                          className="h-2.5 w-2.5 shrink-0 rounded-full"
                          style={{ backgroundColor: color ?? 'var(--muted-foreground)' }}
                        />
                        <span className="truncate">{label.name}</span>
                        <span className="text-[10px] text-muted-foreground">
                          {label.team_id ? 'Team' : 'Shared'}
                        </span>
                        {isSelected && <Tick01Icon className="ml-auto h-3.5 w-3.5 text-primary" />}
                      </CommandItem>
                    );
                  })}
                </CommandGroup>
              </CommandList>
            </Command>
          </PMDropdownContent>
        )}
      </Popover>
    </div>
  );
}
