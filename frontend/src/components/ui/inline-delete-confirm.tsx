import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** Replaces a list row with "Delete {name}? [Delete] [Cancel]" until the person decides. */
export function InlineDeleteConfirm({
  name,
  onConfirm,
  onCancel,
  pending = false,
  className,
}: {
  name: string;
  onConfirm: () => void;
  onCancel: () => void;
  pending?: boolean;
  className?: string;
}) {
  return (
    <div
      className={cn(
        'flex items-center gap-3 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3',
        className,
      )}
    >
      <span className="min-w-0 flex-1 text-sm text-foreground">
        Delete <span className="font-medium">{name}</span>?
      </span>
      <div className="flex shrink-0 items-center gap-1">
        <Button type="button" variant="destructive" size="xs" disabled={pending} onClick={onConfirm}>
          Delete
        </Button>
        <Button type="button" variant="ghost" size="xs" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </div>
  );
}
