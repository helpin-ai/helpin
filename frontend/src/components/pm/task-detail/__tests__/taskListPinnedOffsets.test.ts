import { describe, expect, it } from 'vitest';

import { getTaskListPinnedOffsets } from '../taskListPinnedOffsets';

describe('getTaskListPinnedOffsets', () => {
  it('keeps the name column directly after display id when type is hidden', () => {
    expect(
      getTaskListPinnedOffsets({
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
      getTaskListPinnedOffsets({
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
