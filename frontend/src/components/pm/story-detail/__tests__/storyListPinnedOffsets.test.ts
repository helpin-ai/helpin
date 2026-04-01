import { describe, expect, it } from 'vitest';

import { getStoryListPinnedOffsets } from '../storyListPinnedOffsets';

describe('getStoryListPinnedOffsets', () => {
  it('keeps the name column directly after display id when type is hidden', () => {
    expect(
      getStoryListPinnedOffsets({
        displayIdWidth: 90,
        typeIconWidth: 40,
        showTypeIcon: false,
      }),
    ).toEqual({
      displayId: 0,
      typeIcon: 90,
      name: 90,
    });
  });

  it('includes the type width in the name offset when type is visible', () => {
    expect(
      getStoryListPinnedOffsets({
        displayIdWidth: 90,
        typeIconWidth: 40,
        showTypeIcon: true,
      }),
    ).toEqual({
      displayId: 0,
      typeIcon: 90,
      name: 130,
    });
  });
});
