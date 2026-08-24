import { useMemo, useState, type ElementType } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { format, formatDistanceToNow } from 'date-fns';
import { toast } from 'sonner';
import { useQueryClient } from '@tanstack/react-query';

import {
  Alert01Icon, ArrowRight02Icon, Calendar01Icon, ChartIncreaseIcon, CheckListIcon,
  CheckmarkSquare02Icon, DashedLineCircleIcon, DollarCircleIcon, LifebuoyIcon,
  Loading01Icon, Mail01Icon, MailReply01Icon, SparklesIcon, StickyNote01Icon,
  TelephoneIcon, UserGroupIcon, ZapIcon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { CreateTaskModal } from '@/components/pm/CreateTaskModal';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { openDealRoute } from '@/components/crm/deal-detail/dealRouteNavigation';
import {
  useCompanySummary, useContactSummary, useCreateTask, useDealSummary, useEmailAccounts,
  useRefreshCompanySummary, useRefreshContactSummary, useRefreshDealSummary,
} from '@/hooks/queries';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { associationsService } from '@/lib/services/associationsService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CRMEntitySummary, CRMSummarySource, SummaryHighlight } from '@/lib/crmTypes';
import type { CreateTaskRequest, WorkflowWithStates } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

interface EntitySummaryCardProps {
  workspaceId: string;
  contactId?: string;
  companyId?: string;
  dealId?: string;
  presentation?: 'default' | 'compact' | 'overview';
  onOpenTab?: (tab: 'emails' | 'meetings' | 'calls' | 'support' | 'notes', threadId?: string) => void;
  onTaskCreated?: () => void;
}

const highlightIcons: Record<SummaryHighlight['kind'], ElementType> = {
  momentum: ChartIncreaseIcon, risk: Alert01Icon, next_step: ArrowRight02Icon,
  stakeholder: UserGroupIcon, signal: ZapIcon,
};

const sourceIcons: Record<string, ElementType> = {
  email: Mail01Icon, meeting: Calendar01Icon, call: TelephoneIcon, note: StickyNote01Icon,
  task: CheckListIcon, deal: DollarCircleIcon, support: LifebuoyIcon, signal: ZapIcon,
};

function SummaryEyebrow() {
  return <div className="flex items-center gap-2 text-[12px] font-semibold uppercase tracking-[0.06em] text-muted-foreground/70"><SparklesIcon className="h-[15px] w-[15px] text-muted-foreground" />Summary</div>;
}

function LegacySummary({ summary, loading, pending, onGenerate, compact }: {
  summary?: CRMEntitySummary | null; loading: boolean; pending: boolean; onGenerate: () => void; compact: boolean;
}) {
  const hasBody = Boolean(summary?.summary_markdown.trim());
  return (
    <div>
      <div className="mb-2 flex items-center justify-between">
        <SummaryEyebrow />
        {hasBody ? <Button variant="ghost" size="sm" className="h-6 gap-1 px-2 text-[11px] text-muted-foreground" onClick={onGenerate} disabled={pending}>{pending ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <SparklesIcon className="h-3 w-3" />}{pending ? 'Generating…' : 'Regenerate'}</Button> : null}
      </div>
      <div className={cn('rounded-xl border border-dashed border-border/70 bg-muted/25 px-5 py-4', compact && 'rounded-md px-4 py-3')}>
        {loading ? <div className="flex items-center gap-2 text-sm text-muted-foreground"><Loading01Icon className="h-3.5 w-3.5 animate-spin" />Loading latest summary…</div>
          : hasBody ? <><p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground/90">{summary?.summary_markdown}</p>{summary?.highlights.length ? <div className="mt-3 grid gap-1.5">{summary.highlights.map((highlight, index) => { const Icon = highlightIcons[highlight.kind] ?? SparklesIcon; return <div key={`${highlight.kind}-${index}`} className="flex items-start gap-2 text-xs leading-relaxed text-foreground/85"><Icon className="mt-0.5 h-3 w-3 text-muted-foreground" /><span>{highlight.text}</span></div>; })}</div> : null}</>
            : <div className="flex flex-col items-start justify-between gap-3 sm:flex-row sm:items-center"><div><h3 className="text-[14px] font-semibold">Not enough signal yet</h3><p className="mt-1 text-[12.5px] leading-relaxed text-muted-foreground">A few recent interactions will make this summary more useful.</p></div><Button variant="outline" size="sm" onClick={onGenerate} disabled={pending}>{pending ? 'Generating…' : 'Generate anyway'}</Button></div>}
      </div>
    </div>
  );
}

function SummaryProgress({ summary }: { summary?: CRMEntitySummary | null }) {
  const steps = summary?.readiness?.steps ?? Array.from({ length: 3 }, (_, index) => ({ key: `empty-${index}`, label: 'One more activity', complete: false }));
  return <div className="mt-4 grid max-w-[640px] grid-cols-1 gap-2.5 sm:grid-cols-3 sm:gap-0">{steps.map((step) => <div key={step.key} className="pr-2.5"><div className={cn('h-0.5 w-full', step.complete ? 'bg-foreground' : 'bg-border/70')} /><div className={cn('mt-2 flex items-center gap-1.5 text-[12px]', step.complete ? 'text-foreground/75' : 'text-muted-foreground/65')}>{step.complete ? <CheckmarkSquare02Icon className="h-3.5 w-3.5" /> : <DashedLineCircleIcon className="h-3.5 w-3.5" />}<span>{step.label}</span></div></div>)}</div>;
}

export function EntitySummaryCard(props: EntitySummaryCardProps) {
  if (props.presentation === 'overview') return <OverviewEntitySummaryCard {...props} />;
  return <LegacyEntitySummaryCard {...props} />;
}

function LegacyEntitySummaryCard({ workspaceId, contactId, companyId, dealId, presentation = 'default' }: EntitySummaryCardProps) {
  const contactQuery = useContactSummary(workspaceId, contactId ?? '');
  const companyQuery = useCompanySummary(workspaceId, companyId ?? '');
  const dealQuery = useDealSummary(workspaceId, dealId ?? '');
  const contactRefresh = useRefreshContactSummary(workspaceId, contactId ?? '');
  const companyRefresh = useRefreshCompanySummary(workspaceId, companyId ?? '');
  const dealRefresh = useRefreshDealSummary(workspaceId, dealId ?? '');
  const query = contactId ? contactQuery : companyId ? companyQuery : dealQuery;
  const refresh = contactId ? contactRefresh : companyId ? companyRefresh : dealRefresh;
  const generateSummary = () => refresh.mutate(undefined, {
    onSuccess: (result) => { if (result.warnings?.length) toast.warning(result.warnings[0]); },
    onError: (error) => toast.error(error instanceof Error ? error.message : 'Summary could not be generated'),
  });
  return <LegacySummary summary={query.data} loading={query.isLoading} pending={refresh.isPending} onGenerate={generateSummary} compact={presentation === 'compact'} />;
}

function OverviewEntitySummaryCard({ workspaceId, contactId, companyId, dealId, onOpenTab, onTaskCreated }: EntitySummaryCardProps) {
  const contactQuery = useContactSummary(workspaceId, contactId ?? '');
  const companyQuery = useCompanySummary(workspaceId, companyId ?? '');
  const dealQuery = useDealSummary(workspaceId, dealId ?? '');
  const contactRefresh = useRefreshContactSummary(workspaceId, contactId ?? '');
  const companyRefresh = useRefreshCompanySummary(workspaceId, companyId ?? '');
  const dealRefresh = useRefreshDealSummary(workspaceId, dealId ?? '');
  const query = contactId ? contactQuery : companyId ? companyQuery : dealQuery;
  const refresh = contactId ? contactRefresh : companyId ? companyRefresh : dealRefresh;
  const summary = query.data;
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug ?? '');
  const accounts = useEmailAccounts(workspaceId);
  const { teams } = useAccessibleTeams(workspaceId);
  const createTask = useCreateTask(workspaceId);
  const [taskOpen, setTaskOpen] = useState(false);
  const [taskWorkflow, setTaskWorkflow] = useState<WorkflowWithStates | null>(null);
  const [taskTeamId, setTaskTeamId] = useState('');
  const [openingTask, setOpeningTask] = useState(false);
  const target = contactId ? { type: 'contact' as const, id: contactId } : companyId ? { type: 'company' as const, id: companyId } : dealId ? { type: 'deal' as const, id: dealId } : null;
  const emailSource = useMemo(() => summary?.sources?.find((source) => source.type === 'email' && (source.thread_id || source.entity_id)), [summary?.sources]);
  const canDraftReply = Boolean(emailSource && (accounts.data ?? []).some((account) => account.can_send));

  const generateSummary = () => refresh.mutate(undefined, {
    onSuccess: (result) => { if (result.warnings?.length) toast.warning(result.warnings[0]); },
    onError: (error) => toast.error(error instanceof Error ? error.message : 'Summary could not be generated'),
  });
  const openSource = (source: CRMSummarySource) => {
    const entityId = source.entity_id || source.source_id;
    if (source.type === 'email') onOpenTab?.('emails', source.thread_id || source.entity_id);
    else if (source.type === 'task' && entityId) openTaskRoute(navigate as never, location as never, workspaceSlug, entityId);
    else if (source.type === 'deal' && entityId) openDealRoute(navigate as never, location as never, workspaceSlug, entityId);
    else if (source.type === 'meeting') onOpenTab?.('meetings');
    else if (source.type === 'call') onOpenTab?.('calls');
    else if (source.type === 'note') onOpenTab?.('notes');
    else if (source.type === 'support') onOpenTab?.('support');
  };
  const openCreateTask = async () => {
    const team = teams[0];
    if (!team) { toast.error('Join a team before creating a task'); return; }
    setOpeningTask(true);
    try {
      const workflow = await pmWorkflowService.resolveTeamWorkflow(workspaceId, team.id);
      if (workflow.error || !workflow.data) throw new Error(workflow.error || 'Could not resolve team workflow');
      setTaskTeamId(team.id); setTaskWorkflow(workflow.data); setTaskOpen(true);
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Could not open task creator'); }
    finally { setOpeningTask(false); }
  };
  const createAndLinkTask = async (payload: CreateTaskRequest) => {
    const result = await createTask.mutateAsync(payload);
    if (target && result.task?.id) {
      const association = await associationsService.createAssociation({ workspace_id: workspaceId, from_object_type: 'task', from_object_id: result.task.id, to_object_type: target.type, to_object_id: target.id });
      if (association.error) throw new Error(association.error);
      queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'tasks'] }); onTaskCreated?.();
    }
    return result.task ? { id: result.task.id, task: result.task } : undefined;
  };

  const hasBody = Boolean(summary?.summary_markdown.trim());
  const progressCount = summary?.readiness?.count ?? 0;
  const progressThreshold = summary?.readiness?.threshold ?? 3;
  return (
    <section aria-label="Summary" className="border-b border-border/60">
      <div className="flex items-center gap-3 px-4 pb-2 pt-5 sm:px-6 lg:px-10"><SummaryEyebrow /><div className="ml-auto flex items-center gap-2 text-[12px] text-muted-foreground/70">{hasBody && summary?.computed_at ? <span>Generated {formatDistanceToNow(new Date(summary.computed_at), { addSuffix: true })}</span> : null}{hasBody && summary?.computed_at ? <span className="h-3 w-px bg-border" /> : null}{hasBody ? <button type="button" className="inline-flex items-center gap-1.5 text-muted-foreground transition-colors hover:text-foreground disabled:opacity-50" onClick={generateSummary} disabled={refresh.isPending}>{refresh.isPending ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <SparklesIcon className="h-3.5 w-3.5" />}{refresh.isPending ? 'Regenerating…' : 'Regenerate'}</button> : null}</div></div>
      {query.isLoading ? <div className="px-4 pb-6 pt-2 sm:px-6 lg:px-10"><div className="h-4 w-3/4 animate-pulse rounded-sm bg-muted" /><div className="mt-2 h-4 w-1/2 animate-pulse rounded-sm bg-muted" /></div>
        : hasBody ? <div className="px-4 pb-5 pt-0.5 sm:px-6 lg:px-10"><p className="max-w-[760px] whitespace-pre-wrap text-[14px] leading-[1.7] text-foreground/90 [text-wrap:pretty]">{summary?.summary_markdown}</p>{(summary?.next_step || summary?.sources?.length) ? <div className="mt-4 grid gap-5 border-t border-border/40 pt-3.5 md:grid-cols-[minmax(0,1fr)_1px_300px] md:gap-6"><div className="min-w-0"><div className="text-[11px] font-semibold uppercase tracking-[0.06em] text-muted-foreground/65">Next step</div><p className="mt-1.5 max-w-[520px] text-[13.5px] leading-[1.55] text-foreground/85 [text-wrap:pretty]">{summary?.next_step || 'Review the latest activity and choose the next follow-up.'}</p><div className="mt-2.5 flex flex-wrap items-center gap-[18px] text-[12.5px]"><button type="button" className="inline-flex items-center gap-1.5 text-muted-foreground transition-colors hover:text-foreground" onClick={() => void openCreateTask()} disabled={openingTask}>{openingTask ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <CheckmarkSquare02Icon className="h-3.5 w-3.5" />}Create task</button><button type="button" className="inline-flex items-center gap-1.5 text-muted-foreground transition-colors hover:text-foreground disabled:cursor-not-allowed disabled:opacity-45" disabled={!canDraftReply} title={emailSource ? (canDraftReply ? undefined : 'Only the owner of a connected mailbox can reply') : 'No email thread was used for this summary'} onClick={() => emailSource && openSource(emailSource)}><MailReply01Icon className="h-3.5 w-3.5" />Draft reply</button></div></div><div className="hidden bg-border/40 md:block" /><div className="min-w-0"><div className="text-[11px] font-semibold uppercase tracking-[0.06em] text-muted-foreground/65">Based on</div><div className="mt-1.5 grid gap-1.5">{(summary?.sources ?? []).map((source) => { const Icon = sourceIcons[source.type] ?? SparklesIcon; return <button key={source.key} type="button" className="flex min-w-0 items-center gap-2 text-left text-[12.5px] text-muted-foreground transition-colors hover:text-foreground" onClick={() => openSource(source)}><Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground/65" /><span className="min-w-0 flex-1 truncate">{source.label}</span><span className="shrink-0 text-[11.5px] text-muted-foreground/60">{format(new Date(source.occurred_at), 'MMM d')}</span></button>; })}</div></div></div> : null}</div>
          : <div className="px-4 pb-6 pt-1 sm:px-6 lg:px-10"><div className="flex max-w-[860px] flex-col items-start justify-between gap-5 sm:flex-row"><div className="min-w-0 flex-1"><h3 className="text-[14px] font-medium text-foreground">Not enough signal yet</h3><p className="mt-1 max-w-[520px] text-[13px] leading-[1.6] text-muted-foreground">The summary writes itself as activity builds. {progressCount} of {progressThreshold} qualifying activities so far.</p><SummaryProgress summary={summary} />{refresh.isPending ? <p className="mt-3 text-[12px] text-amber-700 dark:text-amber-400">Generating early from limited evidence, so confidence may be lower.</p> : null}</div><button type="button" className="inline-flex shrink-0 items-center gap-1.5 border-b border-border px-0.5 py-1.5 text-[13px] text-muted-foreground transition-colors hover:border-foreground hover:text-foreground disabled:opacity-50" onClick={generateSummary} disabled={refresh.isPending}>{refresh.isPending ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <SparklesIcon className="h-3.5 w-3.5" />}{refresh.isPending ? 'Generating…' : 'Generate anyway'}</button></div></div>}
      <CreateTaskModal open={taskOpen} onOpenChange={setTaskOpen} workspaceId={workspaceId} workflow={taskWorkflow ?? undefined} initialTeamId={taskTeamId} initialName={summary?.next_step || 'Follow up on this relationship'} onCreate={createAndLinkTask} />
    </section>
  );
}
