import { describe, expect, it } from 'vitest';

import {
  CHAT_WIDGET_AI_RESPONSE_MODES,
  DEFAULT_CHAT_WIDGET_AI_RESPONSE_MODE,
  getChatWidgetAIResponseModeForUI,
  isChatWidgetAIResponseModeActive,
} from '../responseModes';

describe('CHAT_WIDGET_AI_RESPONSE_MODES', () => {
  it('defines the automatic modes shown in the dropdown', () => {
    expect(CHAT_WIDGET_AI_RESPONSE_MODES).toEqual([
      { value: 'internal_note', label: 'Add internal note' },
      { value: 'ai_first', label: 'Reply directly' },
    ]);
  });

  it('defaults to replying when AI Assistant is enabled', () => {
    expect(DEFAULT_CHAT_WIDGET_AI_RESPONSE_MODE).toBe('ai_first');
  });

  it('keeps legacy off mode out of the dropdown state', () => {
    expect(getChatWidgetAIResponseModeForUI('off')).toBe('ai_first');
    expect(getChatWidgetAIResponseModeForUI(undefined)).toBe('ai_first');
    expect(getChatWidgetAIResponseModeForUI('internal_note')).toBe('internal_note');
  });

  it('treats only visible modes as active automatic modes', () => {
    expect(isChatWidgetAIResponseModeActive('off')).toBe(false);
    expect(isChatWidgetAIResponseModeActive('')).toBe(false);
    expect(isChatWidgetAIResponseModeActive('internal_note')).toBe(true);
    expect(isChatWidgetAIResponseModeActive('ai_first')).toBe(true);
  });
});
