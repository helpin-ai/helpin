import { describe, expect, it } from 'vitest';

import type { CapabilitiesResponse } from '../capabilityTypes';
import {
  onboardingConditionsFromCapabilities,
  onboardingStepsFor,
  resolveOnboardingStep,
  shouldOfferAIStep,
  siteLabel,
  websiteDisplayName,
} from '../workspaceOnboardingFlow';

function capabilities(edition: CapabilitiesResponse['edition'], ai: string): CapabilitiesResponse {
  return { edition, capabilities: [{ key: 'ai_chat', status: ai as never, detail: '', required: false }] };
}

describe('workspaceOnboardingFlow', () => {
  it('reads edition and AI status from the capabilities response', () => {
    expect(onboardingConditionsFromCapabilities(capabilities('community', 'needs_setup'))).toEqual({ edition: 'community', ai: 'needs_setup' });
    expect(onboardingConditionsFromCapabilities(undefined)).toEqual({ edition: undefined, ai: undefined });
  });

  it('offers Connect AI only on Community without working AI', () => {
    expect(shouldOfferAIStep({ edition: 'community', ai: 'needs_setup' })).toBe(true);
    expect(shouldOfferAIStep({ edition: 'community', ai: 'unable_to_verify' })).toBe(true);
    expect(shouldOfferAIStep({ edition: 'community', ai: 'ready' })).toBe(false);
    expect(shouldOfferAIStep({ edition: 'enterprise', ai: 'needs_setup' })).toBe(false);
    expect(shouldOfferAIStep({})).toBe(false);
  });

  it('orders the steps with Connect AI only when it applies', () => {
    expect(onboardingStepsFor({ includeAI: false })).toEqual(['website', 'setup']);
    expect(onboardingStepsFor({ includeAI: true })).toEqual(['website', 'ai', 'setup']);
  });

  describe('resolveOnboardingStep (resume by URL)', () => {
    it('starts at the website step until a workspace exists', () => {
      expect(resolveOnboardingStep({ requested: 'setup', hasWorkspace: false, includeAI: false })).toBe('website');
    });

    it('never returns to the website form once the workspace exists', () => {
      expect(resolveOnboardingStep({ requested: 'website', hasWorkspace: true, includeAI: false })).toBe('setup');
      expect(resolveOnboardingStep({ requested: undefined, hasWorkspace: true, includeAI: true })).toBe('ai');
    });

    it('waits for capabilities before deciding on Connect AI, except for setup', () => {
      expect(resolveOnboardingStep({ requested: undefined, hasWorkspace: true, includeAI: undefined })).toBeUndefined();
      expect(resolveOnboardingStep({ requested: 'setup', hasWorkspace: true, includeAI: undefined })).toBe('setup');
    });
  });

  it('shows a host name for the website being read', () => {
    expect(websiteDisplayName('https://www.acme.com/pricing')).toBe('acme.com');
    expect(websiteDisplayName('')).toBe('your website');
  });

  it('names a site with its path unless the path is only a language', () => {
    expect(siteLabel('https://www.acme.com/')).toBe('acme.com');
    expect(siteLabel('https://acme.com/docs/')).toBe('acme.com/docs');
    expect(siteLabel('https://help.acme.com/en')).toBe('help.acme.com');
  });
});
