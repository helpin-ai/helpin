export type TeamType = 'engineering' | 'custom';

export type DefaultStoryType = 'feature' | 'bug' | 'chore';

export type VisibilityFieldKey = 'priority' | 'task_type' | 'severity' | 'labels' | 'epic' | 'sprint' | 'estimate' | 'due_date' | 'blocked' | 'delivery' | 'dev_history';

interface TeamPreset {
  defaultStoryType: DefaultStoryType;
  estimate: {
    enabled: boolean;
    scale: 'exponential' | 'fibonacci' | 'linear' | 'tshirt' | 'hours';
    extended: boolean;
    allow_zero: boolean;
    count_unestimated_as_one: boolean;
  };
}

export const TEAM_TYPE_PRESETS: Record<TeamType, TeamPreset> = {
  engineering: { defaultStoryType: 'feature', estimate: { enabled: true, scale: 'fibonacci', extended: false, allow_zero: false, count_unestimated_as_one: true } },
  custom: { defaultStoryType: 'feature', estimate: { enabled: false, scale: 'linear', extended: false, allow_zero: false, count_unestimated_as_one: true } },
};

export const WORKSPACE_TEAM_SUGGESTIONS: { name: string; teamType: TeamType; selected: boolean }[] = [
  { name: 'Engineering', teamType: 'engineering', selected: true },
  { name: 'Product', teamType: 'engineering', selected: true },
  { name: 'Design', teamType: 'custom', selected: false },
  { name: 'Support', teamType: 'custom', selected: false },
  { name: 'Marketing', teamType: 'custom', selected: true },
];

export function slugifyTeamHandle(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, '')
    .replace(/[\s_]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '');
}

/** Normalize legacy team types (product, design, support, etc.) to 'engineering' | 'custom'. */
export function normalizeTeamType(raw?: string): TeamType {
  return raw === 'engineering' ? 'engineering' : 'custom';
}

export function buildPresetFieldVisibility(teamType: TeamType): Record<VisibilityFieldKey, boolean> {
  const engineering: Record<VisibilityFieldKey, boolean> = {
    priority: true,
    task_type: true,
    severity: false,
    labels: true,
    epic: true,
    sprint: true,
    estimate: true,
    due_date: true,
    blocked: false,
    delivery: true,
    dev_history: true,
  };

  if (teamType === 'engineering') return engineering;

  return {
    ...engineering,
    task_type: false,
    estimate: false,
    delivery: false,
    dev_history: false,
  };
}
