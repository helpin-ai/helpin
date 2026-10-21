import { describe, expect, it } from 'vitest';

import type { CapabilitiesResponse } from '../capabilityTypes';
import {
  areEmailInvitesUnavailable,
  canGenerateCompanyContext,
  describeContextGenerationError,
  nextOnboardingStep,
  onboardingConditionsFromCapabilities,
  onboardingStepsFor,
  resolveGitHubStepInclusion,
  resolveOnboardingStep,
  shouldOfferAIStep,
  shouldOfferGitHubStep,
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
  it('offers the method choice before provider setup without inserting the assistant branch into progress', () => {
    expect(onboardingStepsFor({ includeAI: true, includeMethod: true })).toEqual(['workspace', 'method', 'ai', 'context', 'teams', 'invite', 'finish']);
    expect(nextOnboardingStep('method', { includeAI: true, includeMethod: true })).toBe('ai');
    expect(resolveOnboardingStep({ requested: 'assistant', hasWorkspace: true, includeMethod: true, includeAI: undefined, includeGitHub: undefined })).toBe('assistant');
    expect(resolveOnboardingStep({ requested: 'method', hasWorkspace: true, includeMethod: false, includeAI: false, includeGitHub: false })).toBe('context');
    expect(resolveOnboardingStep({ requested: 'assistant', hasWorkspace: true, includeMethod: undefined, includeAI: false, includeGitHub: false })).toBeUndefined();
  });
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
    expect(onboardingStepsFor({ includeAI: true, includeGitHub: false })).toEqual(['workspace', 'ai', 'context', 'teams', 'invite', 'finish']);
    expect(onboardingStepsFor({ includeAI: false, includeGitHub: false })).toEqual(['workspace', 'context', 'teams', 'invite', 'finish']);
    expect(nextOnboardingStep('workspace', { includeAI: true, includeGitHub: false })).toBe('ai');
    expect(nextOnboardingStep('workspace', { includeAI: false, includeGitHub: false })).toBe('context');
    expect(nextOnboardingStep('ai', { includeAI: false, includeGitHub: false })).toBe('context');
    expect(nextOnboardingStep('invite', { includeAI: false, includeGitHub: false })).toBe('finish');
    expect(nextOnboardingStep('finish', { includeAI: false, includeGitHub: false })).toBe('finish');
  });

  describe('resolveOnboardingStep (resume by URL)', () => {
    it('always starts at the workspace step until a workspace exists', () => {
      expect(resolveOnboardingStep({ requested: 'teams', hasWorkspace: false, includeAI: false, includeGitHub: false })).toBe('workspace');
      expect(resolveOnboardingStep({ requested: undefined, hasWorkspace: false, includeAI: false, includeGitHub: false })).toBe('workspace');
    });

    it('never returns to the workspace form once the workspace exists', () => {
      expect(resolveOnboardingStep({ requested: 'workspace', hasWorkspace: true, includeAI: true, includeGitHub: false })).toBe('ai');
      expect(resolveOnboardingStep({ requested: 'workspace', hasWorkspace: true, includeAI: false, includeGitHub: false })).toBe('context');
      expect(resolveOnboardingStep({ requested: undefined, hasWorkspace: true, includeAI: false, includeGitHub: false })).toBe('context');
    });

    it('waits for capabilities before deciding on Connect AI', () => {
      expect(resolveOnboardingStep({ requested: 'ai', hasWorkspace: true, includeAI: undefined, includeGitHub: false })).toBeUndefined();
      expect(resolveOnboardingStep({ requested: 'ai', hasWorkspace: true, includeAI: false, includeGitHub: false })).toBe('context');
      expect(resolveOnboardingStep({ requested: 'ai', hasWorkspace: true, includeAI: true, includeGitHub: false })).toBe('ai');
    });

    it('resumes later steps as requested', () => {
      expect(resolveOnboardingStep({ requested: 'invite', hasWorkspace: true, includeAI: undefined, includeGitHub: false })).toBe('invite');
      expect(resolveOnboardingStep({ requested: 'finish', hasWorkspace: true, includeAI: false, includeGitHub: false })).toBe('finish');
    });
  });

  describe('Connect GitHub', () => {
    const offer = { edition: 'community' as const, github: 'needs_setup' as const, goals: ['product_delivery' as const], isOwner: true };

    it('is offered only when every condition holds', () => {
      expect(shouldOfferGitHubStep(offer)).toBe(true);
      expect(shouldOfferGitHubStep({ ...offer, github: 'unable_to_verify' })).toBe(true);
      expect(shouldOfferGitHubStep({ ...offer, edition: 'enterprise' })).toBe(false);
      expect(shouldOfferGitHubStep({ ...offer, goals: ['customer_support', 'automation_mastery'] })).toBe(false);
      expect(shouldOfferGitHubStep({ ...offer, goals: ['sales_crm', 'product_delivery'] })).toBe(true);
      expect(shouldOfferGitHubStep({ ...offer, github: 'ready' })).toBe(false);
      expect(shouldOfferGitHubStep({ ...offer, github: 'unavailable' })).toBe(false);
      expect(shouldOfferGitHubStep({ ...offer, isOwner: false })).toBe(false);
    });

    it('comes after Connect AI and before company context', () => {
      expect(onboardingStepsFor({ includeAI: true, includeGitHub: true })).toEqual(['workspace', 'ai', 'github', 'context', 'teams', 'invite', 'finish']);
      expect(onboardingStepsFor({ includeAI: false, includeGitHub: true })).toEqual(['workspace', 'github', 'context', 'teams', 'invite', 'finish']);
      expect(nextOnboardingStep('ai', { includeAI: true, includeGitHub: true })).toBe('github');
      expect(nextOnboardingStep('ai', { includeAI: true, includeGitHub: false })).toBe('context');
      expect(nextOnboardingStep('workspace', { includeAI: false, includeGitHub: true })).toBe('github');
      expect(nextOnboardingStep('github', { includeAI: true, includeGitHub: false })).toBe('context');
      // Skip for now continues from the step to company context.
      expect(nextOnboardingStep('github', { includeAI: true, includeGitHub: true })).toBe('context');
    });

    it('resolves requested and skipped GitHub steps from the URL', () => {
      const resolve = (requested: 'workspace' | 'ai' | 'github' | undefined, includeAI: boolean | undefined, includeGitHub: boolean | undefined) =>
        resolveOnboardingStep({ requested, hasWorkspace: true, includeAI, includeGitHub });
      expect(resolve('github', false, true)).toBe('github');
      expect(resolve('github', true, false)).toBe('context');
      expect(resolve('github', false, undefined)).toBeUndefined();
      expect(resolve('ai', false, true)).toBe('github');
      expect(resolve('ai', true, undefined)).toBe('ai');
      expect(resolve('workspace', false, undefined)).toBeUndefined();
      expect(resolve(undefined, false, false)).toBe('context');
    });

    it('decides inclusion from capabilities, ownership and goals, remembering it once shown', () => {
      const base = {
        workspaceId: 'ws-1',
        rememberedWorkspaceId: null,
        requested: false,
        capabilitiesSettled: true,
        conditions: { edition: 'community' as const, github: 'needs_setup' as const },
        accessSettled: true,
        role: 'owner',
        goals: ['product_delivery' as const],
      };
      expect(resolveGitHubStepInclusion(base)).toBe(true);
      expect(resolveGitHubStepInclusion({ ...base, capabilitiesSettled: false })).toBeUndefined();
      expect(resolveGitHubStepInclusion({ ...base, accessSettled: false })).toBeUndefined();
      expect(resolveGitHubStepInclusion({ ...base, goals: undefined })).toBeUndefined();
      expect(resolveGitHubStepInclusion({ ...base, role: 'admin' })).toBe(false);
      expect(resolveGitHubStepInclusion({ ...base, role: 'member' })).toBe(false);
      expect(resolveGitHubStepInclusion({ ...base, goals: ['customer_support'] })).toBe(false);
      expect(resolveGitHubStepInclusion({ ...base, conditions: { edition: 'enterprise', github: 'needs_setup' } })).toBe(false);
      expect(resolveGitHubStepInclusion({ ...base, conditions: { edition: 'community', github: 'unavailable' } })).toBe(false);
      // GitHub returns to ?step=github already connected: the owner still sees the result.
      expect(resolveGitHubStepInclusion({ ...base, requested: true, goals: undefined, conditions: { edition: 'community', github: 'ready' } })).toBe(true);
      expect(resolveGitHubStepInclusion({ ...base, requested: true, role: 'admin' })).toBe(false);
      // Once shown, a connection that turns GitHub ready keeps the step in the flow.
      expect(resolveGitHubStepInclusion({ ...base, rememberedWorkspaceId: 'ws-1', capabilitiesSettled: false })).toBe(true);
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
