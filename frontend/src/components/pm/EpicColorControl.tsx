import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { ColorPicker } from './ColorPicker';
import { resolveEpicColor } from './epicColor';

interface EpicColorControlProps {
  value?: string | null;
  onChange?: (color: string) => void;
  disabled?: boolean;
}

export function EpicColorControl({ value, onChange, disabled }: EpicColorControlProps) {
  const color = resolveEpicColor(value);
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(color);
  const square = <span className="block h-4 w-4 shrink-0 rounded-[4px] border border-foreground/10" style={{ backgroundColor: open ? draft : color }} />;

  if (!onChange) return <span className="inline-flex h-7 w-7 shrink-0 items-center justify-center" title={`Epic color: ${color}`} aria-label={`Epic color: ${color}`}>{square}</span>;

  return (
    <Popover open={open} onOpenChange={(next) => { setOpen(next); if (next) setDraft(color); }}>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label="Change epic color"
          title="Change epic color"
          disabled={disabled}
          className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md hover:bg-quiet-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-quiet-text-primary disabled:opacity-50"
        >
          {square}
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-[260px] max-w-[calc(100vw-2rem)] space-y-3 p-3">
        <p className="text-sm font-medium">Epic color</p>
        <ColorPicker value={draft} onChange={setDraft} shape="square" />
        <div className="flex justify-end gap-2">
          <Button size="sm" variant="ghost" onClick={() => setOpen(false)}>Cancel</Button>
          <Button size="sm" onClick={() => { onChange(draft); setOpen(false); }}>Apply</Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
