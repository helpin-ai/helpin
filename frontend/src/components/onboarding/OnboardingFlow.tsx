import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useQueryClient } from '@tanstack/react-query';
import {
  usePermissions,
  useWorkspaceAccess,
  useWorkspaceBySlug,
  useWorkspaceCapabilities,
  useWorkspaces,
} from '@/hooks/queries';
import { useSetupGuideEnabled } from '@/hooks/useSetupGuideEnabled';
import { useTitle } from '@/hooks/useTitle';
import { Loading01Icon } from '@/lib/icons';
import { queryKeys } from '@/lib/queryKeys';
import type { Workspace } from '@/lib/types';
import {
  areEmailInvitesUnavailable,
  canGenerateCompanyContext,
  nextOnboardingStep,
  onboardingConditionsFromCapabilities,
  onboardingStepsFor,
  resolveOnboardingStep,
  shouldOfferAIStep,
} from '@/lib/workspaceOnboardingFlow';
import { workspaceDefaultsFromUserEmail } from '@/lib/workspaceOnboardingDefaults';
import { workspaceCreatedDestination, type WorkspaceOnboardingStep } from '@/lib/workspaceOnboardingMode';
import { useAuthStore } from '@/stores/authStore';
import { CompanyContextStep } from './CompanyContextStep';
import { ConnectAIStep } from './ConnectAIStep';
import { FinishStep } from './FinishStep';
import { InviteStep } from './InviteStep';
import { OnboardingShell } from './OnboardingShell';
import { TeamsStep } from './TeamsStep';
import { WorkspaceStep } from './WorkspaceStep';

export type OnboardingFlowProps = {
  /** `step` from the URL. */
  step?: WorkspaceOnboardingStep;
  /** `workspace` slug from the URL, present once the workspace exists. */
  workspaceSlug?: string;
};

const headerLinkClassName = 'public-page-back rounded-sm underline-offset-4 hover:text-foreground hover:underline';

/**
 * Workspace onboarding for every edition, in the signup page shell:
 * workspace → Connect AI (Community without AI) → company context → teams →
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
  const { data: access } = useWorkspaceAccess(workspace?.id ?? '');
  const { has } = usePermissions(access);
  // Once shown for a workspace, Connect AI stays in the flow even after it succeeds.
  const [aiStepWorkspaceId, setAiStepWorkspaceId] = useState<string | null>(null);
  const [createdTeams, setCreatedTeams] = useState<{ id: string; handle: string }[]>([]);

  const conditions = onboardingConditionsFromCapabilities(capabilities.data);
  const capabilitiesSettled = capabilities.isSuccess || capabilities.isError;
  const includeAI = workspace
    ? capabilitiesSettled
      ? shouldOfferAIStep(conditions) || aiStepWorkspaceId === workspace.id
      : undefined
    : false;
  const loadingWorkspace = Boolean(workspaceSlug) && workspaceQuery.isLoading;
  const missingWorkspace = Boolean(workspaceSlug) && workspaceQuery.isError;
  const current = loadingWorkspace
    ? undefined
    : resolveOnboardingStep({ requested: step, hasWorkspace: Boolean(workspace), includeAI });

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

  const goTo = (next: WorkspaceOnboardingStep) => {
    if (!workspace) return;
    void navigate({ to: '/onboarding', search: { step: next, workspace: workspace.slug } });
  };
  const advance = (from: WorkspaceOnboardingStep) => goTo(nextOnboardingStep(from, { includeAI: Boolean(includeAI) }));

  const handleCreated = (created: Workspace) => {
    queryClient.setQueryData(queryKeys.workspaces.bySlug(created.slug), created);
    // Replace the form entry so browser Back can't return to it and create another workspace.
    void navigate({ to: '/onboarding', search: { step: 'ai', workspace: created.slug }, replace: true });
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
  const steps = onboardingStepsFor({ includeAI: Boolean(includeAI) });
  const aiCapability = capabilities.data?.capabilities.find((capability) => capability.key === 'ai_chat');

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
        title={firstWorkspace ? 'Set up your workspace' : 'Create a workspace'}
        description="Name it and choose what you want to set up. You can change both later."
        headerAction={headerAction}
      >
        <WorkspaceStep useEmailDefaults={firstWorkspace} onCreated={handleCreated} />
      </OnboardingShell>
    );
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
          capability={aiCapability}
          workspaceId={workspace.id}
          slug={workspace.slug}
          canManage={access ? has('workspace.update') : true}
          onContinue={() => advance('ai')}
        />
      </OnboardingShell>
    );
  }

  if (current === 'ai' || current === 'context') {
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
            setCreatedTeams((existing) => [...existing, ...created]);
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
