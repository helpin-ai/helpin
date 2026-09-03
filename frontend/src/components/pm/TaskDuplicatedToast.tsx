import { showEntityCreatedToast } from '@/components/ui/entity-created-toast';
import { TASK_TYPE_CONFIG } from '@/lib/pmConstants';
import type { TaskType } from '@/lib/pmTypes';

interface TaskDuplicatedToastOptions {
  taskName: string;
  taskKey?: string | null;
  taskType?: TaskType;
  onOpen: () => void;
}

export function showTaskDuplicatedToast(options: TaskDuplicatedToastOptions) {
  return showEntityCreatedToast({
    entityLabel: 'Task',
    eyebrow: 'Task duplicated',
    title: options.taskName,
    identifier: options.taskKey ? { label: 'Task ID', value: options.taskKey } : undefined,
    tone: 'pm',
    icon: options.taskType ? TASK_TYPE_CONFIG[options.taskType].icon : undefined,
    openLabel: 'Open duplicate',
    onOpen: options.onOpen,
  });
}
