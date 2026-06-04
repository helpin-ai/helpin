export function normalizeCodingSessionPreviewPanelKey(value: string) {
  const normalized = value.trim().toLowerCase();
  if (normalized === 'story_plan') return 'task_plan';
  if (normalized === 'story_plan_doc') return 'task_plan_doc';
  return normalized;
}
