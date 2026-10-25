import type { SupportMessage } from '@/lib/pmTypes';

export type AIActivity = 'pause' | 'return' | 'handoff';

/** One "AI checked" line; source is set only when a tool call backs it. */
export interface SupportHandoffCheck {
  source?: string;
  finding?: string;
}

/** Structured AI handoff saved in the note metadata. Empty parts are omitted. */
export interface SupportHandoffCard {
  issue?: string;
  reason: string;
  checked?: SupportHandoffCheck[];
  nothing_checked?: boolean;
  open_questions?: string[];
  unavailable_data?: string[];
  help_article_gap?: string;
  held_reply?: string;
  investigation_requested?: boolean;
}

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

/** Reads the structured handoff card; older notes have none and use their text. */
export function getSupportHandoffCard(message: Pick<SupportMessage, 'metadata'>): SupportHandoffCard | null {
  try {
    const card = JSON.parse(message.metadata || '{}')?.handoff;
    return card && typeof card.reason === 'string' && card.reason.trim() ? card as SupportHandoffCard : null;
  } catch {
    return null;
  }
}
