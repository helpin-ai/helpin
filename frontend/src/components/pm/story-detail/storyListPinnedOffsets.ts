export interface StoryListPinnedOffsets {
  displayId: number;
  typeIcon: number;
  name: number;
}

export function getStoryListPinnedOffsets({
  displayIdWidth,
  typeIconWidth,
  showTypeIcon,
}: {
  displayIdWidth: number;
  typeIconWidth: number;
  showTypeIcon: boolean;
}): StoryListPinnedOffsets {
  const displayId = 0;
  const typeIcon = displayIdWidth;
  const name = displayIdWidth + (showTypeIcon ? typeIconWidth : 0);

  return {
    displayId,
    typeIcon,
    name,
  };
}
