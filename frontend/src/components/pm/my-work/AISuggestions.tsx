import { useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { toast } from 'sonner';
import { QuietEmptyState, QuietListRow, QuietTextAction } from '@/components/design-system/quiet';
import { Button } from '@/components/ui/button';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { pmAISuggestionsService as service, type PMAISuggestion } from '@/lib/services/pmAISuggestionsService';
import { unwrapRequired } from '@/lib/queryUtils';
import { timeAgo } from '@/lib/utils';

interface Props { ws: string; memberId: string; slug: string; canEdit: boolean; canReadCRM: boolean }
const keys = (ws: string, memberId: string) => ['pm', ws, 'ai-suggestions', memberId] as const;

export function AISuggestions({ ws, memberId, slug, canEdit, canReadCRM }: Props) {
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<PMAISuggestion>();
  const heading = useRef<HTMLHeadingElement>(null);
  const opener = useRef<HTMLElement | null>(null);
  const query = useQuery({
    queryKey: [...keys(ws, memberId), 'list', page],
    queryFn: async () => unwrapRequired(await service.list(ws, page), 'AI suggestions'),
    enabled: !!ws && !!memberId,
    refetchInterval: 30_000,
  });

  return <div className="pb-28">
    <h2 ref={heading} tabIndex={-1} className="sr-only">AI suggestions</h2>
    {query.isPending ? <p role="status" className="py-8 text-sm text-quiet-text-tertiary">Loading suggestions…</p>
      : query.isError ? <QuietEmptyState title="Unable to load suggestions" description="Try again to load your meeting follow-ups." action={<QuietTextAction onClick={() => void query.refetch()}>Try again</QuietTextAction>} />
      : !query.data.data.length ? <QuietEmptyState title="You're all caught up" description="Internal meeting follow-ups that need your review will appear here." action={page > 1 ? <QuietTextAction onClick={() => setPage(1)}>Back to first page</QuietTextAction> : undefined} />
      : <>
        <div className="border-t border-quiet-divider-strong">
          {query.data.data.map((item) => <QuietListRow key={item.id}
            title={<span className="line-clamp-2 break-words">{item.title}</span>}
            detail={<span className="block truncate">{item.meeting_title || 'Meeting'}{validDate(item.meeting_at) && <> · {meetingDate(item.meeting_at!)}</>}</span>}
            provenance={<span>Suggested {validDate(item.created_at) ? timeAgo(item.created_at) : 'recently'}</span>}
            trailing={<span className="text-xs text-quiet-text-secondary">Review</span>}
            className="px-0 focus-visible:outline-2 focus-visible:outline-quiet-field"
            onClick={() => { opener.current = document.activeElement as HTMLElement; setSelected(item); }} />)}
        </div>
        {query.data.total_pages > 1 && <nav aria-label="Suggestion pages" className="flex items-center justify-between gap-4 py-5">
          <QuietTextAction disabled={page === 1 || query.isFetching} onClick={() => setPage((value) => value - 1)}>Previous</QuietTextAction>
          <span className="text-xs text-quiet-text-tertiary">{page} of {query.data.total_pages}</span>
          <QuietTextAction disabled={page >= query.data.total_pages || query.isFetching} onClick={() => setPage((value) => value + 1)}>Next</QuietTextAction>
        </nav>}
      </>}
    {selected && <SuggestionReview key={selected.id} {...{ ws, memberId, slug, canEdit, canReadCRM }} item={selected}
      onClose={() => setSelected(undefined)}
      onReviewed={() => { opener.current = null; setSelected(undefined); if (page > 1 && query.data?.data.length === 1) setPage((value) => value - 1); }}
      restoreFocus={() => { if (opener.current?.isConnected) opener.current.focus(); else heading.current?.focus(); }} />}
  </div>;
}

function SuggestionReview({ ws, memberId, slug, canEdit, canReadCRM, item, onClose, onReviewed, restoreFocus }: Props & {
  item: PMAISuggestion; onClose: () => void; onReviewed: () => void; restoreFocus: () => void;
}) {
  const client = useQueryClient();
  const [decisionError, setDecisionError] = useState('');
  const [copied, setCopied] = useState(false);
  const query = useQuery({ queryKey: [...keys(ws, memberId), 'detail', item.id], queryFn: async () => unwrapRequired(await service.detail(ws, item.id), 'Suggestion'), staleTime: 0 });
  const decision = useMutation({
    mutationFn: async (kind: 'accept' | 'dismiss') => {
      if (!query.data?.revision) throw new Error('Reload this suggestion before reviewing it.');
      const response = await service.decide(ws, item.id, kind, query.data.revision);
      if (response.error || !response.data) {
        throw new Error(response.status === 409 || response.status === 404 ? 'This suggestion has changed or was already reviewed. Close it and refresh your suggestions.' : 'Your decision could not be saved. Please try again.');
      }
      return kind;
    },
    onSuccess: (kind) => {
      toast.success(kind === 'accept' ? 'Marked as reviewed' : 'Suggestion dismissed');
      onReviewed();
      void client.invalidateQueries({ queryKey: keys(ws, memberId) });
      void client.invalidateQueries({ queryKey: ['crm', ws] });
    },
    onError: (error) => { setDecisionError(error.message); void client.invalidateQueries({ queryKey: keys(ws, memberId) }); },
  });
  const data = query.data;
  const title = data?.title || item.title;
  const subjectIsTitle = data?.draft_subject.trim().toLowerCase() === title.trim().toLowerCase();
  const copy = async () => {
    if (!data) return;
    try { await navigator.clipboard.writeText([data.draft_subject, data.draft_body].filter(Boolean).join('\n\n')); setCopied(true); }
    catch { toast.error('Unable to copy. Select the draft text to copy it.'); }
  };
  return <Sheet open onOpenChange={(open) => { if (!open && !decision.isPending) onClose(); }}>
    <SheetContent onCloseAutoFocus={(event) => { event.preventDefault(); restoreFocus(); }}
      className="gap-0 overflow-hidden data-[side=right]:w-full data-[side=right]:sm:max-w-[600px]">
      <SheetHeader className="shrink-0 border-b border-quiet-divider-strong py-5 pr-14">
        <SheetTitle className="break-words text-xl font-semibold leading-snug">{title}</SheetTitle>
        <SheetDescription className="sr-only">Review this AI-drafted meeting follow-up.</SheetDescription>
      </SheetHeader>
      <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain break-words px-6 py-5">
        {query.isPending ? <p role="status" className="text-sm text-quiet-text-tertiary">Loading draft…</p>
          : query.isError ? <QuietEmptyState title="Draft unavailable" description="It may have changed or already been reviewed." action={<QuietTextAction onClick={() => void query.refetch()}>Try again</QuietTextAction>} />
          : data && <>
            <div className="mb-6 text-xs leading-6 text-quiet-text-tertiary">
              {canReadCRM ? <Link to="/w/$slug/crm/meetings/$meetingId" params={{ slug, meetingId: data.meeting_id }} className="text-quiet-text-secondary underline decoration-quiet-field underline-offset-4 hover:text-quiet-text-primary">{data.meeting_title || 'Open meeting'}</Link> : <span>{data.meeting_title || 'Meeting'}</span>}
              {validDate(data.meeting_at) && <span> · {meetingDate(data.meeting_at!)}</span>}
              {validDate(data.created_at) && <div><Tooltip><TooltipTrigger asChild><time tabIndex={0} dateTime={data.created_at} className="focus-visible:outline-2 focus-visible:outline-quiet-field">Suggested {timeAgo(data.created_at)}</time></TooltipTrigger><TooltipContent>{new Date(data.created_at).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })}</TooltipContent></Tooltip></div>}
            </div>
            <div className="mb-4 flex items-center justify-between gap-4"><h3 className="text-sm font-medium text-quiet-text-secondary">Suggested follow-up</h3><QuietTextAction onClick={() => void copy()}>{copied ? 'Copied' : 'Copy draft'}</QuietTextAction></div>
            {!subjectIsTitle && data.draft_subject && <p className="mb-3 text-sm font-medium">{data.draft_subject}</p>}
            <p className="whitespace-pre-wrap text-sm leading-7 text-quiet-text-primary">{data.draft_body}</p>
          </>}
      </div>
      {data && !query.isError && <div className="shrink-0 border-t border-quiet-divider-strong px-6 py-4 pb-[max(1rem,env(safe-area-inset-bottom))]">
        {decisionError && <p role="alert" className="mb-3 text-sm text-quiet-accent">{decisionError}</p>}
        {canEdit ? <><div className="flex items-center justify-between gap-4"><QuietTextAction disabled={decision.isPending || query.isFetching} onClick={() => { setDecisionError(''); decision.mutate('dismiss'); }}>Dismiss</QuietTextAction><Button variant="outline" size="sm" disabled={decision.isPending || query.isFetching || !data.revision} onClick={() => { setDecisionError(''); decision.mutate('accept'); }}>{decision.isPending ? 'Saving…' : 'Mark reviewed'}</Button></div><p className="mt-3 text-xs text-quiet-text-tertiary">Marking reviewed records your review. Nothing is sent.</p></> : <p className="text-xs text-quiet-text-tertiary">You can read and copy this draft. PM edit access is needed to mark it reviewed or dismiss it.</p>}
      </div>}
    </SheetContent>
  </Sheet>;
}
function validDate(at?: string | null) { return !!at && !Number.isNaN(Date.parse(at)); }
function meetingDate(at: string) { return new Date(at).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' }); }
