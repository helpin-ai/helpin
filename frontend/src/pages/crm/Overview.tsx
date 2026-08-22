import { useMemo } from 'react';
import { format, isSameDay } from 'date-fns';
import { useLocation, useNavigate } from '@tanstack/react-router';
import {
  Alert01Icon,
  ArrowRight01Icon,
  Calendar01Icon,
  CheckmarkCircle02Icon,
  CheckListIcon,
  Clock03Icon,
  DollarCircleIcon,
  SparklesIcon,
} from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { MeetingPlatformIcon } from '@/components/crm/MeetingPlatform';
import { groupUpcomingMeetings } from '@/components/crm/upcomingCalendarMeetingGroups';
import { useDeals, useHealthScores, usePendingSuggestions } from '@/hooks/queries/useCRM';
import { useUpcomingCalendarMeetings } from '@/hooks/queries/useCRMMeetings';
import { useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useTasks } from '@/hooks/queries/useTasks';
import { useTitle } from '@/hooks/useTitle';
import { buildDealAttentionItems, buildTodayTaskItems } from '@/lib/crmToday';
import { detectMeetingPlatform } from '@/lib/meetingPresentation';
import { cn, timeAgo } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CRMSuggestionType } from '@/lib/crmTypes';
import { openDealRoute } from '@/components/crm/deal-detail/dealRouteNavigation';

const suggestionLabels: Record<CRMSuggestionType, string> = {
  follow_up: 'Follow up',
  deal_create: 'New deal',
  deal_advance: 'Stage change',
  enrichment: 'Enrichment',
  risk_alert: 'Risk alert',
};

function MetricCard({
  label,
  value,
  icon: Icon,
  attention,
}: {
  label: string;
  value: number | string;
  icon: typeof Clock03Icon;
  attention?: boolean;
}) {
  return (
    <div className="flex items-center gap-3 rounded-lg border border-border/30 bg-muted/20 px-4 py-3">
      <Icon className={cn(
        'h-4 w-4 shrink-0',
        attention ? 'text-amber-600 dark:text-amber-400' : 'text-muted-foreground',
      )} />
      <div className="min-w-0">
        <p className="text-lg font-semibold leading-none tabular-nums">{value}</p>
        <p className="mt-0.5 text-[11px] text-muted-foreground/70">{label}</p>
      </div>
    </div>
  );
}

function SectionEmpty({ children }: { children: string }) {
  return (
    <div className="flex min-h-32 items-center justify-center px-6 py-8 text-center text-sm text-muted-foreground">
      {children}
    </div>
  );
}

function LoadingRows({ count = 3 }: { count?: number }) {
  return <div className="space-y-2 p-3">{Array.from({ length: count }).map((_, index) => <Skeleton key={index} className="h-14 w-full" />)}</div>;
}

export function CRMOverviewPage() {
  useTitle('CRM Overview');
  const navigate = useNavigate();
  const location = useLocation();
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const workspaceSlug = workspace?.slug ?? '';
  const now = useMemo(() => new Date(), []);

  const { data: access, isLoading: accessLoading } = useWorkspaceAccess(workspaceId);
  const memberId = access?.membership.id;
  const tasksQuery = useTasks(workspaceId, {
    per_page: 100,
    archived: false,
    owner_member_ids: memberId,
    include_contacts: true,
    include_companies: true,
    include_deals: true,
  }, { enabled: Boolean(memberId) });
  const meetingsQuery = useUpcomingCalendarMeetings(workspaceId);
  const suggestionsQuery = usePendingSuggestions(workspaceId);
  const dealsQuery = useDeals(workspaceId, { per_page: 100 });
  const healthQuery = useHealthScores(workspaceId);

  const dueTasks = useMemo(
    () => memberId ? buildTodayTaskItems(tasksQuery.data?.data ?? [], now) : [],
    [memberId, now, tasksQuery.data?.data],
  );

  const meetingGroups = useMemo(() => groupUpcomingMeetings(meetingsQuery.data?.data ?? [])
    .map((group) => ({
      ...group,
      occurrences: [...group.occurrences].sort((left, right) =>
        Date.parse(left.event.start_time) - Date.parse(right.event.start_time)),
    }))
    .sort((left, right) =>
      Date.parse(left.occurrences[0]?.event.start_time ?? '') - Date.parse(right.occurrences[0]?.event.start_time ?? '')),
  [meetingsQuery.data?.data]);
  const upcomingGroups = meetingGroups.slice(0, 5);
  const meetingsToday = meetingGroups.filter((group) => {
    const startTime = group.occurrences[0]?.event.start_time;
    return startTime ? isSameDay(new Date(startTime), now) : false;
  }).length;

  const suggestions = useMemo(() => [...(suggestionsQuery.data?.data ?? [])]
    .sort((left, right) => {
      const leftRisk = left.suggestion_type === 'risk_alert' ? 1 : 0;
      const rightRisk = right.suggestion_type === 'risk_alert' ? 1 : 0;
      return rightRisk - leftRisk || right.confidence - left.confidence;
    }),
  [suggestionsQuery.data?.data]);

  const dealsNeedingAttention = useMemo(
    () => buildDealAttentionItems(dealsQuery.data?.data ?? [], healthQuery.data?.data ?? [], now),
    [dealsQuery.data?.data, healthQuery.data?.data, now],
  );

  const hasAttention = dueTasks.length > 0 || suggestions.length > 0 || dealsNeedingAttention.length > 0;
  const hasLoadError = tasksQuery.isError || meetingsQuery.isError || suggestionsQuery.isError || dealsQuery.isError || healthQuery.isError;
  const isOverviewLoading = accessLoading || tasksQuery.isLoading || meetingsQuery.isLoading || suggestionsQuery.isLoading || dealsQuery.isLoading || healthQuery.isLoading;
  const hasOverviewContent = hasAttention || upcomingGroups.length > 0;

  const retryAll = () => {
    void tasksQuery.refetch();
    void meetingsQuery.refetch();
    void suggestionsQuery.refetch();
    void dealsQuery.refetch();
    void healthQuery.refetch();
  };

  const openSuggestion = () => {
    void navigate({ to: '/w/$slug/crm/review', params: { slug: workspaceSlug } });
  };

  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <div className="mx-auto w-full max-w-4xl">
        <header className="mb-6 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-lg font-semibold">CRM Overview</h2>
            <p className="text-sm text-muted-foreground">Meetings, CRM follow-ups, and deals that need your attention.</p>
          </div>
          <Button variant="outline" size="sm" onClick={() => navigate({ to: '/w/$slug/crm/meetings', params: { slug: workspaceSlug } })}>
            <Calendar01Icon className="h-4 w-4" />
            View meetings
          </Button>
        </header>

        {hasLoadError && (
          <div className="mb-6 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-amber-500/30 bg-amber-500/5 px-4 py-3">
            <div>
              <p className="text-sm font-medium">Some CRM information could not be loaded</p>
              <p className="text-xs text-muted-foreground">The available sections are still shown below.</p>
            </div>
            <Button variant="outline" size="sm" onClick={retryAll}>Try again</Button>
          </div>
        )}

        <div className="mb-8 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <MetricCard
            label="CRM follow-ups"
            value={accessLoading || tasksQuery.isLoading ? '-' : dueTasks.length}
            icon={CheckListIcon}
            attention={dueTasks.length > 0}
          />
          <MetricCard
            label="Meetings today"
            value={meetingsQuery.isLoading ? '-' : meetingsToday}
            icon={Calendar01Icon}
          />
          <MetricCard
            label="Deals to review"
            value={dealsQuery.isLoading || healthQuery.isLoading ? '-' : dealsNeedingAttention.length}
            icon={DollarCircleIcon}
            attention={dealsNeedingAttention.length > 0}
          />
          <MetricCard
            label="AI review"
            value={suggestionsQuery.isLoading ? '-' : suggestions.length}
            icon={SparklesIcon}
            attention={suggestions.length > 0}
          />
        </div>

        {!hasOverviewContent && !isOverviewLoading && (
          <div className="flex flex-col items-center px-4 py-16 text-center">
            <div className="mb-5 flex h-14 w-14 items-center justify-center rounded-full bg-emerald-500/10 text-emerald-600">
              <CheckmarkCircle02Icon className="h-7 w-7" />
            </div>
            <div>
              <h3 className="mb-1 text-base font-medium">Your CRM is caught up</h3>
              <p className="max-w-md text-sm text-muted-foreground">New meetings, CRM-linked follow-ups, deal risks, and AI suggestions will appear here.</p>
            </div>
          </div>
        )}

        {(hasOverviewContent || isOverviewLoading) && (
          <div className="space-y-10">
          <main className="contents">
            <Card className="gap-0 border-0 bg-transparent py-0 shadow-none">
              <CardHeader className="mb-1 flex-row items-center justify-between px-1 py-0">
                <div>
                  <CardTitle className="text-xs font-medium uppercase tracking-wide text-muted-foreground">CRM follow-ups</CardTitle>
                </div>
                <div className="flex items-center gap-2">
                  {dueTasks.length > 0 && <span className="text-xs tabular-nums text-muted-foreground/50">{dueTasks.length}</span>}
                  <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={() => navigate({ to: '/w/$slug/pm/my-work', params: { slug: workspaceSlug } })}>
                    All tasks
                  </Button>
                </div>
              </CardHeader>
              <CardContent className="divide-y divide-border/40 p-0">
                {accessLoading || tasksQuery.isLoading ? <LoadingRows /> : dueTasks.length === 0 ? (
                  <SectionEmpty>No CRM-linked follow-ups are due today.</SectionEmpty>
                ) : dueTasks.slice(0, 8).map(({ task, timing, deadline }) => (
                  <button
                    key={task.id}
                    type="button"
                    className="flex w-full items-center gap-3 rounded-md px-2 py-2.5 text-left transition-colors hover:bg-muted/40"
                    onClick={() => navigate({ to: '/w/$slug/pm/tasks/$taskId', params: { slug: workspaceSlug, taskId: task.id }, search: { team: undefined, run: undefined } })}
                  >
                    <div className={cn(
                      'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border',
                      timing === 'overdue' ? 'border-rose-500/20 bg-rose-500/10 text-rose-600' : 'bg-muted/30 text-muted-foreground',
                    )}>
                      {timing === 'overdue' ? <Alert01Icon className="h-4 w-4" /> : <CheckListIcon className="h-4 w-4" />}
                    </div>
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium">{task.name}</p>
                      <p className="mt-0.5 truncate text-xs text-muted-foreground">
                        {task.task_key}{task.team_name ? ` - ${task.team_name}` : ''}{task.state_name ? ` - ${task.state_name}` : ''}
                      </p>
                    </div>
                    <div className="shrink-0 text-right">
                      <p className={cn('text-xs font-medium', timing === 'overdue' && 'text-rose-600')}>
                        {timing === 'overdue' ? `Overdue ${format(deadline, 'MMM d')}` : deadline.getHours() === 0 && deadline.getMinutes() === 0 ? 'Due today' : `Due ${format(deadline, 'p')}`}
                      </p>
                    </div>
                    <ArrowRight01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  </button>
                ))}
              </CardContent>
            </Card>

            <Card className="gap-0 border-0 bg-transparent py-0 shadow-none">
              <CardHeader className="mb-1 flex-row items-center justify-between px-1 py-0">
                <div>
                  <CardTitle className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Upcoming meetings</CardTitle>
                </div>
                <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={() => navigate({ to: '/w/$slug/crm/meetings', params: { slug: workspaceSlug } })}>
                  All meetings
                </Button>
              </CardHeader>
              <CardContent className="divide-y divide-border/40 p-0">
                {meetingsQuery.isLoading ? <LoadingRows /> : upcomingGroups.length === 0 ? (
                  <SectionEmpty>No upcoming meetings were found.</SectionEmpty>
                ) : upcomingGroups.map((group) => {
                  const candidate = group.occurrences[0];
                  if (!candidate) return null;
                  const { event, meeting } = candidate;
                  const platform = meeting?.platform ?? detectMeetingPlatform(event.meeting_url ?? '');
                  const attendeeCount = event.attendees.filter((attendee) => !attendee.self).length;
                  return (
                    <button
                      key={group.key}
                      type="button"
                      className="grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-md px-2 py-2.5 text-left transition-colors hover:bg-muted/40"
                      onClick={() => meeting
                        ? navigate({ to: '/w/$slug/crm/meetings/$meetingId', params: { slug: workspaceSlug, meetingId: meeting.id } })
                        : navigate({ to: '/w/$slug/crm/meetings', params: { slug: workspaceSlug } })}
                    >
                      <div className="flex min-w-0 items-center gap-3">
                        {platform ? <MeetingPlatformIcon platform={platform} /> : <div className="h-8 w-8 shrink-0 rounded-lg border bg-muted/30" />}
                        <div className="min-w-0">
                          <p className="truncate text-sm font-medium">{event.title}</p>
                          <p className="mt-0.5 truncate text-xs text-muted-foreground">
                            {group.recurring ? 'Recurring - ' : ''}{format(new Date(event.start_time), 'EEE, MMM d - p')}
                            {attendeeCount ? ` - ${attendeeCount} attendee${attendeeCount === 1 ? '' : 's'}` : ''}
                          </p>
                        </div>
                      </div>
                      <Badge variant={candidate.effective_auto_join ? 'secondary' : 'outline'} className="font-normal">
                        {candidate.effective_auto_join ? 'Notetaker scheduled' : 'Notetaker off'}
                      </Badge>
                    </button>
                  );
                })}
              </CardContent>
            </Card>
          </main>

          <aside className="contents">
            <Card className="gap-0 border-0 bg-transparent py-0 shadow-none">
              <CardHeader className="mb-1 flex-row items-center justify-between px-1 py-0">
                <div>
                  <CardTitle className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Deals needing attention</CardTitle>
                </div>
                {dealsNeedingAttention.length > 0 && <Badge variant="secondary">{dealsNeedingAttention.length}</Badge>}
              </CardHeader>
              <CardContent className="divide-y divide-border/40 p-0">
                {dealsQuery.isLoading || healthQuery.isLoading ? <LoadingRows /> : dealsNeedingAttention.length === 0 ? (
                  <SectionEmpty>No open deals need attention.</SectionEmpty>
                ) : dealsNeedingAttention.slice(0, 5).map(({ deal, reasons }) => (
                  <button
                    key={deal.id}
                    type="button"
                    className="flex w-full items-start gap-3 rounded-md px-2 py-2.5 text-left transition-colors hover:bg-muted/40"
                    onClick={() => openDealRoute(navigate as never, location, workspaceSlug, deal.id)}
                  >
                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-amber-500/20 bg-amber-500/10 text-amber-700 dark:text-amber-300">
                      <DollarCircleIcon className="h-4 w-4" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium">{deal.name}</p>
                      <p className="mt-0.5 line-clamp-2 text-xs leading-5 text-muted-foreground">{reasons.join(' - ')}</p>
                    </div>
                    <ArrowRight01Icon className="mt-2 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  </button>
                ))}
              </CardContent>
            </Card>

            <Card className="gap-0 border-0 bg-transparent py-0 shadow-none">
              <CardHeader className="mb-1 flex-row items-center justify-between px-1 py-0">
                <div>
                  <CardTitle className="text-xs font-medium uppercase tracking-wide text-muted-foreground">AI review</CardTitle>
                </div>
                <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={() => navigate({ to: '/w/$slug/crm/review', params: { slug: workspaceSlug } })}>
                  Open queue
                </Button>
              </CardHeader>
              <CardContent className="divide-y divide-border/40 p-0">
                {suggestionsQuery.isLoading ? <LoadingRows /> : suggestions.length === 0 ? (
                  <SectionEmpty>No suggestions are waiting for review.</SectionEmpty>
                ) : suggestions.slice(0, 4).map((suggestion) => (
                  <button
                    key={suggestion.id}
                    type="button"
                    className="flex w-full items-start gap-3 rounded-md px-2 py-2.5 text-left transition-colors hover:bg-muted/40"
                    onClick={openSuggestion}
                  >
                    <div className={cn(
                      'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border',
                      suggestion.suggestion_type === 'risk_alert'
                        ? 'border-rose-500/20 bg-rose-500/10 text-rose-600'
                        : 'bg-muted/30 text-muted-foreground',
                    )}>
                      {suggestion.suggestion_type === 'risk_alert' ? <Alert01Icon className="h-4 w-4" /> : <SparklesIcon className="h-4 w-4" />}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <p className="truncate text-sm font-medium">{suggestion.title}</p>
                        <span className="shrink-0 text-[11px] text-muted-foreground">{Math.round(suggestion.confidence * 100)}%</span>
                      </div>
                      <p className="mt-0.5 truncate text-xs text-muted-foreground">
                        {suggestionLabels[suggestion.suggestion_type]} - {timeAgo(suggestion.created_at)}
                      </p>
                    </div>
                    <ArrowRight01Icon className="mt-2 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  </button>
                ))}
              </CardContent>
            </Card>
          </aside>
        </div>
        )}
      </div>
    </div>
  );
}
