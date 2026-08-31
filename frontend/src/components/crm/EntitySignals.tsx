import { formatDistanceToNow } from 'date-fns';
import { toast } from 'sonner';

import {
  Alert01Icon, Award01Icon, ChartIncreaseIcon, Clock01Icon,
  CheckmarkCircle02Icon, Mail01Icon, MoreVerticalIcon, Shield01Icon, UserGroupIcon, ZapIcon,
} from '@/lib/icons';
import { useCRMSignalFeedback, useCompanySignals, useContactSignals, useDealSignals, useDismissCRMSignal, useEmailAccounts, useWorkspaceMembers } from '@/hooks/queries';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CRMSignal, CRMSignalDismissalReason, CRMSignalType } from '@/lib/crmTypes';
import { cn } from '@/lib/utils';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

type SignalTone = 'positive' | 'blocker' | 'risk' | 'neutral';

const signalConfig: Record<CRMSignalType, { icon: React.ElementType; label: string; tone: SignalTone; toneLabel: string }> = {
  buying_intent: { icon: ChartIncreaseIcon, label: 'Buying intent', tone: 'positive', toneLabel: 'Buying' },
  objection: { icon: Alert01Icon, label: 'Objection raised', tone: 'blocker', toneLabel: 'Blocker' },
  competitor_mention: { icon: UserGroupIcon, label: 'Competitor mentioned', tone: 'neutral', toneLabel: 'Evaluating' },
  budget_signal: { icon: ZapIcon, label: 'Budget signal', tone: 'positive', toneLabel: 'Buying' },
  timeline_signal: { icon: Clock01Icon, label: 'Timeline identified', tone: 'neutral', toneLabel: 'Timing' },
  champion_signal: { icon: Award01Icon, label: 'Champion identified', tone: 'positive', toneLabel: 'Expanding' },
  risk_signal: { icon: Shield01Icon, label: 'Relationship risk', tone: 'risk', toneLabel: 'Risk' },
};

const toneClasses: Record<SignalTone, { marker: string; icon: string; word: string }> = {
  positive: { marker: 'bg-teal-700 dark:bg-teal-400', icon: 'text-teal-700 dark:text-teal-400', word: 'text-teal-700 dark:text-teal-400' },
  blocker: { marker: 'bg-orange-700 dark:bg-orange-400', icon: 'text-orange-700 dark:text-orange-400', word: 'text-orange-700 dark:text-orange-400' },
  risk: { marker: 'bg-foreground/45', icon: 'text-foreground/65', word: 'text-foreground/65' },
  neutral: { marker: 'bg-transparent', icon: 'text-muted-foreground', word: 'text-foreground/65' },
};

const dismissalReasons: Array<{ value: CRMSignalDismissalReason; label: string }> = [
  { value: 'incorrect_evidence', label: 'Incorrect evidence' },
  { value: 'wrong_entity', label: 'Wrong person or account' },
  { value: 'duplicate', label: 'Duplicate signal' },
  { value: 'irrelevant', label: 'Not relevant' },
  { value: 'handled', label: 'Already handled' },
  { value: 'bad_timing', label: 'Bad timing' },
];

interface EntitySignalsProps {
  workspaceId: string;
  contactId?: string;
  dealId?: string;
  companyId?: string;
  presentation?: 'default' | 'compact' | 'overview';
  onOpenSource?: (signal: CRMSignal) => void;
}

function sourceLabel(signal: CRMSignal) {
  if (signal.source_type === 'email') return signal.metadata?.mailbox_email ? `Email · ${signal.metadata.mailbox_email}` : 'Email';
  if (signal.source_type === 'meeting') return 'Meeting';
  if (signal.source_type === 'call') return 'Call';
  if (signal.source_type === 'support') return 'Support';
  if (signal.source_type === 'note') return 'Note';
  return 'CRM activity';
}

function sourceAction(signal: CRMSignal) {
  if (signal.source_type === 'email') return 'Open thread';
  if (signal.source_type === 'meeting') return 'Open meeting';
  if (signal.source_type === 'call') return 'View call';
  if (signal.source_type === 'support') return 'View ticket';
  if (signal.source_type === 'note') return 'View note';
  return 'View activity';
}

function canOpenSource(signal: CRMSignal, onOpenSource?: (signal: CRMSignal) => void) {
  if (!onOpenSource) return false;
  if (signal.source_type === 'email') return !!signal.source_thread_id;
  if (signal.source_type === 'support') return !!signal.source_thread_id;
  return ['meeting', 'note', 'call'].includes(signal.source_type) && !!signal.source_id;
}

function EntitySignalsEmptyState({ workspaceId }: { workspaceId: string }) {
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug ?? '');
  const accounts = useEmailAccounts(workspaceId);
  const members = useWorkspaceMembers(workspaceId);
  const connectedMembers = new Set((accounts.data ?? []).filter((account) => account.is_active && account.status === 'connected').map((account) => account.member_id)).size;
  const watched = [
    { icon: ChartIncreaseIcon, title: 'Buying intent', detail: 'Pricing, product, and purchase signals' },
    { icon: UserGroupIcon, title: 'Buying committee', detail: 'New stakeholders and internal champions' },
    { icon: Alert01Icon, title: 'Blockers', detail: 'Objections and unanswered questions' },
    { icon: Shield01Icon, title: 'Churn risk', detail: 'Falling engagement and support pressure' },
  ];
  return (
    <div className="px-4 pb-6 pt-1 sm:px-6 lg:px-10">
      <h3 className="text-[14px] font-medium text-foreground">Nothing to act on yet</h3>
      <p className="mt-1 max-w-[560px] text-[13px] leading-[1.6] text-muted-foreground">Signals appear automatically as email, meeting, support, and CRM activity syncs. Here is what we watch for.</p>
      <div className="mt-4 max-w-[680px] border-t border-border/40">
        {watched.map(({ icon: Icon, title, detail }) => <div key={title} className="flex items-center gap-3 border-b border-border/40 py-2.5"><Icon className="h-[15px] w-[15px] shrink-0 text-muted-foreground/55" /><span className="w-[150px] shrink-0 text-[13px] font-medium text-foreground/80">{title}</span><span className="text-[12.5px] text-muted-foreground/65">{detail}</span></div>)}
      </div>
      <div className="mt-4 flex flex-wrap items-center gap-[18px]">
        <button type="button" className="inline-flex items-center gap-1.5 border-b border-border px-0.5 py-1.5 text-[13px] text-muted-foreground transition-colors hover:border-foreground hover:text-foreground" onClick={() => { window.location.href = `/w/${workspaceSlug}/settings/crm-email`; }}><Mail01Icon className="h-3.5 w-3.5" />Connect a mailbox</button>
        <span className="text-[12.5px] text-muted-foreground/65">{connectedMembers} of {members.data?.length ?? 0} employees connected</span>
      </div>
    </div>
  );
}

export function EntitySignals({ workspaceId, contactId, dealId, companyId, presentation = 'default', onOpenSource }: EntitySignalsProps) {
  const contactQuery = useContactSignals(workspaceId, contactId ?? '');
  const dealQuery = useDealSignals(workspaceId, dealId ?? '');
  const companyQuery = useCompanySignals(workspaceId, companyId ?? '');
  const dismiss = useDismissCRMSignal(workspaceId, contactId, dealId, companyId);
  const feedback = useCRMSignalFeedback(workspaceId, contactId, dealId, companyId);
  const query = contactId ? contactQuery : dealId ? dealQuery : companyQuery;
  const signals = (query.data?.data ?? []) as CRMSignal[];
  const overview = presentation === 'overview';

  if (query.isLoading) return <div className={cn(overview ? 'px-4 pb-6 pt-1 sm:px-6 lg:px-10' : 'py-2')}><div className="h-16 animate-pulse rounded bg-muted/35" /></div>;
  if (signals.length === 0) return overview ? <EntitySignalsEmptyState workspaceId={workspaceId} /> : <div className="border border-dashed border-border/60 px-4 py-6 text-center text-sm text-muted-foreground">No signals yet</div>;

  return (
    <div className={cn(overview ? 'border-t border-border/40' : 'space-y-2')}>
      {signals.map((signal) => {
        const config = signalConfig[signal.signal_type] || signalConfig.buying_intent;
        const tone = toneClasses[config.tone];
        const Icon = config.icon;
        return (
          <article key={signal.id} className={cn('group relative flex items-start gap-3 transition-colors', overview ? 'border-b border-border/40 px-4 py-3.5 hover:bg-muted/25 sm:px-6 lg:px-10' : 'rounded-md border p-2.5')}>
            {overview ? <span className={cn('absolute inset-y-0 left-0 w-[3px]', tone.marker)} /> : null}
            <Icon className={cn('mt-0.5 h-[15px] w-[15px] shrink-0', tone.icon)} />
            <div className="min-w-0 flex-1">
              <div className="flex min-w-0 items-center gap-2 pr-8">
                <span className="truncate text-[13.5px] font-semibold tracking-[-0.008em] text-foreground/90">{config.label}</span>
                <span className={cn('shrink-0 text-[11.5px] font-semibold uppercase tracking-[0.03em]', tone.word)}>{config.toneLabel}</span>
                <time className="ml-auto shrink-0 text-[11.5px] text-muted-foreground/60">{formatDistanceToNow(new Date(signal.detected_at), { addSuffix: true })}</time>
              </div>
              {signal.acted_at || signal.reviewed_at ? (
                <div className="mt-1 flex items-center gap-1.5 text-[11px] font-medium text-emerald-700 dark:text-emerald-300">
                  <CheckmarkCircle02Icon className="h-3.5 w-3.5" />
                  {signal.acted_at ? 'Acted on' : 'Reviewed'}
                </div>
              ) : null}
              <p className="mt-1 max-w-[700px] text-[13px] leading-[1.6] text-muted-foreground [text-wrap:pretty]">{signal.evidence_excerpt || signal.summary}</p>
              <div className="mt-1.5 flex flex-wrap items-center gap-2 text-[11.5px] text-muted-foreground/60"><span>{sourceLabel(signal)}</span>{signal.contact_name ? <><span className="h-2.5 w-px bg-border" /><span>{signal.contact_name}</span></> : null}{signal.deal_name ? <><span className="h-2.5 w-px bg-border" /><span>{signal.deal_display_id ? `${signal.deal_display_id} · ` : ''}{signal.deal_name}</span></> : null}{canOpenSource(signal, onOpenSource) ? <><span className="h-2.5 w-px bg-border" /><button type="button" className="font-medium text-orange-700 hover:text-orange-800 dark:text-orange-400" onClick={() => onOpenSource?.(signal)}>{sourceAction(signal)}</button></> : null}</div>
            </div>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button type="button" className="absolute right-3 top-3.5 rounded p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground sm:opacity-55 sm:group-hover:opacity-100 sm:focus:opacity-100" title="Signal actions" aria-label="Signal actions" disabled={dismiss.isPending || feedback.isPending}><MoreVerticalIcon className="h-4 w-4" /></button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem disabled={!!signal.reviewed_at || !!signal.acted_at} onSelect={() => feedback.mutate({ signalId: signal.id, action: 'reviewed' }, { onSuccess: () => toast.success('Signal marked reviewed'), onError: (error) => toast.error(error instanceof Error ? error.message : 'Signal could not be marked reviewed') })}>
                  {signal.reviewed_at || signal.acted_at ? 'Reviewed' : 'Mark reviewed'}
                </DropdownMenuItem>
                <DropdownMenuItem disabled={!!signal.acted_at} onSelect={() => feedback.mutate({ signalId: signal.id, action: 'acted' }, { onSuccess: () => toast.success('Signal marked acted on'), onError: (error) => toast.error(error instanceof Error ? error.message : 'Signal could not be marked acted') })}>
                  {signal.acted_at ? 'Acted on' : 'Mark acted on'}
                </DropdownMenuItem>
                {dismissalReasons.map((reason) => (
                  <DropdownMenuItem key={reason.value} onSelect={() => dismiss.mutate({ signalId: signal.id, reason: reason.value }, { onSuccess: () => toast.success('Signal dismissed'), onError: (error) => toast.error(error instanceof Error ? error.message : 'Signal could not be dismissed') })}>
                    {reason.label}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          </article>
        );
      })}
    </div>
  );
}
