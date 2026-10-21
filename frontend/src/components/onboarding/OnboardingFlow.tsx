import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useQueryClient } from '@tanstack/react-query';
import {
  usePermissions,
  useSetup,
  useWorkspaceAccess,
  useWorkspaceBySlug,
  useWorkspaceCapabilities,
  useWorkspaces,
} from '@/hooks/queries';
import { useSetupGuideEnabled } from '@/hooks/useSetupGuideEnabled';
import { useTitle } from '@/hooks/useTitle';
import { Loading01Icon } from '@/lib/icons';
import { queryKeys } from '@/lib/queryKeys';
import type { SetupGoalKey } from '@/lib/setupTypes';
import type { Workspace } from '@/lib/types';
import {
  areEmailInvitesUnavailable,
  canGenerateCompanyContext,
  nextOnboardingStep,
  onboardingConditionsFromCapabilities,
  onboardingStepsFor,
  resolveGitHubStepInclusion,
  resolveOnboardingStep,
  shouldOfferAIStep,
} from '@/lib/workspaceOnboardingFlow';
import { workspaceDefaultsFromUserEmail } from '@/lib/workspaceOnboardingDefaults';
import { workspaceCreatedDestination, type WorkspaceOnboardingStep } from '@/lib/workspaceOnboardingMode';
import { useAuthStore } from '@/stores/authStore';
import { CompanyContextStep } from './CompanyContextStep';
import { ConnectAIStep } from './ConnectAIStep';
import { ConnectGitHubStep } from './ConnectGitHubStep';
import { FinishStep } from './FinishStep';
import { InviteStep } from './InviteStep';
import { OnboardingActions, OnboardingShell, OnboardingTextButton } from './OnboardingShell';
import { TeamsStep } from './TeamsStep';
import { WorkspaceStep } from './WorkspaceStep';
import { SetupMethodStep } from './SetupMethodStep';
import { WorkspaceSetupAssistantContent } from '@/components/setup/WorkspaceSetupAssistant';
import { Button } from '@/components/ui/button';

export type OnboardingFlowProps = {
  /** `step` from the URL. */
  step?: WorkspaceOnboardingStep;
  /** `workspace` slug from the URL, present once the workspace exists. */
  workspaceSlug?: string;
};

const headerLinkClassName = 'public-page-back rounded-sm underline-offset-4 hover:text-foreground hover:underline';

/**
 * Workspace onboarding for every edition, in the signup page shell:
 * workspace → setup method (UI or external assistant) → Connect AI (Community without AI) → Connect GitHub (optional, for
 * Community owners who plan and ship projects) → company context → teams →
 * invite → finish. The URL holds the step and, after creation, the workspace
 * slug, so reloads resume and Back never creates a second workspace.
 */
export function OnboardingFlow({ step, workspaceSlug }: OnboardingFlowProps) {
  useTitle('Onboarding');
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useAuthStore((state) => state.user);
  const signOut = useAuthStore((state) => state.signOut);
  const setupGuideEnabled = useSetupGuideEnabled();
  const { data: workspaces = [], isLoading: workspacesLoading } = useWorkspaces();
  const workspaceQuery = useWorkspaceBySlug(workspaceSlug ?? '');
  const workspace = workspaceSlug ? workspaceQuery.data ?? undefined : undefined;
  const capabilities = useWorkspaceCapabilities(workspace?.id);
  const accessQuery = useWorkspaceAccess(workspace?.id ?? '');
  const access = accessQuery.data;
  const { has } = usePermissions(access);
  const includeMethod = !setupGuideEnabled ? false : access
    ? has('workspace.update') || has('team.manage') || has('workspace.invites.manage') || has('support.admin')
    : accessQuery.isError ? false : undefined;
  // Once shown for a workspace, Connect AI and Connect GitHub stay in the flow even after they succeed.
  const [aiStepWorkspaceId, setAiStepWorkspaceId] = useState<string | null>(null);
  const [gitHubStepWorkspaceId, setGitHubStepWorkspaceId] = useState<string | null>(null);
  // Goals chosen on the workspace step of this visit; the Setup guide stores them for later visits.
  const [createdGoals, setCreatedGoals] = useState<{ workspaceId: string; goals: SetupGoalKey[] } | null>(null);
  const [createdTeamsState, setCreatedTeamsState] = useState<{ workspaceId: string; teams: { id: string; handle: string }[] } | null>(null);
  const createdTeams = createdTeamsState?.workspaceId === workspace?.id ? createdTeamsState?.teams ?? [] : [];
  const setup = useSetup(setupGuideEnabled ? workspace?.id : undefined);

  const conditions = onboardingConditionsFromCapabilities(capabilities.data);
  const capabilitiesSettled = capabilities.isSuccess || capabilities.isError;
  const includeAI = workspace
    ? capabilitiesSettled
      ? shouldOfferAIStep(conditions) || aiStepWorkspaceId === workspace.id
      : undefined
    : false;
  const includeGitHub = workspace
    ? resolveGitHubStepInclusion({
      workspaceId: workspace.id,
      rememberedWorkspaceId: gitHubStepWorkspaceId,
      requested: step === 'github',
      capabilitiesSettled,
      conditions,
      accessSettled: Boolean(access) || accessQuery.isError === true,
      role: access?.membership?.role,
      goals: createdGoals?.workspaceId === workspace.id
        ? createdGoals.goals
        : setupGuideEnabled
          ? setup.isSuccess ? setup.data?.goals ?? [] : setup.isError ? [] : undefined
          : [],
    })
    : false;
  const loadingWorkspace = Boolean(workspaceSlug) && workspaceQuery.isLoading;
  const missingWorkspace = Boolean(workspaceSlug) && workspaceQuery.isError;
  const current = loadingWorkspace
    ? undefined
    : resolveOnboardingStep({ requested: step, hasWorkspace: Boolean(workspace), includeAI, includeGitHub, includeMethod });

  useEffect(() => {
    if (loadingWorkspace || !current) return;
    const slug = missingWorkspace ? undefined : workspace?.slug;
    if (current !== step || slug !== workspaceSlug) {
      void navigate({ to: '/onboarding', search: { step: current, workspace: slug }, replace: true });
    }
  }, [current, loadingWorkspace, missingWorkspace, navigate, step, workspace?.slug, workspaceSlug]);

  // Remembered during render (not in an effect) so a successful connection,
  // which turns AI `ready`, doesn't bounce the person past their result.
  if (current === 'ai' && workspace && aiStepWorkspaceId !== workspace.id) {
    setAiStepWorkspaceId(workspace.id);
  }
  if (current === 'github' && workspace && gitHubStepWorkspaceId !== workspace.id) {
    setGitHubStepWorkspaceId(workspace.id);
  }

  const inclusions = { includeAI: Boolean(includeAI), includeGitHub: Boolean(includeGitHub), includeMethod: Boolean(includeMethod) };
  const goTo = (next: WorkspaceOnboardingStep) => {
    if (!workspace) return;
    void navigate({ to: '/onboarding', search: { step: next, workspace: workspace.slug } });
  };
  const advance = (from: WorkspaceOnboardingStep) => goTo(nextOnboardingStep(from, inclusions));

  const handleCreated = (created: Workspace, goals: SetupGoalKey[]) => {
    setCreatedGoals({ workspaceId: created.id, goals });
    queryClient.setQueryData(queryKeys.workspaces.bySlug(created.slug), created);
    // Replace the form entry so browser Back can't return to it and create another workspace.
    void navigate({ to: '/onboarding', search: { step: setupGuideEnabled ? 'method' : 'ai', workspace: created.slug }, replace: true });
  };

  const goToWorkspace = () => {
    if (!workspace) return;
    void navigate({ to: workspaceCreatedDestination({ slug: workspace.slug, setupGuideEnabled }) });
  };

  const hasOtherWorkspaces = workspaces.some((item) => item.id !== workspace?.id);
  const headerAction = (
    <div className="flex items-center gap-4">
      {hasOtherWorkspaces && (
        <button type="button" className={headerLinkClassName} onClick={() => void navigate({ to: '/workspaces' })}>
          Back to workspaces
        </button>
      )}
      <button type="button" className={headerLinkClassName} onClick={() => void signOut()}>
        Sign out
      </button>
    </div>
  );
  const steps = onboardingStepsFor(inclusions);
  const aiCapability = capabilities.data?.capabilities.find((capability) => capability.key === 'ai_chat');
  const gitHubCapability = capabilities.data?.capabilities.find((capability) => capability.key === 'github');

  if (!current || (current === 'workspace' && workspacesLoading)) {
    return (
      <OnboardingShell steps={steps} current={undefined} title="Setting things up" headerAction={headerAction}>
        <p role="status" className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loading01Icon className="h-4 w-4 animate-spin" aria-hidden="true" />
          Loading your workspace…
        </p>
      </OnboardingShell>
    );
  }

  if (current === 'workspace' || !workspace) {
    const firstWorkspace = workspaces.length === 0;
    return (
      <OnboardingShell
        steps={steps}
        current="workspace"
        title={firstWorkspace ? 'Create your workspace' : 'Create a workspace'}
        description={firstWorkspace
          ? 'A workspace is your team’s home in Helpin. Projects, support, docs and customers live here, and you invite teammates into it. Most companies need only one.'
          : 'Each workspace keeps its own projects, support, docs, customers and members. Use a separate one for a different company or a team that works apart.'}
        headerAction={headerAction}
      >
        <WorkspaceStep useEmailDefaults={firstWorkspace} onCreated={handleCreated} />
      </OnboardingShell>
    );
  }

  if (current === 'method') {
    return <OnboardingShell steps={steps} current="method" title="How would you like to set up your workspace?" description={`${workspace.name} is created. Choose how to continue.`} headerAction={headerAction}>
      <SetupMethodStep onUI={() => goTo('ai')} onAssistant={() => goTo('assistant')} onLater={goToWorkspace} />
    </OnboardingShell>;
  }

  if (current === 'assistant') {
    const continueInHelpin = async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.bySlug(workspace.slug), exact: true });
      await queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.settings(workspace.id), exact: true });
      goTo('ai');
    };
    return <OnboardingShell steps={steps} current="assistant" title="Set up your workspace with your AI assistant" description="Connect your assistant, then give it your workspace setup prompt." headerAction={headerAction}>
      <WorkspaceSetupAssistantContent key={workspace.id} workspaceId={workspace.id} slug={workspace.slug} />
      <OnboardingActions className="sm:justify-between">
        <OnboardingTextButton onClick={() => goTo('method')}>Back</OnboardingTextButton>
        <Button variant="outline" onClick={() => void continueInHelpin()}>Continue in Helpin</Button>
      </OnboardingActions>
    </OnboardingShell>;
  }

  if (current === 'ai' && aiCapability) {
    return (
      <OnboardingShell
        steps={steps}
        current="ai"
        title="Connect AI"
        description="Agents, drafting and summaries run on an AI provider you choose."
        headerAction={headerAction}
      >
        <ConnectAIStep
          workspaceId={workspace.id}
          canManage={access ? has('workspace.update') : true}
          onContinue={() => advance('ai')}
        />
      </OnboardingShell>
    );
  }

  if (current === 'github' && gitHubCapability) {
    return (
      <OnboardingShell
        steps={steps}
        current="github"
        title="Connect GitHub"
        description="Optional. Connect repositories so project work and code stay linked."
        headerAction={headerAction}
      >
        <ConnectGitHubStep
          workspaceId={workspace.id}
          capability={gitHubCapability}
          onContinue={() => advance('github')}
        />
      </OnboardingShell>
    );
  }

  if (current === 'ai' || current === 'github' || current === 'context') {
    return (
      <OnboardingShell steps={steps} current="context" title="Tell Helpin about your company" headerAction={headerAction}>
        <CompanyContextStep
          key={workspace.id}
          workspace={workspace}
          defaultWebsiteUrl={workspacesLoading || workspaces.length <= 1 ? workspaceDefaultsFromUserEmail(user?.email).websiteUrl : ''}
          canGenerate={canGenerateCompanyContext(conditions)}
          onContinue={() => advance('context')}
        />
      </OnboardingShell>
    );
  }

  if (current === 'teams') {
    return (
      <OnboardingShell
        steps={steps}
        current="teams"
        title="Set up teams"
        description="Pick the teams you need. You can add more later."
        headerAction={headerAction}
      >
        <TeamsStep
          workspaceId={workspace.id}
          createdHandles={createdTeams.map((team) => team.handle)}
          onDone={(created) => {
            setCreatedTeamsState({
              workspaceId: workspace.id,
              teams: [...new Map(created.map(team => [team.id, team])).values()],
            });
            advance('teams');
          }}
        />
      </OnboardingShell>
    );
  }

  if (current === 'invite') {
    const emailUnavailable = areEmailInvitesUnavailable(conditions);
    return (
      <OnboardingShell
        steps={steps}
        current="invite"
        title="Invite your team"
        description={emailUnavailable ? undefined : 'Invitations are sent by email. You can also do this later.'}
        headerAction={headerAction}
      >
        <InviteStep
          workspaceId={workspace.id}
          teamIds={createdTeams.map((team) => team.id)}
          emailUnavailable={emailUnavailable}
          onDone={() => advance('invite')}
        />
      </OnboardingShell>
    );
  }

  return (
    <OnboardingShell
      steps={steps}
      current="finish"
      title="You’re set"
      description={`${workspace.name} is ready.`}
      headerAction={headerAction}
    >
      <FinishStep
        workspaceId={workspace.id}
        canLoadSampleData={has('workspace.update')}
        onGoToWorkspace={goToWorkspace}
      />
    </OnboardingShell>
  );
}
