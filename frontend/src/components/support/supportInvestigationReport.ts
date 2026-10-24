import type { SupportMessage } from '@/lib/pmTypes';

const ENGINEERING_HEADING = /(^|\n)## For engineering\n/;

export interface SupportInvestigationReport {
  summary: string;
  engineering: string;
  suggestedReply: string;
}

/** Splits a support-first code check note so the inbox can fold engineering detail away. */
export function getSupportInvestigationReport(message: Pick<SupportMessage, 'metadata'>, content: string): SupportInvestigationReport | null {
  let metadata: { report_format?: unknown; suggested_reply?: unknown } | null;
  try {
    metadata = JSON.parse(message.metadata || '{}');
  } catch {
    return null;
  }
  if (metadata?.report_format !== 'support_v1') return null;
  const suggestedReply = typeof metadata.suggested_reply === 'string' ? metadata.suggested_reply.trim() : '';
  const match = ENGINEERING_HEADING.exec(content);
  // Translated notes may not keep the heading; show them whole.
  if (!match) return { summary: content, engineering: '', suggestedReply };
  return {
    summary: content.slice(0, match.index).trim(),
    engineering: content.slice(match.index + match[0].length).trim(),
    suggestedReply,
  };
}
