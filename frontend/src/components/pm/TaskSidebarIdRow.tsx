import { Check, Copy } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { cn } from '@/lib/utils';

interface TaskSidebarIdRowProps {
  displayId: string | number;
  className?: string;
}

export function TaskSidebarIdRow({ displayId, className }: TaskSidebarIdRowProps) {
  const { copied, copy } = useCopyToClipboard();

  return (
    <div className={cn('mb-4 flex min-w-0 items-center gap-2', className)}>
      <span className="shrink-0 text-xs font-medium text-muted-foreground">Task ID:</span>
      <span className="min-w-0 truncate text-sm font-semibold text-foreground">{displayId}</span>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="h-6 w-6 shrink-0"
        aria-label={`Copy task ID ${displayId}`}
        onClick={() => copy(String(displayId))}
      >
        {copied ? <Check className="h-3.5 w-3.5 text-green-500" /> : <Copy className="h-3.5 w-3.5" />}
      </Button>
    </div>
  );
}
