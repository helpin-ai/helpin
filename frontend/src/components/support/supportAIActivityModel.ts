import type { SupportMessage } from '@/lib/pmTypes';

export type AIActivity = 'pause' | 'return' | 'handoff';

/** One "AI checked" line; source is set only when a tool call backs it. */
interface SupportHandoffCheck {
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

export interface SupportCodeCheckTask {
  kind: 'existing' | 'new';
  key?: string;
  /** Set only when the key resolved to a task in this workspace. */
  id?: string;
  text: string;
}

/** Code-check card saved in the note metadata; filled in place when done. */
export interface SupportCodeCheck {
  status: 'running' | 'done' | 'failed';
  focus?: string;
  message?: string;
  found?: string;
  reply?: string;
  docs?: string;
  task?: SupportCodeCheckTask;
  investigate?: string;
  unavailable_data?: string[];
  engineering?: string;
}

export function getSupportCodeCheck(message: Pick<SupportMessage, 'is_internal' | 'message_type' | 'metadata'>): SupportCodeCheck | null {
  if (!message.is_internal || message.message_type !== 'note') return null;
  try {
    const check = JSON.parse(message.metadata || '{}')?.code_check;
    return check && typeof check.status === 'string' ? check as SupportCodeCheck : null;
  } catch {
    return null;
  }
}

/**
 * AI cards from before the latest handoff fold to one line, so the current
 * handoff and its code check stay in focus.
 */
export function foldedSupportAICardIds(messages: SupportMessage[]): Set<string> {
  const latest = messages.findLast((message) => getSupportAIActivity(message) === 'handoff');
  if (!latest) return new Set();
  const cutoff = Date.parse(latest.created_at);
  return new Set(messages
    .filter((message) => message.id !== latest.id && Date.parse(message.created_at) < cutoff
      && (getSupportAIActivity(message) === 'handoff' || getSupportCodeCheck(message)))
    .map((message) => message.id));
}
