export const DEFAULT_AI_HANDOFF_FOLLOWUPS = 5;

export function formatAIHandoffFollowupOption(count: number) {
  return `${count} AI follow-up${count === 1 ? '' : 's'}`;
}
