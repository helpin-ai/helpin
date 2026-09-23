import { describe, expect, it } from 'vitest';

import type { CapabilitiesResponse } from '../capabilityTypes';
import {
  areEmailInvitesUnavailable,
  canGenerateCompanyContext,
  describeContextGenerationError,
  nextOnboardingStep,
  onboardingConditionsFromCapabilities,
  onboardingStepsFor,
  resolveOnboardingStep,
  shouldOfferAIStep,
  toggleOrderedSelection,
  websiteDisplayName,
} from '../workspaceOnboardingFlow';

function capabilities(edition: CapabilitiesResponse['edition'], ai: string, email: string): CapabilitiesResponse {
  return {
    edition,
    capabilities: [
      { key: 'ai_chat', status: ai as never, detail: '', required: false },
      { key: 'email_outbound', status: email as never, detail: '', required: false },
    ],
  };
}

describe('workspaceOnboardingFlow', () => {
  it('reads edition, AI and email status from the capabilities response', () => {
    expect(onboardingConditionsFromCapabilities(capabilities('community', 'needs_setup', 'ready'))).toEqual({
      edition: 'community',
      ai: 'needs_setup',
      email: 'ready',
    });
    expect(onboardingConditionsFromCapabilities(undefined)).toEqual({ edition: undefined, ai: undefined, email: undefined });
  });

  it('offers Connect AI only on Community without working AI', () => {
    expect(shouldOfferAIStep({ edition: 'community', ai: 'needs_setup' })).toBe(true);
    expect(shouldOfferAIStep({ edition: 'community', ai: 'unable_to_verify' })).toBe(true);
    expect(shouldOfferAIStep({ edition: 'community', ai: 'ready' })).toBe(false);
    expect(shouldOfferAIStep({ edition: 'enterprise', ai: 'needs_setup' })).toBe(false);
    expect(shouldOfferAIStep({})).toBe(false);
  });

  it('allows generating context only when AI works or may work', () => {
    expect(canGenerateCompanyContext({ ai: 'ready' })).toBe(true);
    expect(canGenerateCompanyContext({ ai: 'unable_to_verify' })).toBe(true);
    expect(canGenerateCompanyContext({ ai: 'needs_setup' })).toBe(false);
    expect(canGenerateCompanyContext({})).toBe(false);
  });

  it('treats invitations as unavailable without outbound email', () => {
    expect(areEmailInvitesUnavailable({ email: 'needs_setup' })).toBe(true);
    expect(areEmailInvitesUnavailable({ email: 'unavailable' })).toBe(true);
    expect(areEmailInvitesUnavailable({ email: 'ready' })).toBe(false);
    expect(areEmailInvitesUnavailable({ email: 'unable_to_verify' })).toBe(false);
  });

  it('orders the steps with Connect AI only when it applies', () => {
    expect(onboardingStepsFor({ includeAI: true })).toEqual(['workspace', 'ai', 'context', 'teams', 'invite', 'finish']);
    expect(onboardingStepsFor({ includeAI: false })).toEqual(['workspace', 'context', 'teams', 'invite', 'finish']);
    expect(nextOnboardingStep('workspace', { includeAI: true })).toBe('ai');
    expect(nextOnboardingStep('workspace', { includeAI: false })).toBe('context');
    expect(nextOnboardingStep('ai', { includeAI: false })).toBe('context');
    expect(nextOnboardingStep('invite', { includeAI: false })).toBe('finish');
    expect(nextOnboardingStep('finish', { includeAI: false })).toBe('finish');
  });

  describe('resolveOnboardingStep (resume by URL)', () => {
    it('always starts at the workspace step until a workspace exists', () => {
      expect(resolveOnboardingStep({ requested: 'teams', hasWorkspace: false, includeAI: false })).toBe('workspace');
      expect(resolveOnboardingStep({ requested: undefined, hasWorkspace: false, includeAI: false })).toBe('workspace');
    });

    it('never returns to the workspace form once the workspace exists', () => {
      expect(resolveOnboardingStep({ requested: 'workspace', hasWorkspace: true, includeAI: true })).toBe('ai');
      expect(resolveOnboardingStep({ requested: 'workspace', hasWorkspace: true, includeAI: false })).toBe('context');
      expect(resolveOnboardingStep({ requested: undefined, hasWorkspace: true, includeAI: false })).toBe('context');
    });

    it('waits for capabilities before deciding on Connect AI', () => {
      expect(resolveOnboardingStep({ requested: 'ai', hasWorkspace: true, includeAI: undefined })).toBeUndefined();
      expect(resolveOnboardingStep({ requested: 'ai', hasWorkspace: true, includeAI: false })).toBe('context');
      expect(resolveOnboardingStep({ requested: 'ai', hasWorkspace: true, includeAI: true })).toBe('ai');
    });

    it('resumes later steps as requested', () => {
      expect(resolveOnboardingStep({ requested: 'invite', hasWorkspace: true, includeAI: undefined })).toBe('invite');
      expect(resolveOnboardingStep({ requested: 'finish', hasWorkspace: true, includeAI: false })).toBe('finish');
    });
  });

  it('keeps selection order as priority with no maximum', () => {
    let selected: string[] = [];
    for (const goal of ['sales', 'support', 'docs', 'internal', 'projects']) {
      selected = toggleOrderedSelection(selected, goal);
    }
    expect(selected).toEqual(['sales', 'support', 'docs', 'internal', 'projects']);
    selected = toggleOrderedSelection(selected, 'sales');
    expect(selected).toEqual(['support', 'docs', 'internal', 'projects']);
    selected = toggleOrderedSelection(selected, 'sales');
    expect(selected[selected.length - 1]).toBe('sales');
  });

  it('shows the server sentence for validation and generation failures only', () => {
    expect(describeContextGenerationError({ status: 422, error: 'We couldn’t read acme.com.' })).toBe('We couldn’t read acme.com.');
    expect(describeContextGenerationError({ status: 400, error: 'Enter a valid website address.' })).toBe('Enter a valid website address.');
    expect(describeContextGenerationError({ status: 500, error: 'pq: connection refused' })).toBe('We couldn’t generate a description right now. You can write it yourself.');
    expect(describeContextGenerationError({ error: null })).toBe('We couldn’t generate a description right now. You can write it yourself.');
  });

  it('shows a host name for the website being read', () => {
    expect(websiteDisplayName('https://acme.com/pricing')).toBe('acme.com');
    expect(websiteDisplayName('')).toBe('your website');
  });
});
