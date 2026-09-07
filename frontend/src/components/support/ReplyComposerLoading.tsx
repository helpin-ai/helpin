import { Button } from '@/components/ui/button';
import { Loading01Icon } from '@/lib/icons';

/** Matches the empty editor's footprint while its code or conversation loads. */
export function ReplyComposerLoading() {
  return (
    <div
      data-support-reply-composer
      aria-busy="true"
      className="relative mx-3 mb-4 h-[146px] shrink-0 rounded-xl border border-border/40 bg-card"
    >
      <div aria-hidden="true" className="flex h-[40px] items-end gap-1 px-4">
        <span className="rounded-full bg-blue-50 px-3 py-1 text-xs font-medium text-blue-700 dark:bg-blue-900/25 dark:text-blue-400">Reply</span>
        <span className="px-3 py-1 text-xs text-muted-foreground">Note</span>
      </div>
      <div role="status" className="flex h-[64px] items-center gap-2 px-4 text-sm text-muted-foreground">
        <Loading01Icon aria-hidden="true" className="h-4 w-4 animate-spin motion-reduce:animate-none" />
        Preparing reply…
      </div>
      <div className="flex h-[40px] items-start justify-end px-4 pb-3">
        <Button size="sm" disabled className="h-7 rounded-full px-3 text-xs">Send</Button>
      </div>
    </div>
  );
}
