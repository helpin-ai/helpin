import { describe, expect, it } from 'vitest';

import {
  CHAT_WIDGET_AI_RESPONSE_MODES,
  DEFAULT_CHAT_WIDGET_AI_RESPONSE_MODE,
  getChatWidgetAIResponseModeForUI,
  isChatWidgetAIResponseModeActive,
  isChatWidgetAIFirst,
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

describe('isChatWidgetAIFirst', () => {
  it.each([
    [true, 'ai_first', 'chat', true],
    [true, 'ai_first', 'both', true],
    [true, 'ai_first', 'email', false],
    [false, 'ai_first', 'chat', false],
    [true, 'internal_note', 'chat', false],
    [true, 'off', 'chat', false],
  ])('matches visitor-facing AI for enabled=%s, mode=%s, channels=%s', (enabled, mode, channels, expected) => {
    expect(isChatWidgetAIFirst(enabled, mode, channels)).toBe(expected);
  });
});
