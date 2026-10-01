import { useState } from 'react';
import { QuietPropertyRow, QuietStatusText } from '@/components/design-system/quiet';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import type { KnowledgeSourceRow } from '@/lib/knowledgeSourcesPresentation';

export function KnowledgeSourceSync({ source }: { source: KnowledgeSourceRow }) {
  const [open, setOpen] = useState(false);
  const syncing = source.status === 'queued' || source.status === 'running';
  const lastSync = validDate(source.lastSyncAt);
  const nextSync = validDate(source.nextSyncAt);
  const duration = syncDuration(source.syncStartedAt, source.syncCompletedAt);
  const status = syncStatus(source.status);
  const schedule = source.type === 'helpin_docs' ? 'On changes' : source.type === 'file' ? 'Manual' : nextSync ? 'Daily' : 'Auto-sync paused';

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <div className="space-y-1 whitespace-nowrap text-xs">
        <div className="flex items-center gap-2">
          <span className="w-7 text-quiet-text-tertiary">Last</span>
          {lastSync ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <SheetTrigger asChild>
                  <button type="button" aria-label={`View sync details for ${source.name}`} className="cursor-pointer text-quiet-text-secondary underline decoration-dotted decoration-quiet-text-tertiary underline-offset-4 transition-colors hover:text-quiet-accent hover:decoration-current focus-visible:outline-2 focus-visible:outline-ring">
                    <time dateTime={source.lastSyncAt!}>{compactTime(lastSync)}</time>
                  </button>
                </SheetTrigger>
              </TooltipTrigger>
              <TooltipContent>{fullTime(lastSync)} · View sync details</TooltipContent>
            </Tooltip>
          ) : <span className="text-quiet-text-secondary">Not synced yet</span>}
        </div>
        <div className="flex items-center gap-2 text-quiet-text-tertiary">
          {nextSync ? <><span className="w-7">Next</span><time dateTime={source.nextSyncAt!} title={fullTime(nextSync)}>{compactTime(nextSync)}</time></> : <span>{schedule}</span>}
        </div>
      </div>
      <SheetContent side="right" className="gap-0 overflow-hidden border-quiet-divider-strong p-0 data-[side=right]:w-full data-[side=right]:sm:max-w-[640px]">
        <SheetHeader className="shrink-0 border-b border-quiet-divider-strong px-6 py-5 pr-16 text-left">
          <SheetTitle>Sync details</SheetTitle>
          <SheetDescription className="truncate text-xs">{source.name}</SheetDescription>
        </SheetHeader>
        <div className="min-h-0 flex-1 space-y-6 overflow-y-auto px-6 py-5">
          <div className="space-y-3 [&>div]:px-0 [&>div>span:last-child]:whitespace-normal [&>div>span:last-child]:break-words">
            <QuietPropertyRow label="Status" value={<QuietStatusText tone={status.tone} pulse={syncing}>{status.label}</QuietStatusText>} />
            {source.syncStartedAt && <QuietPropertyRow label={source.status === 'queued' ? 'Last started' : 'Started'} value={timestamp(source.syncStartedAt)} />}
            {source.syncCompletedAt && <QuietPropertyRow label={source.status === 'queued' ? 'Last finished' : 'Finished'} value={timestamp(source.syncCompletedAt)} />}
            {duration && <QuietPropertyRow label={source.status === 'queued' ? 'Last duration' : 'Duration'} value={duration} />}
            <QuietPropertyRow label="Indexed" value={source.countLabel} />
            <QuietPropertyRow label="Chunks" value={source.indexedChunks ?? 0} />
            <QuietPropertyRow label="Refresh" value={schedule} />
            {nextSync && <QuietPropertyRow label="Next sync" value={timestamp(source.nextSyncAt!)} />}
          </div>
          {syncing && <div role="status" className="space-y-2 text-xs text-quiet-text-secondary">
            <p>{source.status === 'queued' ? 'Queued for sync' : 'Syncing source'} · {source.progress}%</p>
            <div className="h-1.5 overflow-hidden rounded-full bg-muted"><div className="h-full bg-primary transition-all" style={{ width: `${Math.max(0, Math.min(source.progress, 100))}%` }} /></div>
          </div>}
          {source.warning && <p role="status" className="border-t border-quiet-divider-strong pt-4 text-sm text-quiet-text-secondary">{source.warning}</p>}
          {source.error && <p role="alert" className="border-t border-quiet-divider-strong pt-4 text-sm text-destructive">{source.error}</p>}
        </div>
      </SheetContent>
    </Sheet>
  );
}

function validDate(value?: string | null) {
  if (!value) return null;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

function compactTime(date: Date) {
  return date.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
}

function fullTime(date: Date) {
  return date.toLocaleString(undefined, { year: 'numeric', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit', second: '2-digit', timeZoneName: 'short' });
}

function timestamp(value: string) {
  const date = validDate(value);
  return date ? <time dateTime={value}>{fullTime(date)}</time> : 'Unavailable';
}

function syncDuration(started?: string | null, completed?: string | null) {
  const start = validDate(started), end = validDate(completed);
  if (!start || !end || end < start) return null;
  const seconds = Math.round((end.getTime() - start.getTime()) / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  return minutes < 60 ? `${minutes}m ${seconds % 60}s` : `${Math.floor(minutes / 60)}h ${minutes % 60}m`;
}

function syncStatus(status: KnowledgeSourceRow['status']): { label: string; tone: 'neutral' | 'positive' | 'blocker' | 'lifecycle' } {
  switch (status) {
    case 'ready': return { label: 'Completed', tone: 'positive' };
    case 'failed': return { label: 'Failed', tone: 'blocker' };
    case 'running': return { label: 'Running', tone: 'lifecycle' };
    case 'queued': return { label: 'Queued', tone: 'neutral' };
    case 'stale': return { label: 'Needs refresh', tone: 'blocker' };
    default: return { label: 'Disabled', tone: 'neutral' };
  }
}
