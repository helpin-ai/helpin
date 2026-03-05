import { useState } from 'react';
import { Check, Loader2, Plus, Tag, X } from 'lucide-react';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
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
      className={cn(
        'inline-flex h-5 items-center gap-1 rounded-sm border-[0.5px] px-2 text-[11px] font-medium text-foreground/80',
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
      {label.name}
      {onRemove && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
          className="ml-0.5 rounded-sm opacity-60 transition-opacity hover:opacity-100"
        >
          <X className="h-3 w-3" />
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
}

export function LabelPicker({
  workspaceId,
  teamId,
  selectedLabelIds,
  onChange,
  labels,
  onLabelsChange,
  className,
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

  return (
    <div className={cn('flex flex-wrap items-center gap-1', className)}>
      {selectedLabels.map((label) => (
        <LabelBadge key={label.id} label={label} onRemove={() => removeLabel(label.id)} />
      ))}

      <Popover open={open} onOpenChange={(v) => { setOpen(v); if (!v) setSearch(''); }}>
        <PopoverTrigger asChild>
          <button
            type="button"
            className={cn(
              'inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-xs text-muted-foreground transition-colors hover:bg-accent cursor-pointer',
              selectedLabels.length === 0 && 'text-muted-foreground',
            )}
            onClick={(e) => {
              e.stopPropagation();
              setOpen(true);
            }}
          >
            <Tag className="h-3 w-3" />
            {selectedLabels.length === 0 ? 'Add label' : 'Add'}
          </button>
        </PopoverTrigger>
        {open && (
          <PopoverContent
            className="w-[220px] p-0"
            align="start"
            side="bottom"
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => e.stopPropagation()}
          >
            <Command shouldFilter={true}>
              <CommandInput
                placeholder="Search labels..."
                className="h-8 text-xs"
                value={search}
                onValueChange={setSearch}
              />
              <CommandList>
                <CommandEmpty className="py-1.5 px-2">
                  <button
                    type="button"
                    className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-xs text-muted-foreground hover:bg-accent hover:text-foreground transition-colors cursor-pointer"
                    onClick={createAndSelect}
                    disabled={creating}
                  >
                    {creating ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin shrink-0" />
                    ) : (
                      <Plus className="h-3.5 w-3.5 shrink-0" />
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

                    return (
                      <CommandItem
                        key={label.id}
                        value={label.name}
                        onSelect={() => toggleLabel(label.id)}
                        className="flex items-center gap-2 text-xs"
                      >
                        <span
                          className="h-2.5 w-2.5 shrink-0 rounded-full"
                          style={{ backgroundColor: color ?? 'var(--muted-foreground)' }}
                        />
                        <span className="truncate">{label.name}</span>
                        <span className="text-[10px] text-muted-foreground">
                          {label.team_id ? 'Team' : 'Shared'}
                        </span>
                        {isSelected && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
                      </CommandItem>
                    );
                  })}
                </CommandGroup>
              </CommandList>
            </Command>
          </PopoverContent>
        )}
      </Popover>
    </div>
  );
}
