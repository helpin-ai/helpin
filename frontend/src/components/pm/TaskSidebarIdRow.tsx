import { useState } from 'react';
import { Check, Copy, GitBranch, Terminal } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import type { TaskType } from '@/lib/pm-types/project';
import { cn } from '@/lib/utils';

interface TaskSidebarIdRowProps {
  displayId: string | number;
  taskKey?: string;
  taskName?: string;
  taskType?: TaskType;
  className?: string;
}

const BRANCH_PREFIX: Record<TaskType, string> = {
  feature: 'feature',
  bug: 'fix',
  chore: 'chore',
};

/** Slugify text into a git-safe branch segment (lowercase, hyphens, no trailing dash). */
function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48);
}

export function buildGitBranch(taskKey: string, taskName?: string, taskType?: TaskType): string {
  const prefix = BRANCH_PREFIX[taskType ?? 'feature'];
  const id = taskKey.toLowerCase();
  if (!taskName?.trim()) return `${prefix}/${id}`;
  return `${prefix}/${id}-${slugify(taskName)}`;
}

export function TaskSidebarIdRow({ displayId, taskKey, taskName, taskType, className }: TaskSidebarIdRowProps) {
  const { copied, copy } = useCopyToClipboard();
  const { copied: branchCopied, copy: copyBranch } = useCopyToClipboard();
  const { copied: cmdCopied, copy: copyCmd } = useCopyToClipboard();
  const [open, setOpen] = useState(false);

  const branchName = buildGitBranch(taskKey ?? String(displayId), taskName, taskType);
  const checkoutCmd = `git checkout -b ${branchName}`;

  return (
    <div className={cn('mb-4 flex min-w-0 items-center gap-2', className)}>
      <span className="shrink-0 text-xs font-medium text-muted-foreground">Task ID:</span>
      <span className="min-w-0 truncate text-sm font-semibold text-foreground">{taskKey ?? displayId}</span>
      <QuickTooltip label="Copy task ID">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-6 w-6 shrink-0"
          aria-label={`Copy task ID ${taskKey ?? displayId}`}
          onClick={() => copy(taskKey ?? String(displayId))}
        >
          {copied ? <Check className="h-3.5 w-3.5 text-green-500" /> : <Copy className="h-3.5 w-3.5" />}
        </Button>
      </QuickTooltip>

      <Popover open={open} onOpenChange={setOpen}>
        <QuickTooltip label="Copy git branch">
          <PopoverTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-6 w-6 shrink-0"
              aria-label="Copy git branch name"
            >
              <GitBranch className="h-3.5 w-3.5" />
            </Button>
          </PopoverTrigger>
        </QuickTooltip>
        <PopoverContent align="start" className="w-72 space-y-1.5 p-2" sideOffset={4} onOpenAutoFocus={(e) => e.preventDefault()} onFocusOutside={(e) => e.preventDefault()}>
          <div>
            <p className="text-[11px] leading-none font-medium text-muted-foreground">Branch</p>
            <div className="mt-1.5 flex items-center gap-1 rounded border border-border bg-muted/50 px-2 py-1">
              <GitBranch className="h-3 w-3 shrink-0 text-muted-foreground" />
              <code className="min-w-0 flex-1 truncate text-[11px]">{branchName}</code>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="h-5 w-5 shrink-0"
                aria-label="Copy branch name"
                onClick={(e) => { e.stopPropagation(); copyBranch(branchName); }}
              >
                {branchCopied ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
              </Button>
            </div>
          </div>
          <div>
            <p className="text-[11px] leading-none font-medium text-muted-foreground">Checkout</p>
            <div className="mt-1.5 flex items-center gap-1 rounded border border-border bg-muted/50 px-2 py-1">
              <Terminal className="h-3 w-3 shrink-0 text-muted-foreground" />
              <code className="min-w-0 flex-1 truncate text-[11px]">{checkoutCmd}</code>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="h-5 w-5 shrink-0"
                aria-label="Copy checkout command"
                onClick={(e) => { e.stopPropagation(); copyCmd(checkoutCmd); }}
              >
                {cmdCopied ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
              </Button>
            </div>
          </div>
        </PopoverContent>
      </Popover>
    </div>
  );
}
