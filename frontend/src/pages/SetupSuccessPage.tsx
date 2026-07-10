import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { AlertCircle, ArrowRight, CheckCircle2, ChevronDown, Circle, Lock, Sparkles } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useSetup } from '@/hooks/queries';
import { resolveSetupAction } from '@/lib/setupActions';
import { trackAnalyticsEvent } from '@/lib/analytics';
import { setupService } from '@/lib/services/setupService';
import type { SetupJourney, SetupTask } from '@/lib/setupTypes';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';

function defaultExpandedJourney(journeys: SetupJourney[]) {
  const actionable = journeys.find((journey) => journey.tasks.some((task) =>
    task.status === 'available' || task.status === 'needs_attention',
  ));
  if (actionable) return actionable.key;
  return journeys.find((journey) => journey.tasks.some((task) => task.status === 'blocked'))?.key;
}

export function SetupSuccessPage() {
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const slug = workspace?.slug ?? '';
  const setup = useSetup(workspaceId);
  const trackedWorkspace = useRef<string | null>(null);
  const expansionWorkspace = useRef('');
  const expansionInitialized = useRef(false);
  const [expandedJourneys, setExpandedJourneys] = useState<Record<string, boolean>>({});

  const percent = useMemo(() => {
    if (!setup.data?.total_count) return 0;
    return Math.round((setup.data.completed_count / setup.data.total_count) * 100);
  }, [setup.data]);

  useEffect(() => {
    if (!setup.data || !workspaceId || trackedWorkspace.current === workspaceId) return;
    trackedWorkspace.current = workspaceId;
    trackAnalyticsEvent('setup_guide_viewed', {
      workspace_id: workspaceId,
      goals: setup.data.goals,
      completed_count: setup.data.completed_count,
      total_count: setup.data.total_count,
      recommended_task_key: setup.data.recommended?.task_key,
    });
  }, [setup.data, workspaceId]);

  useEffect(() => {
    const journeyKeys = setup.data?.journeys.map((journey) => journey.key) ?? [];
    const workspaceChanged = expansionWorkspace.current !== workspaceId;
    if (workspaceChanged) {
      expansionWorkspace.current = workspaceId;
      expansionInitialized.current = false;
    }
    if (!workspaceId || journeyKeys.length === 0) {
      if (workspaceChanged) setExpandedJourneys({});
      return;
    }
    setExpandedJourneys((current) => {
      if (!expansionInitialized.current) {
        expansionInitialized.current = true;
        const defaultKey = defaultExpandedJourney(setup.data?.journeys ?? []);
        return Object.fromEntries(journeyKeys.map((key) => [key, key === defaultKey]));
      }
      return Object.fromEntries(journeyKeys.map((key) => [key, current[key] ?? false]));
    });
  }, [setup.data?.journeys, workspaceId]);

  useEffect(() => {
    const navigateToJourney = (event: Event) => {
      const journeyKey = (event as CustomEvent<{ journeyKey?: string }>).detail?.journeyKey;
      if (!journeyKey) return;
      setExpandedJourneys((current) => ({ ...current, [journeyKey]: true }));
      requestAnimationFrame(() => {
        document.getElementById(`setup-journey-${journeyKey}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
      });
    };
    window.addEventListener('setup-journey-navigate', navigateToJourney);
    return () => window.removeEventListener('setup-journey-navigate', navigateToJourney);
  }, []);

  const runAction = (key: string, taskKey?: string) => {
    const destination = resolveSetupAction(key, slug);
    if (!destination) {
      toast.info('This action is not available in your workspace yet.');
      return;
    }
    if (taskKey) {
      void setupService.startRecommendation(workspaceId, taskKey);
      trackAnalyticsEvent('setup_recommendation_started', {
        workspace_id: workspaceId,
        task_key: taskKey,
        action_key: key,
      });
    }
    void navigate({ to: destination });
  };

  const toggleJourney = (journeyKey: string) => {
    setExpandedJourneys((current) => ({ ...current, [journeyKey]: !current[journeyKey] }));
  };

  if (setup.isLoading) return <SetupLoading />;
  if (setup.isError || !setup.data) {
    return (
      <div className="mx-auto flex min-h-[60vh] max-w-xl flex-col items-center justify-center px-6 text-center">
        <p className="text-lg font-semibold">We couldn’t load your success guide.</p>
        <p className="mt-2 text-sm text-muted-foreground">Your product data is safe. Try loading the guide again.</p>
        <Button className="mt-5" variant="outline" onClick={() => setup.refetch()}>Try again</Button>
      </div>
    );
  }

  const view = setup.data;
  const recommendedDestination = view.recommended ? resolveSetupAction(view.recommended.action.key, slug) : undefined;

  return (
    <main className="h-full overflow-y-auto bg-background">
      <div className="mx-auto max-w-6xl px-5 pb-24 pt-8 sm:px-8 sm:pb-28 lg:pb-32 lg:pt-12">
        <header className="max-w-3xl">
          <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-primary">
            <Sparkles className="h-3.5 w-3.5" /> Setup & success
          </div>
          <h1 className="mt-4 text-3xl font-semibold tracking-tight sm:text-4xl">Get your workspace set up for success.</h1>
          <p className="mt-3 max-w-2xl text-sm leading-6 text-muted-foreground sm:text-base">
            Start with the basics, build a workflow that sticks, and automate repeat work when you’re ready.
          </p>
          <div className="mt-7 flex max-w-2xl items-center gap-4">
            <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
              <div className="h-full rounded-full bg-primary transition-[width] duration-500 motion-reduce:transition-none" style={{ width: `${percent}%` }} />
            </div>
            <span className="w-28 text-right text-xs tabular-nums text-muted-foreground">{view.completed_count} of {view.total_count} verified</span>
          </div>
        </header>

        <section className="relative mt-10 overflow-hidden rounded-2xl border border-primary/20 bg-primary/[0.035] p-6 shadow-sm sm:p-8">
          {view.recommended ? (
            <div className="relative flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
              <div className="max-w-2xl">
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-primary">Recommended next step</p>
                <h2 className="mt-3 text-2xl font-semibold tracking-tight">{view.recommended.title}</h2>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">{view.recommended.reason}</p>
              </div>
              <Button size="lg" className="shrink-0 gap-2 rounded-full px-6" disabled={!recommendedDestination} onClick={() => runAction(view.recommended!.action.key, view.recommended!.task_key)}>
                {view.recommended.action.label}<ArrowRight className="h-4 w-4" />
              </Button>
            </div>
          ) : (
            <div className="relative flex items-center gap-4">
              <span className="flex h-11 w-11 items-center justify-center rounded-full bg-emerald-500/15 text-emerald-600"><CheckCircle2 className="h-6 w-6" /></span>
              <div><h2 className="text-xl font-semibold">Your active journeys are complete.</h2><p className="mt-1 text-sm text-muted-foreground">Keep using the workflows—new power steps will appear as the platform grows.</p></div>
            </div>
          )}
        </section>

        <div className="mt-14 space-y-14">
          {view.journeys.map((journey) => (
            <JourneySection
              key={journey.key}
              journey={journey}
              expanded={Boolean(expandedJourneys[journey.key])}
              onToggle={() => toggleJourney(journey.key)}
              onAction={runAction}
            />
          ))}
        </div>

      </div>
    </main>
  );
}

function JourneySection({ journey, expanded, onToggle, onAction }: { journey: SetupJourney; expanded: boolean; onToggle: () => void; onAction: (key: string, taskKey?: string) => void }) {
  const taskListId = `setup-journey-tasks-${journey.key}`;
  return (
    <section id={`setup-journey-${journey.key}`} className="scroll-mt-6">
      <button
        type="button"
        className="group/journey flex w-full flex-col gap-4 border-b pb-5 text-left outline-none transition-colors hover:border-foreground/25 focus-visible:rounded-md focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-4 sm:flex-row sm:items-end sm:justify-between"
        aria-expanded={expanded}
        aria-controls={taskListId}
        onClick={onToggle}
      >
        <span className="max-w-2xl">
          <span className="block text-xl font-semibold">{journey.title}</span>
          <span className="mt-1 block text-sm leading-6 text-muted-foreground">{journey.description}</span>
        </span>
        <span className="flex shrink-0 items-center gap-3 text-xs tabular-nums text-muted-foreground">
          {journey.completed_count}/{journey.total_count} complete
          <ChevronDown className={cn('h-4 w-4 transition-transform duration-200 motion-reduce:transition-none', expanded && 'rotate-180')} aria-hidden="true" />
        </span>
      </button>
      <ol id={taskListId} className="divide-y" hidden={!expanded}>
        {journey.tasks.map((task, index) => <JourneyTaskRow key={task.key} task={task} index={index} onAction={onAction} />)}
      </ol>
    </section>
  );
}

function JourneyTaskRow({ task, index, onAction }: { task: SetupTask; index: number; onAction: (key: string, taskKey?: string) => void }) {
  const completed = task.status === 'completed';
  const needsAttention = task.status === 'needs_attention';
  const blocked = task.status === 'blocked';
  const stateHint = blocked ? task.blocked_reason : needsAttention ? 'This setup item needs attention.' : undefined;
  return (
    <li className="group flex gap-4 py-5 sm:gap-6">
      <div className="flex w-8 shrink-0 justify-center pt-0.5" aria-hidden="true">{completed ? <CheckCircle2 className="h-5 w-5 text-emerald-500" /> : needsAttention ? <AlertCircle className="h-5 w-5 text-amber-500" /> : blocked ? <Lock className="h-4 w-4 text-muted-foreground/55" /> : <Circle className="h-5 w-5 text-muted-foreground/45" />}</div>
      <div className="min-w-0 flex-1 sm:flex sm:items-center sm:justify-between sm:gap-6">
        <div className="min-w-0"><div className="flex flex-wrap items-center gap-2"><p className={cn('text-sm font-medium leading-5 break-words', completed && 'text-muted-foreground')}>{task.title}</p>{!task.shared && <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">Your step</span>}{blocked && stateHint && <span className="rounded-full bg-amber-500/10 px-2 py-0.5 text-[10px] font-medium text-amber-700 dark:text-amber-300" title={stateHint} aria-label={stateHint}>Blocked</span>}</div></div>
        {!completed && !blocked && task.action && <Button variant="ghost" size="sm" className="mt-3 h-8 shrink-0 gap-1.5 px-0 text-xs hover:bg-transparent sm:mt-0 sm:px-3" onClick={() => onAction(task.action!.key, task.key)}>{task.action.label}<ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5 motion-reduce:transition-none" /></Button>}
        {completed && <span className="mt-2 block shrink-0 text-xs text-emerald-600 sm:mt-0">Verified</span>}
        {needsAttention && <span className="mt-2 block shrink-0 text-xs text-amber-600 sm:mt-0">Needs attention</span>}
      </div>
      <span className="sr-only">Step {index + 1}</span>
    </li>
  );
}

function SetupLoading() {
  return <div className="mx-auto max-w-6xl space-y-8 px-5 py-10 sm:px-8"><Skeleton className="h-8 w-72" /><Skeleton className="h-5 w-[32rem] max-w-full" /><Skeleton className="h-40 w-full rounded-2xl" /><Skeleton className="h-80 w-full" /></div>;
}
