export type TaskListGroupByOption =
  | 'none'
  | 'workflow_state'
  | 'task_type'
  | 'priority'
  | 'severity'
  | 'epic'
  | 'sprint'
  | 'owner';

export interface TaskListGroupOption {
  value: TaskListGroupByOption;
  label: string;
}

export interface TaskListGroupingVisibility {
  task_type: boolean;
  priority: boolean;
  severity: boolean;
  epic: boolean;
  sprint: boolean;
}

export interface TaskListGroupingContext {
  /** When set, the list is inside an epic — hide the "Epic" group option. */
  epicId?: string;
  /** When set, the list is inside a sprint — hide the "Sprint" group option. */
  sprintId?: string;
}

const TASK_LIST_GROUP_OPTIONS: TaskListGroupOption[] = [
  { value: 'none', label: 'None' },
  { value: 'workflow_state', label: 'States' },
  { value: 'owner', label: 'Members' },
  { value: 'task_type', label: 'Task Type' },
  { value: 'priority', label: 'Priority' },
  { value: 'severity', label: 'Severity' },
  { value: 'epic', label: 'Epic' },
  { value: 'sprint', label: 'Sprint' },
];

export function getVisibleTaskListGroupOptions(
  visibility: TaskListGroupingVisibility,
  context?: TaskListGroupingContext,
): TaskListGroupOption[] {
  return TASK_LIST_GROUP_OPTIONS.filter((option) => {
    if (option.value === 'task_type') return visibility.task_type;
    if (option.value === 'priority') return visibility.priority;
    if (option.value === 'severity') return visibility.severity;
    if (option.value === 'epic') return visibility.epic && !context?.epicId;
    if (option.value === 'sprint') return visibility.sprint && !context?.sprintId;
    return true;
  });
}
