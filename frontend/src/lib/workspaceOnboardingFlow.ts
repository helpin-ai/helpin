import type { CapabilitiesResponse, Capability, CapabilityKey } from './capabilityTypes';
import type { SetupGoalKey } from './setupTypes';
import { workspaceOnboardingSteps, type WorkspaceOnboardingStep } from './workspaceOnboardingMode';

/**
 * What the server reports about a freshly created workspace, reduced to the
 * facts the onboarding flow branches on.
 */
/** Where workspace AI keys are managed after onboarding. */
export const AI_SETTINGS_PATH_LABEL = 'Settings → AI & knowledge → AI models';

export type OnboardingConditions = {
  /** The API edition. Cloud/Enterprise manage AI for the workspace. */
  edition?: CapabilitiesResponse['edition'];
  ai?: Capability['status'];
  email?: Capability['status'];
  github?: Capability['status'];
};

function statusOf(response: CapabilitiesResponse | undefined, key: CapabilityKey) {
  return response?.capabilities.find((capability) => capability.key === key)?.status;
}

export function onboardingConditionsFromCapabilities(response: CapabilitiesResponse | undefined): OnboardingConditions {
  return {
    edition: response?.edition,
    ai: statusOf(response, 'ai_chat'),
    email: statusOf(response, 'email_outbound'),
    github: statusOf(response, 'github'),
  };
}

/**
 * The Connect AI step appears only on Community servers where the workspace has
 * no working AI yet: nothing is connected (`needs_setup`), or a connection
 * exists but has never been checked (`unable_to_verify`, which the step tests).
 * Cloud and Enterprise provide AI themselves, so the step never appears there.
 */
export function shouldOfferAIStep({ edition, ai }: OnboardingConditions) {
  return edition === 'community' && (ai === 'needs_setup' || ai === 'unable_to_verify');
}

/** Generating company context needs an AI connection that works, or may work. */
export function canGenerateCompanyContext({ ai }: OnboardingConditions) {
  return ai === 'ready' || ai === 'unable_to_verify';
}

/** Invitations are sent by email; without outbound email they can't be delivered. */
export function areEmailInvitesUnavailable({ email }: OnboardingConditions) {
  return email === 'needs_setup' || email === 'unavailable';
}

/** Setup goal whose people are offered the optional Connect GitHub step. */
export const GITHUB_STEP_GOAL: SetupGoalKey = 'product_delivery';

export type GitHubStepConditions = {
  edition?: CapabilitiesResponse['edition'];
  github?: Capability['status'];
  /** Setup goals chosen for the workspace, in priority order. */
  goals: readonly SetupGoalKey[];
  /** Only the workspace owner can create or install the GitHub App from Helpin. */
  isOwner: boolean;
};

/**
 * The optional Connect GitHub step appears only when every condition holds: a
 * Community server, the person chose to plan and ship projects, GitHub isn't
 * ready for the workspace yet, and they own the workspace. Admins who aren't
 * owners and members skip it silently; so does a server where GitHub is
 * unavailable (Projects or Agents turned off).
 */
export function shouldOfferGitHubStep({ edition, github, goals, isOwner }: GitHubStepConditions) {
  return edition === 'community'
    && isOwner
    && goals.includes(GITHUB_STEP_GOAL)
    && (github === 'needs_setup' || github === 'unable_to_verify');
}

/**
 * Whether this visit includes Connect GitHub, or undefined while something it
 * depends on is still loading. Once shown for a workspace the step stays, so a
 * successful connection (GitHub turns ready) doesn't skip past its result. A
 * URL that asks for the step (GitHub returns to `?step=github`) keeps it for
 * the owner on Community, even when GitHub is ready by then.
 */
export function resolveGitHubStepInclusion({
  workspaceId,
  rememberedWorkspaceId,
  requested,
  capabilitiesSettled,
  conditions,
  accessSettled,
  role,
  goals,
}: {
  workspaceId: string;
  rememberedWorkspaceId: string | null;
  requested: boolean;
  capabilitiesSettled: boolean;
  conditions: OnboardingConditions;
  accessSettled: boolean;
  role: string | undefined;
  /** Undefined while the workspace's goals are loading. */
  goals: readonly SetupGoalKey[] | undefined;
}): boolean | undefined {
  if (rememberedWorkspaceId === workspaceId) return true;
  if (!capabilitiesSettled) return undefined;
  const { edition, github } = conditions;
  if (edition !== 'community' || !github || github === 'unavailable') return false;
  if (!accessSettled) return undefined;
  if (role !== 'owner') return false;
  if (requested) return true;
  if (goals === undefined) return undefined;
  return shouldOfferGitHubStep({ edition, github, goals, isOwner: true });
}

/** Optional steps included in this flow. */
export type OnboardingInclusions = { includeAI: boolean; includeGitHub?: boolean; includeMethod?: boolean };

/** The steps shown in the progress indicator, in order. */
export function onboardingStepsFor({ includeAI, includeGitHub = false, includeMethod = false }: OnboardingInclusions): WorkspaceOnboardingStep[] {
  return workspaceOnboardingSteps.filter((step) =>
    step !== 'assistant' && (step !== 'method' || includeMethod) && (step !== 'ai' || includeAI) && (step !== 'github' || includeGitHub));
}

export function nextOnboardingStep(
  current: WorkspaceOnboardingStep,
  inclusions: OnboardingInclusions,
): WorkspaceOnboardingStep {
  const steps = onboardingStepsFor(inclusions);
  // Walk the full order so a step left out of this flow (`ai`, `github`) still has a successor.
  const order = workspaceOnboardingSteps;
  for (let index = order.indexOf(current) + 1; index < order.length; index += 1) {
    if (steps.includes(order[index])) return order[index];
  }
  return 'finish';
}

/**
 * The step to render for a URL. Before a workspace exists only the workspace
 * step is possible. Once it exists the workspace step is never shown again, so
 * Back/Forward or a reload can't create a second workspace. `includeAI` and
 * `includeGitHub` are undefined while what they depend on is still loading;
 * the optional steps wait for them.
 */
export function resolveOnboardingStep({
  requested,
  hasWorkspace,
  includeAI,
  includeGitHub,
  includeMethod,
}: {
  requested: WorkspaceOnboardingStep | undefined;
  hasWorkspace: boolean;
  includeAI: boolean | undefined;
  includeGitHub: boolean | undefined;
  includeMethod?: boolean;
}): WorkspaceOnboardingStep | undefined {
  if (!hasWorkspace) return 'workspace';
  const fromGitHub = () => {
    if (includeGitHub === undefined) return undefined;
    return includeGitHub ? 'github' : 'context';
  };
  const fromAI = () => {
    if (includeAI === undefined) return undefined;
    return includeAI ? 'ai' : fromGitHub();
  };
  if (requested === 'method' || requested === 'assistant') {
    if (includeMethod === undefined) return undefined;
    return includeMethod ? requested : fromAI();
  }
  if (!requested || requested === 'workspace' || requested === 'ai') return fromAI();
  if (requested === 'github') return fromGitHub();
  return requested;
}

/**
 * Adds or removes a goal. Order is priority: the first goal selected leads
 * the Setup guide. There is no maximum.
 */
export function toggleOrderedSelection<T>(selected: readonly T[], value: T): T[] {
  return selected.includes(value)
    ? selected.filter((item) => item !== value)
    : [...selected, value];
}

/**
 * The inline message for a failed context generation. The server's sentence
 * is written for people for validation (400) and generation (422) failures;
 * anything else gets a neutral sentence.
 */
export function describeContextGenerationError({ status, error }: { status?: number; error: string | null }) {
  if ((status === 400 || status === 422) && error?.trim()) {
    return error.trim();
  }
  return 'We couldn’t generate a description right now. You can write it yourself.';
}

/** Host name shown while Helpin reads a website, for example `acme.com`. */
export function websiteDisplayName(url: string) {
  return url.trim().replace(/^https?:\/\//i, '').replace(/\/.*$/, '') || 'your website';
}
