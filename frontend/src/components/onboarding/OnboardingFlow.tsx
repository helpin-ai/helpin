import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useQueryClient } from '@tanstack/react-query';
import { usePermissions, useWorkspaceAccess, useWorkspaceBySlug, useWorkspaceCapabilities, useWorkspaces } from '@/hooks/queries';
import { useTitle } from '@/hooks/useTitle';
import { Loading01Icon } from '@/lib/icons';
import { queryKeys } from '@/lib/queryKeys';
import type { Workspace } from '@/lib/types';
import {
  onboardingConditionsFromCapabilities,
  onboardingStepsFor,
  resolveOnboardingStep,
  shouldOfferAIStep,
} from '@/lib/workspaceOnboardingFlow';
import { workspaceDefaultsFromUserEmail } from '@/lib/workspaceOnboardingDefaults';
import { clearPendingSupportSetup, readPendingSupportSetup } from '@/lib/pendingSupportSetup';
import { workspaceCreatedDestination, type WorkspaceOnboardingStep } from '@/lib/workspaceOnboardingMode';
import { useAuthStore } from '@/stores/authStore';
import { Button } from '@/components/ui/button';
import { ConnectAIStep } from './ConnectAIStep';
import { OnboardingShell } from './OnboardingShell';
import { SupportSetupProgress } from './SupportSetupProgress';
import { SetupReview } from './SupportWebsiteStep';

export type OnboardingFlowProps = {
  /** `step` from the URL. */
  step?: WorkspaceOnboardingStep;
  /** `workspace` slug from the URL, present once the workspace exists. */
  workspaceSlug?: string;
};

const headerLinkClassName = 'public-page-back rounded-sm underline-offset-4 hover:text-foreground hover:underline';

/**
 * Support onboarding after sign-in. The website, what was found and the AI's
 * first answer happen on the public /start page; the choices made there are
 * saved, and this flow creates the workspace and starts support from them,
 * then shows (Community without AI) Connect AI and setup progress. Without
 * saved choices it sends the person to /start. The URL holds the step and
 * workspace slug, so reloads resume and Back never creates a second workspace.
 */
export function OnboardingFlow({ step, workspaceSlug }: OnboardingFlowProps) {
  useTitle('Set up support');
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useAuthStore((state) => state.user);
  const signOut = useAuthStore((state) => state.signOut);
  const { data: workspaces = [], isLoading: workspacesLoading } = useWorkspaces();
  const workspaceQuery = useWorkspaceBySlug(workspaceSlug ?? '');
  const workspace = workspaceSlug ? workspaceQuery.data ?? undefined : undefined;
  const capabilities = useWorkspaceCapabilities(workspace?.id);
  const accessQuery = useWorkspaceAccess(workspace?.id ?? '');
  const { has } = usePermissions(accessQuery.data);
  // Read once: it is cleared when setup starts.
  const [pending] = useState(readPendingSupportSetup);
  // Once shown for a workspace, Connect AI stays in the flow even after it succeeds.
  const [aiStepWorkspaceId, setAiStepWorkspaceId] = useState<string | null>(null);

  const capabilitiesSettled = capabilities.isSuccess || capabilities.isError;
  const includeAI = workspace
    ? capabilitiesSettled
      ? shouldOfferAIStep(onboardingConditionsFromCapabilities(capabilities.data)) || aiStepWorkspaceId === workspace.id
      : undefined
    : false;
  const loadingWorkspace = Boolean(workspaceSlug) && workspaceQuery.isLoading;
  const missingWorkspace = Boolean(workspaceSlug) && workspaceQuery.isError;
  const current = loadingWorkspace ? undefined : resolveOnboardingStep({ requested: step, hasWorkspace: Boolean(workspace), includeAI });

  useEffect(() => {
    if (loadingWorkspace || !current) return;
    const slug = missingWorkspace ? undefined : workspace?.slug;
    if (current !== step || slug !== workspaceSlug) {
      void navigate({ to: '/onboarding', search: { step: current, workspace: slug }, replace: true });
    }
  }, [current, loadingWorkspace, missingWorkspace, navigate, step, workspace?.slug, workspaceSlug]);

  const needsStart = current === 'website' && !workspace && !pending && !workspacesLoading;
  useEffect(() => {
    if (!needsStart) return;
    const website = workspaces.length === 0 ? workspaceDefaultsFromUserEmail(user?.email).websiteUrl.replace(/^https:\/\//, '') : '';
    void navigate({ to: '/start', search: { website: website || undefined }, replace: true });
  }, [navigate, needsStart, user?.email, workspaces.length]);

  // Remembered during render (not in an effect) so a successful connection,
  // which turns AI `ready`, doesn't bounce the person past their result.
  if (current === 'ai' && workspace && aiStepWorkspaceId !== workspace.id) {
    setAiStepWorkspaceId(workspace.id);
  }

  const handleStarted = (created: Workspace) => {
    clearPendingSupportSetup();
    queryClient.setQueryData(queryKeys.workspaces.bySlug(created.slug), created);
    // Replace the form entry so browser Back can't return to it and create another workspace.
    void navigate({ to: '/onboarding', search: { step: undefined, workspace: created.slug }, replace: true });
  };
  const goToInbox = () => {
    if (workspace) void navigate({ to: workspaceCreatedDestination(workspace.slug) });
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

  if (!current || (current === 'website' && workspacesLoading) || needsStart) {
    return (
      <OnboardingShell steps={steps} current={undefined} title="Setting things up" headerAction={headerAction}>
        <p role="status" className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loading01Icon className="h-4 w-4 animate-spin" aria-hidden="true" />
          Loading…
        </p>
      </OnboardingShell>
    );
  }

  if (current === 'website' || !workspace) {
    if (!pending) return null;
    return (
      <OnboardingShell steps={steps} current="website" title={`Setting up ${pending.companyName}`} description="Creating your workspace and starting your support setup." headerAction={headerAction}>
        <SetupReview
          detection={pending.detection}
          initial={pending}
          autoStart
          onBack={() => {
            clearPendingSupportSetup();
            void navigate({ to: '/start', search: { website: pending.detection.website_url.replace(/^https:\/\//, '') } });
          }}
          onStarted={handleStarted}
        />
      </OnboardingShell>
    );
  }

  if (current === 'ai' && aiCapability) {
    return (
      <OnboardingShell steps={steps} current="ai" title="Connect AI" description="Your AI agent runs on a provider you choose. Your articles and website are being copied meanwhile." headerAction={headerAction}>
        <ConnectAIStep
          workspaceId={workspace.id}
          canManage={accessQuery.data ? has('workspace.update') : true}
          onContinue={() => void navigate({ to: '/onboarding', search: { step: 'setup', workspace: workspace.slug } })}
        />
      </OnboardingShell>
    );
  }

  return (
    <OnboardingShell steps={steps} current="setup" title={`Setting up ${workspace.name}`} description="This keeps running in the background, so you can start using Helpin now." headerAction={headerAction}>
      <SupportSetupProgress workspaceId={workspace.id} slug={workspace.slug} />
      <div className="flex justify-end">
        <Button size="lg" className="w-full sm:w-auto sm:min-w-44" onClick={goToInbox}>Go to your inbox</Button>
      </div>
    </OnboardingShell>
  );
}
