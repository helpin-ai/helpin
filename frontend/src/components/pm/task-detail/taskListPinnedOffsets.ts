export interface TaskListPinnedOffsets {
  displayId: number;
  typeIcon: number;
  name: number;
}

export function getTaskListPinnedOffsets({
  displayIdWidth,
  typeIconWidth,
  showTypeIcon,
}: {
  displayIdWidth: number;
  typeIconWidth: number;
  showTypeIcon: boolean;
}): TaskListPinnedOffsets {
  const displayId = 0;
  const typeIcon = displayIdWidth;
  const name = displayIdWidth + (showTypeIcon ? typeIconWidth : 0);

  return {
    displayId,
    typeIcon,
    name,
  };
}
