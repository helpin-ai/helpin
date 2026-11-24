import { describe, expect, it } from 'vitest';

import { workspaceDefaultsFromUserEmail } from '../workspaceOnboardingDefaults';

describe('workspaceOnboardingDefaults', () => {
  it('suggests workspace details from a company email domain', () => {
    expect(workspaceDefaultsFromUserEmail('jane.doe@acme-labs.io')).toEqual({
      name: 'Acme Labs',
      slug: 'acme-labs',
      workspaceKey: 'ACM',
      websiteUrl: 'https://acme-labs.io',
    });
  });

  it('uses the registrable domain for common two-part country suffixes', () => {
    expect(workspaceDefaultsFromUserEmail('founder@mail.example.co.uk')).toEqual({
      name: 'Example',
      slug: 'example',
      workspaceKey: 'EXA',
      websiteUrl: 'https://example.co.uk',
    });
  });

  it('does not suggest company details for personal email providers', () => {
    expect(workspaceDefaultsFromUserEmail('person@gmail.com')).toEqual({
      name: '',
      slug: '',
      workspaceKey: '',
      websiteUrl: '',
    });
  });
});
