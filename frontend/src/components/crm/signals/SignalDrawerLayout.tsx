import type { ReactNode } from 'react';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { timeAgo } from '@/lib/utils';
import { actionRequiresFollowThrough } from '@/lib/crmSituationPresentation';
import type { CRMSuggestion } from '@/lib/crmTypes';
import type { CRMSituationItem } from '@/lib/crmSituationTypes';

export function SignalDrawerLayout({ title, description, onClose, children }: { title: string; description: string; onClose: () => void; children: ReactNode }) {
  return <Sheet open onOpenChange={(open) => { if (!open) onClose(); }}>
    <SheetContent className="gap-0 overflow-hidden data-[side=right]:w-full data-[side=right]:sm:max-w-[600px]">
      <SheetHeader className="shrink-0 border-b border-quiet-divider-strong py-5 pr-14">
        <SheetTitle className="break-words text-[20px] font-semibold leading-snug">{title}</SheetTitle>
        <SheetDescription className="sr-only">{description}</SheetDescription>
      </SheetHeader>
      <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain break-words px-6 pt-4 pb-10">{children}</div>
    </SheetContent>
  </Sheet>;
}

export function SignalArrival({ at, label = 'Received' }: { at?: string; label?: string }) {
  if (!at || Number.isNaN(Date.parse(at))) return null;
  return <Tooltip>
    <TooltipTrigger asChild><time dateTime={at} tabIndex={0} className="text-xs text-quiet-text-tertiary focus-visible:outline-2 focus-visible:outline-quiet-field">{label} {timeAgo(at)}</time></TooltipTrigger>
    <TooltipContent>{new Date(at).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'long' })}</TooltipContent>
  </Tooltip>;
}

// Only remove exact repetitions (ignoring case and whitespace), never similar
// claims: a small wording difference can change the meaning of evidence.
export function repeatsSignalText(text: string | null | undefined, displayed: (string | null | undefined)[]) {
  const normalize = (value: string) => value.trim().replace(/\s+/g, ' ').toLowerCase();
  return !!text?.trim() && displayed.some((value) => !!value?.trim() && normalize(value) === normalize(text));
}

export function currentRecommendationText(actions: CRMSuggestion[]) {
  return actions.filter((action) => action.status === 'pending' || action.status === 'accepted' &&
    (action.execution_status !== 'succeeded' || actionRequiresFollowThrough(action)))
    .flatMap((action) => [action.title, action.description]);
}

export function signalOverviewText(item: CRMSituationItem) {
  return [item.situation.title, item.situation.lifecycle === 'closed' ? item.situation.outcome_summary : item.situation.next_step];
}
