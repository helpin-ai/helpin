import { describe, it, expect } from 'vitest';
import { inboxAIReplyStatus } from '../inboxAIReplyStatus';
import type { SupportInboxSettings } from '@/lib/pmTypes';
const settings = { ai_enabled: true, ai_agent_id: 'agent', ai_response_mode: 'ai_first', ai_reply_channels: 'both' } as SupportInboxSettings;
describe('effective inbox AI reply status', () => {
  it('shows effective modes rather than inheritance labels', () => {
    expect(inboxAIReplyStatus(settings)).toBe('On');
    expect(inboxAIReplyStatus(settings, true)).toBe('Off');
    expect(inboxAIReplyStatus({...settings, ai_enabled:false})).toBe('Off');
    expect(inboxAIReplyStatus({...settings, ai_response_mode:'internal_note'})).toBe('Internal notes');
    expect(inboxAIReplyStatus({...settings, ai_reply_channels:'email'})).toBe('Email only');
    expect(inboxAIReplyStatus({...settings, ai_reply_channels:'chat'})).toBe('Chat only');
    expect(inboxAIReplyStatus({...settings, ai_agent_id:undefined})).toBe('Off');
  });
});
