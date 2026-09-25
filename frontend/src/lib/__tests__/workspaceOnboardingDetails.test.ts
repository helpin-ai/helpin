import { describe, expect, it } from 'vitest';

import { validateWorkspaceOnboardingDetails } from '../workspaceOnboardingDetails';

describe('workspaceOnboardingDetails', () => {
  it('does not require a website to create the workspace', () => {
    expect(validateWorkspaceOnboardingDetails({
      hasOrganization: true,
      name: 'Acme',
      slug: 'acme',
      goalCount: 1,
    })).toBeNull();
  });

  it('requires a name', () => {
    expect(validateWorkspaceOnboardingDetails({
      hasOrganization: true,
      name: ' ',
      slug: '',
      goalCount: 1,
    })).toBe('Enter a workspace name.');
  });

  it('requires at least one goal and has no maximum', () => {
    expect(validateWorkspaceOnboardingDetails({
      hasOrganization: true,
      name: 'Acme',
      slug: 'acme',
      goalCount: 0,
    })).toBe('Select at least one thing to set up.');
    expect(validateWorkspaceOnboardingDetails({
      hasOrganization: true,
      name: 'Acme',
      slug: 'acme',
      goalCount: 5,
    })).toBeNull();
  });

  it('requires an organization', () => {
    expect(validateWorkspaceOnboardingDetails({
      hasOrganization: false,
      name: 'Acme',
      slug: 'acme',
      goalCount: 1,
    })).toBe('Enter an organization name to continue.');
  });
});
