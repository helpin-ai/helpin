import { useMemo } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { CRMSearchResults } from '@/components/crm/CRMSearchResults';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Activity01Icon,
  Alert01Icon,
  BulbIcon,
  Calendar01Icon,
  ChartIncreaseIcon,
  CheckmarkCircle02Icon,
  Clock03Icon,
  DollarCircleIcon,
  FavouriteIcon,
  Mail01Icon,
  Message01Icon,
  Search01Icon,
  SparklesIcon,
  Tick01Icon,
  UserCheck01Icon,
} from '@/lib/icons';
import {
  useAcceptSuggestion,
  useBuyerSignals,
  useDismissSuggestion,
  useHealthScores,
  usePendingSuggestions,
} from '@/hooks/queries/useCRM';
import { useTitle } from '@/hooks/useTitle';
import { cn, timeAgo } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CRMBuyerSignal, CRMDealHealthScore, CRMSignalSourceType, CRMSignalType, CRMSuggestion } from '@/lib/crmTypes';

type Tone = 'neutral' | 'good' | 'warn' | 'danger' | 'accent';

const signalTypeConfig: Record<CRMSignalType, { label: string; tone: Tone; icon: typeof Activity01Icon }> = {
  buying_intent: { label: 'Buying intent', tone: 'good', icon: DollarCircleIcon },
  budget_signal: { label: 'Budget', tone: 'good', icon: ChartIncreaseIcon },
  timeline_signal: { label: 'Timeline', tone: 'accent', icon: Calendar01Icon },
  champion_signal: { label: 'Champion', tone: 'good', icon: UserCheck01Icon },
  competitor_mention: { label: 'Competitor', tone: 'warn', icon: Alert01Icon },
  objection: { label: 'Objection', tone: 'warn', icon: Message01Icon },
  risk_signal: { label: 'Risk', tone: 'danger', icon: Alert01Icon },
};

const sourceConfig: Record<CRMSignalSourceType, { label: string; icon: typeof Activity01Icon }> = {
  email: { label: 'Email', icon: Mail01Icon },
  meeting: { label: 'Meeting', icon: Calendar01Icon },
  support: { label: 'Support', icon: Message01Icon },
  note: { label: 'Note', icon: Message01Icon },
  manual: { label: 'Manual', icon: Activity01Icon },
};

const suggestionTypeConfig: Record<CRMSuggestion['suggestion_type'], { label: string; icon: typeof BulbIcon; tone: Tone }> = {
  deal_create: { label: 'New deal', icon: DollarCircleIcon, tone: 'good' },
  deal_advance: { label: 'Stage advance', icon: ChartIncreaseIcon, tone: 'accent' },
  follow_up: { label: 'Follow up', icon: Mail01Icon, tone: 'neutral' },
  enrichment: { label: 'Enrichment', icon: BulbIcon, tone: 'neutral' },
  risk_alert: { label: 'Risk alert', icon: Alert01Icon, tone: 'danger' },
};

function toneClasses(tone: Tone) {
  switch (tone) {
    case 'good':
      return 'bg-emerald-500/10 text-emerald-700 border-emerald-500/20 dark:text-emerald-300';
    case 'warn':
      return 'bg-amber-500/10 text-amber-700 border-amber-500/20 dark:text-amber-300';
    case 'danger':
      return 'bg-rose-500/10 text-rose-700 border-rose-500/20 dark:text-rose-300';
    case 'accent':
      return 'bg-sky-500/10 text-sky-700 border-sky-500/20 dark:text-sky-300';
    default:
      return 'bg-muted text-muted-foreground border-border';
  }
}

function progressClasses(tone: Tone) {
  switch (tone) {
    case 'good':
      return 'bg-emerald-500';
    case 'warn':
      return 'bg-amber-500';
    case 'danger':
      return 'bg-rose-500';
    case 'accent':
      return 'bg-sky-500';
    default:
      return 'bg-foreground/60';
  }
}

function healthTone(score: number): Tone {
  if (score >= 70) return 'good';
  if (score >= 40) return 'warn';
  return 'danger';
}

function formatFactorName(key: string) {
  return key.replace(/_/g, ' ');
}

function formatFactorValue(value: unknown) {
  if (typeof value === 'number') return Number.isInteger(value) ? String(value) : value.toFixed(1);
  if (typeof value === 'boolean') return value ? 'Yes' : 'No';
  if (typeof value === 'string') return value;
  return null;
}

function EmptyState({
  icon: Icon,
  title,
  description,
}: {
  icon: typeof Activity01Icon;
  title: string;
  description: string;
}) {
  return (
    <div className="flex min-h-40 flex-col items-center justify-center rounded-lg border border-dashed border-border/80 bg-muted/20 px-6 py-8 text-center">
      <div className="flex h-10 w-10 items-center justify-center rounded-lg border bg-background">
        <Icon className="h-5 w-5 text-muted-foreground" />
      </div>
      <p className="mt-3 text-sm font-medium">{title}</p>
      <p className="mt-1 max-w-sm text-xs leading-5 text-muted-foreground">{description}</p>
    </div>
  );
}

function IntelligenceReadinessState({
  onConnectEmail,
  onImportRecords,
  onTuneThresholds,
  onOpenContacts,
}: {
  onConnectEmail: () => void;
  onImportRecords: () => void;
  onTuneThresholds: () => void;
  onOpenContacts: () => void;
}) {
  return (
    <div className="rounded-lg border bg-card">
      <div className="grid gap-6 p-6 lg:grid-cols-[minmax(0,1fr)_380px]">
        <div className="flex min-h-72 flex-col justify-center">
          <div className="flex h-11 w-11 items-center justify-center rounded-lg border bg-muted/40">
            <SparklesIcon className="h-5 w-5 text-muted-foreground" />
          </div>
          <h2 className="mt-4 text-lg font-semibold">No CRM intelligence yet</h2>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
            Insights appear after Helpin has CRM activity to analyze. Connect email or add CRM records, then buyer signals,
            review suggestions, and deal health scores will show here when there is enough evidence.
          </p>
          <div className="mt-5 flex flex-wrap gap-2">
            <Button size="sm" onClick={onConnectEmail}>
              <Mail01Icon className="h-4 w-4" />
              Connect email
            </Button>
            <Button size="sm" variant="outline" onClick={onImportRecords}>
              <Activity01Icon className="h-4 w-4" />
              Import CRM records
            </Button>
            <Button size="sm" variant="outline" onClick={onTuneThresholds}>
              <ChartIncreaseIcon className="h-4 w-4" />
              Review thresholds
            </Button>
          </div>
        </div>

        <div className="space-y-3">
          <div className="rounded-lg border bg-muted/20 p-4">
            <p className="text-sm font-medium">What creates buyer signals</p>
            <p className="mt-1 text-xs leading-5 text-muted-foreground">
              Email threads, meetings, support conversations, notes, and CRM activity can produce signals such as intent,
              budget, objections, risk, and timeline pressure.
            </p>
          </div>
          <div className="rounded-lg border bg-muted/20 p-4">
            <p className="text-sm font-medium">What creates review work</p>
            <p className="mt-1 text-xs leading-5 text-muted-foreground">
              Suggestions only appear when a detected action is useful enough to recommend but not confident enough to run automatically.
            </p>
          </div>
          <div className="rounded-lg border bg-muted/20 p-4">
            <p className="text-sm font-medium">Need to inspect CRM data?</p>
            <p className="mt-1 text-xs leading-5 text-muted-foreground">
              Start with contacts and deals to confirm the workspace has records for intelligence to attach to.
            </p>
            <Button size="sm" variant="ghost" className="mt-3 h-8 px-2" onClick={onOpenContacts}>
              Open contacts
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

function StatTile({
  label,
  value,
  detail,
  icon: Icon,
  tone = 'neutral',
}: {
  label: string;
  value: string | number;
  detail: string;
  icon: typeof Activity01Icon;
  tone?: Tone;
}) {
  return (
    <Card size="sm" className="gap-3 rounded-lg py-4">
      <CardContent className="flex items-start justify-between gap-3 px-4">
        <div className="min-w-0">
          <p className="text-xs text-muted-foreground">{label}</p>
          <p className="mt-1 text-2xl font-semibold tabular-nums">{value}</p>
          <p className="mt-1 truncate text-xs text-muted-foreground">{detail}</p>
        </div>
        <div className={cn('flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border', toneClasses(tone))}>
          <Icon className="h-4 w-4" />
        </div>
      </CardContent>
    </Card>
  );
}

function BuyerSignalRow({ signal }: { signal: CRMBuyerSignal }) {
  const config = signalTypeConfig[signal.signal_type] ?? {
    label: signal.signal_type.replace(/_/g, ' '),
    tone: 'neutral' as const,
    icon: Activity01Icon,
  };
  const source = sourceConfig[signal.source_type] ?? sourceConfig.manual;
  const SignalIcon = config.icon;
  const SourceIcon = source.icon;
  const confidence = Math.round(signal.confidence * 100);

  return (
    <div className="rounded-lg border bg-card p-3 transition-colors hover:bg-muted/20">
      <div className="flex items-start gap-3">
        <div className={cn('mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border', toneClasses(config.tone))}>
          <SignalIcon className="h-4 w-4" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline" className={cn('capitalize', toneClasses(config.tone))}>
              {config.label}
            </Badge>
            <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
              <SourceIcon className="h-3.5 w-3.5" />
              {source.label}
            </span>
            {signal.metadata?.message_direction ? (
              <span className="text-xs capitalize text-muted-foreground">{signal.metadata.message_direction}</span>
            ) : null}
          </div>

          <p className="mt-2 text-sm leading-5">{signal.summary || 'Signal detected'}</p>

          {signal.evidence_excerpt ? (
            <p className="mt-2 line-clamp-2 rounded-md border-l-2 border-border bg-muted/30 px-3 py-2 text-xs leading-5 text-muted-foreground">
              {signal.evidence_excerpt}
            </p>
          ) : null}

          <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-1">
              <Clock03Icon className="h-3.5 w-3.5" />
              {timeAgo(signal.detected_at)}
            </span>
            {typeof signal.metadata?.participant_count === 'number' ? (
              <span>{signal.metadata.participant_count} participants</span>
            ) : null}
            {signal.source_thread_id ? <span>Thread linked</span> : null}
          </div>
        </div>
        <div className="w-16 shrink-0 text-right">
          <p className="text-xs font-medium tabular-nums">{confidence}%</p>
          <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted">
            <div className={cn('h-full rounded-full', progressClasses(config.tone))} style={{ width: `${confidence}%` }} />
          </div>
        </div>
      </div>
    </div>
  );
}

function ReviewQueue({
  suggestions,
  isLoading,
  hasSignals,
  onAccept,
  onDismiss,
  isMutating,
  onViewAll,
}: {
  suggestions: CRMSuggestion[];
  isLoading: boolean;
  hasSignals: boolean;
  onAccept: (id: string) => void;
  onDismiss: (id: string) => void;
  isMutating: boolean;
  onViewAll: () => void;
}) {
  return (
    <Card className="rounded-lg">
      <CardHeader className="pb-0">
        <div className="flex items-center justify-between gap-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <SparklesIcon className="h-4 w-4 text-muted-foreground" />
            Review Queue
          </CardTitle>
          <Button variant="ghost" size="sm" className="h-8" onClick={onViewAll}>
            View all
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-2">
        {isLoading ? (
          <div className="py-6 text-sm text-muted-foreground">Loading suggestions...</div>
        ) : suggestions.length === 0 ? (
          <EmptyState
            icon={CheckmarkCircle02Icon}
            title={hasSignals ? 'Signals found, nothing needs review' : 'No suggested actions yet'}
            description={
              hasSignals
                ? 'The signal feed has activity, but nothing currently crosses the review threshold for manual approval.'
                : 'Suggested actions will appear after buyer signals point to a deal, follow-up, enrichment, or risk worth reviewing.'
            }
          />
        ) : (
          suggestions.slice(0, 4).map((suggestion) => {
            const config = suggestionTypeConfig[suggestion.suggestion_type] ?? {
              label: suggestion.suggestion_type.replace(/_/g, ' '),
              icon: BulbIcon,
              tone: 'neutral' as const,
            };
            const Icon = config.icon;
            const confidence = Math.round(suggestion.confidence * 100);

            return (
              <div key={suggestion.id} className="rounded-lg border p-3">
                <div className="flex items-start gap-3">
                  <div className={cn('mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border', toneClasses(config.tone))}>
                    <Icon className="h-4 w-4" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <Badge variant="outline" className={cn('text-[11px]', toneClasses(config.tone))}>
                        {config.label}
                      </Badge>
                      <span className="text-xs text-muted-foreground">{confidence}% confidence</span>
                    </div>
                    <p className="mt-1 line-clamp-2 text-sm font-medium leading-5">{suggestion.title}</p>
                    {suggestion.description ? (
                      <p className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">{suggestion.description}</p>
                    ) : null}
                    <div className="mt-3 flex gap-2">
                      <Button size="xs" onClick={() => onAccept(suggestion.id)} disabled={isMutating}>
                        <Tick01Icon className="h-3 w-3" />
                        Approve
                      </Button>
                      <Button size="xs" variant="ghost" onClick={() => onDismiss(suggestion.id)} disabled={isMutating}>
                        Dismiss
                      </Button>
                    </div>
                  </div>
                </div>
              </div>
            );
          })
        )}
      </CardContent>
    </Card>
  );
}

function HealthRow({ healthScore }: { healthScore: CRMDealHealthScore }) {
  const tone = healthTone(healthScore.score);
  const factors = Object.entries(healthScore.factors ?? {})
    .map(([key, value]) => [key, formatFactorValue(value)] as const)
    .filter((entry): entry is readonly [string, string] => entry[1] !== null)
    .slice(0, 2);

  return (
    <div className="rounded-lg border p-3">
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-medium">Deal {healthScore.deal_id.slice(0, 8)}</p>
          <p className="mt-1 text-xs text-muted-foreground">{timeAgo(healthScore.calculated_at)}</p>
        </div>
        <div className="flex min-w-24 items-center justify-end gap-2">
          <div className="h-2 w-20 overflow-hidden rounded-full bg-muted">
            <div className={cn('h-full rounded-full', progressClasses(tone))} style={{ width: `${healthScore.score}%` }} />
          </div>
          <span className="w-7 text-right text-xs font-semibold tabular-nums">{healthScore.score}</span>
        </div>
      </div>
      {factors.length > 0 ? (
        <div className="mt-3 flex flex-wrap gap-1.5">
          {factors.map(([key, value]) => (
            <Badge key={key} variant="secondary" className="max-w-full truncate text-[11px] font-normal">
              {formatFactorName(key)}: {value}
            </Badge>
          ))}
        </div>
      ) : null}
    </div>
  );
}

export function InsightsPage() {
  useTitle('CRM Insights');
  const navigate = useNavigate();
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const { data: signalsData, isLoading: signalsLoading } = useBuyerSignals(wsId, {});
  const { data: healthData, isLoading: healthLoading } = useHealthScores(wsId);
  const { data: suggestionsData, isLoading: suggestionsLoading } = usePendingSuggestions(wsId);
  const acceptSuggestion = useAcceptSuggestion(wsId);
  const dismissSuggestion = useDismissSuggestion(wsId);

  const signals = signalsData?.data ?? [];
  const healthScores = healthData?.data ?? [];
  const suggestions = useMemo(
    () => [...(suggestionsData?.data ?? [])].sort((a, b) => b.confidence - a.confidence),
    [suggestionsData],
  );
  const signalStats = useMemo(() => {
    const recentHighConfidence = signals.filter((signal) => signal.confidence >= 0.8).length;
    const avgConfidence = signals.length
      ? Math.round((signals.reduce((sum, signal) => sum + signal.confidence, 0) / signals.length) * 100)
      : 0;
    const linkedThreads = signals.filter((signal) => !!signal.source_thread_id).length;
    return { recentHighConfidence, avgConfidence, linkedThreads };
  }, [signals]);
  const healthStats = useMemo(() => {
    const atRisk = healthScores.filter((score) => score.score < 40).length;
    const healthy = healthScores.filter((score) => score.score >= 70).length;
    return { atRisk, healthy };
  }, [healthScores]);
  const isLoadingIntelligence = signalsLoading || healthLoading || suggestionsLoading;
  const hasAnyIntelligence = signals.length > 0 || suggestions.length > 0 || healthScores.length > 0;

  const handleAccept = (id: string) => {
    acceptSuggestion.mutate(
      { id },
      {
        onSuccess: () => toast.success('Suggestion approved'),
        onError: (error) => toast.error(`Failed to approve: ${error.message}`),
      },
    );
  };

  const handleDismiss = (id: string) => {
    dismissSuggestion.mutate(id, {
      onSuccess: () => toast.success('Suggestion dismissed'),
      onError: (error) => toast.error(`Failed to dismiss: ${error.message}`),
    });
  };

  const goToReview = () => {
    if (!wsSlug) return;
    void navigate({ to: '/w/$slug/crm/review', params: { slug: wsSlug } });
  };

  const goToEmailSettings = () => {
    if (!wsSlug) return;
    void navigate({ to: '/w/$slug/settings/crm-email', params: { slug: wsSlug } });
  };

  const goToImportSettings = () => {
    if (!wsSlug) return;
    void navigate({ to: '/w/$slug/settings/import', params: { slug: wsSlug } });
  };

  const goToAutonomySettings = () => {
    if (!wsSlug) return;
    void navigate({ to: '/w/$slug/settings/crm-autonomy', params: { slug: wsSlug } });
  };

  const goToContacts = () => {
    if (!wsSlug) return;
    void navigate({ to: '/w/$slug/crm/contacts', params: { slug: wsSlug } });
  };

  return (
    <div className="h-full overflow-auto">
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-4 px-4 py-4 md:px-6">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="text-xl font-semibold tracking-tight">Insights</h1>
            <p className="text-sm text-muted-foreground">Buyer signals, suggested actions, and deal health in one place.</p>
          </div>
          {suggestions.length > 0 ? (
            <Button variant="outline" size="sm" onClick={goToReview} disabled={!wsSlug}>
              <SparklesIcon className="h-4 w-4" />
              Review suggestions
            </Button>
          ) : (
            <Button variant="outline" size="sm" onClick={goToEmailSettings} disabled={!wsSlug}>
              <Mail01Icon className="h-4 w-4" />
              CRM email settings
            </Button>
          )}
        </div>

        {!isLoadingIntelligence && !hasAnyIntelligence ? (
          <IntelligenceReadinessState
            onConnectEmail={goToEmailSettings}
            onImportRecords={goToImportSettings}
            onTuneThresholds={goToAutonomySettings}
            onOpenContacts={goToContacts}
          />
        ) : (
          <>
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <StatTile
                label="Buyer signals"
                value={signalsLoading ? '-' : signals.length}
                detail={`${signalStats.recentHighConfidence} high confidence`}
                icon={Activity01Icon}
                tone="accent"
              />
              <StatTile
                label="Pending review"
                value={suggestionsLoading ? '-' : suggestions.length}
                detail={suggestions.length === 1 ? '1 suggested action' : `${suggestions.length} suggested actions`}
                icon={SparklesIcon}
                tone={suggestions.length > 0 ? 'warn' : 'neutral'}
              />
              <StatTile
                label="Deal health"
                value={healthLoading ? '-' : healthScores.length}
                detail={`${healthStats.healthy} healthy, ${healthStats.atRisk} at risk`}
                icon={FavouriteIcon}
                tone={healthStats.atRisk > 0 ? 'danger' : healthScores.length > 0 ? 'good' : 'neutral'}
              />
              <StatTile
                label="Signal quality"
                value={signalsLoading || signals.length === 0 ? '-' : `${signalStats.avgConfidence}%`}
                detail={signals.length === 0 ? 'Waiting for signals' : `${signalStats.linkedThreads} linked threads`}
                icon={CheckmarkCircle02Icon}
                tone={signals.length > 0 ? 'good' : 'neutral'}
              />
            </div>

            <div className="grid min-h-0 gap-4 xl:grid-cols-[minmax(0,1.55fr)_minmax(340px,0.85fr)]">
          <Card className="rounded-lg">
            <CardHeader className="pb-0">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <CardTitle className="flex items-center gap-2 text-base">
                  <Activity01Icon className="h-4 w-4 text-muted-foreground" />
                  Recent Buyer Signals
                </CardTitle>
                {signals.length > 0 ? (
                  <Badge variant="secondary" className="font-normal">
                    Latest {Math.min(signals.length, 12)}
                  </Badge>
                ) : null}
              </div>
            </CardHeader>
            <CardContent>
              {signalsLoading ? (
                <div className="py-10 text-center text-sm text-muted-foreground">Loading buyer signals...</div>
              ) : signals.length === 0 ? (
                <EmptyState
                  icon={Activity01Icon}
                  title="No buyer signals detected yet"
                  description="Email, meeting, support, and CRM activity will produce signal cards as the workspace gathers sales context."
                />
              ) : (
                <div className="space-y-2">
                  {signals.slice(0, 12).map((signal) => (
                    <BuyerSignalRow key={signal.id} signal={signal} />
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          <div className="space-y-4">
            <ReviewQueue
              suggestions={suggestions}
              isLoading={suggestionsLoading}
              hasSignals={signals.length > 0}
              onAccept={handleAccept}
              onDismiss={handleDismiss}
              isMutating={acceptSuggestion.isPending || dismissSuggestion.isPending}
              onViewAll={goToReview}
            />

            <Card className="rounded-lg">
              <CardHeader className="pb-0">
                <CardTitle className="flex items-center gap-2 text-base">
                  <FavouriteIcon className="h-4 w-4 text-muted-foreground" />
                  Deal Health
                </CardTitle>
              </CardHeader>
              <CardContent>
                {healthLoading ? (
                  <div className="py-6 text-sm text-muted-foreground">Loading health scores...</div>
                ) : healthScores.length === 0 ? (
                  <EmptyState
                    icon={FavouriteIcon}
                    title={signals.length > 0 ? 'Health scores need deal activity' : 'No deal health scores yet'}
                    description={
                      signals.length > 0
                        ? 'Buyer signals are flowing, but deal health needs linked deals, activity history, and progression signals.'
                        : 'Scores will appear after deals have enough activity, signals, and progression history.'
                    }
                  />
                ) : (
                  <div className="space-y-2">
                    {healthScores.slice(0, 8).map((healthScore) => (
                      <HealthRow key={healthScore.id} healthScore={healthScore} />
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>

            <Card className="rounded-lg">
              <CardHeader className="pb-0">
                <CardTitle className="flex items-center gap-2 text-base">
                  <Search01Icon className="h-4 w-4 text-muted-foreground" />
                  Find CRM Records
                </CardTitle>
              </CardHeader>
              <CardContent>
                <CRMSearchResults workspaceId={wsId} />
              </CardContent>
            </Card>
          </div>
        </div>
          </>
        )}
      </div>
    </div>
  );
}
