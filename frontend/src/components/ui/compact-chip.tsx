import { Delete01Icon } from '@/lib/icons';

interface CompactChipProps {
  title: string;
  displayId?: string;
  avatar?: React.ReactNode;
  onClick?: () => void;
  onRemove?: () => void;
}

export function CompactChip({ title, displayId, avatar, onClick, onRemove }: CompactChipProps) {
  return (
    <div className="flex items-center gap-2 rounded-md border px-2 py-1.5 text-xs">
      {avatar && <span className="flex shrink-0 items-center justify-center h-6 w-6">{avatar}</span>}
      {onClick ? (
        <button
          type="button"
          className="min-w-0 flex-1 text-left cursor-pointer hover:underline"
          onClick={onClick}
        >
          <span className="font-medium truncate block">{title}</span>
          {displayId && (
            <span className="truncate block text-[10px] text-muted-foreground">{displayId}</span>
          )}
        </button>
      ) : (
        <div className="min-w-0 flex-1">
          <span className="font-medium truncate block">{title}</span>
          {displayId && (
            <span className="truncate block text-[10px] text-muted-foreground">{displayId}</span>
          )}
        </div>
      )}
      {onRemove && (
        <button type="button" onClick={onRemove} className="shrink-0 text-muted-foreground hover:text-destructive transition-colors">
          <Delete01Icon className="h-3 w-3" />
        </button>
      )}
    </div>
  );
}
