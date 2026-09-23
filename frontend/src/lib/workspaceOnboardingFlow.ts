import type { CapabilitiesResponse, Capability, CapabilityKey } from './capabilityTypes';
import type { WorkspaceOnboardingStep } from './workspaceOnboardingMode';

/**
 * What the server reports about a freshly created workspace, reduced to the
 * facts the onboarding flow branches on.
 */
/** Where workspace AI keys are managed after onboarding. */
export const AI_SETTINGS_PATH_LABEL = 'Settings → AI & knowledge → AI setup';

export type OnboardingConditions = {
  /** The API edition. Cloud/Enterprise manage AI for the workspace. */
  edition?: CapabilitiesResponse['edition'];
  ai?: Capability['status'];
  email?: Capability['status'];
};

function statusOf(response: CapabilitiesResponse | undefined, key: CapabilityKey) {
  return response?.capabilities.find((capability) => capability.key === key)?.status;
}

export function onboardingConditionsFromCapabilities(response: CapabilitiesResponse | undefined): OnboardingConditions {
  return {
    edition: response?.edition,
    ai: statusOf(response, 'ai_chat'),
    email: statusOf(response, 'email_outbound'),
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

/** The steps shown in the progress indicator, in order. */
export function onboardingStepsFor({ includeAI }: { includeAI: boolean }): WorkspaceOnboardingStep[] {
  return includeAI
    ? ['workspace', 'ai', 'context', 'teams', 'invite', 'finish']
    : ['workspace', 'context', 'teams', 'invite', 'finish'];
}

export function nextOnboardingStep(
  current: WorkspaceOnboardingStep,
  { includeAI }: { includeAI: boolean },
): WorkspaceOnboardingStep {
  const steps = onboardingStepsFor({ includeAI });
  const index = steps.indexOf(current);
  if (index === -1) {
    // Only `ai` can be missing from the list; it is followed by `context`.
    return 'context';
  }
  return steps[Math.min(index + 1, steps.length - 1)];
}

/**
 * The step to render for a URL. Before a workspace exists only the workspace
 * step is possible. Once it exists the workspace step is never shown again, so
 * Back/Forward or a reload can't create a second workspace. `includeAI` is
 * undefined while capabilities are still loading; the `ai` step waits for it.
 */
export function resolveOnboardingStep({
  requested,
  hasWorkspace,
  includeAI,
}: {
  requested: WorkspaceOnboardingStep | undefined;
  hasWorkspace: boolean;
  includeAI: boolean | undefined;
}): WorkspaceOnboardingStep | undefined {
  if (!hasWorkspace) return 'workspace';
  if (!requested || requested === 'workspace') {
    if (includeAI === undefined) return undefined;
    return nextOnboardingStep('workspace', { includeAI });
  }
  if (requested === 'ai') {
    if (includeAI === undefined) return undefined;
    return includeAI ? 'ai' : 'context';
  }
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
