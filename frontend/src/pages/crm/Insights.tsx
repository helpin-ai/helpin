import { useMemo } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { CRMSearchResults } from '@/components/crm/CRMSearchResults';
import { SignalWorkspaceFeed } from '@/components/crm/SignalWorkspaceFeed';
import { SuggestionCard } from '@/components/crm/SuggestionCard';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Activity01Icon,
  ChartIncreaseIcon,
  CheckmarkCircle02Icon,
  FavouriteIcon,
  Mail01Icon,
  Search01Icon,
  SparklesIcon,
} from '@/lib/icons';
import {
  useAcceptSuggestion,
  useCRMSignals,
  useDismissSuggestion,
  useHealthScores,
  usePendingSuggestions,
} from '@/hooks/queries/useCRM';
import { useTitle } from '@/hooks/useTitle';
import { cn, timeAgo } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CRMDealHealthScore, CRMSignalDismissalReason, CRMSuggestion } from '@/lib/crmTypes';
import { QuietPageHeader } from '@/components/design-system/quiet';

type Tone = 'neutral' | 'good' | 'warn' | 'danger' | 'accent';

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
            Insights appear after Helpin has CRM activity to analyze. Connect email or add CRM records, then CRM signals,
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
            <p className="text-sm font-medium">What creates CRM signals</p>
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
  onDismiss: (id: string, reason: CRMSignalDismissalReason) => void;
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
                : 'Suggested actions will appear after CRM signals point to a deal, follow-up, enrichment, or risk worth reviewing.'
            }
          />
        ) : (
          suggestions.slice(0, 4).map((suggestion) => <SuggestionCard key={suggestion.id} suggestion={suggestion} onAccept={onAccept} onDismiss={onDismiss} isAccepting={isMutating} isDismissing={isMutating} compact />)
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
  useTitle('CRM Signals');
  const navigate = useNavigate();
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const { data: signalsData, isLoading: signalsLoading } = useCRMSignals(wsId, {});
  const { data: healthData, isLoading: healthLoading } = useHealthScores(wsId);
  const { data: suggestionsData, isLoading: suggestionsLoading } = usePendingSuggestions(wsId);
  const acceptSuggestion = useAcceptSuggestion(wsId);
  const dismissSuggestion = useDismissSuggestion(wsId);

  const signals = useMemo(() => signalsData?.data ?? [], [signalsData?.data]);
  const healthScores = useMemo(() => healthData?.data ?? [], [healthData?.data]);
  const suggestions = useMemo(
    () => [...(suggestionsData?.data ?? [])].sort((a, b) => b.confidence - a.confidence),
    [suggestionsData],
  );
  const signalStats = useMemo(() => {
    const highPriority = signals.filter((signal) => signal.severity === 'high').length;
    const verified = signals.filter((signal) => signal.evidence_identity_trust === 'verified').length;
    const verifiedShare = signals.length ? Math.round((verified / signals.length) * 100) : 0;
    const linkedSources = signals.filter((signal) => !!signal.source_thread_id || !!signal.source_id).length;
    return { highPriority, verifiedShare, linkedSources };
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

  const handleDismiss = (id: string, reason: CRMSignalDismissalReason) => {
    dismissSuggestion.mutate({ id, reason }, {
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
    <div className="flex h-full min-h-0 flex-col">
      <QuietPageHeader
        variant="shell"
        title="Signal inbox"
        description="See what changed, why it matters, and what to do next."
        actions={suggestions.length > 0 ? (
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
      />

      <div className="min-h-0 flex-1 overflow-auto">
        <div className="mx-auto flex w-full max-w-7xl flex-col gap-4 px-4 py-4 md:px-6">

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
                label="CRM signals"
                value={signalsLoading ? '-' : signalsData?.total ?? 0}
                detail={`${signalStats.highPriority} high priority in recent results`}
                icon={Activity01Icon}
                tone="accent"
              />
              <StatTile
                label="Pending review"
                value={suggestionsLoading ? '-' : suggestionsData?.total ?? 0}
                detail={(suggestionsData?.total ?? 0) === 1 ? '1 recommended action' : `${suggestionsData?.total ?? 0} recommended actions`}
                icon={SparklesIcon}
                tone={suggestions.length > 0 ? 'warn' : 'neutral'}
              />
              <StatTile
                label="Deal health"
                value={healthLoading ? '-' : healthData?.total ?? 0}
                detail={`${healthStats.healthy} healthy, ${healthStats.atRisk} at risk`}
                icon={FavouriteIcon}
                tone={healthStats.atRisk > 0 ? 'danger' : healthScores.length > 0 ? 'good' : 'neutral'}
              />
              <StatTile
                label="Verified evidence"
                value={signalsLoading || signals.length === 0 ? '-' : `${signalStats.verifiedShare}%`}
                detail={signals.length === 0 ? 'Waiting for signals' : `${signalStats.linkedSources} exact sources in recent results`}
                icon={CheckmarkCircle02Icon}
                tone={signals.length > 0 ? 'good' : 'neutral'}
              />
            </div>

            <div className="grid min-h-0 gap-4 xl:grid-cols-[minmax(0,1.55fr)_minmax(340px,0.85fr)]">
              <div className="min-w-0 rounded-lg border bg-card px-4 py-4 sm:px-5">
                <SignalWorkspaceFeed workspaceId={wsId} />
              </div>

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
                            ? 'CRM signals are flowing, but deal health needs linked deals, activity history, and progression signals.'
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
    </div>
  );
}
