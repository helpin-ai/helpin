export type StoryListGroupByOption =
  | 'none'
  | 'workflow_state'
  | 'story_type'
  | 'priority'
  | 'severity'
  | 'epic'
  | 'sprint'
  | 'owner';

export interface StoryListGroupOption {
  value: StoryListGroupByOption;
  label: string;
}

export interface StoryListGroupingVisibility {
  story_type: boolean;
  priority: boolean;
  severity: boolean;
  epic: boolean;
  sprint: boolean;
}

const STORY_LIST_GROUP_OPTIONS: StoryListGroupOption[] = [
  { value: 'none', label: 'None' },
  { value: 'workflow_state', label: 'States' },
  { value: 'owner', label: 'Members' },
  { value: 'story_type', label: 'Story Type' },
  { value: 'priority', label: 'Priority' },
  { value: 'severity', label: 'Severity' },
  { value: 'epic', label: 'Epic' },
  { value: 'sprint', label: 'Sprint' },
];

export function getVisibleStoryListGroupOptions(
  visibility: StoryListGroupingVisibility,
): StoryListGroupOption[] {
  return STORY_LIST_GROUP_OPTIONS.filter((option) => {
    if (option.value === 'story_type') return visibility.story_type;
    if (option.value === 'priority') return visibility.priority;
    if (option.value === 'severity') return visibility.severity;
    if (option.value === 'epic') return visibility.epic;
    if (option.value === 'sprint') return visibility.sprint;
    return true;
  });
}
