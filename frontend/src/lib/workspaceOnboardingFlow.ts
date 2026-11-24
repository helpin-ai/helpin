import type { CapabilitiesResponse, Capability, CapabilityKey } from './capabilityTypes';
import type { WorkspaceOnboardingStep } from './workspaceOnboardingMode';

/** Where workspace AI keys are managed after onboarding. */
export const AI_SETTINGS_PATH_LABEL = 'Settings → AI & knowledge → AI models';

/**
 * What the server reports about a freshly created workspace, reduced to the
 * facts the onboarding flow branches on.
 */
export type OnboardingConditions = {
  /** The API edition. Cloud/Enterprise manage AI for the workspace. */
  edition?: CapabilitiesResponse['edition'];
  ai?: Capability['status'];
};

function statusOf(response: CapabilitiesResponse | undefined, key: CapabilityKey) {
  return response?.capabilities.find((capability) => capability.key === key)?.status;
}

export function onboardingConditionsFromCapabilities(response: CapabilitiesResponse | undefined): OnboardingConditions {
  return { edition: response?.edition, ai: statusOf(response, 'ai_chat') };
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

/** The steps shown in the progress indicator, in order. */
export function onboardingStepsFor({ includeAI }: { includeAI: boolean }): WorkspaceOnboardingStep[] {
  return includeAI ? ['website', 'ai', 'setup'] : ['website', 'setup'];
}

/**
 * The step to render for a URL. Before a workspace exists only the website
 * step is possible; once it exists that step is never shown again, so Back or
 * a reload can't create a second workspace. `includeAI` is undefined while
 * capabilities load, and the step waits for it.
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
  if (!hasWorkspace) return 'website';
  if (requested === 'setup') return 'setup';
  if (includeAI === undefined) return undefined;
  return includeAI ? 'ai' : 'setup';
}

/** Host name shown while Helpin reads a website, for example `acme.com`. */
export function websiteDisplayName(url: string) {
  return url.trim().replace(/^https?:\/\//i, '').replace(/^www\./i, '').replace(/\/.*$/, '') || 'your website';
}

/**
 * A site as people say it: `help.acme.com`, or `acme.com/docs` when the
 * address is a path on the site. A language path such as `/en` is left out.
 */
export function siteLabel(url: string) {
  try {
    const parsed = new URL(url);
    const path = parsed.pathname.replace(/\/+$/, '');
    return parsed.hostname.replace(/^www\./, '') + (/^\/[a-z]{2}(-[a-z]{2})?$/i.test(path) ? '' : path);
  } catch {
    return websiteDisplayName(url);
  }
}
