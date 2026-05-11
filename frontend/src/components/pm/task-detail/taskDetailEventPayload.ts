import type { Task, TaskDetail } from '@/lib/pmTypes';

export function buildPatchedTaskFromDetail(detail: TaskDetail): Task {
  return {
    ...detail.task,
    labels: detail.labels,
    epic_name: detail.epic_name ?? detail.task.epic_name,
    sprint_name: detail.sprint_name ?? detail.task.sprint_name,
    state_name: detail.state?.name ?? detail.task.state_name,
    state_type: detail.state?.state_type ?? detail.task.state_type,
    state_color: detail.state?.color ?? detail.task.state_color,
  };
}
