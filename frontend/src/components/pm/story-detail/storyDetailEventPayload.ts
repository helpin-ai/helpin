import type { Story, StoryDetail } from '@/lib/pmTypes';

export function buildPatchedStoryFromDetail(detail: StoryDetail): Story {
  return {
    ...detail.story,
    labels: detail.labels.length > 0 ? detail.labels : detail.story.labels,
    epic_name: detail.epic_name ?? detail.story.epic_name,
    sprint_name: detail.sprint_name ?? detail.story.sprint_name,
    owner_name: detail.owner_member
      ? (detail.owner_member.display_name ?? detail.owner_member.email)
      : detail.story.owner_name,
    state_name: detail.state?.name ?? detail.story.state_name,
    state_type: detail.state?.state_type ?? detail.story.state_type,
    state_color: detail.state?.color ?? detail.story.state_color,
  };
}
