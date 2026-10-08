import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { AlertCircleIcon, ArrowDown01Icon, ArrowRight01Icon, CheckmarkCircle02Icon, CircleIcon, LockIcon } from '@/lib/icons';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { systemStatusEnabled } from '@edition/config';
import { workspaceSidebarSafeInsetClassName } from '@/components/design-system/quiet';
import { SampleDataCard } from '@/components/setup/SampleDataButton';
import { SetupSettingsLink } from '@/components/setup/CapabilityActions';
import { SupportSetupAssistant } from '@/components/setup/SupportSetupAssistant';
import { GitHubReturnNotice } from '@/components/setup/GitHubReturnNotice';
import { useGitHubReturnResult } from '@/hooks/useGitHubReturnResult';
import { blockingServices } from '@/components/setup/capabilityPresentation';
import { SetupTaskRequirement } from '@/components/setup/SetupTaskRequirement';
import { journeyTaskRequirements, type SetupTaskRequirementKind } from '@/components/setup/setupTaskRequirements';
import { usePermissions, useSetup, useUpdateSetupGoals, useWorkspaceAccess, useWorkspaceCapabilities } from '@/hooks/queries';
import { resolveSetupAction } from '@/lib/setupActions';
import { trackAnalyticsEvent } from '@/lib/analytics';
import { setupService } from '@/lib/services/setupService';
import type { CapabilitiesResponse } from '@/lib/capabilityTypes';
import type { SetupGoalKey, SetupJourney, SetupTask } from '@/lib/setupTypes';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';

function defaultExpandedJourney(journeys: SetupJourney[]) {
  const actionable = journeys.find((journey) => journey.tasks.some((task) =>
    task.status === 'available' || task.status === 'needs_attention',
  ));
  if (actionable) return actionable.key;
  return journeys.find((journey) => journey.tasks.some((task) => task.status === 'blocked'))?.key;
}

const SETUP_GOAL_OPTIONS: Array<{ key: SetupGoalKey; label: string }> = [
  { key: 'product_delivery', label: 'Plan and ship team projects' },
  { key: 'customer_support', label: 'Scale customer support' },
  { key: 'help_center_docs', label: 'Publish help center docs' },
  { key: 'internal_docs', label: 'Build internal knowledge' },
  { key: 'sales_crm', label: 'Build a sales pipeline' },
  { key: 'automation_mastery', label: 'Automate repeatable work' },
];

export function SetupSuccessPage() {
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const slug = workspace?.slug ?? '';
  const setup = useSetup(workspaceId);
  const updateGoals = useUpdateSetupGoals(workspaceId);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { has } = usePermissions(access);
  const capabilities = useWorkspaceCapabilities(workspaceId);
  const githubReturn = useGitHubReturnResult();
  const trackedWorkspace = useRef<string | null>(null);
  const expansionWorkspace = useRef('');
  const expansionInitialized = useRef(false);
  const [expandedJourneys, setExpandedJourneys] = useState<Record<string, boolean>>({});
  const [goalEditorOpen, setGoalEditorOpen] = useState(false);
  const [draftGoals, setDraftGoals] = useState<SetupGoalKey[]>([]);

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

  const openGoalEditor = () => {
    setDraftGoals(setup.data?.goals ?? []);
    setGoalEditorOpen(true);
  };

  const toggleGoal = (key: SetupGoalKey) => {
    setDraftGoals((current) => {
      if (current.includes(key)) return current.filter((goal) => goal !== key);
      return [...current, key];
    });
  };

  const saveGoals = () => {
    if (draftGoals.length === 0) {
      toast.info('Choose at least one goal.');
      return;
    }
    updateGoals.mutate(draftGoals, { onSuccess: () => setGoalEditorOpen(false) });
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
  const canManage = has('workspace.update');
  const taskContext: TaskRequirementContext = {
    capabilities: capabilities.data,
    workspaceId,
    slug,
    canManage,
    isOwner: access?.membership?.role === 'owner',
  };
  const blockedServiceCount = canManage && systemStatusEnabled ? blockingServices(capabilities.data).length : 0;

  return (
    <main className="h-full overflow-y-auto bg-background">
      <div className="mx-auto max-w-5xl px-5 pb-24 pt-6 sm:px-8 sm:pb-28 lg:pt-8">
        <header className={cn('flex flex-wrap items-end justify-between gap-x-6 gap-y-3 border-b border-quiet-divider-strong pb-4', workspaceSidebarSafeInsetClassName)}>
          <div className="min-w-0">
            <h1 className="text-[20px] font-semibold tracking-[-0.018em] text-quiet-text-primary">Setup guide</h1>
            <div className="mt-2 flex items-center gap-3">
              <div className="h-0.5 w-32 overflow-hidden bg-quiet-divider-strong" aria-hidden="true">
                <div className="h-full bg-quiet-positive transition-[width] duration-500 motion-reduce:transition-none" style={{ width: `${percent}%` }} />
              </div>
              <span className="text-[12px] tabular-nums text-quiet-text-tertiary">{view.completed_count} of {view.total_count} core steps verified</span>
            </div>
          </div>
          <div className="flex items-center gap-4">
            {has('support.admin') && view.journeys.some((journey) => journey.key === 'customer_support') && (
              <SupportSetupAssistant workspaceId={workspaceId} slug={slug} />
            )}
            {has('workspace.update') && (
              <button type="button" className={quietTextAction} onClick={openGoalEditor}>Edit goals</button>
            )}
          </div>
        </header>

        <section className="border-b border-quiet-divider-light py-4" aria-label={view.recommended ? 'Recommended next step' : 'Setup complete'}>
          {view.recommended ? (
            <div className="flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-6">
              <div className="min-w-0 flex-1">
                <p className="text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">{percent >= 100 ? 'Next value step' : 'Recommended next step'}</p>
                <h2 className="mt-1 text-[13.5px] font-semibold leading-5 tracking-[-0.008em] text-quiet-text-primary">{view.recommended.title}</h2>
                <p className="text-[12.5px] leading-5 text-quiet-text-secondary">{view.recommended.reason}</p>
              </div>
              <Button size="sm" className="shrink-0 gap-1.5" disabled={!recommendedDestination} onClick={() => runAction(view.recommended!.action.key, view.recommended!.task_key)}>
                {view.recommended.action.label}<ArrowRight01Icon className="h-3.5 w-3.5" aria-hidden="true" />
              </Button>
            </div>
          ) : (
            <div className="flex items-start gap-3">
              <CheckmarkCircle02Icon className="mt-0.5 h-4 w-4 shrink-0 text-quiet-positive" aria-hidden="true" />
              <div>
                <h2 className="text-[13.5px] font-semibold leading-5 tracking-[-0.008em] text-quiet-text-primary">Your active journeys are complete.</h2>
                <p className="text-[12.5px] leading-5 text-quiet-text-secondary">Keep using the workflows—new power steps will appear as the platform grows.</p>
              </div>
            </div>
          )}
        </section>

        <SampleDataCard className="border-b border-quiet-divider-light py-4" workspaceId={workspaceId} canManage={canManage} />

        <GitHubReturnNotice result={githubReturn} className="pt-4" />

        {blockedServiceCount > 0 && (
          <div className="relative mt-6 py-1 pl-4" role="note" data-testid="required-services-notice">
            <span aria-hidden="true" className="absolute inset-y-0 left-0 w-[3px] bg-quiet-accent" />
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <p className="text-sm text-quiet-text-primary">
                {blockedServiceCount === 1 ? 'A required server service needs attention.' : `${blockedServiceCount} required server services need attention.`}
              </p>
              <SetupSettingsLink slug={slug} path="settings/system-status">Open System status</SetupSettingsLink>
            </div>
          </div>
        )}

        <div className="mt-9 space-y-9">
          {view.journeys.map((journey) => (
            <JourneySection
              key={journey.key}
              journey={journey}
              expanded={Boolean(expandedJourneys[journey.key])}
              onToggle={() => toggleJourney(journey.key)}
              onAction={runAction}
              taskContext={taskContext}
            />
          ))}
        </div>

      </div>
      <Dialog open={goalEditorOpen} onOpenChange={setGoalEditorOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Edit setup goals</DialogTitle>
            <DialogDescription>Select everything you want to set up. The first goal leads your guide; workspace essentials and recommended automation remain available.</DialogDescription>
          </DialogHeader>
          <div className="divide-y rounded-lg border">
            {SETUP_GOAL_OPTIONS.map((option) => {
              const id = `setup-goal-${option.key}`;
              return (
                <Label key={option.key} htmlFor={id} className="flex cursor-pointer items-center gap-3 px-3 py-3 text-sm font-medium">
                  <Checkbox id={id} checked={draftGoals.includes(option.key)} onCheckedChange={() => toggleGoal(option.key)} />
                  <span>{option.label}</span>
                </Label>
              );
            })}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setGoalEditorOpen(false)}>Cancel</Button>
            <Button onClick={saveGoals} disabled={updateGoals.isPending}>{updateGoals.isPending ? 'Saving…' : 'Save goals'}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </main>
  );
}

type TaskRequirementContext = {
  capabilities?: CapabilitiesResponse;
  workspaceId: string;
  slug: string;
  canManage: boolean;
  isOwner: boolean;
};

function JourneySection({ journey, expanded, onToggle, onAction, taskContext }: { journey: SetupJourney; expanded: boolean; onToggle: () => void; onAction: (key: string, taskKey?: string) => void; taskContext: TaskRequirementContext }) {
  const taskListId = `setup-journey-tasks-${journey.key}`;
  const requirements = journeyTaskRequirements(journey.tasks, taskContext.capabilities);
  return (
    <section id={`setup-journey-${journey.key}`} className="scroll-mt-6">
      <button
        type="button"
        className="group/journey flex w-full flex-col gap-2 border-b border-quiet-divider-strong pb-3 text-left outline-none focus-visible:rounded-[6px] focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-quiet-text-tertiary sm:flex-row sm:items-end sm:justify-between sm:gap-6"
        aria-expanded={expanded}
        aria-controls={taskListId}
        onClick={onToggle}
      >
        <span className="max-w-2xl">
          <span className="block text-[15px] font-semibold tracking-[-0.012em] text-quiet-text-primary">{journey.title}</span>
          <span className="mt-0.5 block text-[12.5px] leading-5 text-quiet-text-secondary">{journey.description}</span>
        </span>
        <span className="flex shrink-0 items-center gap-2 text-[12px] tabular-nums text-quiet-text-tertiary transition-colors group-hover/journey:text-quiet-text-secondary">
          {journey.completed_count}/{journey.total_count} core complete
          <ArrowDown01Icon className={cn('h-[15px] w-[15px] transition-transform duration-200 motion-reduce:transition-none', expanded && 'rotate-180')} aria-hidden="true" />
        </span>
      </button>
      <ol id={taskListId} className="divide-y divide-quiet-divider-light" hidden={!expanded}>
        {journey.tasks.map((task, index) => (
          <JourneyTaskRow key={task.key} task={task} index={index} onAction={onAction} requirement={requirements.get(task.key)} taskContext={taskContext} />
        ))}
      </ol>
    </section>
  );
}

function JourneyTaskRow({ task, index, onAction, requirement, taskContext }: { task: SetupTask; index: number; onAction: (key: string, taskKey?: string) => void; requirement?: SetupTaskRequirementKind; taskContext: TaskRequirementContext }) {
  const completed = task.status === 'completed';
  const needsAttention = task.status === 'needs_attention';
  const blocked = task.status === 'blocked';
  const stateHint = blocked ? task.blocked_reason : needsAttention ? 'This setup item needs attention.' : undefined;
  return (
    <li className="group flex gap-3 px-1 py-3 transition-colors hover:bg-quiet-row-hover">
      <div className="flex w-5 shrink-0 justify-center pt-0.5" aria-hidden="true">{completed ? <CheckmarkCircle02Icon className="h-4 w-4 text-quiet-positive" /> : needsAttention ? <AlertCircleIcon className="h-4 w-4 text-quiet-accent" /> : blocked ? <LockIcon className="h-4 w-4 text-quiet-empty-glyph" /> : <CircleIcon className="h-4 w-4 text-quiet-empty-glyph" />}</div>
      <div className="min-w-0 flex-1 sm:flex sm:items-center sm:justify-between sm:gap-6">
        <div className="min-w-0 sm:flex-1"><div className="flex flex-wrap items-baseline gap-x-2.5 gap-y-1"><p className={cn('break-words text-[13.5px] leading-5 tracking-[-0.008em]', completed ? 'font-medium text-quiet-text-tertiary' : 'font-semibold text-quiet-text-primary')}>{task.title}</p>{!task.shared && <span className={toneLabel('text-quiet-text-tertiary')}>Your step</span>}{blocked && stateHint && <span className={toneLabel('text-quiet-accent')} title={stateHint} aria-label={stateHint}>Blocked</span>}</div>{blocked && stateHint && <p className="mt-0.5 text-[12.5px] leading-5 text-quiet-text-tertiary">{stateHint}</p>}{requirement && taskContext.capabilities && (
          <SetupTaskRequirement
            kind={requirement}
            capabilities={taskContext.capabilities}
            workspaceId={taskContext.workspaceId}
            slug={taskContext.slug}
            canManage={taskContext.canManage}
            isOwner={taskContext.isOwner}
          />
        )}</div>
        {!completed && !blocked && task.action && <button type="button" className={cn(quietTextAction, 'mt-2 shrink-0 sm:mt-0')} onClick={() => onAction(task.action!.key, task.key)}>{task.action.label}<ArrowRight01Icon className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5 motion-reduce:transition-none" aria-hidden="true" /></button>}
        {completed && <span className={cn(toneLabel('text-quiet-positive'), 'mt-2 block shrink-0 sm:mt-0')}>Verified</span>}
        {needsAttention && <span className={cn(toneLabel('text-quiet-accent'), 'mt-2 block shrink-0 sm:mt-0')}>Needs attention</span>}
      </div>
      <span className="sr-only">Step {index + 1}</span>
    </li>
  );
}

/** Quiet text action: label plus optional icon, no border or fill. */
const quietTextAction = 'inline-flex items-center gap-1.5 rounded-[6px] text-[12.5px] font-medium text-quiet-text-secondary transition-colors hover:text-quiet-text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-quiet-text-tertiary';

/** Tone label: an uppercase meaning-colored word, not a pill. */
function toneLabel(color: string) {
  return cn('text-[11.5px] font-semibold uppercase tracking-[0.03em]', color);
}

function SetupLoading() {
  return <div className="mx-auto max-w-5xl space-y-6 px-5 py-8 sm:px-8"><Skeleton className="h-6 w-40" /><Skeleton className="h-3 w-56" /><Skeleton className="h-16 w-full" /><Skeleton className="h-80 w-full" /></div>;
}
