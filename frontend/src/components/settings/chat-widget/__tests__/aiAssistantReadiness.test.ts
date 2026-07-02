import { describe, expect, it } from 'vitest';
import { getChatWidgetAIAssistantEnableBlocker, getChatWidgetAIAssistantToggleToast } from '../aiAssistantReadiness';
import { NO_AGENT_VALUE } from '../constants';

describe('getChatWidgetAIAssistantEnableBlocker', () => {
  it('requires a support agent before AI Assistant can be enabled', () => {
    expect(getChatWidgetAIAssistantEnableBlocker({ aiAgentId: NO_AGENT_VALUE, supportAgentCount: 2 })).toBe(
      'Select a support agent before enabling AI Assistant.',
    );
    expect(getChatWidgetAIAssistantEnableBlocker({ aiAgentId: '', supportAgentCount: 2 })).toBe(
      'Select a support agent before enabling AI Assistant.',
    );
  });

  it('asks users to create an agent when none exist', () => {
    expect(getChatWidgetAIAssistantEnableBlocker({ aiAgentId: NO_AGENT_VALUE, supportAgentCount: 0 })).toBe(
      'Create a support agent before enabling AI Assistant.',
    );
  });

  it('allows enabling when a support agent is selected', () => {
    expect(getChatWidgetAIAssistantEnableBlocker({ aiAgentId: 'agent-1', supportAgentCount: 1 })).toBeNull();
  });
});

describe('getChatWidgetAIAssistantToggleToast', () => {
  it('returns matching toast copy when AI Assistant is enabled or disabled', () => {
    expect(getChatWidgetAIAssistantToggleToast(true)).toBe('AI Assistant enabled');
    expect(getChatWidgetAIAssistantToggleToast(false)).toBe('AI Assistant disabled');
  });
});
