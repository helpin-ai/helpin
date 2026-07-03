export const DEFAULT_CHAT_WIDGET_AI_RESPONSE_MODE = 'ai_first';

export const CHAT_WIDGET_AI_RESPONSE_MODES = [
  { value: 'internal_note', label: 'Add internal note' },
  { value: 'ai_first', label: 'Reply directly' },
] as const;

export function isChatWidgetAIResponseModeActive(mode: string | null | undefined) {
  return CHAT_WIDGET_AI_RESPONSE_MODES.some((option) => option.value === mode);
}

export function getChatWidgetAIResponseModeForUI(mode: string | null | undefined) {
  return isChatWidgetAIResponseModeActive(mode) ? String(mode) : DEFAULT_CHAT_WIDGET_AI_RESPONSE_MODE;
}
