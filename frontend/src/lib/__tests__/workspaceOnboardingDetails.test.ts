import { describe, expect, it } from 'vitest';

import { validateWorkspaceOnboardingDetails } from '../workspaceOnboardingDetails';

describe('workspaceOnboardingDetails', () => {
  it('requires a website URL before continuing', () => {
    expect(validateWorkspaceOnboardingDetails({
      hasOrganization: true,
      name: 'Acme',
      slug: 'acme',
      websiteUrl: '',
    })).toEqual('Enter your company or product website');
  });

  it('allows continuing when required details are present', () => {
    expect(validateWorkspaceOnboardingDetails({
      hasOrganization: true,
      name: 'Acme',
      slug: 'acme',
      websiteUrl: 'https://acme.com',
    })).toBeNull();
  });
});
