import type { TaskType, Priority } from '@/lib/pmTypes';

export interface PreparedTaskDraft {
  name: string;
  description: string;
  task_type: TaskType;
  priority: Priority;
  source_hash?: string;
}
export type DraftEdits = Partial<Record<keyof PreparedTaskDraft, boolean>>;

export function mergePreparedTaskDraft<T extends { name: string; description: string; task_type: string; priority: string }>(form: T, draft: PreparedTaskDraft, edited: DraftEdits): T {
  return { ...form,
    ...(!edited.name && { name: draft.name }),
    ...(!edited.description && { description: draft.description }),
    ...(!edited.task_type && { task_type: draft.task_type }),
    ...(!edited.priority && { priority: draft.priority }),
  };
}
