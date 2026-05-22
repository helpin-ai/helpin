export interface TaskListPinnedOffsets {
  select: number;
  displayId: number;
  typeIcon: number;
  name: number;
}

export function getTaskListPinnedOffsets({
  selectWidth,
  displayIdWidth,
  typeIconWidth,
  showTypeIcon,
}: {
  selectWidth: number;
  displayIdWidth: number;
  typeIconWidth: number;
  showTypeIcon: boolean;
}): TaskListPinnedOffsets {
  const select = 0;
  const displayId = selectWidth;
  const typeIcon = selectWidth + displayIdWidth;
  const name = selectWidth + displayIdWidth + (showTypeIcon ? typeIconWidth : 0);

  return {
    select,
    displayId,
    typeIcon,
    name,
  };
}
