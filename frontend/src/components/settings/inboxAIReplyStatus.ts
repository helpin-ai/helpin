import type { SupportInboxSettings } from '@/lib/pmTypes';

export function inboxAIReplyStatus(settings?: SupportInboxSettings, disabled = false): string {
  if (!settings) return 'Loading…';
  if (disabled || !settings.ai_enabled || !settings.ai_agent_id || settings.ai_response_mode === 'off') return 'Off';
  if (settings.ai_response_mode === 'internal_note') return 'Internal notes';
  if (settings.ai_response_mode !== 'ai_first') return 'Off';
  if (settings.ai_reply_channels === 'both') return 'On';
  return settings.ai_reply_channels === 'email' ? 'Email only' : 'Chat only';
}
