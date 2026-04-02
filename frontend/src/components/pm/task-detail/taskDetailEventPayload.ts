import type { Story, StoryDetail } from '@/lib/pmTypes';

export function buildPatchedTaskFromDetail(detail: StoryDetail): Story {
  return {
    ...detail.task,
    labels: detail.labels.length > 0 ? detail.labels : detail.task.labels,
    epic_name: detail.epic_name ?? detail.task.epic_name,
    sprint_name: detail.sprint_name ?? detail.task.sprint_name,
    owner_name: detail.owner_member
      ? (detail.owner_member.display_name ?? detail.owner_member.email)
      : detail.task.owner_name,
    state_name: detail.state?.name ?? detail.task.state_name,
    state_type: detail.state?.state_type ?? detail.task.state_type,
    state_color: detail.state?.color ?? detail.task.state_color,
  };
}
