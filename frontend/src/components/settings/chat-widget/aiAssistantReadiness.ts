import { NO_AGENT_VALUE } from './constants';

export function getChatWidgetAIAssistantEnableBlocker({
  aiAgentId,
  supportAgentCount,
}: {
  aiAgentId: string | null | undefined;
  supportAgentCount: number;
}) {
  const selectedAgentID = aiAgentId?.trim();
  if (!selectedAgentID || selectedAgentID === NO_AGENT_VALUE) {
    return supportAgentCount > 0
      ? 'Select a support agent before enabling AI Assistant.'
      : 'Create a support agent before enabling AI Assistant.';
  }
  return null;
}

export function getChatWidgetAIAssistantToggleToast(enabled: boolean) {
  return enabled ? 'AI Assistant enabled' : 'AI Assistant disabled';
}
