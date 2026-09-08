import type { Epic } from '@/lib/pmTypes';

export function groupEpicsByLifecycle(epics: Iterable<Epic>) {
  const notStarted: Epic[] = [];
  const inProgress: Epic[] = [];
  const completed: Epic[] = [];

  for (const epic of epics) {
    if (epic.completed) completed.push(epic);
    else if (epic.started) inProgress.push(epic);
    else notStarted.push(epic);
  }

  return [
    { label: 'Not started', epics: notStarted },
    { label: 'In progress', epics: inProgress },
    { label: 'Completed', epics: completed },
  ];
}
