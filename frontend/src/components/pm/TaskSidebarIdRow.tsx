import { Check, Copy, GitBranch } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { cn } from '@/lib/utils';

interface TaskSidebarIdRowProps {
  displayId: string | number;
  taskName?: string;
  className?: string;
}

/** Slugify text into a git-safe branch segment (lowercase, hyphens, no trailing dash). */
function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48);
}

export function buildGitBranch(displayId: string | number, taskName?: string): string {
  const id = String(displayId).toLowerCase();
  if (!taskName?.trim()) return `feature/${id}`;
  return `feature/${id}-${slugify(taskName)}`;
}

export function TaskSidebarIdRow({ displayId, taskName, className }: TaskSidebarIdRowProps) {
  const { copied, copy } = useCopyToClipboard();
  const { copied: branchCopied, copy: copyBranch } = useCopyToClipboard();

  return (
    <div className={cn('mb-4 flex min-w-0 items-center gap-2', className)}>
      <span className="shrink-0 text-xs font-medium text-muted-foreground">Task ID:</span>
      <span className="min-w-0 truncate text-sm font-semibold text-foreground">{displayId}</span>
      <QuickTooltip label="Copy task ID">
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
      </QuickTooltip>
      <QuickTooltip label={branchCopied ? 'Copied!' : 'Copy git branch name'}>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-6 w-6 shrink-0"
          aria-label="Copy git branch name"
          onClick={() => copyBranch(buildGitBranch(displayId, taskName))}
        >
          {branchCopied ? (
            <Check className="h-3.5 w-3.5 text-green-500" />
          ) : (
            <GitBranch className="h-3.5 w-3.5" />
          )}
        </Button>
      </QuickTooltip>
    </div>
  );
}
