import { describe, expect, it } from 'vitest';

import { moreContextCopy } from '../onboardingContextPresentation';

describe('onboardingContextPresentation', () => {
  it('frames source access as agent context, not background usage', () => {
    expect(moreContextCopy.description).toContain('Helpin agents can use for product context');
    expect(moreContextCopy.description.toLowerCase()).not.toContain('background');
  });

  it('uses background only for website sync status', () => {
    expect(moreContextCopy.websiteSyncStatus).toContain('syncing your website');
    expect(moreContextCopy.websiteSyncStatus.toLowerCase()).toContain('background');
  });

  it('explains why GitHub repositories help agents', () => {
    expect(moreContextCopy.githubDescription).toContain('implementation details');
    expect(moreContextCopy.githubDescription).toContain('documentation gaps');
  });
});
