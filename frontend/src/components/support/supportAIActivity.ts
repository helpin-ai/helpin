import type { SupportMessage } from '@/lib/pmTypes';

export type AIActivity = 'pause' | 'return' | 'handoff';

export function getSupportAIActivity(message: SupportMessage): AIActivity | null {
  if (!message.is_internal) return null;
  if (message.message_type === 'system') {
    if (message.system_event_type === 'ai_paused') return 'pause';
    if (message.system_event_type === 'ai_returned') return 'return';
  }
  if (message.message_type !== 'note') return null;
  try {
    const metadata = JSON.parse(message.metadata || '{}');
    if (metadata?.ai_handoff_brief === true) {
      return message.sender_user_id && metadata.agent_authored === false ? 'pause' : 'handoff';
    }
  } catch { /* Older notes can have no metadata. */ }
  // Narrow compatibility path for notes written before typed control events.
  if (message.sender_display_name === 'AI control' && message.sender_user_id &&
      message.content.startsWith('Returned to AI. AI will respond to the next customer message.')) return 'return';
  return null;
}
