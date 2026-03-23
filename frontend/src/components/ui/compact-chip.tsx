import { Trash2 } from 'lucide-react';

interface CompactChipProps {
  title: string;
  displayId?: string;
  onClick?: () => void;
  onRemove?: () => void;
}

export function CompactChip({ title, displayId, onClick, onRemove }: CompactChipProps) {
  return (
    <div className="flex items-center justify-between gap-1.5 rounded-md border px-2 py-1.5 text-xs">
      <button
        type="button"
        className="min-w-0 flex-1 text-left"
        onClick={onClick}
        disabled={!onClick}
      >
        <span className="font-medium truncate block">{title}</span>
      </button>
      {displayId && (
        <span className="shrink-0 text-[10px] text-muted-foreground">{displayId}</span>
      )}
      {onRemove && (
        <button type="button" onClick={onRemove} className="shrink-0 text-muted-foreground hover:text-destructive transition-colors">
          <Trash2 className="h-3 w-3" />
        </button>
      )}
    </div>
  );
}
