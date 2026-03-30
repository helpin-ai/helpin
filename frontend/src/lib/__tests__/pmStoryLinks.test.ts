import { describe, expect, it } from 'vitest';

import { buildStoryCopyUrl, buildStoryPath, buildStoryUrl } from '@/lib/pmStoryLinks';

describe('pmStoryLinks', () => {
  it('builds the canonical story path', () => {
    expect(buildStoryPath('acme-team', 'story-123')).toBe('/w/acme-team/pm/stories/story-123');
  });

  it('builds the canonical story url when workspace context is available', () => {
    expect(
      buildStoryUrl({
        origin: 'https://stage.helpin.ai',
        slug: 'acme-team',
        storyId: 'story-123',
      }),
    ).toBe('https://stage.helpin.ai/w/acme-team/pm/stories/story-123');
  });

  it('falls back to the current page story query when workspace context is unavailable', () => {
    expect(
      buildStoryCopyUrl({
        currentHref: 'https://stage.helpin.ai/w/acme-team/pm/epics/epic-7?tab=active',
        displayId: 42,
      }),
    ).toBe('https://stage.helpin.ai/w/acme-team/pm/epics/epic-7?tab=active&story=42');
  });
});
