export type TeamType = 'engineering' | 'product' | 'design' | 'support' | 'marketing' | 'sales' | 'hr' | 'operations' | 'custom';

export type DefaultStoryType = 'feature' | 'bug' | 'chore';

export type VisibilityFieldKey = 'priority' | 'story_type' | 'severity' | 'labels' | 'epic' | 'sprint' | 'estimate' | 'due_date' | 'blocked' | 'delivery' | 'dev_history';

export const TEAM_TYPE_OPTIONS: { value: TeamType; label: string; description: string }[] = [
  { value: 'engineering', label: 'Engineering', description: 'Software development and technical work.' },
  { value: 'product', label: 'Product', description: 'Product management and strategy.' },
  { value: 'design', label: 'Design', description: 'UI/UX design and creative work.' },
  { value: 'support', label: 'Support', description: 'Customer support and service.' },
  { value: 'marketing', label: 'Marketing', description: 'Marketing campaigns and content.' },
  { value: 'sales', label: 'Sales', description: 'Sales pipeline and outreach.' },
  { value: 'hr', label: 'HR', description: 'Human resources and people operations.' },
  { value: 'operations', label: 'Operations', description: 'Business operations and logistics.' },
  { value: 'custom', label: 'Custom', description: 'A custom team with no preset defaults.' },
];

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
  product: { defaultStoryType: 'feature', estimate: { enabled: true, scale: 'tshirt', extended: false, allow_zero: false, count_unestimated_as_one: true } },
  design: { defaultStoryType: 'feature', estimate: { enabled: true, scale: 'tshirt', extended: false, allow_zero: false, count_unestimated_as_one: true } },
  support: { defaultStoryType: 'chore', estimate: { enabled: false, scale: 'fibonacci', extended: false, allow_zero: false, count_unestimated_as_one: true } },
  marketing: { defaultStoryType: 'feature', estimate: { enabled: false, scale: 'tshirt', extended: false, allow_zero: false, count_unestimated_as_one: true } },
  sales: { defaultStoryType: 'feature', estimate: { enabled: false, scale: 'tshirt', extended: false, allow_zero: false, count_unestimated_as_one: true } },
  hr: { defaultStoryType: 'chore', estimate: { enabled: false, scale: 'tshirt', extended: false, allow_zero: false, count_unestimated_as_one: true } },
  operations: { defaultStoryType: 'chore', estimate: { enabled: false, scale: 'tshirt', extended: false, allow_zero: false, count_unestimated_as_one: true } },
  custom: { defaultStoryType: 'feature', estimate: { enabled: true, scale: 'fibonacci', extended: false, allow_zero: false, count_unestimated_as_one: true } },
};

export const WORKSPACE_TEAM_SUGGESTIONS: { name: string; teamType: TeamType; selected: boolean }[] = [
  { name: 'Engineering', teamType: 'engineering', selected: true },
  { name: 'Product', teamType: 'product', selected: true },
  { name: 'Design', teamType: 'design', selected: false },
  { name: 'Support', teamType: 'support', selected: false },
  { name: 'Marketing', teamType: 'marketing', selected: false },
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

export function buildPresetFieldVisibility(teamType: TeamType): Record<VisibilityFieldKey, boolean> {
  const base: Record<VisibilityFieldKey, boolean> = {
    priority: true,
    story_type: true,
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

  switch (teamType) {
    case 'engineering':
      return base;
    case 'product':
      return { ...base, delivery: false, dev_history: false };
    case 'design':
      return { ...base, severity: false, delivery: false, dev_history: false, estimate: false };
    case 'support':
      return { ...base, epic: false, sprint: false, estimate: false, delivery: false, dev_history: false };
    case 'marketing':
    case 'sales':
      return { ...base, severity: false, epic: false, estimate: false, delivery: false, dev_history: false };
    case 'hr':
    case 'operations':
      return { ...base, severity: false, epic: false, sprint: false, estimate: false, delivery: false, dev_history: false };
    case 'custom':
    default:
      return base;
  }
}
